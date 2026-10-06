package handlers

import (
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

func newTGTestState(t *testing.T) *app.State {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	lg := logger.New(filepath.Join(dir, "t.log"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	// 不配 Token —— SendReply 会直接 return，测试不打网络
	return app.NewState(storage.Paths{DataDir: dir}, config.New(database), lg, database)
}

// 命令必须和下单文本分得干净:把 "24sk602 gra 2" 当成命令会让下单失效,
// 把 "/help" 当成型号会让 bot 去抢一台叫 /help 的机器。
func TestHandleCommandOnlyClaimsSlashMessages(t *testing.T) {
	s := newTGTestState(t)
	cases := []struct {
		text    string
		isCmd   bool
		comment string
	}{
		{"/help", true, ""},
		{"/status", true, ""},
		{"/queue", true, ""},
		{"/cancel all", true, ""},
		{"/help@my_ovh_bot", true, "群里命令会带 @botname"},
		{"/HELP", true, "命令大小写不敏感"},
		{"/nonsense", true, "不认识的命令也要接住并回提示，不能落到下单解析"},
		{"24sk602", false, "这是下单文本"},
		{"24sk602 gra 2", false, "这是下单文本"},
		{"hello", false, ""},
		{"", false, ""},
	}
	for _, c := range cases {
		got := handleCommand(s, nil, int64(1), 1, c.text)
		if got != c.isCmd {
			t.Errorf("handleCommand(%q) = %v, 期望 %v %s", c.text, got, c.isCmd, c.comment)
		}
	}
}

// /cancel 是从 Telegram 上唯一能"撤销"的动作,匹配错了要么删不掉、
// 要么把别的任务一起删了。
func TestCancelCommandMatching(t *testing.T) {
	s := newTGTestState(t)
	s.QueueMu.Lock()
	s.Queue = []types.QueueItem{
		{ID: "aaaaaaaa-1111", PlanCode: "24sk602", Datacenter: "gra", Status: "running"},
		{ID: "bbbbbbbb-2222", PlanCode: "24sk603", Datacenter: "rbx", Status: "pending"},
		{ID: "cccccccc-3333", PlanCode: "24sk604", Datacenter: "sbg", Status: "completed"},
	}
	s.QueueMu.Unlock()

	// 前缀命中一个
	out := cancelText(s, []string{"aaaaaaaa"})
	if !strings.Contains(out, "已取消 1 个") {
		t.Fatalf("按前缀取消失败: %s", out)
	}
	s.QueueMu.Lock()
	n := len(s.Queue)
	s.QueueMu.Unlock()
	if n != 2 {
		t.Fatalf("应剩 2 条，实际 %d", n)
	}

	// 已完成的任务不该被 all 扫掉 —— 它已经花过钱了,删了等于抹掉记录
	out = cancelText(s, []string{"all"})
	if !strings.Contains(out, "已取消 1 个") {
		t.Fatalf("/cancel all 应只取消进行中的那 1 条: %s", out)
	}
	s.QueueMu.Lock()
	remaining := len(s.Queue)
	remainingStatus := ""
	if remaining == 1 {
		remainingStatus = s.Queue[0].Status
	}
	s.QueueMu.Unlock()
	if remaining != 1 || remainingStatus != "completed" {
		t.Fatalf("已完成的任务不该被取消，剩余 %d 条 status=%s", remaining, remainingStatus)
	}

	// 没有参数要给用法,不能静默什么都不做
	if out := cancelText(s, nil); !strings.Contains(out, "用法") {
		t.Fatalf("空参数应回用法: %s", out)
	}
	// 匹配不到要说清楚
	if out := cancelText(s, []string{"zzzz"}); !strings.Contains(out, "没找到") {
		t.Fatalf("匹配不到应明说: %s", out)
	}
}

// 太短的前缀同时命中多条时必须拒绝,不能随便挑一条删。
func TestCancelRefusesAmbiguousPrefix(t *testing.T) {
	s := newTGTestState(t)
	s.QueueMu.Lock()
	s.Queue = []types.QueueItem{
		{ID: "abc11111", PlanCode: "p1", Status: "running"},
		{ID: "abc22222", PlanCode: "p2", Status: "running"},
	}
	s.QueueMu.Unlock()

	out := cancelText(s, []string{"abc"})
	if !strings.Contains(out, "太短") {
		t.Fatalf("有歧义的前缀应当拒绝而不是乱删: %s", out)
	}
	s.QueueMu.Lock()
	n := len(s.Queue)
	s.QueueMu.Unlock()
	if n != 2 {
		t.Fatalf("有歧义时不该删任何东西，实际剩 %d", n)
	}
}

// /help 必须把下单格式讲全,这是用户唯一能知道该发什么的地方。
func TestHelpTextCoversOrderFormat(t *testing.T) {
	h := helpText()
	for _, must := range []string{"型号", "机房", "数量", "/queue", "/cancel", "/status"} {
		if !strings.Contains(h, must) {
			t.Errorf("/help 里缺少 %q", must)
		}
	}
}

// 回复长度必须夹在 Telegram 的 4096 上限内,否则 sendMessage 直接失败,
// 用户发了命令什么都收不到。
func TestClampReply(t *testing.T) {
	if got := clampReply("短"); got != "短" {
		t.Fatalf("短文本不该被改: %q", got)
	}
	long := strings.Repeat("x", tgMaxReplyLen+500)
	got := clampReply(long)
	if len(got) > 4096 {
		t.Fatalf("截断后仍超 Telegram 上限: %d", len(got))
	}
	if !strings.Contains(got, "截断") {
		t.Fatal("截断了要告诉用户")
	}
}
