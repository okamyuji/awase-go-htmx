// Package auth パスワードのハッシュ化とログイン失敗の制限です。
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/scrypt"
)

// scrypt のパラメータです。値は基本設計書の7.1節に従います。
const (
	scryptN      = 32768
	scryptR      = 8
	scryptP      = 1
	saltLen      = 16
	scryptKeyLen = 32
)

var b64 = base64.RawStdEncoding

// ScryptHasher port.PasswordHasher の scrypt 実装です。
type ScryptHasher struct {
	// sem 1回の計算で約32MiBを使うため、同時に1件だけ計算させます。
	sem    chan struct{}
	derive func(password, salt []byte, n, r, p, keyLen int) ([]byte, error)
	// dummy passwd のように照合しない使い方で scrypt を1回分むだに計算しないよう、初めて使うときに作ります。
	dummy func() string
}

// NewScryptHasher ScryptHasher を返します。
func NewScryptHasher() *ScryptHasher {
	h := &ScryptHasher{sem: make(chan struct{}, 1), derive: scrypt.Key}
	h.dummy = sync.OnceValue(func() string {
		enc, err := h.Hash(rand.Text())
		if err != nil {
			panic(fmt.Sprintf("auth: scrypt parameters are invalid: %v", err))
		}
		return enc
	})
	return h
}

// DummyHash 誰のパスワードにも一致しないハッシュです。アカウントがないときの照合に使います。
func (h *ScryptHasher) DummyHash() string { return h.dummy() }

// Hash password を "scrypt$N$r$p$ソルト$ハッシュ" の形に符号化します。
func (h *ScryptHasher) Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	rand.Read(salt) //nolint:errcheck // Go 1.24 以降の crypto/rand.Read はエラーを返しません。
	key, err := h.key([]byte(password), salt, scryptN, scryptR, scryptP)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("scrypt$%d$%d$%d$%s$%s", scryptN, scryptR, scryptP, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify password が encoded と一致すれば真を返します。形式が壊れていれば偽です。
func (h *ScryptHasher) Verify(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "scrypt" {
		return false
	}
	n, errN := strconv.Atoi(parts[1])
	r, errR := strconv.Atoi(parts[2])
	p, errP := strconv.Atoi(parts[3])
	salt, errS := b64.DecodeString(parts[4])
	want, errW := b64.DecodeString(parts[5])
	if errN != nil || errR != nil || errP != nil || errS != nil || errW != nil {
		return false
	}
	got, err := h.key([]byte(password), salt, n, r, p)
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

func (h *ScryptHasher) key(password, salt []byte, n, r, p int) ([]byte, error) {
	h.sem <- struct{}{}
	defer func() { <-h.sem }()
	return h.derive(password, salt, n, r, p, scryptKeyLen)
}
