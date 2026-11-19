package db

import (
	"reflect"
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

func TestMonitorAccountsRoundTripUpsertAndDelete(t *testing.T) {
	database, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	for _, id := range []string{"a", "b"} {
		if err := database.UpsertAccount(types.OVHAccount{ID: id, Name: id, Endpoint: "ovh-eu"}); err != nil {
			t.Fatal(err)
		}
	}
	sub := types.Subscription{PlanCode: "custom", CreatedAt: types.NowISO(), AutoOrder: true, AutoPay: true, AutoOrderAccountID: "a", AutoOrderAccountIDs: []string{"a", "b"}, Quantity: 2}
	if err := database.UpsertMonitorSubscription(sub); err != nil {
		t.Fatal(err)
	}
	if err := database.UpsertMonitorSubscription(sub); err != nil {
		t.Fatal(err)
	}
	rows, err := database.ListMonitorSubscriptions()
	if err != nil || len(rows) != 1 || !reflect.DeepEqual(rows[0].AutoOrderAccountIDs, []string{"a", "b"}) {
		t.Fatalf("upsert lost account list: %+v %v", rows, err)
	}
	if err := database.DeleteAccount("a"); err != nil {
		t.Fatal(err)
	}
	rows, err = database.ListMonitorSubscriptions()
	if err != nil || !reflect.DeepEqual(rows[0].AutoOrderAccountIDs, []string{"b"}) || rows[0].AutoOrderAccountID != "b" || !rows[0].AutoOrder || !rows[0].AutoPay {
		t.Fatalf("deleting one account affected remaining target: %+v %v", rows, err)
	}
	if err := database.DeleteAccount("b"); err != nil {
		t.Fatal(err)
	}
	rows, err = database.ListMonitorSubscriptions()
	if err != nil || len(rows[0].AutoOrderAccountIDs) != 0 || rows[0].AutoOrderAccountID != "" || rows[0].AutoOrder || rows[0].AutoPay {
		t.Fatalf("last account deletion did not disable ordering: %+v %v", rows, err)
	}
}
