package app

import (
	"errors"
	"fmt"
)

// RejectedError 入队被业务规则拒绝(重复任务、监控已停止/暂停、队列超上限、参数非法…),
// 不是存储故障。调用方据此返回 409/400 和原因本身,而不是"没能写进数据库"——
// 后者会让用户以为数据库坏了,去查一个不存在的故障。
type RejectedError struct{ Reason string }

func (e *RejectedError) Error() string { return e.Reason }

// Rejected 构造一个业务拒绝错误
func Rejected(format string, args ...interface{}) error {
	return &RejectedError{Reason: fmt.Sprintf(format, args...)}
}

// IsRejected 判断错误链里是否有业务拒绝
func IsRejected(err error) bool {
	var r *RejectedError
	return errors.As(err, &r)
}
