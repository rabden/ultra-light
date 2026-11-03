package core

// The panel's measures, in points (DIPs), from view.zig.
const (
	PanelWidth  = 680
	HeaderH     = 58
	FooterH     = 38
	RowH        = 40
	CalcH       = 92
	SectionH    = 28
	ListPad     = 6
	PanelRadius = 18
	// MaxHeight is the tallest the panel gets: every app row, the three web
	// rows and the completions under three section heads, or a search's
	// page: 58 + 38 + 2 + 12 + 12×40 + 3×28 = 674.
	MaxHeight = HeaderH + FooterH + 2 + ListPad*2 + MaxRows*RowH + 3*SectionH
	// SettingsHeight: the PC's 560 plus two switch rows (Open Searches in
	// Browser, Show in Tray), the Mac's 652.
	SettingsHeight = 560 + 2*46
)

// SearchHeight is the panel's height for the rows it lists.
func SearchHeight(r *Results) float32 {
	h := float32(HeaderH + FooterH + 1)
	if len(r.Rows) == 0 {
		return h
	}
	h += 1 + ListPad*2
	for i, row := range r.Rows {
		if row.Kind == KindCalc {
			h += CalcH
		} else {
			h += RowH
		}
		if _, ok := r.Section(i); ok {
			h += SectionH
		}
	}
	return h
}

// RowTop returns where row i starts in the list, below its padding, and how
// tall it is: where the selection highlight goes.
func RowTop(r *Results, i int) (y, h float32) {
	for k, row := range r.Rows {
		if _, ok := r.Section(k); ok {
			y += SectionH
		}
		rh := float32(RowH)
		if row.Kind == KindCalc {
			rh = CalcH
		}
		if k == i {
			return y, rh
		}
		y += rh
	}
	return y, RowH
}

// WebAreaHeight is the height of a search's page in a panel h tall: between
// the header and the footer and their hairlines.
func WebAreaHeight(h float32) float32 { return max(0, h-HeaderH-FooterH-2) }
