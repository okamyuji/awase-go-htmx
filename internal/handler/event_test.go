package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/okamyuji/awase-go-htmx/internal/domain"
	"github.com/okamyuji/awase-go-htmx/internal/port"
)

var errBoom = errors.New("boom")

type fakeEvents struct {
	list    []domain.EventSummary
	view    port.EventView
	err     error
	created [3]string
	calls   []string
}

func (f *fakeEvents) List(context.Context) ([]domain.EventSummary, error) { return f.list, f.err }
func (f *fakeEvents) Create(_ context.Context, t, m, c string) (string, error) {
	f.created = [3]string{t, m, c}
	return "ev1", f.err
}
func (f *fakeEvents) View(_ context.Context, id string) (port.EventView, error) {
	if id != f.view.Event.ID {
		return port.EventView{}, domain.ErrNotFound
	}
	return f.view, nil
}
func (f *fakeEvents) UpdateMeta(_ context.Context, id, t, m string) error {
	f.calls = append(f.calls, "meta:"+id+":"+t)
	return f.err
}
func (f *fakeEvents) AddCandidates(_ context.Context, id, c string) error {
	f.calls = append(f.calls, "add:"+id+":"+c)
	return f.err
}
func (f *fakeEvents) DeleteCandidate(_ context.Context, id string, cid int64) error {
	f.calls = append(f.calls, "delcand:"+id)
	return f.err
}
func (f *fakeEvents) Delete(_ context.Context, id string) error {
	f.calls = append(f.calls, "del:"+id)
	return f.err
}

type fakeAnswers struct {
	answer  domain.Answer
	err     error
	choices map[int64]domain.Choice
	calls   []string
}

func (f *fakeAnswers) Create(_ context.Context, id, name, _ string, ch map[int64]domain.Choice) error {
	f.choices = ch
	f.calls = append(f.calls, "create:"+id+":"+name)
	return f.err
}
func (f *fakeAnswers) Get(_ context.Context, id string, aid int64) (domain.Answer, error) {
	if aid != f.answer.ID {
		return domain.Answer{}, domain.ErrNotFound
	}
	return f.answer, nil
}
func (f *fakeAnswers) Update(_ context.Context, id string, aid int64, name, _ string, ch map[int64]domain.Choice) error {
	f.choices = ch
	f.calls = append(f.calls, "update:"+id+":"+name)
	return f.err
}
func (f *fakeAnswers) Delete(_ context.Context, id string, _ int64) error {
	f.calls = append(f.calls, "delete:"+id)
	return f.err
}

var updated = time.Date(2026, 11, 1, 15, 30, 0, 0, time.UTC)

func sampleView() port.EventView {
	cands := []domain.Candidate{{ID: 11, Label: "11/3 <b>夜</b>"}, {ID: 12, Label: "11/4"}}
	answers := []domain.Answer{
		{ID: 7, EventID: "ev1", Name: "<script>x</script>", Comment: "よろしく", Choices: map[int64]domain.Choice{11: domain.Yes}},
	}
	return port.EventView{
		Event:   domain.Event{ID: "ev1", Title: "飲み会", Memo: "駅前", Candidates: cands, UpdatedAt: updated},
		Answers: answers,
		Tally:   domain.Tally(cands, answers),
	}
}

func newEventHandler(ev *fakeEvents, an *fakeAnswers) http.Handler {
	return New(Deps{Auth: &fakeAuth{}, Events: ev, Answers: an, Logger: slog.New(slog.DiscardHandler)})
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	return do(h, withSession(httptest.NewRequest(http.MethodGet, path, nil)))
}

func post(h http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	return do(h, withSession(postForm(path, form)))
}

func assertRedirect(t *testing.T, rec *httptest.ResponseRecorder, to string) {
	t.Helper()
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != to {
		t.Fatalf("status=%d location=%q, want 303 %s; body=%s", rec.Code, rec.Header().Get("Location"), to, rec.Body.String())
	}
}

func assertBody(t *testing.T, rec *httptest.ResponseRecorder, status int, parts ...string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, status, rec.Body.String())
	}
	for _, p := range parts {
		if !strings.Contains(rec.Body.String(), p) {
			t.Errorf("body lacks %q", p)
		}
	}
}

