// Package order provides application business services for the automated purchase queue and history.
//
// 依赖方向:handlers → order.Service → QueueStore/HistoryStore(由 app.State 实现)→ db。
// Service 只依赖本包定义的存储接口,不 import app,也不直接碰数据库 ——
// 运行时内存才是队列真相源,绕过它写库会被下一次整表落库覆盖。
package order

import (
	"errors"
	"fmt"

	"github.com/ovh-webui/server/internal/types"
)

var (
	// ErrItemNotFound 任务不存在
	ErrItemNotFound = errors.New("queue item not found")
	// ErrInvalidInterval 重试间隔超出允许范围
	ErrInvalidInterval = fmt.Errorf("重试间隔必须是 %d ~ %d 之间的整数秒", types.MinRetryInterval, types.MaxRetryInterval)
	// ErrInvalidStatus 不允许由用户设置的状态
	ErrInvalidStatus = errors.New("状态只能是 running / pending / paused")
	// ErrPersist 内存已生效但落库失败(重启后会丢失这次修改)
	ErrPersist = errors.New("persist failed")
)

// 用户可手动设置的任务状态。completed / failed 只能由队列处理器写入。
const (
	StatusRunning = "running"
	StatusPending = "pending"
	StatusPaused  = "paused"
)

// QueueStore 队列的"内存 + 落库"存储契约。
type QueueStore interface {
	QueueSnapshot() []types.QueueItem
	EnqueueItems(items []types.QueueItem, prepend bool) error
	// RemoveQueueItem 返回被删的副本(不存在为 nil);err 表示落库失败
	RemoveQueueItem(id string) (*types.QueueItem, error)
	ClearQueueItems() ([]types.QueueItem, error)
	// MutateQueueItem found=false 表示不存在;found=true 且 err!=nil 表示落库失败
	MutateQueueItem(id string, fn func(*types.QueueItem)) (types.QueueItem, bool, error)
}

// HistoryStore 购买历史的"内存 + 落库"存储契约。
type HistoryStore interface {
	HistorySnapshot() []types.PurchaseHistoryEntry
	ClearHistoryEntries() error
}

// Service 队列与购买历史的应用服务。
type Service interface {
	ListQueue() []types.QueueItem
	Enqueue(item types.QueueItem, prepend bool) error
	RemoveQueueItem(id string) (*types.QueueItem, error)
	ClearQueue() ([]types.QueueItem, error)
	UpdateQueueInterval(id string, seconds int) error
	UpdateQueueStatus(id, status string) (types.QueueItem, error)
	ListHistory() []types.PurchaseHistoryEntry
	ClearHistory() error
}

type orderService struct {
	queue   QueueStore
	history HistoryStore
}

// NewService creates a new order application service.
func NewService(q QueueStore, h HistoryStore) Service {
	return &orderService{queue: q, history: h}
}

func persistErr(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", ErrPersist, err)
}

func (s *orderService) ListQueue() []types.QueueItem {
	return s.queue.QueueSnapshot()
}

// Enqueue 入队并落库;失败时 store 会把这条从内存撤回,不存在半成功状态。
func (s *orderService) Enqueue(item types.QueueItem, prepend bool) error {
	return s.queue.EnqueueItems([]types.QueueItem{item}, prepend)
}

func (s *orderService) RemoveQueueItem(id string) (*types.QueueItem, error) {
	removed, err := s.queue.RemoveQueueItem(id)
	return removed, persistErr(err)
}

func (s *orderService) ClearQueue() ([]types.QueueItem, error) {
	old, err := s.queue.ClearQueueItems()
	return old, persistErr(err)
}

// UpdateQueueInterval 修改重试间隔(单位:秒,与 QueueItem.RetryInterval 一致)。
func (s *orderService) UpdateQueueInterval(id string, seconds int) error {
	if seconds < types.MinRetryInterval || seconds > types.MaxRetryInterval {
		return ErrInvalidInterval
	}
	_, found, err := s.queue.MutateQueueItem(id, func(it *types.QueueItem) {
		it.RetryInterval = seconds
	})
	if !found {
		return ErrItemNotFound
	}
	return persistErr(err)
}

// UpdateQueueStatus 用户暂停 / 恢复任务。空状态按 pending 处理(兼容旧客户端)。
func (s *orderService) UpdateQueueStatus(id, status string) (types.QueueItem, error) {
	if status == "" {
		status = StatusPending
	}
	switch status {
	case StatusRunning, StatusPending, StatusPaused:
	default:
		return types.QueueItem{}, ErrInvalidStatus
	}
	item, found, err := s.queue.MutateQueueItem(id, func(it *types.QueueItem) {
		it.Status = status
	})
	if !found {
		return types.QueueItem{}, ErrItemNotFound
	}
	return item, persistErr(err)
}

func (s *orderService) ListHistory() []types.PurchaseHistoryEntry {
	return s.history.HistorySnapshot()
}

func (s *orderService) ClearHistory() error {
	return persistErr(s.history.ClearHistoryEntries())
}
