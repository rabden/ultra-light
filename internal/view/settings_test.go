package view

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/rabden/ultra-light/internal/core"
	"github.com/rabden/ultra-light/internal/hotkey"
)

// prefs records what Settings asked the app (tests.zig's Prefs).
type prefs struct {
	choice      hotkey.Choice
	applied     []hotkey.Choice
	problem     string
	login       bool
	tray, can   bool
	suggestSets int
	searchesOut *bool
	account     AccountState
	signIns     int
	signOuts    int
	terms       int
	checks      int
	restarts    int
	welcomes    int
	quits       int
	ready       string
}

func (f *prefs) Choice() hotkey.Choice { return f.choice }
func (f *prefs) Apply(c hotkey.Choice) { f.choice = c; f.applied = append(f.applied, c) }
func (f *prefs) KeyProblem() string    { return f.problem }
func (f *prefs) Login(set *bool) bool {
	if set != nil {
		f.login = *set
	}
	return f.login
}
func (f *prefs) Suggest(bool)        { f.suggestSets++ }
func (f *prefs) SearchesOut(on bool) { f.searchesOut = &on }
func (f *prefs) Tray(set *bool) (bool, bool) {
	if set != nil {
		f.tray = *set
	}
	return f.tray, f.can
}
func (f *prefs) Account() AccountState { return f.account }
func (f *prefs) Email() string         { return "ring@example.com" }
func (f *prefs) SignIn()               { f.signIns++ }
func (f *prefs) SignOut()              { f.signOuts++ }
func (f *prefs) Terms()                { f.terms++ }
func (f *prefs) Version() string       { return "2.0.1" }
func (f *prefs) Ready() string         { return f.ready }
func (f *prefs) Updatable() bool       { return true }
func (f *prefs) Check()                { f.checks++ }
func (f *prefs) Restart()              { f.restarts++ }
func (f *prefs) Welcome()              { f.welcomes++ }
func (f *prefs) Quit()                 { f.quits++ }

func newPrefsRig(t *testing.T) (*rig, *prefs) {
	r := newRig(t)
	f := &prefs{tray: true, can: true}
	r.p.Settings.Hooks = f
	return r, f
}

func (r *rig) settle() { r.tt.Frame(); r.tt.Advance(time.Second) }

func TestSettingsOpeners(t *testing.T) {
	r, f := newPrefsRig(t)
	// The gear is in the footer, named (typed first, so no app is).
	r.typ("fire")
	if err := r.tt.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	r.settle()
	if !r.p.Settings.Open || !r.tt.HasText("Open Nanolight with") {
		t.Fatal("the gear opens Settings")
	}
	r.tt.Key(0, ui.KeyEscape)
	r.settle()
	if r.p.Settings.Open {
		t.Fatal("Esc goes back")
	}

	// Ctrl+, opens it; Esc goes back to search, the field with the keys.
	r.tt.Key(ui.Ctrl, ui.KeyComma)
	r.settle()
	if !r.p.Settings.Open {
		t.Fatal("Ctrl+, opens Settings")
	}
	for _, s := range []string{"Google Suggestions", "Open Searches in Browser", "This PC", "Open at Login", "Show in Tray",
		"Sync Across Computers", "Sign In with Codegraff", "Terms & Privacy", "Nanolight 2.0.1", "Check for Updates",
		"Show Welcome Again", "Quit Nanolight", "Alt Space", "Ctrl Shift Space", "Back"} {
		if !r.tt.HasText(s) {
			t.Errorf("Settings lacks %q", s)
		}
	}
	r.snap("settings")
	r.tt.Key(0, ui.KeyEscape)
	r.settle()
	if r.p.Settings.Open || !r.tt.HasText("Firefox") {
		t.Fatal("back to search")
	}
	r.tt.Key(0, ui.KeyEscape)
	r.settle()

	// Typing "settings" lists the row; Return opens it.
	r.typ("settings")
	if !r.tt.HasText("Nanolight Settings") {
		t.Fatal("the settings row")
	}
	rows, _ := r.p.Rows()
	at := -1
	for i, row := range rows {
		if row.Kind == core.KindSettings {
			at = i
		}
	}
	if at < 0 {
		t.Fatal("no settings row")
	}
	for range at {
		r.tt.Key(0, ui.KeyDown)
	}
	r.tt.Key(0, ui.KeyEnter)
	r.settle()
	if !r.p.Settings.Open {
		t.Fatal("Return on the row opens Settings")
	}

	// → picks the next shortcut and tells the host; ← back.
	r.tt.Key(0, ui.KeyRight)
	if len(f.applied) != 1 || f.applied[0] != hotkey.OptSpace {
		t.Fatalf("→ applied %v", f.applied)
	}
	r.tt.Key(0, ui.KeyLeft)
	if f.choice != hotkey.CmdSpace {
		t.Fatalf("← applied %v", f.choice)
	}
	// Return on the shortcut row steps on (a PC has no recording).
	r.tt.Key(0, ui.KeyEnter)
	if f.choice != hotkey.OptSpace {
		t.Fatalf("Return applied %v", f.choice)
	}

	// ↓ onto Google Suggestions, Return flips it; ←/→ there do nothing.
	if !r.p.SuggestOn {
		t.Fatal("suggestions start on")
	}
	r.tt.Key(0, ui.KeyDown)
	r.tt.Key(0, ui.KeyEnter)
	r.settle()
	if r.p.Settings.sel != 1 || !r.p.Settings.Open || r.p.SuggestOn || f.suggestSets != 1 {
		t.Fatalf("flip: sel %d open %v on %v sets %d", r.p.Settings.sel, r.p.Settings.Open, r.p.SuggestOn, f.suggestSets)
	}
	r.tt.Key(0, ui.KeyRight)
	if len(f.applied) != 3 {
		t.Fatal("→ off the shortcut row changes nothing")
	}
	// Space does the row too.
	r.tt.Key(0, ui.KeySpace)
	if !r.p.SuggestOn || f.suggestSets != 2 {
		t.Fatal("Space flips back")
	}
	if r.p.Height() > core.MaxHeight {
		t.Fatal("too tall")
	}
}

