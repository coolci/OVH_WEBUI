package app

import (
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

// ServerListCache 服务器列表内存缓存
type ServerListCache struct {
	mu        sync.RWMutex
	Data      []types.ServerPlan
	Timestamp *time.Time
	TTL       time.Duration
}

// NewServerListCache 默认 2 小时 TTL（懒加载：仅访问触发刷新，无后台定时器）
func NewServerListCache() *ServerListCache {
	return &ServerListCache{TTL: 2 * time.Hour}
}

// Get 返回缓存副本和是否有效
func (s *ServerListCache) Get() ([]types.ServerPlan, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Timestamp == nil {
		return nil, false
	}
	valid := time.Since(*s.Timestamp) < s.TTL
	cp := make([]types.ServerPlan, len(s.Data))
	copy(cp, s.Data)
	return cp, valid
}

// Set 更新缓存，时间戳=NOW
func (s *ServerListCache) Set(data []types.ServerPlan) {
	s.SetAt(data, time.Now())
}

// SetAt 用指定时间戳更新缓存。
// 启动时从 SQLite 回灌历史数据要用这个，保留真实的 updated_at，
// 否则旧数据被当作刚拉的，过期判断会出错。
func (s *ServerListCache) SetAt(data []types.ServerPlan, ts time.Time) {
	s.mu.Lock()
	s.Data = data
	s.Timestamp = &ts
	s.mu.Unlock()
}

// State 聚合所有共享运行状态
type State struct {
	Paths       storage.Paths
	Config      *config.Store
	OVH         *ovh.Factory
	Logger      *logger.Logger
	ServerCache *ServerListCache
	DB          *db.DB // SQLite 持久化层

	APIKey string
	Port   string

	// 多账户:内存里持有全部 OVH 账户副本(启动从 SQLite 加载),
	// OVH Factory 通过 FindAccount 闭包按 id 查询
	AccountsMu sync.RWMutex
	Accounts   []types.OVHAccount

	QueueMu sync.Mutex
	Queue   []types.QueueItem

	HistoryMu sync.Mutex
	History   []types.PurchaseHistoryEntry

	ServerPlansMu sync.RWMutex
	ServerPlans   []types.ServerPlan

	DeletedTaskIDsMu sync.Mutex
	DeletedTaskIDs   map[string]struct{}

	VPSSubsMu        sync.Mutex
	VPSSubscriptions []types.VPSSubscription
	VPSCheckInterval int

	MonitorRunning        bool
	QueueProcessorRunning bool

	// 串行化全表 Replace 落盘，避免并发 SaveHistory/SaveQueue 快照互相覆盖丢数据
	historyPersistMu sync.Mutex
	queuePersistMu   sync.Mutex
}

// NewState 构造应用状态。DB 必须已 Open。
func NewState(paths storage.Paths, cfg *config.Store, lg *logger.Logger, sqliteDB *db.DB) *State {
	s := &State{
		Paths:                 paths,
		Config:                cfg,
		Logger:                lg,
		ServerCache:           NewServerListCache(),
		DB:                    sqliteDB,
		DeletedTaskIDs:        make(map[string]struct{}),
		Accounts:              []types.OVHAccount{},
		Queue:                 []types.QueueItem{},
		History:               []types.PurchaseHistoryEntry{},
		ServerPlans:           []types.ServerPlan{},
		VPSSubscriptions:      []types.VPSSubscription{},
		VPSCheckInterval:      60,
		QueueProcessorRunning: true,
	}
	// Factory 闭包注入 lookup,允许按 id 查账户(空 id → 默认)
	s.OVH = ovh.NewFactory(cfg, s.FindAccount)
	return s
}

// HasAnyAccount 是否至少有一个 OVH 账户。
// 多账户场景下,旧的 state.Config.HasCredentials() 不再可靠(新用户的 kv['config'] 可能为空),
// 凡是判断"系统能不能调 OVH"都应该走这个。
func (s *State) HasAnyAccount() bool {
	s.AccountsMu.RLock()
	defer s.AccountsMu.RUnlock()
	return len(s.Accounts) > 0
}

// FindAccount 多账户查找。id="" 返回默认账户(没默认 → 第一个);否则按 ID 精确匹配。
// OVH Factory 的 lookup 走这个,所有 ClientFor(accountID) 都会绕一圈到这里。
func (s *State) FindAccount(id string) (types.OVHAccount, bool) {
	s.AccountsMu.RLock()
	defer s.AccountsMu.RUnlock()
	if id == "" {
		for _, a := range s.Accounts {
			if a.IsDefault {
				return a, true
			}
		}
		if len(s.Accounts) > 0 {
			return s.Accounts[0], true
		}
		return types.OVHAccount{}, false
	}
	for _, a := range s.Accounts {
		if a.ID == id {
			return a, true
		}
	}
	return types.OVHAccount{}, false
}

// ReloadAccounts 从 SQLite 重新加载账户到内存,并把整个 OVH client 缓存清掉,
// 强制下次 ClientFor() 用最新凭据重建。
// 账户 CRUD 操作完成后调一次。
func (s *State) ReloadAccounts() error {
	accs, err := s.DB.ListAccounts()
	if err != nil {
		return err
	}
	s.AccountsMu.Lock()
	if accs == nil {
		accs = []types.OVHAccount{}
	}
	s.Accounts = accs
	s.AccountsMu.Unlock()
	s.OVH.InvalidateAll()
	return nil
}

// LoadAll 启动时从 SQLite 加载全部持久化数据到内存。
// 列表字段保证非 nil（JSON 序列化为 [] 而非 null）。
func (s *State) LoadAll() {
	// accounts: 必须最先加载,因为别的数据/loop 都按 account_id 索引
	s.migrateLegacyConfigToAccount() // 老用户从 kv['config'] 自动建默认账户
	if accs, err := s.DB.ListAccounts(); err == nil {
		if accs == nil {
			accs = []types.OVHAccount{}
		}
		s.AccountsMu.Lock()
		s.Accounts = accs
		s.AccountsMu.Unlock()
		s.Logger.Info("已加载 OVH 账户: "+intStr(len(accs))+" 个", "system")
	} else {
		s.Logger.Error("load accounts: "+err.Error(), "system")
	}

	// queue
	if items, err := s.DB.ListQueue(); err == nil {
		s.Queue = items
	} else {
		s.Logger.Error("load queue: "+err.Error(), "system")
	}
	if s.Queue == nil {
		s.Queue = []types.QueueItem{}
	}

	// history
	if items, err := s.DB.ListHistory(); err == nil {
		s.History = items
	} else {
		s.Logger.Error("load history: "+err.Error(), "system")
	}
	if s.History == nil {
		s.History = []types.PurchaseHistoryEntry{}
	}

	// servers
	if plans, err := s.DB.ListServers(); err == nil && len(plans) > 0 {
		s.ServerPlans = plans
		// 用 SQLite 里真实的 updated_at 重建缓存时间戳，
		// 这样过期的旧数据下次访问能正确触发刷新；NOW 会导致旧数据被当作"刚刷的"。
		if tsMs, err := s.DB.ServersUpdatedAt(); err == nil && tsMs > 0 {
			s.ServerCache.SetAt(plans, time.UnixMilli(tsMs))
		} else {
			s.ServerCache.Set(plans)
		}
		s.Logger.Info("已从 SQLite 加载服务器目录并同步到缓存", "system")
	} else if err != nil {
		s.Logger.Error("load servers: "+err.Error(), "system")
	}
	if s.ServerPlans == nil {
		s.ServerPlans = []types.ServerPlan{}
	}

	// vps subscriptions
	if subs, err := s.DB.ListVPSSubscriptions(); err == nil {
		s.VPSSubscriptions = subs
	} else {
		s.Logger.Error("load vps subs: "+err.Error(), "system")
	}
	if s.VPSSubscriptions == nil {
		s.VPSSubscriptions = []types.VPSSubscription{}
	}
	// vps check interval 存 kv
	var ci int
	if ok, _ := s.DB.GetKV("vps_check_interval", &ci); ok && ci > 0 {
		s.VPSCheckInterval = ci
	}
}

// CountActiveQueues 统计未完成的队列项
func (s *State) CountActiveQueues() int {
	s.QueueMu.Lock()
	defer s.QueueMu.Unlock()
	cnt := 0
	for _, it := range s.Queue {
		if it.Status == "running" || it.Status == "pending" || it.Status == "paused" {
			cnt++
		}
	}
	return cnt
}

// CountAvailableServers 统计有库存的型号
func (s *State) CountAvailableServers() int {
	s.ServerPlansMu.RLock()
	defer s.ServerPlansMu.RUnlock()
	cnt := 0
	for _, p := range s.ServerPlans {
		for _, dc := range p.Datacenters {
			if dc.Availability != "unavailable" && dc.Availability != "unknown" {
				cnt++
				break
			}
		}
	}
	return cnt
}

// CountPurchase 统计成功/失败订单数
func (s *State) CountPurchase() (success, failed int) {
	s.HistoryMu.Lock()
	defer s.HistoryMu.Unlock()
	for _, h := range s.History {
		switch h.Status {
		case "success":
			success++
		case "failed":
			failed++
		}
	}
	return
}

// SaveQueue 把内存中 Queue 整表覆盖写入 SQLite（串行化，取最新快照）
func (s *State) SaveQueue() error {
	s.queuePersistMu.Lock()
	defer s.queuePersistMu.Unlock()
	s.QueueMu.Lock()
	cp := make([]types.QueueItem, len(s.Queue))
	copy(cp, s.Queue)
	s.QueueMu.Unlock()
	return s.DB.ReplaceQueue(cp)
}

// SaveHistory 把内存中 History 整表覆盖写入 SQLite（串行化，取最新快照）
func (s *State) SaveHistory() error {
	s.historyPersistMu.Lock()
	defer s.historyPersistMu.Unlock()
	s.HistoryMu.Lock()
	cp := make([]types.PurchaseHistoryEntry, len(s.History))
	copy(cp, s.History)
	s.HistoryMu.Unlock()
	return s.DB.ReplaceHistory(cp)
}

// SaveServers 把内存中 ServerPlans 整表覆盖写入 SQLite
func (s *State) SaveServers() error {
	s.ServerPlansMu.RLock()
	cp := make([]types.ServerPlan, len(s.ServerPlans))
	copy(cp, s.ServerPlans)
	s.ServerPlansMu.RUnlock()
	return s.DB.ReplaceServers(cp)
}

// migrateLegacyConfigToAccount 老用户升级:如果 SQLite 里没账户但 kv['config'] 有
// 完整 OVH 凭据,自动把它建成一个名为"默认账户"的 OVHAccount,设默认,
// 并把现有所有 queue/history/sniper_task 的 account_id 列回填指向它。
// 已经有账户的话什么都不做,幂等。
func (s *State) migrateLegacyConfigToAccount() {
	n, err := s.DB.CountAccounts()
	if err != nil {
		s.Logger.Error("count accounts: "+err.Error(), "system")
		return
	}
	if n > 0 {
		return // 已经有账户,跳过
	}
	cfg := s.Config.Get()
	if cfg.AppKey == "" || cfg.AppSecret == "" || cfg.ConsumerKey == "" {
		// 没老凭据,首次安装,等用户在 OvhCredsGate 创建第一个账户
		return
	}
	zone := cfg.Zone
	if zone == "" {
		zone = "IE"
	}
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "ovh-eu"
	}
	iam := cfg.IAM
	if iam == "" {
		iam = "go-ovh-" + strings.ToLower(zone)
	}
	acc := types.OVHAccount{
		ID:          uuid.NewString(),
		Name:        "默认账户",
		Endpoint:    endpoint,
		Zone:        zone,
		AppKey:      cfg.AppKey,
		AppSecret:   cfg.AppSecret,
		ConsumerKey: cfg.ConsumerKey,
		IAM:         iam,
		IsDefault:   true,
		CreatedAt:   types.NowISO(),
	}
	if err := s.DB.UpsertAccount(acc); err != nil {
		s.Logger.Error("migrate legacy config to account: "+err.Error(), "system")
		return
	}
	// 回填现有数据的 account_id 列(从空值 → 新账户 ID)
	for _, stmt := range []string{
		`UPDATE queue SET account_id = ? WHERE account_id = '' OR account_id IS NULL`,
		`UPDATE history SET account_id = ? WHERE account_id = '' OR account_id IS NULL`,
		`UPDATE config_sniper_tasks SET account_id = ? WHERE account_id = '' OR account_id IS NULL`,
	} {
		if _, err := s.DB.Exec(stmt, acc.ID); err != nil {
			s.Logger.Warn("backfill account_id: "+err.Error(), "system")
		}
	}
	s.Logger.Info("已把旧 kv['config'] 迁移成默认账户: "+acc.Name+" ("+acc.Zone+")", "system")
}

// intStr 小工具,避免在 LoadAll 里临时引 strconv
func intStr(n int) string {
	// 简化版,只处理 0..9999
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// SaveAll 一次性保存所有数据
func (s *State) SaveAll() {
	if err := s.SaveQueue(); err != nil {
		s.Logger.Error("save queue: "+err.Error(), "system")
	}
	if err := s.SaveHistory(); err != nil {
		s.Logger.Error("save history: "+err.Error(), "system")
	}
	if err := s.SaveServers(); err != nil {
		s.Logger.Error("save servers: "+err.Error(), "system")
	}
}
