package monitor

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/catalog"
)

// notification 单次状态变化通知（内部）
type notification struct {
	dc               string
	status           string
	oldStatus        string
	hasOld           bool
	statusKey        string
	changeType       string
	priceCheckFailed bool
	priceCheckError  string
	configTraceID    string
	traceID          string
	detectedTime     string
	durationText     string
}

func (n notification) oldStatusJSON() interface{} {
	if !n.hasOld {
		return nil
	}
	return n.oldStatus
}

// CheckAvailabilityChange 对应 Python: check_availability_change
func (m *Monitor) CheckAvailabilityChange(sub *Subscription, traceID string) {
	planCode := sub.PlanCode
	// 监控用订阅 auto_order 账户的 subsidiary 拉 catalog,这样跨子公司 multi-account
	// 触发 auto-order 时,options 匹配能命中目标账户独有的项。无 auto-order 账户时落默认账户。
	currentAvailability := catalog.CheckServerAvailabilityWithConfigs(m.state, planCode, sub.AutoOrderAccountID)
	if len(currentAvailability) == 0 {
		m.state.Logger.Warn(fmt.Sprintf("无法获取 %s 的可用性信息", planCode), "monitor")
		return
	}

	if sub.LastStatus == nil {
		sub.LastStatus = map[string]string{}
	}
	lastStatus := sub.LastStatus
	monitoredDCs := sub.Datacenters

	m.state.Logger.Info(fmt.Sprintf("订阅 %s - 监控数据中心: %v", planCode, monitoredDCs), "monitor")
	m.state.Logger.Info(fmt.Sprintf("订阅 %s - 当前发现 %d 个配置组合", planCode, len(currentAvailability)), "monitor")

	for configKey, configData := range currentAvailability {
		memory := configData.Memory
		storage := configData.Storage
		configDisplay := memory + " + " + storage

		configTraceID := uuid.NewString()
		m.state.Logger.Info(fmt.Sprintf("检查配置: %s [config-trace:%s]", configDisplay, configTraceID), "monitor")

		configInfo := map[string]interface{}{
			"memory":  memory,
			"storage": storage,
			"display": configDisplay,
			"options": configData.Options,
		}

		type dcStatus struct {
			status    string
			statusKey string
			oldStatus string
			hasOld    bool
		}
		dcStatusMap := map[string]dcStatus{}
		priceCheckTasks := []string{}
		for dc, status := range configData.Datacenters {
			if len(monitoredDCs) > 0 && !containsString(monitoredDCs, dc) {
				continue
			}
			statusKey := dc + "|" + configKey
			old, hasOld := lastStatus[statusKey]
			dcStatusMap[dc] = dcStatus{status: status, statusKey: statusKey, oldStatus: old, hasOld: hasOld}
			if status != "unavailable" {
				priceCheckTasks = append(priceCheckTasks, dc)
			}
		}

		// 并发价格校验
		priceCheckResults := map[string][2]interface{}{}
		if len(priceCheckTasks) > 0 {
			var pcMu sync.Mutex
			var wg sync.WaitGroup
			workers := len(priceCheckTasks)
			if workers > 10 {
				workers = 10
			}
			sem := make(chan struct{}, workers)
			for _, dc := range priceCheckTasks {
				wg.Add(1)
				sem <- struct{}{}
				go func(dc string) {
					defer wg.Done()
					defer func() { <-sem }()
					ok, errMsg := m.verifyPriceAvailable(m.resolvePriceAccount(sub), planCode, dc, configInfo)
					pcMu.Lock()
					priceCheckResults[dc] = [2]interface{}{ok, errMsg}
					pcMu.Unlock()
					if ok {
						m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 价格校验通过 [config-trace:%s]",
							planCode, dc, configDisplay, configTraceID), "monitor")
					} else {
						m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 价格校验失败，原因: %s [config-trace:%s]",
							planCode, dc, configDisplay, errMsg, configTraceID), "monitor")
					}
				}(dc)
			}
			wg.Wait()
		}

		notifications := []notification{}

		for dc, ds := range dcStatusMap {
			actualStatus := ds.status
			priceCheckFailed := false
			priceCheckError := ""

			if ds.status != "unavailable" {
				if v, ok := priceCheckResults[dc]; ok {
					okBool, _ := v[0].(bool)
					errStr, _ := v[1].(string)
					if !okBool {
						actualStatus = "price_check_failed"
						priceCheckFailed = true
						priceCheckError = errStr
						m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 可用性显示有货但价格校验失败，原因: %s，标记为price_check_failed",
							planCode, dc, configDisplay, errStr), "monitor")
					} else {
						actualStatus = "available"
						m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 可用性有货且价格校验通过，确认有货",
							planCode, dc, configDisplay), "monitor")
					}
				} else {
					actualStatus = "price_check_failed"
					priceCheckFailed = true
					priceCheckError = "价格校验未执行"
				}
			}

			statusChanged := false
			changeType := ""

			if !ds.hasOld {
				if actualStatus == "price_check_failed" {
					m.state.Logger.Info(fmt.Sprintf("首次检查: %s@%s [%s] 可用性有货但价格校验失败，发送通知",
						planCode, dc, configDisplay), "monitor")
					if sub.NotifyAvailable {
						statusChanged = true
						changeType = "price_check_failed"
					}
				} else if actualStatus == "unavailable" {
					m.state.Logger.Info(fmt.Sprintf("首次检查: %s@%s [%s] 无货", planCode, dc, configDisplay), "monitor")
					if sub.NotifyUnavailable {
						statusChanged = true
						changeType = "unavailable"
					}
				} else {
					m.state.Logger.Info(fmt.Sprintf("首次检查: %s@%s [%s] 有货（价格校验通过），发送通知",
						planCode, dc, configDisplay), "monitor")
					if sub.NotifyAvailable {
						statusChanged = true
						changeType = "available"
					}
				}
			} else if ds.oldStatus == "unavailable" && actualStatus == "available" {
				if sub.NotifyAvailable {
					statusChanged = true
					changeType = "available"
					m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 从无货变有货（价格校验通过）",
						planCode, dc, configDisplay), "monitor")
				}
			} else if ds.oldStatus == "unavailable" && actualStatus == "price_check_failed" {
				m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 从无货变可用性有货但价格校验失败，发送通知",
					planCode, dc, configDisplay), "monitor")
				if sub.NotifyAvailable {
					statusChanged = true
					changeType = "price_check_failed"
				}
			} else if ds.oldStatus == "price_check_failed" && actualStatus == "available" {
				if sub.NotifyAvailable {
					statusChanged = true
					changeType = "available"
					m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 从价格校验失败变有货（价格校验通过）",
						planCode, dc, configDisplay), "monitor")
				}
			} else if ds.oldStatus == "price_check_failed" && actualStatus == "unavailable" {
				if sub.NotifyUnavailable {
					statusChanged = true
					changeType = "unavailable"
					m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 从价格校验失败变无货",
						planCode, dc, configDisplay), "monitor")
				}
			} else if ds.oldStatus == "available" && actualStatus == "unavailable" {
				if sub.NotifyUnavailable {
					statusChanged = true
					changeType = "unavailable"
					m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 从有货变无货", planCode, dc, configDisplay), "monitor")
				}
			} else if ds.oldStatus == "available" && actualStatus == "price_check_failed" {
				m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 从有货变可用性有货但价格校验失败，发送通知",
					planCode, dc, configDisplay), "monitor")
				if sub.NotifyAvailable {
					statusChanged = true
					changeType = "price_check_failed"
				}
			}

			if statusChanged {
				detectedTime := m.nowBeijing().Format(time.RFC3339Nano)
				n := notification{
					dc:               dc,
					status:           actualStatus,
					oldStatus:        ds.oldStatus,
					hasOld:           ds.hasOld,
					statusKey:        ds.statusKey,
					changeType:       changeType,
					priceCheckFailed: priceCheckFailed,
					priceCheckError:  priceCheckError,
					configTraceID:    configTraceID,
					traceID:          traceID,
					detectedTime:     detectedTime,
				}
				if changeType == "available" && ds.oldStatus == "unavailable" {
					n.durationText = m.calcDuration(sub, dc, configDisplay, []string{"unavailable", "price_check_failed"})
				}
				notifications = append(notifications, n)
			}

			lastStatus[ds.statusKey] = actualStatus
		}

		// 价格查询（同一配置只查一次）
		var priceText string
		var priceFetchError string
		hasAvail := false
		for _, n := range notifications {
			if n.changeType == "available" {
				hasAvail = true
				break
			}
		}
		if hasAvail {
			firstDC := ""
			for _, n := range notifications {
				if n.changeType == "available" && n.status != "unavailable" {
					firstDC = n.dc
					break
				}
			}
			if firstDC != "" {
				priceText, priceFetchError = m.getPriceWithTimeout(m.resolvePriceAccount(sub), planCode, firstDC, configInfo, 30*time.Second)
				if priceText != "" {
					m.state.Logger.Debug(fmt.Sprintf("配置 %s 价格获取成功: %s，将在所有通知中复用", configDisplay, priceText), "monitor")
				} else {
					m.state.Logger.Warn(fmt.Sprintf("配置 %s 价格获取失败，通知中不包含价格信息", configDisplay), "monitor")
					if priceFetchError == "" {
						priceFetchError = "价格接口未返回结果"
					}
				}
			}
		}

		// 分类
		var availables, unavailables, priceFailed []notification
		for _, n := range notifications {
			switch n.changeType {
			case "available":
				availables = append(availables, n)
			case "unavailable":
				unavailables = append(unavailables, n)
			case "price_check_failed":
				priceFailed = append(priceFailed, n)
			}
		}

		// 自动下单：仅边沿触发 unavailable → available。
		// 禁止 !hasOld（首检/重启后无基线）直接下单，避免误抢购。
		orderTargets := []notification{}
		for _, n := range availables {
			if !n.priceCheckFailed && n.hasOld && n.oldStatus == "unavailable" {
				orderTargets = append(orderTargets, n)
			}
		}
		// 触发条件:订阅勾了 AutoOrder + 指定了某个账户。
		// 没账户(AutoOrderAccountID="") → 只发可用通知,不下单(用户明确决定)。
		if len(orderTargets) > 0 && sub.AutoOrder {
			if sub.AutoOrderAccountID == "" {
				m.state.Logger.Info(fmt.Sprintf("[monitor] %s 触发 auto_order 但未指定账户,只通知不下单", planCode), "monitor")
			} else {
				m.batchOrder(planCode, configInfo, orderTargets, sub.Quantity, sub.AutoOrderAccountID)
			}
		}

		// 发送有货通知
		if len(availables) > 0 {
			m.state.Logger.Info(fmt.Sprintf("准备发送汇总提醒: %s [%s] - %d个机房有货",
				planCode, configDisplay, len(availables)), "monitor")
			configInfoWithPrice := copyMap(configInfo)
			if priceText != "" {
				configInfoWithPrice["cached_price"] = priceText
			}
			availDCs := make([]map[string]interface{}, 0, len(availables))
			for _, n := range availables {
				dcInfo := map[string]interface{}{"dc": n.dc, "status": n.status}
				if n.durationText != "" {
					dcInfo["duration_text"] = n.durationText
				}
				if n.detectedTime != "" {
					dcInfo["detected_time"] = n.detectedTime
				}
				availDCs = append(availDCs, dcInfo)
			}
			configTraceForNotif := ""
			if len(availables) > 0 {
				configTraceForNotif = availables[0].configTraceID
			}
			errIfNoPrice := ""
			if priceText == "" {
				errIfNoPrice = priceFetchError
			}
			m.SendAvailabilityAlertGrouped(planCode, availDCs, configInfoWithPrice, sub.ServerName,
				errIfNoPrice, traceID, configTraceForNotif)

			for _, n := range availables {
				entry := HistoryEntry{
					Timestamp:  m.nowBeijing().Format(time.RFC3339Nano),
					Datacenter: n.dc,
					Status:     n.status,
					ChangeType: n.changeType,
					OldStatus:  n.oldStatusJSON(),
					Config:     configInfo,
				}
				sub.History = append(sub.History, entry)
			}
		}

		// 价格校验失败通知
		for _, n := range priceFailed {
			m.state.Logger.Info(fmt.Sprintf("准备发送价格校验失败提醒: %s@%s [%s] - 可用性有货但价格校验失败",
				planCode, n.dc, configDisplay), "monitor")
			priceTextFailed := m.GetPriceInfoText(m.resolvePriceAccount(sub), planCode, n.dc, configInfo)
			configInfoFailed := copyMap(configInfo)
			if priceTextFailed != "" {
				configInfoFailed["cached_price"] = priceTextFailed
				configInfoFailed["price_check_error"] = n.priceCheckError
			}
			m.SendAvailabilityAlert(planCode, n.dc, "unavailable", "price_check_failed",
				configInfoFailed, sub.ServerName, "", n.priceCheckError, traceID, n.configTraceID, n.detectedTime)
			entry := HistoryEntry{
				Timestamp:  m.nowBeijing().Format(time.RFC3339Nano),
				Datacenter: n.dc,
				Status:     "price_check_failed",
				ChangeType: "price_check_failed",
				OldStatus:  n.oldStatusJSON(),
				Config:     configInfo,
			}
			sub.History = append(sub.History, entry)
		}

		// 下架聚合通知
		if len(unavailables) > 0 {
			m.state.Logger.Info(fmt.Sprintf("准备发送聚合下架提醒: %s [%s] - %d个机房",
				planCode, configDisplay, len(unavailables)), "monitor")
			unavailDCs := make([]map[string]interface{}, 0, len(unavailables))
			for _, n := range unavailables {
				dcInfo := map[string]interface{}{"dc": n.dc, "status": n.status}
				isBecame := n.changeType == "unavailable" && n.hasOld && n.oldStatus != "unavailable"
				if isBecame {
					if d := m.calcDuration(sub, n.dc, configDisplay, []string{"available"}); d != "" {
						dcInfo["duration_text"] = d
					}
				}
				unavailDCs = append(unavailDCs, dcInfo)
			}
			configTraceForNotif := ""
			if len(unavailables) > 0 {
				configTraceForNotif = unavailables[0].configTraceID
			}
			m.SendUnavailableAlertGrouped(planCode, unavailDCs, configInfo, sub.ServerName,
				traceID, configTraceForNotif)
			for _, n := range unavailables {
				entry := HistoryEntry{
					Timestamp:  m.nowBeijing().Format(time.RFC3339Nano),
					Datacenter: n.dc,
					Status:     n.status,
					ChangeType: n.changeType,
					OldStatus:  n.oldStatusJSON(),
					Config:     configInfo,
				}
				sub.History = append(sub.History, entry)
			}
		}

		m.limitHistorySize(sub, 100)
	}

	// 新配置初始化
	for configKey, configData := range currentAvailability {
		for dc, status := range configData.Datacenters {
			statusKey := dc + "|" + configKey
			if _, ok := lastStatus[statusKey]; !ok {
				lastStatus[statusKey] = status
			}
		}
	}
	sub.LastStatus = lastStatus
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// calcDuration 计算最近一次相反状态到现在的历时
func (m *Monitor) calcDuration(sub *Subscription, dc, configDisplay string, targetChangeTypes []string) string {
	var lastTS string
	for i := len(sub.History) - 1; i >= 0; i-- {
		entry := sub.History[i]
		if entry.Datacenter != dc {
			continue
		}
		matched := false
		for _, t := range targetChangeTypes {
			if entry.ChangeType == t {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if configDisplay != "" && entry.Config != nil {
			if d, ok := entry.Config["display"].(string); ok && d != configDisplay {
				continue
			}
		}
		lastTS = entry.Timestamp
		if lastTS != "" {
			break
		}
	}
	if lastTS == "" {
		return ""
	}
	startDT, err := time.Parse(time.RFC3339Nano, lastTS)
	if err != nil {
		startDT, err = time.Parse(time.RFC3339, lastTS)
		if err != nil {
			return ""
		}
	}
	delta := m.nowBeijing().Sub(startDT)
	totalSec := int(delta.Seconds())
	if totalSec < 0 {
		totalSec = 0
	}
	days := totalSec / 86400
	rem := totalSec % 86400
	hours := rem / 3600
	minutes := (rem % 3600) / 60
	seconds := rem % 60
	if days > 0 {
		return fmt.Sprintf("历时 %d天%d小时%d分%d秒", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("历时 %d小时%d分%d秒", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("历时 %d分%d秒", minutes, seconds)
	}
	return fmt.Sprintf("历时 %d秒", seconds)
}
