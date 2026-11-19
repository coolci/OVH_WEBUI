package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/repository"
	"github.com/ovh-webui/server/internal/service/account"
	"github.com/ovh-webui/server/internal/service/catalog"
	"github.com/ovh-webui/server/internal/service/monitor"
	"github.com/ovh-webui/server/internal/service/order"
	"github.com/ovh-webui/server/internal/service/server"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

// CatalogWatchController owns the independent server-list discovery lifecycle.
type CatalogWatchController interface {
	SetEnabled(context.Context, bool) error
	Snapshot() interface{}
	Run(context.Context)
}

// State 聚合所有共享运行状态
type State struct {
	CatalogWatchOnce sync.Once
	CatalogWatch     CatalogWatchController
	Paths            storage.Paths
	Config           *config.Store
	OVH              *ovh.Factory
	Logger           *logger.Logger
	ServerCache      *ServerListCache
	DB               *db.DB // SQLite 持久化层

	APIKey string
	Port   string

	// 多账户:内存里持有全部 OVH 账户副本(启动从 SQLite 加载),
	// OVH Factory 通过 FindAccount 闭包按 id 查询
	AccountsMu sync.RWMutex
	Accounts   []types.OVHAccount

	QueueMu     sync.Mutex
	Queue       []types.QueueItem
	queueWakeCh chan struct{}
	// stockWake 监控发现有货后被标为"立即到期"的队列任务 id(见 stock_wake.go)
	stockWakeMu sync.Mutex
	stockWake   map[string]struct{}
	// QueueMu 同时保护用户对监控自动建单的停止记录。
	automaticOrderBlocks   map[string]bool
	pausedMonitorScopes    []MonitorQueueScope
	monitorManuallyStopped bool

	HistoryMu sync.Mutex
	History   []types.PurchaseHistoryEntry

	ServerPlansMu sync.RWMutex
	ServerPlans   []types.ServerPlan

	DeletedTaskIDsMu sync.Mutex
	DeletedTaskIDs   map[string]struct{}

	// 正在跑 PurchaseServer 的任务 → 取消函数。
	// DeletedTaskIDs 只是个标记,处理器要到下一轮才会看它;而一轮下单链路有 10 次
	// OVH 调用、每次最长 60s。用户删任务的那一刻如果链路正跑到一半,这里的 cancel
	// 让正在进行的 HTTP 调用立刻中断,而不是把这一轮跑完 —— 包括结账。
	taskCancelMu sync.Mutex
	taskCancel   map[string]context.CancelFunc

	VPSSubsMu sync.Mutex

	// 保存串行化锁。Save* 是"快照 + 全表覆盖",两个并发保存里
	// 晚拍快照的可能先落库,把新数据覆盖掉 —— 必须让"拍快照到写完"整段串行。
	// 和上面那些数据锁分开:数据锁保护内存读写(要短),这些保护落库顺序(会持有到 IO 结束)。
	saveQueueMu   sync.Mutex
	saveHistoryMu sync.Mutex
	saveServersMu sync.Mutex

	// 启动时哪张表没读出来。
	//
	// Save* 全是"DELETE 整表 + 重新 INSERT 内存快照"。LoadAll 读失败时只记了条日志,
	// 内存留空,于是**第一次保存就把那张表整个抹了** —— 加一列忘了同步结构体、
	// 库文件被别的进程锁住、磁盘临时 IO 错,任何一种都够把用户全部抢购历史/队列
	// 永久删掉,而且删得静悄悄。
	//
	// 所以:读失败的表一律禁止再写。宁可这次运行不落库,也不能拿空内存去覆盖磁盘上
	// 那份还完好的数据。用户重启一次(或修好 schema)就能恢复。
	loadFailedMu     sync.RWMutex
	loadFailed       map[string]string
	VPSSubscriptions []types.VPSSubscription
	VPSCheckInterval int

	MonitorRunning        bool
	QueueProcessorRunning bool

	// onProxyError 代理故障回调,由 main 接到 proxyguard。
	onProxyError   func(accountID string, err error)
	onProxySuccess func(accountID string)

	// 领域应用服务层 (Clean Architecture Application Services)
	AccountService account.Service
	OrderService   order.Service
	CatalogService catalog.Service
	ServerService  server.Service
	MonitorService monitor.Service

	// 数据持久层契约 (Repository Layer Contracts)
	DeviceRepo   repository.DeviceRepository
	TelegramRepo repository.TelegramRepository
	ServerRepo   repository.ServerRepository
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
		taskCancel:            make(map[string]context.CancelFunc),
		Accounts:              []types.OVHAccount{},
		Queue:                 []types.QueueItem{},
		queueWakeCh:           make(chan struct{}, 1),
		History:               []types.PurchaseHistoryEntry{},
		ServerPlans:           []types.ServerPlan{},
		VPSSubscriptions:      []types.VPSSubscription{},
		VPSCheckInterval:      60,
		QueueProcessorRunning: true,
	}
	// 队列/历史以内存为真相源:OrderService 依赖 State 自身实现的 QueueStore/HistoryStore,
	// 而不是直接依赖 DB(否则会被下一次整表落库覆盖)。
	s.OrderService = order.NewService(s, s)
	if sqliteDB != nil {
		s.AccountService = account.NewService(sqliteDB)
		s.CatalogService = catalog.NewService(sqliteDB)
		s.ServerService = server.NewService(sqliteDB)
		s.MonitorService = monitor.NewService(sqliteDB)

		s.DeviceRepo = sqliteDB
		s.TelegramRepo = sqliteDB
		s.ServerRepo = sqliteDB
	}
	// Factory 闭包注入 lookup,允许按 id 查账户(空 id → 默认)
	s.OVH = ovh.NewFactory(cfg, s.FindAccount)
	return s
}

