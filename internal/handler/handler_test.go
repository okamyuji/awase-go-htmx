package handler

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHealthzReturnsOK(t *testing.T) {
	h := New(Deps{Auth: &fakeAuth{}})
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "ok" {
		t.Fatalf("body = %q, want ok", got)
	}
}

type failingWriter struct{ *httptest.ResponseRecorder }

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func (failingWriter) WriteString(string) (int, error) { return 0, errors.New("broken pipe") }

func TestHealthzLogsWriteError(t *testing.T) {
	var logs bytes.Buffer
	h := New(Deps{Auth: &fakeAuth{}, Logger: slog.New(slog.NewJSONHandler(&logs, nil))})

	h.ServeHTTP(failingWriter{httptest.NewRecorder()}, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if !strings.Contains(logs.String(), "write response") {
		t.Fatalf("logs = %q, want write response", logs.String())
	}
}

// loginFormOfSize 正しい資格情報に詰め物を足して、本文がちょうど n バイトのログインのフォームを作ります。
func loginFormOfSize(t *testing.T, n int) *http.Request {
	t.Helper()
	body := url.Values{"username": {"demo"}, "password": {"not-a-real-pass-1"}}.Encode() + "&pad="
	body += strings.Repeat("a", n-len(body))
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestLoginFormOverBodyLimitIsRejectedBeforeLogin(t *testing.T) {
	a := &fakeAuth{}
	h, _ := newTestHandler(t, a)

	rec := do(h, loginFormOfSize(t, maxBodyBytes+1))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if a.loginIP != "" {
		t.Fatalf("Login was called for an oversized body")
	}
}

func TestLoginFormAtBodyLimitIsAccepted(t *testing.T) {
	h, _ := newTestHandler(t, &fakeAuth{})

	rec := do(h, loginFormOfSize(t, maxBodyBytes))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
}

func TestAuthenticatedFormOverBodyLimitIsRejected(t *testing.T) {
	h, _ := newTestHandler(t, &fakeAuth{})
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader("title="+strings.Repeat("a", maxBodyBytes)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: cookieName, Value: goodToken})

	rec := do(h, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func TestMalformedFormIsRejectedBeforeLogin(t *testing.T) {
	a := &fakeAuth{}
	h, _ := newTestHandler(t, a)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := do(h, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if a.loginIP != "" {
		t.Fatalf("Login was called for a malformed body")
	}
}

func TestRequestLogIncludesClientIP(t *testing.T) {
	h, logs := newTestHandler(t, &fakeAuth{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Real-IP", "203.0.113.7")

	do(h, req)

	if !strings.Contains(logs.String(), `"ip":"203.0.113.7"`) {
		t.Fatalf("logs = %q, want ip 203.0.113.7", logs.String())
	}
}
