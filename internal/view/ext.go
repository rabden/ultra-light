package view

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/rabden/ultra-light/internal/core"
	"github.com/rabden/ultra-light/internal/raycast"
)

// A Raycast extension's command or a script command, drawn in the panel in
// place of the rows while it runs (raycast_view.zig): its list (a search
// field over sections of items), its detail (markdown as text, or a
// script's output) or its form, its actions in the footer (Return the
// first, Ctrl+↩ the second, Ctrl+K all of them). What it shows comes from
// its process as Views; keys and choices go back as lines (Sender).

const (
	extRowH     = 44
	extSectionH = 26
	extPad      = 6
	maxFields   = 12
	noteFor     = 2500 * time.Millisecond
)

// Sender takes a line for the command's process.
type Sender func(line []byte)

// Ext is the command screen's state.
type Ext struct {
	Open bool

	panel  *Panel
	name   string
	icon   *ui.Bitmap
	view   *raycast.View
	to     Sender
	starts int

	query string
	input ui.Handle
	sent  string
	sel   int

	actionsOpen bool
	actSel      int

	note    string
	noteBad bool
	noteAt  time.Time

	files map[string]*ui.Bitmap

	fields  [maxFields]string
	checks  [maxFields]bool
	inputs  [maxFields]ui.Handle
	formKey string
	focus   int // the field to give the keys to, or -1
}

// Start opens the screen for a command: its name, its extension's icon,
// and where its lines go.
func (e *Ext) Start(name string, icon *ui.Bitmap, to Sender) {
	e.Stop()
	e.name, e.icon, e.to = name, icon, to
	e.Open = true
	e.starts++
	e.query, e.sent = "", ""
	e.focus = -1
	e.input.Focus()
}

// Stop leaves the screen; the search has the keys again.
func (e *Ext) Stop() {
	wasOpen := e.Open
	e.Open = false
	e.view = nil
	e.sel = 0
	e.actionsOpen = false
	e.note = ""
	e.formKey = ""
	if wasOpen && e.panel != nil {
		e.panel.input.Focus()
	}
}

// View is the view showing, or nil.
func (e *Ext) View() *raycast.View { return e.view }

// Show takes the process's new top view.
func (e *Ext) Show(v *raycast.View) {
	wasList := e.view == nil || e.view.IsList()
	e.view = v
	// A new kind of view (a push, a pop) starts at the top.
	if !v.IsList() || !wasList {
		e.sel = 0
	}
	if v.IsList() {
		if v.Selected != nil {
			for i := range v.Count() {
				if v.Item(i).ID == *v.Selected {
					e.sel = i
				}
			}
		}
		e.sel = min(e.sel, max(v.Count()-1, 0))
	}
	if v.IsForm() {
		e.loadForm(v)
	} else {
		e.formKey = ""
	}
}

// Note says text in the footer for a while, from the next frame.
func (e *Ext) Note(text string, bad bool) {
	e.note, e.noteBad = text, bad
	e.noteAt = time.Time{}
}

func (e *Ext) say(kind, field string, value any) {
	if e.to != nil {
		e.to(raycast.Say(kind, field, value))
	}
}

func (e *Ext) run(a raycast.Action) {
	e.actionsOpen = false
	e.say("action", "id", a.ID)
}

func (e *Ext) actions() []raycast.Action {
	if e.view == nil {
		return nil
	}
	return e.view.ActionsAt(e.sel)
}

