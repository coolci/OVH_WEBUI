package monitor

import "testing"

// SetSubscriptionOptions 只能动 Options,别的字段一个都不许碰。
//
// 为什么单独测这个:改配置的另一条路是"删了重建"(AddSubscription 是 upsert),
// 那会清空 LastStatus —— 下一轮检查就把"本来就有货"当成 无货→有货 的跳变,
// 发一条根本没发生的补货通知,开了自动下单的还会真下单。
// 同理 AutoOrder / Quantity / AutoOrderAccountID / AutoPay 是用户的花钱设置,
// 调用方凑不齐这些字段时走 upsert 就等于把它们悄悄改掉。
func TestSetSubscriptionOptionsOnlyTouchesOptions(t *testing.T) {
	m := &Monitor{
		subscriptions: []*Subscription{{
			PlanCode:           "24sk602",
			Datacenters:        []string{"gra", "rbx"},
			Options:            []string{"old-addon"},
			NotifyAvailable:    true,
			AutoOrder:          true,
			Quantity:           3,
			AutoOrderAccountID: "acc-eu",
			AutoPay:            true,
			LastStatus:         map[string]string{"gra": "1H-low"},
			History:            []HistoryEntry{{Datacenter: "gra", Status: "1H-low"}},
		}},
	}

	want := []string{"ram-64g-ecc-2133-24sk60", "softraid-2x480ssd-24sk60"}
	if err := m.SetSubscriptionOptions("24sk602", "old-addon", want); err != nil {
		t.Fatalf("订阅存在却返回错误: %v", err)
	}
	s := m.subscriptions[0]
	if len(s.Options) != 2 || s.Options[0] != want[0] {
		t.Fatalf("Options 没改成: %v", s.Options)
	}
	// 花钱相关的设置必须原样
	if !s.AutoOrder || s.Quantity != 3 || s.AutoOrderAccountID != "acc-eu" || !s.AutoPay {
		t.Fatalf("自动下单设置被改动了: %+v", s)
	}
	// 状态与历史必须原样 —— 清掉会造成一条虚假的补货通知
	if len(s.LastStatus) != 1 || s.LastStatus["gra"] != "1H-low" {
		t.Fatalf("LastStatus 被动过: %v", s.LastStatus)
	}
	if len(s.History) != 1 {
		t.Fatalf("History 被动过: %v", s.History)
	}
	if len(s.Datacenters) != 2 {
		t.Fatalf("Datacenters 被动过: %v", s.Datacenters)
	}

	// 传空 = 改回"盯全部配置"
	if err := m.SetSubscriptionOptions("24sk602", OptionsKey(want), nil); err != nil {
		t.Fatalf("改回全部配置失败: %v", err)
	}
	if len(m.subscriptions[0].Options) != 0 {
		t.Fatalf("空 options 应清空,实际 %v", m.subscriptions[0].Options)
	}

	// 不存在的订阅要明确返回 false,让调用方能告诉用户"这条订阅已经没了"
	if err := m.SetSubscriptionOptions("不存在", "", want); err != ErrSubscriptionNotFound {
		t.Fatalf("不存在的订阅应返回 ErrSubscriptionNotFound, 实际 %v", err)
	}

	// 入参切片是调用方的,不能被内部持有 —— 否则调用方之后改它会悄悄改掉订阅
	_ = m.SetSubscriptionOptions("24sk602", "", want)
	want[0] = "被调用方改掉了"
	if m.subscriptions[0].Options[0] == "被调用方改掉了" {
		t.Fatal("内部直接持有了调用方的切片,应该拷贝一份")
	}
}

// 改窄配置不能和同型号另一条订阅撞成同一个配置键(库表主键是 plan_code+options),
// 只改原配置键对应的那条,并且要让进行中的检查失效。
func TestSetSubscriptionOptionsGuardsIdentity(t *testing.T) {
	m := &Monitor{subscriptions: []*Subscription{
		{PlanCode: "p", Options: []string{"ram-a"}},
		{PlanCode: "p", Options: []string{}},
	}}
	if err := m.SetSubscriptionOptions("p", "", []string{"ram-a"}); err != ErrSubscriptionConflict {
		t.Fatalf("撞配置键应返回冲突, 实际 %v", err)
	}
	if len(m.subscriptions[1].Options) != 0 {
		t.Fatalf("冲突时不能改动: %v", m.subscriptions[1].Options)
	}
	before := m.subscriptions[1].revision
	if err := m.SetSubscriptionOptions("p", "", []string{"ram-b"}); err != nil {
		t.Fatal(err)
	}
	if m.subscriptions[0].Options[0] != "ram-a" || m.subscriptions[1].Options[0] != "ram-b" {
		t.Fatal("改错了订阅")
	}
	if m.subscriptions[1].revision == before {
		t.Fatal("配置变了 revision 必须递增,旧一轮检查不能再凭旧配置下单")
	}
}
