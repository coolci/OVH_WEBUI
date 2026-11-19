package monitor

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/notify"
	"github.com/ovh-webui/server/internal/telegram"
)

func (m *Monitor) SendAvailabilityAlert(planCode, datacenter, status, changeType string,
	configInfo map[string]interface{}, serverName, durationText, priceCheckError, traceID, configTraceID, detectedTime string,
	accountID ...string) {

	var msg strings.Builder
	pushTime := m.nowBeijing()
	summary, memH, diskH := formatConfigSpec(configInfo)

	switch changeType {
	case "available":
		msg.WriteString(fmt.Sprintf("🎉 服务器上架通知 · %s\n", planCode))
		msg.WriteString(telegram.CardDivider + "\n")
		if serverName != "" {
			msg.WriteString("🖥️ 产品名称: " + serverName + "\n")
		}
		msg.WriteString("📍 现货节点: " + dcDisplayCN(datacenter) + " (" + strings.ToUpper(datacenter) + ")\n")
		if summary != "" {
			msg.WriteString("⚙️ 硬件规格: " + summary + "\n")
		} else {
			if memH != "" {
				msg.WriteString("💾 内存: " + memH + "\n")
			}
			if diskH != "" {
				msg.WriteString("💿 存储: " + diskH + "\n")
			}
		}
		priceText, _ := configInfo["cached_price"].(string)
		if priceText == "" {
			// 用 30 秒超时保护，
			// 否则在 OVH 价格 API 卡死时整个通知会阻塞
			priceText, _ = m.getPriceWithTimeout(planCode, datacenter, configInfo, 30*time.Second)
		}
		installText, _ := configInfo["install_price"].(string)
		firstMonthText, _ := configInfo["first_month_price"].(string)
		if installText == "" {
			options := optionsFromConfig(configInfo)
			accountID := accountIDFromConfig(configInfo)
			info := m.resolvePlanPriceInfo(planCode, accountID, options)
			installText = info.InstallText
			if firstMonthText == "" {
				firstMonthText = info.FirstMonthText
			}
		}
		if firstMonthText == "" && installText != "" && priceText != "" {
			firstMonthText = computeFirstMonthTotal(priceText, installText)
		}
		if priceText != "" {
			if installText != "" {
				msg.WriteString("💰 月付价格: " + priceText + "\n")
				msg.WriteString("💵 安装费用: " + formatInstallFeeLine(installText) + "\n")
				if firstMonthText != "" {
					msg.WriteString("🧾 首月合计: " + firstMonthText + "\n")
				}
			} else {
				msg.WriteString("💰 实时价格: " + priceText + "\n")
			}
		}
		msg.WriteString("📊 可用状态: " + status + "\n")
		if durationText != "" {
			msg.WriteString("⏱️ 上次无货→本次有货: " + strings.TrimPrefix(durationText, "历时 ") + "\n")
		}
		msg.WriteString(telegram.CardDivider + "\n")
		if detectedTime != "" {
			if t, err := time.Parse(time.RFC3339Nano, detectedTime); err == nil {
				delay := pushTime.Sub(t)
				secs := int(delay.Seconds())
				minutes := secs / 60
				rem := secs % 60
				msg.WriteString("⏰ 检测时间: " + t.Format("2006-01-02 15:04:05") + "\n")
				msg.WriteString("📤 推送时间: " + pushTime.Format("2006-01-02 15:04:05") + "\n")
				switch {
				case secs > 0 && minutes > 0:
					msg.WriteString(fmt.Sprintf("⏱️ 推送延迟: %d分%d秒\n", minutes, rem))
				case secs > 0:
					msg.WriteString(fmt.Sprintf("⏱️ 推送延迟: %d秒\n", rem))
				default:
					msg.WriteString("⏱️ 推送延迟: <1秒\n")
				}
			}
		} else {
			msg.WriteString("⏰ 推送时间: " + pushTime.Format("2006-01-02 15:04:05") + "\n")
		}
		if traceLine := formatTraceBlock(traceID, configTraceID); traceLine != "" {
			msg.WriteString(traceLine + "\n")
		}
		msg.WriteString(telegram.CardDivider + "\n")
		activeCoupon := telegram.ActiveCoupon(m.state)
		if activeCoupon != "" {
			msg.WriteString("🎫 默认优惠码: " + activeCoupon + " (抢购就绪 · 按钮下单自动抵扣)\n")
		}
		cmdCopy := "/buy " + planCode + " " + strings.ToLower(datacenter) + " 1"
		if activeCoupon != "" {
			cmdCopy += " coupon=" + activeCoupon
		}
		msg.WriteString("💡 快捷下单指令 (点击即可复制):\n`" + cmdCopy + "`\n")

	case "price_check_failed":
		kind, friendly := ClassifyPriceCheckError(priceCheckError)
		if kind == ErrorKindPreSale {
			msg.WriteString(fmt.Sprintf("⏳ 预售等待提醒 · %s\n", planCode))
		} else if kind == ErrorKindRegionBlocked {
			msg.WriteString(fmt.Sprintf("🚫 跨区受限提醒 · %s\n", planCode))
		} else {
			msg.WriteString(fmt.Sprintf("⚠️ 价格校验告警 · %s\n", planCode))
		}
		msg.WriteString(telegram.CardDivider + "\n")
		if serverName != "" {
			msg.WriteString("🖥️ 产品名称: " + serverName + "\n")
		}
		msg.WriteString("📍 现货节点: " + dcDisplayCN(datacenter) + " (" + strings.ToUpper(datacenter) + ")\n")
		if summary != "" {
			msg.WriteString("⚙️ 硬件规格: " + summary + "\n")
		}
		if priceText, ok := configInfo["cached_price"].(string); ok && priceText != "" {
			msg.WriteString("💰 实时价格: " + priceText + "\n")
		}
		msg.WriteString("📊 库存状态: 可用性检测有货\n")
		if priceCheckError != "" {
			if kind == ErrorKindPreSale {
				msg.WriteString(fmt.Sprintf("⏳ 预售等待中: %s（已跳过本轮下单，后台持续蹲守解禁）\n", friendly))
			} else if kind == ErrorKindRegionBlocked {
				msg.WriteString(fmt.Sprintf("🚫 跨区受限: %s（建议换绑同大区账户）\n", friendly))
			} else {
				msg.WriteString(fmt.Sprintf("❌ 校验失败: %s（已安全跳过自动下单）\n", priceCheckError))
			}
		} else {
			msg.WriteString("❌ 校验失败: 价格校验未通过（已安全跳过自动下单）\n")
		}
		msg.WriteString(telegram.CardDivider + "\n")
		msg.WriteString("⏰ 发生时间: " + pushTime.Format("2006-01-02 15:04:05") + "\n")
		if traceLine := formatTraceBlock(traceID, configTraceID); traceLine != "" {
			msg.WriteString(traceLine + "\n")
		}

	default:
		msg.WriteString(fmt.Sprintf("📦 服务器下架通知 · %s\n", planCode))
		msg.WriteString(telegram.CardDivider + "\n")
		if serverName != "" {
			msg.WriteString("🖥️ 产品名称: " + serverName + "\n")
		}
		if summary != "" {
			msg.WriteString("⚙️ 硬件规格: " + summary + "\n")
		} else {
			if memH != "" {
				msg.WriteString("💾 内存: " + memH + "\n")
			}
			if diskH != "" {
				msg.WriteString("💿 存储: " + diskH + "\n")
			}
		}
		msg.WriteString("📍 下架机房: " + dcDisplayCN(datacenter) + " (" + strings.ToUpper(datacenter) + ")\n")
		msg.WriteString("📊 当前状态: 已无货 (缺货)\n")
		if durationText != "" {
			msg.WriteString("⏱️ 上架持续: " + strings.TrimPrefix(durationText, "历时 "))
		}
		msg.WriteString(telegram.CardDivider + "\n")
		msg.WriteString("⏰ 发生时间: " + pushTime.Format("2006-01-02 15:04:05") + "\n")
		if traceLine := formatTraceBlock(traceID, configTraceID); traceLine != "" {
			msg.WriteString(traceLine + "\n")
		}
	}

	configDesc := ""
	if configInfo != nil {
		if d, ok := configInfo["display"].(string); ok {
			configDesc = " [" + d + "]"
		}
	}
	var replyMarkup map[string]interface{}
	if changeType == "available" {
		options := optionsFromConfig(configInfo)
		explicitAccounts := append([]string(nil), accountID...)
		if cfgAcc := accountIDFromConfig(configInfo); cfgAcc != "" {
			explicitAccounts = append(explicitAccounts, cfgAcc)
		}
		btnAccounts := m.AccountsForPlan(planCode)
		if len(btnAccounts) <= 1 {
			btnAccounts = nil
		}
		type btn struct {
			Text         string `json:"text"`
			CallbackData string `json:"callback_data"`
		}
		var kb [][]btn

		if len(btnAccounts) >= 2 && len(btnAccounts) <= 3 {
			var accRow []btn
			for _, acc := range btnAccounts {
				subUUID := uuid.NewString()
				m.AddMessageUUID(subUUID, planCode, datacenter, options, configInfo)
				if m.state.DB != nil {
					_ = m.state.DB.SetTelegramButtonAccount(subUUID, acc.ID)
				}
				cbAcc, _ := json.Marshal(map[string]string{"a": "add_to_queue", "u": subUUID})
				accRow = append(accRow, btn{
					Text:         "⚡ 抢购 · " + acc.Name,
					CallbackData: string(cbAcc),
				})
			}
			kb = append(kb, accRow)
			masterUUID := uuid.NewString()
			m.AddMessageUUID(masterUUID, planCode, datacenter, options, configInfo)
			if btnAccID := m.resolveNotifyAccountID(planCode, explicitAccounts...); btnAccID != "" && m.state.DB != nil {
				_ = m.state.DB.SetTelegramButtonAccount(masterUUID, btnAccID)
			}
			cbQ, _ := json.Marshal(map[string]string{"a": "add_to_queue", "u": masterUUID})
			cbPick, _ := json.Marshal(map[string]string{"a": "pick_acc", "u": masterUUID})
			kb = append(kb, []btn{
				{Text: "📥 挂机入队", CallbackData: string(cbQ)},
				{Text: "👤 更多账户", CallbackData: string(cbPick)},
			})
		} else {
			btnUUID := uuid.NewString()
			m.AddMessageUUID(btnUUID, planCode, datacenter, options, configInfo)
			btnAccID := m.resolveNotifyAccountID(planCode, explicitAccounts...)
			if btnAccID != "" && m.state.DB != nil {
				_ = m.state.DB.SetTelegramButtonAccount(btnUUID, btnAccID)
			}
			cb, _ := json.Marshal(map[string]string{"a": "add_to_queue", "u": btnUUID})
			kb = append(kb, []btn{
				{Text: "⚡ 立即抢购 (" + dcButtonLabel(datacenter) + ")", CallbackData: string(cb)},
			})
			if m.state != nil && len(m.state.AccountsSnapshot()) > 1 {
				cbPick, _ := json.Marshal(map[string]string{"a": "pick_acc", "u": btnUUID})
				kb = append(kb, []btn{
					{Text: "📥 加入抢购队列", CallbackData: string(cb)},
					{Text: "👤 切换下单账户", CallbackData: string(cbPick)},
				})
			}
		}
		replyMarkup = map[string]interface{}{"inline_keyboard": kb}
	}
	m.state.Logger.Info(fmt.Sprintf("正在发送Telegram通知: %s@%s%s", planCode, datacenter, configDesc), "monitor")
	if notify.Broadcast(m.state, msg.String(), replyMarkup) > 0 {
		m.state.Logger.Info(fmt.Sprintf("✅ Telegram通知发送成功: %s@%s%s - %s", planCode, datacenter, configDesc, changeType), "monitor")
	} else {
		m.state.Logger.Warn(fmt.Sprintf("⚠️ Telegram通知发送失败: %s@%s%s", planCode, datacenter, configDesc), "monitor")
	}
}

