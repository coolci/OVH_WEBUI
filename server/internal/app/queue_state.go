package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/orderterm"
	"github.com/ovh-webui/server/internal/types"
)

// Save* 系列都是"拍内存快照 → 全表 DELETE+INSERT"。
//
// 快照和写库必须在同一把锁里,否则并发调用会丢数据:
// A 拍快照(60 条)→ B 拍快照(59 条)→ B 先写库 → A 后写库,
// 库里最后是 A 那份也就罢了 —— 真实情况是 goroutine 调度随机,
// 晚拍的快照经常先落库,新数据被旧快照整表覆盖。
//
// 实测(internal/app 的并发测试):一边追加 60 条历史一边并发触发保存,
// 库里最后只剩 19~35 条,丢一半以上。而代码里有八处 `go state.SaveHistory()`
// 是 fire-and-forget 调用的,批量抢购时会同时触发 —— 抢到的订单记录
// 就这么没了,重启后历史里查无此单。
//
// 每类数据一把独立的保存锁:历史和队列互不阻塞。
// 注意这把锁必须包住整个"快照+写库",而不只是写库。
func (s *State) SaveQueue() error {
	if err := s.SaveBlocked("queue"); err != nil {
		return err
	}
	s.saveQueueMu.Lock()
	defer s.saveQueueMu.Unlock()

	var deleted map[string]struct{}
	if s.DB != nil {
		deleted, _ = s.DB.DeletedAccountIDSet()
	}

	var goneTaskIDs []string
	s.QueueMu.Lock()
	if len(deleted) > 0 {
		kept := make([]types.QueueItem, 0, len(s.Queue))
		for _, it := range s.Queue {
			if it.AccountID != "" {
				if _, isDel := deleted[it.AccountID]; isDel {
					goneTaskIDs = append(goneTaskIDs, it.ID)
					continue
				}
			}
			kept = append(kept, it)
		}
		s.Queue = kept
	}
	cp := make([]types.QueueItem, len(s.Queue))
	copy(cp, s.Queue)
	s.QueueMu.Unlock()

	for _, id := range goneTaskIDs {
		s.MarkTaskDeleted(id)
	}
	if s.DB == nil {
		return nil
	}
	return s.DB.ReplaceQueue(cp)
}

// MarkLoadFailed 记下某张表启动时没读出来,之后禁止覆盖写它。
func (s *State) MarkLoadFailed(table string, err error) {
	s.loadFailedMu.Lock()
	if s.loadFailed == nil {
		s.loadFailed = make(map[string]string)
	}
	s.loadFailed[table] = err.Error()
	s.loadFailedMu.Unlock()
	s.Logger.Error(
		"load "+table+" 失败,已禁止本次运行覆盖写该表(避免用空内存抹掉磁盘上的数据): "+err.Error(),
		"system",
	)
}

// SaveBlocked 读失败的表返回非 nil,调用方据此放弃保存。
func (s *State) SaveBlocked(table string) error {
	s.loadFailedMu.RLock()
	reason, bad := s.loadFailed[table]
	s.loadFailedMu.RUnlock()
	if !bad {
		return nil
	}
	return fmt.Errorf("拒绝写 %s:启动时这张表就没读出来(%s),再写会用空内存覆盖掉磁盘上的数据。请修复后重启", table, reason)
}

// LoadFailures 返回启动时读失败的表,给健康检查/前端横幅用。
func (s *State) LoadFailures() map[string]string {
	s.loadFailedMu.RLock()
	defer s.loadFailedMu.RUnlock()
	out := make(map[string]string, len(s.loadFailed))
	for k, v := range s.loadFailed {
		out[k] = v
	}
	return out
}

