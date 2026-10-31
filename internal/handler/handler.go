// Package handler awase の HTTP ハンドラとミドルウェアです。
package handler

import (
	"cmp"
	"io"
	"log/slog"
	"net/http"

	"github.com/okamyuji/awase-go-htmx/internal/port"
)

// Deps ハンドラが使う依存です。組み立ては cmd/awase が行います。
type Deps struct {
	Auth         port.AuthService
	Events       port.EventService
	Answers      port.AnswerService
	Logger       *slog.Logger
	CookieSecure bool
}

type app struct {
	auth         port.AuthService
	events       port.EventService
	answers      port.AnswerService
	log          *slog.Logger
	cookieSecure bool
	pages        *renderer
}

// New 全経路を登録した http.Handler を返します。
func New(d Deps) http.Handler {
	a := &app{
		auth:         d.Auth,
		events:       d.Events,
		answers:      d.Answers,
		log:          cmp.Or(d.Logger, slog.Default()),
		cookieSecure: d.CookieSecure,
		pages:        newRenderer("login", "index", "event_new", "event", "event_edit", "answer_edit", "notfound"),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.healthz)
	mux.Handle("GET /static/", staticHandler())
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, a.pages.asset("favicon.svg"), http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /login", a.loginPage)
	mux.HandleFunc("POST /login", a.login)
	private := map[string]http.HandlerFunc{
		"POST /logout":                 a.logout,
		"GET /{$}":                     a.index,
		"GET /events/new":              a.newEvent,
		"POST /events":                 a.createEvent,
		"GET /events/{id}":             a.showEvent,
		"GET /events/{id}/edit":        a.editEvent,
		"POST /events/{id}":            a.updateEvent,
		"POST /events/{id}/candidates": a.addCandidates,
		"POST /events/{id}/candidates/{cid}/delete": a.deleteCandidate,
		"POST /events/{id}/delete":                  a.deleteEvent,
		"POST /events/{id}/answers":                 a.createAnswer,
		"GET /events/{id}/answers/{aid}/edit":       a.editAnswer,
		"POST /events/{id}/answers/{aid}":           a.updateAnswer,
		"POST /events/{id}/answers/{aid}/delete":    a.deleteAnswer,
	}
	for pattern, h := range private {
		mux.Handle(pattern, a.requireAuth(h))
	}
	return a.logRequests(securityHeaders(http.NewCrossOriginProtection().Handler(limitBody(mux))))
}

func (a *app) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := io.WriteString(w, "ok"); err != nil {
		a.log.WarnContext(r.Context(), "write response", "err", err)
	}
}
