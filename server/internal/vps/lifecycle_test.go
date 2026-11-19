package vps

import (
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/types"
)

func TestAuditVPSRestartMustNotDeadlock(t *testing.T) {
	dir := t.TempDir()
	state := &app.State{
		VPSCheckInterval: 60,
		Logger:           logger.New(filepath.Join(dir, "audit.log"), slog.New(slog.NewTextHandler(io.Discard, nil))),
	}
	oldWG := &sync.WaitGroup{}
	oldWG.Add(1)
	runningMu.Lock()
	running = true
	generation = 1
	loopWg = oldWG
	runningMu.Unlock()

	tgCheckMu.Lock()
	lastTGCheck = time.Now()
	tgCheckMu.Unlock()

	go func() {
		defer oldWG.Done()
		monitorLoopGen(state, 1)
	}()

	deadline := time.Now().Add(time.Second)
	ready := false
	for !ready {
		for _, entry := range state.Logger.Snapshot() {
			if strings.Contains(entry.Message, "等待 60 秒") {
				ready = true
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("old real loop did not enter sleep")
		}
		time.Sleep(time.Millisecond)
	}

	if !Stop(state) {
		t.Fatal("stop rejected")
	}

	started := make(chan bool, 1)
	go func() {
		started <- Start(state)
	}()

	select {
	case <-started:
	case <-time.After(1250 * time.Millisecond):
		if runningMu.TryLock() {
			runningMu.Unlock()
			t.Fatal("unexpected unlocked runningMu")
		}
		t.Fatal("real monitorLoopGen cannot exit via stillMine: Start owns runningMu while blocked on its WaitGroup")
	}
}

// VPS 自动下单已重新开启:PurchaseVPS 走真实下单链路,缺下单账户是确定性失败
func TestPurchaseVPSRequiresOrderAccount(t *testing.T) {
	out := PurchaseVPS(nil, types.VPSSubscription{ID: "sub-audit", PlanCode: "vps-starter-1-2-20", OvhSubsidiary: "IE", AutoOrder: true}, "GRA")
	if out.Success || !out.Fatal || !strings.Contains(out.Reason, "下单账户") {
		t.Fatalf("missing account must be a fatal failure: %+v", out)
	}
}
