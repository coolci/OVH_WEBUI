package catalogwatch

import (
	"context"
	"fmt"
	"time"

	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/types"
)

const (
	maxReconcileAttempts = 60
	maxReconcileWindow   = 24 * time.Hour
)

// Reconcile keeps running while ordering is switched off. It never submits a
// checkout, and it cannot restore buying permission from an earlier round.
func (w *Watcher) Reconcile(ctx context.Context) {
	w.mu.Lock()
	if ctx.Err() != nil || w.loadErr != nil || len(w.active) >= maxConcurrentAttempts {
		w.mu.Unlock()
		return
	}
	var job *Job
	for _, j := range w.data.Jobs {
		if j.Status == "uncertain" && w.active[j.ID] == nil && j.CheckDue <= time.Now().UnixMilli() && (job == nil || j.CheckDue < job.CheckDue) {
			job = j
		}
	}
	if job == nil {
		w.mu.Unlock()
		return
	}
	for _, h := range w.state.HistorySnapshot() {
		if h.TaskID == job.TaskID && job.TaskID != "" && h.Status == "success" && h.OrderID != "" {
			job.Status, job.OrderID, job.Reason = "completed", h.OrderID, "已从成功历史确认建单，不再重试"
			if job.Checkout != nil {
				ovh.ReleaseCart(job.Checkout.CartID)
			}
			_ = w.save()
			w.mu.Unlock()
			return
		}
	}
	checkCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	w.active[job.ID] = cancel
	job.CheckAttempts++
	job.CheckDue = time.Now().Add(time.Minute).UnixMilli()
	account, exists := w.state.FindAccount(job.AccountID)
	accountOK := exists && (job.Endpoint == "" || job.Endpoint == account.Endpoint)

	var journalArg *purchase.CheckoutJournal
	if job.Checkout != nil {
		cp := *job.Checkout
		if cp.PlanCode == "" {
			cp.PlanCode = job.PlanCode
		}
		if cp.Datacenter == "" && len(job.Datacenters) == 1 {
			cp.Datacenter = job.Datacenters[0]
		}
		journalArg = &cp
	}
	w.mu.Unlock()

	var result purchase.CheckoutCheck
	var err error
	if accountOK {
		result, err = w.check(checkCtx, w.state, job.AccountID, journalArg)
	} else {
		err = fmt.Errorf("原账户已删除或接口地区变更")
	}
	// Persist success/history before updating the job. On restart the history
	// check above prevents a second notification or a second checkout.
	if err == nil && result.OrderID != "" {
		item := types.QueueItem{ID: job.TaskID, AccountID: job.AccountID, PlanCode: job.PlanCode, Options: append([]string{}, job.Options...), AutoPay: false}
		if len(job.Datacenters) == 1 {
			item.Datacenter = job.Datacenters[0]
		}
		purchase.RecordCheckedOrder(w.state, &item, result)
	}
	cancel()
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.active, job.ID)

	exceededMaxWindow := false
	if journalArg != nil && journalArg.StartedAt != "" {
		if t, errParse := time.Parse(time.RFC3339Nano, journalArg.StartedAt); errParse == nil && time.Since(t) >= maxReconcileWindow && job.CheckAttempts >= 5 {
			exceededMaxWindow = true
		}
	}

	switch {
	case err == nil && result.OrderID != "":
		job.Status, job.OrderID, job.Reason = "completed", result.OrderID, "后台查单确认成功，仅停止此组合（自动付款关闭）"
	case err == nil && result.NotSubmitted:
		job.Status, job.Reason = "cancelled", "确认未提交订单；原轮次已关闭"
		if w.data.Enabled && job.Round == w.data.Round && accountOK && job.AccountID == w.data.AccountID {
			job.Status, job.Reason = "pending", "确认未提交结账，自动恢复尝试"
			job.Due = time.Now().Add(5 * time.Second).UnixMilli()
		}
	case err == nil && result.CartExpired:
		job.Status, job.Reason = "failed", "原购物车已过期或已超核对窗口且官方订单列表无成交记录，终止自动核对（不重复建单）"
	case !accountOK:
		job.Status, job.Reason = "failed", "原账户已删除或接口地区变更，终止后台核对（不重复建单）"
	case job.CheckAttempts >= maxReconcileAttempts || exceededMaxWindow:
		job.Status = "failed"
		job.Reason = "超过最大后台核对时长/次数上限仍未查到订单，已安全终止核对（不重复建单）"
		if err != nil {
			job.Reason += "：" + err.Error()
		}
	default:
		job.Reason = "正在自动核对；尚不能确认结果，不重复建单"
		if err != nil {
			job.Reason += "：" + err.Error()
		}
	}

	if job.Status != "uncertain" && job.Checkout != nil {
		ovh.ReleaseCart(job.Checkout.CartID)
	}
	if job.Status == "uncertain" {
		shift := job.CheckAttempts - 1
		if shift < 0 {
			shift = 0
		}
		if shift > 6 {
			shift = 6
		}
		backoff := time.Duration(15*(1<<shift)) * time.Second
		if backoff > 15*time.Minute {
			backoff = 15 * time.Minute
		}
		job.CheckDue = time.Now().Add(backoff).UnixMilli()
	}
	_ = w.save()
}
