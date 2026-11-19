package purchase

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/notify"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

func recordSuccess(state *app.State, item *types.QueueItem, orderID, orderURL, expirationTime string, priceInfo *types.PriceInfo, couponWarning ...string) {
	// 锁内只改内存,SaveHistory 在锁外 —— 它自己会拿 HistoryMu(快照),
	// Go mutex 不可重入,持锁同步调它(c419176 引入)首次下单成功即死锁全队列
	state.HistoryMu.Lock()
	defer func() {
		state.HistoryMu.Unlock()
		_ = state.SaveHistory()
	}()
	now := types.NowISO()
	warn := ""
	if len(couponWarning) > 0 {
		warn = couponWarning[0]
	}

	for i := range state.History {
		if state.History[i].TaskID == item.ID {
			state.History[i].Status = "success"
			state.History[i].AccountID = item.AccountID
			state.History[i].Duration = item.Duration
			state.History[i].OrderID = orderID
			state.History[i].OrderURL = orderURL
			state.History[i].ErrorMessage = nil
			state.History[i].CouponWarning = warn
			state.History[i].PurchaseTime = now
			state.History[i].AttemptCount = item.RetryCount
			state.History[i].Options = item.Options
			if expirationTime != "" {
				state.History[i].ExpirationTime = expirationTime
			}
			if priceInfo != nil {
				state.History[i].Price = priceInfo
			}
			state.Logger.Info("更新抢购历史(成功) 任务ID: "+item.ID, "purchase")
			return
		}
	}

	entry := types.PurchaseHistoryEntry{
		ID:            uuid.NewString(),
		TaskID:        item.ID,
		AccountID:     item.AccountID,
		Duration:      item.Duration,
		PlanCode:      item.PlanCode,
		Datacenter:    item.Datacenter,
		Options:       item.Options,
		Status:        "success",
		OrderID:       orderID,
		OrderURL:      orderURL,
		CouponWarning: warn,
		PurchaseTime:  now,
		AttemptCount:  item.RetryCount,
	}
	if expirationTime != "" {
		entry.ExpirationTime = expirationTime
	}
	if priceInfo != nil {
		entry.Price = priceInfo
	}
	state.History = append(state.History, entry)
	state.Logger.Info("创建抢购历史(成功) 任务ID: "+item.ID, "purchase")
}

func recordFailure(state *app.State, item *types.QueueItem, errMsg string) {
	state.HistoryMu.Lock()
	defer func() {
		state.HistoryMu.Unlock()
		_ = state.SaveHistory()
	}()
	now := types.NowISO()

	for i := range state.History {
		if state.History[i].TaskID == item.ID {
			state.History[i].Status = "failed"
			state.History[i].AccountID = item.AccountID
			state.History[i].Duration = item.Duration
			state.History[i].OrderID = ""
			state.History[i].OrderURL = ""
			em := errMsg
			state.History[i].ErrorMessage = &em
			state.History[i].PurchaseTime = now
			state.History[i].AttemptCount = item.RetryCount
			state.History[i].Options = item.Options
			state.Logger.Info("更新抢购历史(失败) 任务ID: "+item.ID, "purchase")
			return
		}
	}
	em := errMsg
	entry := types.PurchaseHistoryEntry{
		ID:           uuid.NewString(),
		TaskID:       item.ID,
		AccountID:    item.AccountID,
		Duration:     item.Duration,
		PlanCode:     item.PlanCode,
		Datacenter:   item.Datacenter,
		Options:      item.Options,
		Status:       "failed",
		ErrorMessage: &em,
		PurchaseTime: now,
		AttemptCount: item.RetryCount,
	}
	state.History = append(state.History, entry)
	state.Logger.Info("创建抢购历史(失败) 任务ID: "+item.ID, "purchase")
}

// orderDetailRetryDelays 查订单详情失败后的重试间隔(测试可缩短)
var orderDetailRetryDelays = []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second, 15 * time.Second, 30 * time.Second}

