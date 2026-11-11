package catalogwatch

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/types"
)

type Job struct {
	Round         uint64                    `json:"round"`
	Checkout      *purchase.CheckoutJournal `json:"checkout,omitempty"`
	CheckDue      int64                     `json:"checkDue,omitempty"`
	CheckAttempts int                       `json:"checkAttempts,omitempty"`
	Endpoint      string                    `json:"endpoint,omitempty"`

	PerDatacenter bool            `json:"perDatacenter,omitempty"`
	TaskID        string          `json:"taskId,omitempty"`
	FailedDCs     map[string]bool `json:"failedDCs,omitempty"`
	Candidate
	ID        string `json:"id"`
	AccountID string `json:"accountId"`
	Status    string `json:"status"`
	NextDC    int    `json:"nextDC"`
	Attempts  int    `json:"attempts"`
	Due       int64  `json:"due"`
	Reason    string `json:"reason"`
	OrderID   string `json:"orderId,omitempty"`
	CreatedAt string `json:"createdAt"`
}
type Data struct {
	Round       uint64               `json:"round"`
	Enabled     bool                 `json:"enabled"`
	AccountID   string               `json:"accountId"`
	AccountName string               `json:"accountName"`
	Endpoint    string               `json:"endpoint"`
	Subsidiary  string               `json:"subsidiary"`
	BaselineAt  string               `json:"baselineAt"`
	Baseline    map[string]Candidate `json:"baseline"`
	Jobs        map[string]*Job      `json:"jobs"`
	LastCheck   string               `json:"lastCheck"`
	Error       string               `json:"error"`
}
type Watcher struct {
	mu         sync.Mutex
	state      *app.State
	data       Data
	loadErr    error
	generation uint64
	active     map[string]context.CancelFunc
	once       sync.Once
	// loops Run 起的全部循环;关停时等它们(含在途下单)退出再关库
	loops sync.WaitGroup
	fetch func(context.Context, *app.State, string) (map[string]Candidate, error)
	check func(context.Context, *app.State, string, *purchase.CheckoutJournal) (purchase.CheckoutCheck, error)
	buy   func(context.Context, *app.State, *types.QueueItem) purchase.Outcome
}

func For(s *app.State) *Watcher {
	s.CatalogWatchOnce.Do(func() { s.CatalogWatch = New(s) })
	return s.CatalogWatch.(*Watcher)
}
func New(s *app.State) *Watcher {
	w := &Watcher{state: s, active: map[string]context.CancelFunc{}, fetch: fetch, buy: purchase.PurchaseServer, check: purchase.CheckCheckout, data: Data{Enabled: true, Baseline: map[string]Candidate{}, Jobs: map[string]*Job{}}}
	var raw string
	err := s.DB.Get(&raw, "SELECT data FROM catalog_watch WHERE id=1")
	if err == nil {
		err = json.Unmarshal([]byte(raw), &w.data)
	}
	if err != nil && err != sql.ErrNoRows {
		w.loadErr = err
		w.data.Enabled = false
		w.data.Error = "独立状态读取失败，禁止自动下单"
	}
	if w.data.Baseline == nil {
		w.data.Baseline = map[string]Candidate{}
	}
	if w.data.Jobs == nil {
		w.data.Jobs = map[string]*Job{}
	}
	// 结账凭据里记着的购物车要留给后续核对,遗留购物车清理不能删
	for _, j := range w.data.Jobs {
		if j.Checkout != nil {
			ovh.TrackCart(j.Checkout.CartID)
		}
	}
	for _, j := range w.data.Jobs {
		if j.Status == "attempting" {
			j.Status = "uncertain"
			j.Reason = "服务曾在尝试期间中断，后台自动核对订单"
		}
	}
	for _, j := range w.data.Jobs {
		if j.Status == "uncertain" && j.TaskID != "" {
			for _, h := range s.HistorySnapshot() {
				if h.TaskID == j.TaskID && h.Status == "success" && h.OrderID != "" {
					j.Status = "completed"
					j.OrderID = h.OrderID
					j.Reason = "已从购买历史确认建单成功"
				}
			}
		}
	}
	// Split old pending groups without replaying old successes or uncertain checkouts.
	migrated := false
	for id, j := range w.data.Jobs {
		if j.PerDatacenter {
			continue
		}
		delete(w.data.Jobs, id)
		dcs := unique(j.Datacenters)
		// 旧版未决结账只对应一辆车、一个机房(正在尝试的那个)。
		attemptedDC := ""
		if len(dcs) > 0 {
			attemptedDC = dcs[0]
			if j.NextDC >= 0 && j.NextDC < len(dcs) {
				attemptedDC = dcs[j.NextDC]
			}
		}
		for _, dc := range dcs {
			copyJob := *j
			// 不能共用旧 job 的 FailedDCs / Checkout:拆出来的每个机房 job 各自演进,
			// 共用同一个 Checkout 会让每个机房都去核对同一辆车、拿同一张订单标成已完成
			copyJob.FailedDCs = nil
			copyJob.Checkout = nil
			if j.Checkout != nil && dc == attemptedDC {
				cp := *j.Checkout
				copyJob.Checkout = &cp
			}
			if j.Status == "uncertain" && j.Checkout != nil && dc != attemptedDC {
				copyJob.Status = "cancelled"
				copyJob.Reason = "旧版未决结账只在原尝试机房核对，其他机房不在升级时自动回补"
			}
			copyJob.Datacenters = []string{dc}
			copyJob.PerDatacenter = true
			copyJob.ID = locationKey(j.AccountID, j.Candidate, dc)
			copyJob.NextDC = 0
			if j.Status == "completed" {
				copyJob.Status = "cancelled"
				copyJob.Reason = "旧版已建单配置不在升级时回补其他机房"
			}
			for _, h := range s.HistorySnapshot() {
				if h.TaskID == j.TaskID && h.Status == "success" && h.Datacenter == dc {
					copyJob.Status = "completed"
					copyJob.OrderID = h.OrderID
				}
			}
			if j.Status == "pending" && j.FailedDCs[dc] {
				copyJob.Status = "failed"
			}
			if _, exists := w.data.Jobs[copyJob.ID]; !exists {
				w.data.Jobs[copyJob.ID] = &copyJob
			}
		}
		migrated = true
	}
	if migrated {
		_ = w.save()
	}
	return w
}