// EnqueueItems 先落库再发布到运行队列,失败时不暴露可执行的任务。
//
// 四条入队路径(网页新建 / 快速下单 / TG 一键按钮 / TG 文本下单)以前各写各的,
// 对"落库失败"的处理有四种:撤回+归还按钮、返回 warning 但内存保留、
// 返回带警告的文本、以及 `_ = SaveQueue()` 直接吞掉。
// 最后那种在自动下单路径上 —— 监控发现有货、建了任务、落库失败无人知晓,
// 重启后任务没了,而用户以为一直在抢。
//
// 统一成一种语义:**要么内存和磁盘都有,要么两边都没有**。
// 半成功状态("这次能跑但重启就丢")对抢购来说是最坏的,它看起来完全正常。
//
// prepend=true 把这批放到队首(快速下单要抢在别的任务前面)。
func (s *State) EnqueueItems(items []types.QueueItem, prepend bool) error {
	return s.enqueueItems(items, prepend, false)
}

// EnqueueAutomaticItems 不允许监控绕过已暂停或取消的同型号/账户/机房。
func (s *State) EnqueueAutomaticItems(items []types.QueueItem, prepend bool) error {
	return s.enqueueItems(items, prepend, true)
}

// enqueueItems 在同一队列锁内校验自动建单许可并持久化任务。
func (s *State) enqueueItems(items []types.QueueItem, prepend, automatic bool) error {
	if len(items) == 0 {
		return nil
	}
	// 购买时长在入队这一个共同入口统一规整:空值落成 P1M,非法值整批拒绝,
	// 不让各个入口(网页/TG/监控)各自猜默认值。
	for i := range items {
		duration, err := orderterm.Normalize(items[i].Duration)
		if err != nil {
			return &RejectedError{Reason: err.Error()}
		}
		items[i].Duration = duration
	}
	if err := s.SaveBlocked("queue"); err != nil {
		return err
	}
	s.saveQueueMu.Lock()
	defer s.saveQueueMu.Unlock()
	s.QueueMu.Lock()
	defer s.QueueMu.Unlock()
	if automatic {
		for _, item := range items {
			if err := s.checkAutomaticOrderLocked(item); err != nil {
				return err
			}
		}
	}
	// 队列总量闸门。放在这里是因为四条入队路径(网页新建 / 快速下单 / TG 文本下单 /
	// 监控自动下单)全都汇到这个函数 —— 加一次就都受保护。
	//
	// 要防的是"多打一个数字"这种事:网页端建任务的循环没有上界,
	// 数量填 9999 × 5 个机房 = 近 5 万条任务,而每一条都是一次真实下单尝试。
	// 正常用法离 MaxQueueItems 很远,撞上它基本可以断定是填错了。
	if len(s.Queue)+len(items) > types.MaxQueueItems {
		have, want := len(s.Queue), len(items)
		return Rejected(
			"队列里已有 %d 条任务,这次还要加 %d 条,会超过上限 %d。"+
				"每条任务都是一次真实的下单尝试 —— 请先确认数量没填错,"+
				"或到队列页清理掉不需要的任务",
			have, want, types.MaxQueueItems)
	}
	next := make([]types.QueueItem, 0, len(s.Queue)+len(items))
	if prepend {
		next = append(append(next, items...), s.Queue...)
	} else {
		next = append(append(next, s.Queue...), items...)
	}
	// 与 SaveQueue 相同的锁顺序;提交前不让处理器读取待入队任务。
	if s.DB != nil {
		if err := s.DB.ReplaceQueue(next); err != nil {
			return err
		}
	}
	s.Queue = next
	if !automatic {
		for _, item := range items {
			if err := s.SetAutomaticOrderBlockedLocked(item, false); err != nil {
				s.Logger.Warn("任务已创建,自动建单仍保持停止: "+err.Error(), "queue")
			}
		}
	}
	s.WakeQueueProcessor()
	return nil
}

// QueueWakeCh 返回队列唤醒事件通道
func (s *State) QueueWakeCh() <-chan struct{} {
	if s == nil {
		return nil
	}
	return s.queueWakeCh
}