// backfillOrderDetail 下单成功后异步补 history 行的 expirationTime + price。
// 不阻塞 PurchaseServer 主流程,即便这一步失败 history 也已经标 success(只是少了价格 / 过期时间)。
// 在独立 goroutine 跑,持有 OVH client 引用,只 read /me/order/{orderID}。
//
// 付款截止时间(expirationDate)只有这里会写。促销订单可能 15 分钟不付就作废,
// 而下单那一刻正是 OVH 最容易 429 的时候 —— 只查一次、失败就放弃的话,
// 历史里就没有截止时间,用户只能干等。所以失败按退避重试,总共约一分钟。
func backfillOrderDetail(state *app.State, client *ovhsdk.Client, taskID, orderID string) {
	var orderInfo map[string]interface{}
	var lastErr error
	for attempt := 0; attempt <= len(orderDetailRetryDelays); attempt++ {
		if attempt > 0 {
			time.Sleep(orderDetailRetryDelays[attempt-1])
		}
		orderInfo = nil
		if lastErr = client.Get("/me/order/"+orderID, &orderInfo); lastErr == nil {
			break
		}
		state.Logger.Warn(fmt.Sprintf("异步查询订单 %s 详情失败(第 %d 次): %s", orderID, attempt+1, lastErr.Error()), "purchase")
	}
	if lastErr != nil {
		state.Logger.Error(fmt.Sprintf("订单 %s 详情多次查询失败,付款截止时间未知 —— 请尽快到 OVH 订单页付款", orderID), "purchase")
		return
	}

	// billing.Order 里 expirationDate（订单待付款到期作废时间）与 retractionDate
	// （法定撤销权截止日）是两个语义完全不同的 datetime，历史里展示的"过期时间"
	// 指的是前者，拿 retractionDate 当付款截止时间会让用户误判付款窗口。
	//
	// 注意 retractionDate 现在是有意义的:checkout 不再传 waiveRetractationPeriod,
	// 撤回权没被放弃,这个日期就是"在此之前还能申请无理由撤单"的真实截止时间。
	// 而这个 fix 之前下的单都弃权了,OVH 对它们不返回 retractionDate ——
	// 前端据此判断要不要显示撤回入口,不用自己算 14 天(算了就会给老订单
	// 显示一个点了必然失败的按钮)。
	expirationTime := ""
	if exp, ok := orderInfo["expirationDate"].(string); ok && exp != "" {
		expirationTime = exp
	}
	// 撤销权截止日单独存 RetractionTime,不混进 ExpirationTime(付款到期),
	// 两者语义不同,混用会让用户误判付款窗口
	retractionTime := ""
	if ret, ok := orderInfo["retractionDate"].(string); ok && ret != "" {
		retractionTime = ret
		state.Logger.Debug(fmt.Sprintf("订单 %s 撤销权截止日: %s", orderID, ret), "purchase")
	}

	// /me/order 返回的价格字段:priceWithTax / priceWithoutTax / tax,
	// 每个典型形式 { value: <number-or-json.Number>, currencyCode: string, text: string }。
	// 复用 extract(),它能容 float64 / json.Number / string / int 各种来法。
	pickCurrency := func(field interface{}) string {
		m, ok := field.(map[string]interface{})
		if !ok {
			return ""
		}
		c, _ := m["currencyCode"].(string)
		return c
	}

	var priceInfo *types.PriceInfo
	withTax := extract(orderInfo["priceWithTax"])
	withoutTax := extract(orderInfo["priceWithoutTax"])
	tax := extract(orderInfo["tax"])
	currency := pickCurrency(orderInfo["priceWithTax"])
	if currency == "" {
		currency = pickCurrency(orderInfo["priceWithoutTax"])
	}
	if currency == "" {
		currency = pickCurrency(orderInfo["tax"])
	}
	// 以前这里兜底写死 "EUR"。币种是按子公司定的,不是按站点:实测公开目录 locale.currencyCode
	// IE=EUR / CA=QC=CAD / US=WE=WS=USD / SG=SGD / AU=AUD ——
	// 给一张美元账单盖上 EUR 会直接误导花了多少钱。宁可留空让上层显示"未知"。
	if currency == "" {
		state.Logger.Warn(fmt.Sprintf("订单 %s 的价格里没有 currencyCode，币种留空（不再默认 EUR：美区/加区/亚太子公司都不是欧元计价）", orderID), "purchase")
	}
	if withTax != nil || withoutTax != nil {
		priceInfo = &types.PriceInfo{
			WithTax:      withTax,
			WithoutTax:   withoutTax,
			Tax:          tax,
			CurrencyCode: currency,
		}
	}

	// 支付状态也顺手读一次(之后由 OrderStatusLoop 定时刷)。
	// 拿到的多半是 notPaid —— 但"明确的待付款"和"什么都不知道"对用户是两回事
	orderStatus := ""
	if st, err := FetchOrderStatus(client, orderID); err != nil {
		state.Logger.Warn(fmt.Sprintf("查询订单 %s 状态失败: %s", orderID, err.Error()), "purchase")
	} else {
		orderStatus = st
	}

	if expirationTime == "" && priceInfo == nil && orderStatus == "" {
		return
	}

	state.HistoryMu.Lock()
	// defer 先 Unlock 再 Save:SaveHistory 内部拿 HistoryMu(快照),
	// 持锁调它 = 同 goroutine 不可重入死锁(跟 recordSuccess 同型 bug)
	defer func() {
		state.HistoryMu.Unlock()
		_ = state.SaveHistory()
	}()
	for i := range state.History {
		if state.History[i].TaskID != taskID {
			continue
		}
		changed := false
		if orderStatus != "" {
			state.History[i].OrderStatus = orderStatus
			state.History[i].OrderStatusAt = types.NowISO()
			changed = true
		}
		if expirationTime != "" && state.History[i].ExpirationTime != expirationTime {
			state.History[i].ExpirationTime = expirationTime
			changed = true
		}
		if retractionTime != "" && state.History[i].RetractionTime != retractionTime {
			state.History[i].RetractionTime = retractionTime
			changed = true
		}
		if priceInfo != nil && state.History[i].Price == nil {
			state.History[i].Price = priceInfo
			changed = true
		}
		if changed {
			state.Logger.Info(fmt.Sprintf("补全订单 %s 详情: 过期时间=%q 价格=%v",
				orderID, expirationTime, priceInfo != nil), "purchase")
		}
		return
	}
}

