package core

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestEligible(t *testing.T) {
	if !Eligible("liquid glass") || !Eligible("  zig  ") || Eligible("z") {
		t.Error("basic eligibility")
	}
	for _, q := range []string{"github.com", "www.zig", "localhost 3000", "me@x", "a/b", "http://x", `c:\x`, "tab\there", strings.Repeat("a", SuggestMaxLen+1)} {
		if Eligible(q) {
			t.Errorf("%q must not go out", q)
		}
	}
}

func TestSuggestURL(t *testing.T) {
	if got := SuggestURL(" liquid gl & co "); got != "https://suggestqueries.google.com/complete/search?client=firefox&q=liquid+gl+%26+co" {
		t.Error(got)
	}
}

func TestDecode(t *testing.T) {
	body := []byte(`["liquid gl",["liquid glass","Liquid  Gl","liquid glass","liquid glue","liquid glass apple","liquid glass ui","liquid glucose"],[],{"google:suggestsubtypes":[[512]]}]`)
	l, ok := DecodeSuggestions(body, "Liquid  GL")
	if !ok || len(l) != 4 || l[0] != "liquid glass" || l[3] != "liquid glass ui" {
		t.Fatalf("decode: %v %q", ok, l)
	}
	if _, ok := DecodeSuggestions(body, "liquid"); ok {
		t.Error("answer for other words accepted")
	}
	for _, b := range []string{"<html>", `["x"]`, `["x", null]`} {
		if _, ok := DecodeSuggestions([]byte(b), "x"); ok {
			t.Errorf("%q accepted", b)
		}
	}
	l, ok = DecodeSuggestions([]byte(`["caf",["caf\u00e9"]]`), "caf")
	if !ok || l[0] != "café" {
		t.Errorf("escapes: %q", l)
	}
}

func TestCache(t *testing.T) {
	var c SuggestCache
	said := []string{"liquid glass", "liquid glue", "liquid glass apple"}
	c.Put("liquid gl", said)
	if out, ok := c.Lookup("Liquid GL"); !ok || len(out) != 3 {
		t.Fatalf("exact: %v %q", ok, out)
	}
	out, ok := c.Lookup("liquid gla")
	if ok || !slices.Equal(out, []string{"liquid glass", "liquid glass apple"}) {
		t.Fatalf("prefix: %v %q", ok, out)
	}
	if out, ok := c.Lookup("pizza"); ok || len(out) != 0 {
		t.Fatalf("pizza: %q", out)
	}
	for i := 0; i < cacheSlots; i++ {
		c.Put(fmt.Sprintf("q%d", i), said)
	}
	if out, ok := c.Lookup("liquid gl"); ok && len(out) != 0 {
		t.Error("oldest answer must go")
	}
	if _, ok := c.Lookup("q63"); !ok {
		t.Error("newest answer kept")
	}
}

func TestGhost(t *testing.T) {
	names := []string{"Safari", "System Settings"}
	var r Results
	r.Build("saf", names, nil, nil, nil)
	name := func(i int) string { return names[i] }
	if g := Ghost("saf", false, r.Rows, name, nil); g != "ari" {
		t.Errorf("saf → %q", g)
	}
	if g := Ghost("saf", true, r.Rows, name, nil); g != "" {
		t.Errorf("revealed → %q", g)
	}
	web := []string{"liquid glass"}
	r.Build("liquid gl", names, nil, nil, web)
	if g := Ghost("liquid gl", false, r.Rows, name, web); g != "ass" {
		t.Errorf("liquid gl → %q", g)
	}
	r.Build("safari", names, nil, nil, nil)
	if g := Ghost("safari", false, r.Rows, name, nil); g != "" {
		t.Errorf("whole name → %q", g)
	}
}

func TestHeights(t *testing.T) {
	if MaxHeight != 674 {
		t.Fatalf("max height %d", MaxHeight)
	}
	var r Results
	if h := SearchHeight(&r); h != 97 {
		t.Errorf("empty %v", h)
	}
	r.Build("2^10 * 3", nil, nil, nil, nil)
	if h := SearchHeight(&r); h != 97+1+12+92+28 {
		t.Errorf("calc %v", h)
	}
	names := []string{"Safari", "Mail", "Maps", "Pages", "Calendar", "Calculator", "FaceTime"}
	r.Build("a", names, nil, nil, []string{"a1", "a2", "a3", "a4"})
	if h := SearchHeight(&r); h != MaxHeight {
		t.Errorf("full %v", h)
	}
	if y, h := RowTop(&r, 6); y != 28+6*40+28 || h != 40 {
		t.Errorf("row 6 at %v %v", y, h)
	}
	if WebAreaHeight(674) != 576 {
		t.Error("web area")
	}
	if !slices.Equal(HighlightRuns([]int{0, 1, 2, 5, 7, 8}), [][2]int{{0, 3}, {5, 6}, {7, 9}}) {
		t.Error("runs")
	}
}
