package handlers

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
	ovhsdk "github.com/ovh/go-ovh/ovh"
)

type historyOrderView struct {
	types.PurchaseHistoryEntry
	db.OrderCancellation
}

type cancellationInfo struct {
	AccountID      string   `json:"accountId"`
	OrderID        int64    `json:"orderId"`
	Status         string   `json:"status"`
	Mode           string   `json:"mode"`
	Message        string   `json:"message"`
	CanRestore     bool     `json:"canRestore"`
	Amount         *float64 `json:"amount"`
	Currency       string   `json:"currency"`
	RetractionDate string   `json:"retractionDate"`
	ExpirationDate string   `json:"expirationDate"`
	Reasons        []gin.H  `json:"reasons"`
}

// 历史行 ID 和账户必须同时匹配，绝不回退到默认账户。
func historyCancellationTarget(state *app.State, c *gin.Context) (types.PurchaseHistoryEntry, int64, bool) {
	accountID := strings.TrimSpace(c.Query("account"))
	for _, h := range state.HistorySnapshot() {
		if h.ID != c.Param("id") || accountID == "" || h.AccountID != accountID {
			continue
		}
		id, err := strconv.ParseInt(h.OrderID, 10, 64)
		if h.Status == "success" && err == nil && id > 0 && state.HasDB() {
			if _, ok := state.FindAccount(accountID); ok {
				return h, id, true
			}
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "未找到该账户的成功订单"})
	return types.PurchaseHistoryEntry{}, 0, false
}

func loadCancellation(ctx context.Context, state *app.State, h types.PurchaseHistoryEntry, id int64) (cancellationInfo, error) {
	info := cancellationInfo{AccountID: h.AccountID, OrderID: id, Mode: "unavailable", Reasons: retractionReasons}
	client, err := state.OVH.IsolatedClientFor(h.AccountID)
	if err != nil {
		return info, err
	}
	client.Timeout = 15 * time.Second
	path, err := buildOVHPath("me", "order", strconv.FormatInt(id, 10))
	if err != nil {
		return info, err
	}
	var order struct {
		OrderID        int64  `json:"orderId"`
		ExpirationDate string `json:"expirationDate"`
		RetractionDate string `json:"retractionDate"`
		Price          struct {
			Value    *float64 `json:"value"`
			Currency string   `json:"currencyCode"`
		} `json:"priceWithTax"`
	}
	if err := client.GetWithContext(ctx, path, &order); err != nil {
		return info, err
	}
	if order.OrderID != id {
		return info, fmt.Errorf("OVH 返回的订单号不一致")
	}
	statusPath, err := buildOVHPath("me", "order", strconv.FormatInt(id, 10), "status")
	if err != nil {
		return info, err
	}
	if err := client.GetWithContext(ctx, statusPath, &info.Status); err != nil {
		return info, err
	}
	info.Amount, info.Currency = order.Price.Value, order.Price.Currency
	info.ExpirationDate, info.RetractionDate = order.ExpirationDate, order.RetractionDate
	action, err := state.DB.GetOrderCancellation(h.AccountID, id)
	if err != nil {
		return info, err
	}
	if info.Status == "cancelled" || info.Status == "cancelling" {
		info.Message = "OVH 已取消订单或正在取消中。"
		return info, nil
	}
	if action.RetractionState != "" && action.RetractionState != "failed" {
		info.Message = "撤单请求已登记，请先在 OVH 核对结果，不要重复提交。"
		return info, nil
	}
	if info.Status == "notPaid" {
		var payment struct {
			State string `json:"state"`
		}
		attempted, err := state.GetKV(db.OrderPaymentKey(h.AccountID, id), &payment)
		if err != nil {
			return info, err
		}
		if attempted {
			info.Message = "已有付款请求，请先在 OVH 确认付款结果。"
			return info, nil
		}
		expiry, valid := parseOVHTime(order.ExpirationDate)
		if order.ExpirationDate != "" && (!valid || !expiry.After(time.Now())) {
			info.Message = "订单付款期限已结束或无法确认，请在 OVH 核对。"
			return info, nil
		}
		if action.AbandonedAt != "" {
			info.CanRestore = valid && expiry.After(time.Now())
			info.Message = "已放弃此订单，等待 OVH 到期作废。付款期限内可以撤销放弃。"
		} else {
			info.Mode = "abandon"
			info.Message = "OVH 不支持直接取消未付款订单。此操作仅在本系统放弃订单并阻止付款，订单由 OVH 到期作废；不会向 OVH 提交撤单，也不会自动重新抢购。"
		}
		return info, nil
	}
	acc, _ := state.FindAccount(h.AccountID)
	if !retractionSupportedRegion(acc.Endpoint) {
		info.Message = "美区 API 不支持撤单退款，请联系 OVH 客服。"
		return info, nil
	}
	switch info.Status {
	case "checking", "delivering", "delivered", "documentsRequested":
	default:
		info.Message = "订单状态未知，无法确认可撤单，请先到 OVH 核对。"
		return info, nil
	}
	deadline, valid := parseOVHTime(order.RetractionDate)
	if !valid || !deadline.After(time.Now()) {
		info.Message = "OVH 未提供有效撤回期限，或期限已过，当前无法申请撤单。"
		return info, nil
	}
	if info.Amount == nil || math.IsNaN(*info.Amount) || math.IsInf(*info.Amount, 0) || *info.Amount < 0 || info.Currency == "" {
		return info, fmt.Errorf("无法确认订单含税金额或币种")
	}
	info.Mode = "retract"
	info.Message = "将向 OVH 申请撤回整张订单，影响订单内所有产品及其数据（含备份）。退款由 OVH 按规则原路处理，请备份数据并核对订单。"
	return info, nil
}

