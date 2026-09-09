package ovh

import (
	"errors"
	"fmt"
	"strings"

	ovhsdk "github.com/ovh/go-ovh/ovh"
	"github.com/ovh-webui/server/internal/numconv"
)

// PaymentMethodInfo 支付方式元数据
type PaymentMethodInfo struct {
	ID            int64
	PaymentMeanID int64
	PaymentType   string
	Default       bool
}

// ResolvePaymentMethod 为指定订单自动解析最优扣款方式
func ResolvePaymentMethod(client *ovhsdk.Client, orderID string, preferredID *int64) (*PaymentMethodInfo, error) {
	if client == nil {
		return nil, errors.New("ovh client is nil")
	}

	// 1. 如果调用方明确指定了支付方式 ID
	if preferredID != nil && *preferredID > 0 {
		var detail map[string]interface{}
		if err := client.Get(fmt.Sprintf("/me/payment/method/%d", *preferredID), &detail); err == nil {
			meanID, _ := numconv.ToInt64(detail["paymentMeanId"])
			pType, _ := detail["paymentType"].(string)
			isDef, _ := detail["default"].(bool)
			return &PaymentMethodInfo{
				ID:            *preferredID,
				PaymentMeanID: meanID,
				PaymentType:   pType,
				Default:       isDef,
			}, nil
		}
		return &PaymentMethodInfo{ID: *preferredID}, nil
	}

	// 2. 查询该订单适用的支付方式：/me/order/{orderId}/paymentMethods
	if orderID != "" {
		var orderMethodIDs []interface{}
		if err := client.Get(fmt.Sprintf("/me/order/%s/paymentMethods", orderID), &orderMethodIDs); err == nil && len(orderMethodIDs) > 0 {
			var firstValid *PaymentMethodInfo
			for _, rawID := range orderMethodIDs {
				id, _ := numconv.ToInt64(rawID)
				if id <= 0 {
					continue
				}
				var detail map[string]interface{}
				if err := client.Get(fmt.Sprintf("/me/payment/method/%d", id), &detail); err == nil {
					isDef, _ := detail["default"].(bool)
					meanID, _ := numconv.ToInt64(detail["paymentMeanId"])
					pType, _ := detail["paymentType"].(string)
					info := &PaymentMethodInfo{
						ID:            id,
						PaymentMeanID: meanID,
						PaymentType:   pType,
						Default:       isDef,
					}
					if isDef {
						return info, nil
					}
					if firstValid == nil {
						firstValid = info
					}
				}
			}
			if firstValid != nil {
				return firstValid, nil
			}
			// 兜底直接用第一个 ID
			if fID, _ := numconv.ToInt64(orderMethodIDs[0]); fID > 0 {
				return &PaymentMethodInfo{ID: fID}, nil
			}
		}
	}

	// 3. 查询账户默认支付方式：/me/payment/method?default=true
	var defaultIDs []interface{}
	if err := client.Get("/me/payment/method?default=true", &defaultIDs); err == nil && len(defaultIDs) > 0 {
		for _, rawID := range defaultIDs {
			id, _ := numconv.ToInt64(rawID)
			if id > 0 {
				var detail map[string]interface{}
				_ = client.Get(fmt.Sprintf("/me/payment/method/%d", id), &detail)
				meanID, _ := numconv.ToInt64(detail["paymentMeanId"])
				pType, _ := detail["paymentType"].(string)
				return &PaymentMethodInfo{
					ID:            id,
					PaymentMeanID: meanID,
					PaymentType:   pType,
					Default:       true,
				}, nil
			}
		}
	}

	// 4. 从所有支付方式列表中取第一个有效支付方式
	var allIDs []interface{}
	if err := client.Get("/me/payment/method", &allIDs); err == nil && len(allIDs) > 0 {
		for _, rawID := range allIDs {
			id, _ := numconv.ToInt64(rawID)
			if id <= 0 {
				continue
			}
			var detail map[string]interface{}
			if err := client.Get(fmt.Sprintf("/me/payment/method/%d", id), &detail); err == nil {
				status, _ := detail["status"].(string)
				if status == "VALID" || status == "" {
					meanID, _ := numconv.ToInt64(detail["paymentMeanId"])
					pType, _ := detail["paymentType"].(string)
					return &PaymentMethodInfo{
						ID:            id,
						PaymentMeanID: meanID,
						PaymentType:   pType,
					}, nil
				}
			}
		}
	}

	return nil, errors.New("账户未绑定可用的支付方式（信用卡/PayPal），请先在 OVH 控制台绑定")
}

// PayOrder 对指定订单执行扣款指令（自动解析默认或指定支付方式）
func PayOrder(client *ovhsdk.Client, orderID string, preferredID *int64) (map[string]interface{}, error) {
	if orderID == "" {
		return nil, errors.New("缺少 orderId 参数")
	}

	// 1. 预检订单当前状态：若订单已付款/处于核验或交付状态，避免重复提交
	var currentStatus string
	if errStatus := client.Get(fmt.Sprintf("/me/order/%s/status", orderID), &currentStatus); errStatus == nil {
		currentStatus = strings.Trim(strings.TrimSpace(currentStatus), "\"")
		switch currentStatus {
		case "checking":
			return map[string]interface{}{"status": "checking", "alreadyPaid": true}, fmt.Errorf("订单 %s 付款已收到，OVH 正在核验审核中（checking），无需重复支付", orderID)
		case "delivering", "delivered":
			return map[string]interface{}{"status": currentStatus, "alreadyPaid": true}, fmt.Errorf("订单 %s 已完成支付并正在交付中（%s），无需重复支付", orderID, currentStatus)
		case "cancelled", "cancelling":
			return nil, fmt.Errorf("订单 %s 已作废或已取消（%s），无法再支付", orderID, currentStatus)
		}
	}

	methodInfo, err := ResolvePaymentMethod(client, orderID, preferredID)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"paymentMethod": map[string]interface{}{
			"id": methodInfo.ID,
		},
	}

	var payRes map[string]interface{}
	payErr := client.Post(fmt.Sprintf("/me/order/%s/pay", orderID), payload, &payRes)
	if payErr == nil {
		return payRes, nil
	}

	// 拦截 OVH 重复扣款/不可支付报错：提示友好信息
	if strings.Contains(payErr.Error(), "can't be paid with this payment method") {
		return nil, fmt.Errorf("该订单已被受理或已付款核验中，无需且无法重复扣款")
	}

	// 如果 /pay 失败，尝试降级到 /payWithRegisteredPaymentMean
	if methodInfo.PaymentType != "" {
		fallbackPayload := map[string]interface{}{
			"paymentMean": methodInfo.PaymentType,
		}
		if methodInfo.PaymentMeanID > 0 {
			fallbackPayload["paymentMeanId"] = methodInfo.PaymentMeanID
		}
		var fallbackRes map[string]interface{}
		if errFallback := client.Post(fmt.Sprintf("/me/order/%s/payWithRegisteredPaymentMean", orderID), fallbackPayload, &fallbackRes); errFallback == nil {
			return fallbackRes, nil
		}
	}

	return nil, payErr
}
