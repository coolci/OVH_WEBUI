package purchase

import (
	"context"
	"fmt"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// 遗留购物车清理。
//
// 进程崩溃、强制退出、或删车请求本身失败时,本程序建的购物车会留在 OVH 账户下。
// 高频抢购累积的车会把账户推向 OVH 限流 —— 偏偏是在补货那一刻下不了单。
//
// 只删同时满足三个条件的车:
//  1. description 是本程序的标记(ovh.CartMarker)—— 用户在 OVH 网站上建的车、
//     加标记之前本程序建的老车都不动;
//  2. 本进程没有登记在用(下单中、预热池、结账结果待核对的车都已登记);
//  3. 不是只读车(已经结账转成订单的车 OVH 会标只读,本来也删不掉)。

// CartCleanupResult 一次清理的结果
type CartCleanupResult struct {
	AccountID string `json:"accountId"`
	Listed    int    `json:"listed"`
	Deleted   int    `json:"deleted"`
	Kept      int    `json:"kept"`
	Error     string `json:"error,omitempty"`
}

// CleanupLeftoverCarts 逐个账户清理本程序留下的购物车
func CleanupLeftoverCarts(ctx context.Context, state *app.State) []CartCleanupResult {
	var results []CartCleanupResult
	for _, acc := range state.AccountsSnapshot() {
		res := CartCleanupResult{AccountID: acc.ID}
		client, err := state.OVH.IsolatedClientFor(acc.ID)
		if err != nil {
			res.Error = err.Error()
			results = append(results, res)
			continue
		}
		var ids []string
		if err := client.GetWithContext(ctx, "/order/cart", &ids); err != nil {
			res.Error = ovh.Explain(err)
			results = append(results, res)
			continue
		}
		res.Listed = len(ids)
		for _, id := range ids {
			if ctx.Err() != nil {
				break
			}
			if ovh.CartInUse(id) {
				res.Kept++
				continue
			}
			var cart struct {
				Description string `json:"description"`
				ReadOnly    bool   `json:"readOnly"`
			}
			if err := client.GetWithContext(ctx, "/order/cart/"+id, &cart); err != nil {
				res.Kept++
				continue
			}
			if cart.Description != ovh.CartMarker || cart.ReadOnly || ovh.CartInUse(id) {
				res.Kept++
				continue
			}
			if err := client.DeleteWithContext(ctx, "/order/cart/"+id, nil); err != nil {
				res.Kept++
				state.Logger.Debug(fmt.Sprintf("删除遗留购物车 %s 失败: %s", id, err.Error()), "purchase")
				continue
			}
			res.Deleted++
		}
		if res.Deleted > 0 {
			state.Logger.Info(fmt.Sprintf("已清理账户 %s 的 %d 辆遗留购物车", acc.Name, res.Deleted), "purchase")
		}
		results = append(results, res)
	}
	return results
}

// CartJanitorLoop 启动 30 秒后清理一次,之后每 6 小时一次
func CartJanitorLoop(ctx context.Context, state *app.State) {
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			CleanupLeftoverCarts(runCtx, state)
			cancel()
			timer.Reset(6 * time.Hour)
		}
	}
}