const maxConcurrentAttempts = 100

// Bound pre-checkout work. Checkout already submitted is still allowed to settle
// under the SDK's HTTP timeout; never abandon it and start another purchase.
const attemptTimeout = 2 * time.Minute

func locationKey(account string, c Candidate, dc string) string {
	return key(account+"\x00"+dc, c)
}

func (w *Watcher) cancelAll() {
	for _, cancel := range w.active {
		cancel()
	}
}
func (w *Watcher) save() error {
	if w.loadErr != nil {
		return w.loadErr
	}
	raw, err := json.Marshal(w.data)
	if err == nil {
		_, err = w.state.DB.Exec("INSERT INTO catalog_watch(id,data) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data", string(raw))
	}
	if err != nil {
		w.loadErr = err
		w.data.Enabled = false
		w.data.Error = "独立状态保存失败，自动下单已停止"
		w.cancelAll()
	}
	return err
}
func (w *Watcher) SetEnabled(_ context.Context, enabled bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.loadErr != nil {
		return fmt.Errorf("独立状态存储不可用: %w", w.loadErr)
	}
	if enabled == w.data.Enabled {
		return nil
	}
	if enabled && len(w.active) > 0 {
		return fmt.Errorf("上一轮正在结束，请等待结果确认后再开启")
	}
	w.generation++
	w.data.Round++
	w.cancelAll()
	w.data.Enabled = enabled
	if enabled {
		w.data.AccountID = ""
		w.data.AccountName = ""
		w.data.Endpoint = ""
		w.data.Subsidiary = ""
		w.data.LastCheck = ""
		w.data.BaselineAt = ""
		w.data.Baseline = map[string]Candidate{}
		w.data.Error = ""
	} else {
		for _, j := range w.data.Jobs {
			if j.Status == "pending" {
				j.Status = "cancelled"
				j.Reason = "用户关闭自动下单"
			}
		}
	}
	return w.save()
}
func (w *Watcher) Snapshot() interface{} {
	w.mu.Lock()
	defer w.mu.Unlock()
	jobs := []Job{}
	counts := map[string]int{}
	for _, j := range w.data.Jobs {
		if j.Round == w.data.Round {
			counts[j.Status]++
		}
		jobs = append(jobs, *j)
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt > jobs[j].CreatedAt })
	if len(jobs) > 30 {
		jobs = jobs[:30]
	}
	return map[string]interface{}{"enabled": w.data.Enabled, "accountId": w.data.AccountID, "accountName": w.data.AccountName, "subsidiary": w.data.Subsidiary, "baselineAt": w.data.BaselineAt, "baselineCount": len(w.data.Baseline), "lastCheck": w.data.LastCheck, "error": w.data.Error, "active": len(w.active) > 0, "counts": counts, "round": w.data.Round, "automaticReconciliation": true, "jobs": jobs, "autoPay": false, "intervalSeconds": 2, "orderScope": "plan-options-datacenter", "maxConcurrentAttempts": maxConcurrentAttempts}
}
func (w *Watcher) accountOK() bool {
	a, ok := w.state.FindAccount(w.data.AccountID)
	return ok && a.ID == w.data.AccountID && a.Endpoint == w.data.Endpoint && catalog.SubsidiaryOfAccount(a) == w.data.Subsidiary
}

