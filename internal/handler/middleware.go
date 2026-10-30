package handler

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const cookieName = "awase_session"

type ctxKey struct{}

func authed(ctx context.Context) bool {
	v, _ := ctx.Value(ctxKey{}).(bool)
	return v
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func (a *app) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		a.log.InfoContext(r.Context(), "request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "ms", time.Since(start).Milliseconds())
	})
}

// requireAuth セッションのない要求をログイン画面へ移動させます。元のページを覚えるのは GET の場合だけです。
func (a *app) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, err := a.authenticate(r)
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		if ok {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, true)))
			return
		}
		target := "/login"
		if r.Method == http.MethodGet {
			target += "?next=" + url.QueryEscape(r.URL.RequestURI())
		}
		// hx-boost の要求に 303 を返すと、htmx は応答を差し込むだけでアドレスバーが元のままになります。
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", target)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
	})
}

func (a *app) authenticate(r *http.Request) (bool, error) {
	c, err := r.Cookie(cookieName)
	if errors.Is(err, http.ErrNoCookie) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return a.auth.Authenticate(r.Context(), c.Value)
}

func (a *app) serverError(w http.ResponseWriter, r *http.Request, err error) {
	a.log.ErrorContext(r.Context(), "internal error", "path", r.URL.Path, "err", err)
	http.Error(w, "エラーが発生しました", http.StatusInternalServerError)
}

// clientIP nginx が Cloudflare の CF-Connecting-IP から復元して設定する X-Real-IP を使います。
// awase-app はポートを公開しないので、nginx を経由しない要求は届きません。
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// safeNext ログイン後の移動先を awase 内のパスに限ります。条件を満たさなければ一覧へ移動させます。
func safeNext(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" ||
		!strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.Contains(raw, `\`) {
		return "/"
	}
	return raw
}
