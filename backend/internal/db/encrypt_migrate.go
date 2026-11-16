package db

import (
	"encoding/json"
	"fmt"

	"github.com/ovh-webui/server/internal/secret"
)

// EncryptExistingSecrets 把老库里已有的明文凭据就地加密。
//
// 涵盖范围（BUG-SEC-06 / BUG-SEC-07）：
//   - ovh_accounts.{app_key, app_secret, consumer_key, proxy_url}
//   - kv['config'].{appKey, appSecret, consumerKey, tgToken, tgWebhookSecret, notifyWebhookUrl}
//
// 迁移是幂等的:空串或已经是密文的跳过。加密不可用时直接返回,什么都不做。
// 单条失败不中断整体:宁可少加密一条,也不能因为一条坏数据让用户启动不了。
func (db *DB) EncryptExistingSecrets() (migrated int, err error) {
	if !secret.Enabled() {
		return 0, nil
	}

	// ---- 1. ovh_accounts 的凭据列与代理列 ----
	type row struct {
		ID          string `db:"id"`
		AppKey      string `db:"app_key"`
		AppSecret   string `db:"app_secret"`
		ConsumerKey string `db:"consumer_key"`
		ProxyURL    string `db:"proxy_url"`
	}
	var rows []row
	if err := db.Select(&rows, `SELECT id, app_key, app_secret, consumer_key, proxy_url FROM ovh_accounts`); err != nil {
		return 0, fmt.Errorf("读取账户失败: %w", err)
	}
	needsEnc := func(v string) bool {
		return v != "" && !secret.IsEncrypted(v)
	}
	enc := func(v string) string {
		if v == "" || secret.IsEncrypted(v) {
			return v
		}
		return secret.Encrypt(v)
	}
	for _, r := range rows {
		if !needsEnc(r.AppKey) && !needsEnc(r.AppSecret) && !needsEnc(r.ConsumerKey) && !needsEnc(r.ProxyURL) {
			continue // 全部为空或已经加密过
		}
		if _, uerr := db.Exec(
			`UPDATE ovh_accounts SET app_key = ?, app_secret = ?, consumer_key = ?, proxy_url = ? WHERE id = ?`,
			enc(r.AppKey), enc(r.AppSecret), enc(r.ConsumerKey), enc(r.ProxyURL), r.ID,
		); uerr != nil {
			err = fmt.Errorf("加密账户 %s 失败: %w", r.ID, uerr)
			continue
		}
		migrated++
	}

	// ---- 2. kv['config'] 里的 Telegram Token / webhook secret / NotifyWebhookURL ----
	var raw string
	if gerr := db.Get(&raw, `SELECT value FROM kv WHERE key = 'config'`); gerr == nil && raw != "" {
		if updated, changed := encryptConfigJSON(raw); changed {
			if _, uerr := db.Exec(`UPDATE kv SET value = ? WHERE key = 'config'`, updated); uerr != nil {
				if err == nil {
					err = fmt.Errorf("加密 kv[config] 失败: %w", uerr)
				}
			} else {
				migrated++
			}
		}
	}

	return migrated, err
}

// encryptConfigJSON 把 config JSON 里的敏感字段就地加密,返回 (新 JSON, 是否有改动)
func encryptConfigJSON(raw string) (string, bool) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return raw, false
	}
	changed := false
	for _, k := range []string{"appKey", "appSecret", "consumerKey", "tgToken", "tgWebhookSecret", "notifyWebhookUrl"} {
		v, _ := m[k].(string)
		if v == "" || secret.IsEncrypted(v) {
			continue
		}
		m[k] = secret.Encrypt(v)
		changed = true
	}
	if !changed {
		return raw, false
	}
	out, err := json.Marshal(m)
	if err != nil {
		return raw, false
	}
	return string(out), true
}
