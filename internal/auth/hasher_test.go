package auth

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHashVerifyRoundTrip(t *testing.T) {
	h := NewScryptHasher()

	enc, err := h.Hash("not-a-real-pass-1")

	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "scrypt$32768$8$1$") {
		t.Fatalf("encoded = %q", enc)
	}
	if !h.Verify(enc, "not-a-real-pass-1") {
		t.Fatal("Verify(correct) = false")
	}
	if h.Verify(enc, "not-a-real-pass-2") {
		t.Fatal("Verify(wrong) = true")
	}
}

func TestHashUsesFreshSalt(t *testing.T) {
	h := NewScryptHasher()
	a, errA := h.Hash("same-password-x")
	b, errB := h.Hash("same-password-x")
	if errA != nil || errB != nil || a == b {
		t.Fatalf("hashes equal or error: %v %v", errA, errB)
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	h := NewScryptHasher()
	for _, bad := range []string{"", "scrypt$1$2$3", "bcrypt$32768$8$1$AAAA$AAAA", "scrypt$x$8$1$AAAA$AAAA", "scrypt$32768$8$1$!!!$AAAA", "scrypt$32768$8$1$AAAA$!!!", "scrypt$3$8$1$AAAA$AAAA"} {
		if h.Verify(bad, "anything") {
			t.Fatalf("Verify(%q) = true", bad)
		}
	}
}

func TestDummyHashNeverMatchesButCostsScrypt(t *testing.T) {
	h := NewScryptHasher()

	start := time.Now()
	ok := h.Verify(h.DummyHash(), "")

	if ok {
		t.Fatal("dummy hash matched")
	}
	if time.Since(start) < time.Millisecond {
		t.Fatal("dummy verify returned without computing scrypt")
	}
}

func TestScryptRunsOneAtATime(t *testing.T) {
	h := NewScryptHasher()
	enc, err := h.Hash("not-a-real-pass-1")
	if err != nil {
		t.Fatal(err)
	}
	var running, peak atomic.Int32
	base := h.derive
	h.derive = func(pw, salt []byte, n, r, p, keyLen int) ([]byte, error) {
		now := running.Add(1)
		defer running.Add(-1)
		for {
			cur := peak.Load()
			if now <= cur || peak.CompareAndSwap(cur, now) {
				break
			}
		}
		return base(pw, salt, n, r, p, keyLen)
	}

	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() { h.Verify(enc, "not-a-real-pass-1") })
	}
	wg.Wait()

	if peak.Load() != 1 {
		t.Fatalf("peak concurrent scrypt = %d, want 1", peak.Load())
	}
}
