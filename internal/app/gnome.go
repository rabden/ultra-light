package app

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/rabden/ultra-light/internal/hotkey"
)

// On GNOME the shortcut is one of GNOME's own custom shortcuts (Settings ›
// Keyboard › Custom Shortcuts), "Nanolight", running nanolight --toggle:
// GNOME's portal binds keys only after a dialog, and not at all on older
// GNOME, while its own shortcuts always work, on Wayland and X11.

const (
	mediaKeys = "org.gnome.settings-daemon.plugins.media-keys"
	ownPath   = "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/nanolight/"
)

// onGNOME: the desktop is GNOME and its settings can be written.
func onGNOME() bool {
	if !strings.Contains(strings.ToUpper(os.Getenv("XDG_CURRENT_DESKTOP")), "GNOME") {
		return false
	}
	_, err := exec.LookPath("gsettings")
	return err == nil
}

func gsettingsGet(schema, key string) string {
	out, _ := exec.Command("gsettings", "get", schema, key).Output()
	return strings.TrimSpace(string(out))
}

func gsettingsSet(schema, key, value string) error {
	if out, err := exec.Command("gsettings", "set", schema, key, value).CombinedOutput(); err != nil {
		return fmt.Errorf("gsettings set %s %s: %v: %s", schema, key, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// customPaths are GNOME's custom shortcuts, by their dconf paths.
func customPaths() []string {
	bs := hotkey.ParseGSettings("x y " + gsettingsGet(mediaKeys, "custom-keybindings"))
	if len(bs) == 0 {
		return nil
	}
	return bs[0].Accels
}

func setCustomPaths(paths []string) error {
	q := make([]string, len(paths))
	for i, p := range paths {
		q[i] = strconv.Quote(p)
	}
	v := "@as []"
	if len(q) > 0 {
		v = "[" + strings.Join(q, ", ") + "]"
	}
	return gsettingsSet(mediaKeys, "custom-keybindings", v)
}

// toggleCommand is what GNOME runs: this program, with --toggle (the
// AppImage when there is one), quoted as GNOME splits it.
func toggleCommand() string {
	exe := os.Getenv("APPIMAGE")
	if exe == "" {
		exe, _ = os.Executable()
	}
	return "'" + strings.ReplaceAll(exe, "'", `'\''`) + "' --toggle"
}

// gnomeBind makes, or changes, Nanolight's own GNOME shortcut.
func gnomeBind(c hotkey.Choice) error {
	where := mediaKeys + ".custom-keybinding:" + ownPath
	for _, kv := range [][2]string{
		{"name", "'Nanolight'"},
		{"command", strconv.Quote(toggleCommand())},
		{"binding", "'" + c.GTK() + "'"},
	} {
		if err := gsettingsSet(where, kv[0], kv[1]); err != nil {
			return err
		}
	}
	if paths := customPaths(); !slices.Contains(paths, ownPath) {
		return setCustomPaths(append(paths, ownPath))
	}
	return nil
}

// gnomeUnbind takes Nanolight's GNOME shortcut away.
func gnomeUnbind() {
	paths := customPaths()
	if i := slices.Index(paths, ownPath); i >= 0 {
		setCustomPaths(slices.Delete(paths, i, i+1))
	}
	where := mediaKeys + ".custom-keybinding:" + ownPath
	for _, k := range []string{"name", "command", "binding"} {
		exec.Command("gsettings", "reset", where, k).Run()
	}
}
