package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// 入力の上限です。値は要件定義書の3章に従います。
const (
	MaxTitle      = 100
	MaxMemo       = 1000
	MaxLabel      = 100
	MaxCandidates = 50
)

// Candidate イベントの候補日時です。
type Candidate struct {
	ID       int64
	Position int
	Label    string
}

// Event 日程を決めたい集まりです。
type Event struct {
	ID         string
	Title      string
	Memo       string
	Candidates []Candidate
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// EventSummary イベント一覧の1行です。
type EventSummary struct {
	ID             string
	Title          string
	CandidateCount int
	AnswerCount    int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// EventDraft 検証済みの新しいイベントの入力です。
type EventDraft struct {
	Title  string
	Memo   string
	Labels []string
}

// ParseCandidates 候補の入力を改行で分け、前後の空白を除き、空行を捨てます。
func ParseCandidates(text string) []string {
	var out []string
	for line := range strings.Lines(text) {
		if s := strings.TrimSpace(line); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// NewEventDraft タイトル、メモ、候補を検証し、空白を除いた値を返します。
func NewEventDraft(title, memo, candidates string) (EventDraft, error) {
	ve := ValidationError{}
	d := EventDraft{
		Title:  checkLen(ve, "title", title, 1, MaxTitle),
		Memo:   checkLen(ve, "memo", memo, 0, MaxMemo),
		Labels: ParseCandidates(candidates),
	}
	checkLabels(ve, d.Labels, 0)
	return d, ve.orNil()
}

// NormalizeMeta タイトルとメモを検証し、空白を除いた値を返します。
func NormalizeMeta(title, memo string) (string, string, error) {
	ve := ValidationError{}
	t := checkLen(ve, "title", title, 1, MaxTitle)
	m := checkLen(ve, "memo", memo, 0, MaxMemo)
	return t, m, ve.orNil()
}

// NewCandidateLabels 追加する候補を検証します。existing は既存の候補の件数です。
func NewCandidateLabels(text string, existing int) ([]string, error) {
	ve := ValidationError{}
	labels := ParseCandidates(text)
	checkLabels(ve, labels, existing)
	return labels, ve.orNil()
}

// CheckCandidateDeletable 候補が count 件のイベントから1件を削除できるかを確かめます。
func CheckCandidateDeletable(count int) error {
	if count <= 1 {
		return ValidationError{"candidates": "最後の候補は削除できません"}
	}
	return nil
}

func checkLabels(ve ValidationError, labels []string, existing int) {
	total := existing + len(labels)
	if len(labels) == 0 || total > MaxCandidates {
		ve["candidates"] = fmt.Sprintf("候補は合計1〜%d件で入力してください", MaxCandidates)
		return
	}
	for _, l := range labels {
		if utf8.RuneCountInString(l) > MaxLabel {
			ve["candidates"] = fmt.Sprintf("候補は1件%d文字までです", MaxLabel)
			return
		}
	}
}