func (m *Monitor) SendNewServerAlert(server map[string]interface{}) {
	planCode, _ := server["planCode"].(string)
	name, _ := server["name"].(string)
	cpu, _ := server["cpu"].(string)
	mem, _ := server["memory"].(string)
	stor, _ := server["storage"].(string)
	bw, _ := server["bandwidth"].(string)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("🆕 发现新服务器型号 · %s\n", planCode))
	b.WriteString(telegram.CardDivider + "\n")
	if name != "" {
		b.WriteString("🖥️ 型号名称: " + name + "\n")
	}
	if cpu != "" {
		b.WriteString("🧠 处理器: " + cpu + "\n")
	}
	memH := telegram.HumanizeMemory(mem)
	if memH == "" {
		memH = mem
	}
	storH := telegram.HumanizeStorage(stor)
	if storH == "" {
		storH = stor
	}
	if memH != "" && storH != "" {
		b.WriteString("⚙️ 硬件规格: " + memH + " ｜ " + storH + "\n")
	} else {
		if memH != "" {
			b.WriteString("💾 内存规格: " + memH + "\n")
		}
		if storH != "" {
			b.WriteString("💿 存储规格: " + storH + "\n")
		}
	}
	if bw != "" {
		b.WriteString("🌐 官方带宽: " + bw + "\n")
	}
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("⏰ 发现时间: " + m.nowBeijing().Format("2006-01-02 15:04:05") + "\n")
	b.WriteString("⚡ 查看库存: /stock " + planCode)

	type btn struct {
		Text         string `json:"text"`
		CallbackData string `json:"callback_data"`
	}
	var replyMarkup map[string]interface{}
	if planCode != "" {
		replyMarkup = map[string]interface{}{
			"inline_keyboard": [][]btn{
				{
					{Text: "📦 查看当前库存", CallbackData: "i:P:s:" + planCode},
					{Text: "⚡ 一键加入监控", CallbackData: "i:M:" + planCode},
				},
			},
		}
	}
	notify.Broadcast(m.state, b.String(), replyMarkup)
	m.state.Logger.Info(fmt.Sprintf("发送新服务器提醒: %v", planCode), "monitor")
}

