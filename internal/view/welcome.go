package view

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/rabden/ultra-light/internal/core"
	"github.com/rabden/ultra-light/internal/hotkey"
)

// The first run, inside the panel itself (onboard.zig). Three steps, each
// one thing to do:
//  1. the shortcut: Alt Space by default, and whether something else holds
//     it, with the one thing that frees it;
//  2. press it for real: the panel closes, and comes back on the second
//     press (a desktop shortcut running nanolight --toggle counts);
//  3. a first search to run, where Settings are, and opening at login.
//
// Esc skips at any step and it stays skipped; "Show Welcome Again" replays it.

// WelcomeStep is one of the three.
type WelcomeStep int

const (
	StepKey WelcomeStep = iota
	StepTry
	StepDone
)

// KeyState is what the desktop says of a shortcut.
type KeyState int

const (
	// KeyFree: registered, or free to be (X11).
	KeyFree KeyState = iota
	// KeyHeld: another app or the desktop grabbed it (X11).
	KeyHeld
	// KeyDesktop: the desktop's shortcut settings hold it (the Wayland
	// GlobalShortcuts portal took it).
	KeyDesktop
	// KeyUnusable: no way to take a key here (Wayland without the portal);
	// a desktop shortcut running nanolight --toggle is the way.
	KeyUnusable
	// KeyTaken: one of the desktop's own shortcuts (GNOME's window menu,
	// KRunner) holds it, and gets it first.
	KeyTaken
)

// WelcomeHost is what the steps ask of the app: none of it from a test.
type WelcomeHost interface {
	// Apply uses a shortcut from now on (registers it, keeps it).
	Apply(c hotkey.Choice)
	// KeyState probes a shortcut.
	KeyState(c hotkey.Choice) KeyState
	// Login reports whether Nanolight opens at login; set changes it first.
	Login(set *bool) bool
	// Done: the welcome is over; keep that.
	Done()
	// KeyTaken says what of the desktop's holds c, or ""; canFree: whether
	// Nanolight can take it over.
	KeyTaken(c hotkey.Choice) (what string, canFree bool)
	// TakeKey takes c from the desktop and uses it.
	TakeKey(c hotkey.Choice)
}

type example struct{ q, what, icon string }

// An app every Linux has, math, and words for Grok or X.
var examples = []example{
	{"term", "Open a terminal", "terminal"},
	{"2^10 * 3", "Do math", "plus"},
	{"liquid glass", "Ask Grok or search X", "sparkles"},
}

// Welcome is the welcome's state.
type Welcome struct {
	Active bool
	Hooks  WelcomeHost
	step   WelcomeStep
	choice hotkey.Choice
	// Step 2: the shortcut closed the panel, then opened it again.
	closed, reopened bool
	reopenedAt       time.Time
	login            bool
	state            KeyState
	takenWhat        string
	canFree          bool
	starts, steps    int
	copiedAt         time.Time
}

// Start shows the welcome from its first step, with the shortcut in use.
func (w *Welcome) Start(c hotkey.Choice) {
	w.Active = true
	w.step = StepKey
	w.choice = c
	w.closed, w.reopened = false, false
	w.reopenedAt = time.Time{}
	w.starts++
	w.steps++
	w.Refresh()
}

// Refresh looks again at the shortcut and the login (on each showing).
func (w *Welcome) Refresh() {
	if w.Hooks == nil {
		return // tests keep what they set
	}
	w.state = w.Hooks.KeyState(w.choice)
	w.takenWhat, w.canFree = w.Hooks.KeyTaken(w.choice)
	w.login = w.Hooks.Login(nil)
}

// SetState sets what the desktop says of the shortcut, for tests.
func (w *Welcome) SetState(s KeyState) { w.state = s }

// Step is the step showing.
func (w *Welcome) Step() WelcomeStep { return w.step }

// Pressed: the shortcut (or nanolight --toggle) fired, the panel visible
// before it — step 2's presses.
func (w *Welcome) Pressed(visible bool) {
	if !w.Active || w.step != StepTry {
		return
	}
	if visible {
		w.closed = true
	} else if w.closed {
		w.reopened = true
	}
}

const (
	heldExtra     = 30
	unusableExtra = 62
)

