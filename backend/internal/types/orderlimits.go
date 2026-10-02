package types

// PRD F04.7 / D-28: Single Source of Truth for order limits.
// Synchronized with src/lib/order-limits.ts via orderlimits_sync_test.go.

const (
	MaxOrderQuantity = 20  // 单个任务最多台数
	MaxOrderFanout   = 60  // 一次最多选多少个机房/批量生成任务
	MaxQueueSize     = 500 // 队列任务总上限
)

// ClampOrderQuantity clamps quantity to [1, MaxOrderQuantity].
func ClampOrderQuantity(qty int) int {
	if qty < 1 {
		return 1
	}
	if qty > MaxOrderQuantity {
		return MaxOrderQuantity
	}
	return qty
}
