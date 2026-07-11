package db

import (
	"database/sql"
	"fmt"
	"time"
)

// TryClaimTelegramUpdate 幂等认领 update_id。
// 返回 claimed=true 表示首次处理；false 表示已处理过（重放）。
func (db *DB) TryClaimTelegramUpdate(updateID int64) (claimed bool, err error) {
	if updateID <= 0 {
		// 无 update_id 时不走幂等表（仍由上层 secret/chat 校验）
		return true, nil
	}
	now := float64(time.Now().Unix())
	res, err := db.Exec(
		`INSERT OR IGNORE INTO telegram_updates (update_id, processed_at) VALUES (?, ?)`,
		updateID, now,
	)
	if err != nil {
		return false, fmt.Errorf("claim telegram update: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// CleanupTelegramUpdates 删除 processed_at < beforeUnix 的旧 update_id。
func (db *DB) CleanupTelegramUpdates(beforeUnix float64) (int64, error) {
	res, err := db.Exec(`DELETE FROM telegram_updates WHERE processed_at < ?`, beforeUnix)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// TryConsumeTelegramButton 原子消费按钮 UUID（一次性 nonce）。
// 仅当 used_at=0 且存在时成功；成功后 used_at 写入当前时间。
func (db *DB) TryConsumeTelegramButton(id string) (ok bool, err error) {
	row, ok, err := db.ClaimTelegramButton(id)
	_ = row
	return ok, err
}

// ClaimTelegramButton 原子认领未使用按钮并返回完整行数据。
// 成功时 used_at 已写入；入队失败请调用 UnclaimTelegramButton 回滚。
func (db *DB) ClaimTelegramButton(id string) (row TelegramButtonRow, ok bool, err error) {
	if id == "" {
		return row, false, nil
	}
	now := float64(time.Now().Unix())
	// SQLite: 先更新再读；用事务保证并发安全
	tx, err := db.Beginx()
	if err != nil {
		return row, false, fmt.Errorf("begin claim button: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(
		`UPDATE telegram_order_buttons SET used_at = ? WHERE id = ? AND (used_at IS NULL OR used_at = 0)`,
		now, id,
	)
	if err != nil {
		return row, false, fmt.Errorf("claim telegram button: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return row, false, nil
	}
	err = tx.Get(&row, `SELECT id, plan_code, datacenter, options, config_info, created_at, COALESCE(used_at,0) AS used_at
		FROM telegram_order_buttons WHERE id = ?`, id)
	if err != nil {
		return row, false, fmt.Errorf("read claimed button: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return row, false, fmt.Errorf("commit claim button: %w", err)
	}
	return row, true, nil
}

// UnclaimTelegramButton 入队失败时回滚按钮，允许用户重试。
func (db *DB) UnclaimTelegramButton(id string) error {
	if id == "" {
		return nil
	}
	_, err := db.Exec(`UPDATE telegram_order_buttons SET used_at = 0 WHERE id = ?`, id)
	return err
}

// IsTelegramButtonUsed 查询按钮是否已消费（不存在视为 used=false, exists=false）
func (db *DB) IsTelegramButtonUsed(id string) (used bool, exists bool, err error) {
	var usedAt float64
	err = db.Get(&usedAt, `SELECT COALESCE(used_at,0) FROM telegram_order_buttons WHERE id = ?`, id)
	if err == sql.ErrNoRows {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return usedAt > 0, true, nil
}
