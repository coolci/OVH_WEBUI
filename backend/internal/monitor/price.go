package monitor

import (
	"fmt"
	"strings"
	"time"

	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/price"
)

// optionsFromConfig 取本次配置组合要询价的 addon 列表
func optionsFromConfig(configInfo map[string]interface{}) []string {
	options := []string{}
	if configInfo == nil {
		return options
	}
	if opts, ok := configInfo["options"].([]string); ok {
		return opts
	}
	if optsRaw, ok := configInfo["options"].([]interface{}); ok {
		for _, o := range optsRaw {
			if s, ok := o.(string); ok {
				options = append(options, s)
			}
		}
	}
	return options
}

// accountIDFromConfig 取本次询价该用哪个账户。
// 账户随 configInfo 走而不是加参数,是为了让 notify.go 里那条"通知时兜底再查一次价格"的
// 路径也能拿到同一个账户 —— 询价账户决定 endpoint 和 ovhSubsidiary,
// 用默认账户验价、却用订阅指定账户下单,两边的可售性和币种都可能对不上。
func accountIDFromConfig(configInfo map[string]interface{}) string {
	if configInfo == nil {
		return ""
	}
	id, _ := configInfo["account_id"].(string)
	return id
}

// —— 通知里的币种 ——
//
// 24 个子公司分属 EU / US / CA 三个站点,计价币种一共 11 种,逐个用
// /v1/order/catalog/public/eco?ovhSubsidiary=X 的 locale.currencyCode 实测过:
//
//	EUR: CZ DE ES FI FR IE IT LT NL PT   GBP: GB   PLN: PL
//	MAD: MA   TND: TN   XOF: SN                          (以上 EU 站点)
//	CAD: CA QC   USD: ASIA WE WS   AUD: AU   INR: IN   SGD: SG   (以上 CA 站点)
//	USD: US                                              (US 站点)
//
// 以前这里在 currencyCode 缺失时一律兜底成 "EUR",并且符号表只认 EUR/USD ——
// 美区/加区账户的通知会把 USD/CAD 的金额印成 "€xx",用户按欧元判断值不值得抢。
// 现在兜底改成按账户子公司取,拿不准就只印 ISO 代码,绝不假装是欧元。
//
// TODO(needsOther): 这张表和 ovh.subsidiaryRegions 是同一个维度的东西,
// 更适合放在 internal/ovh/helpers.go 里供 internal/price 一起用
// (internal/price/price.go 现在只能在 summary 没回币种时留空)。
var subsidiaryCurrency = map[string]string{
	// EU 站点
	"CZ": "EUR", "DE": "EUR", "ES": "EUR", "EU": "EUR", "FI": "EUR", "FR": "EUR",
	"IE": "EUR", "IT": "EUR", "LT": "EUR", "NL": "EUR", "PT": "EUR",
	"GB": "GBP", "PL": "PLN", "MA": "MAD", "TN": "TND", "SN": "XOF",
	// CA 站点
	"CA": "CAD", "QC": "CAD", "ASIA": "USD", "WE": "USD", "WS": "USD",
	"AU": "AUD", "IN": "INR", "SG": "SGD",
	// US 站点
	"US": "USD",
}

// currencyForSubsidiary 子公司的计价币种;未知子公司返回 ""(宁可不印符号也不猜)
func currencyForSubsidiary(sub string) string {
	return subsidiaryCurrency[strings.ToUpper(strings.TrimSpace(sub))]
}

// subsidiaryOfPricingAccount 询价用的那个账户的 ovhSubsidiary。
// accountID 为空时 FindAccount 返回默认账户 —— 与 check.go 里 choice.accountID==""
// 落默认账户查库存是同一个口径,验价和查库存不会跑到两个账户上去。
// 子公司的取法统一走 catalog.SubsidiaryOfAccount(zone 优先,空则按 endpoint 推),
// 不在这里重写一遍 zone/endpoint 的兜底逻辑。
func (m *Monitor) subsidiaryOfPricingAccount(accountID string) string {
	acc, ok := m.state.FindAccount(accountID)
	if !ok {
		return ""
	}
	return catalog.SubsidiaryOfAccount(acc)
}

// defaultCurrencyForAccount OVH 没回 currencyCode 时,按账户所属子公司兜底。
func (m *Monitor) defaultCurrencyForAccount(accountID string) string {
	return currencyForSubsidiary(m.subsidiaryOfPricingAccount(accountID))
}

// formatMoney 把金额渲染成通知里那一行。
// $ 家族(USD/CAD/AUD/SGD)必须带国别前缀:光一个 "$" 在同时管着美区账户和
// 加区账户的监控里是有歧义的,CAD 和 USD 差着三成汇率。
// 没有独占符号的币种(MAD/TND/XOF...)直接印 ISO 代码。
func formatMoney(currency string, v float64) string {
	return formatAmount(currency, v) + "/月"
}