// WakeQueueProcessor 触发队列处理器立即唤醒执行，避免轮询等待
func (s *State) WakeQueueProcessor() {
	if s == nil || s.queueWakeCh == nil {
		return
	}
	select {
	case s.queueWakeCh <- struct{}{}:
	default:
	}
}

var ErrQueueNotFound = errors.New("任务不存在")
var ErrQueueNotEditable = errors.New("已下单的任务不能修改；如需再买，请使用「再抢一单」新建任务")

// UpdateQueueItem 编辑任务配置，落库成功后才发布新配置。
//
// 运行中 / 待执行的任务可直接修改，无需先暂停：落库后取消旧配置正在进行的那一轮，
// 保持原运行状态并立即唤醒队列处理器，下一轮立刻按新配置执行；
// 暂停 / 失败的任务修改后保持暂停；已下单（completed）的任务禁止修改。
// duration 为 nil 表示不改购买时长（兼容旧客户端）。
func (s *State) UpdateQueueItem(id, datacenter string, options []string, retryInterval int, autoPay bool, coupon string, duration *string) error {
	var selectedDuration string
	if duration != nil {
		d, err := orderterm.Normalize(*duration)
		if err != nil {
			return err
		}
		selectedDuration = d
	}
	if err := s.SaveBlocked("queue"); err != nil {
		return err
	}
	s.saveQueueMu.Lock()
	defer s.saveQueueMu.Unlock()
	s.QueueMu.Lock()
	defer s.QueueMu.Unlock()
	next := append([]types.QueueItem(nil), s.Queue...)
	for i := range next {
		if next[i].ID != id {
			continue
		}
		if next[i].Status == "completed" {
			return ErrQueueNotEditable
		}
		active := next[i].Status == "running" || next[i].Status == "pending"
		next[i].Datacenter = datacenter
		next[i].Options = append([]string(nil), options...)
		next[i].RetryInterval = retryInterval
		next[i].AutoPay = autoPay
		next[i].Coupon = coupon
		if duration != nil {
			next[i].Duration = selectedDuration
		}
		if !active {
			next[i].Status = "paused"
		} else {
			next[i].StopReason = ""
		}
		next[i].FailureCount = 0
		next[i].LastCheckTime = 0
		next[i].UpdatedAt = types.NowISO()
		if s.DB != nil {
			if err := s.DB.ReplaceQueue(next); err != nil {
				return err
			}
		}
		s.Queue = next
		if active {
			if err := s.SetAutomaticOrderBlockedLocked(next[i], false); err != nil && s.Logger != nil {
				s.Logger.Warn("任务已按新配置继续，自动建单停止记录未能更新: "+err.Error(), "queue")
			}
			s.UnregisterTaskCancel(id)
			s.WakeQueueProcessor()
		}
		return nil
	}
	return ErrQueueNotFound
}

