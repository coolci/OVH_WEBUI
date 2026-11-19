package vps

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/domain/datacenter"
	"github.com/ovh-webui/server/internal/notify"
	"github.com/ovh-webui/server/internal/orderterm"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

// recordVPSPurchase 把下单结果写进抢购历史,和独服共用一张表 ——
// 用户不关心"这是独服还是 VPS 的历史",只关心"我到底买到了什么"。
// 同步落库:120 秒防抖闸门和"这条订阅已经买到过"的判断都靠这份历史,
// 只留在内存里的话,下单后崩溃重启就会对同一次补货再买一台。
func recordVPSPurchase(state *app.State, sub types.VPSSubscription, dcCode string, out Outcome) error {
	entry := types.PurchaseHistoryEntry{
		ID:           uuid.NewString(),
		TaskID:       "vps:" + sub.ID,
		AccountID:    sub.AutoOrderAccountID,
		PlanCode:     sub.PlanCode,
		Duration:     orderterm.Default,
		Datacenter:   dcCode,
		Options:      []string{},
		PurchaseTime: types.NowISO(),
		AttemptCount: 1,
	}
	if out.Success {
		entry.Status = "success"
		entry.OrderID = out.OrderID
		entry.OrderURL = out.OrderURL
	} else {
		entry.Status = "failed"
		reason := out.Reason
		entry.ErrorMessage = &reason
	}
	state.HistoryMu.Lock()
	state.History = append(state.History, entry)
	state.HistoryMu.Unlock()
	return state.SaveHistory()
}

// subscriptionCurrent 订阅仍存在,且自动下单相关配置与快照一致。
// 补货检查和下单之间用户可能删了订阅、关了自动下单、换了账户或机型 ——
// 那时继续下单就是替用户做了他刚刚取消的决定。
func subscriptionCurrent(state *app.State, snapshot types.VPSSubscription) bool {
	live, ok := state.FindVPSSubscription(snapshot.ID)
	if !ok || live == nil {
		return false
	}
	return live.AutoOrder && live.AutoOrderAccountID == snapshot.AutoOrderAccountID &&
		live.PlanCode == snapshot.PlanCode && effectiveSubsidiary(state, *live) == effectiveSubsidiary(state, snapshot) &&
		stringSliceEqual(live.Datacenters, snapshot.Datacenters) &&
		live.MonitorLinux == snapshot.MonitorLinux && live.MonitorWindows == snapshot.MonitorWindows &&
		live.OS == snapshot.OS && live.Quantity == snapshot.Quantity && live.AutoPay == snapshot.AutoPay
}

// effectiveSubsidiary 老订阅可能没存子公司,监控循环会按下单账户推一个;比较时两边用同一口径
func effectiveSubsidiary(state *app.State, sub types.VPSSubscription) string {
	if s := NormalizeSubsidiary(sub.OvhSubsidiary); s != "" {
		return s
	}
	return DefaultSubsidiary(state, sub.AutoOrderAccountID)
}

// disableAutoOrder 关掉这条订阅的自动下单(落库)。成功、结果不确定、确定性失败之后都调用:
// 订阅的语义是"补货时替我抢一次",不是"每次补货都再买一遍"。
func disableAutoOrder(state *app.State, sub types.VPSSubscription, reason string) {
	_, _, err := state.MutateVPSSubscription(sub.ID, func(s *types.VPSSubscription) bool {
		if !s.AutoOrder && !s.AutoPay {
			return false
		}
		s.AutoOrder, s.AutoPay = false, false
		return true
	})
	if err != nil {
		state.Logger.Error("保存 VPS 自动下单停止状态失败: "+err.Error(), "vps_purchase")
		return
	}
	state.Logger.Info(fmt.Sprintf("[VPS下单] 已关闭订阅 %s(%s)的自动下单: %s", sub.ID, sub.PlanCode, reason), "vps_purchase")
}

// saveObservation 下单前先把这一轮的库存观察(LastStatus / History)落库。
// 不先存的话:下单成功 → 进程在本轮末尾保存前崩溃 → 重启后基准还是"无货",
// 同一次补货再被判成"无货→有货",又买一台。
func saveObservation(state *app.State, sub *types.VPSSubscription) error {
	return saveObservationWithBaseline(state, sub, nil)
}

func saveObservationWithBaseline(state *app.State, sub *types.VPSSubscription, base *vpsSubBaseline) error {
	history := sub.History
	if len(history) > 100 {
		history = history[len(history)-100:]
	}
	var stale bool
	_, found, err := state.MutateVPSSubscription(sub.ID, func(s *types.VPSSubscription) bool {
		if base != nil && (base.Stale || !baselineMatchesLiveLocked(s, base)) {
			markBaselineStaleLocked(s, base)
			stale = true
			return false
		}
		s.LastStatus = make(map[string]string, len(sub.LastStatus))
		for k, v := range sub.LastStatus {
			s.LastStatus[k] = v
		}
		s.History = append([]map[string]interface{}(nil), history...)
		if base != nil {
			base.ExpectedStatus = copyStringMap(s.LastStatus)
		}
		return true
	})
	if err != nil {
		return err
	}
	if stale {
		return errStaleSubscriptionRevision
	}
	if !found {
		return fmt.Errorf("订阅已被删除")
	}
	return nil
}

