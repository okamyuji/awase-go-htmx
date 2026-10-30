package service

import (
	"context"
	"slices"

	"github.com/okamyuji/awase-go-htmx/internal/domain"
	"github.com/okamyuji/awase-go-htmx/internal/port"
)

// Events port.EventService の実装です。
type Events struct {
	repo    port.EventRepository
	answers port.AnswerRepository
	ids     port.IDGen
	clock   port.Clock
}

// NewEvents 依存を受け取って Events を返します。
func NewEvents(r port.EventRepository, a port.AnswerRepository, ids port.IDGen, c port.Clock) *Events {
	return &Events{repo: r, answers: a, ids: ids, clock: c}
}

// List イベントを作成日時の新しい順に返します。
func (s *Events) List(ctx context.Context) ([]domain.EventSummary, error) {
	return s.repo.List(ctx)
}

// Create 入力を検証してイベントを作り、その ID を返します。
func (s *Events) Create(ctx context.Context, title, memo, candidates string) (string, error) {
	d, err := domain.NewEventDraft(title, memo, candidates)
	if err != nil {
		return "", err
	}
	now := s.clock.Now()
	ev := domain.Event{ID: s.ids.NewID(), Title: d.Title, Memo: d.Memo, CreatedAt: now, UpdatedAt: now}
	for i, l := range d.Labels {
		ev.Candidates = append(ev.Candidates, domain.Candidate{Position: i, Label: l})
	}
	return ev.ID, s.repo.Create(ctx, ev)
}

// View イベントと回答と集計を返します。
func (s *Events) View(ctx context.Context, id string) (port.EventView, error) {
	ev, err := s.repo.Get(ctx, id)
	if err != nil {
		return port.EventView{}, err
	}
	answers, err := s.answers.ListByEvent(ctx, id)
	if err != nil {
		return port.EventView{}, err
	}
	return port.EventView{Event: ev, Answers: answers, Tally: domain.Tally(ev.Candidates, answers)}, nil
}

// UpdateMeta タイトルとメモを検証して変えます。
func (s *Events) UpdateMeta(ctx context.Context, id, title, memo string) error {
	t, m, err := domain.NormalizeMeta(title, memo)
	if err != nil {
		return err
	}
	return s.repo.UpdateMeta(ctx, id, t, m, s.clock.Now())
}

// AddCandidates 既存の候補と合わせて上限を超えないことを確かめてから、候補を足します。
func (s *Events) AddCandidates(ctx context.Context, id, candidates string) error {
	ev, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	labels, err := domain.NewCandidateLabels(candidates, len(ev.Candidates))
	if err != nil {
		return err
	}
	return s.repo.AddCandidates(ctx, id, labels, s.clock.Now())
}

// DeleteCandidate そのイベントの候補を消します。最後の1件は消せません。
func (s *Events) DeleteCandidate(ctx context.Context, id string, cid int64) error {
	ev, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(ev.Candidates, func(c domain.Candidate) bool { return c.ID == cid }) {
		return domain.ErrNotFound
	}
	if err := domain.CheckCandidateDeletable(len(ev.Candidates)); err != nil {
		return err
	}
	return s.repo.DeleteCandidate(ctx, id, cid, s.clock.Now())
}

// Delete イベントを消します。
func (s *Events) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