// ClearServerCache 清理服务器内存与/或 SQLite 缓存。
func (s *State) ClearServerCache(cacheType string) []string {
	cleared := []string{}
	if cacheType == "all" || cacheType == "memory" {
		s.ServerPlansMu.Lock()
		s.ServerPlans = []types.ServerPlan{}
		s.ServerPlansMu.Unlock()
		s.ServerCache.Clear()
		cleared = append(cleared, "memory")
		s.Logger.Info("已清除内存缓存", "")
	}
	if cacheType == "all" || cacheType == "sqlite" {
		if s.ServerRepo != nil {
			if err := s.ServerRepo.ClearServers(); err != nil {
				s.Logger.Error("清除 SQLite 服务器缓存失败: "+err.Error(), "")
			} else {
				cleared = append(cleared, "sqlite_servers")
				s.Logger.Info("已清除 SQLite 服务器缓存", "")
			}
			if err := s.ServerRepo.ClearCatalogs(); err != nil {
				s.Logger.Error("清除 SQLite catalog 缓存失败: "+err.Error(), "")
			} else {
				cleared = append(cleared, "sqlite_catalogs")
				s.Logger.Info("已清除 SQLite catalog 缓存", "")
			}
		}
	}
	return cleared
}

// DBPath 返回底层 SQLite 文件路径(若未打开则为空)。
func (s *State) DBPath() string {
	if s.DB != nil {
		return s.DB.Path
	}
	return ""
}

// HasDB 底层 SQLite 数据库是否已就绪。
func (s *State) HasDB() bool {
	return s != nil && s.DB != nil
}

// GetKV 从底层 SQLite KV 表读取 JSON 值。
func (s *State) GetKV(key string, dst interface{}) (bool, error) {
	if s == nil || s.DB == nil {
		return false, errors.New("database not initialized")
	}
	return s.DB.GetKV(key, dst)
}

// SetKV 写入 JSON 值到底层 SQLite KV 表。
func (s *State) SetKV(key string, val interface{}) error {
	if s == nil || s.DB == nil {
		return errors.New("database not initialized")
	}
	return s.DB.SetKV(key, val)
}

// DeleteKV 从底层 SQLite KV 表删除指定键。
func (s *State) DeleteKV(key string) error {
	if s == nil || s.DB == nil {
		return errors.New("database not initialized")
	}
	return s.DB.DeleteKV(key)
}

// ClaimOrderPayment 原子登记某账户某订单的付款请求。
func (s *State) ClaimOrderPayment(accountID string, orderID int64) (bool, error) {
	if s == nil || s.DB == nil {
		return false, errors.New("database not initialized")
	}
	return s.DB.ClaimOrderPayment(accountID, orderID)
}

// GetOrderCancellation 查询某账户某订单的放弃/撤单状态。
func (s *State) GetOrderCancellation(accountID string, orderID int64) (db.OrderCancellation, error) {
	if s == nil || s.DB == nil {
		return db.OrderCancellation{}, errors.New("database not initialized")
	}
	return s.DB.GetOrderCancellation(accountID, orderID)
}

