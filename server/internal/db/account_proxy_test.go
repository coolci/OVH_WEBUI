package db

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ovh-webui/server/internal/secret"

	"github.com/ovh-webui/server/internal/types"
)

// 加了列忘了同步行结构 / INSERT / upsert 的任何一处，写入都会被静默丢弃 ——
// 界面上代理填着、实际出站还是直连，而用户完全看不出来。
// VPS 的自动下单字段就是这么丢过一次。
func TestAccountProxyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	in := types.OVHAccount{
		ID: "a1", Name: "欧区", Endpoint: "ovh-eu", Zone: "IE",
		AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true,
		ProxyURL:    "socks5://user:pass@1.2.3.4:1080",
		Fingerprint: "chrome-like",
	}
	if err := d.UpsertAccount(in); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := d.ListAccounts()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("期望 1 个账户，实际 %d", len(got))
	}
	if got[0].ProxyURL != in.ProxyURL {
		t.Fatalf("代理地址没存下来：%q", got[0].ProxyURL)
	}
	if got[0].Fingerprint != "chrome-like" {
		t.Fatalf("指纹配置没存下来：%q", got[0].Fingerprint)
	}

	// 改一次也要生效（upsert 的 SET 子句漏字段是另一个常见坑）
	in.ProxyURL = "http://other:9090"
	in.Fingerprint = "legacy-http1"
	if err := d.UpsertAccount(in); err != nil {
		t.Fatalf("upsert2: %v", err)
	}
	got, _ = d.ListAccounts()
	if got[0].ProxyURL != "http://other:9090" || got[0].Fingerprint != "legacy-http1" {
		t.Fatalf("更新没生效：%+v", got[0])
	}
}

// 代理串带凭据，必须加密落盘 —— 和三个 API 密钥同等对待。
func TestAccountProxyEncryptedAtRest(t *testing.T) {
	dir := t.TempDir()
	// 没有密钥时 Encrypt 是空操作（AppSecret 也一样），所以这里先把加密打开，
	// 否则这个测试验不到任何东西。
	t.Setenv("OVH_DB_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err := secret.Init(dir, filepath.Join(dir, ".env")); err != nil {
		t.Fatalf("secret init: %v", err)
	}
	if !secret.Enabled() {
		t.Skip("加密没启用，跳过")
	}
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	const secretPart = "SuperSecretProxyPassword"
	acc := types.OVHAccount{
		ID: "a1", Name: "x", Endpoint: "ovh-eu", Zone: "IE",
		AppKey: "k", AppSecret: "s", ConsumerKey: "c",
		ProxyURL: "socks5://u:" + secretPart + "@1.2.3.4:1080",
	}
	if err := d.UpsertAccount(acc); err != nil {
		t.Fatal(err)
	}

	// 直接读原始列：不应该看到明文密码
	var raw string
	if err := d.Get(&raw, `SELECT proxy_url FROM ovh_accounts WHERE id = ?`, "a1"); err != nil {
		t.Fatalf("读原始列: %v", err)
	}
	if strings.Contains(raw, secretPart) {
		t.Fatalf("代理密码是明文落盘的：%q", raw)
	}
}
