package domain

import "testing"

func TestTallyCountsAndMarksAllBestCandidates(t *testing.T) {
	cands := []Candidate{{ID: 1, Label: "a"}, {ID: 2, Label: "b"}, {ID: 3, Label: "c"}}
	answers := []Answer{
		{Choices: map[int64]Choice{1: Yes, 2: Yes, 3: No}},
		{Choices: map[int64]Choice{1: Yes, 2: Yes, 3: Maybe}},
		{Choices: map[int64]Choice{1: Maybe, 2: No}},
	}

	got := Tally(cands, answers)

	want := []CandidateTally{
		{Candidate: cands[0], Yes: 2, Maybe: 1, No: 0, Best: true},
		{Candidate: cands[1], Yes: 2, Maybe: 0, No: 1, Best: true},
		{Candidate: cands[2], Yes: 0, Maybe: 1, No: 1, Best: false},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestTallyMarksNothingWhenNoYes(t *testing.T) {
	got := Tally([]Candidate{{ID: 1}}, []Answer{{Choices: map[int64]Choice{1: No}}})

	if got[0].Best || got[0].No != 1 {
		t.Fatalf("got %+v", got[0])
	}
}

func TestTallyEmptyAnswers(t *testing.T) {
	got := Tally([]Candidate{{ID: 1}}, nil)

	if len(got) != 1 || got[0].Best || got[0].Yes != 0 {
		t.Fatalf("got %+v", got)
	}
}
