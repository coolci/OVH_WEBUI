package app

import (
	"sync"
	"time"

	"github.com/ovh-webui/server/internal/types"
)

// DefaultServerBucket 默认账户视角的缓存桶 key。
// SQLite 里的 servers 只有一张全局表（没有 account 维度），启动回灌和落盘都只认这个桶。
const DefaultServerBucket = ""

// serverListBucket 一个账户视角下的目录快照
type serverListBucket struct {
	data []types.ServerPlan
	ts   time.Time
}

// ServerListCache 服务器列表内存缓存，按"账户视角"分桶（懒加载：仅访问触发刷新，无后台定时器）。
//
// 为什么要分桶：目录内容由账户的 endpoint(EU/US/CA) + ovhSubsidiary 共同决定。
// 单桶时 ?account=<US 账户> 的请求在 TTL 内会直接命中默认(EU)账户那份目录，
// 用户切了账户却看到别人的机型集合；而"非默认账户干脆不写缓存"的保守做法
// 又会让这些账户每次刷新都重拉全部 plan，把 OVH 429 风险放大回去。分桶后两头都解决。
type ServerListCache struct {
	mu      sync.RWMutex
	buckets map[string]*serverListBucket
	TTL     time.Duration
}

// NewServerListCache 默认 120 秒 TTL
func NewServerListCache() *ServerListCache {
	return &ServerListCache{TTL: 120 * time.Second, buckets: map[string]*serverListBucket{}}
}

// GetBucket 返回指定账户视角的缓存副本和是否仍在 TTL 内
func (s *ServerListCache) GetBucket(key string) ([]types.ServerPlan, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b := s.buckets[key]
	if b == nil {
		return nil, false
	}
	cp := make([]types.ServerPlan, len(b.data))
	copy(cp, b.data)
	return cp, time.Since(b.ts) < s.TTL
}

// TimestampOf 指定桶的写入时间；桶不存在返回 nil。
// 以前 Timestamp 是导出字段、被 handler 直接裸读，跟 Set 之间没有同步；
// 改成带锁的方法顺手把这个数据竞争一起消掉。
func (s *ServerListCache) TimestampOf(key string) *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b := s.buckets[key]
	if b == nil {
		return nil
	}
	ts := b.ts
	return &ts
}

// SetBucket 更新指定桶，时间戳=NOW；data 为空表示删除该桶
func (s *ServerListCache) SetBucket(key string, data []types.ServerPlan) {
	s.SetBucketAt(key, data, time.Now())
}

// SetBucketAt 用指定时间戳更新指定桶。
// 启动时从 SQLite 回灌历史数据要用这个，保留真实的 updated_at，
// 否则旧数据被当作刚拉的，过期判断会出错。
func (s *ServerListCache) SetBucketAt(key string, data []types.ServerPlan, ts time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(data) == 0 {
		delete(s.buckets, key)
		return
	}
	cp := make([]types.ServerPlan, len(data))
	copy(cp, data)
	s.buckets[key] = &serverListBucket{data: cp, ts: ts}
}

// Clear 清空全部桶（"清内存缓存"语义：所有账户视角一起清）
func (s *ServerListCache) Clear() {
	s.mu.Lock()
	s.buckets = map[string]*serverListBucket{}
	s.mu.Unlock()
}

// Get / Set / SetAt 是默认桶的简写，保留给不关心账户的调用方
func (s *ServerListCache) Get() ([]types.ServerPlan, bool) { return s.GetBucket(DefaultServerBucket) }

func (s *ServerListCache) Set(data []types.ServerPlan) { s.SetBucket(DefaultServerBucket, data) }

func (s *ServerListCache) SetAt(data []types.ServerPlan, ts time.Time) {
	s.SetBucketAt(DefaultServerBucket, data, ts)
}
