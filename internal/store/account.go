package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/okamyuji/awase-go-htmx/internal/port"
)

// Account port.AccountRepository の SQLite 実装です。
type Account struct{ db *sql.DB }

// NewAccount db を使う Account を返します。
func NewAccount(db *sql.DB) *Account { return &Account{db: db} }

// Get 共有アカウントを返します。
func (s *Account) Get(ctx context.Context) (port.Account, error) {
	var a port.Account
	err := s.db.QueryRowContext(ctx, "SELECT username, password_hash, version FROM account WHERE id = 1").
		Scan(&a.Username, &a.PasswordHash, &a.Version)
	return a, notFoundOn(err)
}

// Set 共有アカウントを置き換えて版を1増やし、同じトランザクションで全セッションを消します。
func (s *Account) Set(ctx context.Context, username, hash string, now time.Time) (version int64, err error) {
	err = withTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, "SELECT coalesce(max(version), 0) + 1 FROM account").Scan(&version); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO account (id, username, password_hash, version, updated_at) VALUES (1, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET username = excluded.username, password_hash = excluded.password_hash,
			version = excluded.version, updated_at = excluded.updated_at`, username, hash, version, now.Unix()); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "DELETE FROM sessions")
		return err
	})
	return version, err
}
