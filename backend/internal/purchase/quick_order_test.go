package purchase

import (
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

func quickOrderTestState(t *testing.T) *app.State {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	lg := logger.New(filepath.Join(dir, "t.log"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return app.NewState(storage.Paths{DataDir: dir}, config.New(database), lg, database)
}

func TestEnqueueQuickOrderValidations(t *testing.T) {
	s := quickOrderTestState(t)

	// 1. Missing planCode or datacenter
	res := EnqueueQuickOrder(s, QuickOrderParams{
		AccountID:  "acc-1",
		PlanCode:   "",
		Datacenter: "gra",
	})
	if res.Success || res.Code != "E75DA4B08" {
		t.Fatalf("缺少 planCode 应该返回 E75DA4B08，实际: %+v", res)
	}

	// 2. Missing account_id
	res = EnqueueQuickOrder(s, QuickOrderParams{
		AccountID:  "",
		PlanCode:   "24sk602",
		Datacenter: "gra",
	})
	if res.Success || res.Code != "E34CBF1D4" {
		t.Fatalf("缺少 account_id 应该返回 E34CBF1D4，实际: %+v", res)
	}

	// 3. Nonexistent account_id
	res = EnqueueQuickOrder(s, QuickOrderParams{
		AccountID:  "acc-nonexistent",
		PlanCode:   "24sk602",
		Datacenter: "gra",
	})
	if res.Success || res.Code != "E7B315B13" {
		t.Fatalf("账户不存在应该返回 E7B315B13，实际: %+v", res)
	}
}

func TestEnqueueQuickOrderDuplicateChecks(t *testing.T) {
	s := quickOrderTestState(t)

	// 预先插入一个可用账户
	acc := types.OVHAccount{
		ID:        "acc-1",
		Name:      "test-account",
		Endpoint:  "ovh-eu",
		AppKey:    "key",
		AppSecret: "secret",
	}
	if err := s.DB.UpsertAccount(acc); err != nil {
		t.Fatal(err)
	}
	s.ReloadAccounts()

	// 1. 模拟监控来源且已下过单 (120s 闸门)
	now := types.NowISO()
	s.HistoryMu.Lock()
	s.History = append(s.History, types.PurchaseHistoryEntry{
		ID:           "hist-1",
		PlanCode:     "24sk602",
		Datacenter:   "gra",
		Options:      []string{"opt-a", "opt-b"},
		Status:       "success",
		PurchaseTime: now,
	})
	s.HistoryMu.Unlock()

	res := EnqueueQuickOrder(s, QuickOrderParams{
		AccountID:          "acc-1",
		PlanCode:           "24sk602",
		Datacenter:         "gra",
		Options:            []string{"opt-b", "opt-a"}, // 无序也应算同配置
		FromMonitor:        true,
		SkipDuplicateCheck: true,
	})
	if res.Success || res.Code != "EF4FA206C" || res.HTTPStatus != http.StatusTooManyRequests {
		t.Fatalf("120s内已成功下单应该被闸门拦截 EF4FA206C，实际: %+v", res)
	}

	// 2. 模拟非监控来源(用户手动)遇到正在运行的队列任务
	s.HistoryMu.Lock()
	s.History = nil // 清空历史
	s.HistoryMu.Unlock()

	s.QueueMu.Lock()
	s.Queue = append(s.Queue, types.QueueItem{
		ID:         "q-1",
		PlanCode:   "24sk602",
		Datacenter: "rbx",
		Options:    []string{"opt-1"},
		Status:     "running",
	})
	s.QueueMu.Unlock()

	resManual := EnqueueQuickOrder(s, QuickOrderParams{
		AccountID:          "acc-1",
		PlanCode:           "24sk602",
		Datacenter:         "rbx",
		Options:            []string{"opt-1"},
		FromMonitor:        false,
		SkipDuplicateCheck: false,
	})
	if resManual.Success || resManual.Code != "EE77AF09E" || resManual.HTTPStatus != http.StatusTooManyRequests {
		t.Fatalf("队列中已有同配置任务应该返回 EE77AF09E，实际: %+v", resManual)
	}
}

func TestEnqueueQuickOrderFromMonitorNoBlocking(t *testing.T) {
	s := quickOrderTestState(t)

	acc := types.OVHAccount{
		ID:        "acc-1",
		Name:      "test-account",
		Endpoint:  "ovh-eu",
		AppKey:    "key",
		AppSecret: "secret",
	}
	if err := s.DB.UpsertAccount(acc); err != nil {
		t.Fatal(err)
	}
	s.ReloadAccounts()

	// 监控来源带 options 入队: 即使无缓存且未连接 OVH, 也不阻断入队
	res := EnqueueQuickOrder(s, QuickOrderParams{
		AccountID:          "acc-1",
		PlanCode:           "24sk602",
		Datacenter:         "gra",
		Options:            []string{"opt-test"},
		FromMonitor:        true,
		SkipDuplicateCheck: true,
	})

	if !res.Success {
		t.Fatalf("监控来源快速下单不应因询价被阻断，但返回错误: %v", res.Error)
	}
	if len(s.Queue) != 1 {
		t.Fatalf("任务应已进入队列，队列长度: %d", len(s.Queue))
	}
	if s.Queue[0].PlanCode != "24sk602" || s.Queue[0].Datacenter != "gra" {
		t.Fatalf("入队任务参数不符: %+v", s.Queue[0])
	}
	if !s.Queue[0].QuickOrder || s.Queue[0].Priority != 100 {
		t.Fatalf("QuickOrder 属性未正确设置: %+v", s.Queue[0])
	}
}

func TestFingerprint(t *testing.T) {
	fp1 := fingerprint([]string{"b", "a", "c"})
	fp2 := fingerprint([]string{"a", "c", "b"})
	if fp1 != fp2 || fp1 != "a|b|c" {
		t.Fatalf("fingerprint 应排序去重: fp1=%s, fp2=%s", fp1, fp2)
	}

	fpEmpty := fingerprint(nil)
	if fpEmpty != "" {
		t.Fatalf("fingerprint nil 应返回空串: %s", fpEmpty)
	}
}