// formatAmount 只格式化金额,不带周期后缀。
//
// 拆出来是因为安装费是**一次性**的 —— 用 formatMoney 会印成 "€17.99/月",
// 把一笔一次性费用说成月付,直接误导花多少钱。
func formatAmount(currency string, v float64) string {
	switch strings.ToUpper(currency) {
	case "EUR":
		return fmt.Sprintf("€%.2f", v)
	case "GBP":
		return fmt.Sprintf("£%.2f", v)
	case "INR":
		return fmt.Sprintf("₹%.2f", v)
	case "PLN":
		return fmt.Sprintf("%.2f zł", v)
	case "USD":
		return fmt.Sprintf("US$%.2f", v)
	case "CAD":
		return fmt.Sprintf("CA$%.2f", v)
	case "AUD":
		return fmt.Sprintf("A$%.2f", v)
	case "SGD":
		return fmt.Sprintf("S$%.2f", v)
	case "":
		// 连账户子公司都推不出来:只给数字 + 明确提示,不冒充任何币种
		return fmt.Sprintf("%.2f(币种未知)", v)
	default:
		return fmt.Sprintf("%.2f %s", v, strings.ToUpper(currency))
	}
}

// 返回 (是否可下单, 失败原因)
func (m *Monitor) verifyPriceAvailable(planCode, datacenter string, configInfo map[string]interface{}) (bool, string) {
	options := optionsFromConfig(configInfo)
	accountID := accountIDFromConfig(configInfo)

	// 监控轮询验价: 直接进程内调用带有短 TTL 缓存的 price.GetInternalCached,
	// 避免 HTTP 回环网络开销与 429 抖动。
	result := price.GetInternalCached(m.state, accountID, planCode, datacenter, options)
	if !result.Success {
		errMsg := result.Error
		if errMsg == "" {
			errMsg = "未知错误"
		}
		m.state.Logger.Debug(fmt.Sprintf("价格校验失败: %s@%s - %s", planCode, datacenter, errMsg), "monitor")
		return false, errMsg
	}

	// degraded = 价格算出来了,但购物车没配成用户要的样子(dedicated_os / region 没设上)。
	// 同样的配置在 purchase.go 是 fail-fast、在 quick_order 是 400 拒绝入队,
	// 这里若只看 success 就会"发有货通知 + 触发自动下单",然后订单静默创建失败,
	// 用户只看到告警、拿不到机器。校验闸门必须跟下单闸门同口径。
	if result.Degraded {
		reason := result.DegradedReason
		if reason == "" {
			reason = "购物车必填配置未设置成功"
		}
		errMsg := "询价结果降级，无法下单：" + reason
		m.state.Logger.Debug(fmt.Sprintf("价格校验失败: %s@%s - %s", planCode, datacenter, errMsg), "monitor")
		return false, errMsg
	}

	if result.Price == nil {
		m.state.Logger.Debug(fmt.Sprintf("价格校验失败: %s@%s - price字段缺失", planCode, datacenter), "monitor")
		return false, "price字段缺失"
	}
	prices := result.Price.Prices
	if prices == nil {
		m.state.Logger.Debug(fmt.Sprintf("价格校验失败: %s@%s - prices字段缺失或类型错误", planCode, datacenter), "monitor")
		return false, "prices字段缺失或类型错误"
	}
	withTax := prices["withTax"]
	if withTax == nil {
		errMsg := "withTax无效(<nil>)"
		m.state.Logger.Debug(fmt.Sprintf("价格校验失败: %s@%s - %s", planCode, datacenter, errMsg), "monitor")
		return false, errMsg
	}
	if v, ok := numconv.ToFloat64(withTax); ok {
		if v == 0 {
			m.state.Logger.Debug(fmt.Sprintf("价格校验失败: %s@%s - withTax无效(0)", planCode, datacenter), "monitor")
			return false, "withTax无效(0)"
		}
	}
	m.state.Logger.Debug(fmt.Sprintf("价格校验通过: %s@%s - 含税价格: %v", planCode, datacenter, withTax), "monitor")
	return true, ""
}

// PlanPriceInfo 结构化价格信息
type PlanPriceInfo struct {
	MonthlyText    string
	InstallText    string
	FirstMonthText string
	Partial        bool
}

// resolvePlanPriceInfo 优先从公开目录获取纯月费、安装费与首月总计（带2小时缓存，毫秒级返回，不占账户配额）
func (m *Monitor) resolvePlanPriceInfo(planCode, accountID string, options []string) PlanPriceInfo {
	p, err := catalog.PriceForOptions(m.state, accountID, planCode, options)
	if err != nil {
		return PlanPriceInfo{}
	}
	info := PlanPriceInfo{Partial: p.Partial}
	if p.Monthly > 0 {
		info.MonthlyText = formatMoney(p.Currency, p.Monthly)
	}
	if p.Install > 0 {
		text := formatAmount(p.Currency, p.Install)
		if p.Partial {
			text += "（部分 addon 未计入，实际可能更高）"
		}
		info.InstallText = text
		if p.Monthly > 0 {
			info.FirstMonthText = formatAmount(p.Currency, p.Monthly+p.Install)
		}
	}
	return info
}

