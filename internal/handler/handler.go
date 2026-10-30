// Package handler awase の HTTP ハンドラとミドルウェアです。
package handler

import (
	"cmp"
	"io"
	"log/slog"
	"net/http"
)

// Deps ハンドラが使う依存です。組み立ては cmd/awase が行います。
type Deps struct {
	Logger *slog.Logger
}

type app struct {
	log *slog.Logger
}

// New 全経路を登録した http.Handler を返します。
func New(d Deps) http.Handler {
	a := &app{log: cmp.Or(d.Logger, slog.Default())}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.healthz)
	return mux
}

func (a *app) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := io.WriteString(w, "ok"); err != nil {
		a.log.WarnContext(r.Context(), "write response", "err", err)
	}
}
