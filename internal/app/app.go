// Package app runs Nanolight: it starts things in the Zig app's order
// (main.zig), owns the panel's window, the global shortcut, the app index,
// Google's suggestions, the results page, the tray, sync and updates, and is
// the view's Host. Everything here runs on mygo's main thread unless it says
// otherwise; goroutines hand their results back with Window.Update.
package app

import (
	"log"
	"math"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/rabden/ultra-light/internal/account"
	"github.com/rabden/ultra-light/internal/apps"
	"github.com/rabden/ultra-light/internal/core"
	"github.com/rabden/ultra-light/internal/hotkey"
	"github.com/rabden/ultra-light/internal/store"
	"github.com/rabden/ultra-light/internal/view"
)

// devVersion is what a build without a version says, as the Zig app's dev
// builds do; the updater stays off for it.
const devVersion = "0.1.0"

// Version is the app's version: the one `mygo build` links in, else
// devVersion.
func Version() string {
	if v := mygo.App.Version(); v != "" {
		return v
	}
	return devVersion
}

// App is the running Nanolight.
type App struct {
	kv    *store.Store
	index *apps.Index
	panel *view.Panel
	win   *mygo.Window

	choice    hotkey.Choice
	accel     string // the accelerator registered, "" if none
	hotkeyErr error

	installed []mygo.InstalledApp
	extra     []apps.Entry // Raycast's rows and script commands
	scanAt    time.Time
	scanning  bool
	icons     map[string]*ui.Bitmap

	keepOpen bool
	quitting bool
	blurAt   time.Time
	login    *bool // opens at login, as last read
	trayCan  bool  // the desktop shows tray icons
	tray     *mygo.Tray
	acct     *account.Account
	up       updates
	dkeys    deskKeys
	page     pageState
	sugg     suggester
	ext      extRunner
	script   scriptRunner
}

var nl *App

// Run starts Nanolight and returns when it quits.
func Run() error {
	a := &App{icons: map[string]*ui.Bitmap{}, index: &apps.Index{}}
	nl = a
	a.ext.a, a.script.a = a, a
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {}) // a launcher keeps running
	mygo.App.OnSecondInstance(func(args []string, _ string) {
		if slices.Contains(args, "--toggle") {
			a.Toggle()
		} else {
			a.Show()
		}
	})
	mygo.App.WhenReady(a.start)
	return mygo.App.Run()
}

// start follows main.zig's order: apps, kv, settings, the hidden panel,
// Raycast's rows, the shortcut, the welcome, the tray, updates, the
// account, and last the panel shown (unless restarted by an update or
// opened at login).
func (a *App) start() {
	quitZigCopy()
	a.kv = store.Open(store.DataDir())
	a.panel = view.NewPanel(a, a.index)
	a.panel.SuggestOn = a.kv.Bool(store.KeySuggest, true)
	a.panel.SearchesOut = a.kv.Bool(store.KeySearchesOut, false)
	a.choice = hotkey.Load(must(a.kv.Get(store.KeyHotkey)))
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:         "Nanolight",
		Panel:         true,
		Hidden:        true,
		Frameless:     true,
		Transparent:   true,
		DisableShadow: true,
		DisableResize: true,
		SkipTaskbar:   true,
		Width:         core.PanelWidth,
		Height:        int(math.Ceil(float64(a.panel.Height()))),
		Content:       ui.View(a.panel.View),
	})
	// Hide rather than close (Alt+F4), unless the app is quitting: Quit in
	// Settings or the tray, the session ending (SIGTERM).
	a.win.OnClose(func(e *mygo.CloseEvent) {
		if a.quitting {
			return
		}
		e.PreventDefault()
		a.Hide()
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { a.quitting = true })
	a.win.OnBlur(a.blurred)
	a.win.OnHide(a.hidden)
	mygo.Theme.OnUpdated(a.win.Invalidate)
	a.scan()
	a.startRaycast()
	a.useKey(a.choice)
	a.startSettings()
	a.startWelcome()
	a.startTray()
	a.startUpdates()
	a.startAccount()
	if note := a.updatedNote(); note != "" {
		a.panel.Note = note
		return
	}
	if mygo.App.WasOpenedAtLogin() {
		return // started with the session: quietly, the shortcut ready
	}
	a.Show()
}

func must(s string, _ bool) string { return s }

// updatedNote reads and clears nanolight.updated: restarted by an update,
// the panel stays hidden and says so the next time it opens.
func (a *App) updatedNote() string {
	v, ok := a.kv.Get(store.KeyUpdated)
	if !ok {
		return ""
	}
	a.kv.Remove(store.KeyUpdated)
	return "Updated to " + v
}