// Poll is separate from order execution and never touches ServerCache or CatalogService.
func (w *Watcher) Poll(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	w.mu.Lock()
	if !w.data.Enabled || w.loadErr != nil {
		w.mu.Unlock()
		return nil
	}
	if w.data.AccountID == "" {
		a, ok := w.state.FindAccount("")
		if !ok {
			w.data.Error = "等待配置默认账户"
			w.mu.Unlock()
			return fmt.Errorf("没有默认账户")
		}
		sub := catalog.SubsidiaryOfAccount(a)
		if !ovh.KnownSubsidiary(sub) || ovh.SubsidiaryRegion(sub) != ovh.EndpointRegion(a.Endpoint) {
			w.data.Error = "默认账户销售地区配置不匹配"
			w.mu.Unlock()
			return fmt.Errorf("账户地区不匹配")
		}
		w.data.AccountID = a.ID
		w.data.AccountName = a.Name
		w.data.Endpoint = a.Endpoint
		w.data.Subsidiary = sub
		if err := w.save(); err != nil {
			w.mu.Unlock()
			return err
		}
	}
	if !w.accountOK() {
		w.cancelAll()
		w.data.Error = "绑定账户已删除或销售地区已变更；请关闭后重新开启"
		w.mu.Unlock()
		return fmt.Errorf("绑定账户已变更")
	}
	gen, sub := w.generation, w.data.Subsidiary
	w.mu.Unlock()
	current, err := w.fetch(ctx, w.state, sub)
	w.mu.Lock()
	defer w.mu.Unlock()
	if gen != w.generation || !w.data.Enabled {
		return nil
	}
	if err != nil {
		w.data.Error = err.Error()
		return err
	}
	if len(current) == 0 {
		w.data.Error = "目录为空，保留原基线"
		return fmt.Errorf("目录为空")
	}
	// First successful snapshot is the baseline, not an ordering event.
	if w.data.BaselineAt == "" {
		w.data.Baseline = current
		w.data.BaselineAt = time.Now().UTC().Format(time.RFC3339)
		w.data.LastCheck = w.data.BaselineAt
		w.data.Error = ""
		return w.save()
	}
	dirty := false
	for k, c := range current {
		old, seen := w.data.Baseline[k]
		locations := unique(append(append([]string{}, old.Datacenters...), c.Datacenters...))
		if seen && sameLocations(old.Datacenters, locations) {
			continue
		}
		dirty = true
		for _, dc := range unique(c.Datacenters) {
			wasSeen := false
			for _, oldDC := range old.Datacenters {
				if oldDC == dc {
					wasSeen = true
				}
			}
			if seen && wasSeen {
				continue
			}
			id := locationKey(w.data.AccountID, c, dc)
			if _, exists := w.data.Jobs[id]; !exists {
				one := c
				one.Datacenters = []string{dc}
				w.data.Jobs[id] = &Job{Round: w.data.Round, Endpoint: w.data.Endpoint, PerDatacenter: true, Candidate: one, ID: id, AccountID: w.data.AccountID, Status: "pending", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
			}
		}
		// Remember locations too: removal/reappearance is not a new ordering event.
		c.Datacenters = locations
		w.data.Baseline[k] = c
	}
	w.data.LastCheck = time.Now().UTC().Format(time.RFC3339)
	w.data.Error = ""
	if dirty {
		return w.save()
	}
	return nil
}
func sameLocations(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (w *Watcher) Attempt(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	w.mu.Lock()
	if !w.data.Enabled || w.data.BaselineAt == "" || w.loadErr != nil || len(w.active) >= maxConcurrentAttempts || !w.accountOK() {
		w.mu.Unlock()
		return
	}
	// Claim exactly one plan/options/location under the lock before releasing it.
	var job *Job
	for _, j := range w.data.Jobs {
		if j.Status == "pending" && j.Due <= time.Now().UnixMilli() && (job == nil || j.Due < job.Due || j.Due == job.Due && j.ID < job.ID) {
			job = j
		}
	}
	if job == nil {
		w.mu.Unlock()
		return
	}
	if len(job.Datacenters) != 1 {
		job.Status = "failed"
		job.Reason = "任务必须绑定唯一目标机房"
		_ = w.save()
		w.mu.Unlock()
		return
	}
	dc := job.Datacenters[0]
	item := types.QueueItem{ID: "catalog-watch-" + job.ID + "-" + dc, AccountID: job.AccountID, PlanCode: job.PlanCode, Datacenter: dc, Options: append([]string{}, job.Options...), AutoPay: false, Force: true, Status: "running"}
	job.TaskID = item.ID
	job.Status = "attempting"
	job.Attempts++
	job.Checkout = &purchase.CheckoutJournal{}
	attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
	w.active[job.ID] = cancel
	if err := w.save(); err != nil {
		delete(w.active, job.ID)
		cancel()
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()
	journalCtx := purchase.WithCheckoutJournal(attemptCtx, func(j purchase.CheckoutJournal) error {
		w.mu.Lock()
		defer w.mu.Unlock()
		if j.OrderID == "" && (!w.data.Enabled || attemptCtx.Err() != nil) {
			return fmt.Errorf("尝试已取消")
		}
		job.Checkout = &j
		ovh.TrackCart(j.CartID)
		return w.save()
	})
	outcome := w.buy(journalCtx, w.state, &item)
	cancel()
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.active, job.ID)
	switch {
	case outcome.Success:
		job.Status = "completed"
		job.Reason = "本机型配置在本机房已建单 1 台，仅停止此组合（自动付款关闭）"
	case outcome.PauseTask:
		job.Status = "uncertain"
		job.Reason = outcome.Reason
		if job.Reason == "" {
			job.Reason = "结账结果不确定，后台自动核对订单"
		}
	case outcome.Cancelled && w.data.Enabled:
		job.Status = "pending"
		job.Reason = "服务正在停止，重启后恢复尚未提交的尝试"
		if errors.Is(attemptCtx.Err(), context.DeadlineExceeded) {
			job.Reason = "本轮下单前请求超时，释放并发名额后重试；其他组合独立执行"
		}
		job.Due = time.Now().Add(5 * time.Second).UnixMilli()
	case outcome.Cancelled || !w.data.Enabled:
		job.Status = "cancelled"
		job.Reason = "自动下单已停止"
	default:
		job.Status = "pending"
		if outcome.Fatal {
			job.Status = "failed"
		}
		job.Due = time.Now().Add(5 * time.Second).UnixMilli()
		job.Reason = outcome.Reason
		if job.Reason == "" {
			job.Reason = "本机房未成功，稍后重试；其他机房独立执行"
		}
	}
	for _, h := range w.state.HistorySnapshot() {
		if h.TaskID == item.ID && h.Status == "success" {
			job.Status = "completed"
			job.OrderID = h.OrderID
		}
	}
	_ = w.save()
}
func (w *Watcher) Run(ctx context.Context) {
	w.once.Do(func() {
		w.loops.Add(2 + maxConcurrentAttempts)
		go func() {
			defer w.loops.Done()
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					w.Reconcile(ctx)
				}
			}
		}()
		go func() {
			defer w.loops.Done()
			delay := time.Duration(0)
			for {
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
				if err := w.Poll(ctx); err != nil {
					if delay < 5*time.Second {
						delay = 5 * time.Second
					} else {
						delay *= 2
					}
					if delay > 60*time.Second {
						delay = 60 * time.Second
					}
				} else {
					delay = 2 * time.Second
				}
			}
		}()
		for worker := 0; worker < maxConcurrentAttempts; worker++ {
			go func() {
				defer w.loops.Done()
				ticker := time.NewTicker(time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						w.Attempt(ctx)
					}
				}
			}()
		}
	})
}

// Wait 等 Run 起的循环全部退出(最多 timeout)。ctx 取消后调用;返回 false = 超时。
// 在途的下单尝试里结账请求不接受取消,必须等它返回、结果写进库后才能关数据库。
func (w *Watcher) Wait(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		w.loops.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}
