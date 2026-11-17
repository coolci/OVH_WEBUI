package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// OrderCancellation 独立持久化，避免历史整表保存覆盖撤单记录。
type OrderCancellation struct {
	AbandonedAt     string `json:"abandonedAt,omitempty"`
	RetractionState string `json:"retractionState,omitempty"`
	UpdatedAt       string `json:"updatedAt,omitempty"`
}

var ErrOrderActionConflict = errors.New("订单已有付款或撤单请求，请先核对 OVH 状态")

func OrderCancellationKey(accountID string, orderID int64) string {
	return fmt.Sprintf("order-cancellation:%s:%d", accountID, orderID)
}

func (db *DB) GetOrderCancellation(accountID string, orderID int64) (OrderCancellation, error) {
	var action OrderCancellation
	_, err := db.GetKV(OrderCancellationKey(accountID, orderID), &action)
	return action, err
}

func cancellationInTx(tx *sqlx.Tx, key string) (OrderCancellation, error) {
	var action OrderCancellation
	var raw string
	err := tx.Get(&raw, `SELECT value FROM kv WHERE key = ?`, key)
	if err == sql.ErrNoRows {
		return action, nil
	}
	if err != nil {
		return action, err
	}
	err = json.Unmarshal([]byte(raw), &action)
	return action, err
}

// ChangeOrderCancellation 与付款登记共用写事务，阻止放弃和付款同时生效。
// retract 先登记 sending，再向 OVH 发送；未决结果重启后也不会重发。
func (db *DB) ChangeOrderCancellation(accountID string, orderID int64, operation string) error {
	tx, err := db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	key := OrderCancellationKey(accountID, orderID)
	action, err := cancellationInTx(tx, key)
	if err != nil {
		return err
	}
	if action.RetractionState != "" && action.RetractionState != "failed" {
		return ErrOrderActionConflict
	}
	if operation == "abandon" {
		var count int
		if err := tx.Get(&count, `SELECT COUNT(*) FROM kv WHERE key = ?`, OrderPaymentKey(accountID, orderID)); err != nil {
			return err
		}
		if count != 0 {
			return ErrOrderActionConflict
		}
		if action.AbandonedAt == "" {
			action.AbandonedAt = time.Now().UTC().Format(time.RFC3339Nano)
		}
	} else if operation == "restore" {
		action.AbandonedAt = ""
	} else if operation == "retract" {
		if action.AbandonedAt != "" {
			return ErrOrderActionConflict
		}
		action.RetractionState = "sending"
	} else {
		return fmt.Errorf("未知的订单操作")
	}
	action.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	raw, err := json.Marshal(action)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO kv(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, string(raw)); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) SetOrderRetractionState(accountID string, orderID int64, status string) error {
	action, err := db.GetOrderCancellation(accountID, orderID)
	if err != nil {
		return err
	}
	action.RetractionState = status
	action.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return db.SetKV(OrderCancellationKey(accountID, orderID), action)
}
