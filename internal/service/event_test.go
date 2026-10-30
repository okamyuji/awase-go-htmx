package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/okamyuji/awase-go-htmx/internal/domain"
)

func newEventFixture() (*Events, *Answers, *memRepo) {
	m := newMemRepo()
	clk := &fixedClock{now: t0}
	return NewEvents(m, memAnswers{m}, &seqIDs{}, clk), NewAnswers(m, memAnswers{m}, clk), m
}

func mustCreate(t *testing.T, ev *Events, cands string) string {
	t.Helper()
	id, err := ev.Create(t.Context(), " 会 ", " メモ ", cands)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func assertField(t *testing.T, err error, field string) {
	t.Helper()
	ve, ok := errors.AsType[domain.ValidationError](err)
	if !ok {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if _, ok := ve[field]; !ok {
		t.Fatalf("err = %v, want field %s", ve, field)
	}
}

func TestEventsCreateStoresNormalizedEvent(t *testing.T) {
	ev, _, m := newEventFixture()

	id := mustCreate(t, ev, "a\r\nb")

	got := m.events[id]
	if id != "ev1" || got.Title != "会" || got.Memo != "メモ" || len(got.Candidates) != 2 || got.Candidates[1].Label != "b" || got.Candidates[1].Position != 1 || !got.CreatedAt.Equal(t0) || !got.UpdatedAt.Equal(t0) {
		t.Fatalf("event = %+v", got)
	}
}

func TestEventsCreateRejectsInvalidInput(t *testing.T) {
	ev, _, m := newEventFixture()

	_, err := ev.Create(t.Context(), strings.Repeat("a", 101), "", "x")

	assertField(t, err, "title")
	if len(m.events) != 0 {
		t.Fatal("invalid event stored")
	}
}

func TestEventsViewTalliesAnswers(t *testing.T) {
	ev, an, m := newEventFixture()
	id := mustCreate(t, ev, "a\nb")
	c := m.events[id].Candidates
	if err := an.Create(t.Context(), id, "x", "", map[int64]domain.Choice{c[0].ID: domain.Yes, c[1].ID: domain.No}); err != nil {
		t.Fatal(err)
	}

	v, err := ev.View(t.Context(), id)

	if err != nil || len(v.Answers) != 1 || !v.Tally[0].Best || v.Tally[1].No != 1 {
		t.Fatalf("view = %+v %v", v, err)
	}
	if _, err := ev.View(t.Context(), "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestEventsUpdateMetaValidates(t *testing.T) {
	ev, _, m := newEventFixture()
	id := mustCreate(t, ev, "a")

	assertField(t, ev.UpdateMeta(t.Context(), id, "", ""), "title")
	if err := ev.UpdateMeta(t.Context(), id, " 新 ", ""); err != nil || m.events[id].Title != "新" {
		t.Fatalf("update = %v %+v", err, m.events[id])
	}
}

func TestEventsAddCandidatesCountsExisting(t *testing.T) {
	ev, _, m := newEventFixture()
	id := mustCreate(t, ev, strings.Repeat("x\n", 49))

	assertField(t, ev.AddCandidates(t.Context(), id, "y\nz"), "candidates")
	if err := ev.AddCandidates(t.Context(), id, "y"); err != nil || len(m.events[id].Candidates) != 50 {
		t.Fatalf("add = %v, count %d", err, len(m.events[id].Candidates))
	}
	if err := ev.AddCandidates(t.Context(), "nope", "y"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestEventsDeleteCandidateRules(t *testing.T) {
	ev, _, m := newEventFixture()
	id := mustCreate(t, ev, "a\nb")
	other := mustCreate(t, ev, "z")
	first, second := m.events[id].Candidates[0].ID, m.events[id].Candidates[1].ID

	if err := ev.DeleteCandidate(t.Context(), id, m.events[other].Candidates[0].ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other event's candidate: %v", err)
	}
	if err := ev.DeleteCandidate(t.Context(), id, first); err != nil {
		t.Fatal(err)
	}
	assertField(t, ev.DeleteCandidate(t.Context(), id, second), "candidates")
	if err := ev.DeleteCandidate(t.Context(), "nope", 1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing event: %v", err)
	}
}

func TestEventsListAndDelete(t *testing.T) {
	ev, _, _ := newEventFixture()
	id := mustCreate(t, ev, "a")

	if list, err := ev.List(t.Context()); err != nil || len(list) != 1 {
		t.Fatalf("list = %v %v", list, err)
	}
	if err := ev.Delete(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if err := ev.Delete(t.Context(), id); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}
