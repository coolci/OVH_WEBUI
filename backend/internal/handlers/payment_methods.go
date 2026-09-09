package handlers

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/ovh"
)

// GetPaymentMethods GET /api/ovh/payment-methods
// 获取账户已绑定的有效支付方式（信用卡/PayPal/账户余额等）
func GetPaymentMethods(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		var methodIDs []interface{}
		if err := client.Get("/me/payment/method", &methodIDs); err != nil {
			state.Logger.Error("获取支付方式列表失败: "+err.Error(), "payment")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "获取支付方式失败: " + err.Error()})
			return
		}

		if len(methodIDs) == 0 {
			c.JSON(http.StatusOK, gin.H{"success": true, "methods": []map[string]interface{}{}})
			return
		}

		details := parallelGetDetails(client, methodIDs, func(k interface{}) string {
			return fmt.Sprintf("/me/payment/method/%v", k)
		}, 10)

		var list []map[string]interface{}
		for _, d := range details {
			if d != nil {
				list = append(list, d)
			}
		}

		// 排序：默认支付方式置顶
		sort.Slice(list, func(i, j int) bool {
			defI, _ := list[i]["default"].(bool)
			defJ, _ := list[j]["default"].(bool)
			if defI != defJ {
				return defI
			}
			idI, _ := numconv.ToInt64(list[i]["paymentMethodId"])
			idJ, _ := numconv.ToInt64(list[j]["paymentMethodId"])
			return idI > idJ
		})

		c.JSON(http.StatusOK, gin.H{"success": true, "methods": list})
	}
}

// PayOrder POST /api/order/:orderId/pay
// 对未支付订单执行一键扣款（自动匹配用户指定或账户默认支付方式）
func PayOrder(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		orderID := c.Param("orderId")
		if orderID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 orderId 参数"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		var body struct {
			PaymentMethodID *int64 `json:"paymentMethodId,omitempty"`
		}
		_ = c.ShouldBindJSON(&body)

		payRes, payErr := ovh.PayOrder(client, orderID, body.PaymentMethodID)
		if payErr != nil {
			state.Logger.Error(fmt.Sprintf("订单 %s 扣款失败: %s", orderID, payErr.Error()), "payment")
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   fmt.Sprintf("支付失败: %s", payErr.Error()),
			})
			return
		}

		state.Logger.Info(fmt.Sprintf("订单 %s 扣款指令已成功提交", orderID), "payment")
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "支付指令已提交",
			"result":  payRes,
		})
	}
}
