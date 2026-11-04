package view

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/rabden/ultra-light/internal/core"
	"github.com/rabden/ultra-light/internal/hotkey"
)

// steps records what the welcome asked the app (tests.zig's Steps).
type steps struct {
	applied []hotkey.Choice
	held    map[hotkey.Choice]KeyState
	login   bool
	done    int
	taken   map[hotkey.Choice]string
	took    []hotkey.Choice
}

func (s *steps) KeyTaken(c hotkey.Choice) (string, bool) { return s.taken[c], true }
func (s *steps) TakeKey(c hotkey.Choice)                 { s.took = append(s.took, c) }

func (s *steps) Apply(c hotkey.Choice)             { s.applied = append(s.applied, c) }
func (s *steps) KeyState(c hotkey.Choice) KeyState { return s.held[c] }
func (s *steps) Done()                             { s.done++ }
func (s *steps) Login(set *bool) bool {
	if set != nil {
		s.login = *set
	}
	return s.login
}

func newWelcomeRig(t *testing.T, st *steps) *rig {
	r := newRig(t)
	r.p.Welcome.Hooks = st
	r.p.Welcome.Start(hotkey.CmdSpace)
	r.settle()
	return r
}

func TestWelcome(t *testing.T) {
	st := &steps{held: map[hotkey.Choice]KeyState{hotkey.CmdSpace: KeyHeld}}
	r := newWelcomeRig(t, st)
	w := &r.p.Welcome
	if !r.tt.HasText("Welcome to Nanolight") || !r.tt.HasText("1 of 3") {
		t.Fatal("step 1")
	}
	// Who holds Alt Space; going on anyway is said as such.
	if !r.tt.HasText("Alt Space is held by another app or your desktop.") || !r.tt.HasText("Use Alt Space anyway") ||
		!r.tt.HasText("Pick another key above.") {
		t.Fatal("the clash")
	}
	if r.p.Height() != 360 {
		t.Fatalf("held height %v", r.p.Height())
	}
	r.snap("welcome-1-held")
	r.tt.Key(0, ui.KeyRight)
	r.settle()
	if !r.tt.HasText("Ctrl Alt Space is free on this PC.") || !r.tt.HasText("Continue") || r.p.Height() != 330 {
		t.Fatal("free")
	}
	r.snap("welcome-1")
	r.tt.Key(0, ui.KeyEnter)
	r.settle()
	if len(st.applied) != 1 || st.applied[0] != hotkey.OptSpace {
		t.Fatalf("applied %v", st.applied)
	}
	if !r.tt.HasText("Press Ctrl Alt Space now") || !r.tt.HasText("2 of 3") || r.p.Height() != 318 {
		t.Fatal("step 2")
	}
	r.snap("welcome-2")
	// Return never moves it on: the key closing the panel, then opening it.
	r.tt.Key(0, ui.KeyEnter)
	w.Pressed(false)
	r.settle()
	if w.Step() != StepTry {
		t.Fatal("still step 2")
	}
	w.Pressed(true)
	w.Pressed(false)
	r.tt.Frame()
	r.tt.Advance(100 * time.Millisecond)
	r.now = r.now.Add(100 * time.Millisecond)
	r.tt.Frame()
	if w.Step() != StepTry || !r.tt.HasText("Back again") {
		t.Fatal("both ticks show for a moment")
	}
	r.now = r.now.Add(time.Second)
	r.settle()
	r.settle()
	if !r.tt.HasText("You're set.") || r.p.Height() != 396 {
		t.Fatalf("step 3: %v %v", w.Step(), r.p.Height())
	}
	r.snap("welcome-3")
	// The login switch.
	if err := r.tt.Click("Open Nanolight when you log in"); err != nil {
		t.Fatal(err)
	}
	if !st.login {
		t.Fatal("login on")
	}
	// Ctrl+2: the math example, run in the launcher.
	r.tt.Key(ui.Ctrl, ui.Key2)
	r.settle()
	if st.done != 1 || w.Active || !r.tt.HasText("3,072") || r.p.Query() != "2^10 * 3" {
		t.Fatalf("done %d active %v query %q", st.done, w.Active, r.p.Query())
	}
	if !r.tt.Focused("Search") {
		t.Fatal("the field has the keys")
	}

	// Esc skips from the first step, keeping the key shown there.
	st.applied = nil
	r.p.Welcome.Start(hotkey.CtrlSpace)
	r.settle()
	r.tt.Key(0, ui.KeyEscape)
	r.settle()
	if st.done != 2 || w.Active || len(st.applied) != 1 || st.applied[0] != hotkey.CtrlSpace {
		t.Fatalf("skip: done %d applied %v", st.done, st.applied)
	}
	if r.p.Height() > core.MaxHeight {
		t.Fatal("too tall")
	}
}

// Where no key can be taken (Wayland without the portal), the welcome says
// how to make a desktop shortcut, and nanolight --toggle counts as the key.
func TestWelcomeWithoutKeys(t *testing.T) {
	st := &steps{held: map[hotkey.Choice]KeyState{}}
	for _, c := range hotkey.All {
		st.held[c] = KeyUnusable
	}
	r := newWelcomeRig(t, st)
	w := &r.p.Welcome
	if !r.tt.HasText("Alt Space can't be used here.") || !r.tt.HasText("nanolight --toggle") || !r.tt.HasText("Continue") {
		t.Fatal("the --toggle way")
	}
	if r.p.Height() != 330+unusableExtra {
		t.Fatalf("height %v", r.p.Height())
	}
	r.snap("welcome-1-toggle")
	if err := r.tt.Click("Copy Command"); err != nil {
		t.Fatal(err)
	}
	if r.h.copied != "nanolight --toggle" {
		t.Fatal("copied")
	}
	r.tt.Key(0, ui.KeyEnter)
	r.settle()
	w.Pressed(true)
	w.Pressed(false)
	r.tt.Frame()
	r.now = r.now.Add(time.Second)
	r.settle()
	r.settle()
	if w.Step() != StepDone {
		t.Fatal("two --toggle runs finish step 2")
	}
	r.tt.Key(0, ui.KeyEnter)
	r.settle()
	if w.Active || r.p.Query() != "" {
		t.Fatal("Return starts searching")
	}
}

// On Wayland with the portal, the desktop holds the key, which is fine.
func TestWelcomeDesktopKey(t *testing.T) {
	st := &steps{held: map[hotkey.Choice]KeyState{hotkey.CmdSpace: KeyDesktop}}
	r := newWelcomeRig(t, st)
	if !r.tt.HasText("Your desktop sets this shortcut, in its keyboard settings.") || !r.tt.HasText("Continue") {
		t.Fatal("desktop")
	}
}

func TestWelcomeTakenKey(t *testing.T) {
	st := &steps{taken: map[hotkey.Choice]string{hotkey.CmdSpace: "opens GNOME's window menu"},
		held: map[hotkey.Choice]KeyState{hotkey.CmdSpace: KeyTaken}}
	r := newWelcomeRig(t, st)
	r.snap("welcome-taken")
	if !r.tt.HasText("Alt Space opens GNOME's window menu.") || !r.tt.HasText("Use Alt Space anyway") {
		t.Fatal("says what holds it, and offers it anyway")
	}
	// Return takes it over, rather than using a key that won't work.
	r.tt.Key(0, ui.KeyEnter)
	r.settle()
	if len(st.took) != 1 || len(st.applied) != 0 {
		t.Fatalf("took %v applied %v", st.took, st.applied)
	}
}
