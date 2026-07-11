package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// TelegramButtonRow 一键下单按钮 UUID → 下单参数（持久化，避免进程重启后按钮失效）
type TelegramButtonRow struct {
	ID         string  `db:"id"`
	PlanCode   string  `db:"plan_code"`
	Datacenter string  `db:"datacenter"`
	Options    string  `db:"options"`     // JSON []string
	ConfigInfo string  `db:"config_info"` // JSON object
	CreatedAt  float64 `db:"created_at"`  // unix seconds
	UsedAt     float64 `db:"used_at"`     // >0 已消费
}

// UpsertTelegramButton 写入/更新一键下单按钮缓存
func (db *DB) UpsertTelegramButton(id, planCode, datacenter string, options []string, configInfo map[string]interface{}, createdAt float64) error {
	if options == nil {
		options = []string{}
	}
	optsRaw, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("marshal options: %w", err)
	}
	if configInfo == nil {
		configInfo = map[string]interface{}{}
	}
	cfgRaw, err := json.Marshal(configInfo)
	if err != nil {
		return fmt.Errorf("marshal config_info: %w", err)
	}
	if createdAt <= 0 {
		createdAt = float64(time.Now().Unix())
	}
	_, err = db.Exec(
		`INSERT INTO telegram_order_buttons (id, plan_code, datacenter, options, config_info, created_at, used_at)
		 VALUES (?, ?, ?, ?, ?, ?, 0)
		 ON CONFLICT(id) DO UPDATE SET
		   plan_code=excluded.plan_code,
		   datacenter=excluded.datacenter,
		   options=excluded.options,
		   config_info=excluded.config_info,
		   created_at=excluded.created_at,
		   used_at=0`,
		id, planCode, datacenter, string(optsRaw), string(cfgRaw), createdAt,
	)
	if err != nil {
		return fmt.Errorf("upsert telegram button: %w", err)
	}
	return nil
}

// GetTelegramButton 按 UUID 取按钮配置；不存在返回 ok=false
func (db *DB) GetTelegramButton(id string) (row TelegramButtonRow, ok bool, err error) {
	err = db.Get(&row, `SELECT id, plan_code, datacenter, options, config_info, created_at, COALESCE(used_at,0) AS used_at
		FROM telegram_order_buttons WHERE id = ?`, id)
	if err == sql.ErrNoRows {
		return row, false, nil
	}
	if err != nil {
		return row, false, fmt.Errorf("get telegram button: %w", err)
	}
	return row, true, nil
}

// ListTelegramButtonsSince 加载 created_at >= sinceUnix 的全部按钮（启动回灌内存）
func (db *DB) ListTelegramButtonsSince(sinceUnix float64) ([]TelegramButtonRow, error) {
	var rows []TelegramButtonRow
	err := db.Select(&rows,
		`SELECT id, plan_code, datacenter, options, config_info, created_at, COALESCE(used_at,0) AS used_at
		 FROM telegram_order_buttons WHERE created_at >= ? AND (used_at IS NULL OR used_at = 0)`, sinceUnix)
	if err != nil {
		return nil, fmt.Errorf("list telegram buttons: %w", err)
	}
	return rows, nil
}

// DeleteTelegramButton 删除单条
func (db *DB) DeleteTelegramButton(id string) error {
	_, err := db.Exec(`DELETE FROM telegram_order_buttons WHERE id = ?`, id)
	return err
}

// DeleteExpiredTelegramButtons 清理 created_at < beforeUnix 的过期按钮
func (db *DB) DeleteExpiredTelegramButtons(beforeUnix float64) (int64, error) {
	res, err := db.Exec(`DELETE FROM telegram_order_buttons WHERE created_at < ?`, beforeUnix)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// ParseTelegramButtonOptions 解析 options JSON
func ParseTelegramButtonOptions(raw string) []string {
	if raw == "" {
		return []string{}
	}
	var opts []string
	if err := json.Unmarshal([]byte(raw), &opts); err != nil {
		return []string{}
	}
	if opts == nil {
		return []string{}
	}
	return opts
}

// ParseTelegramButtonConfigInfo 解析 config_info JSON
func ParseTelegramButtonConfigInfo(raw string) map[string]interface{} {
	if raw == "" {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil || m == nil {
		return map[string]interface{}{}
	}
	return m
}