func TestSettingsSwitchesAndButtons(t *testing.T) {
	r, f := newPrefsRig(t)
	r.p.OpenSettings()
	r.settle()
	click := func(s string) {
		t.Helper()
		if err := r.tt.Click(s); err != nil {
			t.Fatal(err)
		}
		r.settle()
	}
	click("Show in Tray")
	if f.tray {
		t.Fatal("tray off")
	}
	click("Open Searches in Browser")
	if !r.p.SearchesOut || f.searchesOut == nil || !*f.searchesOut {
		t.Fatal("searches out")
	}
	click("Show in Tray")
	click("Open Searches in Browser")
	if !f.tray || r.p.SearchesOut || *f.searchesOut {
		t.Fatal("back as they were")
	}
	click("Open at Login")
	if !f.login {
		t.Fatal("login")
	}
	click("Sign In with Codegraff")
	click("Codegraff Terms & Privacy")
	click("Check for Updates")
	if f.signIns != 1 || f.terms != 1 || f.checks != 1 {
		t.Fatalf("buttons: %d %d %d", f.signIns, f.terms, f.checks)
	}
	// The last row scrolls into view, above the foot, when a key selects it.
	for r.p.Settings.selected() != rowQuit {
		r.tt.Key(0, ui.KeyDown)
	}
	r.settle()
	q, ok := r.tt.Find("Quit Nanolight")
	if !ok || q.H == 0 || q.Y+q.H > r.p.Settings.Height()-core.FooterH {
		t.Logf("sel %v", r.p.Settings.selected())
		t.Fatalf("Quit at %v, Settings %v tall", q, r.p.Settings.Height())
	}
	click("Quit Nanolight")
	if f.quits != 1 {
		t.Fatal("quit")
	}
	click("Show Welcome Again")
	if f.welcomes != 1 || r.p.Settings.Open {
		t.Fatal("welcome closes Settings")
	}
}

func TestSettingsKeyboardOrder(t *testing.T) {
	r, f := newPrefsRig(t)
	r.p.OpenSettings()
	r.settle()
	order := func() []rowKind { return r.p.Settings.rows() }
	want := []rowKind{rowChoice, rowSuggest, rowBrowser, rowLogin, rowTray, rowSync, rowTerms, rowUpdates, rowWelcome, rowQuit}
	if got := order(); len(got) != len(want) {
		t.Fatalf("signed out: %v", got)
	}
	// ↑ from the top wraps to Quit.
	r.tt.Key(0, ui.KeyUp)
	if r.p.Settings.selected() != rowQuit {
		t.Fatal("wrap up")
	}
	r.tt.Key(ui.Ctrl, ui.KeyN)
	if r.p.Settings.selected() != rowChoice {
		t.Fatal("Ctrl+N wraps down")
	}
	// Signed in: no terms line; "Synced as"; Return signs out.
	f.account = SignedIn
	r.settle()
	if len(order()) != len(want)-1 || !r.tt.HasText("Synced as ring@example.com") || !r.tt.HasText("Sign Out") {
		t.Fatalf("signed in: %v", order())
	}
	for range 5 {
		r.tt.Key(0, ui.KeyDown)
	}
	if r.p.Settings.selected() != rowSync {
		t.Fatalf("at %v", r.p.Settings.selected())
	}
	r.tt.Key(0, ui.KeyEnter)
	if f.signOuts != 1 {
		t.Fatal("sign out")
	}
	// No tray here: its row goes, and Settings is a row shorter.
	f.can = false
	r.settle()
	if r.tt.HasText("Show in Tray") || r.p.Settings.Height() != core.SettingsHeight-46 {
		t.Fatalf("no tray: %v", r.p.Settings.Height())
	}
	// An update waiting: Return restarts into it.
	f.ready = "2.1.0"
	r.settle()
	if !r.tt.HasText("Restart to Update to 2.1.0") {
		t.Fatal("ready")
	}
	if err := r.tt.Click("Restart to Update to 2.1.0"); err != nil {
		t.Fatal(err)
	}
	if f.restarts != 1 {
		t.Fatal("restart")
	}
}

func TestSettingsShortcutProblem(t *testing.T) {
	r, f := newPrefsRig(t)
	f.problem = "Alt Space can't be used here."
	r.p.OpenSettings()
	r.settle()
	if !r.tt.HasText("Alt Space can't be used here.") {
		t.Fatal("the problem shows")
	}
	r.snap("settings-problem")
	if err := r.tt.Click("Copy Command"); err != nil {
		t.Fatal(err)
	}
	r.tt.Frame()
	if r.h.copied != "nanolight --toggle" || !r.tt.HasText("Copied") {
		t.Fatalf("copied %q", r.h.copied)
	}
	if h := r.p.Height(); h > core.MaxHeight {
		t.Fatalf("height %v", h)
	}
	r.tt.Key(0, ui.KeyUp)
	r.settle()
	q, ok := r.tt.Find("Quit Nanolight")
	if !ok || q.H == 0 || q.Y+q.H > r.p.Height()-core.FooterH {
		t.Fatalf("Quit at %v, panel %v", q, r.p.Height())
	}
	r.snap("settings-scrolled")
}
