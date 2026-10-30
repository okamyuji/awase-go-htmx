package port

import (
	"context"
	"errors"
	"time"
)

// SessionTTL セッションの有効期間です。Cookie の Max-Age も同じ値です。
const SessionTTL = 30 * 24 * time.Hour

// 認証の失敗の種類です。
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrLocked             = errors.New("too many failed logins")
)

// LoginLimiter IP アドレスごとのログイン失敗の制限です。
type LoginLimiter interface {
	Blocked(ip string, now time.Time) bool
	Fail(ip string, now time.Time)
	Reset(ip string)
}

// AuthService 共有アカウントのログインとセッションです。
type AuthService interface {
	// Login 成功するとセッションのトークンを返します。失敗は ErrInvalidCredentials、拒否中は ErrLocked です。
	Login(ctx context.Context, ip, username, password string) (string, error)
	Logout(ctx context.Context, token string) error
	Authenticate(ctx context.Context, token string) (bool, error)
}
