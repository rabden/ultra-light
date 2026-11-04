// Package view draws Nanolight's panel in mygo's native UI: the search
// screen, the results page, Settings, the welcome and Raycast's commands.
// It keeps the panel's state and asks a Host for everything outside the
// window, so tests run it in a ui.Tester with a fake host. Port of
// view.zig, calc_card.zig, settings_view.zig, onboard.zig and
// raycast_view.zig.
package view

import (
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/rabden/ultra-light/internal/apps"
	"github.com/rabden/ultra-light/internal/core"
	"github.com/rabden/ultra-light/resources"
)

// Host is what the panel asks of the app around it.
type Host interface {
	// Open opens an entry of the index: launches an app, starts a Raycast
	// command or a script. It returns true when the panel should stay
	// (a command shown in it).
	Open(e apps.Entry) (keepOpen bool)
	OpenURL(url string)
	Hide()
	// Resize follows the panel's height.
	Resize(h float32)
	// Suggest asks Google for query's completions, answered with
	// Panel.Answered. Without it (nil) none are listed.
	Suggest(query string)
	// Copy puts text on the clipboard.
	Copy(text string)
	// SaveLaunches keeps the launch counts (store + sync).
	SaveLaunches(encoded string)
	// Icon returns an entry's picture, or nil.
	Icon(e apps.Entry) *ui.Bitmap

	// ShowPage shows url in the panel's web view and returns false when
	// there is none (no WebKitGTK): the search goes to the browser.
	ShowPage(url string) bool
	// HidePage stops the page and empties it.
	HidePage()
	// Page is the web view the panel shows the page in, or nil.
	Page() ui.NativeWebView
	// PageURL is the address the page is at now.
	PageURL() string
	// PageBack goes back a page.
	PageBack()
}

// Panel is the whole panel's state.
type Panel struct {
	Host  Host
	Index *apps.Index

	query string
	input ui.Handle
	res   core.Results
	sel   int
	// built is the query the rows were built for; gen counts the lists, so
	// a new one snaps the highlight.
	built      string
	builtValid bool
	gen        int
	// revealed is the query as the panel showed: no ghost after it yet.
	revealed string
	// Pending edits of the field, applied as the view builds or from a key.
	pendingQuery *string
	caretEnd     bool
	selectAll    bool
	caretAtEnd   bool

	// Google's completions: whether to ask, those listed, the answers.
	SuggestOn bool
	web       []string
	cache     core.SuggestCache

	// SearchesOut: Return opens Google in the browser, Ctrl+↩ in the panel.
	SearchesOut bool
	// A search's page shows in the panel, for the query as it was then.
	previewing  bool
	previewQ    string
	previewURL  string
	PageLoaded  bool
	pageHeight  float32
	copiedAt    time.Time
	shown       int
	lastH       float32
	Note        string
	appIcon     *ui.Bitmap
	clock       func() time.Time
	pal         Palette
	pageEnabled bool

	Settings Settings
	Welcome  Welcome
	Ext      Ext
}

// NewPanel makes the panel for an index.
func NewPanel(h Host, ix *apps.Index) *Panel {
	p := &Panel{Host: h, Index: ix, SuggestOn: true, clock: time.Now, pageEnabled: true}
	p.appIcon, _ = ui.DecodeBitmap(resources.Icon256)
	p.Settings.init()
	p.Ext.panel = p
	return p
}

// SetClock replaces time.Now, for tests.
func (p *Panel) SetClock(f func() time.Time) { p.clock = f }

// Query is the text in the search field.
func (p *Panel) Query() string { return p.query }

// SetQuery puts text in the search field, the caret at its end.
func (p *Panel) SetQuery(q string) {
	p.query = q
	p.caretEnd = true
}

// Rows returns the rows listed, and which is selected.
func (p *Panel) Rows() ([]core.Row, int) { return p.res.Rows, p.sel }

// Previewing reports whether a search's page shows.
func (p *Panel) Previewing() bool { return p.previewing }

// Shown counts the showings.
func (p *Panel) Shown() int { return p.shown }

// Reveal is called as the panel shows: the query selected, the keys its own,
// the panel coming in again.
func (p *Panel) Reveal() {
	p.Back()
	p.Settings.Hide()
	p.shown++
	p.builtValid = false // suggestions follow what was opened since
	p.revealed = p.query
	p.selectAll = true
	p.input.Focus()
}