// quitZigCopy asks a copy of the Zig build left running to quit: it listens
// on $XDG_RUNTIME_DIR/nanolight.sock or /tmp/nanolight-<uid>.sock.
func quitZigCopy() {
	var paths []string
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		paths = append(paths, filepath.Join(d, "nanolight.sock"))
	}
	paths = append(paths, filepath.Join("/tmp", "nanolight-"+itoa(os.Getuid())+".sock"))
	for _, p := range paths {
		c, err := net.DialTimeout("unix", p, 200*time.Millisecond)
		if err != nil {
			continue
		}
		c.Write([]byte("quit\n"))
		c.Close()
		log.Printf("nanolight: asked the older copy at %s to quit", p)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// Show places the panel, reveals the query, and shows it with the keyboard.
func (a *App) Show() {
	if a.win == nil {
		return
	}
	a.welcomeShowing()
	a.login = nil // read again: the desktop's settings may have changed it
	a.place()
	a.panel.Reveal()
	a.win.Show()
	a.win.Focus()
	a.page.cancelRelease()
	a.maybeRescan()
	a.maybePull()
	a.win.Invalidate()
}

// Toggle hides the panel when it shows with the keyboard, else shows it.
func (a *App) Toggle() {
	if os.Getenv("NANOLIGHT_DEBUG") != "" {
		log.Printf("nanolight: toggle (visible %v, focused %v)", a.win.IsVisible(), a.win.IsFocused())
	}
	if a.win == nil {
		return
	}
	// The shortcut's own key grab takes the focus for a moment (X11's
	// NotifyGrab): a blur just now still counts as showing with the keys.
	showing := a.win.IsVisible() && (a.win.IsFocused() || time.Since(a.blurAt) < blurGrace)
	a.welcomePressed(showing)
	if showing {
		a.win.Hide()
		return
	}
	a.Show()
}

// Hide hides the panel, unless the row just opened keeps it (a Raycast
// command shown in it).
func (a *App) Hide() {
	if a.keepOpen {
		a.keepOpen = false
		return
	}
	if a.win != nil && a.win.IsVisible() {
		a.win.Hide()
	}
}

// blurred: another window took the keys; the panel goes, and a note
// ("Updated to …") is done.
func (a *App) blurred() {
	// Not at once: the shortcut's key grab blurs the panel too, and gives
	// the focus back as the key is let go.
	a.blurAt = time.Now()
	time.AfterFunc(blurGrace, func() {
		a.win.Update(func() {
			if a.win.IsVisible() && !a.win.IsFocused() && time.Since(a.blurAt) >= blurGrace {
				a.panel.Note = ""
				a.win.Hide()
			}
		})
	})
}

// blurGrace is how long a blur waits before the panel goes.
const blurGrace = 150 * time.Millisecond

// hidden: the page stops, and is let go after a while; updates may go in.
func (a *App) hidden() {
	a.panel.Back()
	a.page.releaseLater()
	a.updateAway()
}

// place puts the panel a fifth of the way down the display with the
// pointer, centered, high enough that it stays on a small screen at its
// tallest (panel_pc.zig).
func (a *App) place() {
	d := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	b := d.Bounds
	x := b.X + max(0, (b.Width-core.PanelWidth)/2)
	y := b.Y + int(math.Max(0, math.Min(float64(b.Height)*0.2, float64(b.Height)-core.MaxHeight-16)))
	a.win.SetPosition(x, y)
}

// useKey registers the shortcut, letting go of the one before.
func (a *App) useKey(c hotkey.Choice) {
	a.choice = c
	if a.accel != "" {
		mygo.GlobalShortcut.Unregister(a.accel)
		a.accel = ""
	}
	if err := mygo.GlobalShortcut.Register(c.Accelerator(), a.Toggle); err != nil {
		a.hotkeyErr = err
		log.Printf("nanolight: %s can't be registered (%v); open it with nanolight --toggle or the tray", c, err)
		return
	}
	a.hotkeyErr = nil
	a.accel = c.Accelerator()
}

// keyTaken probes whether a preset can be registered (X11): register it and
// let it go. The one in use counts as free.
func (a *App) keyTaken(c hotkey.Choice) bool {
	if c.Accelerator() == a.accel {
		return false
	}
	if err := mygo.GlobalShortcut.Register(c.Accelerator(), func() {}); err != nil {
		return true
	}
	mygo.GlobalShortcut.Unregister(c.Accelerator())
	return false
}

// Wayland reports whether the session is Wayland, where the desktop owns
// the keys.
func Wayland() bool {
	return os.Getenv("WAYLAND_DISPLAY") != "" || strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland")
}

// --- view.Host ---

// Resize follows the panel's height.
func (a *App) Resize(h float32) {
	if a.win != nil {
		// The spring overshoots a little; the window never passes the
		// tallest panel, so it stays on a small screen.
		h = min(h, core.MaxHeight)
		a.win.SetSize(core.PanelWidth, int(math.Ceil(float64(h))))
	}
}

// Copy puts text on the clipboard.
func (a *App) Copy(text string) { mygo.Clipboard.WriteText(text) }

// OpenURL opens a URL in the browser.
func (a *App) OpenURL(u string) {
	if err := mygo.Shell.OpenExternal(u); err != nil {
		log.Printf("nanolight: opening %s: %v", u, err)
	}
}

// SaveLaunches keeps the launch counts, and syncs them.
func (a *App) SaveLaunches(enc string) {
	a.kv.Set(store.KeyLaunches, enc)
	a.touched(false)
}
