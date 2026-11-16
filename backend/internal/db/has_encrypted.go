package db

import (
	"strings"

	"github.com/ovh-webui/server/internal/secret"
)

// HasEncryptedSecrets 数据库里是否存有加密过的敏感凭据（含 ovh_accounts.proxy_url 及 kv['config']）。
func (d *DB) HasEncryptedSecrets() (bool, error) {
	var n int
	err := d.QueryRow(`SELECT COUNT(*) FROM ovh_accounts
		WHERE app_key LIKE 'enc:v1:%'
		   OR app_secret LIKE 'enc:v1:%'
		   OR consumer_key LIKE 'enc:v1:%'
		   OR proxy_url LIKE 'enc:v1:%'`).Scan(&n)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}

	// 账户表可能是空的,但全局配置里的 Telegram token / Webhook URL 一样是加密存的
	var raw string
	err = d.QueryRow(`SELECT value FROM kv WHERE key = 'config'`).Scan(&raw)
	if err != nil {
		return false, nil
	}
	return strings.Contains(raw, secret.EncPrefix), nil
}
