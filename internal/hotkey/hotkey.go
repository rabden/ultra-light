// Package hotkey names the four keys that can open the panel, as the Zig
// build keeps them (shortcut.zig): stored by name, shown as a PC keyboard
// says them, registered as mygo accelerators. The Mac's recorded keys
// ("key:<code>:<mods>:<char>") are Mac key codes, which a PC ignores.
package hotkey

import "strings"

// Choice is one of the four offered keys.
type Choice int

const (
	CmdSpace    Choice = iota // Alt Space on a PC, where ⌘ sits
	OptSpace                  // Ctrl Alt Space
	CtrlSpace                 // Ctrl Space
	OptCmdSpace               // Ctrl Shift Space
)

// All are the offered keys, in the order the chips show them.
var All = [...]Choice{CmdSpace, OptSpace, CtrlSpace, OptCmdSpace}

var names = [...]string{"cmd_space", "opt_space", "ctrl_space", "opt_cmd_space"}

// Name is how it is kept and synced.
func (c Choice) Name() string { return names[c] }

// Parse reads a kept name; ok is false for anything else, a Mac's recorded
// key included.
func Parse(s string) (Choice, bool) {
	for i, n := range names {
		if s == n {
			return Choice(i), true
		}
	}
	return CmdSpace, false
}

// Load reads a kept name, falling back to the default (Alt Space).
func Load(s string) Choice {
	c, _ := Parse(s)
	return c
}

// Held is the modifiers as a PC says them: "Alt", "Ctrl Alt".
func (c Choice) Held() string {
	switch c {
	case CmdSpace:
		return "Alt"
	case OptSpace:
		return "Ctrl Alt"
	case CtrlSpace:
		return "Ctrl"
	}
	return "Ctrl Shift"
}

// Mods are the modifier keys one by one, for the welcome's key caps.
func (c Choice) Mods() []string { return strings.Fields(c.Held()) }

// Key is the key itself: "Space".
func (c Choice) Key() string { return "Space" }

// String is the whole key as the panel says it: "Ctrl Alt Space".
func (c Choice) String() string { return c.Held() + " " + c.Key() }

// Accelerator is the key as mygo registers it.
func (c Choice) Accelerator() string {
	switch c {
	case CmdSpace:
		return "Alt+Space"
	case OptSpace:
		return "Ctrl+Alt+Space"
	case CtrlSpace:
		return "Ctrl+Space"
	}
	return "Ctrl+Shift+Space"
}

// Step returns the offered key before (dir < 0) or after it, wrapping.
func (c Choice) Step(dir int) Choice {
	n := Choice(len(All))
	if dir < 0 {
		return (c + n - 1) % n
	}
	return (c + 1) % n
}