// autoOrderOnRestock 补货时自动下单。
// 只对"从无货变成有货"的机房下单 —— 首次检查发现的存量不算补货,
// 那可能是一台挂了几个月的订阅刚启动,用户并没打算现在就买。
// 抢到一次就关掉自动下单;确定性失败或结果不确定也关掉,等用户确认后再手动打开。
func autoOrderOnRestock(state *app.State, sub types.VPSSubscription, dcs []map[string]interface{}) {
	if !sub.AutoOrder || strings.TrimSpace(sub.AutoOrderAccountID) == "" || !subscriptionCurrent(state, sub) {
		return
	}
	// 同独服 quick-order 的 120 秒近期成功闸门:VPS 库存自然抖动
	// (有货→抢光→再放货)每个回摆都会重新触发无货→有货跳变,
	// 不挡的话同一窗口反复买 quantity 台直到用户肉眼发现。
	// 另外:这条订阅只要成功买到过一次,就不再自动下单(关闭开关并返回)。
	state.HistoryMu.Lock()
	nowTS := time.Now().Unix()
	alreadyBought := false
	recent := false
	for i := len(state.History) - 1; i >= 0; i-- {
		h := &state.History[i]
		if h.Status != "success" {
			continue
		}
		if h.TaskID == "vps:"+sub.ID && h.OrderID != "" {
			alreadyBought = true
			break
		}
		if h.PlanCode == sub.PlanCode && h.AccountID == sub.AutoOrderAccountID {
			if t, ok := types.ParseTS(h.PurchaseTime); ok && nowTS-t.Unix() < 120 {
				recent = true
				break
			}
		}
	}
	state.HistoryMu.Unlock()
	if alreadyBought {
		disableAutoOrder(state, sub, "该订阅此前已成功下单")
		return
	}
	if recent {
		state.Logger.Info("[VPS下单] 同配置 120 秒内已成功下过单,跳过本次补货(防抖动重复下单): "+sub.PlanCode, "vps_purchase")
		return
	}
	for _, dc := range dcs {
		if !subscriptionCurrent(state, sub) {
			state.Logger.Info("[VPS下单] 订阅在下单过程中被修改或删除,停止本次自动下单: "+sub.PlanCode, "vps_purchase")
			return
		}
		code := restockOrderDatacenter(dc)
		if code == "" {
			continue
		}
		out := PurchaseVPS(state, sub, code)
		if err := recordVPSPurchase(state, sub, code, out); err != nil {
			state.Logger.Error("保存 VPS 下单结果失败,已停止自动下单: "+err.Error(), "vps_purchase")
			out.Fatal = true
			out.Reason += "；结果保存失败,自动下单已停止,请人工核对订单"
		}
		if out.Success || out.Fatal {
			disableAutoOrder(state, sub, out.Reason)
		}

		if out.Success {
			// 同独服:checkout 默认 autoPayWithPreferredPaymentMethod:false,
			// "成功"= 订单已创建、未付款、逾期作废。通知里必须说清楚。
			payNote := "⚠️ 订单尚未付款：请尽快打开订单链接完成付款,逾期未付订单会自动作废。\n" +
				"(未付款订单可在购买历史中放弃，等待到期作废)"
				// if sub.AutoPay {
				// payNote = "💳 已请求用账户默认支付方式自动付款,请打开订单链接核对扣款是否成功。\n" +
				// "(未付款订单可在购买历史中放弃，等待到期作废)"
				// }
			// 同独服:通知里发需要登录的控制面板深链,不发 checkout 那个带凭证的 url。
			// 带凭证那份存在本地历史里,界面上照样一键可付。
			linkURL := "请在 OVH 控制台 → 账单 → 订单 中查看"
			if acc, ok := state.FindAccount(sub.AutoOrderAccountID); ok {
				if u := ovh.ManagerOrderURL(acc.Endpoint, out.OrderID); u != "" {
					linkURL = u
				}
			}
			msg := fmt.Sprintf("🎉 VPS 下单成功\n\n型号: %s\n机房: %s\n数量: %d\n订单: %s\n%s\n\n%s\n\n该订阅的自动下单已自动关闭,如需再抢请到 VPS 监控页重新打开。",
				sub.PlanCode, code, max(sub.Quantity, 1), out.OrderID, linkURL, payNote)
			notify.Broadcast(state, msg, nil)
			// 抢到就停:订阅是"盯着补货",不是"把所有机房都买一遍"
			return
		}
		state.Logger.Error(fmt.Sprintf("[VPS下单] %s @ %s 失败: %s", sub.PlanCode, code, out.Reason), "vps_purchase")
		tail := ""
		if out.Fatal {
			tail = "\n\n该订阅的自动下单已关闭,请核对后到 VPS 监控页重新打开。"
		}
		notify.Broadcast(state, fmt.Sprintf("⚠️ VPS 自动下单失败\n\n型号: %s\n机房: %s\n原因: %s%s",
			sub.PlanCode, code, out.Reason, tail), nil)
		if out.Fatal {
			// 确定性失败 / 结果不确定:换个机房也是同样结果,或者可能已经买到,别再刷了
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// restockOrderDatacenter 取下单用的机房名。可用性接口每个机房给两个名字:
// code 是 eu-west-lim 这种新长码,datacenter(这里存成 name)是 DE / GRA / EU-WEST-LZ-AMS 这种短名。
// 购物车的 vps_datacenter 只认短名 —— 交长码 checkout 直接 400
// "Invalid VPS datacenter 'EU-WEST-LIM'"。LIM→DE、ERI→UK、MUM→YNM
// 这几个连大写都对不上,只能用 OVH 自己给的那个字段。
func restockOrderDatacenter(dc map[string]interface{}) string {
	if name, _ := dc["name"].(string); strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	code, _ := dc["code"].(string)
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	if short := datacenter.VPSShortCode(code); short != "" {
		return short
	}
	return code
}
