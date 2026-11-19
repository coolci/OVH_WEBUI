package purchase

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/orderterm"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

// orderSubsidiary 归一化并校验账户的下单子公司。
//
// OVH 三个站点各自只认自家的 ovhSubsidiary,而且大小写敏感。实测 POST /order/cart:
//
//	eu.api.ovh.com       {"ovhSubsidiary":"IE"} → 200;"ie"/"US"/"CA" → 400 invalid ovhSubsidiary
//	api.us.ovhcloud.com  只有 "US" → 200
//	ca.api.ovh.com       只有 CA 系(CA/QC/SG/AU/...)→ 200
//
// 账户表里的 zone 可能是早期未归一化的小写值,也可能被直接改库改成跨大区的值
// (handlers/accounts.go 只在创建/更新时校验)。建车是抢购链路第一步,这一步 400
// 整单就没了,所以下单前再兜一次:大写化 + 校验归属大区,不合法退回 endpoint 的默认子公司。
// 归属表只有 internal/ovh/helpers.go 一份,这里只调它派生出来的函数,不另写映射。
// skipPrewarmKey 本轮不再用预热车(预热车已在 OVH 侧失效、正在重新装车时)
type skipPrewarmKey struct{}

// staleCartError 结账被拒的原因是"这辆车本身没了 / 过期了",而不是配置或库存问题。
// 只认确定性的 4xx:超时、429 这类不确定结果绝不能当成"车坏了"去重新下单。
func staleCartError(err error) bool {
	var apiErr *ovhsdk.APIError
	if !errors.As(err, &apiErr) || apiErr.Code < 400 || apiErr.Code >= 500 || apiErr.Code == 408 || apiErr.Code == 429 {
		return false
	}
	if apiErr.Code == http.StatusNotFound {
		return true
	}
	msg := strings.ToLower(apiErr.Message)
	return strings.Contains(msg, "cart") &&
		(strings.Contains(msg, "expired") || strings.Contains(msg, "does not exist") || strings.Contains(msg, "not found"))
}

func unassignedCartError(err error) bool {
	var apiErr *ovhsdk.APIError
	if !errors.As(err, &apiErr) || (apiErr.Code != http.StatusBadRequest && apiErr.Code != http.StatusForbidden) {
		return false
	}
	msg := strings.ToLower(apiErr.Message)
	return strings.Contains(msg, "assign")
}

func orderSubsidiary(state *app.State, acc types.OVHAccount, logTag string) string {
	// 大写化 / zone 为空时按 endpoint 推,统一走 catalog.SubsidiaryOfAccount(全项目唯一口径),
	// 这里只在它之上再加一道"跨大区"校验
	sub := catalog.SubsidiaryOfAccount(acc)
	fallback := ovh.DefaultSubsidiaryForEndpoint(acc.Endpoint)
	if !ovh.KnownSubsidiary(sub) || ovh.SubsidiaryRegion(sub) != ovh.EndpointRegion(acc.Endpoint) {
		state.Logger.Warn(fmt.Sprintf("账户 %s 的子公司 %q 不属于 endpoint %s 所在的 %s 区（OVH 会以 invalid ovhSubsidiary 拒绝建车），本次改用 %s",
			acc.Name, acc.Zone, acc.Endpoint, ovh.EndpointRegion(acc.Endpoint), fallback), logTag)
		return fallback
	}
	return sub
}

// regionAllowedValues 从 requiredConfiguration 响应里取 region 的合法取值 + 是否必填。
//
// schema 里 order.cart.ConfigurationRequirements 只声明了 label/required/type,但三区实测
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

// pickAllowedRegion 从 allowedValues 里挑一个 region:优先取 prefer(按机房推出来的那个),
// 挑不中就取第一个 —— 美区只有一个候选,取第一个就等于取对。
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

// filterHardwareOptions 剔除非硬件 / 许可证类选项，只留下真正决定机器配置的 addon。
// （注意 "panel" 不在过滤词里：FQN 推断出来的 addon 不会撞这词，
// 留着会误伤；旧版有 "panel" 是因为前端可能塞 cpanel 选项过来）
func filterHardwareOptions(state *app.State, opts []string, logSkipped bool) []string {
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
			if logSkipped {
				state.Logger.Info("跳过非硬件/许可证选项: "+opt, "purchase")
			}
			continue
		}
		filtered = append(filtered, opt)
	}
	return filtered
}

