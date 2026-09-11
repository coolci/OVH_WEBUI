package proxyguard

import (
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

func newGuardState(t *testing.T) *app.State {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	lg := logger.New(filepath.Join(dir, "t.log"),
		slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})))
	st := app.NewState(storage.Paths{DataDir: dir}, config.New(database), lg, database)
	acc := types.OVHAccount{ID: "eu", Name: "欧区", Endpoint: "ovh-eu", Zone: "IE",
		AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true,
		ProxyURL: "socks5://u:pw@1.2.3.4:1080"}
	if err := database.UpsertAccount(acc); err != nil {
		t.Fatal(err)
	}
	if err := st.ReloadAccounts(); err != nil {
		t.Fatal(err)
	}
	return st
}

func proxyErr() error {
	return &netfp.ProxyError{Proxy: "socks5://u:***@1.2.3.4:1080", Err: errors.New("connection refused")}
}

// 偶发一两次不该停任务 —— 代理抖一下很常见，而停任务的代价是错过补货窗口。
func TestSingleFailureDoesNotTrip(t *testing.T) {
	st := newGuardState(t)
	g := Init(st)
	st.QueueMu.Lock()
	st.Queue = []types.QueueItem{{ID: "q1", AccountID: "eu", PlanCode: "24sk602", Status: "running"}}
	st.QueueMu.Unlock()

	g.Report("eu", proxyErr())
	if Tripped("eu") {
		t.Fatal("失败一次就停任务过于激进")
	}
	st.QueueMu.Lock()
	status := st.Queue[0].Status
	st.QueueMu.Unlock()
	if status != "running" {
		t.Fatalf("任务不该被动，实际 %s", status)
	}
}

// 连续失败到阈值就必须停 —— 拖着只是让用户以为还在抢。
func TestConsecutiveFailuresPauseTasks(t *testing.T) {
	st := newGuardState(t)
	g := Init(st)
	st.QueueMu.Lock()
	st.Queue = []types.QueueItem{
		{ID: "q1", AccountID: "eu", PlanCode: "24sk602", Status: "running"},
		{ID: "q2", AccountID: "eu", PlanCode: "24sk603", Status: "pending"},
		{ID: "q3", AccountID: "eu", PlanCode: "24sk604", Status: "completed"}, // 已完成的不该动
		{ID: "q4", AccountID: "us", PlanCode: "24sk605", Status: "running"},   // 别的账户不该动
	}
	st.QueueMu.Unlock()

	for i := 0; i < failThreshold; i++ {
		g.Report("eu", proxyErr())
	}
	if !Tripped("eu") {
		t.Fatal("连续失败到阈值应当跳闸")
	}

	st.QueueMu.Lock()
	defer st.QueueMu.Unlock()
	if st.Queue[0].Status != "paused" || st.Queue[1].Status != "paused" {
		t.Fatalf("该账户进行中的任务应当被暂停：%s %s", st.Queue[0].Status, st.Queue[1].Status)
	}
	// 用 paused 而不是 failed：代理是能修好的，任务不该被判死
	if st.Queue[0].Status == "failed" {
		t.Fatal("应当是 paused 而不是 failed —— 修好代理要能接着抢")
	}
	if st.Queue[2].Status != "completed" {
		t.Fatal("已完成的任务不该被动 —— 那些已经花过钱了")
	}
	if st.Queue[3].Status != "running" {
		t.Fatal("别的账户的任务被误伤了")
	}
}

// 非代理错误（OVH 自己返回的业务错误）绝不能触发跳闸 —— 误判会把正常任务停掉。
func TestNonProxyErrorNeverTrips(t *testing.T) {
	st := newGuardState(t)
	g := Init(st)
	for i := 0; i < failThreshold*3; i++ {
		g.Report("eu", errors.New("Error 429: Too many requests"))
	}
	if Tripped("eu") {
		t.Fatal("OVH 的业务错误被当成了代理故障 —— 会把正常任务停掉")
	}
}

// 代理恢复后要清零，但任务**不自动恢复**：
// 用户可能已经手动处理过了，替他改回去等于替他决定重新开始花钱。
func TestRecoveryClearsButDoesNotResume(t *testing.T) {
	st := newGuardState(t)
	g := Init(st)
	st.QueueMu.Lock()
	st.Queue = []types.QueueItem{{ID: "q1", AccountID: "eu", PlanCode: "24sk602", Status: "running"}}
	st.QueueMu.Unlock()

	for i := 0; i < failThreshold; i++ {
		g.Report("eu", proxyErr())
	}
	if !Tripped("eu") {
		t.Fatal("应当跳闸")
	}
	g.ReportSuccess("eu")
	if Tripped("eu") {
		t.Fatal("恢复后应当清零")
	}
	st.QueueMu.Lock()
	status := st.Queue[0].Status
	st.QueueMu.Unlock()
	if status != "paused" {
		t.Fatalf("任务不该自动恢复运行，实际 %s", status)
	}
}

// 空账户 ID 不该炸，也不该记账
func TestEmptyAccountIgnored(t *testing.T) {
	st := newGuardState(t)
	g := Init(st)
	g.Report("", proxyErr())
	g.ReportSuccess("")
	if len(Status()) != 0 {
		t.Fatal("空账户 ID 不该产生记录")
	}
}
