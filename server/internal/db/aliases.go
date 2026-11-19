package db

import (
	"fmt"
	"time"
)

// ServerAlias 一条服务器本地别名记录。account_id + service_name 唯一定位。
// alias 是用户在前端给这台机器起的友好名(不下发 OVH,纯本地显示)。
type ServerAlias struct {
	AccountID   string `db:"account_id"`
	ServiceName string `db:"service_name"`
	Alias       string `db:"alias"`
	UpdatedAt   string `db:"updated_at"`
}

// ListAliasesByAccount 取一个账户下所有别名,key=service_name → alias。
func (db *DB) ListAliasesByAccount(accountID string) (map[string]string, error) {
	rows := []ServerAlias{}
	if err := db.Select(&rows,
		`SELECT account_id, service_name, alias, updated_at
		 FROM server_aliases WHERE account_id = ?`,
		accountID); err != nil {
		return nil, fmt.Errorf("list aliases for %s: %w", accountID, err)
	}
	out := map[string]string{}
	for _, r := range rows {
		out[r.ServiceName] = r.Alias
	}
	return out, nil
}

// UpsertAlias 写入/更新一条别名。alias 为空时调用 DeleteAlias 走删除分支。
func (db *DB) UpsertAlias(accountID, serviceName, alias string) error {
	if alias == "" {
		return db.DeleteAlias(accountID, serviceName)
	}
	_, err := db.Exec(
		`INSERT INTO server_aliases (account_id, service_name, alias, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(account_id, service_name) DO UPDATE SET
		   alias      = excluded.alias,
		   updated_at = excluded.updated_at`,
		accountID, serviceName, alias, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("upsert alias %s/%s: %w", accountID, serviceName, err)
	}
	return nil
}

// DeleteAlias 按账户 + service_name 删除一条别名。
func (db *DB) DeleteAlias(accountID, serviceName string) error {
	_, err := db.Exec(
		`DELETE FROM server_aliases WHERE account_id = ? AND service_name = ?`,
		accountID, serviceName,
	)
	if err != nil {
		return fmt.Errorf("delete alias %s/%s: %w", accountID, serviceName, err)
	}
	return nil
}

// DeleteAliasesByAccount 账户被删时级联清掉所有别名。
func (db *DB) DeleteAliasesByAccount(accountID string) error {
	_, err := db.Exec(`DELETE FROM server_aliases WHERE account_id = ?`, accountID)
	if err != nil {
		return fmt.Errorf("delete aliases for account %s: %w", accountID, err)
	}
	return nil
}
