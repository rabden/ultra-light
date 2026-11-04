package hotkey

import "testing"

const gnomeOut = `org.gnome.desktop.wm.keybindings activate-window-menu ['<Alt>space']
org.gnome.desktop.wm.keybindings always-on-top @as []
org.gnome.desktop.wm.keybindings switch-input-source ['<Super>space', 'XF86Keyboard']
org.gnome.desktop.wm.keybindings close ['<Alt>F4']
org.freedesktop.ibus.general.hotkey triggers ['<Control>space']
org.gnome.desktop.wm.keybindings num-workspaces 4
`

const kdeRC = `[org.kde.krunner.desktop]
_k_friendly_name=KRunner
RunClipboard=Alt+Shift+F2,Alt+Shift+F2,Run command on clipboard contents
_launch=Alt+Space\tAlt+F2\tSearch,Alt+Space\tAlt+F2\tSearch,KRunner

[kwin]
_k_friendly_name=KWin
Expose=Ctrl+F9,Ctrl+F9,Toggle Present Windows (Current desktop)
`

func TestGNOMEClashes(t *testing.T) {
	bs := ParseGSettings(gnomeOut)
	if len(bs) != 4 {
		t.Fatalf("%d bindings: %+v", len(bs), bs)
	}
	b, ok := Clash(OptSpace, bs)
	if !ok || b.Key != "activate-window-menu" || b.What != "opens GNOME's window menu" {
		t.Fatalf("Alt Space: %+v %v", b, ok)
	}
	if b.Without(OptSpace) != "@as []" || b.Value() != `["<Alt>space"]` {
		t.Errorf("values %q %q", b.Without(OptSpace), b.Value())
	}
	if b, ok := Clash(CmdSpace, bs); !ok || b.What != "switches the keyboard layout" {
		t.Errorf("Super Space: %+v", b)
	}
	if b, ok := Clash(CtrlSpace, bs); !ok || b.What != "switches the input method" {
		t.Errorf("Ctrl Space: %+v", b)
	}
	for _, c := range []Choice{OptCmdSpace} {
		if b, ok := Clash(c, bs); ok {
			t.Errorf("%s taken by %+v", c, b)
		}
	}
	two := Binding{Accels: []string{"<Alt>space", "<Alt>F3"}}
	if two.Without(OptSpace) != `["<Alt>F3"]` {
		t.Errorf("without: %s", two.Without(OptSpace))
	}
}

func TestKDEClashes(t *testing.T) {
	bs := ParseKDE(kdeRC)
	b, ok := Clash(OptSpace, bs)
	if !ok || b.Desktop != "KDE" || b.What != "is the “KRunner” shortcut of KRunner" {
		t.Fatalf("Alt Space: %+v %v", b, ok)
	}
	if _, ok := Clash(CmdSpace, bs); ok {
		t.Error("Super Space is free")
	}
}

func TestAccels(t *testing.T) {
	for _, tc := range []struct {
		c    Choice
		a    string
		want bool
	}{
		{OptSpace, "<Alt>space", true},
		{OptSpace, "<Mod1>Space", true},
		{OptSpace, "Alt+Space", true},
		{OptCmdSpace, "<Alt><Super>space", true},
		{OptCmdSpace, "Meta+Alt+Space", true},
		{CmdSpace, "<Super>space", true},
		{CmdSpace, "Meta+Space", true},
		{CtrlSpace, "<Control><Shift>space", false},
		{CmdSpace, "<Alt>space", false},
		{OptSpace, "<Alt>F2", false},
	} {
		if got := tc.c.Holds(tc.a); got != tc.want {
			t.Errorf("%s holds %q: %v", tc.c, tc.a, got)
		}
	}
}