// Height is the panel's height for what it shows now.
func (p *Panel) Height() float32 {
	switch {
	case p.Welcome.Active:
		return p.Welcome.Height()
	case p.Settings.Open:
		return p.Settings.Height()
	case p.Ext.Open:
		return p.Ext.Height()
	case p.previewing:
		return core.MaxHeight
	}
	return core.SearchHeight(&p.res)
}

// View builds the window's content: the panel, and nothing round it.
func (p *Panel) View(c *ui.Context) {
	p.pal = PaletteFor(c.Theme().Dark)
	c.SetTheme(theme(c.Theme(), p.pal))
	c.Root().Background(ui.Transparent)
	pal := p.pal
	shell := ui.Column(c.Key(p.shown)).Width(core.PanelWidth).Radius(core.PanelRadius).
		Border(1, pal.Hair).Clip().Background(pal.Fill).TextColor(pal.Text).Font(Font)
	// The entrance: opacity 0→1, y −6→0, scale 0.985→1, on spring 320/32,
	// each showing anew.
	entered := ui.Local(shell, "entered", func() int { return 0 })
	pose := ui.Pose{}
	if *entered == 0 {
		pose = ui.Pose{Y: -6, Scale: 0.985, Fade: 1}
		*entered = 1
		c.Invalidate()
	}
	shell.AnimatePose(pose, panelSpring)
	shell.Children(func() {
		switch {
		case p.Welcome.Active:
			h := p.follow(c, shell, p.Welcome.Height())
			ui.Column(c).FillWidth().Height(h).Clip().Children(func() { p.Welcome.view(c, p) })
		case p.Settings.Open:
			p.Settings.view(c, p, shell)
		case p.Ext.Open:
			p.Ext.screen(c, p, shell)
		default:
			p.search(c, shell)
		}
	})
}

var (
	panelSpring = ui.Animation{Spring: ui.SpringOptions{Stiffness: 320, Damping: 32, Mass: 1}}
	slideSpring = ui.SpringOptions{Stiffness: 700, Damping: 52, Mass: 1}
)

// follow springs the panel's height to target (clamped to the most it
// gets); the window follows it once it moved more than 0.25.
func (p *Panel) follow(c *ui.Context, e ui.Element, target float32) float32 {
	h := e.Spring("panel-h", min(target, core.MaxHeight), panelSpring.Spring)
	if math.Abs(float64(h-p.lastH)) > 0.25 {
		p.lastH = h
		if p.Host != nil {
			p.Host.Resize(h)
		}
	}
	return h
}

// search builds the search screen: the header, the rows or the page, the
// footer.
func (p *Panel) search(c *ui.Context, shell ui.Element) {
	pal := p.pal
	p.shortcuts(c)
	p.header(c)
	p.rebuild()
	// Typing takes the page away: the rows are for the new words.
	if p.previewing && p.query != p.previewQ {
		p.Back()
	}
	h := p.follow(c, shell, p.Height())
	ui.Column(c).FillWidth().Height(max(0, h-core.HeaderH)).Clip().Children(func() {
		if p.previewing {
			Hairline(c, pal)
			area := core.WebAreaHeight(h)
			ui.Box(c).FillWidth().Height(area).Children(func() {
				if page := p.Host.Page(); page != nil {
					ui.WebView(c, page).FillWidth().FillHeight()
				}
				if !p.PageLoaded {
					// Over the page, until it draws.
					ui.Row(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Justify(ui.Center).AlignItems(ui.Center).
						Background(pal.Fill).PassThrough().Children(func() {
						ui.Text(c, "Searching Google…").FontSize(13).TextColor(pal.Sub.Alpha(0.8))
					})
				}
			})
		} else if len(p.res.Rows) > 0 {
			Hairline(c, pal)
			p.list(c)
		}
		ui.Spacer(c)
		p.footer(c)
	})
}

// header is the magnifying glass and the query field, with its ghost.
func (p *Panel) header(c *ui.Context) {
	pal := p.pal
	ui.Row(c).FillWidth().Height(core.HeaderH).PaddingX(18).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(22, 22).Center().Children(func() {
			Symbol(c, "magnifyingglass", 19, Medium).TextColor(pal.Sub)
		})
		p.field(c)
	})
}

