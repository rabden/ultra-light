package view

import "github.com/egoist/mygo/ui"

// Palette holds the panel's colors (palette.zig, the PC's solid fill).
type Palette struct {
	Text, Sub, Hair, Sel, Rim, Tile, Fill ui.Color
	// From nanoui's theme: success, danger, accent, muted; and the
	// welcome's warning.
	Success, Danger, Accent, Muted, Warn ui.Color
	Dark                                 bool
}

var (
	white = ui.RGBA(255, 255, 255, 1)
	black = ui.RGBA(0, 0, 0, 1)
)

// PaletteFor returns the dark or the light palette.
func PaletteFor(dark bool) Palette {
	if dark {
		return Palette{
			Text: ui.Hex("#eeeeee"), Sub: ui.Hex("#b5b5b5"),
			Hair: white.Alpha(0.12), Sel: white.Alpha(0.16), Rim: white.Alpha(0.1), Tile: white.Alpha(0.09),
			Fill:    ui.Hex("#1b1b1c"),
			Success: ui.Hex("#30d158"), Danger: ui.Hex("#ff453a"), Accent: ui.Hex("#0a84ff"),
			Muted: ui.Hex("#98989d"), Warn: ui.Hex("#e0a33a"), Dark: true,
		}
	}
	return Palette{
		Text: ui.Hex("#292929"), Sub: ui.Hex("#6f6f6f"),
		Hair: black.Alpha(0.09), Sel: black.Alpha(0.08), Rim: black.Alpha(0.05), Tile: black.Alpha(0.05),
		Fill:    ui.Hex("#f9f9fa"),
		Success: ui.Hex("#34c759"), Danger: ui.Hex("#ff3b30"), Accent: ui.Hex("#0a84ff"),
		Muted: ui.Hex("#6e6e73"), Warn: ui.Hex("#b26b00"),
	}
}

// Fonts: the first installed of nanoui's order (ui/tt/fontdb.zig).
const (
	Font = "Inter, Geist, Segoe UI, Noto Sans, Cantarell, Ubuntu, DejaVu Sans, Liberation Sans, sans-serif"
	Mono = "JetBrains Mono, Geist Mono, Cascadia Mono, Fira Mono, Ubuntu Mono, DejaVu Sans Mono, Liberation Mono, monospace"
)

// Weights, as CSS's.
const (
	W400 = 400
	W500 = 500
	W600 = 600
	W700 = 700
)

// theme returns the window's theme for the palette: nanoui's accent for
// the caret and the selection, the panel's fonts.
func theme(base *ui.Theme, p Palette) *ui.Theme {
	t := *base
	t.Accent = p.Accent
	t.InactiveAccent = p.Accent
	t.Selection = p.Accent.Alpha(0.28)
	t.Text = p.Text
	t.TextMuted = p.Sub
	t.Success, t.Danger = p.Success, p.Danger
	t.Font = Font
	t.Background = ui.Transparent
	return &t
}
