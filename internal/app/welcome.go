package app

import (
	"github.com/rabden/ultra-light/internal/hotkey"
	"github.com/rabden/ultra-light/internal/store"
	"github.com/rabden/ultra-light/internal/view"
)

// welcomeHost answers the welcome (main.zig's onboard hooks).
type welcomeHost struct{ a *App }

// startWelcome shows the welcome on the first run (no nanolight.onboarded).
func (a *App) startWelcome() {
	a.panel.Welcome.Hooks = welcomeHost{a}
	if !a.kv.Has(store.KeyOnboarded) {
		a.firstFreeKey()
		a.panel.Welcome.Start(a.choice)
	}
}

// firstFreeKey: on a first run where the default key is the desktop's
// (Alt Space: GNOME's window menu, KRunner), the first preset that's free
// is chosen instead (decision 3): one none of the desktop's shortcuts
// holds, and on X11 one no other app grabbed.
func (a *App) firstFreeKey() {
	if a.kv.Has(store.KeyHotkey) {
		return
	}
	if _, taken := a.clash(a.choice); !taken && a.hotkeyErr == nil {
		return
	}
	for _, c := range hotkey.All {
		if _, taken := a.clash(c); taken || c == a.choice || !Wayland() && a.keyTaken(c) {
			continue
		}
		a.useKey(c)
		if a.hotkeyErr == nil {
			a.kv.Set(store.KeyHotkey, c.Name())
			return
		}
	}
}

// welcomeShowing looks again at the keys as the panel shows.
func (a *App) welcomeShowing() {
	if a.panel.Welcome.Active {
		a.panel.Welcome.Refresh()
	}
}

// welcomePressed: the shortcut or nanolight --toggle fired, the panel
// showing before it (step 2 counts the presses).
func (a *App) welcomePressed(showing bool) {
	a.panel.Welcome.Pressed(showing)
	a.win.Invalidate()
}

// showWelcome replays the welcome (Settings, the tray), the panel shown.
func (a *App) showWelcome() {
	a.panel.Settings.Hide()
	a.panel.Welcome.Start(a.choice)
	if !a.win.IsVisible() {
		a.Show()
	}
	a.win.Invalidate()
}

func (h welcomeHost) Apply(c hotkey.Choice)                   { prefs(h).Apply(c) }
func (h welcomeHost) Login(set *bool) bool                    { return prefs(h).Login(set) }
func (h welcomeHost) Done()                                   { h.a.kv.Set(store.KeyOnboarded, "1") }
func (h welcomeHost) KeyTaken(c hotkey.Choice) (string, bool) { return h.a.keyTakenBy(c) }
func (h welcomeHost) TakeKey(c hotkey.Choice)                 { h.a.takeKey(c) }

// KeyState: on X11 the key is grabbed and let go to see; on Wayland the
// desktop's portal holds the keys, or there is none and no key works.
func (h welcomeHost) KeyState(c hotkey.Choice) view.KeyState {
	a := h.a
	if _, taken := a.clash(c); taken {
		return view.KeyTaken
	}
	if Wayland() {
		if a.hotkeyErr != nil {
			return view.KeyUnusable
		}
		return view.KeyDesktop
	}
	if c == a.choice && a.accel != "" {
		return view.KeyFree
	}
	if a.keyTaken(c) {
		return view.KeyHeld
	}
	return view.KeyFree
}
