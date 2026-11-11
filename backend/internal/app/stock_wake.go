package app

import (
	"strings"

	"github.com/ovh-webui/server/internal/ovh"
)

// 监控发现有货 → 立刻唤醒同型号的队列任务。
//
// 队列任务按各自的重试间隔查库存(默认 60 秒),而监控每 5 秒查一次。
// 以前监控看到补货并不会通知队列,同型号的队列任务要等满自己的间隔才开始下单,
// 最多晚将近 1 分钟。这里只做一件事:让匹配的任务"马上到期"。
// 任务醒来后照常先用自己的账户查库存、再结账,防重复 / 暂停 / 取消检查一个不少;
// 唤醒错了(比如任务要的配件不是这一种)也只是多查一次库存。

func normWakeDC(dc string) string {
	dc = strings.TrimSpace(dc)
	if dc == "" {
		return ""
	}
	return ovh.ConvertDisplayDCToAPIDC(dc)
}

// WakeQueueForStock 监控刚观测到 planCode 在 datacenter 有货(查询所用账户的 endpoint 为 endpoint)。
// 把同型号、同机房、同大区的运行中队列任务标为立即到期,并唤醒队列处理器。返回唤醒的任务数。
// endpoint 为空时不按大区过滤。
func (s *State) WakeQueueForStock(planCode, datacenter, endpoint string) int {
	if s == nil || planCode == "" {
		return 0
	}
	dc := normWakeDC(datacenter)
	if dc == "" {
		return 0
	}
	type cand struct{ id, accountID string }
	var cands []cand
	s.QueueMu.Lock()
	for _, it := range s.Queue {
		if it.Status == "running" && it.PlanCode == planCode && normWakeDC(it.Datacenter) == dc {
			cands = append(cands, cand{it.ID, it.AccountID})
		}
	}
	s.QueueMu.Unlock()
	if len(cands) == 0 {
		return 0
	}

	// 三个站点(欧 / 美 / 加)的库存视图互相独立:只唤醒和监控查询账户同一大区的任务。
	// 账户查找放在 QueueMu 外面,避免两把锁的加锁顺序问题。
	region := ""
	if strings.TrimSpace(endpoint) != "" {
		region = ovh.EndpointRegion(endpoint)
	}
	var ids []string
	for _, c := range cands {
		if region != "" {
			if acc, ok := s.FindAccount(c.accountID); ok && ovh.EndpointRegion(acc.Endpoint) != region {
				continue
			}
		}
		ids = append(ids, c.id)
	}
	if len(ids) == 0 {
		return 0
	}
	s.stockWakeMu.Lock()
	if s.stockWake == nil {
		s.stockWake = map[string]struct{}{}
	}
	for _, id := range ids {
		s.stockWake[id] = struct{}{}
	}
	s.stockWakeMu.Unlock()
	s.WakeQueueProcessor()
	return len(ids)
}

// TakeStockWake 队列处理器取走某个任务的"立即到期"标记(取走即清除)。
// RestoreStockWake 把取走但没能派发的唤醒标记放回去(任务在途 / 并发槽已满时),
// 下一次任务返回唤醒循环后它仍会被立即检查。不触发 WakeQueueProcessor:
// 任务返回时自己会唤醒循环,这里再唤醒只会空转一轮。
func (s *State) RestoreStockWake(taskID string) {
	if s == nil || taskID == "" {
		return
	}
	s.stockWakeMu.Lock()
	if s.stockWake == nil {
		s.stockWake = map[string]struct{}{}
	}
	s.stockWake[taskID] = struct{}{}
	s.stockWakeMu.Unlock()
}

func (s *State) TakeStockWake(taskID string) bool {
	if s == nil {
		return false
	}
	s.stockWakeMu.Lock()
	defer s.stockWakeMu.Unlock()
	if _, ok := s.stockWake[taskID]; !ok {
		return false
	}
	delete(s.stockWake, taskID)
	return true
}