// fqnSegmentMatchesOption FQN 段 vs catalog option planCode 的匹配。
// availabilities 的 FQN 段是"短前缀"（ram-128g-noecc-2933 /
// softraid-4x3840nvme-pcie-gen4），而 catalog 给前端的 option value 是带机型后缀的
// 完整 planCode（ram-128g-noecc-2933-rise / softraid-4x3840nvme-pcie-gen4-24adv01-v2），
// 直接相等永远不命中 —— 这正是"按 FQN 精确匹配"整条修复此前空转的根因。
// 口径与前端 web/src/hooks/use-availability.ts 的 partMatchesValue 保持一致：
// 相等，或任一方是对方的 "x-" 前缀（补 "-" 是为了防 ram-1 错配 ram-128g）。
func fqnSegmentMatchesOption(seg, opt string) bool {
	s := strings.ToLower(strings.TrimSpace(seg))
	o := strings.ToLower(strings.TrimSpace(opt))
	if s == "" || o == "" {
		return false
	}
	if s == o {
		return true
	}
	if strings.HasPrefix(o, s+"-") && catalog.IsAllowedOptionSuffix(strings.TrimPrefix(o, s+"-")) {
		return true
	}
	if strings.HasPrefix(s, o+"-") && catalog.IsAllowedOptionSuffix(strings.TrimPrefix(s, o+"-")) {
		return true
	}
	return false
}

// fqnRelevantOptions 从用户提交的 options 里挑出真正参与 FQN 库存判定的硬件项。
//
// 前端抢购对话框提交的是所有分组的选中值（web/src/routes/servers.tsx 的 selectedValues），
// 里面混着 bandwidth-* / vrack-* / cpu 这类 addon —— 它们跟库存解耦、不出现在任何 FQN 里
// （前端 servers.tsx 自己也是这么判的）。拿它们去要求 FQN 覆盖，结果是每条 FQN 都不匹配，
// 整个"按配置判有货"退化成全量判定，等于没修。
//
// 判定不靠猜命名，而是用"本次 availabilities 里真实出现过的 FQN 段"当白名单：
// 段里查无此项的选项一律不参与覆盖判定。许可证/OS 类选项同样天然出局。
func fqnRelevantOptions(availabilities []map[string]interface{}, opts []string, planCode ...string) []string {
	segs := []string{}
	for _, av := range availabilities {
		fqn, _ := av["fqn"].(string)
		segs = append(segs, fqnHardwareParts(fqn, planCode...)...)
	}
	relevant := []string{}
	for _, opt := range opts {
		if strings.TrimSpace(opt) == "" {
			continue
		}
		for _, seg := range segs {
			if fqnSegmentMatchesOption(seg, opt) {
				relevant = append(relevant, opt)
				break
			}
		}
	}
	return relevant
}

