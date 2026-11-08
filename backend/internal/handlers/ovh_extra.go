package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/purchase"
)

// CleanupCarts POST /api/ovh/carts/cleanup
// 手动触发一次遗留购物车清理(后台本来每 6 小时跑一次)。只删带本程序标记、本进程没在用的车。
func CleanupCarts(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
		defer cancel()
		results := purchase.CleanupLeftoverCarts(ctx, state)
		deleted := 0
		for _, r := range results {
			deleted += r.Deleted
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "deleted": deleted, "accounts": results})
	}
}

// DefaultPaymentInfo 账户默认支付方式的检查结果
type DefaultPaymentInfo struct {
	HasDefault bool   `json:"hasDefault"`
	Valid      bool   `json:"valid"`
	Type       string `json:"type,omitempty"`
	Label      string `json:"label,omitempty"`
	Status     string `json:"status,omitempty"`
	Expiration string `json:"expiration,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

var defaultPaymentCache = struct {
	sync.Mutex
	m map[string]struct {
		at   time.Time
		info DefaultPaymentInfo
	}
}{m: map[string]struct {
	at   time.Time
	info DefaultPaymentInfo
}{}}

// GetDefaultPayment GET /api/ovh/payment-default?account_id=
// "自动付款"靠的是 OVH 账户的默认支付方式(checkout 的 autoPayWithPreferredPaymentMethod)。
// 没有默认支付方式、或它已失效/过期时,开了自动付款也只会建出一张未付款订单 ——
// 促销单 15 分钟不付就作废。开启自动付款时前端用它提前提示。结果缓存 10 分钟。
func GetDefaultPayment(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		accountID := strings.TrimSpace(c.Query("account_id"))
		account, ok := state.FindAccount(accountID)
		if !ok || accountID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少有效账户"})
			return
		}
		defaultPaymentCache.Lock()
		if e, hit := defaultPaymentCache.m[account.ID]; hit && time.Since(e.at) < 10*time.Minute && c.Query("refresh") == "" {
			defaultPaymentCache.Unlock()
			c.JSON(http.StatusOK, e.info)
			return
		}
		defaultPaymentCache.Unlock()
		ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
		defer cancel()
		info, err := fetchDefaultPayment(ctx, state, account.ID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "无法读取默认支付方式: " + ovh.Explain(err)})
			return
		}
		defaultPaymentCache.Lock()
		defaultPaymentCache.m[account.ID] = struct {
			at   time.Time
			info DefaultPaymentInfo
		}{time.Now(), info}
		defaultPaymentCache.Unlock()
		c.JSON(http.StatusOK, info)
	}
}

func fetchDefaultPayment(ctx context.Context, state *app.State, accountID string) (DefaultPaymentInfo, error) {
	client, err := state.OVH.IsolatedClientFor(accountID)
	if err != nil {
		return DefaultPaymentInfo{}, err
	}
	var ids []int64
	if err := client.GetWithContext(ctx, "/me/payment/method?default=true", &ids); err != nil {
		return DefaultPaymentInfo{}, err
	}
	if len(ids) == 0 {
		return DefaultPaymentInfo{Reason: "该账户没有设置默认支付方式"}, nil
	}
	var m registeredPaymentMethod
	if err := client.GetWithContext(ctx, fmt.Sprintf("/me/payment/method/%d", ids[0]), &m); err != nil {
		return DefaultPaymentInfo{}, err
	}
	label := m.Label
	if label == "" {
		label = m.Type
	}
	info := DefaultPaymentInfo{HasDefault: true, Type: m.Type, Label: label, Status: m.Status, Expiration: m.ExpirationDate}
	switch {
	case m.Status != "VALID":
		info.Reason = "默认支付方式状态为 " + m.Status + ",无法自动扣款"
	case m.ExpirationDate != "":
		if exp, ok := parseOVHTime(m.ExpirationDate); !ok || !exp.After(time.Now()) {
			info.Reason = "默认支付方式已过期"
		} else {
			info.Valid = true
		}
	default:
		info.Valid = true
	}
	return info, nil
}
