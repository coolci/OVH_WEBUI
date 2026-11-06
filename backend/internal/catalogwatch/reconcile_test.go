package catalogwatch

import (
	"context"
	"fmt"
	"testing"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/purchase"
)

func TestReconciliationNeverRetriesUnknownSubmission(t *testing.T) {
	w, _ := fixture(t)
	ready(t, w)
	j := sole(t, w)
	j.Status = "uncertain"
	j.Checkout = &purchase.CheckoutJournal{CartID: "original", Submitted: true}
	w.check = func(context.Context, *app.State, string, *purchase.CheckoutJournal) (purchase.CheckoutCheck, error) {
		return purchase.CheckoutCheck{}, fmt.Errorf("429 or no visible order yet")
	}
	for i := 0; i < 3; i++ {
		j.CheckDue = 0
		w.Reconcile(context.Background())
		if j.Status != "uncertain" {
			t.Fatal("unknown result must never buy again")
		}
	}
	if j.CheckAttempts != 3 {
		t.Fatal("must keep checking automatically")
	}
}

func TestInterruptedPreCheckoutAutomaticallyResumesOnlyCurrentRound(t *testing.T) {
	for _, toggle := range []bool{false, true} {
		t.Run(fmt.Sprint(toggle), func(t *testing.T) {
			w, s := fixture(t)
			ready(t, w)
			j := sole(t, w)
			j.Status = "attempting"
			j.Checkout = &purchase.CheckoutJournal{}
			if err := w.save(); err != nil {
				t.Fatal(err)
			}
			w = New(s)
			j = sole(t, w)
			if toggle {
				if err := w.SetEnabled(context.Background(), false); err != nil {
					t.Fatal(err)
				}
			}
			w.Reconcile(context.Background())
			want := "pending"
			if toggle {
				want = "cancelled"
			}
			if j.Status != want {
				t.Fatalf("got %s want %s", j.Status, want)
			}
		})
	}
}

func TestRoundCountersResetWithoutDeletingSuccessOrLateResult(t *testing.T) {
	w, s := fixture(t)
	ready(t, w)
	j := sole(t, w)
	j.Status = "completed"
	j.OrderID = "known"
	counts := func() map[string]int { return w.Snapshot().(map[string]interface{})["counts"].(map[string]int) }
	if counts()["completed"] != 1 {
		t.Fatal("initial count")
	}
	for _, enabled := range []bool{false, true} {
		if err := w.SetEnabled(context.Background(), enabled); err != nil {
			t.Fatal(err)
		}
		if counts()["completed"] != 0 {
			t.Fatal("counter not reset")
		}
	}
	if sole(t, w).OrderID != "known" {
		t.Fatal("success ledger erased")
	}
	w = New(s)
	if counts()["completed"] != 0 {
		t.Fatal("round not persisted")
	}
	ready(t, w)
	if len(w.data.Jobs) != 1 || sole(t, w).Status != "completed" {
		t.Fatal("new round replayed old success")
	}
}
