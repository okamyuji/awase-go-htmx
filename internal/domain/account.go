package domain

import (
	"fmt"
	"unicode/utf8"
)

// 共有アカウントの制約です。
const (
	MaxUsername       = 64
	MinPasswordLength = 12
)

// NormalizeUsername ユーザー名を検証し、前後の空白を除いた値を返します。
func NormalizeUsername(s string) (string, error) {
	ve := ValidationError{}
	u := checkLen(ve, "username", s, 1, MaxUsername)
	return u, ve.orNil()
}

// ValidatePassword パスワードの長さを確かめます。空白はパスワードの一部として扱います。
func ValidatePassword(s string) error {
	if utf8.RuneCountInString(s) < MinPasswordLength {
		return ValidationError{"password": fmt.Sprintf("%d文字以上にしてください", MinPasswordLength)}
	}
	return nil
}
