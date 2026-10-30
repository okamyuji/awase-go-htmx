package store

import (
	"context"
	"database/sql"
	"time"
)

// Sessions port.SessionRepository の SQLite 実装です。
type Sessions struct{ db *sql.DB }

// NewSessions db を使う Sessions を返します。
func NewSessions(db *sql.DB) *Sessions { return &Sessions{db: db} }

// Create セッションを保存します。
func (s *Sessions) Create(ctx context.Context, tokenHash []byte, accountVersion int64, now, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO sessions (token_hash, account_version, created_at, expires_at) VALUES (?, ?, ?, ?)",
		tokenHash, accountVersion, now.Unix(), expires.Unix())
	return err
}

// Valid 期限内で、作成時の版が現在のアカウントの版と等しいセッションがあれば真を返します。
func (s *Sessions) Valid(ctx context.Context, tokenHash []byte, now time.Time) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM sessions s JOIN account a ON a.version = s.account_version
		WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, now.Unix()).Scan(&n)
	return n > 0, err
}

// Delete セッションを消します。
func (s *Sessions) Delete(ctx context.Context, tokenHash []byte) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", tokenHash)
	return err
}

// DeleteExpired 期限切れのセッションを消します。
func (s *Sessions) DeleteExpired(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at <= ?", now.Unix())
	return err
}