// orderButtonTarget 上架通知里一颗一键下单按钮 = 一个 (机房, 账户) 组合。
// accountID 为空表示按钮不带账户,点击时退回默认账户。
type orderButtonTarget struct {
	dc        string
	accountID string
	label     string
}

// orderButtonTargets 算出上架通知要放哪些一键下单按钮。
// 拼按钮和预热购物车都用它 —— 预热的 (机房, 账户) 必须和按钮点下去时用的完全一致,
// 否则热好的车在池里对不上号,点按钮照样从零装车。
func (m *Monitor) orderButtonTargets(planCode string, availableDCs []map[string]interface{}, btnAccountID string) []orderButtonTarget {
	// 能买这台机器的账户。多于一个时每个机房 × 每个账户各一颗按钮 ——
	// 抢稀缺机器时多账户就是多一次机会,而看到补货通知那一刻是最不该去打字切账户的时候。
	//
	// 只认目录,不解析 planCode 后缀:实测美区目录里 42 个 `-eu` 后缀的机型
	// 卖的是欧洲机房的机器,却要用**美区**账户下单。
	btnAccounts := m.AccountsForPlan(planCode)
	if len(btnAccounts) <= 1 {
		// 判不出来 / 只有一个能买 → 保持老样子:每个机房一颗按钮,账户走上面解析出来的那个
		btnAccounts = nil
	}
	// 按钮总数封顶。5 个机房 × 3 个账户 = 15 颗,手机上会把消息挤得没法看,
	// Telegram 对键盘也有上限。超了就退回"每机房一颗",账户仍按订阅解析。
	if len(btnAccounts) > 0 && len(availableDCs)*len(btnAccounts) > maxOrderButtons {
		btnAccounts = nil
	}

	targets := []orderButtonTarget{}
	for _, dcInfo := range availableDCs {
		dc, _ := dcInfo["dc"].(string)
		if len(btnAccounts) == 0 {
			label := "⚡ 抢购 · " + dcButtonLabel(dc)
			if len(availableDCs) == 1 {
				label = "⚡ 立即抢购 (" + dcButtonLabel(dc) + ")"
			}
			targets = append(targets, orderButtonTarget{dc: dc, accountID: btnAccountID, label: label})
			continue
		}
		for _, a := range btnAccounts {
			targets = append(targets, orderButtonTarget{dc: dc, accountID: a.ID,
				label: "⚡ " + dcButtonLabel(dc) + " · " + a.Name})
		}
	}
	return targets
}

