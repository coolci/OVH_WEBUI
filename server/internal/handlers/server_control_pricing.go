package handlers

import (
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/ovh"
)

// 服务器的续费价格与安装费（只读）。
//
//   - 续费价格：GET /services/{serviceId} 的 billing.pricing（services.billing.Pricing，三区一致），
//     加上 GET /services/{serviceId}/options 里每个同币种、同周期附加选项的计价（与 OVH 控制台 Service.addOptions 一致）；
//     口径不同或选项读取失败时标 partial，不硬加。
//   - 安装费：按当前公开目录计算该型号 + 各附加选项（billing.plan.code）的不含税安装费与税额。

type servicePricing struct {
	Billing struct {
		Plan struct {
			Code string `json:"code"`
		} `json:"plan"`
		Pricing *struct {
			Duration string `json:"duration"`
			Interval int64  `json:"interval"`
			Price    struct {
				Value    *float64 `json:"value"`
				Currency string   `json:"currencyCode"`
				Text     string   `json:"text"`
			} `json:"price"`
		} `json:"pricing"`
	} `json:"billing"`
}

// configSetupFee 测试中可替换为离线 stub，不拉取公开目录。
var configSetupFee = catalog.ConfigSetupFee

// GetServerPricing GET /api/server-control/:service_name/pricing
func GetServerPricing(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		acc, ok := ovhAccountFor(state, c)
		if !ok {
			noOVHResp(c)
			return
		}
		client, err := state.OVH.IsolatedClientFor(acc.ID)
		if err != nil {
			noOVHResp(c)
			return
		}
		id, err := serviceIDForDedicated(client, svc)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": "读取服务编号失败: " + ovh.Explain(err)})
			return
		}
		idStr := strconv.FormatInt(id, 10)
		svcPath, err := buildOVHPath("services", idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		optPath, err := buildOVHPath("services", idStr, "options")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		var main servicePricing
		if err := client.GetWithContext(c.Request.Context(), svcPath, &main); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": "读取续费价格失败: " + ovh.Explain(err)})
			return
		}
		resp := gin.H{"success": true, "planCode": main.Billing.Plan.Code}
		var opts []servicePricing
		optsErr := client.GetWithContext(c.Request.Context(), optPath, &opts)
		if renewal := sumRenewal(main, opts, optsErr != nil); renewal != nil {
			resp["renewal"] = renewal
		}
		if code := main.Billing.Plan.Code; code != "" {
			addonCodes := make([]string, 0, len(opts))
			for _, o := range opts {
				if o.Billing.Plan.Code != "" {
					addonCodes = append(addonCodes, o.Billing.Plan.Code)
				}
			}
			fee, currency, _, feeErr := configSetupFee(state, acc.ID, code, addonCodes)
			if feeErr != nil {
				resp["installation"] = gin.H{"ok": false, "reason": feeErr.Error()}
			} else {
				resp["installation"] = gin.H{"ok": true, "amount": round2(fee.Price), "tax": round2(fee.Tax), "currency": currency, "partial": optsErr != nil}
			}
		}
		c.JSON(http.StatusOK, resp)
	}
}

// sumRenewal 主服务 + 同币种、同周期的附加选项；选项读不到或口径不同时标 partial，不硬加。
func sumRenewal(main servicePricing, opts []servicePricing, optsFailed bool) gin.H {
	p := main.Billing.Pricing
	if p == nil || p.Price.Value == nil || math.IsNaN(*p.Price.Value) || math.IsInf(*p.Price.Value, 0) || p.Price.Currency == "" {
		return nil
	}
	total := *p.Price.Value
	partial := optsFailed
	options := 0
	for _, o := range opts {
		op := o.Billing.Pricing
		if op == nil || op.Price.Value == nil || math.IsNaN(*op.Price.Value) || math.IsInf(*op.Price.Value, 0) {
			continue
		}
		if op.Price.Currency != p.Price.Currency || (op.Duration != "" && p.Duration != "" && op.Duration != p.Duration) {
			partial = true
			continue
		}
		if *op.Price.Value > 0 {
			options++
		}
		total += *op.Price.Value
	}
	return gin.H{
		"amount":   round2(total),
		"currency": p.Price.Currency,
		"duration": p.Duration,
		"interval": p.Interval,
		"base":     round2(*p.Price.Value),
		"options":  options,
		"partial":  partial,
	}
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