// Height is as tall as what is shown, no taller than the panel gets.
func (e *Ext) Height() float32 {
	v := e.view
	if v == nil {
		return core.HeaderH + core.FooterH + 80
	}
	chrome := float32(core.HeaderH + core.FooterH + 2)
	clamp := func(h float32) float32 { return min(h, core.MaxHeight) }
	if e.actionsOpen {
		return clamp(chrome + extPad*2 + extSectionH + float32(len(v.ActionsAt(e.sel)))*38)
	}
	if v.IsForm() {
		return clamp(chrome + 32 + float32(max(len(v.Fields), 1))*44)
	}
	if !v.IsList() {
		if v.Output != nil {
			lines := min(strings.Count(strings.TrimRight(*v.Output, "\n"), "\n")+1, 200)
			return clamp(max(chrome+36+float32(lines)*19, 150))
		}
		md := ""
		if v.Markdown != nil {
			md = *v.Markdown
		}
		// Long paragraphs wrap: about 80 characters a line.
		lines := strings.Count(md, "\n") + 1 + len(md)/80
		return clamp(max(chrome+46+float32(lines)*24+float32(len(v.Metadata))*22, 150))
	}
	h := chrome + extPad*2
	if v.Count() == 0 {
		return h + 80
	}
	for _, s := range v.Sections {
		if len(s.Items) == 0 {
			continue
		}
		if s.Title != "" {
			h += extSectionH
		}
		h += float32(len(s.Items)) * extRowH
	}
	return clamp(h)
}

func (e *Ext) screen(c *ui.Context, p *Panel, shell ui.Element) {
	e.panel = p
	pal := p.pal
	h := p.follow(c, shell, e.Height())
	for _, k := range extKeys {
		if c.Shortcut(k.mods, k.key) {
			e.key(k.key, k.mods, nil)
		}
	}
	if !e.Open {
		c.Invalidate()
		return
	}
	ui.Column(c.Key(fmt.Sprint("ext-", e.starts))).FillWidth().Height(h).Clip().Children(func() {
		e.header(c, pal)
		Hairline(c, pal)
		ui.Column(c).FillWidth().Grow(1).Shrink(1).Clip().Children(func() {
			switch v := e.view; {
			case e.actionsOpen:
				e.actionList(c, pal)
			case v == nil:
				ui.Row(c).FillWidth().Height(80).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Starting…").FontSize(13).TextColor(pal.Sub.Alpha(0.8))
				})
			case v.IsList():
				e.list(c, pal, v)
			case v.IsForm():
				e.form(c, pal, v)
			default:
				e.detail(c, pal, v)
			}
		})
		e.footer(c, p, pal)
	})
}

var extKeys = []keyMods{
	{0, ui.KeyDown}, {0, ui.KeyUp}, {ui.Ctrl, ui.KeyN}, {ui.Ctrl, ui.KeyP}, {ui.Ctrl, ui.KeyK},
	{ui.Ctrl, ui.KeyEnter}, {0, ui.KeyEnter}, {0, ui.KeyEscape},
}

// textKey sees a field's keys first: the screen's own are taken.
func (e *Ext) textKey(area bool) func(k ui.TextKey) bool {
	return func(k ui.TextKey) bool {
		if area && k.Key == ui.KeyEnter && k.Mods == 0 {
			return false // a text area breaks its line
		}
		if area && (k.Key == ui.KeyUp || k.Key == ui.KeyDown) && k.Mods == 0 {
			return false
		}
		for _, s := range extKeys {
			if s.key == k.Key && s.mods == k.Mods {
				return e.key(k.Key, k.Mods, &k)
			}
		}
		return false
	}
}

// key does what a key does on the screen; it reports whether it took it.
func (e *Ext) key(key ui.Key, mods ui.Modifiers, k *ui.TextKey) bool {
	v := e.view
	acts := e.actions()
	n := 0
	switch {
	case e.actionsOpen:
		n = len(acts)
	case v != nil && v.IsList():
		n = v.Count()
	}
	sel := &e.sel
	if e.actionsOpen {
		sel = &e.actSel
	}
	before := e.sel
	switch {
	case mods == 0 && key == ui.KeyDown, mods == ui.Ctrl && key == ui.KeyN:
		if n == 0 {
			return v != nil && v.IsList()
		}
		*sel = (*sel + 1) % n
	case mods == 0 && key == ui.KeyUp, mods == ui.Ctrl && key == ui.KeyP:
		if n == 0 {
			return v != nil && v.IsList()
		}
		*sel = (*sel + n - 1) % n
	case mods == ui.Ctrl && key == ui.KeyK:
		e.actionsOpen = !e.actionsOpen && len(acts) > 0
		e.actSel = 0
	case mods == ui.Ctrl && key == ui.KeyEnter:
		// A form submits with its first action, as Raycast's does.
		i := 1
		if v != nil && v.IsForm() {
			i = 0
		}
		if len(acts) > i {
			e.run(acts[i])
		}
	case mods == 0 && key == ui.KeyEnter:
		if e.actionsOpen {
			if e.actSel < len(acts) {
				e.run(acts[e.actSel])
			}
		} else if len(acts) > 0 {
			e.run(acts[0])
		}
	case mods == 0 && key == ui.KeyEscape:
		switch {
		case e.actionsOpen:
			e.actionsOpen = false
		case e.query != "" && v != nil && v.IsList():
			e.query = ""
			if k != nil {
				k.SetText("")
			}
		default:
			e.say("pop", "", nil)
		}
	default:
		return false
	}
	if !e.actionsOpen && e.sel != before && v != nil {
		if it := v.Item(e.sel); it != nil {
			e.say("select", "id", it.ID)
		}
	}
	return true
}

