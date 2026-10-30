package main

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/okamyuji/awase-go-htmx/internal/auth"
	"github.com/okamyuji/awase-go-htmx/internal/domain"
	"github.com/okamyuji/awase-go-htmx/internal/store"
)

func dbPath(getenv func(string) string) string {
	return cmp.Or(getenv("AWASE_DB_PATH"), "/data/awase.db")
}

// runPasswd 標準入力の1行目を共有アカウントのパスワードとして設定し、全セッションを無効にします。
func runPasswd(ctx context.Context, args []string, stdin io.Reader, stderr io.Writer, logger *slog.Logger, getenv func(string) string) int {
	fs := flag.NewFlagSet("passwd", flag.ContinueOnError)
	fs.SetOutput(stderr)
	username := fs.String("username", "", "共有アカウントのユーザー名")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := setAccount(ctx, *username, stdin, getenv); err != nil {
		if _, werr := fmt.Fprintln(stderr, "passwd:", err); werr != nil {
			logger.Error("write passwd error", "err", werr)
		}
		return 1
	}
	logger.Info("account updated")
	return 0
}

func setAccount(ctx context.Context, rawUser string, stdin io.Reader, getenv func(string) string) (err error) {
	user, err := domain.NormalizeUsername(rawUser)
	if err != nil {
		return err
	}
	password, err := readPassword(stdin)
	if err != nil {
		return err
	}
	if err := domain.ValidatePassword(password); err != nil {
		return err
	}
	hash, err := auth.NewScryptHasher().Hash(password)
	if err != nil {
		return err
	}
	db, err := store.Open(ctx, dbPath(getenv))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	_, err = store.NewAccount(db).Set(ctx, user, hash, time.Now())
	return err
}

// readPassword 1行目を読み、行末の LF と CRLF だけを取り除きます。空白はパスワードの一部です。
func readPassword(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSuffix(line, "\n")
	return strings.TrimSuffix(line, "\r"), nil
}
