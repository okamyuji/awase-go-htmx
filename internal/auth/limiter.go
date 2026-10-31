package auth

import (
	"net/netip"
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

// Attempt ip のログインの試行を1回数え、照合してよければ真を返します。
// 判定と加算を同じロックの中で行うので、同時に送られた試行も上限を超えて照合されません。
// 成功した試行は Reset で消すため、数え残るのは失敗だけです。
func (l *Limiter) Attempt(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, f := range l.entries {
		if !now.Before(f.until) && now.Sub(f.first) >= failWindow {
			delete(l.entries, k)
		}
	}
	ip = limitKey(ip)
	f, ok := l.entries[ip]
	if ok && now.Before(f.until) {
		return false
	}
	// 窓の切れた記録は上のループで消えているため、残っている記録はすべて窓の中にあります。
	if !ok {
		f = &failures{first: now}
		l.entries[ip] = f
	}
	f.count++
	if f.count >= maxFailures {
		f.until = now.Add(lockFor)
	}
	return true
}

// Reset ip の試行の記録を消します。
func (l *Limiter) Reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, limitKey(ip))
}

// limitKey ip を数える単位にそろえます。IPv6 は /64 で数えます。
// 1つの回線に /64 が割り当てられるため、アドレスを替えるだけで制限を逃れられないようにします。
// IP として読めない値は、そのまま使います。
func limitKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	addr = addr.Unmap()
	if addr.Is4() {
		return addr.String()
	}
	return netip.PrefixFrom(addr, 64).Masked().String()
}
