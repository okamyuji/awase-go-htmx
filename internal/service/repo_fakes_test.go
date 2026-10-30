package service

import (
	"context"
	"maps"
	"slices"
	"time"

	"github.com/okamyuji/awase-go-htmx/internal/domain"
)

type memRepo struct {
	events  map[string]domain.Event
	answers map[int64]domain.Answer
	nextID  int64
}

func newMemRepo() *memRepo {
	return &memRepo{events: map[string]domain.Event{}, answers: map[int64]domain.Answer{}}
}

func (m *memRepo) id() int64 { m.nextID++; return m.nextID }

func (m *memRepo) Create(_ context.Context, ev domain.Event) error {
	for i := range ev.Candidates {
		ev.Candidates[i].ID = m.id()
	}
	m.events[ev.ID] = ev
	return nil
}
func (m *memRepo) Get(_ context.Context, id string) (domain.Event, error) {
	ev, ok := m.events[id]
	if !ok {
		return domain.Event{}, domain.ErrNotFound
	}
	return ev, nil
}
func (m *memRepo) List(context.Context) ([]domain.EventSummary, error) {
	out := []domain.EventSummary{}
	for _, id := range slices.Sorted(maps.Keys(m.events)) {
		out = append(out, domain.EventSummary{ID: id})
	}
	return out, nil
}
func (m *memRepo) UpdateMeta(_ context.Context, id, title, memo string, now time.Time) error {
	ev, ok := m.events[id]
	if !ok {
		return domain.ErrNotFound
	}
	ev.Title, ev.Memo, ev.UpdatedAt = title, memo, now
	m.events[id] = ev
	return nil
}
func (m *memRepo) AddCandidates(_ context.Context, id string, labels []string, _ time.Time) error {
	ev := m.events[id]
	for _, l := range labels {
		ev.Candidates = append(ev.Candidates, domain.Candidate{ID: m.id(), Label: l})
	}
	m.events[id] = ev
	return nil
}
func (m *memRepo) DeleteCandidate(_ context.Context, id string, cid int64, _ time.Time) error {
	ev := m.events[id]
	ev.Candidates = slices.DeleteFunc(ev.Candidates, func(c domain.Candidate) bool { return c.ID == cid })
	m.events[id] = ev
	return nil
}
func (m *memRepo) Delete(_ context.Context, id string) error {
	if _, ok := m.events[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.events, id)
	return nil
}

type memAnswers struct{ m *memRepo }

func (a memAnswers) Create(_ context.Context, ans domain.Answer) (int64, error) {
	ans.ID = a.m.id()
	a.m.answers[ans.ID] = ans
	return ans.ID, nil
}
func (a memAnswers) Get(_ context.Context, eventID string, aid int64) (domain.Answer, error) {
	ans, ok := a.m.answers[aid]
	if !ok || ans.EventID != eventID {
		return domain.Answer{}, domain.ErrNotFound
	}
	return ans, nil
}
func (a memAnswers) ListByEvent(_ context.Context, eventID string) ([]domain.Answer, error) {
	var out []domain.Answer
	for _, id := range slices.Sorted(maps.Keys(a.m.answers)) {
		if ans := a.m.answers[id]; ans.EventID == eventID {
			out = append(out, ans)
		}
	}
	return out, nil
}
func (a memAnswers) Update(_ context.Context, ans domain.Answer) error {
	old, ok := a.m.answers[ans.ID]
	if !ok || old.EventID != ans.EventID {
		return domain.ErrNotFound
	}
	ans.CreatedAt = old.CreatedAt
	a.m.answers[ans.ID] = ans
	return nil
}
func (a memAnswers) Delete(_ context.Context, eventID string, aid int64, _ time.Time) error {
	if ans, ok := a.m.answers[aid]; !ok || ans.EventID != eventID {
		return domain.ErrNotFound
	}
	delete(a.m.answers, aid)
	return nil
}

type seqIDs struct{ n int }

func (s *seqIDs) NewID() string { s.n++; return "ev" + string(rune('0'+s.n)) }
