//go:build !cgo

package db

import (
	"fmt"

	_ "modernc.org/sqlite" // 注册 driver "sqlite"
)

// driverName modernc.org/sqlite 注册的 driver 名
const driverName = "sqlite"

// makeDSN modernc.org/sqlite 的写连接池 DSN 语法：
// 显式开启 _txlock=immediate，避免 WAL 下先读后写 DEFERRED 事务升级锁瞬败报 SQLITE_BUSY (517)。
func makeDSN(path string) string {
	return fmt.Sprintf(
		"file:%s?_txlock=immediate&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)",
		path,
	)
}

// makeReadDSN modernc.org/sqlite 的并发只读连接池 DSN 语法
func makeReadDSN(path string) string {
	return fmt.Sprintf(
		"file:%s?_pragma=query_only(ON)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)",
		path,
	)
}
