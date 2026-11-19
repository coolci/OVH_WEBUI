package app

import (
	"fmt"
	"sync"
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

// BUG-BE-01: Racing SaveHistory/SaveQueue against DeleteAccount + ReloadHistoryFromDB/ReloadQueueFromDB
// must NEVER resurrect deleted-account rows into SQLite or memory, and must NEVER drop concurrent new orders for surviving accounts.
func TestRegression_BE01_ReloadHistoryAndQueueSynchronizedWithSave(t *testing.T) {
	s := newTestState(t)

	if err := s.DB.UpsertAccount(types.OVHAccount{ID: "acc-keep", Name: "Keep", Endpoint: "ovh-us", Zone: "US", IsDefault: true}); err != nil {
		t.Fatal(err)
	}

	for attempt := 0; attempt < 80; attempt++ {
		if err := s.DB.UpsertAccount(types.OVHAccount{ID: "acc-del", Name: "Del", Endpoint: "ovh-eu", Zone: "IE"}); err != nil {
			t.Fatal(err)
		}
		s.HistoryMu.Lock()
		s.History = []types.PurchaseHistoryEntry{
			{ID: "h-del", TaskID: "t-del", AccountID: "acc-del", PlanCode: "24ska01", Status: "success", PurchaseTime: types.NowISO()},
			{ID: "h-keep", TaskID: "t-keep", AccountID: "acc-keep", PlanCode: "24ska01", Status: "success", PurchaseTime: types.NowISO()},
		}
		s.HistoryMu.Unlock()
		if err := s.SaveHistory(); err != nil {
			t.Fatal(err)
		}

		s.QueueMu.Lock()
		s.Queue = []types.QueueItem{
			{ID: "q-del", AccountID: "acc-del", PlanCode: "24ska01", Datacenter: "gra", Status: "running", CreatedAt: types.NowISO(), UpdatedAt: types.NowISO()},
			{ID: "q-keep", AccountID: "acc-keep", PlanCode: "24ska01", Datacenter: "hil", Status: "running", CreatedAt: types.NowISO(), UpdatedAt: types.NowISO()},
		}
		s.QueueMu.Unlock()
		if err := s.SaveQueue(); err != nil {
			t.Fatal(err)
		}

		concurrentID := fmt.Sprintf("h-concurrent-%d", attempt)
		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			defer wg.Done()
			_ = s.SaveHistory()
			_ = s.SaveQueue()
		}()
		go func() {
			defer wg.Done()
			s.HistoryMu.Lock()
			s.History = append(s.History, types.PurchaseHistoryEntry{
				ID: concurrentID, TaskID: "t-conc", AccountID: "acc-keep", PlanCode: "24ska01", Status: "success", PurchaseTime: types.NowISO(),
			})
			s.HistoryMu.Unlock()
			_ = s.SaveHistory()
		}()
		go func() {
			defer wg.Done()
			_ = s.DB.DeleteAccount("acc-del")
			_ = s.ReloadQueueFromDB()
			_ = s.ReloadHistoryFromDB()
		}()
		wg.Wait()

		dbHist, err := s.DB.ListHistory()
		if err != nil {
			t.Fatal(err)
		}
		foundConcurrentDB := false
		for _, h := range dbHist {
			if h.AccountID == "acc-del" {
				t.Fatalf("attempt %d: deleted account 'acc-del' history row %q resurrected into SQLite!", attempt+1, h.ID)
			}
			if h.ID == concurrentID {
				foundConcurrentDB = true
			}
		}
		if !foundConcurrentDB {
			t.Fatalf("attempt %d: concurrent order %q for surviving account 'acc-keep' was lost from SQLite!", attempt+1, concurrentID)
		}
		foundConcurrentMem := false
		for _, h := range s.HistorySnapshot() {
			if h.AccountID == "acc-del" {
				t.Fatalf("attempt %d: deleted account 'acc-del' history row %q remained in memory!", attempt+1, h.ID)
			}
			if h.ID == concurrentID {
				foundConcurrentMem = true
			}
		}
		if !foundConcurrentMem {
			t.Fatalf("attempt %d: concurrent order %q for surviving account 'acc-keep' was lost from memory!", attempt+1, concurrentID)
		}

		dbQueue, err := s.DB.ListQueue()
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range dbQueue {
			if q.AccountID == "acc-del" {
				t.Fatalf("attempt %d: deleted account 'acc-del' queue item %q resurrected into SQLite!", attempt+1, q.ID)
			}
		}
		for _, q := range s.QueueSnapshot() {
			if q.AccountID == "acc-del" {
				t.Fatalf("attempt %d: deleted account 'acc-del' queue item %q remained in memory!", attempt+1, q.ID)
			}
		}
	}
}

// BUG-BE-04: Switching default account unconditionally syncs AppKey/AppSecret/ConsumerKey/Zone/Endpoint/IAM,
// and deleting all accounts clears residual credentials from Config.
func TestRegression_BE04_SyncDefaultAccountCredentialsAndClearOnEmpty(t *testing.T) {
	s := newTestState(t)

	accEU := types.OVHAccount{
		ID: "acc-eu", Name: "EU", Endpoint: "ovh-eu", Zone: "IE", IAM: "go-ovh-ie",
		AppKey: "AK_EU_111", AppSecret: "AS_EU_111", ConsumerKey: "CK_EU_111", IsDefault: true,
	}
	accUS := types.OVHAccount{
		ID: "acc-us", Name: "US", Endpoint: "ovh-us", Zone: "US", IAM: "go-ovh-us",
		AppKey: "AK_US_222", AppSecret: "AS_US_222", ConsumerKey: "CK_US_222", IsDefault: false,
	}
	if err := s.DB.UpsertAccount(accEU); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.UpsertAccount(accUS); err != nil {
		t.Fatal(err)
	}
	if err := s.ReloadAccounts(); err != nil {
		t.Fatal(err)
	}

	cfg1 := s.Config.Get()
	if cfg1.Endpoint != "ovh-eu" || cfg1.AppKey != "AK_EU_111" {
		t.Fatalf("unexpected initial config: endpoint=%q appKey=%q", cfg1.Endpoint, cfg1.AppKey)
	}

	// Switch default account from EU to US -> credentials MUST update to US!
	if err := s.DB.SetDefaultAccount("acc-us"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReloadAccounts(); err != nil {
		t.Fatal(err)
	}

	cfg2 := s.Config.Get()
	if cfg2.Endpoint != "ovh-us" || cfg2.Zone != "US" || cfg2.IAM != "go-ovh-us" {
		t.Fatalf("expected endpoint=ovh-us zone=US iam=go-ovh-us, got endpoint=%q zone=%q iam=%q", cfg2.Endpoint, cfg2.Zone, cfg2.IAM)
	}
	if cfg2.AppKey != "AK_US_222" || cfg2.AppSecret != "AS_US_222" || cfg2.ConsumerKey != "CK_US_222" {
		t.Fatalf("expected US credentials after default switch, got appKey=%q appSecret=%q consumerKey=%q",
			cfg2.AppKey, cfg2.AppSecret, cfg2.ConsumerKey)
	}

	// Delete all accounts -> credentials MUST be cleared from Config!
	if err := s.DB.DeleteAccount("acc-eu"); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.DeleteAccount("acc-us"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReloadAccounts(); err != nil {
		t.Fatal(err)
	}
	cfg3 := s.Config.Get()
	if cfg3.AppKey != "" || cfg3.AppSecret != "" || cfg3.ConsumerKey != "" {
		t.Fatalf("expected credentials cleared when all accounts deleted, got appKey=%q", cfg3.AppKey)
	}
}
