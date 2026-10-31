// Command awase 日程調整アプリのサーバーと管理コマンドです。
package main

import (
	"cmp"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/okamyuji/awase-go-htmx/internal/auth"
	"github.com/okamyuji/awase-go-htmx/internal/handler"
	"github.com/okamyuji/awase-go-htmx/internal/service"
	"github.com/okamyuji/awase-go-htmx/internal/store"
)

const usage = "usage: awase [serve] | awase passwd -username <name> < password"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	logger := slog.New(slog.NewJSONHandler(stdout, nil))
	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "serve":
		return runServe(ctx, logger, getenv)
	case "passwd":
		return runPasswd(ctx, args[1:], stdin, stderr, logger, getenv)
	default:
		if _, err := fmt.Fprintln(stderr, usage); err != nil {
			logger.Error("write usage", "err", err)
		}
		return 2
	}
}

func runServe(ctx context.Context, logger *slog.Logger, getenv func(string) string) (code int) {
	db, err := store.Open(ctx, dbPath(getenv))
	if err != nil {
		logger.Error("open db", "err", err)
		return 1
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("close db", "err", err)
			code = 1
		}
	}()
	addr := cmp.Or(getenv("AWASE_ADDR"), ":8080")
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("listen", "addr", addr, "err", err)
		return 1
	}
	clock := systemClock{}
	events, answers := store.NewEvents(db), store.NewAnswers(db)
	h := handler.New(handler.Deps{
		Auth:         service.NewAuth(store.NewAccount(db), store.NewSessions(db), auth.NewScryptHasher(), auth.NewLimiter(), clock),
		Events:       service.NewEvents(events, answers, randomIDs{}, clock),
		Answers:      service.NewAnswers(events, answers, clock),
		Logger:       logger,
		CookieSecure: cookieSecure(getenv),
	})
	if err := serve(ctx, ln, h, logger); err != nil {
		logger.Error("serve", "err", err)
		return 1
	}
	return 0
}

// cookieSecure 本番では Secure を必ず付けるため、"false" を明示したときだけ外します。
func cookieSecure(getenv func(string) string) bool {
	return getenv("AWASE_COOKIE_SECURE") != "false"
}

// randomIDs イベント ID は URL に出るため、推測できない乱数の文字列にします。
type randomIDs struct{}

func (randomIDs) NewID() string { return rand.Text() }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// newServer h を提供するサーバーを作ります。1つの接続が要求の読み込みと待機で居座れる時間を限ります。
func newServer(h http.Handler) *http.Server {
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// serve ctx が取り消されるまで ln で h を提供し、取り消し後は処理中の要求を待って止まります。
func serve(ctx context.Context, ln net.Listener, h http.Handler, logger *slog.Logger) error {
	srv := newServer(h)
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	logger.Info("listening", "addr", ln.Addr().String())

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
