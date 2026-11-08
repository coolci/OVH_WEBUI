package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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

// OrderFollowUpStep GET /me/order/{orderId}/followUp 的一项(billing.order.FollowUp)
type OrderFollowUpStep struct {
	Step    string `json:"step"`   // VALIDATING / VALIDATED / DELIVERING / AVAILABLE
	Status  string `json:"status"` // TODO / DOING / DONE / ERROR
	History []struct {
		Date        string `json:"date"`
		Label       string `json:"label"` // ORDER_ACCEPTED / PAYMENT_RECEIVED / FRAUD_DOCS_REQUESTED / DELIVERY ...
		Description string `json:"description"`
	} `json:"history"`
}

// GetOrderFollowUp GET /api/ovh/orders/:orderId/followup?account_id=
// 订单付款后的进度(OVH 的 followUp,BETA)+ 订单明细里的 domain(交付后就是服务器/VPS 的 serviceName),
// 购买历史据此显示"验证中 / 交付中 / 已可用",交付后一键跳到服务器控制。
// 注意:/me/order/{id}/associatedObject 返回的是账单/退款对象,不是服务器,所以不用它。
func GetOrderFollowUp(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		accountID := strings.TrimSpace(c.Query("account_id"))
		orderID := strings.TrimSpace(c.Param("orderId"))
		if _, ok := state.FindAccount(accountID); !ok || accountID == "" || orderID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少有效账户或订单号"})
			return
		}
		for _, ch := range orderID {
			if ch < '0' || ch > '9' {
				c.JSON(http.StatusBadRequest, gin.H{"error": "订单号格式不正确"})
				return
			}
		}
		client, err := state.OVH.IsolatedClientFor(accountID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		var steps []OrderFollowUpStep
		followErr := client.GetWithContext(ctx, "/me/order/"+orderID+"/followUp", &steps)
		// 交付出来的服务名:明细的 domain 字段(付款前多为 "*" 或空)
		var services []string
		var detailIDs []int64
		if err := client.GetWithContext(ctx, "/me/order/"+orderID+"/details", &detailIDs); err == nil {
			seen := map[string]bool{}
			for _, id := range detailIDs {
				var detail struct {
					Domain string `json:"domain"`
				}
				if err := client.GetWithContext(ctx, fmt.Sprintf("/me/order/%s/details/%d", orderID, id), &detail); err != nil {
					continue
				}
				d := strings.TrimSpace(detail.Domain)
				if d != "" && d != "*" && !seen[d] {
					seen[d] = true
					services = append(services, d)
				}
			}
		}
		if followErr != nil && len(services) == 0 {
			c.JSON(http.StatusBadGateway, gin.H{"error": "无法读取订单进度: " + ovh.Explain(followErr)})
			return
		}
		if steps == nil {
			steps = []OrderFollowUpStep{}
		}
		if services == nil {
			services = []string{}
		}
		c.JSON(http.StatusOK, gin.H{"steps": steps, "services": services})
	}
}

var osAvailabilityCache = struct {
	sync.Mutex
	m map[string]struct {
		at  time.Time
		oss []string
	}
}{m: map[string]struct {
	at  time.Time
	oss []string
}{}}

// GetOSAvailability GET /api/ovh/os-availability?planCode=&account_id=
// 某个独服型号能装哪些系统(GET /dedicated/server/osAvailabilities?hardware=,免登录,hardware 取型号代码)。
// 按账户所在站点(EU/CA/US)查,结果缓存 1 小时。
func GetOSAvailability(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		plan := strings.TrimSpace(c.Query("planCode"))
		if plan == "" || strings.ContainsAny(plan, " /?&#") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少有效型号"})
			return
		}
		accountID := strings.TrimSpace(c.Query("account_id"))
		if accountID == "" {
			if acc, ok := state.FindAccount(""); ok {
				accountID = acc.ID
			}
		}
		account, ok := state.FindAccount(accountID)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少有效账户"})
			return
		}
		key := account.Endpoint + "|" + plan
		osAvailabilityCache.Lock()
		if e, hit := osAvailabilityCache.m[key]; hit && time.Since(e.at) < time.Hour {
			osAvailabilityCache.Unlock()
			c.JSON(http.StatusOK, gin.H{"os": e.oss})
			return
		}
		osAvailabilityCache.Unlock()
		client, err := state.OVH.IsolatedClientFor(account.ID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
		defer cancel()
		var oss []string
		if err := client.GetUnAuthWithContext(ctx, "/dedicated/server/osAvailabilities?hardware="+url.QueryEscape(plan), &oss); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "无法读取可安装系统: " + ovh.Explain(err)})
			return
		}
		if oss == nil {
			oss = []string{}
		}
		osAvailabilityCache.Lock()
		if len(osAvailabilityCache.m) > 512 {
			osAvailabilityCache.m = map[string]struct {
				at  time.Time
				oss []string
			}{}
		}
		osAvailabilityCache.m[key] = struct {
			at  time.Time
			oss []string
		}{time.Now(), oss}
		osAvailabilityCache.Unlock()
		c.JSON(http.StatusOK, gin.H{"os": oss})
	}
}
