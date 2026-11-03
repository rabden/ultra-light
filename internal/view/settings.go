package view

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/rabden/ultra-light/internal/core"
	"github.com/rabden/ultra-light/internal/hotkey"
)

// Settings, in the panel itself (settings_view.zig): the shortcut, Google's
// suggestions and where searches open, opening at login, the tray icon,
// syncing across computers, updates. Esc goes back to search; ↑/↓ move,
// Return and Space do the row, ←/→ step the shortcut on its row. Without
// hooks (tests) it still draws, and changes nothing the app keeps.

// AccountState is where signing in to Codegraff is (account.zig).
type AccountState int

const (
	SignedOut AccountState = iota
	Starting
	Waiting
	SignedIn
)

// SettingsHost is what Settings asks of the app (settings_host.zig).
type SettingsHost interface {
	Choice() hotkey.Choice
	// Apply registers and keeps a shortcut.
	Apply(c hotkey.Choice)
	// KeyProblem says why the shortcut doesn't open the panel here, or "".
	KeyProblem() string
	// Login reports whether Nanolight opens at login; set changes it first.
	Login(set *bool) bool
	Suggest(on bool)
	SearchesOut(on bool)
	// Tray reports whether the tray icon shows, set changing it first, and
	// whether this desktop can show one at all.
	Tray(set *bool) (on, can bool)
	Account() AccountState
	Email() string
	SignIn()
	SignOut()
	Terms()
	Version() string
	// Updatable: Nanolight updates itself (not a package manager's copy).
	Updatable() bool
	// Ready is the version an update waits to restart into, or "".
	Ready() string
	Check()
	Restart()
	Welcome()
	Quit()
}

type rowKind int

const (
	rowChoice rowKind = iota
	rowSuggest
	rowBrowser
	rowLogin
	rowTray
	rowSync
	rowTerms
	rowUpdates
	rowWelcome
	rowQuit
)

// Settings is the Settings screen's state.
type Settings struct {
	Open  bool
	Hooks SettingsHost
	sel   int
	opens int

	choice  hotkey.Choice
	problem string
	login   bool
	tray    bool
	canTray bool
	canUpd  bool
	account AccountState
	email   string
	version string
	ready   string

	copiedAt time.Time
	// reveal: the selection moved by a key; its row scrolls into view.
	reveal bool
}

const copiedFor = 1400 * time.Millisecond

func (s *Settings) init() {
	s.version = "0.1.0"
	s.canTray = true
	s.canUpd = true
	s.tray = true
}

// Show opens Settings at its first row.
func (s *Settings) Show() {
	s.Open = true
	s.sel = 0
	s.opens++
}

// Hide closes it.
func (s *Settings) Hide() { s.Open = false }

// pull reads what the app says now.
func (s *Settings) pull() {
	h := s.Hooks
	if h == nil {
		return
	}
	s.choice = h.Choice()
	s.problem = h.KeyProblem()
	s.login = h.Login(nil)
	s.tray, s.canTray = h.Tray(nil)
	s.account = h.Account()
	s.email = h.Email()
	s.version = h.Version()
	s.ready = h.Ready()
	s.canUpd = h.Updatable()
}

// problemH is the room the note under the chips takes, when the shortcut
// doesn't work here.
const problemH = 76

// Height is Settings' height: the Mac's 652 (two switch rows more than the
// PC's 560: where searches open, the tray), one row less without a tray,
// and the note when the shortcut doesn't work; never past the panel's most.
func (s *Settings) Height() float32 {
	s.pull()
	h := float32(core.SettingsHeight)
	if !s.canTray {
		h -= 46
	}
	if s.problem != "" {
		h += problemH
	}
	return min(h, core.MaxHeight)
}

func (s *Settings) rows() []rowKind {
	rows := []rowKind{rowChoice, rowSuggest, rowBrowser, rowLogin}
	if s.canTray {
		rows = append(rows, rowTray)
	}
	rows = append(rows, rowSync)
	if s.account == SignedOut || s.account == Starting {
		rows = append(rows, rowTerms)
	}
	if s.canUpd {
		rows = append(rows, rowUpdates)
	}
	return append(rows, rowWelcome, rowQuit)
}

func (s *Settings) selected() rowKind {
	rows := s.rows()
	return rows[min(s.sel, len(rows)-1)]
}