// fqnCoversOptions 判断一条 dedicated.DatacenterAvailability 的 fqn
// （格式 <planCode>.<addon1>.<addon2>...）是否覆盖了用户要的全部硬件选项。
func fqnCoversOptions(fqn string, opts []string, planCode ...string) bool {
	if fqn == "" {
		return false
	}
	segs := fqnHardwareParts(fqn, planCode...)
	for _, want := range opts {
		hit := false
		for _, seg := range segs {
			if fqnSegmentMatchesOption(seg, want) {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

// preorderWait 预订"还没开放"的统一出口:只打日志,不写失败历史、不计失败次数。
// 开售前每一轮都会走到这里,写历史的话几分钟就能把抢购历史刷满。
func preorderWait(state *app.State, item *types.QueueItem, reason string) Outcome {
	state.Logger.Info(fmt.Sprintf("预订任务 %s(%s@%s)等待开放: %s", item.ID, item.PlanCode, item.Datacenter, reason), "purchase")
	return Outcome{WaitingForCatalog: true, Reason: reason}
}

// listingNotReady OVH 明确拒绝(400/404/409)—— 对预订来说是"尚未开放",不是失败。
// 429 / 5xx / 网络错误不在此列,它们仍按普通瞬时错误处理。
func listingNotReady(err error) bool {
	var apiErr *ovhsdk.APIError
	return errors.As(err, &apiErr) && (apiErr.Code == 400 || apiErr.Code == 404 || apiErr.Code == 409)
}

// fqnIsPlan FQN 的型号部分与 planCode 完全相同(型号本身可能含点号)
// optionsFromFQN 从库存条目 FQN(<planCode>.<addon1>.<addon2>…)推断要加购的 addon。
// 完整 planCode 之后才是 addon,型号本身可能包含点号。这些段是"短前缀"
// (ram-128g-noecc-2933),不是 eco/options 里带机型后缀的完整 planCode
// (ram-128g-noecc-2933-rise),匹配 addon 时要允许前缀命中。
// 下单和预热都用它 —— 两边推断口径必须一致,预热车才对得上号。
func optionsFromFQN(fqn, planCode string) []string {
	suffix, ok := strings.CutPrefix(fqn, planCode+".")
	if !ok || suffix == "" {
		return nil
	}
	return strings.Split(suffix, ".")
}

func fqnIsPlan(fqn, planCode string) bool {
	f, p := strings.ToLower(fqn), strings.ToLower(planCode)
	return p != "" && (f == p || strings.HasPrefix(f, p+"."))
}

// preorderConfigurationOptions 下单选项 = 预订映射出的 addon + 库存 FQN 里其余未被
// 匹配条件覆盖的段(如带宽)。库存条件本身已经由 order 里的 addon 表达,不重复添加。
func preorderConfigurationOptions(fqn, planCode string, stock, order []string) []string {
	options := append([]string(nil), order...)
	for _, part := range fqnHardwareParts(fqn, planCode) {
		covered := false
		for _, wanted := range stock {
			if strings.EqualFold(part, wanted) {
				covered = true
				break
			}
		}
		if !covered {
			options = append(options, part)
		}
	}
	return catalog.OrderOptions(options)
}

func preorderFQNCoversOptions(fqn string, opts []string, planCode ...string) bool {
	parts := fqnHardwareParts(fqn, planCode...)
	if len(parts) == 0 {
		return len(opts) == 0
	}
	for _, wanted := range opts {
		found := false
		for _, part := range parts {
			if strings.EqualFold(part, wanted) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// pickPricing 从 order.cart.GenericProductPricing[] 里挑一条计价填 duration / pricingMode。
//
// 优先级：duration 必须与主商品一致 > 同 duration 内优先 rental+renew（按周期续费那条）
// > 实在没有同 duration 才退而求其次。
// 为什么 duration 一致排第一：addon 的 prices[] 常见 P1M/P12M/P24M 三条都是 rental+renew，
// 先按"rental+renew"挑等于取数组里碰巧的第一条，跟主商品 duration 无关；
// 一旦挑中不同 duration，OVH 直接拒这个 addon，而两处 addon POST 都是 fail-fast
// （抢购整单取消 / 询价 success:false），代价远大于"少续了一档"。
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

func hasCapacity(raw interface{}, want string) bool {
	list, _ := raw.([]interface{})
	for _, c := range list {
		if s, _ := c.(string); s == want {
			return true
		}
	}
	return false
}

// lookupEcoPricing 查 GET /order/cart/{cartId}/eco（order.cart.GenericProductDefinition[]）
// 里某个 planCode 的真实可用计价。只在按 P1M/default 加购被 OVH 拒之后才调，
// 保证正常月付机型的抢购链路上不会多一次请求。
// wanted 是要买的周期:月付只在按 P1M/default 加购被拒后才查,非月付在加购前查。
func lookupEcoPricing(state *app.State, client *ovhsdk.Client, cartID, planCode, wanted string) (string, string, bool) {
	var defs []map[string]interface{}
	if err := client.Get("/order/cart/"+cartID+"/eco", &defs); err != nil {
		state.Logger.Warn("查询 Eco 计价失败: "+err.Error(), "purchase")
		return "", "", false
	}
	for _, def := range defs {
		if pc, _ := def["planCode"].(string); pc != planCode {
			continue
		}
		// 只认同一周期(P1M)下的其他计价模式;目录里没有月付就是没有,不能改成年付重试
		d, pm, ok := orderterm.Pick(def["prices"], wanted, "")
		return d, pm, ok
	}
	state.Logger.Debug("在 Eco 目录里未找到 planCode: "+planCode, "purchase")
	return "", "", false
}

func extract(v interface{}) *float64 {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		if f, ok := numconv.ToFloat64(m["value"]); ok {
			return &f
		}
		return nil
	}
	if f, ok := numconv.ToFloat64(v); ok {
		return &f
	}
	return nil
}

// Explicit planCode keeps punctuation in custom model names out of hardware matching.
func fqnHardwareParts(fqn string, planCode ...string) []string {
	if len(planCode) > 0 {
		suffix, ok := strings.CutPrefix(fqn, planCode[0]+".")
		if !ok || suffix == "" {
			return nil
		}
		return strings.Split(suffix, ".")
	}
	parts := strings.Split(fqn, ".")
	return parts[1:]
}
