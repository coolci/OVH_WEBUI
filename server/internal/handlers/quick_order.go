package handlers

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/orderterm"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/purchase"
)

var (
	prewarmInflightMu sync.Mutex
	prewarmInflight   = map[string]bool{}
	prewarmSem        = make(chan struct{}, 4)
)

func canonicalPrewarmKey(accountID, planCode, datacenter string, options []string, coupon, duration string) string {
	opts := make([]string, 0, len(options))
	for _, o := range options {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			opts = append(opts, trimmed)
		}
	}
	sort.Strings(opts)
	return strings.Join([]string{
		accountID,
		planCode,
		datacenter,
		strings.Join(opts, ","),
		strings.TrimSpace(coupon),
		duration,
	}, "|")
}

// QuickOrder POST /api/queue/quick-order
// 外部主动调用或前端触发: 委托给领域层 purchase.EnqueueQuickOrder 执行校验与入队。
func QuickOrder(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		var params purchase.QuickOrderParams
		// 严格解析:未知字段直接拒绝,不能把不支持的下单约束静默当成默认值
		if err := bindOrderJSON(c, &params); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "任务参数格式不正确", "details": err.Error()})
			return
		}
		// 监控已在进程内直接调用 purchase.EnqueueQuickOrder;HTTP 入口不能声称自己来自监控,
		// 否则任意调用方都能跳过重复下单检查和自动建单停止闸门。
		params.FromMonitor = false
		params.SkipDuplicateCheck = false
		if params.AccountID == "" && params.AccountIDCamel != "" {
			params.AccountID = params.AccountIDCamel
		}
		if params.AccountID == "" {
			if acc := c.Query("account"); acc != "" {
				params.AccountID = acc
			} else if acc := c.Query("accountId"); acc != "" {
				params.AccountID = acc
			}
		}

		res := purchase.EnqueueQuickOrder(state, params)
		if !res.Success {
			resp := gin.H{
				"success": false,
				"error":   res.Error,
			}
			if res.Code != "" {
				resp["code"] = res.Code
			}
			c.JSON(res.HTTPStatus, resp)
			return
		}

		resp := gin.H{
			"success": true,
			"message": res.Message,
			"options": res.Options,
		}
		if res.Price != nil {
			resp["price"] = res.Price
		}
		c.JSON(http.StatusOK, resp)
	}
}

// PrewarmOrder POST /api/purchase/prewarm
// 接收前端选型/抢购意图前置预热信号，在 OVH 侧后台异步组装购物车待命
func PrewarmOrder(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			AccountID  string   `json:"account_id"`
			PlanCode   string   `json:"planCode"`
			Datacenter string   `json:"datacenter"`
			Options    []string `json:"options"`
			Coupon     string   `json:"coupon"`
			Duration   string   `json:"duration"`
		}
		if err := bindOrderJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "无效的预热参数: " + err.Error()})
			return
		}
		planCode := strings.TrimSpace(req.PlanCode)
		rawDC := strings.TrimSpace(req.Datacenter)
		if planCode == "" || rawDC == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "planCode 和 datacenter 必填"})
			return
		}
		datacenter := ovh.ConvertDisplayDCToAPIDC(rawDC)
		if datacenter == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "无效的 datacenter 参数"})
			return
		}
		duration, termErr := orderterm.Normalize(req.Duration)
		if termErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": termErr.Error()})
			return
		}
		acc, ok := state.FindAccount(req.AccountID)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "未找到有效下单账户"})
			return
		}
		accountID := acc.ID
		coupon := strings.TrimSpace(req.Coupon)

		// 若池中已就绪，直接返回
		if purchase.HasPrewarmedCart(accountID, planCode, datacenter, req.Options, coupon, duration) {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"ready":   true,
				"message": "预热购物车已就绪待命",
			})
			return
		}

		// 同键在途去重（Singleflight）：防止前端多组件/高频抖动同时触发 N 辆孤儿购物车风暴
		inflightKey := canonicalPrewarmKey(accountID, planCode, datacenter, req.Options, coupon, duration)
		prewarmInflightMu.Lock()
		if prewarmInflight[inflightKey] {
			prewarmInflightMu.Unlock()
			c.JSON(http.StatusOK, gin.H{
				"success":  true,
				"ready":    false,
				"inflight": true,
				"message":  "相同配置的预热装车已在后台进行中",
			})
			return
		}
		prewarmInflight[inflightKey] = true
		prewarmInflightMu.Unlock()

		optionsCopy := append([]string(nil), req.Options...)
		// 异步在 OVH 侧受并发信号量保护组装预热车
		go func() {
			defer func() {
				prewarmInflightMu.Lock()
				delete(prewarmInflight, inflightKey)
				prewarmInflightMu.Unlock()
			}()
			prewarmSem <- struct{}{}
			defer func() { <-prewarmSem }()

			if purchase.HasPrewarmedCart(accountID, planCode, datacenter, optionsCopy, coupon, duration) {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			_, _ = purchase.AssemblePrewarmedCart(ctx, state, accountID, planCode, datacenter, optionsCopy, coupon, duration)
		}()

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"ready":   false,
			"message": "已在后台启动预热装车",
		})
	}
}
