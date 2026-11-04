package hotkey

import (
	"sort"
	"strconv"
	"strings"
)

// The desktop's own shortcuts. A key the desktop keeps for itself (GNOME's
// window menu on Alt Space, KRunner on Plasma) reaches the desktop first:
// Nanolight can ask for it, through the portal or a key grab, and never
// hear it. These read the desktop's settings, so the presets it holds can
// be said to be taken, and GNOME's handed over when asked.

// Binding is one of the desktop's shortcuts.
type Binding struct {
	Desktop string // "GNOME" or "KDE"
	// GNOME: the gsettings schema and key. KDE: the component and action.
	Schema, Key string
	// What it does, said plainly: "opens GNOME's window menu".
	What string
	// Accels are its keys as the desktop writes them: "<Alt>space",
	// "Alt+Space".
	Accels []string
}

// gnomeWhat says what GNOME's best-known shortcuts do.
var gnomeWhat = map[string]string{
	"activate-window-menu":         "opens GNOME's window menu",
	"switch-input-source":          "switches the keyboard layout",
	"switch-input-source-backward": "switches the keyboard layout",
	"toggle-overview":              "opens the Activities overview",
	"toggle-application-view":      "opens GNOME's app grid",
	"panel-run-dialog":             "opens GNOME's Run dialog",
	"triggers":                     "switches the input method",
}

// GNOMESchemas are the schemas that hold GNOME's shortcuts.
var GNOMESchemas = []string{
	"org.gnome.desktop.wm.keybindings",
	"org.gnome.shell.keybindings",
	"org.gnome.mutter.keybindings",
	"org.gnome.mutter.wayland.keybindings",
	"org.gnome.settings-daemon.plugins.media-keys",
	"org.freedesktop.ibus.general.hotkey",
}

// ParseGSettings reads `gsettings list-recursively <schema>`: one shortcut
// per line, "schema key ['<Alt>space', …]". Lines that hold no key list
// are left out.
func ParseGSettings(out string) []Binding {
	var bs []Binding
	for _, line := range strings.Split(out, "\n") {
		f := strings.SplitN(strings.TrimSpace(line), " ", 3)
		if len(f) < 3 {
			continue
		}
		accels, ok := parseStrv(f[2])
		if !ok || len(accels) == 0 {
			continue
		}
		what := gnomeWhat[f[1]]
		if what == "" {
			what = "is GNOME's “" + strings.ReplaceAll(f[1], "-", " ") + "” shortcut"
		}
		bs = append(bs, Binding{Desktop: "GNOME", Schema: f[0], Key: f[1], What: what, Accels: accels})
	}
	return bs
}

// parseStrv reads a GVariant string list: ['a', 'b'] or @as [].
func parseStrv(v string) ([]string, bool) {
	v = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v), "@as"))
	if !strings.HasPrefix(v, "[") || !strings.HasSuffix(v, "]") {
		return nil, false
	}
	var out []string
	rest := v[1 : len(v)-1]
	for {
		i := strings.IndexAny(rest, `'"`)
		if i < 0 {
			return out, true
		}
		q := rest[i]
		j := strings.IndexByte(rest[i+1:], q)
		if j < 0 {
			return nil, false
		}
		out = append(out, rest[i+1:i+1+j])
		rest = rest[i+2+j:]
	}
}

// ParseKDE reads Plasma's kglobalshortcutsrc: [component] sections of
// "action=Current\tOther,Default,Friendly name".
func ParseKDE(rc string) []Binding {
	var bs []Binding
	comp, compName := "", ""
	type pending struct{ action, current, friendly string }
	var acts []pending
	flush := func() {
		for _, p := range acts {
			var accels []string
			// The file writes the tab between keys as \t.
			for _, k := range strings.Split(strings.ReplaceAll(p.current, `\t`, "\t"), "\t") {
				if k = strings.TrimSpace(k); k != "" && k != "none" {
					accels = append(accels, k)
				}
			}
			if len(accels) == 0 {
				continue
			}
			who := compName
			if who == "" {
				who = comp
			}
			what := "is " + who + "'s shortcut"
			if p.friendly != "" {
				what = "is the “" + p.friendly + "” shortcut of " + who
			}
			bs = append(bs, Binding{Desktop: "KDE", Schema: comp, Key: p.action, What: what, Accels: accels})
		}
		acts = nil
	}
	for _, line := range strings.Split(rc, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			comp, compName = line[1:len(line)-1], ""
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if k == "_k_friendly_name" {
			compName = v
			continue
		}
		f := strings.SplitN(v, ",", 3)
		p := pending{action: k, current: f[0]}
		if len(f) == 3 {
			p.friendly = f[2]
		}
		acts = append(acts, p)
	}
	flush()
	return bs
}

// mods are the modifiers a choice holds, by name.
func (c Choice) modSet() string {
	m := strings.Fields(strings.ToLower(c.Held()))
	sort.Strings(m)
	return strings.Join(m, "+")
}

// accelMods reads "<Primary><Alt>space" or "Ctrl+Alt+Space" as its
// modifiers ("alt+ctrl") and key ("space").
func accelMods(a string) (mods, key string) {
	var m []string
	add := func(s string) {
		switch strings.ToLower(s) {
		case "alt", "mod1":
			m = append(m, "alt")
		case "primary", "control", "ctrl":
			m = append(m, "ctrl")
		case "shift":
			m = append(m, "shift")
		case "super", "meta", "mod4", "hyper":
			m = append(m, "super")
		default:
			m = append(m, "?"+strings.ToLower(s))
		}
	}
	if strings.HasPrefix(a, "<") {
		for strings.HasPrefix(a, "<") {
			j := strings.IndexByte(a, '>')
			if j < 0 {
				break
			}
			add(a[1:j])
			a = a[j+1:]
		}
		key = a
	} else {
		parts := strings.Split(a, "+")
		for _, p := range parts[:len(parts)-1] {
			add(p)
		}
		key = parts[len(parts)-1]
	}
	sort.Strings(m)
	return strings.Join(m, "+"), strings.ToLower(strings.TrimSpace(key))
}

// Holds reports whether accel is this choice's key.
func (c Choice) Holds(accel string) bool {
	mods, key := accelMods(accel)
	return key == "space" && mods == c.modSet()
}

// Clash is the desktop shortcut that holds c, if one does.
func Clash(c Choice, bs []Binding) (Binding, bool) {
	for _, b := range bs {
		for _, a := range b.Accels {
			if c.Holds(a) {
				return b, true
			}
		}
	}
	return Binding{}, false
}

// Without is the GNOME value of b with c's keys taken out, as gsettings
// sets it: ['<Alt>F3'] or @as [].
func (b Binding) Without(c Choice) string {
	var keep []string
	for _, a := range b.Accels {
		if !c.Holds(a) {
			keep = append(keep, strconv.Quote(a))
		}
	}
	return strv(keep)
}

// Value is b's keys as gsettings sets them.
func (b Binding) Value() string {
	var q []string
	for _, a := range b.Accels {
		q = append(q, strconv.Quote(a))
	}
	return strv(q)
}

// strv writes a GVariant string list of already quoted strings.
func strv(quoted []string) string {
	if len(quoted) == 0 {
		return "@as []"
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
