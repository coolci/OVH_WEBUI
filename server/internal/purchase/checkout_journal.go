package purchase

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// Persisted before checkout: a process restart must never erase an unknown POST.
type CheckoutJournal struct {
	OrderID    string `json:"orderId,omitempty"`
	CartID     string `json:"cartId"`
	PlanCode   string `json:"planCode,omitempty"`
	Datacenter string `json:"datacenter,omitempty"`
	Submitted  bool   `json:"submitted"`
	StartedAt  string `json:"startedAt"`
}

type journalKey struct{}
type JournalWriter func(CheckoutJournal) error

func WithCheckoutJournal(ctx context.Context, write JournalWriter) context.Context {
	return context.WithValue(ctx, journalKey{}, write)
}

func saveCheckoutJournal(ctx context.Context, cartID string, orderIDs ...string) error {
	return saveCheckoutJournalForItem(ctx, nil, cartID, orderIDs...)
}

func saveCheckoutJournalForItem(ctx context.Context, item *types.QueueItem, cartID string, orderIDs ...string) error {
	if write, ok := ctx.Value(journalKey{}).(JournalWriter); ok {
		j := CheckoutJournal{CartID: cartID, Submitted: true, StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		if item != nil {
			j.PlanCode = item.PlanCode
			j.Datacenter = item.Datacenter
		}
		if len(orderIDs) > 0 {
			j.OrderID = orderIDs[0]
		}
		return write(j)
	}
	return nil
}

type CheckoutCheck struct {
	OrderID      string
	OrderURL     string
	NotSubmitted bool
	CartExpired  bool
}

const (
	expiredCartReconcileGrace   = 10 * time.Minute
	lockedCartMissingOrderLimit = 30 * time.Minute
	unvalidatedCartMaxLifetime  = 2 * time.Hour
)

// CheckCheckout reconciles an uncertain checkout without ever calling the simulation
// endpoint GET /order/cart/{cartId}/checkout (which always returns orderId:null or 400
// on locked carts). Instead, locked or expired carts are reconciled against /me/order
// using journal.StartedAt and normalized datacenter matching.
func CheckCheckout(ctx context.Context, state *app.State, accountID string, journal *CheckoutJournal) (CheckoutCheck, error) {
	if journal == nil {
		return CheckoutCheck{}, fmt.Errorf("旧任务缺少购物车凭据，不能推断未建单")
	}
	if !journal.Submitted {
		return CheckoutCheck{NotSubmitted: true}, nil
	}
	if journal.CartID == "" {
		return CheckoutCheck{}, fmt.Errorf("缺少购物车编号")
	}
	client, err := state.OVH.IsolatedClientFor(accountID)
	if err != nil {
		return CheckoutCheck{}, err
	}
	client.Timeout = 20 * time.Second
	if journal.OrderID != "" {
		var order struct {
			OrderID int64  `json:"orderId"`
			URL     string `json:"url"`
		}
		if err = client.GetWithContext(ctx, "/me/order/"+url.PathEscape(journal.OrderID), &order); err != nil {
			return CheckoutCheck{}, err
		}
		if fmt.Sprint(order.OrderID) != journal.OrderID || order.OrderID <= 0 {
			return CheckoutCheck{}, fmt.Errorf("订单编号未确认")
		}
		return CheckoutCheck{OrderID: journal.OrderID, OrderURL: order.URL}, nil
	}

	acc, _ := state.FindAccount(accountID)
	accountKey := acc.ID
	if accountKey == "" {
		accountKey = accountID
	}
	planCode := strings.TrimSpace(journal.PlanCode)
	since, hasStartedAt := parseJournalStart(journal.StartedAt)
	if !hasStartedAt {
		since = time.Now().Add(-30 * time.Minute)
	}

	reconcileOrders := func() (string, string, error) {
		unlock := lockReconcile(accountKey, planCode)
		defer unlock()
		return ReconcileRecentOrderEx(client, acc.Endpoint, planCode, since, ReconcileOptions{
			Datacenter:       journal.Datacenter,
			ExcludedOrderIDs: claimedOrderIDs(state),
		})
	}

	path := "/order/cart/" + url.PathEscape(journal.CartID)
	var cart struct {
		ReadOnly *bool `json:"readOnly"`
	}
	if err = client.GetWithContext(ctx, path, &cart); err != nil {
		var apiErr *ovhsdk.APIError
		is404 := errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound
		if is404 && planCode != "" {
			oid, link, recErr := reconcileOrders()
			if recErr != nil {
				return CheckoutCheck{}, recErr
			}
			if oid != "" {
				return CheckoutCheck{OrderID: oid, OrderURL: link}, nil
			}
			if hasStartedAt && time.Since(since) >= expiredCartReconcileGrace {
				return CheckoutCheck{CartExpired: true}, nil
			}
		}
		return CheckoutCheck{}, err
	}

	if cart.ReadOnly == nil || !*cart.ReadOnly {
		if planCode != "" && hasStartedAt && time.Since(since) >= unvalidatedCartMaxLifetime {
			oid, link, recErr := reconcileOrders()
			if recErr != nil {
				return CheckoutCheck{}, recErr
			}
			if oid != "" {
				return CheckoutCheck{OrderID: oid, OrderURL: link}, nil
			}
			return CheckoutCheck{CartExpired: true}, nil
		}
		return CheckoutCheck{}, fmt.Errorf("购物车尚未确认完成，继续核对原请求")
	}

	// Cart is readOnly:true (checkout succeeded or locked upstream).
	// Never call GET /order/cart/{cartId}/checkout (order:apiovh:cart/checkout/simulate);
	// reconcile against /me/order instead.
	if planCode == "" {
		return CheckoutCheck{}, fmt.Errorf("已锁定购物车缺少型号信息，无法在 /me/order 中核对订单")
	}
	oid, link, recErr := reconcileOrders()
	if recErr != nil {
		return CheckoutCheck{}, recErr
	}
	if oid != "" {
		return CheckoutCheck{OrderID: oid, OrderURL: link}, nil
	}
	if hasStartedAt && time.Since(since) >= lockedCartMissingOrderLimit {
		return CheckoutCheck{CartExpired: true}, nil
	}
	return CheckoutCheck{}, fmt.Errorf("已验证购物车在 /me/order 中尚未查到关联订单，稍后继续核对")
}

func parseJournalStart(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if t, ok := parseOrderTime(raw); ok {
		return t, true
	}
	if t, ok := types.ParseTS(raw); ok {
		return t, true
	}
	return time.Time{}, false
}

func RecordCheckedOrder(state *app.State, item *types.QueueItem, checked CheckoutCheck) {
	// Existing successful history is also the notification deduplication marker.
	for _, h := range state.HistorySnapshot() {
		if h.TaskID == item.ID && h.Status == "success" {
			return
		}
	}
	client, _ := state.OVH.IsolatedClientFor(item.AccountID)
	account, _ := state.FindAccount(item.AccountID)
	handleReconciledOrder(state, client, account.Endpoint, item, checked.OrderID, checked.OrderURL, newTimeline(), TimingKey(item.PlanCode, item.Datacenter))
}
