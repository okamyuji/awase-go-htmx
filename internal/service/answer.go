package service

import (
	"context"

	"github.com/okamyuji/awase-go-htmx/internal/domain"
	"github.com/okamyuji/awase-go-htmx/internal/port"
)

// Answers port.AnswerService の実装です。
type Answers struct {
	events port.EventRepository
	repo   port.AnswerRepository
	clock  port.Clock
}

// NewAnswers 依存を受け取って Answers を返します。
func NewAnswers(e port.EventRepository, r port.AnswerRepository, c port.Clock) *Answers {
	return &Answers{events: e, repo: r, clock: c}
}

// draft 回答を検証します。送られた候補の集合がイベントの候補の集合と等しくなければ違反です。
func (s *Answers) draft(ctx context.Context, eventID, name, comment string, choices map[int64]domain.Choice) (domain.AnswerDraft, error) {
	ev, err := s.events.Get(ctx, eventID)
	if err != nil {
		return domain.AnswerDraft{}, err
	}
	ids := make([]int64, len(ev.Candidates))
	for i, c := range ev.Candidates {
		ids[i] = c.ID
	}
	return domain.NewAnswerDraft(name, comment, choices, ids)
}

// Create 回答を検証して保存します。
func (s *Answers) Create(ctx context.Context, eventID, name, comment string, choices map[int64]domain.Choice) error {
	d, err := s.draft(ctx, eventID, name, comment, choices)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	_, err = s.repo.Create(ctx, domain.Answer{EventID: eventID, Name: d.Name, Comment: d.Comment, Choices: d.Choices, CreatedAt: now, UpdatedAt: now})
	return err
}

// Get そのイベントの回答を返します。
func (s *Answers) Get(ctx context.Context, eventID string, aid int64) (domain.Answer, error) {
	return s.repo.Get(ctx, eventID, aid)
}

// Update 回答を検証して置き換えます。
func (s *Answers) Update(ctx context.Context, eventID string, aid int64, name, comment string, choices map[int64]domain.Choice) error {
	d, err := s.draft(ctx, eventID, name, comment, choices)
	if err != nil {
		return err
	}
	return s.repo.Update(ctx, domain.Answer{ID: aid, EventID: eventID, Name: d.Name, Comment: d.Comment, Choices: d.Choices, UpdatedAt: s.clock.Now()})
}

// Delete 回答を消します。
func (s *Answers) Delete(ctx context.Context, eventID string, aid int64) error {
	return s.repo.Delete(ctx, eventID, aid, s.clock.Now())
}
