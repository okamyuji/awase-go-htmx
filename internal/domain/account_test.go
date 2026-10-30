package domain

import (
	"strings"
	"testing"
)

func TestNormalizeUsername(t *testing.T) {
	got, err := NormalizeUsername("  demo  ")
	if err != nil || got != "demo" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := NormalizeUsername(strings.Repeat("u", 64)); err != nil {
		t.Fatalf("64文字: %v", err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("u", 65)} {
		_, err := NormalizeUsername(bad)
		fieldErr(t, err, "username")
	}
}

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword(strings.Repeat("p", 12)); err != nil {
		t.Fatalf("12文字: %v", err)
	}
	if err := ValidatePassword("あいうえおかきくけこさし"); err != nil {
		t.Fatalf("全角12文字: %v", err)
	}
	fieldErr(t, ValidatePassword(strings.Repeat("p", 11)), "password")
}