// Height is the step's height (onboard.zig: 330, more with a clash; 318;
// 396 with the login switch).
func (w *Welcome) Height() float32 {
	switch w.step {
	case StepTry:
		return 318
	case StepDone:
		if w.Hooks != nil {
			return 396
		}
		return 350
	}
	switch w.state {
	case KeyHeld, KeyTaken:
		return 330 + heldExtra
	case KeyUnusable:
		return 330 + unusableExtra
	}
	return 330
}

var welcomeKeys = []keyMods{
	{0, ui.KeyEscape}, {0, ui.KeyEnter}, {0, ui.KeyLeft}, {0, ui.KeyRight},
	{ui.Ctrl, ui.Key1}, {ui.Ctrl, ui.Key2}, {ui.Ctrl, ui.Key3},
}

func (w *Welcome) keys(c *ui.Context, p *Panel) bool {
	for _, k := range welcomeKeys {
		if c.Shortcut(k.mods, k.key) && w.Key(p, k.key, k.mods) {
			c.Invalidate()
			if !w.Active {
				return true
			}
		}
	}
	return false
}

// Key does what a key does in the welcome, and reports whether it took it.
func (w *Welcome) Key(p *Panel, key ui.Key, mods ui.Modifiers) bool {
	switch {
	case mods == 0 && key == ui.KeyEscape:
		w.finish(p, "")
	case mods == 0 && key == ui.KeyEnter:
		switch w.step {
		case StepKey:
			w.next(StepTry)
		case StepDone:
			w.finish(p, "")
		default:
			return false
		}
	case mods == 0 && (key == ui.KeyLeft || key == ui.KeyRight) && w.step == StepKey:
		dir := 1
		if key == ui.KeyLeft {
			dir = -1
		}
		w.pick(w.choice.Step(dir))
	case mods == ui.Ctrl && w.step == StepDone:
		for i, d := range digits[:len(examples)] {
			if key == d {
				w.finish(p, examples[i].q)
				return true
			}
		}
		return false
	default:
		return false
	}
	return true
}

func (w *Welcome) pick(ch hotkey.Choice) {
	w.choice = ch
	if w.Hooks != nil {
		w.state = w.Hooks.KeyState(ch)
		w.takenWhat, w.canFree = w.Hooks.KeyTaken(ch)
	}
}

func (w *Welcome) next(s WelcomeStep) {
	if w.step == StepKey && w.Hooks != nil {
		if w.state == KeyTaken && w.canFree {
			w.Hooks.TakeKey(w.choice)
		} else {
			w.Hooks.Apply(w.choice)
		}
	}
	w.step = s
	w.steps++
}

// finish ends the welcome: into the launcher, query typed (a first search)
// or not.
func (w *Welcome) finish(p *Panel, query string) {
	if w.step == StepKey && w.Hooks != nil {
		w.Hooks.Apply(w.choice)
	}
	w.Active = false
	if w.Hooks != nil {
		w.Hooks.Done()
	}
	p.set(query)
	p.builtValid = false
	p.revealed = "\x00" // the ghost may show at once
	p.input.Focus()
}

func (w *Welcome) view(c *ui.Context, p *Panel) {
	if w.keys(c, p) {
		return
	}
	pal := p.pal
	if w.step == StepTry && w.reopened {
		// Both ticks show for a moment, then the last step.
		now := p.clock()
		if w.reopenedAt.IsZero() {
			w.reopenedAt = now
		}
		if wait := 750*time.Millisecond - now.Sub(w.reopenedAt); wait <= 0 {
			w.next(StepDone)
			c.Invalidate()
		} else {
			c.After(wait)
		}
	}
	// A step's content comes in from the right, briefly.
	page := ui.Column(c.Key(fmt.Sprint("step-", w.steps))).FillWidth().Grow(1).Shrink(1)
	entered := ui.Local(page, "entered", func() bool { return false })
	pose := ui.Pose{}
	if !*entered {
		pose = ui.Pose{X: 14, Fade: 1}
		*entered = true
		c.Invalidate()
	}
	page.AnimatePose(pose, panelSpring)
	page.Children(func() {
		switch w.step {
		case StepKey:
			w.keyStep(c, p, pal)
		case StepTry:
			w.tryStep(c, pal)
		case StepDone:
			w.doneStep(c, p, pal)
		}
	})
	w.footer(c, pal)
}

