package domain

import (
	"time"
)

// 入力の上限です。値は要件定義書の3章に従います。
const (
	MaxName    = 30
	MaxComment = 200
)

// Choice 候補ごとの出欠です。
type Choice int8

// 出欠の値です。DB とフォームはこの整数で保持します。
const (
	No    Choice = 0
	Maybe Choice = 1
	Yes   Choice = 2
)

// Symbol 出欠を画面の記号で返します。未知の値は未回答の「−」です。
func (c Choice) Symbol() string {
	switch c {
	case No:
		return "×"
	case Maybe:
		return "△"
	case Yes:
		return "○"
	default:
		return "−"
	}
}

// ParseChoice フォームの値 "0"、"1"、"2" を出欠に変えます。
func ParseChoice(s string) (Choice, bool) {
	switch s {
	case "0":
		return No, true
	case "1":
		return Maybe, true
	case "2":
		return Yes, true
	default:
		return 0, false
	}
}

// Answer 参加者1名分の出欠です。Choices は候補 ID ごとの出欠です。
type Answer struct {
	ID        int64
	EventID   string
	Name      string
	Comment   string
	Choices   map[int64]Choice
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AnswerDraft 検証済みの回答の入力です。
type AnswerDraft struct {
	Name    string
	Comment string
	Choices map[int64]Choice
}

// NewAnswerDraft 回答を検証します。choices の候補 ID の集合は candidateIDs と等しい必要があります。
func NewAnswerDraft(name, comment string, choices map[int64]Choice, candidateIDs []int64) (AnswerDraft, error) {
	ve := ValidationError{}
	d := AnswerDraft{
		Name:    checkLen(ve, "name", name, 1, MaxName),
		Comment: checkLen(ve, "comment", comment, 0, MaxComment),
		Choices: choices,
	}
	if !sameKeys(choices, candidateIDs) {
		ve["choices"] = "すべての候補について出欠を選んでください"
	}
	return d, ve.orNil()
}

func sameKeys(m map[int64]Choice, ids []int64) bool {
	if len(m) != len(ids) {
		return false
	}
	for _, id := range ids {
		if _, ok := m[id]; !ok {
			return false
		}
	}
	return true
}
