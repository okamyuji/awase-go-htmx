package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"html/template"
	"io/fs"
	"net/http"
	"path"

	"github.com/okamyuji/awase-go-htmx/web"
)

// view 全画面に共通のテンプレートの値です。Data は画面ごとの値です。
type view struct {
	Title  string
	Authed bool
	Data   any
}

type renderer struct {
	pages map[string]*template.Template
}

// assetVersions 静的ファイルの内容のハッシュです。URL に付けて、内容が変わったらブラウザと Cloudflare に取り直させます。
func assetVersions() map[string]string {
	out := map[string]string{}
	entries, err := fs.ReadDir(web.FS, "static")
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		b, err := fs.ReadFile(web.FS, path.Join("static", e.Name()))
		if err != nil {
			panic(err)
		}
		sum := sha256.Sum256(b)
		out[e.Name()] = hex.EncodeToString(sum[:5])
	}
	return out
}

// newRenderer 画面ごとに layout と組み合わせたテンプレートを作ります。埋め込みのテンプレートが壊れていれば起動時に panic します。
func newRenderer(pages ...string) *renderer {
	versions := assetVersions()
	funcs := template.FuncMap{
		"asset": func(name string) string { return "/static/" + name + "?v=" + versions[name] },
	}
	r := &renderer{pages: map[string]*template.Template{}}
	for _, p := range pages {
		r.pages[p] = template.Must(template.New("").Funcs(funcs).ParseFS(web.FS, "templates/layout.html", "templates/"+p+".html"))
	}
	return r
}

func (a *app) render(w http.ResponseWriter, r *http.Request, status int, page, title string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	v := view{Title: title, Authed: authed(r.Context()), Data: data}
	if err := a.pages.pages[page].ExecuteTemplate(w, "layout", v); err != nil {
		a.log.ErrorContext(r.Context(), "render", "page", page, "err", err)
	}
}

func staticHandler() http.Handler {
	sub, err := fs.Sub(web.FS, "static")
	if err != nil {
		panic(err)
	}
	files := http.FileServerFS(sub)
	return http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		files.ServeHTTP(w, r)
	}))
}
