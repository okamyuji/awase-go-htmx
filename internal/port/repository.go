// Package port 層をつなぐインターフェースです。サービスはリポジトリを、ハンドラはサービスをここ経由で受け取ります。
package port

import (
	"context"
	"time"

	"github.com/okamyuji/awase-go-htmx/internal/domain"
)

// Clock 現在時刻を返します。テストで時刻を固定するために差し替えます。
type Clock interface {
	Now() time.Time
}

// IDGen イベント ID を作ります。
type IDGen interface {
	NewID() string
}

// PasswordHasher パスワードのハッシュ化と照合です。
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(hash, password string) bool
}

// Account 共有アカウントです。Version は passwd のたびに1増えます。
type Account struct {
	Username     string
	PasswordHash string
	Version      int64
}

// EventRepository イベントと候補の永続化です。存在しない ID には domain.ErrNotFound を返します。
type EventRepository interface {
	Create(ctx context.Context, ev domain.Event) error
	Get(ctx context.Context, id string) (domain.Event, error)
	List(ctx context.Context) ([]domain.EventSummary, error)
	UpdateMeta(ctx context.Context, id, title, memo string, now time.Time) error
	AddCandidates(ctx context.Context, id string, labels []string, now time.Time) error
	DeleteCandidate(ctx context.Context, id string, cid int64, now time.Time) error
	Delete(ctx context.Context, id string) error
}

// AnswerRepository 回答の永続化です。別のイベントの回答 ID には domain.ErrNotFound を返します。
type AnswerRepository interface {
	Create(ctx context.Context, a domain.Answer) (int64, error)
	Get(ctx context.Context, eventID string, aid int64) (domain.Answer, error)
	ListByEvent(ctx context.Context, eventID string) ([]domain.Answer, error)
	Update(ctx context.Context, a domain.Answer) error
	Delete(ctx context.Context, eventID string, aid int64, now time.Time) error
}

// AccountRepository 共有アカウントの永続化です。未設定の Get は domain.ErrNotFound を返します。
type AccountRepository interface {
	Get(ctx context.Context) (Account, error)
	// Set アカウントを置き換えて版を1増やし、全セッションを削除します。新しい版を返します。
	Set(ctx context.Context, username, hash string, now time.Time) (int64, error)
}

// SessionRepository セッションの永続化です。トークンは SHA-256 のハッシュだけを扱います。
type SessionRepository interface {
	Create(ctx context.Context, tokenHash []byte, accountVersion int64, now, expires time.Time) error
	// Valid 期限内で、作成時の版が現在のアカウントの版と等しいときに真を返します。
	Valid(ctx context.Context, tokenHash []byte, now time.Time) (bool, error)
	Delete(ctx context.Context, tokenHash []byte) error
	DeleteExpired(ctx context.Context, now time.Time) error
}
