package service

import (
	"errors"
	"testing"

	"github.com/okamyuji/awase-go-htmx/internal/domain"
)

func TestAnswersRequireExactCandidateSet(t *testing.T) {
	ev, an, m := newEventFixture()
	id := mustCreate(t, ev, "a\nb")
	other := mustCreate(t, ev, "z")
	c := m.events[id].Candidates
	foreign := m.events[other].Candidates[0].ID

	assertField(t, an.Create(t.Context(), id, "x", "", map[int64]domain.Choice{c[0].ID: domain.Yes, foreign: domain.No}), "choices")
	assertField(t, an.Create(t.Context(), id, "x", "", map[int64]domain.Choice{c[0].ID: domain.Yes}), "choices")
	if len(m.answers) != 0 {
		t.Fatal("invalid answer stored")
	}
	if err := an.Create(t.Context(), "nope", "x", "", nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing event: %v", err)
	}
}

func newAnswered(t *testing.T) (*Answers, *memRepo, string, int64, int64) {
	t.Helper()
	ev, an, m := newEventFixture()
	id := mustCreate(t, ev, "a")
	cid := m.events[id].Candidates[0].ID
	if err := an.Create(t.Context(), id, " 太郎 ", " c ", map[int64]domain.Choice{cid: domain.Yes}); err != nil {
		t.Fatal(err)
	}
	return an, m, id, cid, m.nextID
}

func TestAnswersCreateStoresTrimmedValues(t *testing.T) {
	an, _, id, _, aid := newAnswered(t)

	got, err := an.Get(t.Context(), id, aid)

	if err != nil || got.Name != "太郎" || got.Comment != "c" || !got.CreatedAt.Equal(t0) {
		t.Fatalf("get = %+v %v", got, err)
	}
}

func TestAnswersUpdateValidatesAndReplaces(t *testing.T) {
	an, m, id, cid, aid := newAnswered(t)

	assertField(t, an.Update(t.Context(), id, aid, "", "", map[int64]domain.Choice{cid: domain.No}), "name")
	if err := an.Update(t.Context(), id, aid, "花子", "", map[int64]domain.Choice{cid: domain.No}); err != nil {
		t.Fatal(err)
	}
	if got := m.answers[aid]; got.Name != "花子" || got.Choices[cid] != domain.No || !got.UpdatedAt.Equal(t0) {
		t.Fatalf("updated = %+v", got)
	}
	if err := an.Update(t.Context(), "nope", aid, "x", "", nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("update missing event: %v", err)
	}
}

func TestAnswersDeleteRemoves(t *testing.T) {
	an, _, id, _, aid := newAnswered(t)

	if err := an.Delete(t.Context(), id, aid); err != nil {
		t.Fatal(err)
	}

	if _, err := an.Get(t.Context(), id, aid); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}