// ListOrderCancellations 读取全部订单放弃/撤单记录映射。
func (s *State) ListOrderCancellations() (map[string]db.OrderCancellation, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("database not initialized")
	}
	return s.DB.ListOrderCancellations()
}

// ChangeOrderCancellation 原子切换某订单的放弃/恢复/撤单申请状态。
func (s *State) ChangeOrderCancellation(accountID string, orderID int64, operation string) error {
	if s == nil || s.DB == nil {
		return errors.New("database not initialized")
	}
	return s.DB.ChangeOrderCancellation(accountID, orderID, operation)
}

// SetOrderRetractionState 记录 OVH 撤单调用后的最终状态（submitted / failed / uncertain）。
func (s *State) SetOrderRetractionState(accountID string, orderID int64, state string) error {
	if s == nil || s.DB == nil {
		return errors.New("database not initialized")
	}
	return s.DB.SetOrderRetractionState(accountID, orderID, state)
}

// HasAnyAccount 是否至少有一个 OVH 账户。
// 多账户场景下,旧的 state.Config.HasCredentials() 不再可靠(新用户的 kv['config'] 可能为空),
// 凡是判断"系统能不能调 OVH"都应该走这个。
// AccountsSnapshot 账户列表的副本(后台任务遍历用,不持锁做网络请求)
func (s *State) AccountsSnapshot() []types.OVHAccount {
	s.AccountsMu.RLock()
	defer s.AccountsMu.RUnlock()
	return append([]types.OVHAccount(nil), s.Accounts...)
}

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

// ServerCacheKey 目录缓存的分桶 key。
// 目录内容只由 endpoint(决定打哪个大区的 API) + zone(ovhSubsidiary，决定卖哪些机型) 决定，
// 所以按这两项分桶而不是按 accountID —— 同 endpoint 同 zone 的两个账户没必要各拉一遍 96 个 plan。
// 默认账户视角统一映射到 DefaultServerBucket：它对应 SQLite 里那张唯一的 servers 表，
// 启动回灌和落盘都认这个 key，账户查不到时也退到它（此时压根调不了 OVH）。
func (s *State) ServerCacheKey(accountID string) string {
	acc, ok := s.FindAccount(accountID)
	if !ok {
		return DefaultServerBucket
	}
	if def, defOK := s.FindAccount(""); defOK &&
		strings.EqualFold(acc.Endpoint, def.Endpoint) && strings.EqualFold(acc.Zone, def.Zone) {
		return DefaultServerBucket
	}
	return strings.ToLower(acc.Endpoint) + "|" + strings.ToUpper(acc.Zone)
}

