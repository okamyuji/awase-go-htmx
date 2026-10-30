package main

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okamyuji/awase-go-htmx/internal/auth"
	"github.com/okamyuji/awase-go-htmx/internal/domain"
	"github.com/okamyuji/awase-go-htmx/internal/port"
	"github.com/okamyuji/awase-go-htmx/internal/store"
)

func passwd(t *testing.T, dbPath, username, stdin string) (int, string) {
	t.Helper()
	var stderr bytes.Buffer
	code := run(t.Context(), []string{"passwd", "-username", username}, strings.NewReader(stdin), io.Discard, &stderr, env(map[string]string{"AWASE_DB_PATH": dbPath}))
	return code, stderr.String()
}

func readAccount(t *testing.T, dbPath string) (port.Account, error) {
	t.Helper()
	db, err := store.Open(t.Context(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	return store.NewAccount(db).Get(t.Context())
}

func TestPasswdSetsTrimmedUsernameAndStripsCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "awase.db")

	code, stderr := passwd(t, path, "  demo  ", "not-a-real-pass-1\r\nignored\n")

	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr)
	}
	acc, err := readAccount(t, path)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Username != "demo" || acc.Version != 1 {
		t.Fatalf("account = %+v", acc)
	}
	if !auth.NewScryptHasher().Verify(acc.PasswordHash, "not-a-real-pass-1") {
		t.Fatal("stored hash does not match the password without CR")
	}
}

func TestPasswdSecondRunBumpsVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "awase.db")
	if code, stderr := passwd(t, path, "demo", "not-a-real-pass-1\n"); code != 0 {
		t.Fatal(stderr)
	}

	code, stderr := passwd(t, path, "demo", "not-a-real-pass-2")

	acc, err := readAccount(t, path)
	if code != 0 || err != nil || acc.Version != 2 {
		t.Fatalf("code=%d stderr=%s acc=%+v err=%v", code, stderr, acc, err)
	}
}

func TestPasswdRejectsInvalidInputWithoutChangingAccount(t *testing.T) {
	tests := []struct{ name, user, pass string }{
		{"11文字のパスワード", "demo", "not-a-real\n"},
		{"空のユーザー名", "", "not-a-real-pass-1\n"},
		{"空白だけのユーザー名", "   ", "not-a-real-pass-1\n"},
		{"65文字のユーザー名", strings.Repeat("u", 65), "not-a-real-pass-1\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "awase.db")

			code, stderr := passwd(t, path, tt.user, tt.pass)

			if code != 1 || stderr == "" {
				t.Fatalf("code = %d stderr = %q, want 1 with reason", code, stderr)
			}
			if _, err := readAccount(t, path); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("account changed: %v", err)
			}
		})
	}
}

func TestPasswdFailsWhenDBCannotOpen(t *testing.T) {
	code, _ := passwd(t, filepath.Join(t.TempDir(), "no", "such", "awase.db"), "demo", "not-a-real-pass-1\n")
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
}

func TestPasswdRejectsUnknownFlag(t *testing.T) {
	var stderr bytes.Buffer
	code := run(t.Context(), []string{"passwd", "-bogus"}, strings.NewReader(""), io.Discard, &stderr, env(nil))
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
}