func (w *Welcome) keyStep(c *ui.Context, p *Panel, pal Palette) {
	ui.Row(c).FillWidth().Padding(28, 28, 22, 28).Gap(16).AlignItems(ui.Center).Children(func() {
		if p.appIcon != nil {
			ui.Image(c, p.appIcon).Size(56, 56)
		}
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "Welcome to Nanolight").FontSize(22).FontWeight(W600).LetterSpacing(-0.3).TextColor(pal.Text).NoWrap()
			ui.Text(c, "Apps, math, Grok and X — one keystroke away.").FontSize(13.5).TextColor(pal.Sub).NoWrap()
		})
	})
	Hairline(c, pal)
	ui.Column(c).FillWidth().Padding(18, 28, 0, 28).Gap(12).Children(func() {
		SectionLabel(c, pal, "Open it with")
		ui.Row(c).Gap(8).Children(func() {
			for _, ch := range hotkey.All {
				on := ch == w.choice
				bg, border, fg, wt := pal.Tile.Alpha(0.5), pal.Rim, pal.Sub, W500
				if on {
					bg, border, fg, wt = pal.Sel, pal.Text.Alpha(0.35), pal.Text, W600
				} else if w.Hooks != nil {
					if what, _ := w.Hooks.KeyTaken(ch); what != "" {
						fg = pal.Sub.Alpha(0.45) // the desktop holds it
					}
				}
				ch := ch
				ui.Row(c.Key("wkey-"+ch.Name())).Height(36).PaddingX(16).AlignItems(ui.Center).Radius(10).
					Background(bg).Border(1, border).Label(ch.String()).OnClick(func() { w.pick(ch) }).Children(func() {
					ui.Text(c, ch.String()).FontSize(14).FontWeight(wt).TextColor(fg).NoWrap()
				})
			}
		})
		w.status(c, p, pal)
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(2, 0, 0, 0).Children(func() {
			Symbol(c, "lock.open", 11, Regular).TextColor(pal.Sub.Alpha(0.7))
			ui.Text(c, "No permissions to grant: the shortcut works as it is.").FontSize(12).TextColor(pal.Sub.Alpha(0.7)).NoWrap()
		})
	})
}

// status says whether the key is free, and if not, the one thing to do.
func (w *Welcome) status(c *ui.Context, p *Panel, pal Palette) {
	key := w.choice.String()
	tone := pal.Warn
	if w.state == KeyFree || w.state == KeyDesktop {
		tone = pal.Success
	}
	ui.Column(c).FillWidth().Gap(8).Padding(12).Radius(10).Background(tone.Alpha(0.1)).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			icon, line := "exclamationmark.triangle.fill", ""
			switch w.state {
			case KeyFree:
				icon, line = "checkmark.circle.fill", key+" is free on this PC."
			case KeyDesktop:
				icon, line = "checkmark.circle.fill", "Your desktop sets this shortcut, in its keyboard settings."
			case KeyHeld:
				line = key + " is held by another app or your desktop."
			case KeyUnusable:
				line = key + " can't be used here."
			case KeyTaken:
				line = key + " " + w.takenWhat + "."
			}
			Symbol(c, icon, 14, Regular).TextColor(tone)
			ui.Text(c, line).FontSize(13).FontWeight(W500).TextColor(pal.Text).NoWrap()
		})
		switch w.state {
		case KeyTaken:
			hint := "Pick another key above, or change it in your desktop's settings."
			if w.canFree {
				hint = "Press Return to use it anyway: picking another key later gives it back."
			}
			ui.Row(c).Padding(0, 0, 0, 22).Children(func() {
				ui.Text(c, hint).FontSize(12).TextColor(pal.Sub.Alpha(0.8)).NoWrap()
			})
		case KeyHeld:
			// A PC has no one place to free a key: another one is the fix.
			ui.Row(c).Padding(0, 0, 0, 22).Children(func() {
				ui.Text(c, "Pick another key above.").FontSize(12).TextColor(pal.Sub.Alpha(0.8)).NoWrap()
			})
		case KeyUnusable:
			ui.Row(c).Padding(0, 0, 0, 22).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(3).Children(func() {
					ui.Text(c, "Add a keyboard shortcut in your desktop's settings that runs").FontSize(12).TextColor(pal.Sub).NoWrap()
					ui.Text(c, toggleCommand).Font(Mono).FontSize(12).TextColor(pal.Text).NoWrap()
				})
				copyButton(c, p, pal, &w.copiedAt)
			})
		}
	})
}

