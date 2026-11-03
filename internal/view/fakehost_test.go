package view

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/rabden/ultra-light/internal/apps"
)

// fakeHost records what the panel asks of the app.
type fakeHost struct {
	opened   []string
	urls     []string
	hidden   int
	heights  []float32
	asked    []string
	copied   string
	launches string
	page     string
	pageOK   bool
	pageBack int
}

func (h *fakeHost) Open(e apps.Entry) bool {
	h.opened = append(h.opened, e.Name)
	return e.Kind != apps.App
}
func (h *fakeHost) OpenURL(u string)           { h.urls = append(h.urls, u) }
func (h *fakeHost) Hide()                      { h.hidden++ }
func (h *fakeHost) Resize(v float32)           { h.heights = append(h.heights, v) }
func (h *fakeHost) Suggest(q string)           { h.asked = append(h.asked, q) }
func (h *fakeHost) Copy(t string)              { h.copied = t }
func (h *fakeHost) SaveLaunches(s string)      { h.launches = s }
func (h *fakeHost) Icon(apps.Entry) *ui.Bitmap { return nil }
func (h *fakeHost) ShowPage(u string) bool {
	if !h.pageOK {
		return false
	}
	h.page = u
	return true
}
func (h *fakeHost) HidePage()              { h.page = "" }
func (h *fakeHost) Page() ui.NativeWebView { return nil }
func (h *fakeHost) PageURL() string        { return h.page }
func (h *fakeHost) PageBack()              { h.pageBack++ }

func testIndex() *apps.Index {
	ix := &apps.Index{}
	for _, n := range []string{"Firefox", "Files", "Terminal", "Settings", "Text Editor", "Safari", "System Monitor", "Calculator", "LibreOffice Calc"} {
		ix.Add(apps.Entry{Name: n, Folder: "Application", Path: "/usr/share/applications/" + n + ".desktop"})
	}
	ix.Sort()
	return ix
}

type rig struct {
	t   *testing.T
	p   *Panel
	h   *fakeHost
	tt  *ui.Tester
	now time.Time
}

func newRig(t *testing.T) *rig {
	h := &fakeHost{pageOK: true}
	p := NewPanel(h, testIndex())
	r := &rig{t: t, p: p, h: h, now: time.Unix(1000, 0)}
	p.SetClock(func() time.Time { return r.now })
	p.Welcome.Active = false
	r.tt = ui.NewTester(p.View, 680, 674)
	p.Reveal()
	r.tt.Frame()
	r.tt.Advance(time.Second)
	return r
}

func (r *rig) typ(s string) {
	r.tt.Type(s)
	r.tt.Advance(time.Second)
}

func (r *rig) snap(name string) {
	if os.Getenv("NANOLIGHT_SNAPSHOTS") == "" {
		return
	}
	dir := os.Getenv("NANOLIGHT_SNAPSHOTS")
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		r.t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, r.tt.Image())
}