func (e *Ext) header(c *ui.Context, pal Palette) {
	ui.Row(c).FillWidth().Height(core.HeaderH).PaddingX(18).Gap(12).AlignItems(ui.Center).Shrink(0).Children(func() {
		back := ui.Box(c.Key("ext-back")).Size(26, 26).Center().Radius(7).Background(pal.Tile).Label("Back").
			OnClick(func() { e.say("pop", "", nil) })
		back.Children(func() { Symbol(c, "chevron.left", 13, Semibold).TextColor(pal.Sub) })
		v := e.view
		if v == nil || v.IsList() {
			ph := "Search " + e.name + "…"
			if v != nil && v.Placeholder != "" {
				ph = v.Placeholder
			}
			ui.Box(c).Grow(1).Children(func() {
				in := ui.TextInputBase(c.Key("ext-query"), &e.query).Bind(&e.input).FontSize(19).PaddingX(2).FillWidth().
					Placeholder(ph).PlaceholderColor(pal.Sub.Alpha(0.75)).TextColor(pal.Text).CaretWidth(1.5).
					Label("Search").OnTextKey(e.textKey(false))
				_ = in
			})
			if e.query != e.sent {
				e.sent = e.query
				e.sel = 0
				e.say("search", "text", e.query)
			}
		} else {
			t := v.Title
			if t == "" {
				t = e.name
			}
			ui.Text(c, t).FontSize(17).FontWeight(W600).TextColor(pal.Text).SingleLine().Grow(1).Shrink(1)
		}
		if v != nil && v.Loading {
			ui.Text(c, "Loading…").FontSize(12).TextColor(pal.Sub.Alpha(0.7)).NoWrap()
		}
	})
}

func (e *Ext) list(c *ui.Context, pal Palette, v *raycast.View) {
	if v.Count() == 0 {
		ui.Column(c).FillWidth().Height(80).Justify(ui.Center).AlignItems(ui.Center).Gap(4).Children(func() {
			title := "No Results"
			if v.Loading {
				title = "Loading…"
			}
			if v.Empty != nil && v.Empty.Title != nil {
				title = *v.Empty.Title
			}
			ui.Text(c, title).FontSize(14).FontWeight(W500).TextColor(pal.Text).NoWrap()
			if v.Empty != nil && v.Empty.Description != nil {
				ui.Text(c, *v.Empty.Description).FontSize(12).TextColor(pal.Sub)
			}
		})
		return
	}
	ui.Scroll(c.Key("ext-rows")).FillWidth().Padding(extPad, 8).Grow(1).Shrink(1).Children(func() {
		i := 0
		for si, s := range v.Sections {
			if len(s.Items) == 0 {
				continue
			}
			if s.Title != "" {
				ui.Row(c.Key(fmt.Sprint("rc-sec-", si))).FillWidth().Height(extSectionH).PaddingX(12).AlignItems(ui.End).
					Padding(0, 12, 4, 12).Shrink(0).Children(func() {
					SectionLabel(c, pal, s.Title)
				})
			}
			for j := range s.Items {
				e.row(c, pal, &s.Items[j], i)
				i++
			}
		}
	})
}