func TestEveryPrivateRouteRequiresLogin(t *testing.T) {
	h := newEventHandler(&fakeEvents{view: sampleView()}, &fakeAnswers{})
	routes := []struct{ method, path string }{
		{"GET", "/"}, {"GET", "/events/new"}, {"POST", "/events"}, {"GET", "/events/ev1"},
		{"GET", "/events/ev1/edit"}, {"POST", "/events/ev1"}, {"POST", "/events/ev1/candidates"},
		{"POST", "/events/ev1/candidates/11/delete"}, {"POST", "/events/ev1/delete"},
		{"POST", "/events/ev1/answers"}, {"GET", "/events/ev1/answers/7/edit"},
		{"POST", "/events/ev1/answers/7"}, {"POST", "/events/ev1/answers/7/delete"}, {"POST", "/logout"},
	}
	for _, r := range routes {
		rec := do(h, httptest.NewRequest(r.method, r.path, nil))
		want := "/login"
		if r.method == http.MethodGet {
			want += "?next=" + url.QueryEscape(r.path)
		}
		assertRedirect(t, rec, want)
	}
}

func TestIndexListsEventsWithJSTDate(t *testing.T) {
	ev := &fakeEvents{list: []domain.EventSummary{{ID: "ev1", Title: "飲み会", CandidateCount: 3, AnswerCount: 2, UpdatedAt: updated}}}

	rec := get(newEventHandler(ev, &fakeAnswers{}), "/")

	assertBody(t, rec, http.StatusOK, `href="/events/ev1"`, "飲み会", "候補 3 件", "回答 2 件", "2026/11/02 00:30")
}

func TestIndexShowsEmptyMessage(t *testing.T) {
	assertBody(t, get(newEventHandler(&fakeEvents{}, &fakeAnswers{}), "/"), http.StatusOK, "まだイベントがありません")
}

func TestCreateEventRedirectsAndRerendersOnInvalid(t *testing.T) {
	ev := &fakeEvents{}
	h := newEventHandler(ev, &fakeAnswers{})

	assertRedirect(t, post(h, "/events", url.Values{"title": {"会"}, "memo": {"m"}, "candidates": {"a\r\nb"}}), "/events/ev1")
	if ev.created != [3]string{"会", "m", "a\r\nb"} {
		t.Fatalf("created = %q", ev.created)
	}

	ev.err = domain.ValidationError{"title": "1〜100文字で入力してください"}
	rec := post(h, "/events", url.Values{"title": {"keep-me"}, "candidates": {"x"}})
	assertBody(t, rec, http.StatusUnprocessableEntity, "1〜100文字で入力してください", `value="keep-me"`)
}

func TestNewEventPage(t *testing.T) {
	assertBody(t, get(newEventHandler(&fakeEvents{}, &fakeAnswers{}), "/events/new"), http.StatusOK, `action="/events"`, `name="candidates"`)
}

func TestEventPageShowsTallyEscapedAndUnanswered(t *testing.T) {
	rec := get(newEventHandler(&fakeEvents{view: sampleView()}, &fakeAnswers{}), "/events/ev1")

	assertBody(t, rec, http.StatusOK,
		"飲み会", "駅前", `class="best"`, "&lt;script&gt;x&lt;/script&gt;", "11/3 &lt;b&gt;夜&lt;/b&gt;",
		"○", "−", `href="/events/ev1/answers/7/edit"`, "よろしく", `name="choice-11"`, `name="choice-12"`, "2026/11/02 00:30")
	if strings.Contains(rec.Body.String(), "<script>x") {
		t.Fatal("answer name is not escaped")
	}
}

func TestMissingEventIs404(t *testing.T) {
	h := newEventHandler(&fakeEvents{view: sampleView()}, &fakeAnswers{})
	for _, p := range []string{"/events/nope", "/events/nope/edit", "/events/ev1/answers/99/edit", "/events/ev1/answers/abc/edit"} {
		assertBody(t, get(h, p), http.StatusNotFound, "見つかりません")
	}
	assertBody(t, post(h, "/events/ev1/candidates/abc/delete", nil), http.StatusNotFound, "見つかりません")
}

