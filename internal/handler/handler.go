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
	Logger       *slog.Logger
	CookieSecure bool
}

type app struct {
	auth         port.AuthService
	log          *slog.Logger
	cookieSecure bool
	pages        *renderer
}

// New 全経路を登録した http.Handler を返します。
func New(d Deps) http.Handler {
	a := &app{
		auth:         d.Auth,
		log:          cmp.Or(d.Logger, slog.Default()),
		cookieSecure: d.CookieSecure,
		pages:        newRenderer("login", "index"),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.healthz)
	mux.Handle("GET /static/", staticHandler())
	mux.HandleFunc("GET /login", a.loginPage)
	mux.HandleFunc("POST /login", a.login)
	mux.Handle("POST /logout", a.requireAuth(http.HandlerFunc(a.logout)))
	mux.Handle("GET /{$}", a.requireAuth(http.HandlerFunc(a.index)))
	return a.logRequests(securityHeaders(http.NewCrossOriginProtection().Handler(mux)))
}

func (a *app) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := io.WriteString(w, "ok"); err != nil {
		a.log.WarnContext(r.Context(), "write response", "err", err)
	}
}

func (a *app) index(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, "index", "イベント", nil)
}