// SyncDefaultAccountConfig 将当前默认账户的区域、IAM、Endpoint 与凭据同步到 Config,
// 确保全局设置 (GET /api/settings) 以及未传 ?account= 的默认路径与默认账户保持一致。
func (s *State) SyncDefaultAccountConfig() {
	if s.Config == nil {
		return
	}
	defAcc, ok := s.FindAccount("")
	if !ok {
		// Keep the only legacy credential source if its first migration failed.
		var migrated bool
		if _, err := s.DB.GetKV(db.LegacyAccountMigrationKey, &migrated); err != nil || !migrated {
			return
		}

		cfg := s.Config.Get()
		if cfg.AppKey != "" || cfg.AppSecret != "" || cfg.ConsumerKey != "" {
			cfg.AppKey, cfg.AppSecret, cfg.ConsumerKey = "", "", ""
			if err := s.Config.Set(cfg); err != nil && s.Logger != nil {
				s.Logger.Error("clear legacy credentials: "+err.Error(), "system")
			}
		}
		return
	}
	cfg := s.Config.Get()
	changed := false
	if defAcc.Zone != "" && cfg.Zone != defAcc.Zone {
		cfg.Zone = defAcc.Zone
		changed = true
	}
	effectiveIAM := defAcc.IAM
	if effectiveIAM == "" && defAcc.Zone != "" {
		effectiveIAM = "go-ovh-" + strings.ToLower(defAcc.Zone)
	}
	if effectiveIAM != "" && cfg.IAM != effectiveIAM {
		cfg.IAM = effectiveIAM
		changed = true
	}
	if defAcc.Endpoint != "" && cfg.Endpoint != defAcc.Endpoint {
		cfg.Endpoint = defAcc.Endpoint
		changed = true
	}
	// 无条件同步新默认账户的三项凭据（BUG-BE-04），防止切换默认账户后残留旧账户密钥与新区域跨区串配
	if cfg.AppKey != defAcc.AppKey || cfg.AppSecret != defAcc.AppSecret || cfg.ConsumerKey != defAcc.ConsumerKey {
		cfg.AppKey = defAcc.AppKey
		cfg.AppSecret = defAcc.AppSecret
		cfg.ConsumerKey = defAcc.ConsumerKey
		changed = true
	}
	if changed {
		_ = s.Config.Set(cfg)
		if s.Logger != nil {
			s.Logger.Info(fmt.Sprintf("全局运行时设置已同步默认账户 %s: Zone=%s, IAM=%s, Endpoint=%s", defAcc.Name, cfg.Zone, cfg.IAM, cfg.Endpoint), "system")
		}
	}
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
	s.SyncDefaultAccountConfig()
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
		s.SyncDefaultAccountConfig()
		s.Logger.Info("已加载 OVH 账户: "+intStr(len(accs))+" 个", "system")
	} else {
		s.MarkLoadFailed("ovh_accounts", err)
	}

	// queue
	if items, err := s.DB.ListQueue(); err == nil {
		s.Queue = items
	} else {
		s.MarkLoadFailed("queue", err)
	}
	if s.Queue == nil {
		s.Queue = []types.QueueItem{}
	}

	// history
	if items, err := s.DB.ListHistory(); err == nil {
		s.History = items
	} else {
		s.MarkLoadFailed("history", err)
	}
	if s.History == nil {
		s.History = []types.PurchaseHistoryEntry{}
	}
	if err := s.RestoreCompletedQueue(); err != nil {
		s.Logger.Error("恢复已下单任务状态失败: "+err.Error(), "queue")
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
		s.MarkLoadFailed("servers", err)
	}
	if s.ServerPlans == nil {
		s.ServerPlans = []types.ServerPlan{}
	}

	// vps subscriptions
	if subs, err := s.DB.ListVPSSubscriptions(); err == nil {
		s.VPSSubscriptions = subs
	} else {
		s.MarkLoadFailed("vps_subscriptions", err)
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
			// 用白名单而不是 `!= unavailable`:comingSoon(即将上线尚未开卖)下不了单,
			// 算进"可用服务器"会让仪表盘数字虚高
			if ovh.IsAvailableForOrder(dc.Availability) {
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

// HTTPClientFor 按账户的出站配置建一个普通 HTTP 客户端。
//
// 给那些**不带凭据**但仍然打 OVH 的请求用：公开目录、可用性探测等等。
// 它们以前一律走直连 —— 于是即便每个账户都配了代理，这些请求仍然从本机
// 真实 IP 发出去，出口隔离漏了一半。
//
// accountID 为空 → 默认账户的配置。账户不存在 → 直连（这些是公开接口，
// 没有账户也该能查，不该因为找不到账户就整个功能失效）。
//
// 和带凭据那条路一样：配了代理却建不出来时返回错误，**不退回直连**。
func (s *State) HTTPClientFor(accountID string, timeout time.Duration) (*http.Client, error) {
	acc, ok := s.FindAccount(accountID)
	if !ok {
		return &http.Client{Timeout: timeout}, nil
	}
	prof, _ := netfp.LookupProfile(acc.Fingerprint)
	return netfp.Client(netfp.Options{
		ProxyURL: acc.ProxyURL,
		Profile:  prof,
		Timeout:  timeout,
		OnProxyError: func(e error) {
			if s.onProxyError != nil {
				s.onProxyError(acc.ID, e)
			}
		},
		OnProxySuccess: func() {
			if s.onProxySuccess != nil {
				s.onProxySuccess(acc.ID)
			}
		},
	})
}

// SetProxyErrorHook 注入代理故障回调，供 HTTPClientFor 建出来的客户端使用。
func (s *State) SetProxyErrorHook(fn func(accountID string, err error)) {
	s.onProxyError = fn
}

// SetProxySuccessHook 注入代理恢复回调，供 HTTPClientFor 建出来的客户端使用。
func (s *State) SetProxySuccessHook(fn func(accountID string)) {
	s.onProxySuccess = fn
}
