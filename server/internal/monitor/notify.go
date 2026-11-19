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

// resolveNotifyAccountID 决定「这条上架通知生成的一键下单按钮该落到哪个 OVH 账户」。
//
// 为什么必须解析出账户:planCode 是分区的(EU / US / CA 三份目录基本不重合,
// 实测 US 目录 143 个 planCode 里只有少数与 EU 重合)。按钮不带账户时 webhook 只能
// 用「默认账户」下单,多账户跨区的人就会把欧区机型下到美区账户上 —— OVH 返回
// 200 + 空库存而不是报错,队列一直重试到过期,用户看不出是账户选错了。
//
// 优先级:
//  1. explicit —— 调用方(check.go)显式传进来的账户,最权威;
//  2. sub.AutoOrderAccountID —— 用户为这条订阅明确指定的下单账户,与自动下单走同一个;
//  3. sub.LastCheckAccountID —— 本轮真正查到这批库存的账户,它的大区一定含这个 planCode。
//
// 只接受在账户表里还存在的 id:订阅里可能留着已删除账户的 id,存进按钮只会让回调
// 在几小时后才失败,不如当场退回默认账户。
//
// 锁:这里取 subsMu。调用链是 monitorLoop → runSubscriptionCheck → CheckAvailabilityChange
// → 本函数,全程不持有 subsMu(loop.go 只在拷贝订阅列表时短暂加锁),不会自锁。
// 注意 SendNewServerAlert 是在 CheckNewServers 持有 subsMu 时调用的,那条路径没有按钮,
// 也不要在它里面调本函数。
func (m *Monitor) resolveNotifyAccountID(planCode string, explicit ...string) string {
	valid := func(id string) string {
		id = strings.TrimSpace(id)
		if id == "" {
			return ""
		}
		if acc, ok := m.state.FindAccount(id); ok && acc.ID == id {
			return id
		}
		return ""
	}
	for _, id := range explicit {
		if v := valid(id); v != "" {
			return v
		}
	}

	// 订阅的可变字段必须走 s 自己的锁(types.go 里立的约定):
	// LastCheckAccountID 的写者是检查 goroutine 的 beginCheck(只持 s.mu、不持 subsMu),
	// 这里只持 subsMu 裸读就是无同步的读-写竞争。
	// 先在 subsMu 下把指针挑出来,再逐个取带锁快照。
	m.subsMu.Lock()
	matched := make([]*Subscription, 0, 2)
	for _, sub := range m.subscriptions {
		if sub != nil && sub.PlanCode == planCode {
			matched = append(matched, sub)
		}
	}
	m.subsMu.Unlock()

	var autoOrder, lastCheck string
	for _, sub := range matched {
		snap := sub.snapshot()
		if autoOrder == "" {
			autoOrder = snap.AutoOrderAccountID
		}
		if lastCheck == "" {
			lastCheck = snap.LastCheckAccountID
		}
	}

	if v := valid(autoOrder); v != "" {
		return v
	}
	return valid(lastCheck)
}

