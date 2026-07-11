package monitor

import (
	"fmt"
	"time"

	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/price"
)

// resolvePriceAccount 询价账户：优先订阅 auto-order 账户，否则默认账户。
func (m *Monitor) resolvePriceAccount(sub *Subscription) string {
	if sub != nil && sub.AutoOrderAccountID != "" {
		return sub.AutoOrderAccountID
	}
	acc, ok := m.state.FindAccount("")
	if ok {
		return acc.ID
	}
	return ""
}

func optionsFromConfig(configInfo map[string]interface{}) []string {
	options := []string{}
	if configInfo == nil {
		return options
	}
	if opts, ok := configInfo["options"].([]string); ok {
		return append(options, opts...)
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

// verifyPriceAvailable 进程内询价（不再 HTTP 自调 /api/internal/monitor/price）。
// accountID 为空时用默认账户。
func (m *Monitor) verifyPriceAvailable(accountID, planCode, datacenter string, configInfo map[string]interface{}) (bool, string) {
	options := optionsFromConfig(configInfo)
	result := price.GetInternal(m.state, accountID, planCode, datacenter, options)
	if !result.Success {
		errMsg := result.Error
		if errMsg == "" {
			errMsg = "未知错误"
		}
		m.state.Logger.Debug(fmt.Sprintf("价格校验失败: %s@%s - %s", planCode, datacenter, errMsg), "monitor")
		return false, errMsg
	}
	if result.Price == nil {
		m.state.Logger.Debug(fmt.Sprintf("价格校验失败: %s@%s - price字段缺失", planCode, datacenter), "monitor")
		return false, "price字段缺失"
	}
	withTax := result.Price.Prices["withTax"]
	if withTax == nil {
		errMsg := "withTax无效(<nil>)"
		m.state.Logger.Debug(fmt.Sprintf("价格校验失败: %s@%s - %s", planCode, datacenter, errMsg), "monitor")
		return false, errMsg
	}
	if v, ok := numconv.ToFloat64(withTax); ok && v == 0 {
		m.state.Logger.Debug(fmt.Sprintf("价格校验失败: %s@%s - withTax无效(0)", planCode, datacenter), "monitor")
		return false, "withTax无效(0)"
	}
	m.state.Logger.Debug(fmt.Sprintf("价格校验通过: %s@%s - 含税价格: %v", planCode, datacenter, withTax), "monitor")
	return true, ""
}

// GetPriceInfoText 进程内询价并格式化为通知文案
func (m *Monitor) GetPriceInfoText(accountID, planCode, datacenter string, configInfo map[string]interface{}) string {
	options := optionsFromConfig(configInfo)
	m.state.Logger.Debug(fmt.Sprintf("开始获取价格: plan_code=%s, datacenter=%s, options=%v account=%s",
		planCode, datacenter, options, accountID), "monitor")

	result := price.GetInternal(m.state, accountID, planCode, datacenter, options)
	if !result.Success {
		m.state.Logger.Warn("价格获取失败: "+result.Error, "monitor")
		return ""
	}
	if result.Price == nil || result.Price.Prices == nil {
		return ""
	}
	withTaxRaw := result.Price.Prices["withTax"]
	if withTaxRaw == nil {
		m.state.Logger.Warn("价格获取成功但withTax为None", "monitor")
		return ""
	}
	currency, _ := result.Price.Prices["currencyCode"].(string)
	if currency == "" {
		currency = "EUR"
	}
	sym := currency
	switch currency {
	case "EUR":
		sym = "€"
	case "USD":
		sym = "$"
	}
	if v, ok := numconv.ToFloat64(withTaxRaw); ok {
		text := fmt.Sprintf("%s%.2f/月", sym, v)
		m.state.Logger.Debug("价格获取成功: "+text, "monitor")
		return text
	}
	return ""
}

// getPriceWithTimeout 带超时的询价
func (m *Monitor) getPriceWithTimeout(accountID, planCode, datacenter string, configInfo map[string]interface{}, timeout time.Duration) (string, string) {
	type res struct {
		text string
	}
	ch := make(chan res, 1)
	start := time.Now()
	go func() {
		text := m.GetPriceInfoText(accountID, planCode, datacenter, configInfo)
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
