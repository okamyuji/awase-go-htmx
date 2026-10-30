package auth

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

func attemptN(l *Limiter, ip string, n int, at time.Time) int {
	allowed := 0
	for range n {
		if l.Attempt(ip, at) {
			allowed++
		}
	}
	return allowed
}

func TestLimiterAllowsTenAttemptsThenBlocksForFifteenMinutes(t *testing.T) {
	l := NewLimiter()

	if got := attemptN(l, "1.1.1.1", 9, t0); got != 9 {
		t.Fatalf("allowed %d of first 9", got)
	}
	if !l.Attempt("1.1.1.1", t0.Add(time.Minute)) {
		t.Fatal("10th attempt refused")
	}
	if l.Attempt("1.1.1.1", t0.Add(16*time.Minute-time.Second)) {
		t.Fatal("11th attempt allowed before 15 minutes passed")
	}
	if !l.Attempt("1.1.1.1", t0.Add(16*time.Minute)) {
		t.Fatal("still blocked after 15 minutes")
	}
}

func TestLimiterAllowsAtMostTenConcurrentAttempts(t *testing.T) {
	l := NewLimiter()
	var allowed atomic.Int32
	var wg sync.WaitGroup

	for range 100 {
		wg.Go(func() {
			if l.Attempt("ip", t0) {
				allowed.Add(1)
			}
		})
	}
	wg.Wait()

	if allowed.Load() != 10 {
		t.Fatalf("allowed = %d, want 10", allowed.Load())
	}
}

func TestLimiterForgetsAttemptsOutsideWindow(t *testing.T) {
	l := NewLimiter()

	attemptN(l, "ip", 9, t0)
	attemptN(l, "ip", 9, t0.Add(15*time.Minute))

	if !l.Attempt("ip", t0.Add(15*time.Minute)) {
		t.Fatal("blocked although earlier attempts are older than 15 minutes")
	}
}

func TestLimiterResetAndIsolation(t *testing.T) {
	l := NewLimiter()
	attemptN(l, "a", 9, t0)
	attemptN(l, "b", 10, t0)

	l.Reset("a")

	if got := attemptN(l, "a", 10, t0); got != 10 {
		t.Fatalf("a allowed %d after reset, want 10", got)
	}
	if l.Attempt("b", t0) {
		t.Fatal("b not blocked")
	}
}

func TestLimiterDropsStaleEntries(t *testing.T) {
	l := NewLimiter()
	attemptN(l, "old", 3, t0)

	l.Attempt("other", t0.Add(time.Hour))

	if n := len(l.entries); n != 1 {
		t.Fatalf("entries = %d, want only the new one", n)
	}
}
