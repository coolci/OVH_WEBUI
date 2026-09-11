package updater

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// 自更新后进程直接消失的那个竞争。
//
// 现场：Shutdown 让主 goroutine 里的 Serve 立刻返回 ErrServerClosed，
// main 判完那个 if 就走到函数末尾 —— main 返回 = 进程退出。
// 而 exec 排在另一个 goroutine 里，它还得先等 Shutdown 收尾、再关数据库。
// 主 goroutine 几乎总是先跑完，syscall.Exec 从来没被执行过。
// 日志上表现为"正在优雅关闭以完成重启"之后再无下文。
//
// 这个测试不打网络也不真的 exec：它复刻 main 的结构，
// 用 restartPending 标记 + 阻塞来验证"Serve 返回后主 goroutine 没有提前退出"。
func TestServeReturnMustNotRaceRestart(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: http.NewServeMux()}

	var restartPending atomic.Bool
	execReached := make(chan struct{})
	mainReturned := make(chan struct{})

	// 复刻 gracefulRestart：置位 → Shutdown → 关数据库（这里用 sleep 模拟耗时）→ exec
	go func() {
		restartPending.Store(true)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		// 关数据库那一步是有耗时的，正是它给了主 goroutine 抢跑的机会
		time.Sleep(120 * time.Millisecond)
		close(execReached)
	}()

	// 复刻 main 尾部
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			t.Errorf("serve: %v", err)
		}
		if restartPending.Load() {
			// 这里必须挂住，等 exec 接管。真实代码里 exec 成功就再也不返回了。
			<-execReached
		}
		close(mainReturned)
	}()

	select {
	case <-mainReturned:
		select {
		case <-execReached:
			// 对：main 等到了 exec 那一步
		default:
			t.Fatal("main 在 exec 之前就返回了 —— 进程会在这里直接退出，自更新永远完不成")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("超时")
	}
}

// 没有 restartPending 标记的话主 goroutine 会抢跑 —— 证明这个标记不是装饰。
func TestWithoutFlagMainWinsTheRace(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: http.NewServeMux()}

	execReached := make(chan struct{})
	mainReturned := make(chan struct{})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		time.Sleep(120 * time.Millisecond)
		close(execReached)
	}()
	go func() {
		_ = srv.Serve(ln)
		// 旧代码：不看任何标记，直接返回
		close(mainReturned)
	}()

	select {
	case <-mainReturned:
		select {
		case <-execReached:
			t.Log("这次 exec 先到了（竞争，不是每次都复现）")
		default:
			// 这就是线上看到的那一幕
			t.Log("复现：main 先返回，exec 还没执行 —— 进程就此消失")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("超时")
	}
}
