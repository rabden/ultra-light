package core

import (
	"slices"
	"testing"
)

func score(t *testing.T, q, name string) int {
	t.Helper()
	m, ok := Score(q, name)
	if !ok {
		t.Fatalf("%q in %q: no match", q, name)
	}
	return m.Score
}

func TestTiersRank(t *testing.T) {
	if !(score(t, "safari", "Safari") > score(t, "saf", "Safari")) {
		t.Error("whole name must beat its start")
	}
	if !(score(t, "saf", "Safari") > score(t, "code", "Visual Studio Code")) {
		t.Error("start must beat a word's start")
	}
	if !(score(t, "code", "Visual Studio Code") > score(t, "vsc", "Visual Studio Code")) {
		t.Error("word start must beat initials")
	}
	if !(score(t, "vsc", "Visual Studio Code") > score(t, "tudi", "Visual Studio Code")) {
		t.Error("initials must beat substring")
	}
	if !(score(t, "tudi", "Visual Studio Code") > score(t, "vsco", "Visual Studio Code")) {
		t.Error("substring must beat subsequence")
	}
	if _, ok := Score("xyz", "Safari"); ok {
		t.Error("xyz matched Safari")
	}
	if _, ok := Score("", "Safari"); ok {
		t.Error("empty query matched")
	}
}

func TestInitialsAndCapitals(t *testing.T) {
	m, _ := Score("ft", "FaceTime")
	if !slices.Equal(m.Hits, []int{0, 4}) {
		t.Errorf("ft: %v", m.Hits)
	}
	s, _ := Score("ss", "System Settings")
	if !slices.Equal(s.Hits, []int{0, 7}) {
		t.Errorf("ss: %v", s.Hits)
	}
}

func TestTypos(t *testing.T) {
	for _, c := range [][2]string{
		{"chorme", "Google Chrome"}, {"notse", "Notes"}, {"safaro", "Safari"},
		{"safarii", "Safari"}, {"calandr", "Calendar"}, {"sistem", "System Settings"},
	} {
		if _, ok := Score(c[0], c[1]); !ok {
			t.Errorf("%q should find %q", c[0], c[1])
		}
	}
	m, _ := Score("chorme", "Google Chrome")
	if !slices.Equal(m.Hits, []int{7, 8, 9, 10, 11, 12}) {
		t.Errorf("chorme lit %v", m.Hits)
	}
	if !(score(t, "notse", "Notes") < score(t, "nts", "Notes")) {
		t.Error("a typo must rank below a subsequence")
	}
	for _, c := range [][2]string{{"zebra", "Safari"}, {"sax", "Safari"}, {"mapz", "Mail"}} {
		if _, ok := Score(c[0], c[1]); ok {
			t.Errorf("%q should not find %q", c[0], c[1])
		}
	}
}

func TestSubsequenceHighlight(t *testing.T) {
	m, _ := Score("sfr", "Safari")
	if !slices.Equal(m.Hits, []int{0, 2, 4}) {
		t.Errorf("sfr: %v", m.Hits)
	}
}

func TestRank(t *testing.T) {
	names := []string{"Notes Helper", "Notes", "Numbers", "Keynote"}
	out := Rank("no", names, nil, 3)
	if len(out) != 3 || out[0].Index != 1 || out[2].Index != 3 {
		t.Fatalf("rank: %+v", out)
	}
	// The helper, opened once, climbs past Notes.
	out = Rank("no", names, []uint16{1, 0, 0, 0}, 3)
	if len(out) != 3 || out[0].Index != 0 {
		t.Fatalf("boosted: %+v", out)
	}
	// Only as many as there's room for: the worst one is the one dropped.
	two := Rank("no", names, nil, 2)
	if len(two) != 2 || two[1].Index != 0 {
		t.Fatalf("two: %+v", two)
	}
}
