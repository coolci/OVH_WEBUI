/**
 * PRD F04.7 / D-28: Single Source of Truth for order limits.
 * Synchronized with backend/internal/types/orderlimits.go via orderlimits_sync_test.go
 */

export const MAX_ORDER_QUANTITY = 20; // 单个任务最多台数
export const MAX_ORDER_FANOUT = 60;   // 一次最多选多少个机房/批量生成任务
export const MAX_QUEUE_SIZE = 500;    // 队列任务总上限

/**
 * Clamps quantity to [1, MAX_ORDER_QUANTITY].
 * Protects users from accidental typos like 9999 triggering thousands of orders.
 */
export function clampOrderQuantity(qty: number): number {
  if (isNaN(qty) || qty < 1) {
    return 1;
  }
  return Math.min(Math.floor(qty), MAX_ORDER_QUANTITY);
}
