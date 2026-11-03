package app

import (
	"log"
	"os/exec"
	"strings"

	"github.com/egoist/mygo"

	"github.com/rabden/ultra-light/internal/store"
	"github.com/rabden/ultra-light/internal/view"
	"github.com/rabden/ultra-light/resources"
)

// The tray icon (the Mac's menu bar item, decision 5): on by default where
// the desktop shows tray icons, "Show in Tray" turning it off.

func (a *App) startTray() {
	a.trayCan = trayWatcher()
	a.showTray(a.kv.Bool(store.KeyMenuBar, true))
}

// trayWatcher reports whether something shows tray icons
// (StatusNotifierWatcher: KDE, GNOME with the AppIndicator extension,
// Waybar…). Without gdbus to ask, it is taken to be there.
func trayWatcher() bool {
	out, err := exec.Command("gdbus", "call", "--session", "--dest", "org.freedesktop.DBus", "--object-path", "/org/freedesktop/DBus",
		"--method", "org.freedesktop.DBus.NameHasOwner", "org.kde.StatusNotifierWatcher").Output()
	if err != nil {
		return true
	}
	return strings.Contains(string(out), "true")
}

// showTray adds or removes the icon.
func (a *App) showTray(on bool) {
	if !on {
		if a.tray != nil {
			a.tray.Destroy()
			a.tray = nil
		}
		return
	}
	if a.tray != nil || !a.trayCan {
		return
	}
	t, err := mygo.NewTray(mygo.TrayOptions{ID: "nanolight", Icon: resources.Icon256, ToolTip: "Nanolight", Menu: a.trayMenu()})
	if err != nil {
		log.Printf("nanolight: no tray icon: %v", err)
		a.trayCan = false
		return
	}
	a.tray = t
}

// trayChanged builds the menu again (the account, the suggestions, an
// update).
func (a *App) trayChanged() {
	if a.tray != nil {
		a.tray.SetMenu(a.trayMenu())
	}
}

func (a *App) trayMenu() *mygo.Menu {
	item := func(label string, click func()) *mygo.MenuItem {
		return &mygo.MenuItem{Label: label, Click: func(*mygo.MenuItem, *mygo.Window) { click() }}
	}
	sep := &mygo.MenuItem{Type: mygo.MenuItemSeparator}
	settings := item("Settings…", func() {
		// The welcome steps aside (it comes back at the next launch).
		a.panel.Welcome.Active = false
		a.Show()
		a.panel.OpenSettings()
		a.win.Invalidate()
	})
	settings.Accelerator = "Ctrl+,"
	suggest := &mygo.MenuItem{Label: "Google Suggestions", Type: mygo.MenuItemCheckbox, Checked: a.panel.SuggestOn,
		Click: func(it *mygo.MenuItem, _ *mygo.Window) {
			a.panel.SuggestOn = it.Checked
			prefs{a}.Suggest(it.Checked)
			a.panel.Refresh()
		}}
	terms := item("Codegraff Terms & Privacy", func() { a.OpenURL(termsURL) })
	var acct []*mygo.MenuItem
	switch a.accountState() {
	case view.SignedOut:
		acct = []*mygo.MenuItem{item("Sign In with Codegraff to Sync Across Computers…", a.accountClick), terms}
	case view.Starting:
		acct = []*mygo.MenuItem{{Label: "Signing In…", Disabled: true}, terms}
	case view.Waiting:
		acct = []*mygo.MenuItem{item("Approve in Your Browser… (Click to Cancel)", a.accountClick), terms}
	case view.SignedIn:
		who := a.accountEmail()
		if who == "" {
			who = "your Codegraff account"
		}
		acct = []*mygo.MenuItem{{Label: "Synced as " + who, Disabled: true}, item("Sign Out of Codegraff", a.accountClick)}
	}
	update := item("Check for Updates", a.checkUpdates)
	if v := a.updateReady(); v != "" {
		update = item("Restart to Update to "+v, a.restartToUpdate)
	}
	update.Disabled = !a.updatesOn()
	items := []*mygo.MenuItem{settings, item("Open Nanolight", a.Show), item("Welcome…", a.showWelcome), suggest, sep}
	items = append(items, acct...)
	items = append(items, &mygo.MenuItem{Type: mygo.MenuItemSeparator}, update, item("Quit Nanolight", mygo.App.Quit))
	return mygo.NewMenu(items)
}
