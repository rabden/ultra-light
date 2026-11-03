package app

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"github.com/rabden/ultra-light/internal/hotkey"
	"github.com/rabden/ultra-light/internal/store"
	"github.com/rabden/ultra-light/internal/view"
)

// termsURL is Codegraff's terms and privacy (account.zig).
const termsURL = "https://codegraff.com/terms"

// prefs answers Settings (settings_bind.zig): the App, as view.SettingsHost.
type prefs struct{ a *App }

func (a *App) startSettings() {
	a.panel.Settings.Hooks = prefs{a}
	// The Zig build's autostart entry runs the old program: it becomes
	// this one's.
	if zigAutostart() {
		on := true
		prefs{a}.Login(&on)
	}
}

// settingsChanged redraws what shows the settings (Settings, the tray).
func (a *App) settingsChanged() {
	if a.win != nil {
		a.win.Invalidate()
	}
	a.trayChanged()
}

func (s prefs) Choice() hotkey.Choice { return s.a.choice }

// Apply registers the shortcut and keeps it; it syncs.
func (s prefs) Apply(c hotkey.Choice) {
	a := s.a
	a.useKey(c)
	a.kv.Set(store.KeyHotkey, c.Name())
	a.touched(true)
	a.settingsChanged()
}

// KeyProblem: the shortcut couldn't be registered (taken on X11, or no
// GlobalShortcuts portal on Wayland).
func (s prefs) KeyProblem() string {
	if s.a.hotkeyErr == nil {
		return ""
	}
	return s.a.choice.String() + " can't be used here."
}

// Login reports whether Nanolight opens at login. A copy of the Zig build
// left its own autostart entry; it counts, and goes once this one is set.
func (s prefs) Login(set *bool) bool {
	a := s.a
	if set != nil {
		if err := mygo.App.SetOpenAtLogin(*set); err != nil {
			log.Printf("nanolight: open at login: %v", err)
		} else {
			removeZigAutostart()
		}
		a.login = nil
	}
	if a.login == nil {
		on := mygo.App.OpenAtLogin() || zigAutostart()
		a.login = &on
	}
	return *a.login
}

// zigAutostartPath is the Zig build's entry (login.zig).
func zigAutostartPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "autostart", "nanolight.desktop")
}

// zigAutostart reports whether the Zig build's entry is there (not mygo's,
// which passes its login argument).
func zigAutostart() bool {
	data, err := os.ReadFile(zigAutostartPath())
	return err == nil && !strings.Contains(string(data), "--mygo-opened-at-login")
}

func removeZigAutostart() {
	if zigAutostart() {
		os.Remove(zigAutostartPath())
	}
}

// Suggest keeps Google Suggestions on or off; it syncs.
func (s prefs) Suggest(on bool) {
	s.a.kv.SetBool(store.KeySuggest, on)
	s.a.touched(true)
	s.a.settingsChanged()
}

// SearchesOut keeps where Return sends a Google search.
func (s prefs) SearchesOut(on bool) { s.a.kv.SetBool(store.KeySearchesOut, on) }

// Tray reports whether the tray icon shows, and whether it can.
func (s prefs) Tray(set *bool) (bool, bool) {
	a := s.a
	if set != nil {
		a.kv.SetBool(store.KeyMenuBar, *set)
		a.showTray(*set)
	}
	return a.kv.Bool(store.KeyMenuBar, true), a.trayCan
}

func (s prefs) Account() view.AccountState { return s.a.accountState() }
func (s prefs) Email() string              { return s.a.accountEmail() }
func (s prefs) SignIn()                    { s.a.signIn() }
func (s prefs) SignOut()                   { s.a.signOut() }
func (s prefs) Terms()                     { s.a.OpenURL(termsURL) }
func (s prefs) Version() string            { return Version() }
func (s prefs) Ready() string              { return s.a.updateReady() }
func (s prefs) Updatable() bool            { return s.a.updatesOn() }
func (s prefs) Check()                     { s.a.checkUpdates() }
func (s prefs) Restart()                   { s.a.restartToUpdate() }
func (s prefs) Welcome()                   { s.a.showWelcome() }
func (s prefs) Quit()                      { mygo.App.Quit() }
