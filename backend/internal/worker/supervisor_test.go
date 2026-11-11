package worker

import (
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/storage"
)

func TestSupervisorStopIdempotent(t *testing.T) {
	st := &app.State{
		Paths: storage.DefaultPaths(),
	}
	sup := NewSupervisor(st, nil)
	// Stop should be safe to call multiple times without panicking
	sup.Stop()
	sup.Stop()
}

func TestApplySharedProxyNilSafe(t *testing.T) {
	// Should not panic on nil state
	ApplySharedProxy(nil)

	st := &app.State{}
	ApplySharedProxy(st)
}

// Stop 必须等队列循环(含在途结账)真正返回才回来:调用方紧接着就会关数据库,
// 先关库的话结账成功记录写不进去,重启后任务复活再买一台。
func TestSupervisorStopWaitsForQueueLoop(t *testing.T) {
	st := &app.State{Paths: storage.DefaultPaths()}
	sup := NewSupervisor(st, nil)
	sup.queueDone = make(chan struct{})
	released := make(chan struct{})
	go func() {
		<-sup.ctx.Done() // 循环看到取消后,还要等在途任务收尾
		time.Sleep(150 * time.Millisecond)
		close(released)
		close(sup.queueDone)
	}()
	sup.Stop()
	select {
	case <-released:
	default:
		t.Fatal("Stop 在队列循环退出前就返回了")
	}
}
