package auth

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

func failN(l *Limiter, ip string, n int, at time.Time) {
	for range n {
		l.Fail(ip, at)
	}
}

func TestLimiterBlocksOnTenthFailureForFifteenMinutes(t *testing.T) {
	l := NewLimiter()

	failN(l, "1.1.1.1", 9, t0)
	if l.Blocked("1.1.1.1", t0) {
		t.Fatal("blocked after 9 failures")
	}
	l.Fail("1.1.1.1", t0.Add(time.Minute))

	if !l.Blocked("1.1.1.1", t0.Add(time.Minute)) {
		t.Fatal("not blocked after 10 failures")
	}
	if !l.Blocked("1.1.1.1", t0.Add(16*time.Minute-time.Second)) {
		t.Fatal("unblocked before 15 minutes passed")
	}
	if l.Blocked("1.1.1.1", t0.Add(16*time.Minute)) {
		t.Fatal("still blocked after 15 minutes")
	}
}

func TestLimiterForgetsFailuresOutsideWindow(t *testing.T) {
	l := NewLimiter()

	failN(l, "ip", 9, t0)
	l.Fail("ip", t0.Add(15*time.Minute))

	if l.Blocked("ip", t0.Add(15*time.Minute)) {
		t.Fatal("blocked although earlier failures are older than 15 minutes")
	}
}

func TestLimiterResetAndIsolation(t *testing.T) {
	l := NewLimiter()
	failN(l, "a", 9, t0)
	failN(l, "b", 10, t0)

	l.Reset("a")
	l.Fail("a", t0)

	if l.Blocked("a", t0) {
		t.Fatal("a blocked after reset")
	}
	if !l.Blocked("b", t0) {
		t.Fatal("b not blocked")
	}
}

func TestLimiterDropsStaleEntries(t *testing.T) {
	l := NewLimiter()
	failN(l, "old", 3, t0)

	l.Blocked("other", t0.Add(time.Hour))

	if n := len(l.entries); n != 0 {
		t.Fatalf("entries = %d, want 0", n)
	}
}
