package handlers

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/orderterm"
	"github.com/ovh-webui/server/internal/ovh"
)

// 可选购买时长按 (账户, 型号) 缓存。每次查询都要在 OVH 建一辆临时购物车,
// 前端每打开一次下单框就查一次的话,很快就会把账户打进限流 —— 而限流的代价是
// 真正补货时下不了单。周期列表几乎不变,10 分钟足够新鲜。
const orderDurationsTTL = 10 * time.Minute

type durationsEntry struct {
	at        time.Time
	durations []string
}

var orderDurationsCache = struct {
	sync.Mutex
	m map[string]durationsEntry
}{m: map[string]durationsEntry{}}

// orderDurationsFetcher 可在测试里替换;生产实现读 OVH 购物车里的真实报价。
var orderDurationsFetcher = fetchOrderDurations

// GetOrderDurations GET /api/order/durations?account_id=&planCode=
// 返回 OVH 对该账户、该独服型号实际提供的购买时长(ISO 8601,如 P1M/P12M)。
// 只读:建临时购物车读报价后立即删除,从不加商品、从不结账。
func GetOrderDurations(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		accountID := strings.TrimSpace(c.Query("account_id"))
		if accountID == "" {
			accountID = strings.TrimSpace(c.Query("accountId"))
		}
		plan := strings.TrimSpace(c.Query("planCode"))
		account, ok := state.FindAccount(accountID)
		if !ok || accountID == "" || plan == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少有效账户或型号"})
			return
		}
		key := account.ID + "|" + account.Endpoint + "|" + strings.ToUpper(account.Zone) + "|" + plan
		orderDurationsCache.Lock()
		if e, hit := orderDurationsCache.m[key]; hit && time.Since(e.at) < orderDurationsTTL {
			orderDurationsCache.Unlock()
			c.JSON(http.StatusOK, gin.H{"success": true, "durations": e.durations, "default": orderterm.Default})
			return
		}
		orderDurationsCache.Unlock()

		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		durations, err := orderDurationsFetcher(ctx, state, account.ID, catalog.SubsidiaryOfAccount(account), plan)
		if err != nil {
			state.Logger.Warn("读取可选购买时长失败("+plan+"): "+err.Error(), "order")
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": "无法读取 OVH 可选购买时长，请稍后重试"})
			return
		}
		if durations == nil {
			durations = []string{}
		}
		orderDurationsCache.Lock()
		if len(orderDurationsCache.m) > 256 {
			orderDurationsCache.m = map[string]durationsEntry{}
		}
		orderDurationsCache.m[key] = durationsEntry{at: time.Now(), durations: durations}
		orderDurationsCache.Unlock()
		c.JSON(http.StatusOK, gin.H{"success": true, "durations": durations, "default": orderterm.Default})
	}
}

// fetchOrderDurations 用免登录购物车读 OVH 对这个型号的计价周期。
// 2026-11 在 EU 正式环境实测:建车、列型号计价都不需要登录,只有绑定账户/结账/删车需要。
// 只读周期不涉及税额(匿名车按子公司默认税率,与账户实际税率可能不同),所以这里可以放心匿名;
// 匿名车删不掉,OVH 约 24 小时后自动过期,且不挂在任何账户名下。
// 客户端仍取自该账户:沿用它的站点(EU/CA/US)和出站代理,只是请求不签名。
func fetchOrderDurations(ctx context.Context, state *app.State, accountID, subsidiary, plan string) ([]string, error) {
	client, err := state.OVH.IsolatedClientFor(accountID)
	if err != nil {
		return nil, err
	}
	var cart struct {
		ID string `json:"cartId"`
	}
	if err := client.PostUnAuthWithContext(ctx, "/order/cart", ovh.CartCreateBody(subsidiary), &cart); err != nil {
		return nil, err
	}
	if cart.ID == "" {
		return nil, context.DeadlineExceeded
	}
	var offers []map[string]interface{}
	if err := client.GetUnAuthWithContext(ctx, "/order/cart/"+cart.ID+"/eco", &offers); err != nil {
		return nil, err
	}
	for _, offer := range offers {
		if pc, _ := offer["planCode"].(string); pc == plan {
			return orderterm.Durations(offer["prices"]), nil
		}
	}
	return []string{}, nil
}
