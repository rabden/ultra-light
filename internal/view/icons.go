package view

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

// The icons the Zig build draws on Linux: nanoui's own line icons
// (ui/icons.zig), keyed by the SF Symbol names the panel uses, on a 24-unit
// grid with round-ended strokes 1.6 + 0.35×weight units wide. They are
// rebuilt here as SVGs from the same points.

// Weights, as nanoui's: 0 regular … 3 bold.
const (
	Regular = iota
	Medium
	Semibold
	BoldW
)

type op struct {
	kind string // line loop fill cut circle disc box solid arc
	pts  [][2]float64
	v    []float64
}

func line(p ...[2]float64) op          { return op{kind: "line", pts: p} }
func loop(p ...[2]float64) op          { return op{kind: "loop", pts: p} }
func fill(p ...[2]float64) op          { return op{kind: "fill", pts: p} }
func cut(p ...[2]float64) op           { return op{kind: "cut", pts: p} }
func circle(x, y, r float64) op        { return op{kind: "circle", v: []float64{x, y, r}} }
func disc(x, y, r float64) op          { return op{kind: "disc", v: []float64{x, y, r}} }
func box(x, y, w, h, r float64) op     { return op{kind: "box", v: []float64{x, y, w, h, r}} }
func arc(x, y, r, from, to float64) op { return op{kind: "arc", v: []float64{x, y, r, from, to}} }
func pt(x, y float64) [2]float64       { return [2]float64{x, y} }

func star(cx, cy, outer, inner float64, n int) [][2]float64 {
	var pts [][2]float64
	for i := 0; i < 2*n; i++ {
		a := -math.Pi/2 + math.Pi*float64(i)/float64(n)
		r := inner
		if i%2 == 0 {
			r = outer
		}
		pts = append(pts, pt(cx+r*math.Cos(a), cy+r*math.Sin(a)))
	}
	return pts
}

func gear(cx, cy, outer, inner float64, n int) [][2]float64 {
	var pts [][2]float64
	s := math.Pi / float64(n)
	for i := 0; i < n; i++ {
		a := -math.Pi/2 + 2*s*float64(i)
		for j, d := range []float64{-0.55, -0.32, 0.32, 0.55} {
			r := []float64{inner, outer, outer, inner}[j]
			pts = append(pts, pt(cx+r*math.Cos(a+d*s), cy+r*math.Sin(a+d*s)))
		}
	}
	return pts
}

var iconOps = map[string][]op{
	"checkmark":                     {line(pt(5, 12.5), pt(10, 17.5), pt(19, 7))},
	"xmark":                         {line(pt(6, 6), pt(18, 18)), line(pt(18, 6), pt(6, 18))},
	"plus":                          {line(pt(12, 5), pt(12, 19)), line(pt(5, 12), pt(19, 12))},
	"chevron.right":                 {line(pt(9, 6), pt(15, 12), pt(9, 18))},
	"chevron.left":                  {line(pt(15, 6), pt(9, 12), pt(15, 18))},
	"arrow.right":                   {line(pt(5, 12), pt(19, 12)), line(pt(13, 6), pt(19, 12), pt(13, 18))},
	"magnifyingglass":               {circle(10.5, 10.5, 6.5), line(pt(15.5, 15.5), pt(20, 20))},
	"circle":                        {circle(12, 12, 9)},
	"checkmark.circle.fill":         {disc(12, 12, 10), cut(pt(7.5, 12.5), pt(10.5, 15.5), pt(16.5, 9))},
	"sparkles":                      {fill(star(10, 13.5, 8, 2, 4)...), fill(star(18.5, 5.5, 3.8, 1, 4)...)},
	"doc.text":                      {loop(pt(6, 3), pt(14, 3), pt(19, 8), pt(19, 21), pt(6, 21)), line(pt(9.5, 12), pt(15.5, 12)), line(pt(9.5, 16), pt(15.5, 16))},
	"doc.on.doc":                    {box(8.5, 8.5, 12, 12.5, 2), line(pt(4.5, 16), pt(4.5, 5.5), pt(5.5, 4.5), pt(15.5, 4.5))},
	"folder":                        {loop(pt(3, 5.5), pt(9, 5.5), pt(11, 8), pt(21, 8), pt(21, 19), pt(3, 19))},
	"globe":                         {circle(12, 12, 9), line(pt(3, 12), pt(21, 12)), line(pt(12, 3), pt(15, 7), pt(16, 12), pt(15, 17), pt(12, 21)), line(pt(12, 3), pt(9, 7), pt(8, 12), pt(9, 17), pt(12, 21))},
	"terminal":                      {box(3, 4, 18, 16, 2.5), line(pt(7, 9), pt(10, 12), pt(7, 15)), line(pt(12.5, 15), pt(16.5, 15))},
	"gearshape":                     {loop(gear(12, 12, 9.5, 7, 8)...), circle(12, 12, 3)},
	"app.dashed":                    {arc(7, 7, 4, 180, 270), arc(17, 7, 4, 270, 360), arc(17, 17, 4, 0, 90), arc(7, 17, 4, 90, 180), line(pt(10.5, 3), pt(13.5, 3)), line(pt(21, 10.5), pt(21, 13.5)), line(pt(10.5, 21), pt(13.5, 21)), line(pt(3, 10.5), pt(3, 13.5))},
	"app.badge":                     {box(3, 6, 15, 15, 3.5), disc(18.5, 5.5, 3.2)},
	"plus.forwardslash.minus":       {line(pt(8, 4.5), pt(8, 11.5)), line(pt(4.5, 8), pt(11.5, 8)), line(pt(17.5, 4), pt(6.5, 20)), line(pt(13, 16.5), pt(19.5, 16.5))},
	"lock.open":                     {box(5, 11, 14, 10, 2), line(pt(8, 11), pt(8, 7)), arc(12, 7, 4, 180, 360), disc(12, 16, 1.3)},
	"exclamationmark.triangle.fill": {fill(pt(12, 2.5), pt(22, 20), pt(2, 20)), cut(pt(12, 9), pt(12, 13.5)), cut(pt(12, 16.2), pt(12, 17.2))},
}

