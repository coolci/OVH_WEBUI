package catalogwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/types"
)

func TestRegression_BUG_OVH_01_CheckCheckoutReconcilesOrdersAndTerminatesExpired404(t *testing.T) {
	w, s := fixture(t)
	s.Accounts[0].AppKey = "fixture"
	s.Accounts[0].AppSecret = "fixture"
	s.Accounts[0].ConsumerKey = "fixture"

	nowISO := time.Now().UTC().Format(time.RFC3339)
	ordersJSON := `[778899]`

	transport := notificationTransport(func(r *http.Request) (*http.Response, error) {
		p := r.URL.Path
		body := `{}`
		status := 200
		switch {
		case strings.HasSuffix(p, "/auth/time"):
			body = fmt.Sprint(time.Now().Unix())
		case strings.HasSuffix(p, "/order/cart/cart-validated"):
			body = `{"cartId":"cart-validated","readOnly":true}`
		case strings.HasSuffix(p, "/order/cart/cart-validated/checkout") && r.Method == http.MethodGet:
			t.Errorf("CheckCheckout must NOT call simulation endpoint GET %s", p)
			body = `{"orderId":null,"prices":{},"contracts":[]}`
		case strings.HasSuffix(p, "/order/cart/cart-expired"):
			status = 404
			body = `{"message":"Cart not found"}`
		case strings.HasSuffix(p, "/me/order") && r.Method == http.MethodGet:
			body = ordersJSON
		case strings.HasSuffix(p, "/me/order/778899") && r.Method == http.MethodGet:
			body = fmt.Sprintf(`{"orderId":778899,"date":%q,"url":"https://www.ovh.com/order/778899"}`, nowISO)
		case strings.HasSuffix(p, "/me/order/778899/details") && r.Method == http.MethodGet:
			body = `[1]`
		case strings.HasSuffix(p, "/me/order/778899/details/1") && r.Method == http.MethodGet:
			body = `{"orderDetailId":1,"description":"KS (24sk40-v1) - Datacenter: gra"}`
		case strings.HasSuffix(p, "/me/order/778899/status"):
			body = `"notPaid"`
		}
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})
	restore := netfp.SwapSharedTransportForTesting(transport)
	defer restore()
	cli, err := s.OVH.ClientFor(s.Accounts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	cli.Client.Transport = transport

	// 1. Validated readOnly:true cart reconciles via /me/order on first check and releases cart registry
	ovh.TrackCart("cart-validated")
	defer ovh.ReleaseCart("cart-validated")
	w.data.Jobs["job-val"] = &Job{
		ID:        "job-val",
		Round:     1,
		AccountID: s.Accounts[0].ID,
		Endpoint:  s.Accounts[0].Endpoint,
		Candidate: Candidate{PlanCode: "24sk40-v1", Datacenters: []string{"gra"}},
		TaskID:    "catalog-watch-job-val-gra",
		Status:    "uncertain",
		Checkout:  &purchase.CheckoutJournal{CartID: "cart-validated", Submitted: true, StartedAt: time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano)},
		CheckDue:  0,
	}
	w.check = purchase.CheckCheckout
	w.Reconcile(context.Background())

	jVal := w.data.Jobs["job-val"]
	if jVal.Status != "completed" || jVal.OrderID != "778899" {
		t.Fatalf("expected validated cart job to reconcile to completed (#778899), got status=%q orderId=%q reason=%q",
			jVal.Status, jVal.OrderID, jVal.Reason)
	}
	if ovh.CartInUse("cart-validated") {
		t.Fatal("expected cart-validated to be released after reconciliation completed")
	}

	// 2. Expired 404 cart with no matching order in /me/order terminates cleanly instead of looping forever
	ordersJSON = `[]`
	delete(w.data.Jobs, "job-val")
	ovh.TrackCart("cart-expired")
	defer ovh.ReleaseCart("cart-expired")
	w.data.Jobs["job-exp"] = &Job{
		ID:        "job-exp",
		Round:     1,
		AccountID: s.Accounts[0].ID,
		Endpoint:  s.Accounts[0].Endpoint,
		Candidate: Candidate{PlanCode: "24sk40-v1", Datacenters: []string{"rbx"}},
		TaskID:    "catalog-watch-job-exp-rbx",
		Status:    "uncertain",
		Checkout:  &purchase.CheckoutJournal{CartID: "cart-expired", Submitted: true, StartedAt: time.Now().Add(-3 * time.Hour).UTC().Format(time.RFC3339Nano)},
		CheckDue:  0,
	}
	w.Reconcile(context.Background())

	jExp := w.data.Jobs["job-exp"]
	if jExp.Status == "uncertain" || jExp.Status == "pending" {
		t.Fatalf("expected expired 404 cart with empty /me/order to terminate safely, got status=%q reason=%q",
			jExp.Status, jExp.Reason)
	}
	if ovh.CartInUse("cart-expired") {
		t.Fatal("expected cart-expired to be released after terminal reconciliation")
	}
}

