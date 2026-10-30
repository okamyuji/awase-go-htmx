package domain

// CandidateTally 候補ごとの出欠の人数です。Best は○の人数が最多の候補で真になります。
type CandidateTally struct {
	Candidate Candidate
	Yes       int
	Maybe     int
	No        int
	Best      bool
}

// Tally 候補ごとに出欠を数えます。出欠のない回答は数えません。
func Tally(cands []Candidate, answers []Answer) []CandidateTally {
	out := make([]CandidateTally, len(cands))
	maxYes := 0
	for i, c := range cands {
		t := CandidateTally{Candidate: c}
		for _, a := range answers {
			t.add(a.Choices, c.ID)
		}
		out[i] = t
		maxYes = max(maxYes, t.Yes)
	}
	for i := range out {
		out[i].Best = maxYes > 0 && out[i].Yes == maxYes
	}
	return out
}

func (t *CandidateTally) add(choices map[int64]Choice, id int64) {
	c, ok := choices[id]
	if !ok {
		return
	}
	switch c {
	case Yes:
		t.Yes++
	case Maybe:
		t.Maybe++
	case No:
		t.No++
	}
}
