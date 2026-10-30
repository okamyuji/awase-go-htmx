// Package domain awase のエンティティと入力の規則です。標準ライブラリだけに依存します。
package domain

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"
)

// ErrNotFound 指定した ID のイベント、候補、回答が存在しないことを表します。
var ErrNotFound = errors.New("not found")

// ValidationError 入力の制約に違反した項目名と理由の組です。
type ValidationError map[string]string

func (v ValidationError) Error() string {
	parts := make([]string, 0, len(v))
	for _, k := range slices.Sorted(maps.Keys(v)) {
		parts = append(parts, fmt.Sprintf("%s: %s", k, v[k]))
	}
	return "invalid input: " + strings.Join(parts, ", ")
}

func (v ValidationError) orNil() error {
	if len(v) == 0 {
		return nil
	}
	return v
}

// checkLen 前後の空白を除いた s の文字数が lo〜hi に収まらなければ ve に理由を記録し、除いた値を返します。
func checkLen(ve ValidationError, field, s string, lo, hi int) string {
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n < lo || n > hi {
		ve[field] = fmt.Sprintf("%d〜%d文字で入力してください", lo, hi)
	}
	return s
}
