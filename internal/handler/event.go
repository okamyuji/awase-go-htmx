package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/okamyuji/awase-go-htmx/internal/domain"
	"github.com/okamyuji/awase-go-htmx/internal/port"
)

type indexData struct{ Events []domain.EventSummary }

type eventFormData struct {
	Title, Memo, Candidates string
	Errors                  domain.ValidationError
}

type eventPageData struct {
	View       port.EventView
	AnswerForm answerFormView
}

type editData struct {
	Event         domain.Event
	Title, Memo   string
	NewCandidates string
	Errors        domain.ValidationError
}

// fail 存在しない ID は 404 の画面、それ以外は 500 にします。
func (a *app) fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, domain.ErrNotFound) {
		a.render(w, r, http.StatusNotFound, "notfound", "見つかりません", nil)
		return
	}
	a.serverError(w, r, err)
}

// eventPath イベントの画面のパスです。ID は経路から取り出した値なので、パスの区切りを含まないよう符号化します。
func eventPath(id string) string {
	return "/events/" + url.PathEscape(id)
}

func pathInt(r *http.Request, name string) (int64, bool) {
	n, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return n, err == nil
}

func (a *app) index(w http.ResponseWriter, r *http.Request) {
	list, err := a.events.List(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "index", "イベント", indexData{Events: list})
}

func (a *app) newEvent(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, "event_new", "イベントを作る", eventFormData{})
}

func (a *app) createEvent(w http.ResponseWriter, r *http.Request) {
	d := eventFormData{Title: r.PostFormValue("title"), Memo: r.PostFormValue("memo"), Candidates: r.PostFormValue("candidates")}
	id, err := a.events.Create(r.Context(), d.Title, d.Memo, d.Candidates)
	if ve, ok := errors.AsType[domain.ValidationError](err); ok {
		d.Errors = ve
		a.render(w, r, http.StatusUnprocessableEntity, "event_new", "イベントを作る", d)
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	http.Redirect(w, r, eventPath(id), http.StatusSeeOther)
}

func (a *app) showEvent(w http.ResponseWriter, r *http.Request) {
	a.renderEvent(w, r, http.StatusOK, answerForm{}, nil)
}

func (a *app) renderEvent(w http.ResponseWriter, r *http.Request, status int, form answerForm, errs domain.ValidationError) {
	v, err := a.events.View(r.Context(), r.PathValue("id"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	af := answerFormView{Action: eventPath(v.Event.ID) + "/answers", Candidates: v.Event.Candidates, Form: form, Errors: errs, Submit: "回答する"}
	a.render(w, r, status, "event", v.Event.Title, eventPageData{View: v, AnswerForm: af})
}

func (a *app) editEvent(w http.ResponseWriter, r *http.Request) {
	a.renderEdit(w, r, http.StatusOK, nil, func(d *editData) { d.Title, d.Memo = d.Event.Title, d.Event.Memo })
}

// renderEdit 編集画面を描画します。fill は入力欄の値を決めます。
func (a *app) renderEdit(w http.ResponseWriter, r *http.Request, status int, errs domain.ValidationError, fill func(*editData)) {
	v, err := a.events.View(r.Context(), r.PathValue("id"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	d := editData{Event: v.Event, Title: v.Event.Title, Memo: v.Event.Memo, Errors: errs}
	fill(&d)
	a.render(w, r, status, "event_edit", "イベントを編集", d)
}

// afterEdit サービスの結果に応じて、成功なら to へ移動し、入力の違反なら編集画面を 422 で描画し直します。
func (a *app) afterEdit(w http.ResponseWriter, r *http.Request, err error, to string, fill func(*editData)) {
	if ve, ok := errors.AsType[domain.ValidationError](err); ok {
		a.renderEdit(w, r, http.StatusUnprocessableEntity, ve, fill)
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	http.Redirect(w, r, to, http.StatusSeeOther) //nolint:gosec // G710 の to は呼び出し元が eventPath で組み立てた同じオリジンのパスです
}

func (a *app) updateEvent(w http.ResponseWriter, r *http.Request) {
	id, title, memo := r.PathValue("id"), r.PostFormValue("title"), r.PostFormValue("memo")
	err := a.events.UpdateMeta(r.Context(), id, title, memo)
	a.afterEdit(w, r, err, eventPath(id), func(d *editData) { d.Title, d.Memo = title, memo })
}

func (a *app) addCandidates(w http.ResponseWriter, r *http.Request) {
	id, text := r.PathValue("id"), r.PostFormValue("candidates")
	err := a.events.AddCandidates(r.Context(), id, text)
	a.afterEdit(w, r, err, eventPath(id)+"/edit", func(d *editData) { d.NewCandidates = text })
}

func (a *app) deleteCandidate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cid, ok := pathInt(r, "cid")
	if !ok {
		a.fail(w, r, domain.ErrNotFound)
		return
	}
	err := a.events.DeleteCandidate(r.Context(), id, cid)
	a.afterEdit(w, r, err, eventPath(id)+"/edit", func(*editData) {})
}

func (a *app) deleteEvent(w http.ResponseWriter, r *http.Request) {
	if err := a.events.Delete(r.Context(), r.PathValue("id")); err != nil {
		a.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