// CancelTask 取消一个任务当前正在进行的下单执行（如暂停任务时使用，不标记删除）。
func (s *State) CancelTask(id string) {
	s.taskCancelMu.Lock()
	cancel := s.taskCancel[id]
	delete(s.taskCancel, id)
	s.taskCancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// IsTaskActiveInQueue 检查任务是否仍在队列中且未被标记删除或暂停。
func (s *State) IsTaskActiveInQueue(id string) bool {
	if id == "" {
		return true
	}
	if s.IsTaskDeleted(id) {
		return false
	}
	s.QueueMu.Lock()
	defer s.QueueMu.Unlock()
	for i := range s.Queue {
		if s.Queue[i].ID == id {
			return s.Queue[i].Status == "running" || s.Queue[i].Status == "pending"
		}
	}
	return false
}

// IsTaskPaused 检查任务是否在队列中被置为 paused
func (s *State) IsTaskPaused(id string) bool {
	if id == "" {
		return false
	}
	s.QueueMu.Lock()
	defer s.QueueMu.Unlock()
	for i := range s.Queue {
		if s.Queue[i].ID == id {
			return s.Queue[i].Status == "paused"
		}
	}
	return false
}

// MarkTaskDeleted 标记删除并中断本轮下单,所有删除入口统一调用。
func (s *State) MarkTaskDeleted(id string) {
	s.DeletedTaskIDsMu.Lock()
	s.DeletedTaskIDs[id] = struct{}{}
	s.DeletedTaskIDsMu.Unlock()

	s.taskCancelMu.Lock()
	cancel := s.taskCancel[id]
	delete(s.taskCancel, id)
	s.taskCancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// IsTaskDeleted 任务是否已被标记删除。
func (s *State) IsTaskDeleted(id string) bool {
	s.DeletedTaskIDsMu.Lock()
	defer s.DeletedTaskIDsMu.Unlock()
	_, ok := s.DeletedTaskIDs[id]
	return ok
}

// RegisterTaskCancel 登记一个任务这一轮下单的取消函数。PurchaseServer 开跑前调。
//
// 调用方登记完必须再查一次 IsTaskDeleted:登记前一瞬间刚好被删的话,
// MarkTaskDeleted 那时还找不到 cancel 函数,ctx 不会被取消 —— 那次复核把这个窗口堵上。
func (s *State) RegisterTaskCancel(id string, cancel context.CancelFunc) {
	s.taskCancelMu.Lock()
	if s.taskCancel == nil {
		s.taskCancel = make(map[string]context.CancelFunc)
	}
	s.taskCancel[id] = cancel
	s.taskCancelMu.Unlock()
}

// UnregisterTaskCancel 注销并取消本轮下单。轮末释放或用户暂停时调用。
// 已被 MarkTaskDeleted 摘掉的话这里是空操作。
func (s *State) UnregisterTaskCancel(id string) {
	s.taskCancelMu.Lock()
	cancel := s.taskCancel[id]
	delete(s.taskCancel, id)
	s.taskCancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// SaveHistory 把内存中 History 整表覆盖写入 SQLite（持 saveHistoryMu 获取最新快照并剔除已删账户残留）
func (s *State) SaveHistory() error {
	if err := s.SaveBlocked("history"); err != nil {
		return err
	}
	s.saveHistoryMu.Lock()
	defer s.saveHistoryMu.Unlock()

	var deleted map[string]struct{}
	if s.DB != nil {
		deleted, _ = s.DB.DeletedAccountIDSet()
	}

	s.HistoryMu.Lock()
	if len(deleted) > 0 {
		kept := make([]types.PurchaseHistoryEntry, 0, len(s.History))
		for _, h := range s.History {
			if h.AccountID != "" {
				if _, isDel := deleted[h.AccountID]; isDel {
					continue
				}
			}
			kept = append(kept, h)
		}
		s.History = kept
	}
	cp := make([]types.PurchaseHistoryEntry, len(s.History))
	copy(cp, s.History)
	s.HistoryMu.Unlock()

	if s.DB == nil {
		return nil
	}
	return s.DB.ReplaceHistory(cp)
}

// SaveServers 把内存中 ServerPlans 整表覆盖写入 SQLite
func (s *State) SaveServers() error {
	if err := s.SaveBlocked("servers"); err != nil {
		return err
	}
	s.saveServersMu.Lock()
	defer s.saveServersMu.Unlock()
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
	var complete bool
	if _, err := s.DB.GetKV(db.LegacyAccountMigrationKey, &complete); err != nil {
		s.Logger.Error("read legacy migration marker: "+err.Error(), "system")
		return
	}
	if complete {
		return
	}
	n, err := s.DB.CountAccounts()
	if err != nil {
		s.Logger.Error("count accounts: "+err.Error(), "system")
		return
	}
	if n > 0 {
		if err := s.DB.SetKV(db.LegacyAccountMigrationKey, true); err != nil {
			s.Logger.Error("mark legacy account migration: "+err.Error(), "system")
		}
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
	if err := s.DB.SetKV(db.LegacyAccountMigrationKey, true); err != nil {
		s.Logger.Error("mark legacy account migration: "+err.Error(), "system")
	}
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
