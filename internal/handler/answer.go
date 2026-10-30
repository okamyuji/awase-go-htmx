package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/okamyuji/awase-go-htmx/internal/domain"
)

type answerForm struct {
	Name, Comment string
	Choices       map[int64]domain.Choice
}

// answerFormView 回答の入力欄の部品テンプレートに渡す値です。
type answerFormView struct {
	Action     string
	Candidates []domain.Candidate
	Form       answerForm
	Errors     domain.ValidationError
	Submit     string
}

type answerEditData struct {
	Event      domain.Event
	AnswerID   int64
	AnswerForm answerFormView
}

// parseChoices choice-<候補ID> の項目をすべて読みます。候補の集合の検証はサービスが行うので、ここでは捨てません。
func parseChoices(form url.Values) map[int64]domain.Choice {
	out := map[int64]domain.Choice{}
	for k, vs := range form {
		idText, ok := strings.CutPrefix(k, "choice-")
		if !ok || len(vs) == 0 {
			continue
		}
		id, err := strconv.ParseInt(idText, 10, 64)
		c, valid := domain.ParseChoice(vs[0])
		if err == nil && valid {
			out[id] = c
		}
	}
	return out
}

func readAnswerForm(r *http.Request) answerForm {
	if err := r.ParseForm(); err != nil {
		return answerForm{}
	}
	return answerForm{Name: r.PostForm.Get("name"), Comment: r.PostForm.Get("comment"), Choices: parseChoices(r.PostForm)}
}

func (a *app) createAnswer(w http.ResponseWriter, r *http.Request) {
	id, f := r.PathValue("id"), readAnswerForm(r)
	err := a.answers.Create(r.Context(), id, f.Name, f.Comment, f.Choices)
	if ve, ok := errors.AsType[domain.ValidationError](err); ok {
		a.renderEvent(w, r, http.StatusUnprocessableEntity, f, ve)
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	http.Redirect(w, r, eventPath(id), http.StatusSeeOther) //nolint:gosec // G710 の移動先は eventPath で /events/ 以下の同じオリジンのパスに限っています
}

func (a *app) editAnswer(w http.ResponseWriter, r *http.Request) {
	aid, ok := pathInt(r, "aid")
	if !ok {
		a.fail(w, r, domain.ErrNotFound)
		return
	}
	ans, err := a.answers.Get(r.Context(), r.PathValue("id"), aid)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.renderAnswerEdit(w, r, http.StatusOK, aid, answerForm{Name: ans.Name, Comment: ans.Comment, Choices: ans.Choices}, nil)
}

func (a *app) renderAnswerEdit(w http.ResponseWriter, r *http.Request, status int, aid int64, f answerForm, errs domain.ValidationError) {
	v, err := a.events.View(r.Context(), r.PathValue("id"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	af := answerFormView{
		Action:     eventPath(v.Event.ID) + "/answers/" + strconv.FormatInt(aid, 10),
		Candidates: v.Event.Candidates, Form: f, Errors: errs, Submit: "保存",
	}
	a.render(w, r, status, "answer_edit", "回答を編集", answerEditData{Event: v.Event, AnswerID: aid, AnswerForm: af})
}

func (a *app) updateAnswer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	aid, ok := pathInt(r, "aid")
	if !ok {
		a.fail(w, r, domain.ErrNotFound)
		return
	}
	f := readAnswerForm(r)
	err := a.answers.Update(r.Context(), id, aid, f.Name, f.Comment, f.Choices)
	if ve, ok := errors.AsType[domain.ValidationError](err); ok {
		a.renderAnswerEdit(w, r, http.StatusUnprocessableEntity, aid, f, ve)
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	http.Redirect(w, r, eventPath(id), http.StatusSeeOther) //nolint:gosec // G710 の移動先は eventPath で /events/ 以下の同じオリジンのパスに限っています
}

func (a *app) deleteAnswer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	aid, ok := pathInt(r, "aid")
	if !ok {
		a.fail(w, r, domain.ErrNotFound)
		return
	}
	if err := a.answers.Delete(r.Context(), id, aid); err != nil {
		a.fail(w, r, err)
		return
	}
	http.Redirect(w, r, eventPath(id), http.StatusSeeOther) //nolint:gosec // G710 の移動先は eventPath で /events/ 以下の同じオリジンのパスに限っています
}