func (s *Settings) view(c *ui.Context, p *Panel, shell ui.Element) {
	s.pull()
	rows := s.rows()
	if s.sel >= len(rows) {
		s.sel = len(rows) - 1
	}
	if s.keys(c, p, rows) {
		return
	}
	h := p.follow(c, shell, s.Height())
	pal := p.pal
	page := ui.Column(c.Key(s.opens)).FillWidth().Height(h).Clip()
	// In from the right: opacity 0, x +14, on the panel's spring.
	entered := ui.Local(page, "entered", func() bool { return false })
	pose := ui.Pose{}
	if !*entered {
		pose = ui.Pose{X: 14, Fade: 1}
		*entered = true
		c.Invalidate()
	}
	page.AnimatePose(pose, panelSpring)
	page.Children(func() {
		s.head(c, pal)
		// The rows are taller than the panel can be (as on the Mac): they
		// scroll, the selected one kept in view.
		ui.Scroll(c).FillWidth().Padding(2, 20, 6, 20).Gap(10).Grow(1).Shrink(1).Children(func() {
			s.body(c, p, pal)
		})
		s.reveal = false
		Hairline(c, pal)
		ui.Row(c).FillWidth().Height(core.FooterH).PaddingX(16).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Back").FontSize(12).TextColor(pal.Sub.Alpha(0.8)).NoWrap()
			Kbd(c, pal, "esc")
		})
	})
}

func (s *Settings) head(c *ui.Context, pal Palette) {
	ui.Row(c).FillWidth().Height(core.HeaderH).PaddingX(16).Gap(10).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Box(c).Size(28, 28).Radius(8).Center().Background(pal.Tile).Children(func() {
			Symbol(c, "gearshape", 15, Medium).TextColor(pal.Text)
		})
		ui.Text(c, "Settings").FontSize(18).FontWeight(W600).LetterSpacing(-0.2).TextColor(pal.Text).NoWrap()
	})
}

func (s *Settings) body(c *ui.Context, p *Panel, pal Palette) {
	SectionLabel(c, pal, "Open Nanolight with")
	s.choices(c, p, pal)
	if s.problem != "" {
		s.problemNote(c, p, pal)
	}
	SectionLabel(c, pal, "Search")
	s.switchRow(c, p, pal, rowSuggest, "Google Suggestions", p.SuggestOn)
	s.switchRow(c, p, pal, rowBrowser, "Open Searches in Browser", p.SearchesOut)
	SectionLabel(c, pal, "This PC")
	s.switchRow(c, p, pal, rowLogin, "Open at Login", s.login)
	if s.canTray {
		s.switchRow(c, p, pal, rowTray, "Show in Tray", s.tray)
	}
	SectionLabel(c, pal, "Sync Across Computers")
	s.sync(c, p, pal)
	SectionLabel(c, pal, "Updates")
	ui.Textf(c, "Nanolight %s", s.version).FontSize(12.5).TextColor(pal.Sub).NoWrap()
	upd := "Check for Updates"
	if s.ready != "" {
		upd = "Restart to Update to " + s.ready
	}
	if s.canUpd {
		s.button(c, p, pal, rowUpdates, upd)
	}
	s.button(c, p, pal, rowWelcome, "Show Welcome Again")
	s.button(c, p, pal, rowQuit, "Quit Nanolight")
}

// choices are the four presets, as chips.
func (s *Settings) choices(c *ui.Context, p *Panel, pal Palette) {
	ui.Row(c).Gap(8).Shrink(0).Children(func() {
		for _, ch := range hotkey.All {
			on := ch == s.choice
			bg, border, fg, w := pal.Tile.Alpha(0.5), pal.Rim, pal.Sub, W500
			if on {
				bg, border, fg, w = pal.Sel, pal.Text.Alpha(0.35), pal.Text, W600
			}
			ch := ch
			ui.Row(c.Key("key-"+ch.Name())).Height(36).PaddingX(14).AlignItems(ui.Center).Radius(10).
				Background(bg).Border(1, border).Label(ch.String()).OnClick(func() { s.pick(ch) }).Children(func() {
				ui.Text(c, ch.String()).FontSize(13.5).FontWeight(w).TextColor(fg).NoWrap()
			})
		}
	})
}

// toggleCommand is what a desktop shortcut runs where Nanolight can't take
// the key itself.
const toggleCommand = "nanolight --toggle"

