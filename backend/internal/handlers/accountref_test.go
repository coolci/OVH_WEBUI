package handlers

import (
	"strings"
	"testing"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

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

// @xxx 解析错的后果是下到另一个账户上，而那不会报错，只会一直抢不到。
// 所以宁可拒绝也不能"猜一个最像的"。
func TestResolveAccountRef(t *testing.T) {
	st, mon := newWatchTestMonitor(t)
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
	got, err = resolveAccountRef(st, mon, "all", "24sk602")
	if err != "" || len(got) < 1 {
		t.Fatalf("@all 应当返回账户列表，实际 %v err=%q", got, err)
	}
}

// 同一个子公司有两个账户时必须要求用序号，不能随便挑一个 ——
// 挑错了下到另一个账户上，用户看不出来。
func TestResolveAccountRefAmbiguous(t *testing.T) {
	st, _ := newWatchTestMonitor(t)
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

// 没有账户时要说清楚，不能静默返回空
func TestResolveAccountRefNoAccounts(t *testing.T) {
	st, _ := newWatchTestMonitor(t)
	if _, err := resolveAccountRef(st, nil, "us", "24sk602"); err == "" {
		t.Fatal("没有账户时应当明确报错")
	}
}