// prewarmAlertButtons 发上架通知时,为每颗一键下单按钮预热一辆购物车。
// 键要和点按钮后入队的任务完全一致:同一账户(按钮没记账户就是默认账户)、
// 同一机房、同一配件、当前 TG 默认优惠码、月付。
func (m *Monitor) prewarmAlertButtons(planCode string, availableDCs []map[string]interface{},
	configInfo map[string]interface{}, accountID ...string) int {
	if m.state == nil {
		return 0
	}
	options := optionsFromConfig(configInfo)
	coupon := telegram.ActiveCoupon(m.state)
	explicitAccounts := append([]string(nil), accountID...)
	if cfgAcc := accountIDFromConfig(configInfo); cfgAcc != "" {
		explicitAccounts = append(explicitAccounts, cfgAcc)
	}
	btnAccountID := m.resolveNotifyAccountID(planCode, explicitAccounts...)
	unverifiedDCs := map[string]bool{}
	for _, dcInfo := range availableDCs {
		if unverified, _ := dcInfo["price_unverified"].(bool); unverified {
			if dc, _ := dcInfo["dc"].(string); strings.TrimSpace(dc) != "" {
				unverifiedDCs[strings.ToLower(strings.TrimSpace(dc))] = true
			}
		}
	}
	seen := map[string]bool{}
	n := 0
	for _, t := range m.orderButtonTargets(planCode, availableDCs, btnAccountID) {
		acc := t.accountID
		if acc == "" {
			if a, ok := m.state.FindAccount(""); ok {
				acc = a.ID
			}
		}
		dc := strings.TrimSpace(t.dc)
		if acc == "" || dc == "" {
			continue
		}
		if unverifiedDCs[strings.ToLower(dc)] {
			continue
		}
		k := acc + "|" + strings.ToLower(dc)
		if seen[k] {
			continue
		}
		seen[k] = true
		if m.isPlanInCooldown(acc, "", planCode) {
			m.logPlanCooldownSkip(acc, "", planCode, "通知按钮预热")
			continue
		}
		n++
		go m.assemblePrewarmedCartAsync(acc, planCode, dc, options, coupon, "") // TG 一键下单按钮按月付
	}
	return n
}