// problemNote says the shortcut doesn't open the panel here, and how to make
// one that does (a desktop shortcut running nanolight --toggle).
func (s *Settings) problemNote(c *ui.Context, p *Panel, pal Palette) {
	ui.Row(c).FillWidth().Height(problemH-10).Padding(10, 12).Gap(10).Radius(10).
		Background(pal.Warn.Alpha(0.1)).AlignItems(ui.Center).Shrink(0).Children(func() {
		Symbol(c, "exclamationmark.triangle.fill", 14, Medium).TextColor(pal.Warn)
		ui.Column(c).Grow(1).Shrink(1).Gap(3).Children(func() {
			ui.Text(c, s.problem).FontSize(12.5).FontWeight(W500).TextColor(pal.Text)
			ui.Text(c, "Add a keyboard shortcut in your desktop's settings that runs").FontSize(12).TextColor(pal.Sub).NoWrap()
			ui.Text(c, toggleCommand).Font(Mono).FontSize(12).TextColor(pal.Text).NoWrap()
		})
		copyButton(c, p, pal, &s.copiedAt)
	})
}

func (s *Settings) switchRow(c *ui.Context, p *Panel, pal Palette, kind rowKind, title string, on bool) {
	bg := pal.Fill.Alpha(0)
	if s.selected() == kind {
		bg = pal.Sel
	}
	row := ui.Row(c.Key(100 + int(kind))).FillWidth().Height(36).PaddingX(4).Gap(10).AlignItems(ui.Center).Radius(10).
		Background(bg).Label(title).Shrink(0).OnClick(func() { s.sel = s.index(kind); s.flip(p, kind) })
	s.show(row, kind).Children(func() {
		ui.Text(c, title).FontSize(13.5).FontWeight(W500).TextColor(pal.Text).NoWrap()
		ui.Spacer(c)
		Switch(c, pal, on)
	})
}

func (s *Settings) sync(c *ui.Context, p *Panel, pal Palette) {
	switch s.account {
	case SignedOut, Starting:
		t := "Sign In with Codegraff"
		if s.account == Starting {
			t = "Signing In…"
		}
		s.button(c, p, pal, rowSync, t)
		s.termsLine(c, p, pal)
	case Waiting:
		ui.Text(c, "Approve in your browser…").FontSize(13.5).FontWeight(W500).TextColor(pal.Text).NoWrap()
		s.button(c, p, pal, rowSync, "Cancel")
	case SignedIn:
		who := s.email
		if who == "" {
			who = "your Codegraff account"
		}
		ui.Textf(c, "Synced as %s", who).FontSize(13.5).FontWeight(W500).TextColor(pal.Text).NoWrap()
		s.button(c, p, pal, rowSync, "Sign Out")
	}
}

func (s *Settings) termsLine(c *ui.Context, p *Panel, pal Palette) {
	bg := pal.Fill.Alpha(0)
	if s.selected() == rowTerms {
		bg = pal.Sel
	}
	row := ui.Row(c.Key("terms")).FillWidth().Height(28).PaddingX(4).Gap(4).AlignItems(ui.Center).Radius(8).
		Background(bg).Shrink(0).Label("Codegraff Terms & Privacy").OnClick(func() { s.sel = s.index(rowTerms); s.activate(p, rowTerms) })
	s.show(row, rowTerms).Children(func() {
		ui.Text(c, "Uses your Codegraff account; Codegraff's").FontSize(11.5).TextColor(pal.Sub.Alpha(0.75)).NoWrap()
		ui.Text(c, "Terms & Privacy").FontSize(11.5).FontWeight(W500).TextColor(pal.Text.Alpha(0.9)).NoWrap()
		ui.Text(c, "apply").FontSize(11.5).TextColor(pal.Sub.Alpha(0.75)).NoWrap()
	})
}

func (s *Settings) button(c *ui.Context, p *Panel, pal Palette, kind rowKind, text string) {
	on := s.selected() == kind
	bg := pal.Tile.Alpha(0.45)
	if on {
		bg = pal.Sel
	}
	row := ui.Row(c.Key(200+int(kind))).FillWidth().Height(36).PaddingX(12).Gap(8).AlignItems(ui.Center).Radius(10).
		Background(bg).Border(1, pal.Rim).Shrink(0).Label(text).OnClick(func() { s.sel = s.index(kind); s.activate(p, kind) })
	s.show(row, kind).Children(func() {
		ui.Text(c, text).FontSize(13.5).FontWeight(W500).TextColor(pal.Text).NoWrap()
		ui.Spacer(c)
		if on {
			Kbd(c, pal, "↩")
		}
	})
}

