package view

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/rabden/ultra-light/internal/raycast"
)

// heard keeps what the screen said to its process.
type heard struct{ lines []string }

func (h *heard) said(line []byte) { h.lines = append(h.lines, string(line)) }
func (h *heard) has(s string) bool {
	for _, l := range h.lines {
		if strings.Contains(l, s) {
			return true
		}
	}
	return false
}

func str(s string) *string { return &s }

func TestExtOutputThenSearchAgain(t *testing.T) {
	r := newRig(t)
	h := &heard{}
	r.p.Ext.Start("System Info", nil, h.said)
	r.settle()
	// As a script prints: its first line, then the rest.
	for _, sofar := range []string{"", "Linux 6.17 on x86_64\n", "Linux 6.17 on x86_64\nUp: 3 min\n\nFilesystem Size Used\n/dev/vda1 10G 2G"} {
		r.p.Ext.Show(&raycast.View{Kind: "detail", Title: "System Info", Output: str(sofar), Loading: len(sofar) < 40})
		r.tt.Frame()
		r.tt.Advance(200 * time.Millisecond)
	}
	r.settle()
	r.snap("ext-output")
	for _, s := range []string{"Linux 6.17 on x86_64", "Up: 3 min", "/dev/vda1 10G 2G", "System Info"} {
		if !r.tt.HasText(s) {
			t.Errorf("lacks %q", s)
		}
	}
	r.tt.Key(0, ui.KeyEscape)
	if !h.has(`"pop"`) {
		t.Fatal("Esc pops")
	}
	// The runner stops the screen on a pop; the search has the keys again.
	r.p.Ext.Stop()
	r.settle()
	r.typ("gre")
	if r.p.Query() != "gre" {
		t.Fatalf("query %q", r.p.Query())
	}
}

const hnRender = `{"type":"render","view":{"kind":"list","sections":[{"title":"Front Page","items":[{"id":"a","title":"Show HN: Nanolight","subtitle":"rach","icon":{"text":"1"},"accessories":["128"],"actions":[{"id":"open","title":"Open in Browser"},{"id":"copy","title":"Copy Link"}]},{"id":"b","title":"Zig 0.17","actions":[{"id":"open2","title":"Open in Browser"}]}]}]}}`

func TestExtListActions(t *testing.T) {
	r := newRig(t)
	h := &heard{}
	r.p.Ext.Start("Hacker News", nil, h.said)
	m, err := raycast.Parse([]byte(hnRender))
	if err != nil {
		t.Fatal(err)
	}
	r.p.Ext.Show(m.View)
	r.settle()
	r.snap("ext-list")
	for _, s := range []string{"Front Page", "Show HN: Nanolight", "rach", "128", "Open in Browser", "Actions"} {
		if !r.tt.HasText(s) {
			t.Errorf("lacks %q", s)
		}
	}
	if !r.tt.Focused("Search") {
		t.Fatal("the command's search has the keys")
	}
	// Typing searches.
	r.typ("zig")
	if !h.has(`{"type":"search","text":"zig"}`) {
		t.Fatalf("search: %v", h.lines)
	}
	// ↓ selects the next, and says so.
	r.tt.Key(0, ui.KeyDown)
	if !h.has(`{"type":"select","id":"b"}`) {
		t.Fatal("select")
	}
	r.tt.Key(0, ui.KeyUp)
	// Ctrl+K: the actions; ↓ and Return runs the second.
	r.tt.Key(ui.Ctrl, ui.KeyK)
	r.settle()
	if !r.tt.HasText("Copy Link") || !r.p.Ext.actionsOpen {
		t.Fatal("actions")
	}
	r.snap("ext-actions")
	r.tt.Key(0, ui.KeyDown)
	r.tt.Key(0, ui.KeyEnter)
	if !h.has(`{"type":"action","id":"copy"}`) {
		t.Fatalf("action: %v", h.lines)
	}
	// Return on the list runs the first; Ctrl+↩ the second.
	r.settle()
	r.tt.Key(0, ui.KeyEnter)
	if !h.has(`{"type":"action","id":"open"}`) {
		t.Fatal("Return")
	}
	// Esc clears the search first, then pops.
	r.tt.Key(0, ui.KeyEscape)
	r.settle()
	if r.p.Ext.query != "" || h.has(`"pop"`) {
		t.Fatal("Esc clears first")
	}
	r.tt.Key(0, ui.KeyEscape)
	if !h.has(`"pop"`) {
		t.Fatal("then pops")
	}
}

func TestExtFormArguments(t *testing.T) {
	r := newRig(t)
	h := &heard{}
	r.p.Ext.Start("Greet", nil, h.said)
	r.p.Ext.Show(&raycast.View{Kind: "form", Title: "Greet",
		Fields:  []raycast.Field{{ID: "arg1", Type: "text", Title: "Name", Value: ""}, {ID: "arg2", Type: "dropdown", Title: "Lang", Value: "en", Options: []raycast.Option{{Value: "en", Title: "English"}, {Value: "fr", Title: "French"}}}},
		Actions: []raycast.Action{{ID: "run", Title: "Run"}}})
	r.settle()
	r.snap("ext-form")
	// Typed at once: the first field has the keys.
	r.typ("Rach")
	r.tt.Key(0, ui.KeyEnter)
	if !h.has(`"value":"Rach"`) || !h.has(`{"type":"action","id":"run"}`) {
		t.Fatalf("form: %v", h.lines)
	}
	if err := r.tt.Click("French"); err != nil {
		t.Fatal(err)
	}
	if !h.has(`{"type":"field","id":"arg2","value":"fr"}`) {
		t.Fatal("dropdown")
	}
}

func TestExtCompactHeading(t *testing.T) {
	r := newRig(t)
	r.p.Ext.Start("Greet", nil, nil)
	r.p.Ext.Show(&raycast.View{Kind: "detail", Title: "Greet", Markdown: str("# Hello, Rach - from a script")})
	r.p.Ext.Note("Exited with code 1", true)
	r.settle()
	if !r.tt.HasText("Hello, Rach - from a script") || !r.tt.HasText("Exited with code 1") {
		t.Fatal("heading and note")
	}
	r.now = r.now.Add(3 * time.Second)
	r.settle()
	if r.tt.HasText("Exited with code 1") {
		t.Fatal("the note goes after 2.5 s")
	}
}
