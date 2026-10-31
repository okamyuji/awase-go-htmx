package store

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/okamyuji/awase-go-htmx/internal/domain"
	"github.com/okamyuji/awase-go-htmx/internal/port"
)

var t0 = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(t.Context(), filepath.Join(t.TempDir(), "awase.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

func seedEvent(t *testing.T, ev *Events, id string, at time.Time, labels ...string) domain.Event {
	t.Helper()
	e := domain.Event{ID: id, Title: "t-" + id, Memo: "m", CreatedAt: at, UpdatedAt: at}
	for i, l := range labels {
		e.Candidates = append(e.Candidates, domain.Candidate{Position: i, Label: l})
	}
	if err := ev.Create(t.Context(), e); err != nil {
		t.Fatal(err)
	}
	got, err := ev.Get(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestOpenAppliesSchemaIdempotently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "awase.db")
	for range 2 {
		db, err := Open(t.Context(), path)
		if err != nil {
			t.Fatal(err)
		}
		var v int
		if err := db.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&v); err != nil {
			t.Fatal(err)
		}
		if v != 1 {
			t.Fatalf("user_version = %d, want 1", v)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMigrateFailsWhenSchemaCannotBeWritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "awase.db")
	must(t, os.WriteFile(path, nil, 0o600))
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	must(t, err)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})

	if err := migrate(t.Context(), db); err == nil {
		t.Fatal("migrate succeeded on a read-only database, want error")
	}
}

func TestOpenFailsOnUnwritablePath(t *testing.T) {
	if _, err := Open(t.Context(), filepath.Join(t.TempDir(), "missing", "dir", "awase.db")); err == nil {
		t.Fatal("Open succeeded, want error")
	}
}

func TestEventsCreateGetKeepsCandidateOrder(t *testing.T) {
	ev := NewEvents(newDB(t))

	got := seedEvent(t, ev, "e1", t0, "11/3", "11/4", "11/5")

	if got.Title != "t-e1" || got.Memo != "m" || !got.CreatedAt.Equal(t0) || len(got.Candidates) != 3 {
		t.Fatalf("got %+v", got)
	}
	if got.Candidates[0].Label != "11/3" || got.Candidates[2].Label != "11/5" || got.Candidates[0].ID == 0 {
		t.Fatalf("candidates = %+v", got.Candidates)
	}
}

