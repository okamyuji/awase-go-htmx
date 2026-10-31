// Package store port のリポジトリを SQLite で実装します。
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"time"

	"github.com/okamyuji/awase-go-htmx/internal/domain"

	_ "modernc.org/sqlite" // database/sql に "sqlite" ドライバを登録します。
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open path の SQLite を開き、未適用のスキーマを適用します。
func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := migrate(ctx, db); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return db, nil
}

// migrate PRAGMA user_version を適用済みの版として、番号順に未適用の SQL を実行します。
func migrate(ctx context.Context, db *sql.DB) error {
	var current int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	slices.Sort(names)
	for v := current + 1; v <= len(names); v++ {
		if err := applyMigration(ctx, db, names[v-1], v); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, name string, version int) error {
	body, err := migrations.ReadFile(name)
	if err != nil {
		return err
	}
	return withTx(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		// PRAGMA はプレースホルダを受け付けないため、整数を書式で埋め込みます。
		_, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", version))
		return err
	})
}

func withTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}

// mustAffect 1行も変更しなかった場合に domain.ErrNotFound を返します。
func mustAffect(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// touch イベントの更新日時を now にします。イベントがなければ domain.ErrNotFound です。
func touch(ctx context.Context, tx *sql.Tx, eventID string, now time.Time) error {
	return mustAffect(tx.ExecContext(ctx, "UPDATE events SET updated_at = ? WHERE id = ?", now.Unix(), eventID))
}

func closeInto(c io.Closer, err *error) {
	*err = errors.Join(*err, c.Close())
}

func unix(s int64) time.Time {
	return time.Unix(s, 0).UTC()
}

func notFoundOn(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}
