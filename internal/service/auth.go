// Package service awase のユースケースです。リポジトリは port のインターフェースで受け取ります。
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/okamyuji/awase-go-htmx/internal/domain"
	"github.com/okamyuji/awase-go-htmx/internal/port"
)

// Auth port.AuthService の実装です。
type Auth struct {
	account  port.AccountRepository
	sessions port.SessionRepository
	hasher   port.PasswordHasher
	limiter  port.LoginLimiter
	clock    port.Clock
}

// NewAuth 依存を受け取って Auth を返します。
func NewAuth(a port.AccountRepository, s port.SessionRepository, h port.PasswordHasher, l port.LoginLimiter, c port.Clock) *Auth {
	return &Auth{account: a, sessions: s, hasher: h, limiter: l, clock: c}
}

// Login 共有アカウントの資格情報を照合し、成功したらセッションのトークンを返します。
func (s *Auth) Login(ctx context.Context, ip, username, password string) (string, error) {
	now := s.clock.Now()
	if s.limiter.Blocked(ip, now) {
		return "", port.ErrLocked
	}
	// 照合の前に版を読むので、照合中に passwd が確定してもこのセッションは古い版で作られて無効になります。
	acc, err := s.account.Get(ctx)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return "", err
	}
	if !s.matches(acc, err == nil, username, password) {
		s.limiter.Fail(ip, now)
		return "", port.ErrInvalidCredentials
	}
	s.limiter.Reset(ip)
	if err := s.sessions.DeleteExpired(ctx, now); err != nil {
		return "", err
	}
	token := rand.Text()
	if err := s.sessions.Create(ctx, hashToken(token), acc.Version, now, now.Add(port.SessionTTL)); err != nil {
		return "", err
	}
	return token, nil
}

// matches アカウントがない場合とユーザー名が違う場合も scrypt を計算し、応答時間で違いを判別させません。
func (s *Auth) matches(acc port.Account, exists bool, username, password string) bool {
	hash := s.hasher.DummyHash()
	if exists {
		hash = acc.PasswordHash
	}
	userOK := subtle.ConstantTimeCompare([]byte(strings.TrimSpace(username)), []byte(acc.Username)) == 1
	passOK := s.hasher.Verify(hash, password)
	return exists && userOK && passOK
}

// Logout トークンのセッションを消します。
func (s *Auth) Logout(ctx context.Context, token string) error {
	return s.sessions.Delete(ctx, hashToken(token))
}

// Authenticate トークンが有効なセッションなら真を返します。
func (s *Auth) Authenticate(ctx context.Context, token string) (bool, error) {
	if token == "" {
		return false, nil
	}
	return s.sessions.Valid(ctx, hashToken(token), s.clock.Now())
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