func TestEventsGetMissingIsNotFound(t *testing.T) {
	_, err := NewEvents(newDB(t)).Get(t.Context(), "nope")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestEventsListNewestFirstWithCounts(t *testing.T) {
	db := newDB(t)
	ev, an := NewEvents(db), NewAnswers(db)
	old := seedEvent(t, ev, "old", t0, "a")
	seedEvent(t, ev, "new", t0.Add(time.Hour), "a", "b")
	_, err := an.Create(t.Context(), domain.Answer{EventID: "old", Name: "x", Choices: map[int64]domain.Choice{old.Candidates[0].ID: domain.Yes}, CreatedAt: t0, UpdatedAt: t0})
	must(t, err)

	got, err := ev.List(t.Context())

	must(t, err)
	want := []domain.EventSummary{
		{ID: "new", Title: "t-new", CandidateCount: 2, AnswerCount: 0, CreatedAt: t0.Add(time.Hour), UpdatedAt: t0.Add(time.Hour)},
		{ID: "old", Title: "t-old", CandidateCount: 1, AnswerCount: 1, CreatedAt: t0, UpdatedAt: t0},
	}
	if !slices.EqualFunc(got, want, sameSummary) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func sameSummary(a, b domain.EventSummary) bool {
	return a.ID == b.ID && a.Title == b.Title && a.CandidateCount == b.CandidateCount && a.AnswerCount == b.AnswerCount &&
		a.CreatedAt.Equal(b.CreatedAt) && a.UpdatedAt.Equal(b.UpdatedAt)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustNotFound(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestEventsUpdateMetaAndAddCandidates(t *testing.T) {
	ev := NewEvents(newDB(t))
	seedEvent(t, ev, "e1", t0, "a")
	later := t0.Add(time.Minute)

	if err := ev.UpdateMeta(t.Context(), "e1", "新", "メ", later); err != nil {
		t.Fatal(err)
	}
	if err := ev.AddCandidates(t.Context(), "e1", []string{"b", "c"}, later); err != nil {
		t.Fatal(err)
	}
	got, err := ev.Get(t.Context(), "e1")

	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "新" || got.Memo != "メ" || !got.UpdatedAt.Equal(later) || len(got.Candidates) != 3 || got.Candidates[2].Label != "c" {
		t.Fatalf("got %+v", got)
	}
}

func TestEventsMutationsOnMissingEventAreNotFound(t *testing.T) {
	ev := NewEvents(newDB(t))
	for name, err := range map[string]error{
		"UpdateMeta":    ev.UpdateMeta(t.Context(), "nope", "t", "", t0),
		"AddCandidates": ev.AddCandidates(t.Context(), "nope", []string{"a"}, t0),
		"Delete":        ev.Delete(t.Context(), "nope"),
	} {
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
}

func TestEventsDeleteCandidateRemovesChoicesAndRejectsOtherEvent(t *testing.T) {
	db := newDB(t)
	ev, an := NewEvents(db), NewAnswers(db)
	a := seedEvent(t, ev, "a", t0, "a1", "a2")
	b := seedEvent(t, ev, "b", t0, "b1")
	aid, err := an.Create(t.Context(), domain.Answer{EventID: "a", Name: "x", Choices: map[int64]domain.Choice{a.Candidates[0].ID: domain.Yes, a.Candidates[1].ID: domain.No}, CreatedAt: t0, UpdatedAt: t0})
	if err != nil {
		t.Fatal(err)
	}

	if err := ev.DeleteCandidate(t.Context(), "a", b.Candidates[0].ID, t0); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other event's candidate: err = %v, want ErrNotFound", err)
	}
	if err := ev.DeleteCandidate(t.Context(), "a", a.Candidates[0].ID, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, err := an.Get(t.Context(), "a", aid)

	if err != nil {
		t.Fatal(err)
	}
	if len(got.Choices) != 1 || got.Choices[a.Candidates[1].ID] != domain.No {
		t.Fatalf("choices = %v", got.Choices)
	}
	e, err := ev.Get(t.Context(), "a")
	if err != nil || len(e.Candidates) != 1 || !e.UpdatedAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("event = %+v %v", e, err)
	}
}

func TestEventsDeleteCascades(t *testing.T) {
	db := newDB(t)
	ev, an := NewEvents(db), NewAnswers(db)
	e := seedEvent(t, ev, "e1", t0, "a")
	aid, err := an.Create(t.Context(), domain.Answer{EventID: "e1", Name: "x", Choices: map[int64]domain.Choice{e.Candidates[0].ID: domain.Yes}, CreatedAt: t0, UpdatedAt: t0})
	if err != nil {
		t.Fatal(err)
	}

	if err := ev.Delete(t.Context(), "e1"); err != nil {
		t.Fatal(err)
	}

	if _, err := an.Get(t.Context(), "e1", aid); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("answer after delete: %v", err)
	}
	var n int
	if err := db.QueryRowContext(t.Context(), "SELECT count(*) FROM choices").Scan(&n); err != nil || n != 0 {
		t.Fatalf("choices left = %d %v", n, err)
	}
}

type answerFixture struct {
	ev     *Events
	an     *Answers
	c0, c1 int64
	aid    int64
}

var later = t0.Add(time.Hour)

func newAnswerFixture(t *testing.T) answerFixture {
	t.Helper()
	db := newDB(t)
	f := answerFixture{ev: NewEvents(db), an: NewAnswers(db)}
	e := seedEvent(t, f.ev, "e1", t0, "a", "b")
	seedEvent(t, f.ev, "e2", t0, "z")
	f.c0, f.c1 = e.Candidates[0].ID, e.Candidates[1].ID
	aid, err := f.an.Create(t.Context(), domain.Answer{EventID: "e1", Name: "太郎", Comment: "c", Choices: map[int64]domain.Choice{f.c0: domain.Yes, f.c1: domain.Maybe}, CreatedAt: later, UpdatedAt: later})
	must(t, err)
	f.aid = aid
	return f
}

func TestAnswersCreateStoresChoicesAndTouchesEvent(t *testing.T) {
	f := newAnswerFixture(t)

	got, err := f.an.Get(t.Context(), "e1", f.aid)

	must(t, err)
	if got.Name != "太郎" || got.Comment != "c" || got.Choices[f.c0] != domain.Yes || got.Choices[f.c1] != domain.Maybe || !got.CreatedAt.Equal(later) {
		t.Fatalf("got %+v", got)
	}
	e, err := f.ev.Get(t.Context(), "e1")
	must(t, err)
	if !e.UpdatedAt.Equal(later) {
		t.Fatalf("event updated_at = %v, want %v", e.UpdatedAt, later)
	}
}

func TestAnswersAreScopedToTheirEvent(t *testing.T) {
	f := newAnswerFixture(t)

	_, err := f.an.Get(t.Context(), "e2", f.aid)
	mustNotFound(t, err)
	mustNotFound(t, f.an.Update(t.Context(), domain.Answer{ID: f.aid, EventID: "e2", Name: "x", Choices: map[int64]domain.Choice{}}))
	mustNotFound(t, f.an.Delete(t.Context(), "e2", f.aid, later))
}

func TestAnswersUpdateReplacesFields(t *testing.T) {
	f := newAnswerFixture(t)
	upd := domain.Answer{ID: f.aid, EventID: "e1", Name: "花子", Choices: map[int64]domain.Choice{f.c0: domain.No, f.c1: domain.Yes}, UpdatedAt: later.Add(time.Hour)}

	must(t, f.an.Update(t.Context(), upd))

	list, err := f.an.ListByEvent(t.Context(), "e1")
	must(t, err)
	if len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}
	got := list[0]
	if got.Name != "花子" || got.Comment != "" || got.Choices[f.c0] != domain.No || got.Choices[f.c1] != domain.Yes || !got.UpdatedAt.Equal(later.Add(time.Hour)) {
		t.Fatalf("got %+v", got)
	}
	e, err := f.ev.Get(t.Context(), "e1")
	must(t, err)
	if !e.UpdatedAt.Equal(later.Add(time.Hour)) {
		t.Fatalf("event updated_at = %v, want %v", e.UpdatedAt, later.Add(time.Hour))
	}
}

func TestAnswersDeleteRemovesAnswer(t *testing.T) {
	f := newAnswerFixture(t)

	must(t, f.an.Delete(t.Context(), "e1", f.aid, later))

	list, err := f.an.ListByEvent(t.Context(), "e1")
	must(t, err)
	if len(list) != 0 {
		t.Fatalf("after delete = %+v", list)
	}
}

func TestAnswersCreateOnMissingEventIsNotFound(t *testing.T) {
	_, err := NewAnswers(newDB(t)).Create(t.Context(), domain.Answer{EventID: "nope", Name: "x", Choices: map[int64]domain.Choice{}})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestAccountGetUnsetIsNotFoundAndSetBumpsVersion(t *testing.T) {
	acc := NewAccount(newDB(t))

	if _, err := acc.Get(t.Context()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unset: %v", err)
	}
	v1, err := acc.Set(t.Context(), "demo", "h1", t0)
	if err != nil || v1 != 1 {
		t.Fatalf("first set = %d %v", v1, err)
	}
	v2, err := acc.Set(t.Context(), "demo2", "h2", t0)
	if err != nil || v2 != 2 {
		t.Fatalf("second set = %d %v", v2, err)
	}
	got, err := acc.Get(t.Context())
	if err != nil || got != (port.Account{Username: "demo2", PasswordHash: "h2", Version: 2}) {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestSessionsValidRequiresCurrentVersionAndExpiry(t *testing.T) {
	db := newDB(t)
	acc, ss := NewAccount(db), NewSessions(db)
	if _, err := acc.Set(t.Context(), "demo", "h", t0); err != nil {
		t.Fatal(err)
	}
	tok := []byte("token-hash-1")
	if err := ss.Create(t.Context(), tok, 1, t0, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	assertValid(t, ss, tok, t0.Add(59*time.Minute), true)
	assertValid(t, ss, tok, t0.Add(time.Hour), false)
	assertValid(t, ss, []byte("unknown"), t0, false)

	if _, err := db.ExecContext(t.Context(), "UPDATE account SET version = 2"); err != nil {
		t.Fatal(err)
	}
	assertValid(t, ss, tok, t0, false)
}

func TestSessionsAreClearedBySetAndDelete(t *testing.T) {
	db := newDB(t)
	acc, ss := NewAccount(db), NewSessions(db)
	if _, err := acc.Set(t.Context(), "demo", "h", t0); err != nil {
		t.Fatal(err)
	}
	if err := ss.Create(t.Context(), []byte("a"), 1, t0, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := ss.Create(t.Context(), []byte("b"), 1, t0, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	if err := ss.Delete(t.Context(), []byte("a")); err != nil {
		t.Fatal(err)
	}
	assertValid(t, ss, []byte("a"), t0, false)
	if err := ss.DeleteExpired(t.Context(), t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertSessionCount(t, db, 0)

	if err := ss.Create(t.Context(), []byte("c"), 1, t0, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := acc.Set(t.Context(), "demo", "h2", t0); err != nil {
		t.Fatal(err)
	}
	assertSessionCount(t, db, 0)
}

func assertValid(t *testing.T, ss *Sessions, tok []byte, now time.Time, want bool) {
	t.Helper()
	got, err := ss.Valid(t.Context(), tok, now)
	if err != nil || got != want {
		t.Fatalf("Valid(%s, %v) = %v %v, want %v", tok, now, got, err, want)
	}
}

func assertSessionCount(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var n int
	if err := db.QueryRowContext(t.Context(), "SELECT count(*) FROM sessions").Scan(&n); err != nil || n != want {
		t.Fatalf("sessions = %d %v, want %d", n, err, want)
	}
}