const fieldSize = 21

func (p *Panel) field(c *ui.Context) {
	pal := p.pal
	if p.pendingQuery != nil {
		p.query = *p.pendingQuery
		p.pendingQuery = nil
	}
	ui.Box(c).Grow(1).Children(func() {
		in := ui.TextInputBase(c.Key("query"), &p.query).Bind(&p.input).FontSize(fieldSize).PaddingX(2).
			FillWidth().Placeholder("Search apps, do math, search Google…").PlaceholderColor(pal.Sub.Alpha(0.75)).
			TextColor(pal.Text).CaretWidth(1.5).Label("Search").OnTextKey(p.textKey)
		n := utf8.RuneCountInString(p.query)
		switch {
		case p.selectAll:
			in.SetTextSelection(0, n)
			p.selectAll = false
		case p.caretEnd:
			in.SetTextSelection(n, n)
			p.caretEnd = false
		}
		if in.Changed() {
			p.rebuild()
		}
		ghost := p.ghost()
		start, end := in.TextSelection()
		p.caretAtEnd = start == end && end == n
		if ghost == "" || !p.input.Focused(c) || start != end || end != n {
			return
		}
		w, _ := c.MeasureText(0, ui.Span{Text: p.query, Size: fieldSize, Font: Font})
		gw, _ := c.MeasureText(0, ui.Span{Text: ghost, Size: fieldSize, Font: Font})
		if box := p.input.Bounds(c); box.W > 0 && 2+w+gw > box.W-2 {
			return // the text scrolls: no room to say more
		}
		ui.Text(c, ghost).Absolute().Left(2 + w).Top(0).Bottom(0).FontSize(fieldSize).
			TextColor(pal.Muted.Alpha(0.8)).NoWrap().PassThrough()
	})
}

// ghost is what the query is the start of, drawn faintly after the caret.
func (p *Panel) ghost() string {
	return core.Ghost(p.query, p.query == p.revealed, p.res.Rows,
		func(i int) string { return p.Index.Entries[i].Name }, p.web)
}

// rebuild lists the rows for the query, if it changed since they were
// built: with what is known of Google's completions, asked for if not all.
func (p *Panel) rebuild() {
	if p.builtValid && p.query == p.built {
		return
	}
	p.built, p.builtValid = p.query, true
	p.gen++
	p.web = nil
	q := p.query
	if _, math := core.Eval(q); p.SuggestOn && core.Eligible(q) && !math {
		var exact bool
		p.web, exact = p.cache.Lookup(q)
		if !exact && p.Host != nil {
			p.Host.Suggest(q)
		}
	}
	p.fill()
	// The first row: an app, the answer, or the search for what was typed.
	p.sel = 0
}

func (p *Panel) fill() {
	sug := p.Index.Suggestions(core.MaxApps)
	p.res.Build(p.query, p.Index.Names, p.Index.Launches, sug, p.web)
}

// Refresh rebuilds the rows, as after the index changed.
func (p *Panel) Refresh() { p.builtValid = false }

// Answered takes Google's completions of typed (what Host.Suggest asked):
// kept, and listed if it is still what is typed. The selection stays on the
// row it was on, wherever that row now is.
func (p *Panel) Answered(typed string, said []string) {
	p.cache.Put(typed, said)
	if !core.Same(typed, p.query) {
		return
	}
	var was *core.Row
	wasText := ""
	if p.sel < len(p.res.Rows) {
		r := p.res.Rows[p.sel]
		was = &r
		if r.Kind == core.KindSuggest && r.Web < len(p.web) {
			wasText = p.web[r.Web]
		}
	}
	p.web = append([]string(nil), said...)
	p.fill()
	p.sel = 0
	if was == nil {
		return
	}
	for i, now := range p.res.Rows {
		if now.Kind != was.Kind {
			continue
		}
		same := true
		switch was.Kind {
		case core.KindApp:
			same = now.App == was.App
		case core.KindSuggest:
			same = core.Same(p.web[now.Web], wasText)
		}
		if same {
			p.sel = i
			return
		}
	}
}