// accountID 是可变参数而不是必填形参:写入口 check.go 不在本次改动范围内,
// 加必填形参会直接编译不过。传了就用传的,没传就由 resolveNotifyAccountID 反查订阅。
// buildAvailabilityAlert 拼出上架通知的正文和按钮。
//
// 从 SendAvailabilityAlertGrouped 里拆出来，是为了能在测试里直接看到
// 用户真正会收到的那段文字 —— 通知的排版是这个工具的门面，
// 以前只能靠真的触发一次补货才看得见。
// 注意它有副作用：会把按钮 UUID 写进 telegram_order_buttons。
func (m *Monitor) buildAvailabilityAlert(planCode string, availableDCs []map[string]interface{},
	configInfo map[string]interface{}, serverName string, priceErrorMessage string, traceID, configTraceID string,
	accountID ...string) (string, map[string]interface{}) {

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("🎉 服务器上架通知 · %s\n", planCode))
	msg.WriteString(telegram.CardDivider + "\n")

	// 产品名称:型号名 + CPU。
	msg.WriteString("📦 产品名称: " + m.productName(planCode, serverName) + "\n")

	summary, memH, diskH := formatConfigSpec(configInfo)
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
	msg.WriteString(telegram.CardDivider + "\n")

	// 机房 + 可用性。
	var detectedTimes []time.Time
	for _, dcInfo := range availableDCs {
		if dtStr, ok := dcInfo["detected_time"].(string); ok && dtStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, dtStr); err == nil {
				detectedTimes = append(detectedTimes, t)
			}
		}
	}
	if len(availableDCs) == 1 {
		dc, _ := availableDCs[0]["dc"].(string)
		msg.WriteString("📍 现货节点: " + dcLine(dc) + "\n")
		msg.WriteString("✅ 可用状态: " + availWording(availableDCs[0]) + "\n")
	} else {
		msg.WriteString(fmt.Sprintf("📍 现货节点: %d 个机房有货\n", len(availableDCs)))
		for _, dcInfo := range availableDCs {
			dc, _ := dcInfo["dc"].(string)
			msg.WriteString("   ✅ " + dcLine(dc) + " — " + availWording(dcInfo) + "\n")
		}
	}

	msg.WriteString(telegram.CardDivider + "\n")
	priceText, _ := configInfo["cached_price"].(string)
	installText, _ := configInfo["install_price"].(string)
	firstMonthText, _ := configInfo["first_month_price"].(string)
	if firstMonthText == "" && installText != "" && priceText != "" {
		firstMonthText = computeFirstMonthTotal(priceText, installText)
	}

	switch {
	case priceText != "":
		if installText != "" {
			msg.WriteString("💰 月付价格: " + priceText + "\n")
			msg.WriteString("💵 安装费用: " + formatInstallFeeLine(installText) + "\n")
			if firstMonthText != "" {
				msg.WriteString("🧾 首月合计: " + firstMonthText + "\n")
			}
		} else {
			msg.WriteString("💰 价格: " + priceText + "\n")
		}
	case priceErrorMessage != "":
		msg.WriteString("💰 价格: 未获取到（" + priceErrorMessage + "）\n")
		if installText != "" {
			msg.WriteString("💵 安装费用: " + formatInstallFeeLine(installText) + "\n")
		}
	default:
		msg.WriteString("💰 价格: 未获取到\n")
		if installText != "" {
			msg.WriteString("💵 安装费用: " + formatInstallFeeLine(installText) + "\n")
		}
	}

	if n := m.activeQueueCount(planCode); n > 0 {
		msg.WriteString(fmt.Sprintf("⚠️ 这个型号已经有 %d 个任务在抢了（发 /queue 查看）\n", n))
	}

	pushTime := m.nowBeijing()
	detected := pushTime
	if len(detectedTimes) > 0 {
		detected = detectedTimes[0]
		for _, t := range detectedTimes[1:] {
			if t.Before(detected) {
				detected = t
			}
		}
	}
	msg.WriteString(telegram.CardDivider + "\n")
	msg.WriteString("⏰ 检测时间: " + detected.Format("2006-01-02 15:04:05") + "\n")
	if lag := pushTime.Sub(detected); lag >= 30*time.Second {
		msg.WriteString(fmt.Sprintf("⚠️ 推送延迟: %.0f 秒\n", lag.Seconds()))
	}
	if traceLine := formatTraceBlock(traceID, configTraceID); traceLine != "" {
		msg.WriteString(traceLine + "\n")
	}

	activeCoupon := telegram.ActiveCoupon(m.state)
	if activeCoupon != "" {
		msg.WriteString("🎫 默认优惠码: " + activeCoupon + " (抢购就绪 · 按钮下单自动抵扣)\n")
	}
	firstDC := ""
	if len(availableDCs) > 0 {
		firstDC, _ = availableDCs[0]["dc"].(string)
	}
	cmdCopy := "/buy " + planCode
	if firstDC != "" {
		cmdCopy += " " + strings.ToLower(firstDC)
	}
	cmdCopy += " 1"
	if activeCoupon != "" {
		cmdCopy += " coupon=" + activeCoupon
	}
	msg.WriteString(telegram.CardDivider + "\n")
	msg.WriteString("💡 快捷下单指令 (点击即可复制):\n`" + cmdCopy + "`\n")

	// 构建按钮（每行最多 2 个）
	type btn struct {
		Text         string `json:"text"`
		CallbackData string `json:"callback_data"`
	}
	keyboard := [][]btn{}
	row := []btn{}
	options := []string{}
	if configInfo != nil {
		if opts, ok := configInfo["options"].([]string); ok {
			options = opts
		} else if optsRaw, ok := configInfo["options"].([]interface{}); ok {
			for _, o := range optsRaw {
				if s, ok := o.(string); ok {
					options = append(options, s)
				}
			}
		}
	}
	explicitAccounts := append([]string(nil), accountID...)
	if cfgAcc := accountIDFromConfig(configInfo); cfgAcc != "" {
		explicitAccounts = append(explicitAccounts, cfgAcc)
	}
	btnAccountID := m.resolveNotifyAccountID(planCode, explicitAccounts...)
	if btnAccountID == "" {
		// 不是错误:单账户用户、或订阅没勾自动下单时本来就没有账户维度。
		// 记一行是为了在"按钮下到了错误大区"的事故里能一眼看出按钮当时是无账户的。
		m.state.Logger.Debug("一键下单按钮无法解析账户归属，回调时将退回默认账户: "+planCode, "monitor")
	}

	targets := m.orderButtonTargets(planCode, availableDCs, btnAccountID)

	for idx, t := range targets {
		msgUUID := uuid.NewString()
		m.AddMessageUUID(msgUUID, planCode, t.dc, options, configInfo)
		// AddMessageUUID 只写不带账户的行,这里紧接着补写 account_id。
		// 失败只退回"默认账户"的老行为,不影响按钮可用。
		if t.accountID != "" && m.state.DB != nil {
			if err := m.state.DB.SetTelegramButtonAccount(msgUUID, t.accountID); err != nil {
				m.state.Logger.Warn("一键下单按钮账户归属落库失败（回调将退回默认账户）: "+err.Error(), "telegram")
			}
		}
		m.state.Logger.Debug(fmt.Sprintf("生成消息UUID: %s, 配置: %s@%s, options=%v, account=%s",
			msgUUID, planCode, t.dc, options, t.accountID), "monitor")

		cb := map[string]string{"a": "add_to_queue", "u": msgUUID}
		cbStr, _ := json.Marshal(cb)
		if len(cbStr) > 64 {
			m.state.Logger.Warn(fmt.Sprintf("UUID callback_data异常长: %d字节, UUID=%s", len(cbStr), msgUUID), "monitor")
		}
		row = append(row, btn{Text: t.label, CallbackData: string(cbStr)})
		if len(row) >= 2 || idx == len(targets)-1 {
			keyboard = append(keyboard, row)
			row = nil
		}
	}

	// 底部高阶操作栏（全节点一键挂机 / 切换账户）
	masterUUID := uuid.NewString()
	allDCs := make([]string, 0, len(availableDCs))
	for _, d := range availableDCs {
		if dcStr, ok := d["dc"].(string); ok && dcStr != "" {
			allDCs = append(allDCs, dcStr)
		}
	}
	allDCsJoined := strings.Join(allDCs, ",")
	m.AddMessageUUID(masterUUID, planCode, allDCsJoined, options, configInfo)
	if btnAccountID != "" && m.state.DB != nil {
		_ = m.state.DB.SetTelegramButtonAccount(masterUUID, btnAccountID)
	}

	hasMultiAccounts := m.state != nil && len(m.state.AccountsSnapshot()) > 1
	var actionRow []btn
	if len(availableDCs) > 1 {
		cbAll, _ := json.Marshal(map[string]string{"a": "queue_all", "u": masterUUID})
		actionRow = append(actionRow, btn{Text: "📥 全节点挂机入队", CallbackData: string(cbAll)})
	} else if hasMultiAccounts {
		cbQueue, _ := json.Marshal(map[string]string{"a": "queue_all", "u": masterUUID})
		actionRow = append(actionRow, btn{Text: "📥 加入抢购队列", CallbackData: string(cbQueue)})
	}
	if hasMultiAccounts {
		cbPick, _ := json.Marshal(map[string]string{"a": "pick_acc", "u": masterUUID})
		actionRow = append(actionRow, btn{Text: "👤 切换下单账户", CallbackData: string(cbPick)})
	}
	if len(actionRow) > 0 {
		keyboard = append(keyboard, actionRow)
	}

	replyMarkup := map[string]interface{}{"inline_keyboard": keyboard}
	return msg.String(), replyMarkup
}