func GetHistoryCancellation(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		h, id, ok := historyCancellationTarget(state, c)
		if !ok {
			return
		}
		info, err := loadCancellation(c.Request.Context(), state, h, id)
		if err != nil {
			c.JSON(502, gin.H{"error": "读取撤单信息失败: " + ovh.Explain(err)})
			return
		}
		c.JSON(200, info)
	}
}

func CancelHistoryOrder(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		h, id, ok := historyCancellationTarget(state, c)
		if !ok {
			return
		}
		var body struct {
			Confirm  bool     `json:"confirm"`
			Mode     string   `json:"mode"`
			Reason   string   `json:"reason"`
			Comment  string   `json:"comment"`
			Amount   *float64 `json:"amount"`
			Currency string   `json:"currency"`
		}
		if c.ShouldBindJSON(&body) != nil || !body.Confirm {
			c.JSON(400, gin.H{"error": "请确认订单操作"})
			return
		}
		info, err := loadCancellation(c.Request.Context(), state, h, id)
		if err != nil {
			c.JSON(502, gin.H{"error": "撤单前核对失败: " + ovh.Explain(err)})
			return
		}
		operation := body.Mode
		if operation == "restore" {
			if !info.CanRestore {
				c.JSON(409, gin.H{"error": info.Message})
				return
			}
		} else if operation != info.Mode || (operation != "abandon" && operation != "retract") {
			c.JSON(409, gin.H{"error": "订单状态已变化，请重新打开撤单窗口。" + info.Message})
			return
		}
		if operation == "retract" {
			if !isValidRetractionReason(body.Reason) || len([]rune(body.Comment)) > 1000 {
				c.JSON(400, gin.H{"error": "请选择撤单原因，说明最多 1000 字"})
				return
			}
			if body.Amount == nil || info.Amount == nil || *body.Amount != *info.Amount || body.Currency != info.Currency {
				c.JSON(409, gin.H{"error": "订单金额或币种已变化，请重新打开撤单窗口确认"})
				return
			}
		}
		if err := state.DB.ChangeOrderCancellation(h.AccountID, id, operation); err != nil {
			code := 500
			if errors.Is(err, db.ErrOrderActionConflict) {
				code = 409
			}
			c.JSON(code, gin.H{"error": "订单操作未提交: " + err.Error()})
			return
		}
		if operation != "retract" {
			message := "已放弃订单，等待 OVH 到期作废"
			if operation == "restore" {
				message = "已撤销放弃，可以继续手动付款"
			}
			state.Logger.Info(fmt.Sprintf("账户 %s 订单 %d: %s", h.AccountID, id, operation), "order_cancellation")
			c.JSON(200, gin.H{"message": message})
			return
		}
		client, err := state.OVH.IsolatedClientFor(h.AccountID)
		if err != nil {
			_ = state.DB.SetOrderRetractionState(h.AccountID, id, "failed")
			c.JSON(500, gin.H{"error": "账户不可用，撤单未发送"})
			return
		}
		client.Timeout = 45 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		payload := map[string]interface{}{"reason": body.Reason}
		if comment := strings.TrimSpace(body.Comment); comment != "" {
			payload["comment"] = comment
		}
		retractPath, pathErr := buildOVHPath("me", "order", strconv.FormatInt(id, 10), "retraction")
		if pathErr != nil {
			c.JSON(400, gin.H{"error": pathErr.Error()})
			return
		}
		err = client.PostWithContext(ctx, retractPath, payload, nil)
		status := "submitted"
		if err != nil {
			status = "uncertain"
			var apiErr *ovhsdk.APIError
			if errors.As(err, &apiErr) && apiErr.Code >= 400 && apiErr.Code < 500 && apiErr.Code != 408 && apiErr.Code != 409 && apiErr.Code != 429 {
				status = "failed"
			}
		}
		if saveErr := state.DB.SetOrderRetractionState(h.AccountID, id, status); saveErr != nil {
			c.JSON(500, gin.H{"error": "撤单结果未能保存，请先在 OVH 核对，不要重复提交"})
			return
		}
		state.Logger.Info(fmt.Sprintf("账户 %s 订单 %d 撤单: %s", h.AccountID, id, status), "order_cancellation")
		if err != nil {
			message := "撤单结果不确定，请先到 OVH 核对，不要重复提交。"
			if status == "failed" {
				message = "OVH 拒绝了撤单申请: " + ovh.Explain(err)
			}
			c.JSON(502, gin.H{"error": message})
			return
		}
		c.JSON(200, gin.H{"message": "撤单申请已提交，退款及最终结果以 OVH 为准"})
	}
}
