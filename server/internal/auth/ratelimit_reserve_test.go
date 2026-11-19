package auth

import (
	"sync"
	"testing"
	"time"
)

// 一批并发猜测不能在计数涨到上限之前一起通过("先查后记"竞态)。
func TestAuthReserveCountsInflightAttempts(t *testing.T) {
	ResetAuthFailures()
	t.Cleanup(ResetAuthFailures)
	ip := "203.0.113.9"
	now := time.Now()
	var passed int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if blocked, _ := authReserve(ip, now); !blocked {
				mu.Lock()
				passed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if passed != MaxAuthFailures {
		t.Fatalf("同时在途的校验应被限制在 %d 个, 实际放行 %d", MaxAuthFailures, passed)
	}
	for i := 0; i < passed; i++ {
		authRelease(ip, authFailed, now)
	}
	if blocked, _ := authReserve(ip, now); !blocked {
		t.Fatal("失败满额后应被挡下")
	}
}

// 正常用户:校验成功会清零,在途名额归还后不会越积越多。
func TestAuthReserveSuccessClears(t *testing.T) {
	ResetAuthFailures()
	t.Cleanup(ResetAuthFailures)
	ip := "203.0.113.10"
	now := time.Now()
	for i := 0; i < 100; i++ {
		if blocked, _ := authReserve(ip, now); blocked {
			t.Fatalf("第 %d 次正常请求被挡", i)
		}
		authRelease(ip, authSucceeded, now)
	}
	for i := 0; i < MaxAuthFailures-1; i++ {
		authReserve(ip, now)
		authRelease(ip, authFailed, now)
	}
	authReserve(ip, now)
	authRelease(ip, authSucceeded, now)
	if blocked, _ := authReserve(ip, now); blocked {
		t.Fatal("成功后应清零")
	}
}