// list builds the rows under their section heads, over the highlight.
func (p *Panel) list(c *ui.Context) {
	pal := p.pal
	ui.Column(c).FillWidth().Padding(core.ListPad).Children(func() {
		y, rh := core.RowTop(&p.res, p.sel)
		// A new list: the highlight is there at once; within it, it slides.
		hl := ui.Box(c.Key("highlight")).Absolute().Left(core.ListPad).Width(core.PanelWidth - 2*core.ListPad - 2)
		hy := hl.Spring([2]any{"y", p.gen}, y, slideSpring)
		hh := hl.Spring([2]any{"h", p.gen}, rh, slideSpring)
		hl.Top(core.ListPad+hy).Height(hh).Radius(10).Background(pal.Sel).Border(1, pal.Rim)
		for i, r := range p.res.Rows {
			if name, ok := p.res.Section(i); ok {
				ui.Row(c).FillWidth().Height(core.SectionH).Padding(0, 12, 6, 12).AlignItems(ui.End).Children(func() {
					SectionLabel(c, pal, name)
				})
			}
			p.row(c, i, r)
		}
	})
}

func (p *Panel) row(c *ui.Context, i int, r core.Row) {
	pal := p.pal
	big := r.Kind == core.KindCalc
	h := float32(core.RowH)
	if big {
		h = core.CalcH
	}
	ui.Row(c.Key(i)).FillWidth().Height(h).PaddingX(10).Gap(12).AlignItems(ui.Center).OnClick(func() {
		p.sel = i
		p.activateWith(func(s string) { p.set(s) })
	}).Children(func() {
		if big {
			p.calcCard(c)
			return
		}
		p.tile(c, r)
		q := strings.Trim(p.query, " ")
		switch r.Kind {
		case core.KindApp:
			e := p.Index.Entries[r.App]
			ui.RichText(c, p.highlight(e.Name, r.Match.Hits)...).FontSize(13.5).FontWeight(W500).
				TextColor(pal.Text.Alpha(0.82)).SingleLine().Shrink(1)
			ui.Text(c, e.Folder).FontSize(12).TextColor(pal.Sub.Alpha(0.8)).SingleLine().Shrink(1000)
		case core.KindSettings:
			ui.Text(c, "Nanolight Settings").FontSize(13.5).FontWeight(W500).TextColor(pal.Text).NoWrap()
			ui.Text(c, "Shortcut, sync, updates").FontSize(12).TextColor(pal.Sub.Alpha(0.8)).NoWrap()
		case core.KindSuggest:
			w := p.web[r.Web]
			ui.RichText(c, p.added(w)...).FontSize(13.5).TextColor(pal.Text.Alpha(0.72)).SingleLine().Shrink(1)
		default:
			verb, sub := "Search Google for", "google.com"
			switch r.Kind {
			}
			ui.Text(c, verb+" “"+q+"”").FontSize(13.5).FontWeight(W500).TextColor(pal.Text).SingleLine().Shrink(1)
			ui.Text(c, sub).FontSize(12).TextColor(pal.Sub.Alpha(0.8)).NoWrap()
		}
		ui.Spacer(c)
		if i == p.sel {
			label := "Go"
			if r.Kind == core.KindApp || r.Kind == core.KindSettings {
				label = "Open"
			}
			ui.Text(c, label).FontSize(12).FontWeight(W500).TextColor(pal.Sub).NoWrap()
			Kbd(c, pal, "↩")
		} else if i < 9 {
			ui.Text(c, core.Mod+string(rune('1'+i))).FontSize(12).FontWeight(W500).TextColor(pal.Sub.Alpha(0.5)).
				FontFeatures("tnum").NoWrap()
		}
	})
}

// tile is the row's picture: the app's icon, or a symbol on a quiet tile.
func (p *Panel) tile(c *ui.Context, r core.Row) {
	pal := p.pal
	if r.Kind == core.KindApp && p.Host != nil {
		if b := p.Host.Icon(p.Index.Entries[r.App]); b != nil {
			ui.Box(c).Size(26, 26).Center().Children(func() { ui.Image(c, b).Size(26, 26) })
			return
		}
	}
	big := r.Kind == core.KindCalc
	side, radius, size := float32(26), float32(7), float32(13)
	if big {
		side, radius, size = 34, 9, 16
	}
	ui.Box(c).Size(side, side).Radius(radius).Background(pal.Tile).Center().Children(func() {
		name := "app.dashed"
		switch r.Kind {
		case core.KindCalc:
			name = "plus.forwardslash.minus"
		case core.KindSearchWeb:
			name = "globe"
		case core.KindSuggest:
			name = "magnifyingglass"
		case core.KindSettings:
			name = "gearshape"
		}
		Symbol(c, name, size, Medium).TextColor(pal.Text)
	})
}

