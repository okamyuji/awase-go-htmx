package handler

import (
	"errors"
	"net/http"

	"github.com/okamyuji/awase-go-htmx/internal/port"
)

type loginData struct {
	Next     string
	Username string
	Error    string
}

func (a *app) loginPage(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, "login", "ログイン", loginData{Next: safeNext(r.URL.Query().Get("next"))})
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	user, next := r.PostFormValue("username"), safeNext(r.PostFormValue("next"))
	token, err := a.auth.Login(r.Context(), clientIP(r), user, r.PostFormValue("password"))
	if err != nil {
		status, msg := loginFailure(err)
		if status == http.StatusInternalServerError {
			a.log.ErrorContext(r.Context(), "login", "err", err)
		}
		a.render(w, r, status, "login", "ログイン", loginData{Next: next, Username: user, Error: msg})
		return
	}
	// Secure は本番で常に有効で、AWASE_COOKIE_SECURE=false のローカル開発だけ外れます。
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124 の Secure は上記のとおり設定で切り替えます
		Name: cookieName, Value: token, Path: "/", MaxAge: int(port.SessionTTL.Seconds()),
		HttpOnly: true, Secure: a.cookieSecure, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, next, http.StatusSeeOther) //nolint:gosec // G710 の移動先は safeNext で awase 内のパスに限っています
}

func loginFailure(err error) (int, string) {
	switch {
	case errors.Is(err, port.ErrInvalidCredentials):
		return http.StatusUnauthorized, "ユーザー名またはパスワードが違います"
	case errors.Is(err, port.ErrLocked):
		return http.StatusTooManyRequests, "ログインの失敗が続いたため、しばらく待ってからやり直してください"
	default:
		return http.StatusInternalServerError, "エラーが発生しました。時間をおいてやり直してください"
	}
}

func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		if err := a.auth.Logout(r.Context(), c.Value); err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124 の Secure は設定で切り替えます
		Name: cookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: a.cookieSecure, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
