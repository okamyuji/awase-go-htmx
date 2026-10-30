package service

import (
	"context"
	"time"

	"github.com/okamyuji/awase-go-htmx/internal/port"
)

var t0 = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (c *fixedClock) Now() time.Time { return c.now }

type fakeHasher struct{ verified []string }

func (h *fakeHasher) Hash(pw string) (string, error) { return "hash:" + pw, nil }
func (h *fakeHasher) DummyHash() string              { return "dummy" }
func (h *fakeHasher) Verify(hash, pw string) bool {
	h.verified = append(h.verified, hash)
	return hash == "hash:"+pw
}

type fakeAccount struct {
	acc port.Account
	err error
}

func (f *fakeAccount) Get(context.Context) (port.Account, error) { return f.acc, f.err }
func (f *fakeAccount) Set(context.Context, string, string, time.Time) (int64, error) {
	return 0, nil
}

type session struct {
	version int64
	expires time.Time
}

type fakeSessions struct {
	m            map[string]session
	expiredSweep []time.Time
	err          error
}

func newFakeSessions() *fakeSessions { return &fakeSessions{m: map[string]session{}} }

func (f *fakeSessions) Create(_ context.Context, h []byte, v int64, _ time.Time, exp time.Time) error {
	f.m[string(h)] = session{v, exp}
	return f.err
}
func (f *fakeSessions) Valid(_ context.Context, h []byte, now time.Time) (bool, error) {
	s, ok := f.m[string(h)]
	return ok && now.Before(s.expires), f.err
}
func (f *fakeSessions) Delete(_ context.Context, h []byte) error {
	delete(f.m, string(h))
	return f.err
}
func (f *fakeSessions) DeleteExpired(_ context.Context, now time.Time) error {
	f.expiredSweep = append(f.expiredSweep, now)
	return nil
}

type fakeLimiter struct {
	blocked  bool
	attempts []string
	resets   []string
}

func (f *fakeLimiter) Attempt(ip string, _ time.Time) bool {
	f.attempts = append(f.attempts, ip)
	return !f.blocked
}
func (f *fakeLimiter) Reset(ip string) { f.resets = append(f.resets, ip) }