// highlight cuts name into spans, the matched bytes full strength and
// semibold.
func (p *Panel) highlight(name string, hits []int) []ui.Span {
	var spans []ui.Span
	at := 0
	for _, run := range core.HighlightRuns(hits) {
		s, e := run[0], min(run[1], len(name))
		if s >= e || s < at {
			continue
		}
		if s > at {
			spans = append(spans, ui.Span{Text: name[at:s]})
		}
		spans = append(spans, ui.Span{Text: name[s:e], Color: p.pal.Text, Weight: W600})
		at = e
	}
	if at < len(name) {
		spans = append(spans, ui.Span{Text: name[at:]})
	}
	return spans
}

// added shows a completion as a search box does: the words typed plain,
// what it adds stronger.
func (p *Panel) added(w string) []ui.Span {
	q := strings.Trim(p.query, " ")
	if q == "" || len(w) <= len(q) || !core.HasPrefixFold(w, q) {
		return []ui.Span{{Text: w}}
	}
	return []ui.Span{{Text: w[:len(q)]}, {Text: w[len(q):], Color: p.pal.Text, Weight: W600}}
}

// calcCard is the math row: the expression, an arrow, the answer, each over
// a chip; the answer's says Copied for 1.4 s once it is.
func (p *Panel) calcCard(c *ui.Context) {
	pal := p.pal
	half := float32(core.PanelWidth-core.ListPad*2-20-32-24) / 2
	ui.Row(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Gap(6).Children(func() {
		ui.Column(c).Width(half).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, strings.Trim(p.query, " =")).FontSize(20).FontWeight(W500).TextColor(pal.Text.Alpha(0.8)).
				FontFeatures("tnum").SingleLine()
			Chip(c, pal, "Expression", nil)
		})
		ui.Box(c).Size(32, 32).Radius(16).Background(pal.Tile).Center().Children(func() {
			Symbol(c, "arrow.right", 13, Semibold).TextColor(pal.Sub)
		})
		ui.Column(c).Width(half).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, p.res.Answer).FontSize(28).FontWeight(W600).TextColor(pal.Text).FontFeatures("tnum").
				LetterSpacing(-0.3).SingleLine()
			if since := p.clock().Sub(p.copiedAt); since < 1400*time.Millisecond {
				c.After(1400*time.Millisecond - since)
				Chip(c, pal, "Copied", &pal.Success)
			} else if p.res.Hex != "" {
				Chip(c, pal, "Result · "+p.res.Hex, nil)
			} else {
				Chip(c, pal, "Result", nil)
			}
		})
	})
}

// footer: the app's icon and name, the gear, and what Return and Esc do.
func (p *Panel) footer(c *ui.Context) {
	pal := p.pal
	Hairline(c, pal)
	ui.Row(c).FillWidth().Height(core.FooterH).PaddingX(16).Gap(8).AlignItems(ui.Center).Children(func() {
		p.footerName(c)
		ui.Spacer(c)
		p.gear(c)
		if p.previewing {
			ui.Text(c, "Open in Browser").FontSize(12).FontWeight(W500).TextColor(pal.Text.Alpha(0.85)).NoWrap()
			Kbd(c, pal, core.Mod+"↩")
			VRule(c, pal)
			ui.Text(c, "Back").FontSize(12).TextColor(pal.Sub.Alpha(0.8)).NoWrap()
			Kbd(c, pal, "esc")
			return
		}
		if len(p.res.Rows) == 0 {
			return
		}
		ui.Text(c, p.res.Rows[min(p.sel, len(p.res.Rows)-1)].Kind.Action()).FontSize(12).FontWeight(W500).
			TextColor(pal.Text.Alpha(0.85)).NoWrap()
		Kbd(c, pal, "↩")
		VRule(c, pal)
		ui.Text(c, "Close").FontSize(12).TextColor(pal.Sub.Alpha(0.8)).NoWrap()
		Kbd(c, pal, "esc")
	})
}

