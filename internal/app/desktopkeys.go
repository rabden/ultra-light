package app

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rabden/ultra-light/internal/hotkey"
	"github.com/rabden/ultra-light/internal/store"
)

// The desktop's own shortcuts (GNOME's gsettings, Plasma's
// kglobalshortcutsrc): a preset one of them holds reaches the desktop, not
// Nanolight, so it is shown as taken. GNOME's can be taken over: the key
// comes out of GNOME's setting, which is kept as it was and put back once
// Nanolight uses another key.

// deskKeysStale is how old the desktop's shortcuts may be before they are
// read again (Settings and the welcome ask on every frame).
const deskKeysStale = 5 * time.Second

type deskKeys struct {
	list    []hotkey.Binding
	at      time.Time
	loading bool
}

// readDeskKeys reads the desktop's shortcuts; slow-ish (a process per
// GNOME schema), so off the main thread after the first time.
func readDeskKeys() []hotkey.Binding {
	var bs []hotkey.Binding
	if _, err := exec.LookPath("gsettings"); err == nil {
		for _, schema := range hotkey.GNOMESchemas {
			out, err := exec.Command("gsettings", "list-recursively", schema).Output()
			if err == nil {
				bs = append(bs, hotkey.ParseGSettings(string(out))...)
			}
		}
		bs = append(bs, gnomeCustomKeys()...)
	}
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	if rc, err := os.ReadFile(filepath.Join(dir, "kglobalshortcutsrc")); err == nil {
		bs = append(bs, hotkey.ParseKDE(string(rc))...)
	}
	return bs
}

// gnomeCustomKeys are the shortcuts the person made in GNOME's settings
// (relocatable, so list-recursively doesn't show them). Theirs: never
// taken over.
func gnomeCustomKeys() []hotkey.Binding {
	const schema = "org.gnome.settings-daemon.plugins.media-keys"
	out, err := exec.Command("gsettings", "get", schema, "custom-keybindings").Output()
	if err != nil {
		return nil
	}
	paths := hotkey.ParseGSettings("x y " + strings.TrimSpace(string(out)))
	if len(paths) == 0 {
		return nil
	}
	var bs []hotkey.Binding
	for _, path := range paths[0].Accels {
		where := schema + ".custom-keybinding:" + path
		get := func(key string) string {
			v, _ := exec.Command("gsettings", "get", where, key).Output()
			return strings.Trim(strings.TrimSpace(string(v)), `'"`)
		}
		if b := get("binding"); b != "" {
			bs = append(bs, hotkey.Binding{Desktop: "GNOME", Schema: "custom:" + path, Key: "binding",
				What: "is your “" + get("name") + "” shortcut", Accels: []string{b}})
		}
	}
	return bs
}

// desktopKeys are the desktop's shortcuts, read again in the background
// when old.
func (a *App) desktopKeys() []hotkey.Binding {
	d := &a.dkeys
	if d.at.IsZero() {
		d.list, d.at = readDeskKeys(), time.Now()
	} else if time.Since(d.at) > deskKeysStale && !d.loading {
		d.loading = true
		go func() {
			list := readDeskKeys()
			a.win.Update(func() {
				d.list, d.at, d.loading = list, time.Now(), false
				a.win.Invalidate()
			})
		}()
	}
	return d.list
}

// clash is the desktop's shortcut that holds c.
func (a *App) clash(c hotkey.Choice) (hotkey.Binding, bool) {
	return hotkey.Clash(c, a.desktopKeys())
}

// canFree: Nanolight can take b's key over (GNOME's own shortcuts, not
// the ones the person made, nor Plasma's).
func canFree(b hotkey.Binding) bool {
	return b.Desktop == "GNOME" && !strings.HasPrefix(b.Schema, "custom:")
}

// keyTakenBy says what holds c, for Settings and the welcome.
func (a *App) keyTakenBy(c hotkey.Choice) (string, bool) {
	b, ok := a.clash(c)
	if !ok {
		return "", false
	}
	return b.What, canFree(b)
}

// freed is one GNOME shortcut as it was before Nanolight took its key.
type freed struct {
	Schema string `json:"schema"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

func (a *App) freedKeys() []freed {
	var fs []freed
	if v, ok := a.kv.Get(store.KeyFreed); ok {
		json.Unmarshal([]byte(v), &fs)
	}
	return fs
}

func (a *App) setFreed(fs []freed) {
	if len(fs) == 0 {
		a.kv.Remove(store.KeyFreed)
		return
	}
	b, _ := json.Marshal(fs)
	a.kv.Set(store.KeyFreed, string(b))
}

// takeKey takes c from GNOME's shortcuts that hold it, keeping each as it
// was, then uses it.
func (a *App) takeKey(c hotkey.Choice) {
	fs := a.freedKeys()
	for range 4 {
		b, ok := hotkey.Clash(c, readDeskKeys())
		if !ok || !canFree(b) {
			break
		}
		if err := exec.Command("gsettings", "set", b.Schema, b.Key, b.Without(c)).Run(); err != nil {
			log.Printf("nanolight: taking %s from %s %s: %v", c, b.Schema, b.Key, err)
			break
		}
		kept := false
		for _, f := range fs {
			kept = kept || f.Schema == b.Schema && f.Key == b.Key
		}
		if !kept {
			fs = append(fs, freed{b.Schema, b.Key, b.Value()})
		}
		log.Printf("nanolight: took %s from GNOME's %s", c, b.Key)
	}
	a.setFreed(fs)
	a.dkeys.list, a.dkeys.at = readDeskKeys(), time.Now()
	prefs{a}.Apply(c)
}

// giveBack puts back the GNOME shortcuts Nanolight took a key from, but
// the ones holding c (the key it uses now).
func (a *App) giveBack(c hotkey.Choice) {
	fs := a.freedKeys()
	if len(fs) == 0 {
		return
	}
	var keep []freed
	for _, f := range fs {
		was := hotkey.ParseGSettings(f.Schema + " " + f.Key + " " + f.Value)
		if _, holds := hotkey.Clash(c, was); holds {
			keep = append(keep, f)
			continue
		}
		if err := exec.Command("gsettings", "set", f.Schema, f.Key, f.Value).Run(); err != nil {
			log.Printf("nanolight: giving back GNOME's %s: %v", f.Key, err)
			keep = append(keep, f)
			continue
		}
		log.Printf("nanolight: gave GNOME's %s back", f.Key)
	}
	a.setFreed(keep)
	a.dkeys.at = time.Time{}
}
