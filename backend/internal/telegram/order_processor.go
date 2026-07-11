package telegram

// OrderResult 对应 process_telegram_order / EnqueueSingle 返回
type OrderResult struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	TotalOrders   int    `json:"total_orders"`
	CreatedOrders int    `json:"created_orders"`
}

// subset 配置 options 子集匹配
func subset(needle, haystack []string) bool {
	set := map[string]struct{}{}
	for _, h := range haystack {
		set[h] = struct{}{}
	}
	for _, n := range needle {
		if _, ok := set[n]; !ok {
			return false
		}
	}
	return true
}
