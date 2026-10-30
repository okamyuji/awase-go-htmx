package handler

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/okamyuji/awase-go-htmx/internal/port"
)

const goodToken = "good-token"

type fakeAuth struct {
	loginErr  error
	loginIP   string
	loggedOut []string
}

func (f *fakeAuth) Login(_ context.Context, ip, user, pass string) (string, error) {
	f.loginIP = ip
	if f.loginErr != nil {
		return "", f.loginErr
	}
	if user != "demo" || pass != "not-a-real-pass-1" {
		return "", port.ErrInvalidCredentials
	}
	return goodToken, nil
}
func (f *fakeAuth) Logout(_ context.Context, tok string) error {
	f.loggedOut = append(f.loggedOut, tok)
	return nil
}
func (f *fakeAuth) Authenticate(_ context.Context, tok string) (bool, error) {
	return tok == goodToken, nil
}

func newTestHandler(t *testing.T, a *fakeAuth) (http.Handler, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	h := New(Deps{Auth: a, Logger: slog.New(slog.NewJSONHandler(&logs, nil)), CookieSecure: true})
	return h, &logs
}

func do(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func postForm(target string, form url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func withSession(req *http.Request) *http.Request {
	req.AddCookie(&http.Cookie{Name: cookieName, Value: goodToken})
	return req
}

func TestUnauthenticatedGETRedirectsToLoginWithNext(t *testing.T) {
	h, _ := newTestHandler(t, &fakeAuth{})

	rec := do(h, httptest.NewRequest(http.MethodGet, "/?x=1", nil))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login?next=%2F%3Fx%3D1" {
		t.Fatalf("status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestUnauthenticatedPOSTRedirectsToLoginWithoutNext(t *testing.T) {
	h, _ := newTestHandler(t, &fakeAuth{})

	rec := do(h, postForm("/logout", nil))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestUnauthenticatedHtmxRequestGetsHXRedirect(t *testing.T) {
	h, _ := newTestHandler(t, &fakeAuth{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("HX-Request", "true")
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "stale-after-passwd"})

	rec := do(h, req)

	if rec.Header().Get("HX-Redirect") != "/login?next=%2F" {
		t.Fatalf("HX-Redirect = %q, status=%d", rec.Header().Get("HX-Redirect"), rec.Code)
	}
}

func TestPublicPathsNeedNoSession(t *testing.T) {
	h, _ := newTestHandler(t, &fakeAuth{})
	for _, p := range []string{"/login", "/healthz", "/static/htmx.min.js", "/static/app.css"} {
		if rec := do(h, httptest.NewRequest(http.MethodGet, p, nil)); rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", p, rec.Code)
		}
	}
}

func TestLoginSuccessSetsCookieAndFollowsSafeNext(t *testing.T) {
	a := &fakeAuth{}
	h, _ := newTestHandler(t, a)
	req := postForm("/login", url.Values{"username": {"demo"}, "password": {"not-a-real-pass-1"}, "next": {"/events/abc"}})
	req.Header.Set("X-Real-IP", "203.0.113.9")

	rec := do(h, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/events/abc" {
		t.Fatalf("status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
	assertSessionCookie(t, rec.Result().Cookies()[0])
	if a.loginIP != "203.0.113.9" {
		t.Fatalf("ip = %q", a.loginIP)
	}
}

func assertSessionCookie(t *testing.T, c *http.Cookie) {
	t.Helper()
	type attrs struct {
		Name, Value, Path string
		MaxAge            int
		HTTPOnly, Secure  bool
		SameSite          http.SameSite
	}
	want := attrs{cookieName, goodToken, "/", 2592000, true, true, http.SameSiteLaxMode}
	got := attrs{c.Name, c.Value, c.Path, c.MaxAge, c.HttpOnly, c.Secure, c.SameSite}
	if got != want {
		t.Fatalf("cookie = %+v, want %+v", got, want)
	}
}

func TestLoginUsesRemoteAddrWithoutRealIP(t *testing.T) {
	a := &fakeAuth{}
	h, _ := newTestHandler(t, a)
	req := postForm("/login", url.Values{"username": {"demo"}, "password": {"not-a-real-pass-1"}})
	req.RemoteAddr = "198.51.100.7:5555"

	do(h, req)

	if a.loginIP != "198.51.100.7" {
		t.Fatalf("ip = %q", a.loginIP)
	}
}

func TestLoginFailureStatuses(t *testing.T) {
	tests := []struct {
		name string
		auth *fakeAuth
		want int
		msg  string
	}{
		{"誤った資格情報", &fakeAuth{}, http.StatusUnauthorized, "ユーザー名またはパスワードが違います"},
		{"拒否中", &fakeAuth{loginErr: port.ErrLocked}, http.StatusTooManyRequests, "しばらく待って"},
		{"内部エラー", &fakeAuth{loginErr: context.DeadlineExceeded}, http.StatusInternalServerError, "エラー"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newTestHandler(t, tt.auth)

			rec := do(h, postForm("/login", url.Values{"username": {"demo"}, "password": {"wrong"}, "next": {"/events/x"}}))

			if rec.Code != tt.want || !strings.Contains(rec.Body.String(), tt.msg) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if len(rec.Result().Cookies()) != 0 {
				t.Fatal("cookie set on failure")
			}
		})
	}
}

func TestSafeNext(t *testing.T) {
	tests := map[string]string{
		"":                    "/",
		"/events/abc":         "/events/abc",
		"/events/abc?x=1":     "/events/abc?x=1",
		"https://example.com": "/",
		"//example.com":       "/",
		"/\\example.com":      "/",
		"/\t/example.com":     "/",
		"events":              "/",
		"/%zz":                "/",
	}
	for in, want := range tests {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLogoutClearsCookieAndSession(t *testing.T) {
	a := &fakeAuth{}
	h, _ := newTestHandler(t, a)

	rec := do(h, withSession(postForm("/logout", nil)))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
	if c := rec.Result().Cookies()[0]; c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("cookie not cleared: %+v", c)
	}
	if len(a.loggedOut) != 1 || a.loggedOut[0] != goodToken {
		t.Fatalf("loggedOut = %v", a.loggedOut)
	}
}

func TestCrossOriginPOSTIsRejected(t *testing.T) {
	h, _ := newTestHandler(t, &fakeAuth{})
	req := withSession(postForm("/logout", nil))
	req.Header.Set("Sec-Fetch-Site", "cross-site")

	if rec := do(h, req); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestSecurityHeadersAppearOnce(t *testing.T) {
	h, _ := newTestHandler(t, &fakeAuth{})

	rec := do(h, httptest.NewRequest(http.MethodGet, "/login", nil))

	want := map[string]string{
		"Content-Security-Policy": "default-src 'self'",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "same-origin",
		"X-Frame-Options":         "DENY",
	}
	for k, v := range want {
		if got := rec.Header().Values(k); len(got) != 1 || got[0] != v {
			t.Errorf("%s = %v, want [%s]", k, got, v)
		}
	}
}

func TestRequestsAreLoggedAsJSON(t *testing.T) {
	h, logs := newTestHandler(t, &fakeAuth{})

	do(h, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if !strings.Contains(logs.String(), `"msg":"request"`) || !strings.Contains(logs.String(), `"status":200`) {
		t.Fatalf("logs = %s", logs.String())
	}
}

func TestLoginPageCarriesNextAndHtmxConfig(t *testing.T) {
	h, _ := newTestHandler(t, &fakeAuth{})

	body := do(h, httptest.NewRequest(http.MethodGet, "/login?next=%2Fevents%2Fabc", nil)).Body.String()

	for _, s := range []string{`name="next" value="/events/abc"`, `name="htmx-config"`, `"includeIndicatorStyles":false`, `/static/htmx.min.js?v=`, `hx-boost="true"`} {
		if !strings.Contains(body, s) {
			t.Errorf("login page lacks %s", s)
		}
	}
}

func TestStaticAssetsAreCachedLong(t *testing.T) {
	h, _ := newTestHandler(t, &fakeAuth{})

	rec := do(h, httptest.NewRequest(http.MethodGet, "/static/app.css?v=abc", nil))

	if rec.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
}
