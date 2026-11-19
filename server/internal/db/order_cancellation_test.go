package db

import (
	"errors"
	"sync"
	"testing"
)

func TestAbandonPaymentRaceAndPersistence(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for id := int64(1001); id < 1021; id++ {
		var wg sync.WaitGroup
		var paid, abandoned bool
		wg.Add(2)
		go func() {
			defer wg.Done()
			var err error
			paid, err = database.ClaimOrderPayment("account", id)
			if err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			err := database.ChangeOrderCancellation("account", id, "abandon")
			abandoned = err == nil
			if err != nil && !errors.Is(err, ErrOrderActionConflict) {
				t.Error(err)
			}
		}()
		wg.Wait()
		if paid == abandoned {
			t.Fatalf("payment=%v abandonment=%v; exactly one must win", paid, abandoned)
		}
	}
	if err := database.ChangeOrderCancellation("account", 2001, "abandon"); err != nil {
		t.Fatal(err)
	}
	if err := database.ChangeOrderCancellation("account", 2002, "retract"); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	database, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if claimed, err := database.ClaimOrderPayment("account", 2001); err != nil || claimed {
		t.Fatalf("abandonment lost on restart: %v %v", claimed, err)
	}
	if err := database.ChangeOrderCancellation("account", 2002, "retract"); !errors.Is(err, ErrOrderActionConflict) {
		t.Fatalf("retraction retried on restart: %v", err)
	}
	if claimed, err := database.ClaimOrderPayment("other", 2001); err != nil || !claimed {
		t.Fatalf("account isolation failed: %v %v", claimed, err)
	}
}