func (m *Monitor) GetPriceInfoText(planCode, datacenter string, configInfo map[string]interface{}) string {
	options := optionsFromConfig(configInfo)
	accountID := accountIDFromConfig(configInfo)

	// 1. 优先查公开目录中的纯月费:
	// 补货通知要展示的是真实的「月付续费」价格。OVH 购物车接口 (/order/cart/summary)
	// 回的是整张首月订单的含税总额，在有安装费时会把一次性安装费打包进 withTax，
	// 若直接当月费展示会把首月总计误标为“/月”，且造成重复计费误解。
	// 公开目录已带 2 小时缓存，按 capacity 严格分离了 monthly 和 installation，毫秒级响应且不耗配额。
	if p, err := catalog.PriceForOptions(m.state, accountID, planCode, options); err == nil && p.Monthly > 0 {
		text := formatMoney(p.Currency, p.Monthly)
		m.state.Logger.Debug("目录纯月费获取成功: "+text, "monitor")
		return text
	}

	m.state.Logger.Debug(fmt.Sprintf("开始获取价格(回退购物车询价): plan_code=%s, datacenter=%s, options=%v",
		planCode, datacenter, options), "monitor")

	result := price.GetInternalCached(m.state, accountID, planCode, datacenter, options)
	if !result.Success {
		m.state.Logger.Warn("价格获取失败: "+result.Error, "monitor")
		return ""
	}
	if result.Price == nil || result.Price.Prices == nil {
		return ""
	}
	prices := result.Price.Prices
	withTaxRaw, ok := prices["withTax"]
	if !ok || withTaxRaw == nil {
		m.state.Logger.Warn("价格获取成功但withTax为None", "monitor")
		return ""
	}
	subsidiary := m.subsidiaryOfPricingAccount(accountID)
	currency, _ := prices["currencyCode"].(string)
	switch {
	case currency == "":
		// OVH 没回币种时按询价账户的子公司兜底,而不是一律当欧元:
		// 同一条通知在美区账户下是 USD、加区账户下是 CAD,印成 "€" 会误导下单决策。
		currency = currencyForSubsidiary(subsidiary)
		m.state.Logger.Debug(fmt.Sprintf("价格接口未返回 currencyCode,按账户(子公司 %s)兜底为 %q",
			subsidiary, currency), "monitor")
	default:
		// 币种是"这次询价到底落在哪个大区"的免费探针:用美区账户询价却回了 EUR,
		// 说明询价账户和查库存的账户不是同一个(或账户 zone 配错了)。
		// 这种错配的表现只是"通知里的价格偏低/偏高",不报错,不查一下根本发现不了。
		if want := currencyForSubsidiary(subsidiary); want != "" && !strings.EqualFold(want, currency) {
			m.state.Logger.Warn(fmt.Sprintf(
				"询价币种与账户子公司不符: %s@%s 返回 %s,而询价账户(子公司 %s)应为 %s —— 请检查账户 zone / endpoint 配置",
				planCode, datacenter, strings.ToUpper(currency), subsidiary, want), "monitor")
		}
	}
	if v, ok := numconv.ToFloat64(withTaxRaw); ok {
		// 若能拿到目录中的安装费，且购物车总额包含了安装费，则月费应扣除安装费（避免安装费重复计算）
		if p, err := catalog.PriceForOptions(m.state, accountID, planCode, options); err == nil && p.Install > 0 && v > p.Install {
			v -= p.Install
		}
		text := formatMoney(currency, v)
		m.state.Logger.Debug("价格获取成功: "+text, "monitor")
		return text
	}
	return ""
}

// getPriceWithTimeout 带 30 秒超时的询价
func (m *Monitor) getPriceWithTimeout(planCode, datacenter string, configInfo map[string]interface{}, timeout time.Duration) (string, string) {
	type res struct {
		text   string
		errMsg string
	}
	ch := make(chan res, 1)
	start := time.Now()
	go func() {
		text := m.GetPriceInfoText(planCode, datacenter, configInfo)
		ch <- res{text: text}
	}()
	select {
	case r := <-ch:
		if r.text == "" {
			elapsed := time.Since(start).Seconds()
			return "", fmt.Sprintf("价格接口未返回结果（耗时%.1f秒）", elapsed)
		}
		return r.text, ""
	case <-time.After(timeout):
		elapsed := time.Since(start).Seconds()
		errMsg := fmt.Sprintf("价格接口超时（等待%.1f秒）", elapsed)
		m.state.Logger.Warn("价格获取超时，发送不带价格的通知。后台请求将继续运行直到完成。", "monitor")
		return "", errMsg
	}
}

// installPriceText 这套配置的一次性安装费。拿不到就返回空串 —— 通知里那一行直接不出现。
//
// 用目录算而不是询价:询价要建购物车 → 加商品 → 拿 summary → 删车,一次好几秒,
// 而补货通知的全部价值就是"有货那一刻立刻发出去"。目录有 2 小时缓存,也不占账户配额。
func (m *Monitor) installPriceText(planCode, accountID string, options []string) string {
	info := m.resolvePlanPriceInfo(planCode, accountID, options)
	return info.InstallText
}
