package purchase

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// Persisted before checkout: a process restart must never erase an unknown POST.
type CheckoutJournal struct {
	OrderID   string `json:"orderId,omitempty"`
	CartID    string `json:"cartId"`
	Submitted bool   `json:"submitted"`
	StartedAt string `json:"startedAt"`
}
type journalKey struct{}
type JournalWriter func(CheckoutJournal) error

func WithCheckoutJournal(ctx context.Context, write JournalWriter) context.Context {
	return context.WithValue(ctx, journalKey{}, write)
}
func saveCheckoutJournal(ctx context.Context, cartID string, orderIDs ...string) error {
	if write, ok := ctx.Value(journalKey{}).(JournalWriter); ok {
		j := CheckoutJournal{CartID: cartID, Submitted: true, StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
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
}

// Read-only, exact-cart reconciliation. An empty order list, a 404, or an
// unvalidated cart is NOT proof a previously submitted checkout cannot commit.
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
	path := "/order/cart/" + url.PathEscape(journal.CartID)
	var cart struct {
		ReadOnly *bool `json:"readOnly"`
	}
	if err = client.GetWithContext(ctx, path, &cart); err != nil {
		return CheckoutCheck{}, err
	}
	if cart.ReadOnly == nil || !*cart.ReadOnly {
		return CheckoutCheck{}, fmt.Errorf("购物车尚未确认完成，继续核对原请求")
	}
	var checkout struct {
		OrderID int64  `json:"orderId"`
		URL     string `json:"url"`
	}
	if err = client.GetWithContext(ctx, path+"/checkout", &checkout); err != nil {
		return CheckoutCheck{}, err
	}
	if checkout.OrderID <= 0 {
		return CheckoutCheck{}, fmt.Errorf("已验证购物车尚未返回可关联订单编号")
	}
	var order struct {
		OrderID int64  `json:"orderId"`
		URL     string `json:"url"`
	}
	if err = client.GetWithContext(ctx, fmt.Sprintf("/me/order/%d", checkout.OrderID), &order); err != nil {
		return CheckoutCheck{}, err
	}
	if order.OrderID != checkout.OrderID {
		return CheckoutCheck{}, fmt.Errorf("订单关联尚未确认")
	}
	return CheckoutCheck{OrderID: fmt.Sprint(order.OrderID), OrderURL: order.URL}, nil
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