func (e *Ext) row(c *ui.Context, pal Palette, it *raycast.Item, i int) {
	r := ui.Row(c.Key(fmt.Sprint("rc-row-", i))).FillWidth().Height(extRowH).PaddingX(12).Gap(12).AlignItems(ui.Center).
		Radius(10).Shrink(0).Label(it.Title).OnClick(func() {
		if e.sel == i {
			if len(it.Actions) > 0 {
				e.run(it.Actions[0])
			}
			return
		}
		e.sel = i
		e.say("select", "id", it.ID)
	})
	if i == e.sel {
		r.Background(pal.Sel).Border(1, pal.Rim)
		if e.sel != 0 {
			r.ScrollIntoView()
		}
	}
	r.Children(func() {
		e.iconTile(c, pal, it.Icon)
		ui.Text(c, it.Title).FontSize(14).FontWeight(W500).TextColor(pal.Text).SingleLine().Shrink(1)
		if it.Subtitle != "" {
			ui.Text(c, it.Subtitle).FontSize(12.5).TextColor(pal.Sub.Alpha(0.85)).SingleLine().Shrink(2)
		}
		ui.Spacer(c)
		for _, a := range it.Accessories {
			if a != "" {
				ui.Text(c, a).FontSize(12).TextColor(pal.Sub.Alpha(0.7)).FontFeatures("tnum").NoWrap()
			}
		}
	})
}

func (e *Ext) iconTile(c *ui.Context, pal Palette, ic *raycast.Icon) {
	box := ui.Box(c).Size(24, 24).Center().Radius(6).Shrink(0)
	switch {
	case ic == nil:
		box.Children(func() {})
	case ic.Text != nil:
		// An emoji, or a word or number (a rank) on a tile.
		t := *ic.Text
		r, _ := utf8.DecodeRuneInString(t)
		emoji := r >= 0x2300
		size := float32(12)
		if emoji {
			size = 15
		} else {
			box.Background(pal.Tile).Border(1, pal.Rim)
		}
		box.Children(func() {
			ui.Text(c, t).FontSize(size).FontWeight(W600).TextColor(pal.Text).FontFeatures("tnum").NoWrap()
		})
	case ic.File != nil && e.file(*ic.File) != nil:
		b := e.file(*ic.File)
		box.Children(func() { ui.Image(c, b).Size(24, 24).Radius(5) })
	case ic.URL != nil && e.icon != nil:
		// A picture from the web (nothing is fetched): the extension's own
		// icon stands in.
		box.Children(func() { ui.Image(c, e.icon).Size(20, 20).Radius(5) })
	default:
		box.Background(pal.Tile).Children(func() { Symbol(c, "circle", 9, Regular).TextColor(pal.Sub.Alpha(0.6)) })
	}
}

// file is an item's icon from a file, read once.
func (e *Ext) file(path string) *ui.Bitmap {
	if e.files == nil {
		e.files = map[string]*ui.Bitmap{}
	}
	if b, ok := e.files[path]; ok {
		return b
	}
	var b *ui.Bitmap
	if data, err := os.ReadFile(path); err == nil && len(data) < 4<<20 {
		b, _ = ui.DecodeBitmap(data)
	}
	e.files[path] = b
	return b
}

