//go:build cgo

package db

import (
	"fmt"

	_ "github.com/mattn/go-sqlite3" // 注册 driver "sqlite3"
)

// driverName mattn/go-sqlite3 注册的 driver 名
const driverName = "sqlite3"

// makeDSN mattn/go-sqlite3 的写连接池 DSN 语法：显式开启 _txlock=immediate
func makeDSN(path string) string {
	return fmt.Sprintf(
		"file:%s?_txlock=immediate&_journal=WAL&_synchronous=NORMAL&_fk=true&_busy_timeout=5000",
		path,
	)
}

// makeReadDSN mattn/go-sqlite3 的并发只读连接池 DSN 语法
func makeReadDSN(path string) string {
	return fmt.Sprintf(
		"file:%s?_query_only=true&_journal=WAL&_synchronous=NORMAL&_fk=true&_busy_timeout=5000",
		path,
	)
}
