package view

import (
	"github.com/egoist/mygo/ui"
)

// Kbd is nanoui's key cap: padding 5×1, radius 4, the text's color at 6%
// behind it and 12% around it, at least 18 wide, 10.5 medium muted.
func Kbd(c *ui.Context, p Palette, s string) ui.Element {
	return ui.Row(c).Padding(1, 5).Radius(4).Background(p.Text.Alpha(0.06)).Border(1, p.Text.Alpha(0.12)).
		MinWidth(18).Justify(ui.Center).Children(func() {
		ui.Text(c, s).FontSize(10.5).FontWeight(W500).TextColor(p.Muted).NoWrap()
	})
}

// Hairline is a 1-point line of the palette's hair color.
func Hairline(c *ui.Context, p Palette) ui.Element {
	return ui.Box(c).FillWidth().Height(1).Background(p.Hair)
}

// VRule is the footer's 1×14 divider.
func VRule(c *ui.Context, p Palette) ui.Element {
	return ui.Box(c).Width(1).Height(14).Background(p.Hair)
}

// SectionLabel is a section's name: 11.5 semibold sub 85%.
func SectionLabel(c *ui.Context, p Palette, s string) ui.Element {
	return ui.Text(c, s).FontSize(11.5).FontWeight(W600).TextColor(p.Sub.Alpha(0.85)).NoWrap()
}

// Chip is the calculator card's label: gap 4, padding-x 8, 20 tall,
// radius 6, on tile, 11 medium tabular; toned (success) with a checkmark.
func Chip(c *ui.Context, p Palette, s string, tone *ui.Color) ui.Element {
	bg, fg := p.Tile, p.Sub
	if tone != nil {
		bg, fg = tone.Alpha(0.16), *tone
	}
	return ui.Row(c).Gap(4).PaddingX(8).Height(20).AlignItems(ui.Center).Radius(6).Background(bg).Children(func() {
		if tone != nil {
			Symbol(c, "checkmark", 10, BoldW).TextColor(fg)
		}
		ui.Text(c, s).FontSize(11).FontWeight(W500).TextColor(fg).FontFeatures("tnum").NoWrap()
	})
}

// Switch is the settings switch: a 40×24 pill, padding-x 2, success when
// on else tile, a 20-point white knob left or right; no animation.
func Switch(c *ui.Context, p Palette, on bool) ui.Element {
	bg := p.Tile
	if on {
		bg = p.Success
	}
	row := ui.Row(c).Size(40, 24).PaddingX(2).Radius(12).Background(bg).AlignItems(ui.Center)
	if on {
		row.Justify(ui.End)
	}
	return row.Children(func() {
		ui.Box(c).Size(20, 20).Radius(10).Background(white)
	})
}
