package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
)

// orderMappingCache 简单内存缓存
var (
	orderMappingMu        sync.Mutex
	orderMappingCache     map[string]interface{}
	orderMappingCacheTime time.Time
	orderMappingDuration  = 10 * time.Minute
)

// GetOrderMapping GET /api/server-control/order-mapping
func GetOrderMapping(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		forceRefresh := strings.EqualFold(c.Query("forceRefresh"), "true")

		orderMappingMu.Lock()
		if !forceRefresh && orderMappingCache != nil && time.Since(orderMappingCacheTime) < orderMappingDuration {
			cached := orderMappingCache
			orderMappingMu.Unlock()
			state.Logger.Info(fmt.Sprintf("返回缓存的订单映射数据（共 %d 条）", len(cached)), "server_control")
			c.JSON(http.StatusOK, gin.H{
				"success":   true,
				"mapping":   cached,
				"cached":    true,
				"cacheTime": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}
		orderMappingMu.Unlock()

		state.Logger.Info("开始同步订单映射数据...", "server_control")

		// 1. 获取所有服务器创建时间(并发拉 serviceInfos)
		creationDates := []string{}
		var serverList []string
		if err := client.Get("/dedicated/server", &serverList); err == nil {
			details := parallelGetStringKeys(client, serverList, func(sn string) string {
				return "/dedicated/server/" + sn + "/serviceInfos"
			}, 10)
			for _, svcInfo := range details {
				if svcInfo == nil {
					continue
				}
				if cd, ok := svcInfo["creation"].(string); ok && cd != "" {
					creationDates = append(creationDates, cd)
				}
			}
		} else {
			state.Logger.Warn("获取服务器创建时间失败: "+err.Error()+", 将获取最近30天的订单", "server_control")
		}

		var dateFrom, dateTo time.Time
		parsedDates := []time.Time{}
		for _, ds := range creationDates {
			t, err := parseFlexible(ds)
			if err == nil {
				parsedDates = append(parsedDates, t)
			}
		}
		if len(parsedDates) > 0 {
			earliest := parsedDates[0]
			latest := parsedDates[0]
			for _, t := range parsedDates[1:] {
				if t.Before(earliest) {
					earliest = t
				}
				if t.After(latest) {
					latest = t
				}
			}
			dateFrom = earliest.Add(-15 * 24 * time.Hour)
			dateTo = latest.Add(15 * 24 * time.Hour)
			state.Logger.Info(fmt.Sprintf("服务器创建时间范围: %s 到 %s, 订单查询范围: %s 到 %s",
				earliest.Format("2006-01-02"), latest.Format("2006-01-02"),
				dateFrom.Format("2006-01-02"), dateTo.Format("2006-01-02")), "server_control")
		} else {
			dateTo = time.Now().UTC()
			dateFrom = dateTo.Add(-30 * 24 * time.Hour)
		}

		dateFromStr := dateFrom.UTC().Format("2006-01-02T15:04:05+00:00")
		dateToStr := dateTo.UTC().Format("2006-01-02T15:04:05+00:00")
		dateFromEnc := url.QueryEscape(dateFromStr)
		dateToEnc := url.QueryEscape(dateToStr)
		state.Logger.Debug("日期范围查询: from="+dateFromStr+", to="+dateToStr, "server_control")

		var allOrderIDs []int64
		path := "/me/order?date.from=" + dateFromEnc + "&date.to=" + dateToEnc
		if err := client.Get(path, &allOrderIDs); err != nil {
			state.Logger.Error("获取订单列表失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "获取订单列表失败: " + err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("时间范围内获取到 %d 个订单", len(allOrderIDs)), "server_control")

		// 过滤有效订单（5 并发）
		validIDs := []int64{}
		skipped := 0
		statusCounts := map[string]int{}
		var muVal sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, 5)
		for _, oid := range allOrderIDs {
			wg.Add(1)
			sem <- struct{}{}
			go func(id int64) {
				defer wg.Done()
				defer func() { <-sem }()
				var status string
				if err := client.Get(fmt.Sprintf("/me/order/%d/status", id), &status); err != nil {
					muVal.Lock()
					skipped++
					muVal.Unlock()
					return
				}
				lower := strings.ToLower(status)
				muVal.Lock()
				statusCounts[status]++
				if lower != "cancelled" && lower != "cancelledbycustomer" && lower != "cancelledbycustomerrequest" {
					validIDs = append(validIDs, id)
				} else {
					skipped++
				}
				muVal.Unlock()
			}(oid)
		}
		wg.Wait()
		if len(statusCounts) > 0 {
			parts := []string{}
			for k, v := range statusCounts {
				parts = append(parts, fmt.Sprintf("%s: %d", k, v))
			}
			state.Logger.Info("订单状态统计: "+strings.Join(parts, ", "), "server_control")
		}
		state.Logger.Info(fmt.Sprintf("过滤后得到 %d 个有效订单（跳过 %d 个已取消订单）", len(validIDs), skipped), "server_control")

		// 处理订单详情（10 并发）
		mapping := map[string]map[string]interface{}{}
		var muMap sync.Mutex
		var wg2 sync.WaitGroup
		sem2 := make(chan struct{}, 10)
		processed := 0
		errorCount := 0
		for _, oid := range validIDs {
			wg2.Add(1)
			sem2 <- struct{}{}
			go func(id int64) {
				defer wg2.Done()
				defer func() { <-sem2 }()
				var detailIDs []int64
				if err := client.Get(fmt.Sprintf("/me/order/%d/details", id), &detailIDs); err != nil {
					muMap.Lock()
					errorCount++
					muMap.Unlock()
					return
				}
				var orderInfo map[string]interface{}
				_ = client.Get(fmt.Sprintf("/me/order/%d", id), &orderInfo)
				orderDate := ""
				if v, ok := orderInfo["date"].(string); ok {
					orderDate = v
				}
				orderURL := fmt.Sprintf("https://www.ovh.com/manager/dedicated/#/billing/order?orderId=%d", id)
				var orderStatus string
				_ = client.Get(fmt.Sprintf("/me/order/%d/status", id), &orderStatus)
				if orderStatus == "" {
					orderStatus = "unknown"
				}

				for _, did := range detailIDs {
					var d map[string]interface{}
					if err := client.Get(fmt.Sprintf("/me/order/%d/details/%d", id, did), &d); err != nil {
						continue
					}
					serviceName, _ := d["domain"].(string)
					description, _ := d["description"].(string)
					if serviceName == "" {
						continue
					}
					isDedicated := strings.Contains(strings.ToLower(description), "dedicated") ||
						strings.Contains(strings.ToLower(description), "server") ||
						strings.Contains(serviceName, ".ip-") ||
						strings.HasPrefix(serviceName, "ns")
					if !isDedicated {
						continue
					}
					info := map[string]interface{}{
						"orderId":     id,
						"orderDate":   orderDate,
						"orderStatus": orderStatus,
						"orderUrl":    orderURL,
						"detailId":    did,
						"price":       d["totalPrice"],
						"description": description,
					}
					muMap.Lock()
					existing, exists := mapping[serviceName]
					if !exists {
						mapping[serviceName] = info
						processed++
						state.Logger.Info(fmt.Sprintf("✅ 找到服务器映射: %s -> 订单 %d", serviceName, id), "server_control")
					} else {
						existingDate, _ := existing["orderDate"].(string)
						if orderDate > existingDate {
							mapping[serviceName] = info
							state.Logger.Info(fmt.Sprintf("🔄 更新服务器映射: %s -> 订单 %d (更新)", serviceName, id), "server_control")
						}
					}
					muMap.Unlock()
				}
			}(oid)
		}
		wg2.Wait()

		// 缓存
		final := map[string]interface{}{}
		for k, v := range mapping {
			final[k] = v
		}
		orderMappingMu.Lock()
		orderMappingCache = final
		orderMappingCacheTime = time.Now()
		orderMappingMu.Unlock()

		state.Logger.Info(fmt.Sprintf("订单映射同步完成: 成功处理 %d 个服务器映射，%d 个订单处理失败", processed, errorCount), "server_control")
		state.Logger.Info(fmt.Sprintf("返回订单映射数据: 共 %d 个映射", len(final)), "server_control")
		c.JSON(http.StatusOK, gin.H{
			"success":         true,
			"mapping":         final,
			"total":           len(final),
			"processedOrders": len(validIDs),
			"cached":          false,
			"syncTime":        time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func parseFlexible(s string) (time.Time, error) {
	if strings.Contains(s, "T") {
		if s2 := strings.Replace(s, "Z", "+00:00", 1); s2 != "" {
			if t, err := time.Parse("2006-01-02T15:04:05-07:00", s2); err == nil {
				return t, nil
			}
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				return t, nil
			}
		}
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("无法解析日期: %s", s)
}