// copyButton copies nanolight --toggle, and says so for a moment.
func copyButton(c *ui.Context, p *Panel, pal Palette, at *time.Time) {
	b := ui.Row(c.Key("copy-toggle")).Height(28).PaddingX(10).Radius(8).AlignItems(ui.Center).
		Background(pal.Tile).Border(1, pal.Rim).Label("Copy Command").OnClick(func() {
		if p.Host != nil {
			p.Host.Copy(toggleCommand)
		}
		*at = p.clock()
	})
	if b.Hovered() {
		b.Background(pal.Sel)
	}
	label := "Copy Command"
	if since := p.clock().Sub(*at); since < copiedFor {
		label = "Copied"
		c.After(copiedFor - since)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(12).FontWeight(W500).TextColor(pal.Text).NoWrap()
	})
}

func (w *Welcome) tryStep(c *ui.Context, pal Palette) {
	ui.Column(c).FillWidth().Padding(34, 0, 0, 0).Gap(18).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ch := w.choice
			held := c.Modifiers()
			heldMod := true
			for _, m := range ch.Mods() {
				switch m {
				case "Ctrl":
					heldMod = heldMod && held&ui.Ctrl != 0
				case "Alt":
					heldMod = heldMod && held&ui.Alt != 0
				case "Shift":
					heldMod = heldMod && held&ui.Shift != 0
				}
			}
			if mods := ch.Held(); mods != "" {
				// A PC's "Ctrl Alt" is words: wider per letter.
				w.cap(c, pal, "mods", mods, 40+8*float32(len(mods)), heldMod || w.reopened)
				ui.Text(c, "+").FontSize(20).FontWeight(W500).TextColor(pal.Sub.Alpha(0.6))
			}
			w.cap(c, pal, "key", ch.Key(), 180, w.reopened)
		})
		ui.Column(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "Press %s now", w.choice).FontSize(20).FontWeight(W600).LetterSpacing(-0.2).TextColor(pal.Text).NoWrap()
			ui.Text(c, "It closes. Press it again and it comes back, from any app.").FontSize(13.5).TextColor(pal.Sub).NoWrap()
		})
		ui.Row(c).Gap(10).Children(func() {
			tick(c, pal, "Closed", w.closed)
			tick(c, pal, "Back again", w.reopened)
		})
	})
}

var capSpring = ui.SpringOptions{Stiffness: 600, Damping: 40, Mass: 1}

// cap is a key cap that lights up (on a 600/40 spring) while held.
func (w *Welcome) cap(c *ui.Context, pal Palette, key, s string, width float32, lit bool) {
	e := ui.Box(c.Key(key)).Size(width, 64).Center().Radius(14)
	target := float32(0)
	if lit {
		target = 1
	}
	t := e.Spring("lit", target, capSpring)
	e.Background(pal.Tile.Mix(pal.Text.Alpha(0.22), t)).Border(1, pal.Rim.Mix(pal.Text.Alpha(0.45), t)).
		Shadow(0, 3, 0, 0, black.Alpha(0.18*(1-t))).Translate(0, 2*t)
	size := float32(26)
	if len(s) > 3 {
		size = 18
	}
	e.Children(func() {
		ui.Text(c, s).FontSize(size).FontWeight(W500).TextColor(pal.Text).NoWrap()
	})
}

func tick(c *ui.Context, pal Palette, s string, on bool) {
	bg, icon, ic, fg := pal.Tile, "circle", pal.Sub.Alpha(0.6), pal.Sub
	if on {
		bg, icon, ic, fg = pal.Success.Alpha(0.16), "checkmark.circle.fill", pal.Success, pal.Text
	}
	ui.Row(c).Gap(6).Height(28).PaddingX(12).AlignItems(ui.Center).Radius(14).Background(bg).Children(func() {
		Symbol(c, icon, 13, Regular).TextColor(ic)
		ui.Text(c, s).FontSize(12.5).FontWeight(W500).TextColor(fg).NoWrap()
	})
}

