package db

import (
	"database/sql"
	"fmt"
	"time"
)

// TelegramShortIDRow 短 ID 映射行
type TelegramShortIDRow struct {
	ShortID   string `db:"short_id"`
	FullID    string `db:"full_id"`
	Category  string `db:"category"`
	CreatedAt int64  `db:"created_at"`
}

// UpsertShortID 保存或更新短 ID 映射
func (db *DB) UpsertShortID(shortID, fullID, category string) error {
	if shortID == "" || fullID == "" {
		return fmt.Errorf("empty shortID or fullID")
	}
	now := time.Now().Unix()
	_, err := db.Exec(
		`INSERT INTO telegram_short_ids (short_id, full_id, category, created_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(short_id) DO UPDATE SET
		   full_id    = excluded.full_id,
		   category   = excluded.category,
		   created_at = excluded.created_at`,
		shortID, fullID, category, now,
	)
	if err != nil {
		return fmt.Errorf("upsert short id: %w", err)
	}
	return nil
}

// GetShortID 查询短 ID 对应的完整 ID
func (db *DB) GetShortID(shortID string) (string, bool, error) {
	if shortID == "" {
		return "", false, nil
	}
	var fullID string
	err := db.Get(&fullID, `SELECT full_id FROM telegram_short_ids WHERE short_id = ?`, shortID)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get short id: %w", err)
	}
	return fullID, true, nil
}

// FindShortIDByFull 逆向查找完整 ID 现有的短 ID（优先复用最近创建的短 ID）
func (db *DB) FindShortIDByFull(fullID string) (string, bool, error) {
	if fullID == "" {
		return "", false, nil
	}
	var shortID string
	err := db.Get(&shortID, `SELECT short_id FROM telegram_short_ids WHERE full_id = ? ORDER BY created_at DESC LIMIT 1`, fullID)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("find short id by full: %w", err)
	}
	return shortID, true, nil
}

// DeleteExpiredShortIDs 清理过期的短 ID 映射记录
func (db *DB) DeleteExpiredShortIDs(olderThanUnix int64) (int64, error) {
	res, err := db.Exec(`DELETE FROM telegram_short_ids WHERE created_at < ?`, olderThanUnix)
	if err != nil {
		return 0, fmt.Errorf("delete expired short ids: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
