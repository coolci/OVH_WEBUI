package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ovh-webui/server/internal/numconv"
)

// verifyPriceAvailable 对应 Python: _verify_price_available
// 返回 (是否可下单, 失败原因)
func (m *Monitor) verifyPriceAvailable(planCode, datacenter string, configInfo map[string]interface{}) (bool, string) {
	options := []string{}
	if configInfo != nil {
		if opts, ok := configInfo["options"].([]string); ok {
			options = opts
		} else if optsRaw, ok := configInfo["options"].([]interface{}); ok {
			for _, o := range optsRaw {
				if s, ok := o.(string); ok {
					options = append(options, s)
				}
			}
		}
	}

	url := "http://127.0.0.1:" + m.state.Port + "/api/internal/monitor/price"
	body, _ := json.Marshal(map[string]interface{}{
		"plan_code":  planCode,
		"datacenter": datacenter,
		"options":    options,
	})
	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		errMsg := "价格校验API请求失败: " + err.Error()
		m.state.Logger.Debug(fmt.Sprintf("价格校验API请求失败: %s@%s - %s", planCode, datacenter, err.Error()), "monitor")
		return false, errMsg
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return false, "价格校验API响应解析失败"
	}

	success, _ := result["success"].(bool)
	if !success {
		errMsg, _ := result["error"].(string)
		if errMsg == "" {
			errMsg = "未知错误"
		}
		m.state.Logger.Debug(fmt.Sprintf("价格校验失败: %s@%s - %s", planCode, datacenter, errMsg), "monitor")
		return false, errMsg
	}

	priceRaw, ok := result["price"]
	if !ok || priceRaw == nil {
		m.state.Logger.Debug(fmt.Sprintf("价格校验失败: %s@%s - price字段缺失", planCode, datacenter), "monitor")
		return false, "price字段缺失"
	}
	priceInfo, ok := priceRaw.(map[string]interface{})
	if !ok {
		m.state.Logger.Debug(fmt.Sprintf("价格校验失败: %s@%s - price字段类型错误", planCode, datacenter), "monitor")
		return false, "price字段类型错误"
	}
	prices, ok := priceInfo["prices"].(map[string]interface{})
	if !ok {
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

// GetPriceInfoText 对应 Python: _get_price_info
func (m *Monitor) GetPriceInfoText(planCode, datacenter string, configInfo map[string]interface{}) string {
	options := []string{}
	if configInfo != nil {
		if opts, ok := configInfo["options"].([]string); ok {
			options = opts
		} else if optsRaw, ok := configInfo["options"].([]interface{}); ok {
			for _, o := range optsRaw {
				if s, ok := o.(string); ok {
					options = append(options, s)
				}
			}
		}
	}

	m.state.Logger.Debug(fmt.Sprintf("开始获取价格: plan_code=%s, datacenter=%s, options=%v",
		planCode, datacenter, options), "monitor")

	url := "http://127.0.0.1:" + m.state.Port + "/api/internal/monitor/price"
	body, _ := json.Marshal(map[string]interface{}{
		"plan_code":  planCode,
		"datacenter": datacenter,
		"options":    options,
	})
	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		m.state.Logger.Warn("价格API请求失败: "+err.Error(), "monitor")
		return ""
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return ""
	}
	if ok, _ := result["success"].(bool); !ok {
		errMsg, _ := result["error"].(string)
		m.state.Logger.Warn("价格获取失败: "+errMsg, "monitor")
		return ""
	}
	priceInfo, _ := result["price"].(map[string]interface{})
	if priceInfo == nil {
		return ""
	}
	prices, _ := priceInfo["prices"].(map[string]interface{})
	if prices == nil {
		return ""
	}
	withTaxRaw, ok := prices["withTax"]
	if !ok || withTaxRaw == nil {
		m.state.Logger.Warn("价格获取成功但withTax为None", "monitor")
		return ""
	}
	currency, _ := prices["currencyCode"].(string)
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

// getPriceWithTimeout 模拟 Python 中 30 秒超时
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
