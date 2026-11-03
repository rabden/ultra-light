package core

import (
	"fmt"
	"math"
	"strings"
)

// What the panel lists for a query, in order: the answer when it is math,
// the apps that match it (or, with no query, suggestions), then Google: a
// search for exactly what is typed, first, and Google's completions of it
// under that; last, the query asked of Grok or searched on X. Port of
// results.zig.

// Mod is the key held for the panel's shortcuts (Ctrl+1, Ctrl+↩, Ctrl+,).
const Mod = "Ctrl+"

// Machine is what the panel calls the computer it runs on.
const Machine = "PC"

// Kind is what a row is.
type Kind int

const (
	KindCalc Kind = iota
	KindApp
	KindAskGrok
	KindSearchX
	KindSearchWeb
	// KindSuggest is one of Google's completions (Row.Web).
	KindSuggest
	// KindSettings is Nanolight's own Settings, listed when asked for.
	KindSettings
)

// Label is the row's kind as its trailing label shows it.
func (k Kind) Label() string {
	switch k {
	case KindCalc:
		return "Calculator"
	case KindApp:
		return "Application"
	case KindSettings:
		return "Nanolight"
	}
	return "Web"
}

// Action is what Return does to it, as the footer says.
func (k Kind) Action() string {
	switch k {
	case KindCalc:
		return "Copy Answer"
	case KindApp:
		return "Open Application"
	case KindSettings:
		return "Open Settings"
	case KindAskGrok:
		return "Ask Grok"
	case KindSearchX:
		return "Search X"
	}
	return "Search Google"
}

func (k Kind) section(suggested bool) string {
	switch k {
	case KindCalc:
		return "Calculator"
	case KindApp:
		if suggested {
			return "Suggestions"
		}
		return "Applications"
	case KindSettings:
		return "Nanolight"
	case KindSearchWeb, KindSuggest:
		return "Google"
	}
	return "Ask & Search"
}

// Row is one row of the list.
type Row struct {
	Kind Kind
	// App is the app's index (KindApp).
	App int
	// Web is which of the completions (KindSuggest).
	Web int
	// Match holds which of its name's bytes the query matched.
	Match Match
}

const (
	MaxApps = 6
	// MaxWeb is how many of Google's completions fill what room is left.
	MaxWeb = 4
	// MaxRows leaves room, with every app and the three web rows, for
	// three completions.
	MaxRows  = MaxApps + 3 + 3
	handoffs = 3
)

// Results is the list for one query.
type Results struct {
	Rows []Row
	// Suggested: no query, the apps are suggestions, nothing else listed.
	Suggested bool
	// Value is the math's answer; Answer as shown ("3,072"), Hex a whole
	// answer in hexadecimal ("0xC00") or empty.
	Value  float64
	Answer string
	Hex    string
}

// Build lists what query finds among names (with how often each was opened,
// boost) and Google's completions of it (web), or suggestions (indices into
// names) when the query is empty.
func (r *Results) Build(queryIn string, names []string, boost []uint16, suggestions []int, web []string) {
	r.Rows = r.Rows[:0]
	query := strings.Trim(queryIn, " ")
	r.Suggested = query == ""
	if query == "" {
		for _, i := range suggestions[:min(len(suggestions), MaxApps)] {
			r.push(Row{Kind: KindApp, App: i})
		}
		return
	}
	if v, ok := Eval(query); ok {
		r.Value = v
		r.Answer = FormatNumber(v, true)
		r.Hex = ""
		if v >= 0 && v < 1e15 && v == math.Round(v) {
			r.Hex = fmt.Sprintf("0x%X", uint64(v))
		}
		r.push(Row{Kind: KindCalc})
	}
	for _, x := range Rank(query, names, boost, MaxApps) {
		r.push(Row{Kind: KindApp, App: x.Index, Match: x.Match})
	}
	// "settings" / "preferences": Nanolight's own screen, right under the
	// apps: Return still opens the system's Settings, a key away from this.
	if IsSettingsQuery(query) {
		r.push(Row{Kind: KindSettings})
	}
	if len(r.Rows) > 0 && r.Rows[0].Kind == KindCalc {
		return
	}
	room := MaxRows - len(r.Rows) - handoffs
	r.push(Row{Kind: KindSearchWeb})
	for i := 0; i < min(len(web), MaxWeb, room); i++ {
		r.push(Row{Kind: KindSuggest, Web: i})
	}
	r.push(Row{Kind: KindAskGrok})
	r.push(Row{Kind: KindSearchX})
}

// IsSettingsQuery: "settings" and "preferences", the words people type.
func IsSettingsQuery(query string) bool {
	return strings.EqualFold(query, "settings") || strings.EqualFold(query, "preferences")
}

func (r *Results) push(row Row) {
	if len(r.Rows) < MaxRows {
		r.Rows = append(r.Rows, row)
	}
}

// Section returns the header above row i, if a section starts there.
func (r *Results) Section(i int) (string, bool) {
	name := r.Rows[i].Kind.section(r.Suggested)
	if i > 0 && r.Rows[i-1].Kind.section(r.Suggested) == name {
		return "", false
	}
	return name, true
}
