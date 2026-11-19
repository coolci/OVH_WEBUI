package db

import "fmt"

// OrderPaymentKey 按账户与订单隔离付款记录,重启后仍能阻止重复付款。
func OrderPaymentKey(accountID string, orderID int64) string {
	return fmt.Sprintf("order-payment:%s:%d", accountID, orderID)
}

// ClaimOrderPayment 必须在发出付款请求前落库;同一订单只能有一个请求持有记录。
func (db *DB) ClaimOrderPayment(accountID string, orderID int64) (bool, error) {
	tx, err := db.Beginx()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	action, err := cancellationInTx(tx, OrderCancellationKey(accountID, orderID))
	if err != nil {
		return false, err
	}
	if action.AbandonedAt != "" || (action.RetractionState != "" && action.RetractionState != "failed") {
		return false, nil
	}
	result, err := tx.Exec(`INSERT OR IGNORE INTO kv(key,value) VALUES(?,?)`, OrderPaymentKey(accountID, orderID), `{"state":"pending"}`)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, tx.Commit()
}