func (w *Welcome) doneStep(c *ui.Context, p *Panel, pal Palette) {
	ui.Column(c).FillWidth().Padding(26, 28, 16, 28).Gap(4).Children(func() {
		ui.Text(c, "You're set.").FontSize(22).FontWeight(W600).LetterSpacing(-0.3).TextColor(pal.Text).NoWrap()
		ui.Textf(c, "%s opens it anywhere. Start with one of these:", w.choice).FontSize(13.5).TextColor(pal.Sub).NoWrap()
	})
	ui.Column(c).FillWidth().PaddingX(16).Gap(2).Children(func() {
		for i, e := range examples {
			e := e
			r := ui.Row(c.Key("ex-" + e.q)).FillWidth().Height(40).PaddingX(12).Gap(12).AlignItems(ui.Center).Radius(10).
				Label(e.what).OnClick(func() { w.finish(p, e.q) })
			if r.Hovered() {
				r.Background(pal.Sel)
			}
			r.Children(func() {
				ui.Box(c).Size(26, 26).Radius(7).Background(pal.Tile).Center().Children(func() {
					Symbol(c, e.icon, 13, Medium).TextColor(pal.Text)
				})
				ui.Text(c, e.q).Font(Mono).FontSize(13.5).FontWeight(W500).TextColor(pal.Text).NoWrap()
				ui.Text(c, e.what).FontSize(12).TextColor(pal.Sub.Alpha(0.8)).NoWrap()
				ui.Spacer(c)
				ui.Textf(c, "%s%d", core.Mod, i+1).FontSize(12).FontWeight(W500).TextColor(pal.Sub.Alpha(0.55)).FontFeatures("tnum").NoWrap()
			})
		}
	})
	ui.Column(c).FillWidth().Padding(16, 28, 0, 28).Gap(12).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(26, 22).Radius(6).Background(pal.Tile).Center().Children(func() {
				Symbol(c, "magnifyingglass", 12, Semibold).TextColor(pal.Text)
			})
			ui.Text(c, "Settings are on Ctrl+, and the gear at the bottom of the panel.").FontSize(12.5).TextColor(pal.Sub).NoWrap()
		})
		if w.Hooks == nil {
			return
		}
		r := ui.Row(c.Key("login")).Gap(10).AlignItems(ui.Center).Label("Open Nanolight when you log in").OnClick(func() {
			next := !w.login
			w.login = w.Hooks.Login(&next)
		})
		r.Children(func() {
			animatedSwitch(c, r, pal, w.login)
			ui.Text(c, "Open Nanolight when you log in").FontSize(13.5).TextColor(pal.Text).NoWrap()
		})
	})
}

// animatedSwitch is nanoui's toggle: a 40×24 track, a 20 knob with a
// shadow, sliding to the right when on.
func animatedSwitch(c *ui.Context, e ui.Element, pal Palette, on bool) {
	target := float32(0)
	if on {
		target = 1
	}
	t := e.Spring("knob", target, ui.SpringOptions{Stiffness: 500, Damping: 40, Mass: 1})
	ui.Row(c).Size(40, 24).Radius(12).Padding(2, 0, 0, 2+t*16).Background(pal.Tile.Mix(pal.Success, t)).Children(func() {
		ui.Box(c).Size(20, 20).Radius(10).Background(white).Shadow(0, 1, 3, 0, black.Alpha(0.25))
	})
}

func (w *Welcome) footer(c *ui.Context, pal Palette) {
	Hairline(c, pal)
	ui.Row(c).FillWidth().Height(core.FooterH).PaddingX(16).Gap(8).AlignItems(ui.Center).Children(func() {
		n := int(w.step)
		for i := range 3 {
			wd, col := float32(6), pal.Sub.Alpha(0.3)
			if i == n {
				wd, col = 16, pal.Text.Alpha(0.7)
			}
			ui.Box(c).Size(wd, 6).Radius(3).Background(col)
		}
		ui.Textf(c, "  %d of 3", n+1).FontSize(12).TextColor(pal.Sub.Alpha(0.8)).FontFeatures("tnum").NoWrap()
		ui.Spacer(c)
		action := ""
		switch w.step {
		case StepKey:
			action = "Continue"
			if w.state == KeyHeld || w.state == KeyTaken && w.canFree {
				action = "Use " + w.choice.String() + " anyway"
			}
		case StepDone:
			action = "Start searching"
		}
		if action != "" {
			ui.Text(c, action).FontSize(12).FontWeight(W500).TextColor(pal.Text.Alpha(0.85)).NoWrap()
			Kbd(c, pal, "↩")
			VRule(c, pal)
		}
		esc := "Skip"
		if w.step == StepDone {
			esc = "Close"
		}
		ui.Text(c, esc).FontSize(12).TextColor(pal.Sub.Alpha(0.8)).NoWrap()
		Kbd(c, pal, "esc")
	})
}
