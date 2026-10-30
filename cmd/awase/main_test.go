package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func env(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func TestRunUnknownCommandPrintsUsage(t *testing.T) {
	var stderr bytes.Buffer

	code := run(t.Context(), []string{"bogus"}, strings.NewReader(""), io.Discard, &stderr, env(nil))

	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestRunServeFailsOnInvalidAddr(t *testing.T) {
	var stderr bytes.Buffer

	code := run(t.Context(), nil, strings.NewReader(""), io.Discard, &stderr, env(map[string]string{"AWASE_ADDR": "not-an-addr", "AWASE_DB_PATH": filepath.Join(t.TempDir(), "awase.db")}))

	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
}

func TestServeAnswersHealthzAndStopsOnCancel(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(w, "ok"); err != nil {
			t.Error(err)
		}
	})
	go func() { done <- serve(ctx, ln, h, slog.New(slog.DiscardHandler)) }()

	resp, err := http.Get("http://" + ln.Addr().String() + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	if cerr := resp.Body.Close(); cerr != nil {
		t.Error(cerr)
	}
	cancel()
	if err != nil {
		t.Fatal(err)
	}

	if string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}
	if err := <-done; err != nil {
		t.Fatalf("serve returned %v, want nil", err)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRunUnknownCommandStillExits2WhenStderrFails(t *testing.T) {
	var stdout bytes.Buffer

	code := run(t.Context(), []string{"bogus"}, strings.NewReader(""), &stdout, failWriter{}, env(nil))

	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stdout.String(), "write usage") {
		t.Fatalf("log = %q, want write usage error", stdout.String())
	}
}

// cancelOnListening listening のログが書かれた時点で ctx を取り消します。
type cancelOnListening struct {
	bytes.Buffer
	cancel context.CancelFunc
}

func (c *cancelOnListening) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte(`"msg":"listening"`)) {
		c.cancel()
	}
	return c.Buffer.Write(p)
}

func TestRunServeExitsZeroWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	stdout := &cancelOnListening{cancel: cancel}

	code := run(ctx, []string{"serve"}, strings.NewReader(""), stdout, io.Discard, env(map[string]string{"AWASE_ADDR": "127.0.0.1:0", "AWASE_DB_PATH": filepath.Join(t.TempDir(), "awase.db")}))

	if code != 0 {
		t.Fatalf("code = %d, want 0; log=%s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), `"msg":"listening"`) {
		t.Fatalf("log = %q, want JSON listening line", stdout.String())
	}
}

func TestServeReturnsErrorWhenListenerClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}

	err = serve(t.Context(), ln, http.NotFoundHandler(), slog.New(slog.DiscardHandler))

	if err == nil {
		t.Fatal("serve returned nil, want error")
	}
}

func TestRunServeFailsWhenDBCannotOpen(t *testing.T) {
	code := run(t.Context(), nil, strings.NewReader(""), io.Discard, io.Discard, env(map[string]string{"AWASE_ADDR": "127.0.0.1:0", "AWASE_DB_PATH": filepath.Join(t.TempDir(), "no", "awase.db")}))

	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
}

func TestCookieSecureIsOnUnlessExplicitlyFalse(t *testing.T) {
	for v, want := range map[string]bool{"": true, "true": true, "false": false, "0": true} {
		if got := cookieSecure(env(map[string]string{"AWASE_COOKIE_SECURE": v})); got != want {
			t.Errorf("AWASE_COOKIE_SECURE=%q -> %v, want %v", v, got, want)
		}
	}
}

func TestRandomIDsAreUniqueAndURLSafe(t *testing.T) {
	a, b := randomIDs{}.NewID(), randomIDs{}.NewID()
	if a == b || len(a) < 26 || strings.ContainsAny(a, "/?#&%") {
		t.Fatalf("ids = %q %q", a, b)
	}
}