func TestServiceErrorsAre500(t *testing.T) {
	h := newEventHandler(&fakeEvents{err: errBoom}, &fakeAnswers{})
	if rec := get(h, "/"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestEditEventFlows(t *testing.T) {
	ev := &fakeEvents{view: sampleView()}
	h := newEventHandler(ev, &fakeAnswers{})

	assertBody(t, get(h, "/events/ev1/edit"), http.StatusOK, `value="飲み会"`, `action="/events/ev1/candidates/11/delete"`)
	assertRedirect(t, post(h, "/events/ev1", url.Values{"title": {"新"}, "memo": {""}}), "/events/ev1")
	assertRedirect(t, post(h, "/events/ev1/candidates", url.Values{"candidates": {"x"}}), "/events/ev1/edit")
	assertRedirect(t, post(h, "/events/ev1/candidates/11/delete", nil), "/events/ev1/edit")
	assertRedirect(t, post(h, "/events/ev1/delete", nil), "/")
	want := []string{"meta:ev1:新", "add:ev1:x", "delcand:ev1", "del:ev1"}
	if strings.Join(ev.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("calls = %v", ev.calls)
	}
}

func TestEditEventValidationRerenders(t *testing.T) {
	ev := &fakeEvents{view: sampleView()}
	h := newEventHandler(ev, &fakeAnswers{})
	ev.err = domain.ValidationError{"candidates": "最後の候補は削除できません"}

	assertBody(t, post(h, "/events/ev1/candidates/11/delete", nil), http.StatusUnprocessableEntity, "最後の候補は削除できません")
	assertBody(t, post(h, "/events/ev1/candidates", url.Values{"candidates": {"keep-cand"}}), http.StatusUnprocessableEntity, "keep-cand")
	assertBody(t, post(h, "/events/ev1", url.Values{"title": {"keep-title"}}), http.StatusUnprocessableEntity, `value="keep-title"`)
}

func TestCreateAnswerPassesEveryChoiceField(t *testing.T) {
	an := &fakeAnswers{}
	h := newEventHandler(&fakeEvents{view: sampleView()}, an)

	rec := post(h, "/events/ev1/answers", url.Values{"name": {"太郎"}, "choice-11": {"2"}, "choice-12": {"0"}, "choice-99": {"1"}, "choice-x": {"1"}, "choice-13": {"9"}})

	assertRedirect(t, rec, "/events/ev1")
	if len(an.choices) != 3 || an.choices[11] != domain.Yes || an.choices[12] != domain.No || an.choices[99] != domain.Maybe {
		t.Fatalf("choices = %v", an.choices)
	}
}

func TestCreateAnswerInvalidRerendersEventWithInput(t *testing.T) {
	an := &fakeAnswers{err: domain.ValidationError{"choices": "すべての候補について出欠を選んでください"}}
	h := newEventHandler(&fakeEvents{view: sampleView()}, an)

	rec := post(h, "/events/ev1/answers", url.Values{"name": {"keep-name"}, "choice-11": {"1"}})

	assertBody(t, rec, http.StatusUnprocessableEntity, "すべての候補について出欠を選んでください", `value="keep-name"`, `value="1" checked`)
}

func TestEditAnswerShowsNewCandidateUnselected(t *testing.T) {
	an := &fakeAnswers{answer: domain.Answer{ID: 7, EventID: "ev1", Name: "花子", Choices: map[int64]domain.Choice{11: domain.Maybe}}}
	h := newEventHandler(&fakeEvents{view: sampleView()}, an)

	rec := get(h, "/events/ev1/answers/7/edit")

	assertBody(t, rec, http.StatusOK, `value="花子"`, `name="choice-11" value="1" checked`, `name="choice-12"`)
	if strings.Contains(rec.Body.String(), `name="choice-12" value="2" checked`) || strings.Contains(rec.Body.String(), `name="choice-12" value="1" checked`) || strings.Contains(rec.Body.String(), `name="choice-12" value="0" checked`) {
		t.Fatal("new candidate is preselected")
	}
}

func TestUpdateAndDeleteAnswer(t *testing.T) {
	an := &fakeAnswers{answer: domain.Answer{ID: 7, EventID: "ev1"}}
	h := newEventHandler(&fakeEvents{view: sampleView()}, an)

	assertRedirect(t, post(h, "/events/ev1/answers/7", url.Values{"name": {"x"}, "choice-11": {"2"}, "choice-12": {"2"}}), "/events/ev1")
	assertRedirect(t, post(h, "/events/ev1/answers/7/delete", nil), "/events/ev1")
	if strings.Join(an.calls, ",") != "update:ev1:x,delete:ev1" {
		t.Fatalf("calls = %v", an.calls)
	}

	an.err = domain.ValidationError{"name": "1〜30文字で入力してください"}
	assertBody(t, post(h, "/events/ev1/answers/7", url.Values{"name": {""}}), http.StatusUnprocessableEntity, "1〜30文字で入力してください")
	an.err = domain.ErrNotFound
	assertBody(t, post(h, "/events/ev1/answers/7/delete", nil), http.StatusNotFound, "見つかりません")
}

func TestEventPathEscapesSeparators(t *testing.T) {
	if got := eventPath("a/b?c"); got != "/events/a%2Fb%3Fc" {
		t.Fatalf("eventPath = %q", got)
	}
}
