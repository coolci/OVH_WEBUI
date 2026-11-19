package catalog

import (
	"fmt"
	"math"
	"strings"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// ResolveRegion 给下单/询价用的统一入口:先查目录,失败退回静态兜底。
// 返回 (region, 来源说明),region 为空表示不需要发 region 配置项。
func ResolveRegion(state *app.State, accountID, planCode, apiDC string) (string, string) {
	if r, err := RegionForPlan(state, accountID, planCode, apiDC); err == nil {
		return r, "catalog"
	} else {
		state.Logger.Warn(fmt.Sprintf("[region] 从目录解析 %s@%s 的区域失败(%s),退回静态兜底",
			planCode, apiDC, err.Error()), "purchase")
	}
	acc, _ := state.FindAccount(accountID)
	// 兜底也必须用归一化后的子公司:acc.Zone 原样传下去时,zone 为空的美区账户
	// (建号不填 zone 很常见)会被当成非 US 子公司,gra/fra 这些美区也在卖的机房
	// 就会算出 europe —— 而美区目录里 143 个 plan 的 region 只有 united_states,
	// 每一单都卡在 configuration。SubsidiaryOfAccount 会按 endpoint 补出 US。
	return FallbackRegion(apiDC, SubsidiaryOfAccount(acc)), "fallback"
}

// FallbackRegion 目录拉不到时的静态兜底:(机房, 子公司) → region。
// 权威解析永远是目录(RegionForPlan),这里只是网络故障时的最后一道。
//
// 在 ovh.RegionForDCInSubsidiary 之上补两件它做不到的事:
//  1. 归一化机房码 —— 三区的 availabilities 都会返回长码(ca-east-tor-a /
//     eu-west-par-a / us-east-vin-a),而前端机房芯片就是从 availabilities 来的,
//     长码会原样进下单链路;那边只按短码前缀匹配,长码一律落空串。
//  2. tor(多伦多)这类枚举里有、静态表里没有的城市,用目录实测的 region 桶补齐。
func FallbackRegion(apiDC, subsidiary string) string {
	dc := normalizeDCCity(apiDC)
	if dc == "" && !strings.EqualFold(strings.TrimSpace(subsidiary), "US") {
		// 连机房都没有就别猜:非 US 子公司有 canada/europe 两种取值,猜错等于提交一个
		// 该 plan 不接受的配置。返回空串,让调用方去问购物车的 requiredConfiguration。
		return ""
	}
	if r := ovh.RegionForDCInSubsidiary(dc, subsidiary); r != "" {
		return r
	}
	// 到这里只可能是非 US 子公司(US 子公司那条分支恒定返回 united_states)。
	// 非 US 子公司的目录里只有 canada / europe 两种取值,所以 united_states 桶
	// (vin/hil)必须继续返回空串,让调用方去问购物车的 requiredConfiguration,
	// 而不是提交一个该子公司根本没有的取值。
	if b := regionBucketForDC(apiDC); b != "united_states" {
		return b
	}
	return ""
}

// NormalizeDCCity 把 OVH 返回的机房码归一成城市三字码(gra1→gra、ca-east-tor-a→tor)。
// 导出给其它包用,免得各处再各写一份长码解析。
func NormalizeDCCity(dc string) string { return normalizeDCCity(dc) }

// DatacentersForPlan 取该 (账户子公司, planCode) 在目录里可选的机房列表(目录给的原始顺序)。
// 走的是同一份 120 秒缓存,不额外打 OVH。
func DatacentersForPlan(state *app.State, accountID, planCode string) ([]string, error) {
	acc, _ := state.FindAccount(accountID)
	subsidiary := SubsidiaryOfAccount(acc)
	cat, err := loadSubsidiaryCatalog(state, subsidiary)
	if err != nil {
		return nil, err
	}
	pc, ok := cat.plans[planCode]
	if !ok {
		return nil, fmt.Errorf("%w: %s(子公司 %s / %s 站点)", ErrPlanNotInCatalog, planCode, subsidiary, ovh.SubsidiaryRegion(subsidiary))
	}
	return append([]string(nil), pc.datacenters...), nil
}

// PreferredDatacenterForPlan 调用方没指定机房时,给这个 plan 挑一个"它确实在卖"的机房。
// 取不到目录(或该 plan 不在 eco 目录里,例如 Scale/HCI 那些机型)时返回空串,
// 由调用方退回大区默认机房 —— 不能因为挑不出来就不查。
//
// 为什么不能按大区写死一个默认机房(2026-08 实测三份公开 eco 目录):
//
//	US 143 个 plan 里含 vin 的只有 33 个 —— 42 个只卖欧洲机房、36 个只卖 bhs、32 个只卖 sgp/syd/ynm
//	EU/CA 99 个 plan 里含 gra 的 47 个、含 bhs 的 42 个,51 个只卖 sgp/syd/ynm
//
// 写死大区默认机房等于让一大半机型拿一个自己根本不卖的机房去询价(cart 直接报配置不可订购)。
func PreferredDatacenterForPlan(state *app.State, accountID, planCode string) string {
	if planCode == "" {
		return ""
	}
	dcs, err := DatacentersForPlan(state, accountID, planCode)
	if err != nil || len(dcs) == 0 {
		return ""
	}
	acc, _ := state.FindAccount(accountID)
	return pickDatacenter(dcs, regionBucketForSubsidiary(SubsidiaryOfAccount(acc)))
}

// pickDatacenter 在目录列出的机房里挑一个。纯函数,便于测试。
// 目录给的顺序不是字母序,而是 OVH 自己的主次顺序(欧洲机房在前、bhs 在后),
// 所以"第一个"本身就是个合理的缺省;但要先照顾账户所在大区 ——
// 一个加区账户对 [fra gra lon rbx sbg waw bhs] 这种 plan 应该默认 bhs 而不是 fra。
// preferredDCByBucket 各大区的主力机房。同一大区有多个可选机房时优先取它,
// 取不到再退回目录列表里第一个同区机房。
//
// 为什么要这一层:OVH 目录里的机房顺序是它自己的顺序(欧区把 fra 排在 gra 前面),
// 直接取"第一个同区机房"会让欧区的缺省询价机房从格拉沃利讷变成法兰克福 ——
// 功能上没错,但对老用户是无谓的观感变化,而 gra 在这些 plan 的机房列表里本来就有。
var preferredDCByBucket = map[string][]string{
	"europe":        {"gra", "rbx", "sbg"},
	"canada":        {"bhs"},
	"united_states": {"vin", "hil"},
}

func pickDatacenter(dcs []string, wantBucket string) string {
	if len(dcs) == 0 {
		return ""
	}
	if wantBucket != "" {
		// 先看主力机房在不在该 plan 的可选列表里
		for _, want := range preferredDCByBucket[wantBucket] {
			for _, dc := range dcs {
				if normalizeDCCity(dc) == want {
					return dc
				}
			}
		}
		// 主力机房不卖这个机型(实测欧区 99 个 plan 里有 52 个不卖 gra),
		// 退回目录里第一个同区机房
		for _, dc := range dcs {
			if regionBucketForDC(dc) == wantBucket {
				return dc
			}
		}
	}
	return dcs[0]
}

// regionBucketForSubsidiary 子公司 → 它自己那档 region 桶,只用来在多个可选机房里
// 优先挑同区的那个。子公司归属仍然取自 ovh.SubsidiaryRegion 这张唯一权威表,
// 这里只是把 EU/US/CA 翻译成目录里的 region 字符串。
func regionBucketForSubsidiary(subsidiary string) string {
	switch ovh.SubsidiaryRegion(subsidiary) {
	case "US":
		return "united_states"
	case "CA":
		return "canada"
	default:
		return "europe"
	}
}

// WarmRegionCache 启动时后台预热各账户子公司的区域配置。
// 首次解析要拉一份 10MB 的目录(实测 2-7 秒),放在抢购那一单的链路里等于白白让掉先手;
// 启动时先拉好,后续下单直接命中内存。失败不影响功能(下单时会自己重试或走静态兜底)。
func WarmRegionCache(state *app.State) {
	state.AccountsMu.RLock()
	subs := map[string]struct{}{}
	for _, a := range state.Accounts {
		subs[SubsidiaryOfAccount(a)] = struct{}{}
	}
	state.AccountsMu.RUnlock()

	for s := range subs {
		if _, err := loadSubsidiaryCatalog(state, s); err != nil {
			state.Logger.Warn(fmt.Sprintf("[region] 预热 %s 目录失败(下单时会重试): %s", s, err.Error()), "purchase")
		}
	}
}

// PlanPrice 一套配置的完整报价。
type PlanPrice struct {
	// Monthly / Install 统一使用不含税金额(price，不计入 tax)。
	Monthly  float64
	Install  float64
	Currency string
	// Partial 有 addon 在目录里查不到 —— 报价可能偏低,调用方要说明。
	Partial bool
}

// PriceForOptions 按 planCode + 选中的 addon 算月费和安装费。
//
// 口径必须和前端 use-availability.ts 的 computePrice 一致:
// base plan 的月费/安装费,加上每个选中 addon 各自的月费/安装费。
// 统一按不含税价格计算。
// 走的是已经缓存 120 秒的公开目录,不消耗账户 API 配额。
//
// 为什么不复用询价接口(price.GetInternal):那个要真的建购物车、加商品、拿 summary、删车,
// 一次好几秒还会占配额。补货通知是"有货那一刻要立刻发出去"的东西,不能等。
func PriceForOptions(state *app.State, accountID, planCode string, options []string) (PlanPrice, error) {
	acc, _ := state.FindAccount(accountID)
	subsidiary := SubsidiaryOfAccount(acc)
	cat, err := loadSubsidiaryCatalog(state, subsidiary)
	if err != nil {
		return PlanPrice{}, err
	}
	pc, ok := cat.plans[planCode]
	if !ok {
		return PlanPrice{}, fmt.Errorf("%w: %s(子公司 %s)", ErrPlanNotInCatalog, planCode, subsidiary)
	}
	if !pc.monthly.OK {
		return PlanPrice{}, fmt.Errorf("目录里 %s 没有可用的月费计价", planCode)
	}
	out := PlanPrice{
		Monthly:  pc.monthly.Price,
		Install:  pc.install.Price,
		Currency: cat.currency,
	}
	for _, o := range options {
		a, ok := cat.addons[o]
		if !ok {
			// 目录里没有这个 addon —— 报价会偏低,必须让调用方知道,
			// 不能悄悄给出一个"看起来很便宜"的数字。
			out.Partial = true
			continue
		}
		if a.monthly.OK {
			out.Monthly += a.monthly.Price
		}
		if a.install.OK {
			out.Install += a.install.Price
		}
	}
	out.Monthly = math.Round(out.Monthly*100) / 100
	out.Install = math.Round(out.Install*100) / 100
	return out, nil
}

// ConfigSetupFee 目录里一套配置的安装费：plan 自身 + 每个附加选项（内存/硬盘/带宽等）各自的安装费，
// 不含税 / 税额分开返回。目录里查不到的选项（额外 IP、备份空间等不属于硬件配置的服务）不计入，数量记在 skipped。
// 仅表示当前公开目录价，不代表已交付服务器当初实际支付的安装费。
func ConfigSetupFee(state *app.State, accountID, planCode string, addonCodes []string) (fee PriceParts, currency string, skipped int, err error) {
	acc, _ := state.FindAccount(accountID)
	subsidiary := SubsidiaryOfAccount(acc)
	cat, err := loadSubsidiaryCatalog(state, subsidiary)
	if err != nil {
		return PriceParts{}, "", 0, err
	}
	pc, ok := cat.plans[planCode]
	if !ok {
		return PriceParts{}, "", 0, fmt.Errorf("%w: %s(子公司 %s)", ErrPlanNotInCatalog, planCode, subsidiary)
	}
	fee = PriceParts{OK: true}
	if pc.install.OK {
		fee.Price, fee.Tax = pc.install.Price, pc.install.Tax
	}
	for _, code := range addonCodes {
		a, ok := cat.addons[code]
		if !ok {
			skipped++
			continue
		}
		if a.install.OK {
			fee.Price += a.install.Price
			fee.Tax += a.install.Tax
		}
	}
	return fee, cat.currency, skipped, nil
}