func (e *Ext) detail(c *ui.Context, pal Palette, v *raycast.View) {
	ui.Scroll(c.Key("ext-detail")).FillWidth().Padding(18, 26).Gap(10).Grow(1).Shrink(1).Children(func() {
		// A script's output: its lines as printed, monospaced, the last 200.
		if v.Output != nil {
			lines := strings.Split(strings.TrimRight(*v.Output, "\n"), "\n")
			if len(lines) > 200 {
				lines = lines[len(lines)-200:]
			}
			ui.Column(c).FillWidth().Gap(4).Children(func() {
				for _, l := range lines {
					if l == "" {
						l = " "
					}
					ui.Text(c, l).Font(Mono).FontSize(12.5).TextColor(pal.Text)
				}
			})
			return
		}
		md := ""
		if v.Markdown != nil {
			md = *v.Markdown
		}
		first := true
		for _, l := range strings.Split(raycast.Plain(md), "\n") {
			if l == "" {
				continue
			}
			t := ui.Text(c, l).TextColor(pal.Text)
			if first {
				t.FontSize(19).FontWeight(W600)
			} else {
				t.FontSize(14)
			}
			first = false
		}
		if len(v.Metadata) > 0 {
			Hairline(c, pal)
			for _, m := range v.Metadata {
				ui.Row(c).FillWidth().Gap(10).Children(func() {
					ui.Text(c, m.Label).FontSize(12).FontWeight(W500).TextColor(pal.Sub).NoWrap()
					ui.Spacer(c)
					ui.Text(c, m.Text).FontSize(12).TextColor(pal.Text)
				})
			}
		}
	})
}

func isTyped(kind string) bool { return kind == "text" || kind == "textarea" || kind == "password" }

// loadForm takes a new form's values; the first typed field gets the keys.
func (e *Ext) loadForm(v *raycast.View) {
	var key strings.Builder
	for _, f := range v.Fields {
		key.WriteString(f.ID + "\x00")
	}
	if key.String() == e.formKey {
		// The same form again: a dropdown's pick shows, typed text stays.
		for i, f := range v.Fields[:min(len(v.Fields), maxFields)] {
			if f.Type == "dropdown" || f.Type == "checkbox" {
				e.setField(i, f.Value)
			}
		}
		return
	}
	e.formKey = key.String()
	e.focus = -1
	for i, f := range v.Fields[:min(len(v.Fields), maxFields)] {
		e.setField(i, f.Value)
		if e.focus < 0 && isTyped(f.Type) {
			e.focus = i
		}
	}
}

func (e *Ext) setField(i int, v any) {
	switch x := v.(type) {
	case string:
		e.fields[i] = x
	case bool:
		e.checks[i] = x
	default:
		e.fields[i] = ""
	}
}

func (e *Ext) form(c *ui.Context, pal Palette, v *raycast.View) {
	area := v.HasArea()
	ui.Scroll(c.Key("ext-form")).FillWidth().Padding(16, 26).Gap(12).Grow(1).Shrink(1).Children(func() {
		for i, f := range v.Fields[:min(len(v.Fields), maxFields)] {
			switch f.Type {
			case "separator":
				Hairline(c, pal)
				continue
			case "description":
				ui.Text(c, f.Title).FontSize(12.5).TextColor(pal.Sub)
				continue
			}
			ui.Row(c.Key(fmt.Sprint("rc-field-", i))).FillWidth().Gap(14).AlignItems(ui.Center).Children(func() {
				label := f.Title
				if label == "" {
					label = f.ID
				}
				ui.Box(c).Width(130).Shrink(0).Children(func() {
					ui.Text(c, label).FontSize(13).FontWeight(W500).TextColor(pal.Sub).SingleLine()
				})
				switch f.Type {
				case "checkbox":
					r := ui.Row(c.Key("check")).Label(label)
					r.OnClick(func() {
						e.checks[i] = !e.checks[i]
						e.fieldValue(f.ID, e.checks[i])
					})
					r.Children(func() { animatedSwitch(c, r, pal, e.checks[i]) })
				case "dropdown":
					ui.Row(c).Gap(6).Children(func() {
						for _, o := range f.Options[:min(len(f.Options), 4)] {
							bg := pal.Tile
							if s, ok := f.Value.(string); ok && s == o.Value {
								bg = pal.Sel
							}
							ui.Row(c.Key("opt-" + o.Value)).Height(26).PaddingX(10).Justify(ui.Center).AlignItems(ui.Center).
								Radius(7).Background(bg).Label(o.Title).OnClick(func() { e.fieldValue(f.ID, o.Value) }).Children(func() {
								ui.Text(c, o.Title).FontSize(12).TextColor(pal.Text).NoWrap()
							})
						}
					})
				default:
					box := ui.Row(c).Grow(1).PaddingX(10).AlignItems(ui.Center).Radius(8).Background(pal.Tile).Border(1, pal.Rim)
					if f.Type == "textarea" {
						box.Height(96).Padding(6, 10)
					} else {
						box.Height(32)
					}
					box.Children(func() {
						var in ui.Element
						if f.Type == "textarea" {
							in = ui.TextAreaBase(c.Key("in"), &e.fields[i]).FillHeight()
						} else {
							in = ui.TextInputBase(c.Key("in"), &e.fields[i])
							if f.Type == "password" {
								in.Password()
							}
						}
						in.Bind(&e.inputs[i]).FontSize(14).FillWidth().Placeholder(f.Placeholder).
							PlaceholderColor(pal.Sub.Alpha(0.6)).TextColor(pal.Text).Label(label).
							OnTextKey(e.textKey(area && f.Type == "textarea"))
						if e.focus == i {
							e.inputs[i].Focus()
							e.focus = -1
						}
						if in.Changed() {
							e.fieldValue(f.ID, e.fields[i])
						}
					})
				}
			})
		}
	})
}

