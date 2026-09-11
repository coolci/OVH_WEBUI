package handlers

import (
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/types"
)

func newTestState(t *testing.T) *app.State {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	lg := logger.New(filepath.Join(dir, "t.log"),
		slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})))
	st := &app.State{
		DB:     database,
		Logger: lg,
	}
	return st
}

func addTwoAccounts(t *testing.T, st *app.State) {
	t.Helper()
	for _, a := range []types.OVHAccount{
		{ID: "eu", Name: "欧区主号", Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true},
		{ID: "us", Name: "美区小号", Endpoint: "ovh-us", Zone: "US", AppKey: "k", AppSecret: "s", ConsumerKey: "c"},
	} {
		if err := st.DB.UpsertAccount(a); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	if err := st.ReloadAccounts(); err != nil {
		t.Fatalf("reload: %v", err)
	}
}

func TestResolveAccountRef(t *testing.T) {
	st := newTestState(t)
	addTwoAccounts(t, st)

	// 按子公司
	got, err := resolveAccountRef(st, nil, "us", "24sk602")
	if err != "" || len(got) != 1 || got[0].ID != "us" {
		t.Fatalf("@us 应解析到美区账户，实际 %v err=%q", got, err)
	}
	// 按序号
	got, err = resolveAccountRef(st, nil, "1", "24sk602")
	if err != "" || len(got) != 1 || got[0].ID != "eu" {
		t.Fatalf("@1 应是列表第一个，实际 %v err=%q", got, err)
	}
	// 越界要报错，不能回绕
	if _, err = resolveAccountRef(st, nil, "9", "24sk602"); err == "" {
		t.Fatal("@9 越界应当报错")
	}
	// 不存在的子公司
	if _, err = resolveAccountRef(st, nil, "jp", "24sk602"); err == "" {
		t.Fatal("@jp 不存在应当报错")
	} else if !strings.Contains(err, "@1") {
		t.Fatalf("报错里应当列出可用的账户，实际：%s", err)
	}
	// @all：mon 为 nil（判不出来哪些能买）时返回全部，宁可多投不少投
	got, err = resolveAccountRef(st, nil, "all", "24sk602")
	if err != "" || len(got) < 1 {
		t.Fatalf("@all 应当返回账户列表，实际 %v err=%q", got, err)
	}
}

func TestResolveAccountRefAmbiguous(t *testing.T) {
	st := newTestState(t)
	for _, a := range []types.OVHAccount{
		{ID: "eu1", Name: "欧区A", Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true},
		{ID: "eu2", Name: "欧区B", Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c"},
	} {
		if err := st.DB.UpsertAccount(a); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.ReloadAccounts(); err != nil {
		t.Fatal(err)
	}

	got, err := resolveAccountRef(st, nil, "ie", "24sk602")
	if err == "" {
		t.Fatalf("两个 IE 账户时 @ie 有歧义，应当拒绝，实际返回 %v", got)
	}
	if !strings.Contains(err, "@1") || !strings.Contains(err, "@2") {
		t.Fatalf("报错里要给出可用的序号，实际：%s", err)
	}
}

func TestResolveAccountRefNoAccounts(t *testing.T) {
	st := newTestState(t)
	if _, err := resolveAccountRef(st, nil, "us", "24sk602"); err == "" {
		t.Fatal("没有账户时应当明确报错")
	}
}
