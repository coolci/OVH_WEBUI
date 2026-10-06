package telegram

import (
	"encoding/json"
	"testing"
)

// update_id 从 JSON 解出来是 float64。解错就等于 offset 不推进 ——
// 那会让同一批 update 被无限重放,每一轮都重新下一次单。
func TestParseUpdateIDAny(t *testing.T) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(`{"update_id": 123456789}`), &m); err != nil {
		t.Fatal(err)
	}
	if got := parseUpdateIDAny(m["update_id"]); got != 123456789 {
		t.Fatalf("JSON 里的 update_id 应解成 123456789，实际 %d", got)
	}
	if got := parseUpdateIDAny(int64(42)); got != 42 {
		t.Fatalf("int64 应原样返回，实际 %d", got)
	}
	if got := parseUpdateIDAny("nope"); got != 0 {
		t.Fatalf("非数字应返回 0，实际 %d", got)
	}
	if got := parseUpdateIDAny(nil); got != 0 {
		t.Fatalf("nil 应返回 0，实际 %d", got)
	}
}

// 键盘重建:只改按过的那一颗,其余机房的按钮必须原样留着 ——
// 用户很可能想在同一条通知里多买几个机房。
func TestRebuildKeyboardMarksOnlyPressed(t *testing.T) {
	kb := [][]map[string]interface{}{
		{
			{"text": "GRA 一键下单", "callback_data": `{"a":"add_to_queue","u":"uuid-gra"}`},
			{"text": "RBX 一键下单", "callback_data": `{"a":"add_to_queue","u":"uuid-rbx"}`},
		},
		{
			{"text": "SBG 一键下单", "callback_data": `{"a":"add_to_queue","u":"uuid-sbg"}`},
		},
	}
	StashKeyboard(int64(99), 5, kb)

	got := rebuildKeyboard(nil, int64(99), 5, `{"a":"add_to_queue","u":"uuid-rbx"}`, "✅ RBX 已下单")
	if got == nil {
		t.Fatal("命中的按钮应当返回新键盘")
	}
	if got[0][1]["text"] != "✅ RBX 已下单" {
		t.Fatalf("按过的那颗没改成已下单: %v", got[0][1]["text"])
	}
	if got[0][0]["text"] != "GRA 一键下单" || got[1][0]["text"] != "SBG 一键下单" {
		t.Fatal("其余机房的按钮被改动了，用户就没法在同一条通知里买第二个机房")
	}
}

// 取走一次就没了:同一条消息的键盘不该被后续回调重复消费。
func TestRebuildKeyboardConsumesOnce(t *testing.T) {
	kb := [][]map[string]interface{}{{{"text": "A", "callback_data": "x"}}}
	StashKeyboard(int64(1), 1, kb)
	if got := rebuildKeyboard(nil, int64(1), 1, "x", "✅"); got == nil {
		t.Fatal("第一次应当命中")
	}
	if got := rebuildKeyboard(nil, int64(1), 1, "x", "✅"); got != nil {
		t.Fatal("第二次应当为空（已被取走）")
	}
}

// 没记过键盘 / callback_data 对不上时返回 nil，
// 让调用方跳过编辑，而不是把一个空键盘推上去把所有按钮抹掉。
func TestRebuildKeyboardMissHandling(t *testing.T) {
	if got := rebuildKeyboard(nil, int64(7), 7, "whatever", "✅"); got != nil {
		t.Fatal("没记过键盘应返回 nil")
	}
	StashKeyboard(int64(8), 8, [][]map[string]interface{}{{{"text": "A", "callback_data": "aaa"}}})
	if got := rebuildKeyboard(nil, int64(8), 8, "bbb", "✅"); got != nil {
		t.Fatal("callback_data 对不上应返回 nil，不能把键盘清空")
	}
}