func points(p [][2]float64) string {
	var b strings.Builder
	for i, q := range p {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%.3f,%.3f", q[0], q[1])
	}
	return b.String()
}

// iconSVG writes the icon as an SVG on its 24-unit grid.
func iconSVG(name string, weight int) string {
	ops := iconOps[name]
	sw := 1.6 + 0.35*float64(weight)
	stroke := fmt.Sprintf(`fill="none" stroke="#000" stroke-width="%.2f" stroke-linecap="round" stroke-linejoin="miter" stroke-miterlimit="2"`, sw)
	var shapes, cuts strings.Builder
	for _, o := range ops {
		switch o.kind {
		case "line":
			fmt.Fprintf(&shapes, `<polyline points="%s" %s/>`, points(o.pts), stroke)
		case "loop":
			fmt.Fprintf(&shapes, `<polygon points="%s" %s/>`, points(o.pts), stroke)
		case "fill":
			fmt.Fprintf(&shapes, `<polygon points="%s" fill="#000"/>`, points(o.pts))
		case "cut":
			fmt.Fprintf(&cuts, `<polyline points="%s" fill="none" stroke="#000" stroke-width="%.2f" stroke-linecap="butt" stroke-linejoin="miter" stroke-miterlimit="2"/>`, points(o.pts), sw)
		case "circle":
			fmt.Fprintf(&shapes, `<circle cx="%g" cy="%g" r="%g" %s/>`, o.v[0], o.v[1], o.v[2], stroke)
		case "disc":
			fmt.Fprintf(&shapes, `<circle cx="%g" cy="%g" r="%g" fill="#000"/>`, o.v[0], o.v[1], o.v[2])
		case "box":
			fmt.Fprintf(&shapes, `<rect x="%g" y="%g" width="%g" height="%g" rx="%g" %s/>`, o.v[0], o.v[1], o.v[2], o.v[3], o.v[4], stroke)
		case "solid":
			fmt.Fprintf(&shapes, `<rect x="%g" y="%g" width="%g" height="%g" rx="%g" fill="#000"/>`, o.v[0], o.v[1], o.v[2], o.v[3], o.v[4])
		case "arc":
			var p [][2]float64
			for i := 0; i <= 32; i++ {
				t := (o.v[3] + (o.v[4]-o.v[3])*float64(i)/32) * math.Pi / 180
				p = append(p, pt(o.v[0]+o.v[2]*math.Cos(t), o.v[1]+o.v[2]*math.Sin(t)))
			}
			fmt.Fprintf(&shapes, `<polyline points="%s" %s/>`, points(p), stroke)
		}
	}
	body := shapes.String()
	if cuts.Len() > 0 {
		// Cut-outs: what is drawn before them, minus their strokes.
		m := cuts.String() // black in a white mask: cut away
		body = `<defs><mask id="c" maskUnits="userSpaceOnUse" x="0" y="0" width="24" height="24"><rect x="0" y="0" width="24" height="24" fill="#fff"/>` + m + `</mask></defs><g mask="url(#c)">` + body + `</g>`
	}
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">` + body + `</svg>`
}

var (
	iconMu    sync.Mutex
	iconCache = map[[2]any]*ui.SVG{}
)

// IconSVG returns the parsed icon name at weight, made once.
func IconSVG(name string, weight int) *ui.SVG {
	iconMu.Lock()
	defer iconMu.Unlock()
	k := [2]any{name, weight}
	if s, ok := iconCache[k]; ok {
		return s
	}
	if _, ok := iconOps[name]; !ok {
		name = "app.dashed"
	}
	s := ui.MustParseSVG([]byte(iconSVG(name, weight)))
	iconCache[k] = s
	return s
}

// Symbol shows the icon as nanoui draws one of size pt: 1.2×pt square, in
// the text's color.
func Symbol(c *ui.Context, name string, size float32, weight int) ui.Element {
	return ui.Icon(c, IconSVG(name, weight)).FontSize(size * 1.2)
}
