package port

import (
	"context"

	"github.com/okamyuji/awase-go-htmx/internal/domain"
)

// EventView イベントの表示に必要な値です。
type EventView struct {
	Event   domain.Event
	Answers []domain.Answer
	Tally   []domain.CandidateTally
}

// EventService イベントと候補のユースケースです。入力の違反は domain.ValidationError、存在しない ID は domain.ErrNotFound です。
type EventService interface {
	List(ctx context.Context) ([]domain.EventSummary, error)
	Create(ctx context.Context, title, memo, candidates string) (string, error)
	View(ctx context.Context, id string) (EventView, error)
	UpdateMeta(ctx context.Context, id, title, memo string) error
	AddCandidates(ctx context.Context, id, candidates string) error
	DeleteCandidate(ctx context.Context, id string, cid int64) error
	Delete(ctx context.Context, id string) error
}

// AnswerService 回答のユースケースです。エラーの種類は EventService と同じです。
type AnswerService interface {
	Create(ctx context.Context, eventID, name, comment string, choices map[int64]domain.Choice) error
	Get(ctx context.Context, eventID string, aid int64) (domain.Answer, error)
	Update(ctx context.Context, eventID string, aid int64, name, comment string, choices map[int64]domain.Choice) error
	Delete(ctx context.Context, eventID string, aid int64) error
}
