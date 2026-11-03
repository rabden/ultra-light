package apps

import (
	"testing"
)

func TestIndex(t *testing.T) {
	var ix Index
	for _, n := range []string{"Firefox", "files", "Files", "Calculator", "Terminal"} {
		ix.Add(Entry{Name: n, Folder: "Application"})
	}
	ix.Sort()
	if len(ix.Entries) != 4 || ix.Names[0] != "Calculator" || ix.Names[1] != "files" {
		t.Fatalf("index %q", ix.Names)
	}
	// Nothing opened yet: the starters there are, in their order.
	if s := ix.Suggestions(3); len(s) != 3 || ix.Names[s[0]] != "files" || ix.Names[s[1]] != "Firefox" || ix.Names[s[2]] != "Terminal" {
		t.Fatalf("starters %v", s)
	}
	ix.Opened(0)
	enc := ix.Opened(0)
	if s := ix.Suggestions(3); ix.Names[s[0]] != "Calculator" {
		t.Fatalf("most opened first: %v", s)
	}
	if enc != "Calculator\t2\n" {
		t.Errorf("encoded %q", enc)
	}
	ix.Launches[0] = 0
	ix.LoadLaunches(enc + "Gone\t9\nbad line\n")
	if ix.Launches[0] != 2 {
		t.Error("read back")
	}
	ix.Launches[0] = 65535
	ix.Opened(0)
	if ix.Launches[0] != 65535 {
		t.Error("saturates")
	}
}
