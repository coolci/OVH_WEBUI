package handlers

import (
	"encoding/json"
	"testing"

	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

func tgMessage(chatID int64, chatType string, userID int64, username string) map[string]interface{} {
	return map[string]interface{}{
		"update_id": json.Number("1001"),
		"message": map[string]interface{}{
			"message_id": json.Number("7"),
			"text":       "/tasks",
			"chat":       map[string]interface{}{"id": json.Number(jsonInt(chatID)), "type": chatType},
			"from":       map[string]interface{}{"id": json.Number(jsonInt(userID)), "username": username},
		},
	}
}

func jsonInt(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// "Bot 注册了但不回话":未授权的消息以前只打一行不带 ID 的日志,用户不知道该填什么。
// 现在每种拒绝都带原因,并把发送者记进长轮询状态,设置页据此提示该填的 Chat ID。
func TestTelegramRejectionReasonsAreReported(t *testing.T) {
	cases := []struct {
		name     string
		cfgChat  string
		update   map[string]interface{}
		reason   string
		wantChat string
	}{
		{"没填 Chat ID", "", tgMessage(5550001, "private", 5550001, "alice"), telegram.RejectNoChatID, "5550001"},
		{"Chat ID 填错", "123", tgMessage(5550001, "private", 5550001, "alice"), telegram.RejectChatMismatch, "5550001"},
		{"群聊缺白名单", "-1001234567890", tgMessage(-1001234567890, "supergroup", 5550001, "alice"), telegram.RejectGroupNotAllowed, "-1001234567890"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TG_ALLOWED_USER_IDS", "")
			st := newTestState(t)
			// 没有 Token:不会真的往 Telegram 发提示消息
			_ = st.Config.Set(types.Config{TgChatID: tc.cfgChat})
			telegram.ClearRejected()
			ProcessTelegramUpdate(st, nil, tc.update)
			got := telegram.SnapshotPoller().LastRejected
			if got == nil {
				t.Fatal("未授权消息应记录到长轮询状态")
			}
			if got.Reason != tc.reason || got.ChatID != tc.wantChat || got.UserID != "5550001" || got.Username != "alice" {
				t.Fatalf("记录不对: %+v", got)
			}
		})
	}
}

func TestTelegramAuthorizedPrivateChatIsNotRejected(t *testing.T) {
	st := newTestState(t)
	_ = st.Config.Set(types.Config{TgChatID: "5550001"})
	ok, reason := telegram.AuthorizeActor(st, json.Number("5550001"), json.Number("5550001"))
	if !ok || reason != "" {
		t.Fatalf("配置的私聊应放行: ok=%v reason=%q", ok, reason)
	}
	t.Setenv("TG_ALLOWED_USER_IDS", "5550001")
	_ = st.Config.Set(types.Config{TgChatID: "-1001234567890"})
	ok, _ = telegram.AuthorizeActor(st, json.Number("-1001234567890"), json.Number("5550001"))
	if !ok {
		t.Fatal("群聊 + 白名单里的发送者应放行")
	}
}