func TestRegression_BUG_OVH_13_ReenableRestoresSlotsExpandSkipsLicenseAndQueueCoversWildcard(t *testing.T) {
	w, s := fixture(t)

	// 1. Re-enabling catalogwatch must allow new round to replace old cancelled job slots
	candBase := Candidate{PlanCode: "24ska01", Options: []string{"ram-32g"}, Datacenters: []string{"gra"}}
	candNew := Candidate{PlanCode: "24ska01", Options: []string{"ram-32g"}, Datacenters: []string{"gra", "rbx"}}

	var cur map[string]Candidate
	w.fetch = func(context.Context, *app.State, string) (map[string]Candidate, error) {
		out := map[string]Candidate{}
		for k, v := range cur {
			out[k] = v
		}
		return out, nil
	}

	if err := w.SetEnabled(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	cur = map[string]Candidate{key("", candBase): candBase}
	if err := w.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	cur = map[string]Candidate{key("", candNew): candNew}
	if err := w.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	jobID := locationKey(s.Accounts[0].ID, candNew, "rbx")
	if w.data.Jobs[jobID] == nil || w.data.Jobs[jobID].Status != "pending" {
		t.Fatalf("expected initial pending job %s, got %+v", jobID, w.data.Jobs[jobID])
	}

	if err := w.SetEnabled(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if err := w.SetEnabled(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	cur = map[string]Candidate{key("", candBase): candBase}
	if err := w.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	cur = map[string]Candidate{key("", candNew): candNew}
	if err := w.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	jAfter := w.data.Jobs[jobID]
	if jAfter == nil || jAfter.Status != "pending" || jAfter.Round != w.data.Round {
		t.Fatalf("expected re-enabled round %d to create fresh pending job, got %+v", w.data.Round, jAfter)
	}

	// 2. Expand must skip a single plan requiring software license without aborting the whole catalog
	var rawCat Catalog
	if err := json.Unmarshal([]byte(`{
		"plans": [
			{"planCode": "24ska01", "configurations": [{"name": "dedicated_datacenter", "values": ["gra"]}]},
			{"planCode": "24win-special", "configurations": [{"name": "dedicated_datacenter", "values": ["gra"]}],
			 "addonFamilies": [{"name": "distribution-license", "mandatory": true, "addons": ["win-2022"]}]}
		]
	}`), &rawCat); err != nil {
		t.Fatal(err)
	}
	expanded, expandErr := Expand(rawCat)
	if expandErr != nil || len(expanded) != 1 {
		t.Fatalf("expected Expand to succeed with 1 candidate (skipping licensed plan), got len=%d err=%v", len(expanded), expandErr)
	}

	// 3. queueCovers: paused non-uncertain item must NOT block; active empty-DC item must cover all DCs
	s.Queue = []types.QueueItem{
		{ID: "q-empty-paused", AccountID: s.Accounts[0].ID, PlanCode: "24ska01", Datacenter: "", Status: "paused"},
	}
	_, covGRAPaused := queueCovers(s, s.Accounts[0].ID, "24ska01", "gra")
	_, covRBXPaused := queueCovers(s, s.Accounts[0].ID, "24ska01", "rbx")
	if covGRAPaused || covRBXPaused {
		t.Fatalf("expected paused item not to cover any DC: gra=%v rbx=%v", covGRAPaused, covRBXPaused)
	}

	s.Queue = []types.QueueItem{
		{ID: "q-empty-running", AccountID: s.Accounts[0].ID, PlanCode: "24ska01", Datacenter: "", Status: "running"},
	}
	_, covGRARunning := queueCovers(s, s.Accounts[0].ID, "24ska01", "gra")
	_, covRBXRunning := queueCovers(s, s.Accounts[0].ID, "24ska01", "rbx")
	if !covGRARunning || !covRBXRunning {
		t.Fatalf("expected active empty-DC item to cover all DCs: gra=%v rbx=%v", covGRARunning, covRBXRunning)
	}
}
