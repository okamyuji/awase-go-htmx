package handler

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
