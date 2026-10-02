// Package ratelimit provides an in-memory, thread-safe rate limiter (UA-07 / D-29).
// It tracks attempts per key within a moving time window.
package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu          sync.Mutex
	maxAttempts int
	window      time.Duration
	attempts    map[string][]time.Time
}

// NewLimiter creates a Limiter allowing up to maxAttempts within window.
func NewLimiter(maxAttempts int, window time.Duration) *Limiter {
	return &Limiter{
		maxAttempts: maxAttempts,
		window:      window,
		attempts:    make(map[string][]time.Time),
	}
}

// Allow checks whether an action for key is permitted under rate limit.
// If permitted, it records this attempt and returns true. Otherwise false.
func (l *Limiter) Allow(key string) bool {
	return l.AllowAt(key, time.Now())
}

// AllowAt allows injecting clock for deterministic testing.
func (l *Limiter) AllowAt(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)
	times := l.attempts[key]

	// Prune timestamps older than window
	valid := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= l.maxAttempts {
		l.attempts[key] = valid
		return false
	}

	l.attempts[key] = append(valid, now)
	return true
}

// Reset clears attempts for a key (e.g. after successful authentication).
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

// Attempts returns current active attempt count in window for key.
func (l *Limiter) Attempts(key string) int {
	return l.AttemptsAt(key, time.Now())
}

func (l *Limiter) AttemptsAt(key string, now time.Time) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)
	count := 0
	for _, t := range l.attempts[key] {
		if t.After(cutoff) {
			count++
		}
	}
	return count
}
