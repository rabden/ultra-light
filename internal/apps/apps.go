// Package apps keeps the index the search ranks: the installed apps, then
// Raycast's rows and script commands, each name once (case-insensitive,
// first one wins), sorted. How often each was opened is kept in the store
// (nanolight.launches) and lifts it in the ranking; with an empty query the
// most opened, then a few starters, are suggested. Port of apps.zig.
package apps

import (
	"slices"
	"strconv"
	"strings"
)

// Kind is what opening an entry does.
type Kind int

const (
	App           Kind = iota // a desktop entry, launched
	RaycastStore              // "Install Raycast Extension…"
	RaycastCmd                // an installed extension's command
	Script                    // a script command
	ScriptsFolder             // "Script Commands Folder", opened in the file manager
)

// Entry is one row the search can find.
type Entry struct {
	Name string
	// Folder is the row's subtitle: "Application" for apps, the
	// extension's title, "Raycast", or a script's package.
	Folder string
	// Path is what opening it takes: the desktop entry's path for an app,
	// "raycast-store:", "raycast:<dir>#<cmd>", "script:<path>".
	Path string
	Kind Kind
	// ID is a desktop entry's file ID (org.gnome.Nautilus.desktop).
	ID string
	// Icon is the file whose picture the row shows: an extension's PNG.
	Icon string
	// Ref is the platform's handle (mygo.InstalledApp), kept for launching.
	Ref any
}

// Starters are suggested with an empty query when nothing was opened yet
// (Linux's, from apps.zig).
var Starters = []string{"Files", "Firefox", "Terminal", "Settings", "Text Editor"}

// Index is the list of entries, with names and launch counts in step.
type Index struct {
	Entries  []Entry
	Names    []string
	Launches []uint16
}

// Add appends an entry unless one of the same name (case-insensitive) is
// there already.
func (ix *Index) Add(e Entry) {
	if ix.Find(e.Name) >= 0 || e.Name == "" {
		return
	}
	ix.Entries = append(ix.Entries, e)
	ix.Names = append(ix.Names, e.Name)
	ix.Launches = append(ix.Launches, 0)
}

// Sort orders the entries alphabetically, case-insensitive, keeping names
// and counts in step.
func (ix *Index) Sort() {
	type pair struct {
		e Entry
		n uint16
	}
	ps := make([]pair, len(ix.Entries))
	for i := range ps {
		ps[i] = pair{ix.Entries[i], ix.Launches[i]}
	}
	slices.SortStableFunc(ps, func(a, b pair) int { return compareFold(a.e.Name, b.e.Name) })
	for i, p := range ps {
		ix.Entries[i], ix.Launches[i], ix.Names[i] = p.e, p.n, p.e.Name
	}
}

// compareFold compares as Zig's lessThanIgnoreCase: bytes, ASCII lowered.
func compareFold(a, b string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		x, y := lower(a[i]), lower(b[i])
		if x != y {
			return int(x) - int(y)
		}
	}
	return len(a) - len(b)
}

func lower(ch byte) byte {
	if ch >= 'A' && ch <= 'Z' {
		return ch + 32
	}
	return ch
}

// Find returns the index of the entry named name (case-insensitive), or -1.
func (ix *Index) Find(name string) int {
	for i, n := range ix.Names {
		if strings.EqualFold(n, name) {
			return i
		}
	}
	return -1
}

// Suggestions returns up to n entries for an empty query: the most opened
// (descending, ties in index order), then the starters there are.
func (ix *Index) Suggestions(n int) []int {
	var out []int
	best := uint16(65535)
	for len(out) < n {
		var next uint16
		for _, l := range ix.Launches {
			if l < best && l > next {
				next = l
			}
		}
		if next == 0 {
			break
		}
		for i, l := range ix.Launches {
			if l == next && len(out) < n {
				out = append(out, i)
			}
		}
		best = next
	}
	for _, s := range Starters {
		if len(out) == n {
			break
		}
		if i := ix.Find(s); i >= 0 && !slices.Contains(out, i) {
			out = append(out, i)
		}
	}
	return out
}

// Opened counts an opening (saturating) and returns the counts encoded for
// the store.
func (ix *Index) Opened(i int) string {
	if ix.Launches[i] < 65535 {
		ix.Launches[i]++
	}
	return ix.EncodeLaunches()
}

// EncodeLaunches writes every count above 0 as "Name\tcount\n" lines.
func (ix *Index) EncodeLaunches() string {
	var b strings.Builder
	for i, e := range ix.Entries {
		if l := ix.Launches[i]; l > 0 {
			b.WriteString(e.Name)
			b.WriteByte('\t')
			b.WriteString(strconv.Itoa(int(l)))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// LoadLaunches reads counts EncodeLaunches wrote, for the entries there are.
func (ix *Index) LoadLaunches(s string) {
	for line := range strings.SplitSeq(s, "\n") {
		name, count, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		i := ix.Find(name)
		if i < 0 {
			continue
		}
		n, err := strconv.ParseUint(count, 10, 16)
		if err != nil {
			n = 0
		}
		ix.Launches[i] = uint16(n)
	}
}

// Counts returns every count above 0 by name, for sync.
func (ix *Index) Counts() map[string]uint16 {
	m := map[string]uint16{}
	for i, e := range ix.Entries {
		if ix.Launches[i] > 0 {
			m[e.Name] = ix.Launches[i]
		}
	}
	return m
}
