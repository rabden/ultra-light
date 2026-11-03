package hotkey

import "testing"

func TestPresets(t *testing.T) {
	for i, want := range []string{"Alt Space", "Ctrl Alt Space", "Ctrl Space", "Ctrl Shift Space"} {
		c := All[i]
		if c.String() != want {
			t.Errorf("%d: %q, want %q", i, c.String(), want)
		}
		if p, ok := Parse(c.Name()); !ok || p != c {
			t.Errorf("%s doesn't round-trip", c.Name())
		}
	}
	if Load("key:49:256:0") != CmdSpace || Load("") != CmdSpace || Load("ctrl_space") != CtrlSpace {
		t.Error("fallbacks")
	}
	if CmdSpace.Step(-1) != OptCmdSpace || OptCmdSpace.Step(1) != CmdSpace || CmdSpace.Step(1) != OptSpace {
		t.Error("steps wrap")
	}
	if OptSpace.Accelerator() != "Ctrl+Alt+Space" || len(OptCmdSpace.Mods()) != 2 {
		t.Error("accelerators")
	}
}
