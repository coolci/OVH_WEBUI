package price

import (
	"fmt"
	"math"
	"strings"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/orderterm"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

func orderSubsidiary(state *app.State, acc types.OVHAccount) string {
	// 大写化 / zone 为空时按 endpoint 推,统一走 catalog.SubsidiaryOfAccount(全项目唯一口径),
	// 这里只在它之上再加一道"跨大区"校验
	sub := catalog.SubsidiaryOfAccount(acc)
	fallback := ovh.DefaultSubsidiaryForEndpoint(acc.Endpoint)
	if !ovh.KnownSubsidiary(sub) || ovh.SubsidiaryRegion(sub) != ovh.EndpointRegion(acc.Endpoint) {
		state.Logger.Warn(fmt.Sprintf("账户 %s 的子公司 %q 不属于 endpoint %s 所在的 %s 区（OVH 会以 invalid ovhSubsidiary 拒绝建车），本次改用 %s",
			acc.Name, acc.Zone, acc.Endpoint, ovh.EndpointRegion(acc.Endpoint), fallback), "price")
		return fallback
	}
	return sub
}

// regionAllowedValues / pickAllowedRegion 与 purchase.go 的同名函数保持同一份逻辑。
//
// schema 里 order.cart.ConfigurationRequirements 只声明 label/required/type,但三区实测
// GET /order/cart/{cartId}/item/{itemId}/requiredConfiguration 都会带 allowedValues,
// 而且这份值是按"这辆车的子公司 + 这个 planCode"算出来的,天然是本区正确值:
// US 车 → ["united_states"],EU/CA 车 → ["canada","europe"]。
func regionAllowedValues(required []map[string]interface{}) ([]string, bool) {
	for _, conf := range required {
		if label, _ := conf["label"].(string); label != "region" {
			continue
		}
		mandatory, _ := conf["required"].(bool)
		raw, _ := conf["allowedValues"].([]interface{})
		vals := make([]string, 0, len(raw))
		for _, v := range raw {
			if s, _ := v.(string); s != "" {
				vals = append(vals, s)
			}
		}
		return vals, mandatory
	}
	return nil, false
}

func pickAllowedRegion(allowed []string, prefer string) string {
	if len(allowed) == 0 {
		return ""
	}
	for _, a := range allowed {
		if strings.EqualFold(a, prefer) {
			return a
		}
	}
	return allowed[0]
}

// pickPricing 从 order.cart.GenericProductPricing[] 里挑一条计价填 duration / pricingMode。
//
// 优先级:duration 必须与主商品一致 > 同 duration 内优先 rental+renew(按周期续费那条)
// > 实在没有同 duration 才退而求其次;prices 为空或结构异常时回退 P1M/default。
// 为什么 duration 一致排第一:addon 的 prices[] 常见 P1M/P12M/P24M 三条都是 rental+renew,
// 先按 rental+renew 挑等于取数组里碰巧的第一条、跟主商品 duration 无关;
// 一旦挑中与主商品不同的 duration,OVH 直接拒这个 addon,而这里是 fail-fast
// (整次询价 success:false),代价远大于"少续了一档"。
// 与 purchase.go 的 pickPricing 必须保持同一份逻辑。
func pickPricing(pricesRaw interface{}, duration, preferredMode string) (string, string, bool) {
	// 只在同一购买周期内挑计价(上游 orderterm.Pick):绝不换成别的周期,
	// 否则月付主机会挂上年付配件,或整单被悄悄换成年付。
	if d, pm, ok := orderterm.Pick(pricesRaw, duration, preferredMode); ok {
		return d, pm, true
	}
	// 选项根本没给价格表:没有依据判断,按主商品周期原样发出,由 OVH 校验。
	// 给了价格表却没有这个周期 = 确定买不到,返回 false 让调用方取消。
	if list, _ := pricesRaw.([]interface{}); len(list) == 0 {
		if preferredMode == "" {
			preferredMode = "default"
		}
		return duration, preferredMode, true
	}
	return "", "", false
}

// filterHardwareOptions 剔除许可证 / OS / 控制面板类选项,只留真正决定机器配置的 addon。
// 口径与 purchase.go 的同名函数一致 —— 两处必须同进同退,否则会出现
// "询价说这个组合不可定价、抢购却能下单"(或反之)的分裂。
// 这里复制一份而不是 import purchase:price 是 purchase 的下游(quick_order 先询价再入队),
// 反向依赖会成环。
func filterHardwareOptions(state *app.State, opts []string) []string {
	filtered := []string{}
	for _, opt := range opts {
		if opt == "" {
			continue
		}
		lc := strings.ToLower(opt)
		skip := false
		for _, term := range []string{"windows-server", "sql-server", "cpanel-license", "plesk-",
			"-license-", "os-", "control-panel", "license", "security"} {
			if strings.Contains(lc, term) {
				skip = true
				break
			}
		}
		if skip {
			state.Logger.Debug("询价跳过非硬件/许可证选项: "+opt, "price")
			continue
		}
		filtered = append(filtered, opt)
	}
	return filtered
}

func hasCapacity(raw interface{}, want string) bool {
	list, _ := raw.([]interface{})
	for _, c := range list {
		if s, _ := c.(string); s == want {
			return true
		}
	}
	return false
}

// lookupEcoPricing 查 GET /order/cart/{cartId}/eco(order.cart.GenericProductDefinition[])
// 里某个 planCode 的真实可用计价。只在按 P1M/default 加购被 OVH 拒之后才调,
// 这样正常的月付机型一次多余请求都不会发。
func lookupEcoPricing(state *app.State, client *ovhsdk.Client, cartID, planCode, preferDuration string) (string, string, bool) {
	var defs []map[string]interface{}
	if err := client.Get("/order/cart/"+cartID+"/eco", &defs); err != nil {
		state.Logger.Warn("查询 Eco 计价失败: "+err.Error(), "price")
		return "", "", false
	}
	for _, def := range defs {
		if pc, _ := def["planCode"].(string); pc != planCode {
			continue
		}
		// 只认同一周期下的其他计价模式;没有该周期就是买不到,不能换周期重试
		d, pm, ok := orderterm.Pick(def["prices"], preferDuration, "")
		return d, pm, ok
	}
	state.Logger.Debug("在 Eco 目录里未找到 planCode: "+planCode, "price")
	return "", "", false
}

// extractPriceField 兼容字典形式（{value, currencyCode}）与直接值，统一转 float64
// OVH SDK 用 UseNumber，数字是 json.Number，必须经过 numconv 才能拿到正确值
func extractPriceField(v interface{}) (interface{}, string) {
	if v == nil {
		return nil, ""
	}
	if m, ok := v.(map[string]interface{}); ok {
		currency, _ := m["currencyCode"].(string)
		if f, ok := numconv.ToFloat64(m["value"]); ok {
			return f, currency
		}
		return nil, currency
	}
	if f, ok := numconv.ToFloat64(v); ok {
		return f, ""
	}
	return nil, ""
}

// splitRecurringAndSetup 把购物车 summary 的首张账单合计拆成周期费与一次性安装费。
// details[] 里 detailType=INSTALLATION 的行即安装费；同税率按比例拆分含税金额，自动识别明细行是否含税。
// withTax/withoutTax 保持为合计不变，供「价格有效」闸门继续使用。
func splitRecurringAndSetup(info *PriceInfo, summary map[string]interface{}) {
	if info == nil || info.Prices == nil || summary == nil {
		return
	}
	withTax, okWith := numconv.ToFloat64(info.Prices["withTax"])
	withoutTax, okWithout := numconv.ToFloat64(info.Prices["withoutTax"])
	if !okWith || !okWithout || withoutTax <= 0 {
		return
	}
	setupLines, allLines := 0.0, 0.0
	if details, ok := summary["details"].([]interface{}); ok {
		for _, raw := range details {
			d, _ := raw.(map[string]interface{})
			if d == nil {
				continue
			}
			v, _ := extractPriceField(d["totalPrice"])
			f, ok := numconv.ToFloat64(v)
			if !ok {
				continue
			}
			allLines += f
			if d["detailType"] == "INSTALLATION" && f > 0 {
				setupLines += f
			}
		}
	}
	if setupLines <= 0 {
		info.Prices["recurringWithTax"] = withTax
		info.Prices["recurringWithoutTax"] = withoutTax
		return
	}
	setupExcl, setupWith := setupLines, withTax*setupLines/withoutTax
	if math.Abs(allLines-withTax) < math.Abs(allLines-withoutTax) {
		setupWith, setupExcl = setupLines, withoutTax*setupLines/withTax
	}
	if setupWith > withTax || setupExcl > withoutTax {
		return
	}
	recurringExcl, recurringWith := withoutTax-setupExcl, withTax-setupWith
	round := func(v float64) float64 { return math.Round(v*100) / 100 }
	info.Prices["recurringWithTax"] = round(recurringWith)
	info.Prices["recurringWithoutTax"] = round(recurringExcl)
	info.Prices["setupWithTax"] = round(setupWith)
	info.Prices["setupWithoutTax"] = round(setupExcl)
}