// SendAvailabilityAlertGrouped 拼好通知并广播出去。
func (m *Monitor) SendAvailabilityAlertGrouped(planCode string, availableDCs []map[string]interface{},
	configInfo map[string]interface{}, serverName string, priceErrorMessage string, traceID, configTraceID string,
	accountID ...string) {
	msgText, replyMarkup := m.buildAvailabilityAlert(planCode, availableDCs, configInfo,
		serverName, priceErrorMessage, traceID, configTraceID, accountID...)

	// 上架通知按钮预热：广播通知的同时为每颗一键下单按钮预热就绪车,
	// 用户点按钮时直接消费现车结账,省掉装车时间。
	m.prewarmAlertButtons(planCode, availableDCs, configInfo, accountID...)

	configDesc := ""
	if configInfo != nil {
		if d, ok := configInfo["display"].(string); ok {
			configDesc = " [" + d + "]"
		}
	}
	m.state.Logger.Info(fmt.Sprintf("正在发送汇总Telegram通知: %s%s - %d个机房", planCode, configDesc, len(availableDCs)), "monitor")
	if notify.Broadcast(m.state, msgText, replyMarkup) > 0 {
		m.state.Logger.Info(fmt.Sprintf("✅ Telegram汇总通知发送成功: %s%s", planCode, configDesc), "monitor")
	} else {
		m.state.Logger.Warn(fmt.Sprintf("⚠️ Telegram汇总通知发送失败: %s%s", planCode, configDesc), "monitor")
	}
}

