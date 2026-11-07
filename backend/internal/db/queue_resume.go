package db

import (
	"database/sql"

	"github.com/ovh-webui/server/internal/types"
)

// ResumeQueueItem 在同一个事务里提交任务的恢复状态和自动建单许可(移植自上游 v0.4.14)。
//
// 恢复必须"先落库、后生效":以前先改内存再整表保存,保存失败时内存里已经是 running,
// 处理器这一轮就会去下单,而库里仍是 paused —— 重启后状态和真实下单情况对不上。
// blockKey 是 app.automaticOrderKey 生成的停止记录 key,恢复即解除停止。
func (db *DB) ResumeQueueItem(item types.QueueItem, blockKey string) error {
	tx, err := db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE queue SET status=?, updated_at=?, failure_count=?, last_check_time=? WHERE id=?`,
		item.Status, item.UpdatedAt, item.FailureCount, item.LastCheckTime, item.ID)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil {
		return err
	} else if count != 1 {
		return sql.ErrNoRows
	}
	if blockKey != "" {
		if _, err = tx.Exec(`INSERT INTO kv(key,value) VALUES(?,'false') ON CONFLICT(key) DO UPDATE SET value=excluded.value`, blockKey); err != nil {
			return err
		}
	}
	return tx.Commit()
}
