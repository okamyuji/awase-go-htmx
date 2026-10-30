package domain

import (
	"strings"
	"testing"
)

func TestChoiceSymbolAndParse(t *testing.T) {
	tests := []struct {
		in   string
		want Choice
		sym  string
	}{
		{"0", No, "×"},
		{"1", Maybe, "△"},
		{"2", Yes, "○"},
	}
	for _, tt := range tests {
		c, ok := ParseChoice(tt.in)
		if !ok || c != tt.want || c.Symbol() != tt.sym {
			t.Fatalf("ParseChoice(%q) = %v %v %q", tt.in, c, ok, c.Symbol())
		}
	}
	for _, bad := range []string{"", "3", "-1", "yes"} {
		if _, ok := ParseChoice(bad); ok {
			t.Fatalf("ParseChoice(%q) ok, want false", bad)
		}
	}
	if Choice(9).Symbol() != "−" {
		t.Fatalf("unknown symbol = %q", Choice(9).Symbol())
	}
}

func TestNewAnswerDraftAcceptsExactCandidateSet(t *testing.T) {
	d, err := NewAnswerDraft("  太郎 ", " よろしく ", map[int64]Choice{1: Yes, 2: No}, []int64{2, 1})

	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "太郎" || d.Comment != "よろしく" || d.Choices[1] != Yes || d.Choices[2] != No {
		t.Fatalf("draft = %+v", d)
	}
}

func TestNewAnswerDraftRejects(t *testing.T) {
	ids := []int64{1, 2}
	tests := []struct {
		name          string
		user, comment string
		choices       map[int64]Choice
		field         string
	}{
		{"候補が欠けている", "a", "", map[int64]Choice{1: Yes}, "choices"},
		{"余分な候補がある", "a", "", map[int64]Choice{1: Yes, 2: No, 3: Maybe}, "choices"},
		{"別の候補にすり替わっている", "a", "", map[int64]Choice{1: Yes, 9: No}, "choices"},
		{"名前が空", " ", "", map[int64]Choice{1: Yes, 2: No}, "name"},
		{"名前31文字", strings.Repeat("な", 31), "", map[int64]Choice{1: Yes, 2: No}, "name"},
		{"コメント201文字", "a", strings.Repeat("こ", 201), map[int64]Choice{1: Yes, 2: No}, "comment"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAnswerDraft(tt.user, tt.comment, tt.choices, ids)
			fieldErr(t, err, tt.field)
		})
	}
}

func TestNewAnswerDraftAcceptsMaxLengths(t *testing.T) {
	_, err := NewAnswerDraft(strings.Repeat("な", 30), strings.Repeat("こ", 200), map[int64]Choice{1: Maybe}, []int64{1})
	if err != nil {
		t.Fatal(err)
	}
}
