// Command awase 日程調整アプリのサーバーと管理コマンドです。
package main

import (
	"cmp"
	"context"
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

	"github.com/okamyuji/awase-go-htmx/internal/handler"
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

func runServe(ctx context.Context, logger *slog.Logger, getenv func(string) string) int {
	addr := cmp.Or(getenv("AWASE_ADDR"), ":8080")
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("listen", "addr", addr, "err", err)
		return 1
	}
	if err := serve(ctx, ln, handler.New(handler.Deps{Logger: logger}), logger); err != nil {
		logger.Error("serve", "err", err)
		return 1
	}
	return 0
}

// serve ctx が取り消されるまで ln で h を提供し、取り消し後は処理中の要求を待って止まります。
func serve(ctx context.Context, ln net.Listener, h http.Handler, logger *slog.Logger) error {
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
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