// footerName is the app's icon, then "Nanolight · N apps" or the note.
func (p *Panel) footerName(c *ui.Context) {
	pal := p.pal
	if p.appIcon != nil {
		ui.Image(c, p.appIcon).Size(16, 16)
	}
	name := "Nanolight"
	if p.Note != "" {
		name = p.Note
	}
	ui.Textf(c, "%s · %d apps", name, len(p.Index.Entries)).FontSize(12).FontWeight(W500).
		TextColor(pal.Sub.Alpha(0.8)).NoWrap()
}

func (p *Panel) gear(c *ui.Context) {
	pal := p.pal
	g := ui.Box(c.Key("settings-gear")).Size(26, 26).Radius(7).Center().Label("Settings").OnClick(p.OpenSettings)
	if g.Hovered() {
		g.Background(pal.Sel)
	}
	g.Children(func() { Symbol(c, "gearshape", 13, Medium).TextColor(pal.Sub) })
}

// shortcuts handles the search screen's keys when the field doesn't have
// the keyboard (a click elsewhere, or the page).
func (p *Panel) shortcuts(c *ui.Context) {
	for _, k := range searchKeys {
		if c.Shortcut(k.mods, k.key) {
			p.key(k.key, k.mods, nil)
		}
	}
}

type keyMods struct {
	mods ui.Modifiers
	key  ui.Key
}

var searchKeys = func() []keyMods {
	ks := []keyMods{
		{0, ui.KeyDown}, {0, ui.KeyUp}, {ui.Ctrl, ui.KeyN}, {ui.Ctrl, ui.KeyP}, {0, ui.KeyEnter},
		{ui.Ctrl, ui.KeyEnter}, {0, ui.KeyTab}, {0, ui.KeyEscape}, {ui.Ctrl, ui.KeyComma},
		{ui.Ctrl, ui.KeyBracketLeft},
	}
	for _, d := range digits {
		ks = append(ks, keyMods{ui.Ctrl, d})
	}
	return ks
}()

var digits = []ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9}

// textKey sees the field's keys before it does: the panel's own are taken.
func (p *Panel) textKey(k ui.TextKey) bool {
	if (k.Key == ui.KeyRight || k.Key == ui.KeyEnd) && k.Mods == 0 && p.caretAtEnd {
		// → or End at the end of the text takes the ghost, as the field's own.
		if g := p.ghost(); g != "" && k.Text() == p.query {
			s := p.query + g
			p.query = s
			k.SetText(s)
			p.caretEnd = true
			return true
		}
		return false
	}
	for _, s := range searchKeys {
		if s.key == k.Key && s.mods == k.Mods {
			text := k.Text()
			if text != p.query {
				p.query = text
				p.rebuild()
			}
			return p.key(k.Key, k.Mods, &k)
		}
	}
	return false
}

// key does what a key of the search screen does; it returns whether it
// took it. k, when the field saw the key, sets the field's text.
func (p *Panel) key(key ui.Key, mods ui.Modifiers, k *ui.TextKey) bool {
	set := func(s string) {
		p.query = s
		if k != nil {
			k.SetText(s)
		} else {
			p.pendingQuery = &s
		}
		p.caretEnd = true
	}
	if p.previewing {
		switch {
		case mods == ui.Ctrl && key == ui.KeyEnter:
			u := p.previewURL
			if cur := p.Host.PageURL(); strings.HasPrefix(cur, "http") {
				u = cur
			}
			p.go_(u)
			return true
		case mods == 0 && key == ui.KeyEscape:
			p.Back()
			return true
		case mods == ui.Ctrl && key == ui.KeyBracketLeft:
			p.Host.PageBack()
			return true
		}
		return false
	}
	n := len(p.res.Rows)
	switch {
	case mods == ui.Ctrl && key == ui.KeyComma:
		p.OpenSettings()
	case mods == ui.Ctrl && key == ui.KeyEnter:
		p.activateOut(set)
	case mods == 0 && key == ui.KeyTab:
		g := p.ghost()
		if g == "" {
			return false
		}
		set(p.query + g)
	case mods == 0 && key == ui.KeyDown, mods == ui.Ctrl && key == ui.KeyN:
		if n > 0 {
			p.sel = (p.sel + 1) % n
		}
	case mods == 0 && key == ui.KeyUp, mods == ui.Ctrl && key == ui.KeyP:
		if n > 0 {
			p.sel = (p.sel + n - 1) % n
		}
	case mods == 0 && key == ui.KeyEnter:
		p.activateWith(set)
	case mods == 0 && key == ui.KeyEscape:
		if p.query != "" {
			set("")
		} else if p.Host != nil {
			p.Host.Hide()
		}
	case mods == ui.Ctrl && key == ui.KeyBracketLeft:
		return false
	default:
		for d, dk := range digits {
			if mods == ui.Ctrl && key == dk && d < min(n, 9) {
				p.sel = d
				p.activateWith(set)
				return true
			}
		}
		return false
	}
	return true
}