// show scrolls a row into view when a key just selected it.
func (s *Settings) show(e ui.Element, kind rowKind) ui.Element {
	if s.reveal && s.selected() == kind {
		e.ScrollIntoView()
	}
	return e
}

func (s *Settings) index(kind rowKind) int {
	for i, k := range s.rows() {
		if k == kind {
			return i
		}
	}
	return s.sel
}

var settingsKeys = []keyMods{
	{0, ui.KeyEscape}, {0, ui.KeyDown}, {ui.Ctrl, ui.KeyN}, {0, ui.KeyUp}, {ui.Ctrl, ui.KeyP},
	{0, ui.KeyLeft}, {0, ui.KeyRight}, {0, ui.KeyEnter}, {0, ui.KeySpace},
}

// keys handles Settings' keys; it returns true when Settings closed.
func (s *Settings) keys(c *ui.Context, p *Panel, rows []rowKind) bool {
	for _, k := range settingsKeys {
		if !c.Shortcut(k.mods, k.key) {
			continue
		}
		if s.Key(p, k.key, k.mods) && !s.Open {
			c.Invalidate()
			return true
		}
	}
	return false
}

// Key does what a key does in Settings, and reports whether it took it.
func (s *Settings) Key(p *Panel, key ui.Key, mods ui.Modifiers) bool {
	rows := s.rows()
	n := len(rows)
	switch {
	case mods == 0 && key == ui.KeyEscape:
		s.Hide()
		p.input.Focus()
	case mods == 0 && key == ui.KeyDown, mods == ui.Ctrl && key == ui.KeyN:
		s.sel = (s.sel + 1) % n
		s.reveal = true
	case mods == 0 && key == ui.KeyUp, mods == ui.Ctrl && key == ui.KeyP:
		s.sel = (s.sel + n - 1) % n
		s.reveal = true
	case mods == 0 && key == ui.KeyLeft:
		s.nudge(-1)
	case mods == 0 && key == ui.KeyRight:
		s.nudge(1)
	case mods == 0 && (key == ui.KeyEnter || key == ui.KeySpace):
		s.activate(p, rows[min(s.sel, n-1)])
	default:
		return false
	}
	return true
}

// nudge steps the shortcut, on its row only.
func (s *Settings) nudge(dir int) {
	if s.selected() != rowChoice {
		return
	}
	s.pick(s.choice.Step(dir))
}

func (s *Settings) pick(ch hotkey.Choice) {
	s.choice = ch
	if s.Hooks != nil {
		s.Hooks.Apply(ch)
	}
}

func (s *Settings) flip(p *Panel, kind rowKind) {
	h := s.Hooks
	switch kind {
	case rowSuggest:
		p.SuggestOn = !p.SuggestOn
		p.builtValid = false
		if h != nil {
			h.Suggest(p.SuggestOn)
		}
	case rowBrowser:
		p.SearchesOut = !p.SearchesOut
		if h != nil {
			h.SearchesOut(p.SearchesOut)
		}
	case rowLogin:
		next := !s.login
		s.login = next
		if h != nil {
			s.login = h.Login(&next)
		}
	case rowTray:
		next := !s.tray
		s.tray = next
		if h != nil {
			s.tray, _ = h.Tray(&next)
		}
	}
}

func (s *Settings) activate(p *Panel, kind rowKind) {
	h := s.Hooks
	switch kind {
	case rowChoice:
		// A PC steps to the next of the four; there is no recording.
		s.pick(s.choice.Step(1))
	case rowSuggest, rowBrowser, rowLogin, rowTray:
		s.flip(p, kind)
	}
	if h == nil {
		return
	}
	switch kind {
	case rowSync:
		switch s.account {
		case SignedOut:
			h.SignIn()
		case Waiting, SignedIn:
			h.SignOut()
		}
	case rowTerms:
		h.Terms()
	case rowUpdates:
		if s.ready != "" {
			h.Restart()
		} else {
			h.Check()
		}
	case rowWelcome:
		s.Hide()
		h.Welcome()
	case rowQuit:
		h.Quit()
	}
}
