package purchase

import (
	"sort"
	"sync"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

const concurrentBatchSize = 10

// ProcessQueueLoop 对应 Python: process_queue
func ProcessQueueLoop(state *app.State) {
	for {
		state.QueueMu.Lock()
		queueEmpty := len(state.Queue) == 0
		state.QueueMu.Unlock()
		if queueEmpty {
			// 队列为空：清理删除标记
			state.DeletedTaskIDsMu.Lock()
			if len(state.DeletedTaskIDs) > 0 {
				state.DeletedTaskIDs = make(map[string]struct{})
				state.Logger.Debug("队列为空，清理删除标记集合", "queue")
			}
			state.DeletedTaskIDsMu.Unlock()
			time.Sleep(time.Second)
			continue
		}

		// 清理已从队列移除的删除标记
		state.QueueMu.Lock()
		currentIDs := map[string]struct{}{}
		for _, it := range state.Queue {
			currentIDs[it.ID] = struct{}{}
		}
		state.QueueMu.Unlock()
		state.DeletedTaskIDsMu.Lock()
		removed := 0
		for id := range state.DeletedTaskIDs {
			if _, ok := currentIDs[id]; !ok {
				delete(state.DeletedTaskIDs, id)
				removed++
			}
		}
		state.DeletedTaskIDsMu.Unlock()
		if removed > 0 {
			state.Logger.Debug("清理 N 个已从队列移除的删除标记", "queue")
		}

		// 按优先级排序：quickOrder 优先，其次按创建时间倒序
		state.QueueMu.Lock()
		sorted := make([]types.QueueItem, len(state.Queue))
		copy(sorted, state.Queue)
		state.QueueMu.Unlock()
		sort.SliceStable(sorted, func(i, j int) bool {
			a, b := sorted[i], sorted[j]
			ap, bp := 1, 1
			if a.QuickOrder {
				ap = 0
			}
			if b.QuickOrder {
				bp = 0
			}
			if ap != bp {
				return ap < bp
			}
			at, _ := time.Parse(time.RFC3339Nano, a.CreatedAt)
			bt, _ := time.Parse(time.RFC3339Nano, b.CreatedAt)
			return at.After(bt)
		})

		current := time.Now().Unix()
		ready := []types.QueueItem{}
		state.DeletedTaskIDsMu.Lock()
		deletedSnap := make(map[string]struct{}, len(state.DeletedTaskIDs))
		for k := range state.DeletedTaskIDs {
			deletedSnap[k] = struct{}{}
		}
		state.DeletedTaskIDsMu.Unlock()

		// 再快照一次 queue id 集合，用于复核"任务是否还在队列中"（1:1 对应 Python app.py:1185-1188）
		state.QueueMu.Lock()
		queueIDs := make(map[string]struct{}, len(state.Queue))
		for _, q := range state.Queue {
			queueIDs[q.ID] = struct{}{}
		}
		state.QueueMu.Unlock()

		for _, it := range sorted {
			if _, del := deletedSnap[it.ID]; del {
				continue
			}
			// 复核：sorted 是 snapshot，与此同时用户可能删过；如果不在 queue 里则标记 deleted 并跳过
			if _, exists := queueIDs[it.ID]; !exists {
				state.DeletedTaskIDsMu.Lock()
				state.DeletedTaskIDs[it.ID] = struct{}{}
				state.DeletedTaskIDsMu.Unlock()
				continue
			}
			if it.Status != "running" {
				continue
			}
			if it.LastCheckTime == 0 || float64(current)-it.LastCheckTime >= float64(it.RetryInterval) {
				ready = append(ready, it)
			}
		}

		if len(ready) > 0 {
			state.Logger.Debug("准备并发处理 N 个订单", "queue")
			processedIDs := []string{}
			var procMu sync.Mutex

			processSingle := func(it types.QueueItem) {
				state.DeletedTaskIDsMu.Lock()
				_, deleted := state.DeletedTaskIDs[it.ID]
				state.DeletedTaskIDsMu.Unlock()
				if deleted {
					return
				}

				// 更新检查时间、重试次数
				state.QueueMu.Lock()
				var current *types.QueueItem
				for i := range state.Queue {
					if state.Queue[i].ID == it.ID {
						current = &state.Queue[i]
						break
					}
				}
				if current == nil {
					state.QueueMu.Unlock()
					return
				}
				isFirstAttempt := current.LastCheckTime == 0
				current.LastCheckTime = float64(time.Now().Unix())
				current.RetryCount++
				current.UpdatedAt = types.NowISO()
				finalRetry := current.RetryCount
				maxRetries := current.MaxRetries
				snapshot := *current
				state.QueueMu.Unlock()

				if isFirstAttempt {
					state.Logger.Info("首次尝试任务 "+it.ID+": "+it.PlanCode+" 在 "+it.Datacenter, "queue")
				} else {
					state.Logger.Info("重试检查任务 "+it.ID+": "+it.PlanCode+" 在 "+it.Datacenter, "queue")
				}

				success := PurchaseServer(state, &snapshot)
				if success {
					state.QueueMu.Lock()
					for i := range state.Queue {
						if state.Queue[i].ID == it.ID {
							state.Queue[i].Status = "completed"
							state.Queue[i].UpdatedAt = types.NowISO()
							break
						}
					}
					state.QueueMu.Unlock()
					procMu.Lock()
					processedIDs = append(processedIDs, it.ID)
					procMu.Unlock()
					if finalRetry == 1 {
						state.Logger.Info("首次尝试购买成功: "+it.PlanCode, "queue")
					} else {
						state.Logger.Info("重试购买成功: "+it.PlanCode, "queue")
					}
				} else {
					// MaxRetries>0 时达到上限则终止任务（0 = 无限抢购，不终止）
					if maxRetries > 0 && finalRetry >= maxRetries {
						state.QueueMu.Lock()
						for i := range state.Queue {
							if state.Queue[i].ID == it.ID {
								state.Queue[i].Status = "failed"
								state.Queue[i].UpdatedAt = types.NowISO()
								break
							}
						}
						state.QueueMu.Unlock()
						procMu.Lock()
						processedIDs = append(processedIDs, it.ID)
						procMu.Unlock()
						state.Logger.Info("任务达到 MaxRetries 上限已终止: "+it.PlanCode+" ("+it.ID+")", "queue")
					} else if finalRetry == 1 {
						state.Logger.Info("首次尝试购买失败或服务器暂无货: "+it.PlanCode, "queue")
					} else {
						state.Logger.Info("重试购买失败或服务器仍无货: "+it.PlanCode, "queue")
					}
				}
			}

			// 分批并发
			totalBatches := (len(ready) + concurrentBatchSize - 1) / concurrentBatchSize
			for batchIdx := 0; batchIdx < totalBatches; batchIdx++ {
				start := batchIdx * concurrentBatchSize
				end := start + concurrentBatchSize
				if end > len(ready) {
					end = len(ready)
				}
				batch := ready[start:end]
				var wg sync.WaitGroup
				for _, it := range batch {
					wg.Add(1)
					go func(item types.QueueItem) {
						defer wg.Done()
						processSingle(item)
					}(it)
				}
				wg.Wait()
				state.Logger.Debug("批次完成", "queue")
			}

			// 从队列移除已完成
			if len(processedIDs) > 0 {
				procSet := map[string]struct{}{}
				for _, id := range processedIDs {
					procSet[id] = struct{}{}
				}
				state.QueueMu.Lock()
				kept := state.Queue[:0]
				for _, it := range state.Queue {
					if _, ok := procSet[it.ID]; !ok {
						kept = append(kept, it)
					}
				}
				state.Queue = kept
				state.QueueMu.Unlock()
				state.Logger.Info("已从队列移除 N 个已完成的订单", "queue")
			}

			_ = state.SaveQueue()
		}

		time.Sleep(time.Second)
	}
}
