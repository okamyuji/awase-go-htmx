package service

import (
	"crypto/sha256"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/okamyuji/awase-go-htmx/internal/domain"
	"github.com/okamyuji/awase-go-htmx/internal/port"
)

type authFixture struct {
	svc *Auth
	acc *fakeAccount
	ss  *fakeSessions
	hs  *fakeHasher
	lim *fakeLimiter
	clk *fixedClock
}

func newAuthFixture() authFixture {
	f := authFixture{
		acc: &fakeAccount{acc: port.Account{Username: "demo", PasswordHash: "hash:not-a-real-pass-1", Version: 3}},
		ss:  newFakeSessions(),
		hs:  &fakeHasher{},
		lim: &fakeLimiter{},
		clk: &fixedClock{now: t0},
	}
	f.svc = NewAuth(f.acc, f.ss, f.hs, f.lim, f.clk)
	return f
}

func TestLoginSuccessCreatesVersionedSession(t *testing.T) {
	f := newAuthFixture()

	tok, err := f.svc.Login(t.Context(), "1.2.3.4", "  demo ", "not-a-real-pass-1")

	if err != nil || tok == "" {
		t.Fatalf("Login = %q %v", tok, err)
	}
	sum := sha256.Sum256([]byte(tok))
	s, ok := f.ss.m[string(sum[:])]
	if !ok || s.version != 3 || !s.expires.Equal(t0.Add(30*24*time.Hour)) {
		t.Fatalf("session = %+v ok=%v", s, ok)
	}
	if !slices.Equal(f.lim.resets, []string{"1.2.3.4"}) || len(f.lim.fails) != 0 {
		t.Fatalf("limiter resets=%v fails=%v", f.lim.resets, f.lim.fails)
	}
	if !slices.Equal(f.ss.expiredSweep, []time.Time{t0}) {
		t.Fatalf("expired sweep = %v", f.ss.expiredSweep)
	}
}

func TestLoginFailuresAreCountedAndStillVerify(t *testing.T) {
	tests := []struct {
		name, user, pass string
		unset            bool
		wantHash         string
	}{
		{"パスワード違い", "demo", "wrong-password", false, "hash:not-a-real-pass-1"},
		{"ユーザー名違い", "other", "not-a-real-pass-1", false, "hash:not-a-real-pass-1"},
		{"アカウント未設定", "demo", "not-a-real-pass-1", true, "dummy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newAuthFixture()
			if tt.unset {
				f.acc.err = domain.ErrNotFound
			}

			_, err := f.svc.Login(t.Context(), "ip", tt.user, tt.pass)

			if !errors.Is(err, port.ErrInvalidCredentials) {
				t.Fatalf("err = %v", err)
			}
			if !slices.Equal(f.hs.verified, []string{tt.wantHash}) {
				t.Fatalf("verified = %v, want %s", f.hs.verified, tt.wantHash)
			}
			if !slices.Equal(f.lim.fails, []string{"ip"}) || len(f.ss.m) != 0 {
				t.Fatalf("fails=%v sessions=%d", f.lim.fails, len(f.ss.m))
			}
		})
	}
}

func TestLoginBlockedSkipsVerification(t *testing.T) {
	f := newAuthFixture()
	f.lim.blocked = true

	_, err := f.svc.Login(t.Context(), "ip", "demo", "not-a-real-pass-1")

	if !errors.Is(err, port.ErrLocked) || len(f.hs.verified) != 0 {
		t.Fatalf("err=%v verified=%v", err, f.hs.verified)
	}
}

func TestLoginPropagatesStorageErrors(t *testing.T) {
	f := newAuthFixture()
	f.acc.err = errors.New("disk")
	if _, err := f.svc.Login(t.Context(), "ip", "demo", "x"); err == nil || errors.Is(err, port.ErrInvalidCredentials) {
		t.Fatalf("account error: %v", err)
	}

	g := newAuthFixture()
	g.ss.err = errors.New("disk")
	if _, err := g.svc.Login(t.Context(), "ip", "demo", "not-a-real-pass-1"); err == nil {
		t.Fatal("session error swallowed")
	}
}

func TestAuthenticateAndLogout(t *testing.T) {
	f := newAuthFixture()
	tok, err := f.svc.Login(t.Context(), "ip", "demo", "not-a-real-pass-1")
	if err != nil {
		t.Fatal(err)
	}

	if ok, err := f.svc.Authenticate(t.Context(), tok); !ok || err != nil {
		t.Fatalf("Authenticate(valid) = %v %v", ok, err)
	}
	if ok, err := f.svc.Authenticate(t.Context(), ""); ok || err != nil {
		t.Fatalf("Authenticate(empty) = %v %v", ok, err)
	}
	if err := f.svc.Logout(t.Context(), tok); err != nil {
		t.Fatal(err)
	}
	if ok, _ := f.svc.Authenticate(t.Context(), tok); ok {
		t.Fatal("token valid after logout")
	}
}
