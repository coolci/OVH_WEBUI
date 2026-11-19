package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestLimiterSlidingWindow(t *testing.T) {
	limiter := NewLimiter(3, 10*time.Second)
	key := "user_ip_1"
	baseTime := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)

	// First 3 requests should succeed
	if !limiter.AllowAt(key, baseTime) {
		t.Fatal("Request 1 should be allowed")
	}
	if !limiter.AllowAt(key, baseTime.Add(1*time.Second)) {
		t.Fatal("Request 2 should be allowed")
	}
	if !limiter.AllowAt(key, baseTime.Add(2*time.Second)) {
		t.Fatal("Request 3 should be allowed")
	}

	// 4th request within 10s should be rejected
	if limiter.AllowAt(key, baseTime.Add(3*time.Second)) {
		t.Fatal("Request 4 should be rejected")
	}

	// Request after window passes should succeed
	if !limiter.AllowAt(key, baseTime.Add(11*time.Second)) {
		t.Fatal("Request after window should be allowed")
	}
}

func TestLimiterReset(t *testing.T) {
	limiter := NewLimiter(2, 5*time.Minute)
	key := "acc_fail"

	limiter.Allow(key)
	limiter.Allow(key)
	if limiter.Allow(key) {
		t.Fatal("Should be blocked after 2 failures")
	}

	limiter.Reset(key)
	if !limiter.Allow(key) {
		t.Fatal("Should be allowed after reset")
	}
}

func TestLimiterConcurrency(t *testing.T) {
	limiter := NewLimiter(50, 1*time.Second)
	key := "concurrent_key"
	var wg sync.WaitGroup

	allowed := 0
	var mu sync.Mutex

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.Allow(key) {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if allowed != 50 {
		t.Fatalf("Expected exactly 50 allowed requests under concurrent load, got %d", allowed)
	}
}