func (e *Ext) fieldValue(id string, v any) {
	if e.to != nil {
		e.to(raycast.FieldValue(id, v))
	}
}

func (e *Ext) actionList(c *ui.Context, pal Palette) {
	acts := e.actions()
	ui.Scroll(c.Key("ext-actions")).FillWidth().Padding(extPad, 8).Grow(1).Shrink(1).Children(func() {
		ui.Row(c).FillWidth().Height(extSectionH).Padding(0, 12, 4, 12).AlignItems(ui.End).Shrink(0).Children(func() {
			SectionLabel(c, pal, "Actions")
		})
		for i, a := range acts {
			r := ui.Row(c.Key(fmt.Sprint("rc-act-", i))).FillWidth().Height(38).PaddingX(12).Gap(10).AlignItems(ui.Center).
				Radius(9).Shrink(0).Label(a.Title).OnClick(func() { e.run(a) })
			if i == e.actSel {
				r.Background(pal.Sel).Border(1, pal.Rim)
			}
			r.Children(func() {
				ui.Text(c, a.Title).FontSize(13.5).FontWeight(W500).TextColor(pal.Text).NoWrap()
				ui.Spacer(c)
				switch i {
				case 0:
					Kbd(c, pal, "↩")
				case 1:
					Kbd(c, pal, core.Mod+"↩")
				}
			})
		}
	})
}

func (e *Ext) footer(c *ui.Context, p *Panel, pal Palette) {
	Hairline(c, pal)
	acts := e.actions()
	ui.Row(c).FillWidth().Height(core.FooterH).PaddingX(16).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		if e.icon != nil {
			ui.Image(c, e.icon).Size(16, 16).Radius(4)
		}
		now := p.clock()
		if e.note != "" && e.noteAt.IsZero() {
			e.noteAt = now
		}
		if e.note != "" && now.Sub(e.noteAt) < noteFor {
			col := pal.Text
			if e.noteBad {
				col = pal.Danger
			}
			ui.Text(c, e.note).FontSize(12).FontWeight(W500).TextColor(col).SingleLine().Shrink(1)
			c.After(noteFor - now.Sub(e.noteAt))
		} else {
			ui.Text(c, e.name).FontSize(12).TextColor(pal.Sub.Alpha(0.8)).SingleLine().Shrink(1)
		}
		ui.Spacer(c)
		if len(acts) == 0 {
			return
		}
		ui.Text(c, acts[0].Title).FontSize(12).FontWeight(W500).TextColor(pal.Text.Alpha(0.85)).NoWrap()
		cap := "↩"
		if e.view != nil && e.view.IsForm() && e.view.HasArea() {
			cap = core.Mod + "↩"
		}
		Kbd(c, pal, cap)
		if len(acts) > 1 {
			VRule(c, pal)
			ui.Text(c, "Actions").FontSize(12).TextColor(pal.Sub.Alpha(0.8)).NoWrap()
			Kbd(c, pal, core.Mod+"K")
		}
	})
}
