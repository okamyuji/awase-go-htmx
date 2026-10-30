package auth

import (
	"sync"
	"time"
)

// 失敗の数え方です。値は要件定義書の FR-03 に従います。
const (
	maxFailures = 10
	failWindow  = 15 * time.Minute
	lockFor     = 15 * time.Minute
)

type failures struct {
	first time.Time
	count int
	until time.Time
}

// Limiter port.LoginLimiter のプロセス内の実装です。
// shortcut: プロセスを再起動すると回数は消えます。複数プロセスで動かすときは共有の保存先へ移します。
type Limiter struct {
	mu      sync.Mutex
	entries map[string]*failures
}

// NewLimiter 空の Limiter を返します。
func NewLimiter() *Limiter { return &Limiter{entries: map[string]*failures{}} }

// Blocked ip が拒否中なら真を返します。期限切れの項目はここで捨てます。
func (l *Limiter) Blocked(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, f := range l.entries {
		if !now.Before(f.until) && now.Sub(f.first) >= failWindow {
			delete(l.entries, k)
		}
	}
	f, ok := l.entries[ip]
	return ok && now.Before(f.until)
}

// Fail ip の失敗を1回数え、窓の中で上限に達したら lockFor の間拒否します。
func (l *Limiter) Fail(ip string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, ok := l.entries[ip]
	if !ok || now.Sub(f.first) >= failWindow {
		f = &failures{first: now}
		l.entries[ip] = f
	}
	f.count++
	if f.count >= maxFailures {
		f.until = now.Add(lockFor)
	}
}

// Reset ip の失敗の記録を消します。
func (l *Limiter) Reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, ip)
}
