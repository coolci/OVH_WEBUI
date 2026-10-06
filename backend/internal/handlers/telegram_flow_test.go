package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ovh-webui/server/internal/monitor"
)

// callback_data 有 64 字节硬限制，超了 Telegram **不报错**，按钮发出去就是点了没反应。
// 所以键盘里塞的只能是 token + 序号，绝不能是 addon 代码本身
// （ram-64g-noecc-2133 这种一个就二十来字符）。
func TestFlowCallbackDataFitsLimit(t *testing.T) {
	kb := flowKeyboard("a1b2c3d4", []string{
		"64GB DDR4 ECC 2133MHz + 2x480GB SSD SoftRaid  ✅3个机房有货",
		"🌐 全部配置（每套都盯）",
	})
	rows, ok := kb["inline_keyboard"].([][]map[string]string)
	if !ok {
		t.Fatalf("键盘结构不对: %T", kb["inline_keyboard"])
	}
	for _, row := range rows {
		for _, b := range row {
			if n := len(b["callback_data"]); n > 64 {
				t.Fatalf("callback_data %d 字节，超过 Telegram 的 64 上限：%s", n, b["callback_data"])
			}
			var m map[string]interface{}
			if err := json.Unmarshal([]byte(b["callback_data"]), &m); err != nil {
				t.Fatalf("callback_data 不是合法 JSON: %v", err)
			}
			if m["a"] != "wf" || m["t"] != "a1b2c3d4" {
				t.Fatalf("callback_data 内容不对: %v", m)
			}
		}
	}
}

// 流程状态会过期。过期后再点按钮不能当成有效选择继续走下去 ——
// 那会拿着一份用户早就忘了的半成品去建订阅。
func TestFlowExpires(t *testing.T) {
	f := &watchFlow{PlanCode: "24sk602", Step: stepConfig}
	tok := putFlow(f)
	if _, ok := getFlow(tok); !ok {
		t.Fatal("刚存进去应该取得到")
	}
	// 手动过期
	flowMu.Lock()
	flowStore[tok].Expires = flowStore[tok].Expires.Add(-flowTTL - 1)
	flowMu.Unlock()
	if _, ok := getFlow(tok); ok {
		t.Fatal("过期的流程不该还能取到")
	}
}

// 取走一次就作废：同一颗按钮连点两次不能建两条订阅。
func TestFlowTokenConsumedOnce(t *testing.T) {
	f := &watchFlow{PlanCode: "24sk602", Step: stepAction}
	tok := putFlow(f)
	if _, ok := getFlow(tok); !ok {
		t.Fatal("第一次应当取到")
	}
	dropFlow(tok)
	if _, ok := getFlow(tok); ok {
		t.Fatal("dropFlow 之后不该还能取到")
	}
}

// 配置按钮必须稳定排序：map 遍历顺序每次都不一样，
// 用户第二次 /watch 发现按钮换了位置，很容易点错成另一套配置。
func TestConfigLabelsStableAndCapped(t *testing.T) {
	configs := []configChoice{
		{Label: "32G + 2x480 SSD", InStock: 0},
		{Label: "64G + 2x480 SSD", InStock: 2},
	}
	labels := configLabels(configs)
	if len(labels) != 3 {
		t.Fatalf("两套配置 + 一个「全部」= 3 个按钮，实际 %d", len(labels))
	}
	if !strings.Contains(labels[1], "✅2个机房有货") {
		t.Fatalf("有货的要标出来: %v", labels)
	}
	if !strings.Contains(labels[2], "全部配置") {
		t.Fatalf("最后一个必须是「全部配置」: %v", labels)
	}

	// 超过上限要截断,否则键盘长到把消息挤没
	many := make([]configChoice, 20)
	for i := range many {
		many[i] = configChoice{Label: "cfg"}
	}
	if n := len(configLabels(many)); n != flowMaxButtons+1 {
		t.Fatalf("应截断到 %d+1 个按钮，实际 %d", flowMaxButtons, n)
	}
}

// 配置筛选用子集而不是相等：用户在 TG 上只挑内存和存储，
// 而一套配置的 options 还含带宽、vRack 之类他没挑的东西，要求相等会一个都匹配不上。
func TestConfigMatchesFilterIsSubset(t *testing.T) {
	have := []string{"ram-64g-noecc-2133", "softraid-2x480ssd", "bandwidth-500", "vrack-unlimited"}

	if !monitor.ConfigMatchesFilter(nil, have) {
		t.Fatal("空筛选 = 盯全部配置")
	}
	if !monitor.ConfigMatchesFilter([]string{"ram-64g-noecc-2133", "softraid-2x480ssd"}, have) {
		t.Fatal("子集应当匹配 —— 用户没挑的 addon 不该让匹配失败")
	}
	if monitor.ConfigMatchesFilter([]string{"ram-128g"}, have) {
		t.Fatal("不在里面的不该匹配")
	}
}

// 回归：Options 必须一路活到 Snapshot。
// 结构体加字段却漏掉某个转换器，是这个仓库反复踩到的一类 bug ——
// 表现是界面上开关还亮着，实际写入被静默丢弃。这次漏的是 snapshot()，
// 症状是流程里明明选了配置，/subs 却显示「全部」。
func TestWatchFlowOptionsSurviveSnapshot(t *testing.T) {
	st, mon := newWatchTestMonitor(t)
	addTestAccount(t, st)

	f := &watchFlow{
		PlanCode:      "24sk602",
		PickedConfig:  "64G + 2x480 SSD",
		PickedOptions: []string{"ram-64g", "softraid-2x480ssd"},
		AccountID:     "acc-test",
		AccountLabel:  "测试账户（IE）",
	}
	finishWatchFlow(st, mon, f, 1)

	subs := mon.Snapshot()
	if len(subs) != 1 {
		t.Fatalf("期望 1 条订阅，实际 %d", len(subs))
	}
	if len(subs[0].Options) != 2 || subs[0].Options[0] != "ram-64g" {
		t.Fatalf("选中的配置没活到 Snapshot：%+v", subs[0].Options)
	}
	// 也要活到给用户看的那段文字里
	if out := subsText(st, mon); !strings.Contains(out, "ram-64g") {
		t.Fatalf("/subs 应当显示选中的配置，实际：\n%s", out)
	}
}

// 没有任何账户时选了「自动抢」必须降级成只通知，
// 不能假装挂上了 —— 那样补货时什么都不会发生，用户还以为在抢。
func TestWatchFlowDegradesWithoutAccount(t *testing.T) {
	st, mon := newWatchTestMonitor(t)
	f := &watchFlow{PlanCode: "24sk602"}
	out := finishWatchFlow(st, mon, f, 2)
	if strings.Contains(out, "自动抢") {
		t.Fatalf("没有账户时不该说在自动抢：\n%s", out)
	}
	if subs := mon.Snapshot(); len(subs) != 1 || subs[0].AutoOrder {
		t.Fatalf("没有账户时 AutoOrder 必须是 false：%+v", subs)
	}
}