// AcceptGhost takes the ghost into the query (→ or End at the end; Tab).
func (p *Panel) AcceptGhost() bool {
	g := p.ghost()
	if g == "" {
		return false
	}
	p.SetQuery(p.query + g)
	return true
}

// activateWith does Return on the selected row; set replaces the query.
func (p *Panel) activateWith(set func(string)) {
	if len(p.res.Rows) == 0 {
		return
	}
	r := p.res.Rows[min(p.sel, len(p.res.Rows)-1)]
	switch r.Kind {
	case core.KindCalc:
		v, ok := core.Eval(p.query)
		if !ok {
			return
		}
		p.Host.Copy(core.FormatNumber(v, false))
		p.copiedAt = p.clock()
	case core.KindApp:
		p.Host.SaveLaunches(p.Index.Opened(r.App))
		e := p.Index.Entries[r.App]
		if p.Host.Open(e) {
			p.set("")
			return
		}
		p.Host.Hide()
	case core.KindSearchWeb:
		p.Search(p.query, !p.SearchesOut)
	case core.KindSettings:
		p.OpenSettings()
	case core.KindSuggest:
		// Its words fill the field, as a search box's would.
		w := p.web[r.Web]
		set(w)
		p.Search(w, !p.SearchesOut)
	}
}

func (p *Panel) set(s string) {
	p.query = s
	p.pendingQuery = &s
	p.caretEnd = true
}

// activateOut is Ctrl+↩: a search goes where Return doesn't send it.
func (p *Panel) activateOut(set func(string)) {
	if len(p.res.Rows) == 0 {
		return
	}
	r := p.res.Rows[min(p.sel, len(p.res.Rows)-1)]
	switch r.Kind {
	case core.KindSearchWeb:
		p.Search(p.query, p.SearchesOut)
	case core.KindSuggest:
		p.Search(p.web[r.Web], p.SearchesOut)
	default:
		p.activateWith(set)
	}
}

// Search shows Google's page for words: in the panel when inPanel and the
// host can, else in the browser.
func (p *Panel) Search(words string, inPanel bool) {
	u := core.URL(core.KindSearchWeb, words)
	if !inPanel || !p.pageEnabled || !p.Host.ShowPage(u) {
		p.go_(u)
		return
	}
	p.previewURL = u
	p.previewing = true
	p.PageLoaded = false
	p.previewQ = p.query
}

// SetPageEnabled says whether searches can show in the panel (WebKitGTK).
func (p *Panel) SetPageEnabled(on bool) { p.pageEnabled = on }

// PageEnabled reports it.
func (p *Panel) PageEnabled() bool { return p.pageEnabled }

// OpenSettings shows Settings in the panel, the page gone.
func (p *Panel) OpenSettings() {
	p.Back()
	p.Settings.Show()
}

// Back takes the page away: the rows again.
func (p *Panel) Back() {
	if !p.previewing {
		return
	}
	p.previewing = false
	if p.Host != nil {
		p.Host.HidePage()
	}
	p.input.Focus()
}

func (p *Panel) go_(target string) {
	p.Host.OpenURL(target)
	p.Host.Hide()
}

// FocusQuery gives the field the keyboard.
func (p *Panel) FocusQuery() { p.input.Focus() }
