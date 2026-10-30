package domain

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func fieldErr(t *testing.T, err error, field string) {
	t.Helper()
	ve, ok := errors.AsType[ValidationError](err)
	if !ok {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if _, ok := ve[field]; !ok {
		t.Fatalf("ValidationError = %v, want field %q", ve, field)
	}
}

func TestParseCandidatesSplitsLFAndCRLFAndDropsBlankLines(t *testing.T) {
	got := ParseCandidates("  11/3 19:00 \r\n\r\n11/4\n   \n11/5\r\n")

	want := []string{"11/3 19:00", "11/4", "11/5"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestNewEventDraftTrimsAndKeepsValues(t *testing.T) {
	d, err := NewEventDraft("  飲み会  ", "  駅前  ", "11/3\n11/4")

	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "飲み会" || d.Memo != "駅前" || !slices.Equal(d.Labels, []string{"11/3", "11/4"}) {
		t.Fatalf("draft = %+v", d)
	}
}

func TestNewEventDraftBoundaries(t *testing.T) {
	tests := []struct {
		name, title, memo, cands string
		field                    string
	}{
		{"タイトル100文字は通る", strings.Repeat("あ", 100), "", "a", ""},
		{"タイトル101文字は失敗", strings.Repeat("あ", 101), "", "a", "title"},
		{"空白だけのタイトルは失敗", "   ", "", "a", "title"},
		{"メモ1000文字は通る", "t", strings.Repeat("め", 1000), "a", ""},
		{"メモ1001文字は失敗", "t", strings.Repeat("め", 1001), "a", "memo"},
		{"候補100文字は通る", "t", "", strings.Repeat("こ", 100), ""},
		{"候補101文字は失敗", "t", "", strings.Repeat("こ", 101), "candidates"},
		{"候補0件は失敗", "t", "", "\n \r\n", "candidates"},
		{"候補50件は通る", "t", "", strings.Repeat("x\n", 50), ""},
		{"候補51件は失敗", "t", "", strings.Repeat("x\n", 51), "candidates"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEventDraft(tt.title, tt.memo, tt.cands)
			if tt.field == "" {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				return
			}
			fieldErr(t, err, tt.field)
		})
	}
}

func TestNewEventDraftReportsAllFields(t *testing.T) {
	_, err := NewEventDraft("", strings.Repeat("m", 1001), "")

	ve, ok := errors.AsType[ValidationError](err)
	if !ok || len(ve) != 3 {
		t.Fatalf("err = %v, want 3 fields", err)
	}
	if ve.Error() == "" {
		t.Fatal("Error() is empty")
	}
}

func TestNormalizeMeta(t *testing.T) {
	title, memo, err := NormalizeMeta(" 会 ", " m ")
	if err != nil || title != "会" || memo != "m" {
		t.Fatalf("got %q %q %v", title, memo, err)
	}

	_, _, err = NormalizeMeta("", "")
	fieldErr(t, err, "title")
}

func TestNewCandidateLabelsChecksTotal(t *testing.T) {
	got, err := NewCandidateLabels("a\nb", 48)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %q %v, want 2 labels", got, err)
	}

	_, err = NewCandidateLabels("a\nb", 49)
	fieldErr(t, err, "candidates")

	_, err = NewCandidateLabels(" \n", 3)
	fieldErr(t, err, "candidates")

	_, err = NewCandidateLabels(strings.Repeat("z", 101), 0)
	fieldErr(t, err, "candidates")
}

func TestCheckCandidateDeletable(t *testing.T) {
	if err := CheckCandidateDeletable(2); err != nil {
		t.Fatalf("2件: %v", err)
	}
	fieldErr(t, CheckCandidateDeletable(1), "candidates")
}