func (m *Monitor) buildUnavailableAlertGrouped(planCode string, unavailableDCs []map[string]interface{},
	configInfo map[string]interface{}, serverName, traceID, configTraceID string) string {

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("📦 服务器下架通知 · %s\n", planCode))
	msg.WriteString(telegram.CardDivider + "\n")
	if serverName != "" {
		msg.WriteString("🖥️ 产品名称: " + serverName + "\n")
	}
	summary, memH, diskH := formatConfigSpec(configInfo)
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
	msg.WriteString(telegram.CardDivider + "\n")
	msg.WriteString(fmt.Sprintf("🔴 已下架机房 (%d 个):\n", len(unavailableDCs)))
	for _, dcInfo := range unavailableDCs {
		dc, _ := dcInfo["dc"].(string)
		msg.WriteString("  • " + dcDisplayCN(dc) + " (" + strings.ToUpper(dc) + ")")
		if dt, ok := dcInfo["duration_text"].(string); ok && dt != "" {
			msg.WriteString(" - ⏱️ 上架持续: " + strings.TrimPrefix(dt, "历时 "))
		}
		msg.WriteString("\n")
	}
	msg.WriteString(telegram.CardDivider + "\n")
	msg.WriteString("⏰ 发生时间: " + m.nowBeijing().Format("2006-01-02 15:04:05") + "\n")
	if traceLine := formatTraceBlock(traceID, configTraceID); traceLine != "" {
		msg.WriteString(traceLine + "\n")
	}

	return msg.String()
}

func (m *Monitor) SendUnavailableAlertGrouped(planCode string, unavailableDCs []map[string]interface{},
	configInfo map[string]interface{}, serverName, traceID, configTraceID string) {

	msgText := m.buildUnavailableAlertGrouped(planCode, unavailableDCs, configInfo, serverName, traceID, configTraceID)

	configDesc := ""
	if configInfo != nil {
		if d, ok := configInfo["display"].(string); ok {
			configDesc = " [" + d + "]"
		}
	}
	m.state.Logger.Info(fmt.Sprintf("正在发送聚合下架Telegram通知: %s%s - %d个机房", planCode, configDesc, len(unavailableDCs)), "monitor")
	if notify.Broadcast(m.state, msgText, nil) > 0 {
		m.state.Logger.Info(fmt.Sprintf("✅ Telegram聚合下架通知发送成功: %s%s", planCode, configDesc), "monitor")
	} else {
		m.state.Logger.Warn(fmt.Sprintf("⚠️ Telegram聚合下架通知发送失败: %s%s", planCode, configDesc), "monitor")
	}
}
