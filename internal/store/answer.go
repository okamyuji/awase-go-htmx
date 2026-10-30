package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/okamyuji/awase-go-htmx/internal/domain"
)

// Answers port.AnswerRepository の SQLite 実装です。
type Answers struct{ db *sql.DB }

// NewAnswers db を使う Answers を返します。
func NewAnswers(db *sql.DB) *Answers { return &Answers{db: db} }

// Create 回答と出欠を保存し、イベントの更新日時を回答の作成日時にします。
func (s *Answers) Create(ctx context.Context, a domain.Answer) (aid int64, err error) {
	err = withTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := touch(ctx, tx, a.EventID, a.CreatedAt); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, "INSERT INTO answers (event_id, name, comment, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			a.EventID, a.Name, a.Comment, a.CreatedAt.Unix(), a.UpdatedAt.Unix())
		if err != nil {
			return err
		}
		if aid, err = res.LastInsertId(); err != nil {
			return err
		}
		return insertChoices(ctx, tx, aid, a.Choices)
	})
	return aid, err
}

// Get そのイベントの回答を返します。
func (s *Answers) Get(ctx context.Context, eventID string, aid int64) (domain.Answer, error) {
	a := domain.Answer{ID: aid, EventID: eventID}
	var created, updated int64
	err := s.db.QueryRowContext(ctx, "SELECT name, comment, created_at, updated_at FROM answers WHERE id = ? AND event_id = ?", aid, eventID).
		Scan(&a.Name, &a.Comment, &created, &updated)
	if err != nil {
		return domain.Answer{}, notFoundOn(err)
	}
	a.CreatedAt, a.UpdatedAt = unix(created), unix(updated)
	byAnswer, err := s.choices(ctx, "SELECT answer_id, candidate_id, value FROM choices WHERE answer_id = ?", aid)
	a.Choices = byAnswer[aid]
	return a, err
}

// ListByEvent イベントの回答を作成順に返します。
func (s *Answers) ListByEvent(ctx context.Context, eventID string) (out []domain.Answer, err error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, comment, created_at, updated_at FROM answers WHERE event_id = ? ORDER BY id", eventID)
	if err != nil {
		return nil, err
	}
	defer closeInto(rows, &err)
	for rows.Next() {
		a := domain.Answer{EventID: eventID}
		var created, updated int64
		if err := rows.Scan(&a.ID, &a.Name, &a.Comment, &created, &updated); err != nil {
			return nil, err
		}
		a.CreatedAt, a.UpdatedAt = unix(created), unix(updated)
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	byAnswer, err := s.choices(ctx, `SELECT ch.answer_id, ch.candidate_id, ch.value FROM choices ch
		JOIN answers a ON a.id = ch.answer_id WHERE a.event_id = ?`, eventID)
	for i := range out {
		out[i].Choices = byAnswer[out[i].ID]
	}
	return out, err
}

func (s *Answers) choices(ctx context.Context, query string, arg any) (out map[int64]map[int64]domain.Choice, err error) {
	rows, err := s.db.QueryContext(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	defer closeInto(rows, &err)
	out = map[int64]map[int64]domain.Choice{}
	for rows.Next() {
		var aid, cid int64
		var v domain.Choice
		if err := rows.Scan(&aid, &cid, &v); err != nil {
			return nil, err
		}
		if out[aid] == nil {
			out[aid] = map[int64]domain.Choice{}
		}
		out[aid][cid] = v
	}
	return out, rows.Err()
}

// Update 回答の名前、コメント、出欠を置き換えます。
func (s *Answers) Update(ctx context.Context, a domain.Answer) error {
	return withTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := mustAffect(tx.ExecContext(ctx, "UPDATE answers SET name = ?, comment = ?, updated_at = ? WHERE id = ? AND event_id = ?",
			a.Name, a.Comment, a.UpdatedAt.Unix(), a.ID, a.EventID)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM choices WHERE answer_id = ?", a.ID); err != nil {
			return err
		}
		if err := insertChoices(ctx, tx, a.ID, a.Choices); err != nil {
			return err
		}
		return touch(ctx, tx, a.EventID, a.UpdatedAt)
	})
}

// Delete そのイベントの回答を消します。
func (s *Answers) Delete(ctx context.Context, eventID string, aid int64, now time.Time) error {
	return withTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := mustAffect(tx.ExecContext(ctx, "DELETE FROM answers WHERE id = ? AND event_id = ?", aid, eventID)); err != nil {
			return err
		}
		return touch(ctx, tx, eventID, now)
	})
}

func insertChoices(ctx context.Context, tx *sql.Tx, aid int64, choices map[int64]domain.Choice) error {
	for cid, v := range choices {
		if _, err := tx.ExecContext(ctx, "INSERT INTO choices (answer_id, candidate_id, value) VALUES (?, ?, ?)", aid, cid, v); err != nil {
			return err
		}
	}
	return nil
}
