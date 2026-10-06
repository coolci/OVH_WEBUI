package telegram

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

func newAccTestState(t *testing.T) *app.State {
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
	for _, a := range []types.OVHAccount{
		{ID: "eu", Name: "欧区", Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true},
		{ID: "us", Name: "美区", Endpoint: "ovh-us", Zone: "US", AppKey: "k", AppSecret: "s", ConsumerKey: "c"},
	} {
		if err := database.UpsertAccount(a); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	if err := st.ReloadAccounts(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	return st
}

// 没选过就用默认账户，并且要能区分"用户选的"和"退回默认的"——
// 界面上得把后者标出来，否则多账户用户不知道自己在用哪个。
func TestActiveAccountDefaultsToDefault(t *testing.T) {
	st := newAccTestState(t)
	acc, explicit := ActiveAccount(st)
	if acc.ID != "eu" {
		t.Fatalf("没选过应落默认账户 eu，实际 %q", acc.ID)
	}
	if !explicit {
		t.Fatal("没选过时第二个返回值应为 true（表示这就是默认账户，不是退回来的）")
	}
}

func TestActiveAccountSwitch(t *testing.T) {
	st := newAccTestState(t)
	if err := SetActiveAccount(st, "us"); err != nil {
		t.Fatalf("set: %v", err)
	}
	acc, explicit := ActiveAccount(st)
	if acc.ID != "us" {
		t.Fatalf("切换后应是 us，实际 %q", acc.ID)
	}
	if !explicit {
		t.Fatal("显式选过的应当标记为 explicit")
	}
	if ActiveAccountID(st) != "us" {
		t.Fatal("ActiveAccountID 与 ActiveAccount 不一致")
	}
}

// 选中的账户被删掉之后必须退回默认，而不是留着一个死 id ——
// 留着的话下单要到几小时后才失败，且报的是"账户不存在"这种看不懂的错。
func TestActiveAccountFallsBackWhenDeleted(t *testing.T) {
	st := newAccTestState(t)
	if err := SetActiveAccount(st, "us"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := st.DB.DeleteAccount("us"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := st.ReloadAccounts(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	acc, explicit := ActiveAccount(st)
	if acc.ID != "eu" {
		t.Fatalf("账户被删后应退回默认 eu，实际 %q", acc.ID)
	}
	if explicit {
		t.Fatal("退回默认时必须标记成非显式，界面要能说出「用的是默认账户」")
	}
}

// 大区必须写在标签里：三个站点目录互不相通，只写名字用户看不出选对没有。
func TestAccountLabelIncludesRegion(t *testing.T) {
	l := AccountLabel(types.OVHAccount{Name: "美区", Zone: "US"})
	if l == "美区" {
		t.Fatal("标签里必须带子公司和大区")
	}
	for _, must := range []string{"美区", "US"} {
		if !strings.Contains(l, must) {
			t.Fatalf("标签 %q 里缺少 %q", l, must)
		}
	}
}