// BuildOrderSuccessMessage 抢购成功通知的正文。
//
// 抽出来是因为这是整个系统里唯一一条"你花钱了"的消息 ——
// 措辞错一个字用户就可能少付一笔钱或者多买一台。要能单独看、单独测。
//
// 关键在于"成功"两个字的真实语义:checkout 返回订单号就算成功,
// 但订单是**未付款**的,逾期会自动作废。不把这句写出来,
// 用户看到 🎉 就睡了,第二天机器没了还以为是我们没抢到。
func BuildOrderSuccessMessage(item *types.QueueItem, orderID, managerURL string, tls ...*timeline) string {
	payNote := "⚠️ 订单尚未付款：请尽快打开订单链接完成付款，逾期未付订单会自动作废。\n" +
		"（未付款订单不支持直接撤回，可在购买历史中放弃并等待到期作废）"
		// if item.AutoPay {
		// // 只承诺我们真正知道的:已请求自动付款 ≠ 扣款一定成功
		// // (默认支付方式失效/余额不足时 OVH 不会扣成),让用户去核对
		// // 结账 payload 不传 waiveRetractationPeriod，自动付款也不会替用户弃权。
		// payNote = "💳 已请求用账户默认支付方式自动付款，请打开订单链接核对扣款是否成功。\n" +
		// "（本次结账未放弃 14 天撤销期，可在 OVH 订单页撤回）"
		// }
	// 发控制面板深链,不发 checkout 返回的那个 url ——
	// 后者是带凭证的下载链接(OVH 的 billing.Order 里 url 旁边就是 password),
	// 而这条消息会进 Telegram 群和用户配的任意 webhook。
	// 带凭证那份仍然存在本地历史里,界面上照样一键可付。
	linkURL := managerURL
	if linkURL == "" {
		linkURL = "请在 OVH 控制面板 → 账单 → 订单 中查看"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("🎉 OVH 服务器下单成功 · %s\n", item.PlanCode))
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("📍 交付机房: " + telegram.DisplayDCFull(item.Datacenter) + "\n")
	if len(item.Options) > 0 {
		b.WriteString("⚙️ 硬件规格: " + telegram.HumanizeOptionCodes(item.Options) + "\n")
	}
	b.WriteString("🧾 订单编号: " + orderID + "\n")
	b.WriteString("🔗 订单链接: " + linkURL + "\n")

	// 耗时指标明细（如有）
	if len(tls) > 0 && tls[0] != nil {
		if metrics := tls[0].FormatMetrics(); metrics != "" {
			b.WriteString(telegram.CardDivider + "\n")
			b.WriteString(metrics + "\n")
		}
	}

	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString(payNote + "\n")
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("🆔 任务编号: " + item.ID)
	return b.String()
}

// handleReconciledOrder 对账发现订单已建:按成功路径走完(记成功/异步补详情/通知)。
// 以前这里只改 history,调用方注释写着会补 backfill,实际谁都没调;
// 用户看不到“已经锁单”的通知,队列却显示完成。
func handleReconciledOrder(state *app.State, client *ovhsdk.Client, endpoint string, item *types.QueueItem, orderID, orderURL string, tl *timeline, timingKey string, couponWarning ...string) Outcome {
	warn := ""
	if len(couponWarning) > 0 {
		warn = couponWarning[0]
	}
	recordSuccess(state, item, orderID, orderURL, "", nil, warn)
	recordTimingToHistory(state, item.ID, tl)
	state.Logger.Info("[耗时] "+item.PlanCode+"@"+item.Datacenter+" 对账确认成功 "+tl.String(), "purchase")
	if orderID != "" && client != nil {
		go backfillOrderDetail(state, client, item.ID, orderID)
	}
	payURL := ovh.ManagerOrderURL(endpoint, orderID)
	extra := map[string]string{
		"orderId":  orderID,
		"orderUrl": payURL,
	}
	if tl != nil {
		extra["timing"] = tl.FormatMetrics()
	}
	telegram.NotifyTaskProgress(state, item, "success", extra)
	msg := BuildOrderSuccessMessage(item, orderID, payURL, tl)
	if warn != "" {
		msg += "\n\n⚠️ " + warn
	}
	if notify.Broadcast(state, msg, nil) > 0 {
		state.Logger.Info("已为对账订单 "+orderID+" 发送成功通知。", "purchase")
	} else {
		state.Logger.Warn("对账订单 "+orderID+" 已确认,但成功通知未送达任何通道。", "purchase")
	}
	return Outcome{Success: true}
}
