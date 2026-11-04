package view

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/rabden/ultra-light/internal/core"
)

func TestSearchFindsMovesAnswersHandsOff(t *testing.T) {
	r := newRig(t)
	r.snap("empty")
	if !r.tt.HasText("Suggestions") || !r.tt.HasText("Firefox") {
		t.Fatalf("empty query suggests the starters: %q", r.tt.Texts())
	}
	r.typ("saf")
	r.snap("saf")
	rows, sel := r.p.Rows()
	if rows[0].Kind != core.KindApp || r.p.Index.Entries[rows[0].App].Name != "Safari" || sel != 0 {
		t.Fatalf("saf: %+v", rows)
	}
	if !r.tt.HasText("ari") {
		t.Errorf("ghost after saf: %q", r.tt.Texts())
	}
	if !r.tt.HasText("Open Application") || !r.tt.HasText("Search Google for “saf”") {
		t.Errorf("footer and web rows: %q", r.tt.Texts())
	}
	// Down moves, wrapping; Up back.
	r.tt.Key(0, ui.KeyDown)
	if _, sel := r.p.Rows(); sel != 1 {
		t.Errorf("down: %d", sel)
	}
	r.tt.Key(0, ui.KeyUp)
	r.tt.Key(0, ui.KeyUp)
	rows, sel = r.p.Rows()
	if sel != len(rows)-1 {
		t.Errorf("up wraps: %d", sel)
	}
	r.tt.Key(ui.Ctrl, ui.KeyN)
	if _, sel := r.p.Rows(); sel != 0 {
		t.Errorf("ctrl+n wraps: %d", sel)
	}
	// Tab takes the ghost.
	r.tt.Key(0, ui.KeyTab)
	r.tt.Advance(time.Second)
	if r.p.Query() != "safari" {
		t.Errorf("tab: %q", r.p.Query())
	}
	// Return opens Safari, counts it, hides.
	r.tt.Key(0, ui.KeyEnter)
	if len(r.h.opened) != 1 || r.h.opened[0] != "Safari" || r.h.hidden != 1 || !strings.Contains(r.h.launches, "Safari\t1") {
		t.Fatalf("open: %+v", r.h)
	}
}

func TestMathCopies(t *testing.T) {
	r := newRig(t)
	r.typ("2^10 * 3")
	r.snap("calc")
	if !r.tt.HasText("3,072") || !r.tt.HasText("Result · 0xC00") || !r.tt.HasText("Copy Answer") {
		t.Fatalf("calc: %q", r.tt.Texts())
	}
	r.tt.Key(0, ui.KeyEnter)
	r.tt.Frame()
	if r.h.copied != "3072" || !r.tt.HasText("Copied") || r.h.hidden != 0 {
		t.Fatalf("copy: %q %q", r.h.copied, r.tt.Texts())
	}
	r.now = r.now.Add(1500 * time.Millisecond)
	r.tt.Advance(1500 * time.Millisecond)
	if r.tt.HasText("Copied") {
		t.Error("Copied must go after 1.4 s")
	}
}

func TestEscapeClearsThenHides(t *testing.T) {
	r := newRig(t)
	r.typ("term")
	r.tt.Key(0, ui.KeyEscape)
	r.tt.Frame()
	if r.p.Query() != "" || r.h.hidden != 0 {
		t.Fatalf("first esc clears: %q %d", r.p.Query(), r.h.hidden)
	}
	r.tt.Key(0, ui.KeyEscape)
	if r.h.hidden != 1 {
		t.Fatal("second esc hides")
	}
}

func TestCompletionsKeepSelection(t *testing.T) {
	r := newRig(t)
	r.typ("liquid gl")
	if len(r.h.asked) == 0 || r.h.asked[len(r.h.asked)-1] != "liquid gl" {
		t.Fatalf("asked %q", r.h.asked)
	}
	r.p.Answered("liquid gl", []string{"liquid glass", "liquid glue"})
	r.tt.Frame()
	r.snap("completions")
	rows, sel := r.p.Rows()
	if rows[sel].Kind != core.KindSearchWeb {
		t.Fatalf("selection moved off the search: %+v %d", rows, sel)
	}
	if !r.tt.HasText("liquid glass") || !r.tt.HasText("ass") {
		t.Errorf("completions and ghost: %q", r.tt.Texts())
	}
	// Ctrl+2: the first completion, searched in the panel.
	r.tt.Key(ui.Ctrl, ui.Key2)
	r.tt.Advance(time.Second)
	if !r.p.Previewing() || r.h.page != "https://www.google.com/search?q=liquid+glass" || r.p.Query() != "liquid glass" {
		t.Fatalf("page: %v %q %q", r.p.Previewing(), r.h.page, r.p.Query())
	}
	if r.p.Height() != core.MaxHeight || !r.tt.HasText("Searching Google…") || !r.tt.HasText("Open in Browser") {
		t.Errorf("page look: %q", r.tt.Texts())
	}
	r.snap("page")
	// Esc back to the rows.
	r.tt.Key(0, ui.KeyEscape)
	if r.p.Previewing() || r.h.page != "" {
		t.Fatal("esc back")
	}
	// Ctrl+↩ on the Google row: the browser, the panel hides.
	rows, sel = r.p.Rows()
	if rows[sel].Kind != core.KindSearchWeb {
		t.Fatalf("on %+v", rows[sel])
	}
	r.tt.Key(ui.Ctrl, ui.KeyEnter)
	if len(r.h.urls) != 1 || r.h.urls[0] != "https://www.google.com/search?q=liquid+glass" || r.h.hidden != 1 {
		t.Fatalf("handed off: %q %d", r.h.urls, r.h.hidden)
	}
}

func TestTypingReturnsFromPage(t *testing.T) {
	r := newRig(t)
	r.typ("weather")
	r.tt.Key(0, ui.KeyEnter)
	r.tt.Frame()
	if !r.p.Previewing() {
		t.Fatal("return on the Google row shows the page")
	}
	r.typ("s")
	if r.p.Previewing() {
		t.Fatal("typing returns to the rows")
	}
	// Ctrl+↩ on a page opens what it shows in the browser.
	r.p.Search("weather", true)
	r.h.page = "https://www.google.com/search?q=weather&start=10"
	r.tt.Key(ui.Ctrl, ui.KeyEnter)
	if r.h.urls[len(r.h.urls)-1] != "https://www.google.com/search?q=weather&start=10" {
		t.Errorf("open out: %q", r.h.urls)
	}
}

func TestSearchesOut(t *testing.T) {
	r := newRig(t)
	r.p.SearchesOut = true
	r.typ("weather")
	r.tt.Key(0, ui.KeyEnter)
	if r.p.Previewing() || len(r.h.urls) != 1 {
		t.Fatalf("return goes to the browser: %q", r.h.urls)
	}
	r.p.Reveal()
	r.tt.Frame()
	r.tt.Key(ui.Ctrl, ui.KeyEnter)
	if !r.p.Previewing() {
		t.Fatal("ctrl+return shows it in the panel")
	}
}

func TestMaxHeight(t *testing.T) {
	r := newRig(t)
	r.typ("e")
	r.p.Answered("e", []string{"ebay", "espn", "email", "excel"})
	r.tt.Advance(time.Second)
	if h := r.h.heights[len(r.h.heights)-1]; h > core.MaxHeight+0.5 {
		t.Errorf("height %v", h)
	}
	r.snap("full")
}

func TestCtrlLabels(t *testing.T) {
	r := newRig(t)
	r.typ("s")
	if !r.tt.HasText("Ctrl+2") {
		t.Errorf("labels: %q", r.tt.Texts())
	}
}
