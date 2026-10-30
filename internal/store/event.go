package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/okamyuji/awase-go-htmx/internal/domain"
)

// Events port.EventRepository の SQLite 実装です。
type Events struct{ db *sql.DB }

// NewEvents db を使う Events を返します。
func NewEvents(db *sql.DB) *Events { return &Events{db: db} }

// Create イベントと候補を1つのトランザクションで保存します。候補の表示順は並びの順です。
func (s *Events) Create(ctx context.Context, ev domain.Event) error {
	return withTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO events (id, title, memo, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			ev.ID, ev.Title, ev.Memo, ev.CreatedAt.Unix(), ev.UpdatedAt.Unix()); err != nil {
			return err
		}
		labels := make([]string, len(ev.Candidates))
		for i, c := range ev.Candidates {
			labels[i] = c.Label
		}
		return insertCandidates(ctx, tx, ev.ID, labels, 0)
	})
}

// Get イベントと候補を返します。
func (s *Events) Get(ctx context.Context, id string) (domain.Event, error) {
	ev := domain.Event{ID: id}
	var created, updated int64
	err := s.db.QueryRowContext(ctx, "SELECT title, memo, created_at, updated_at FROM events WHERE id = ?", id).
		Scan(&ev.Title, &ev.Memo, &created, &updated)
	if err != nil {
		return domain.Event{}, notFoundOn(err)
	}
	ev.CreatedAt, ev.UpdatedAt = unix(created), unix(updated)
	ev.Candidates, err = s.candidates(ctx, id)
	return ev, err
}

func (s *Events) candidates(ctx context.Context, id string) (out []domain.Candidate, err error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, position, label FROM candidates WHERE event_id = ? ORDER BY position, id", id)
	if err != nil {
		return nil, err
	}
	defer closeInto(rows, &err)
	for rows.Next() {
		var c domain.Candidate
		if err := rows.Scan(&c.ID, &c.Position, &c.Label); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// List イベントを作成日時の新しい順に返します。
func (s *Events) List(ctx context.Context) (out []domain.EventSummary, err error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.title, e.created_at, e.updated_at,
		       (SELECT count(*) FROM candidates c WHERE c.event_id = e.id),
		       (SELECT count(*) FROM answers a WHERE a.event_id = e.id)
		FROM events e ORDER BY e.created_at DESC, e.id DESC`)
	if err != nil {
		return nil, err
	}
	defer closeInto(rows, &err)
	for rows.Next() {
		var e domain.EventSummary
		var created, updated int64
		if err := rows.Scan(&e.ID, &e.Title, &created, &updated, &e.CandidateCount, &e.AnswerCount); err != nil {
			return nil, err
		}
		e.CreatedAt, e.UpdatedAt = unix(created), unix(updated)
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpdateMeta タイトルとメモを変えます。
func (s *Events) UpdateMeta(ctx context.Context, id, title, memo string, now time.Time) error {
	return mustAffect(s.db.ExecContext(ctx, "UPDATE events SET title = ?, memo = ?, updated_at = ? WHERE id = ?", title, memo, now.Unix(), id))
}

// AddCandidates 候補を既存の候補の後ろに足します。
func (s *Events) AddCandidates(ctx context.Context, id string, labels []string, now time.Time) error {
	return withTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := touch(ctx, tx, id, now); err != nil {
			return err
		}
		var next int
		if err := tx.QueryRowContext(ctx, "SELECT coalesce(max(position) + 1, 0) FROM candidates WHERE event_id = ?", id).Scan(&next); err != nil {
			return err
		}
		return insertCandidates(ctx, tx, id, labels, next)
	})
}

// DeleteCandidate そのイベントの候補を1件消します。出欠は外部キーの CASCADE で消えます。
func (s *Events) DeleteCandidate(ctx context.Context, id string, cid int64, now time.Time) error {
	return withTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := mustAffect(tx.ExecContext(ctx, "DELETE FROM candidates WHERE id = ? AND event_id = ?", cid, id)); err != nil {
			return err
		}
		return touch(ctx, tx, id, now)
	})
}

// Delete イベントを消します。候補、回答、出欠は外部キーの CASCADE で消えます。
func (s *Events) Delete(ctx context.Context, id string) error {
	return mustAffect(s.db.ExecContext(ctx, "DELETE FROM events WHERE id = ?", id))
}

func insertCandidates(ctx context.Context, tx *sql.Tx, id string, labels []string, start int) error {
	for i, l := range labels {
		if _, err := tx.ExecContext(ctx, "INSERT INTO candidates (event_id, position, label) VALUES (?, ?, ?)", id, start+i, l); err != nil {
			return err
		}
	}
	return nil
}
