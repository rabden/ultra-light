// Package core holds Nanolight's logic that needs no window: fuzzy ranking,
// the calculator, the rows of the search screen, search URLs, the ghost
// completion, Google suggestions and the panel's heights. It never imports
// mygo, so its tests run anywhere in milliseconds.
//
// Most of it is a byte-for-byte port of the Zig app (src/search.zig,
// calc.zig, results.zig, suggest.zig, view.zig), whose tests are ported
// next to each file.
package core

import (
	"slices"
	"strings"
)

// MaxHits is how many matched bytes a Match keeps for highlighting.
const MaxHits = 32

// Match is how well a query matches a name, and which of the name's bytes
// matched, in order (drawn brighter in the row).
type Match struct {
	Score int
	Hits  []int
}

func run(points, from, n int) Match {
	m := Match{Score: points}
	for i := from; i < from+n; i++ {
		m.add(i)
	}
	return m
}

func (m *Match) add(i int) {
	if len(m.Hits) < MaxHits && i < 256 {
		m.Hits = append(m.Hits, i)
	}
}

func lower(ch byte) byte {
	if ch >= 'A' && ch <= 'Z' {
		return ch + 32
	}
	return ch
}

func isUpper(ch byte) bool { return ch >= 'A' && ch <= 'Z' }
func isLower(ch byte) bool { return ch >= 'a' && ch <= 'z' }

// WordStart reports whether byte i of name starts a word: the name's start,
// after a space or punctuation, or a capital after a lowercase letter
// (FaceTime).
func WordStart(name string, i int) bool {
	if i == 0 {
		return true
	}
	prev, ch := name[i-1], name[i]
	switch prev {
	case ' ', '-', '_', '.', '/', '(':
		return ch != ' '
	}
	return isUpper(ch) && isLower(prev)
}

func startsWithFold(hay, needle string) bool {
	if len(needle) > len(hay) {
		return false
	}
	for i := 0; i < len(needle); i++ {
		if lower(hay[i]) != lower(needle[i]) {
			return false
		}
	}
	return true
}

func indexOfFold(hay, needle string, from int) int {
	if len(needle) == 0 || len(needle) > len(hay) {
		return -1
	}
	for i := from; i+len(needle) <= len(hay); i++ {
		if startsWithFold(hay[i:], needle) {
			return i
		}
	}
	return -1
}

// Score returns the match of query in name, and false when its letters
// aren't all there in order (nor close enough to be a typo). Tiers, best
// first: the whole name, its start, a word's start, the words' initials,
// anywhere inside, the letters in order, and last, typos.
func Score(queryIn, name string) (Match, bool) {
	query := strings.Trim(queryIn, " ")
	if query == "" || name == "" {
		return Match{}, false
	}
	// Shorter names win ties: "Notes" over "Notes Helper".
	slack := min(max(len(name)-len(query), 0), 60)
	if len(query) == len(name) && startsWithFold(name, query) {
		return run(1000, 0, len(name)), true
	}
	if startsWithFold(name, query) {
		return run(900-slack, 0, len(query)), true
	}
	// A word's start: "code" in Visual Studio Code.
	for at := 0; ; {
		i := indexOfFold(name, query, at)
		if i < 0 {
			break
		}
		if WordStart(name, i) {
			return run(800-slack-min(i, 60), i, len(query)), true
		}
		at = i + 1
	}
	if m, ok := initials(query, name); ok {
		return m, true
	}
	if i := indexOfFold(name, query, 0); i >= 0 {
		return run(600-slack-min(i, 60), i, len(query)), true
	}
	if m, ok := subsequence(query, name); ok {
		return m, true
	}
	return typo(query, name)
}

// allowed is how many typos a query of this length may have: none under 4
// letters (too many names are a letter from "map"), one, then two from 7.
func allowed(n int) int {
	switch {
	case n < 4:
		return 0
	case n < 7:
		return 1
	}
	return 2
}

const (
	maxQ = 32
	maxN = 64
)

// table holds edit distances between the query's first i bytes and the
// first j bytes of a name from a word's start.
type table [maxQ + 1][maxN + 1]int

func swapped(q, w string, i, j int) bool {
	return lower(q[i-1]) == lower(w[j-2]) && lower(q[i-2]) == lower(w[j-1])
}

// dist fills t (optimal string alignment: insert, delete, change, swap
// neighbours) and returns the fewest edits to any prefix of word, and that
// prefix's length.
func dist(t *table, q, word string) (errs, end int) {
	for j := 0; j <= len(word); j++ {
		t[0][j] = j
	}
	for i := 1; i <= len(q); i++ {
		t[i][0] = i
		for j := 1; j <= len(word); j++ {
			change := 1
			if lower(q[i-1]) == lower(word[j-1]) {
				change = 0
			}
			v := min(t[i-1][j]+1, t[i][j-1]+1, t[i-1][j-1]+change)
			if i > 1 && j > 1 && swapped(q, word, i, j) {
				v = min(v, t[i-2][j-2]+1)
			}
			t[i][j] = v
		}
	}
	errs = 255
	for j := 0; j <= len(word); j++ {
		if t[len(q)][j] < errs {
			errs, end = t[len(q)][j], j
		}
	}
	return errs, end
}

// typo finds the query as a misspelt start of the name, or of one of its
// words: the fewest edits that make the query some prefix of it, if few
// enough.
func typo(query, name string) (Match, bool) {
	most := allowed(len(query))
	if most == 0 || len(query) > maxQ {
		return Match{}, false
	}
	var t table
	found := false
	var bErrs, bStart, bEnd int
	for s := 0; s < len(name); s++ {
		if !WordStart(name, s) {
			continue
		}
		word := name[s:min(len(name), s+maxN)]
		errs, end := dist(&t, query, word)
		if errs <= most && (!found || errs < bErrs) {
			found, bErrs, bStart, bEnd = true, errs, s, end
		}
	}
	if !found {
		return Match{}, false
	}
	// The bytes that line up with the query's, for the row's highlight.
	word := name[bStart:min(len(name), bStart+maxN)]
	dist(&t, query, word)
	var back []int
	push := func(v int) {
		if len(back) < MaxHits {
			back = append(back, v)
		}
	}
	i, j := len(query), bEnd
	for i > 0 && j > 0 {
		here := t[i][j]
		switch {
		case lower(query[i-1]) == lower(word[j-1]) && here == t[i-1][j-1]:
			push(min(bStart+j-1, 255))
			i, j = i-1, j-1
		case i > 1 && j > 1 && swapped(query, word, i, j) && here == t[i-2][j-2]+1:
			// Both letters are there, the wrong way round: both lit.
			push(min(bStart+j-1, 255))
			push(min(bStart+j-2, 255))
			i, j = i-2, j-2
		case here == t[i-1][j-1]+1:
			i, j = i-1, j-1
		case here == t[i-1][j]+1:
			i--
		default:
			j--
		}
	}
	m := Match{Score: 260 - 70*bErrs - min(len(name), 60)/4 - min(bStart, 20)}
	for k := len(back); k > 0; k-- {
		m.add(back[k-1])
	}
	return m, true
}

// initials matches the query as the words' initials, in order, every one.
func initials(query, name string) (Match, bool) {
	if len(query) < 2 {
		return Match{}, false
	}
	var m Match
	q := 0
	for i := 0; i < len(name); i++ {
		if q == len(query) {
			break
		}
		if !WordStart(name, i) {
			continue
		}
		if lower(name[i]) == lower(query[q]) {
			m.add(i)
			q++
		}
	}
	if q < len(query) {
		return Match{}, false
	}
	m.Score = 750 - min(len(name), 60)
	return m, true
}

// subsequence matches the letters in order anywhere; each one on a word's
// start or right after the last scores more, gaps score less.
func subsequence(query, name string) (Match, bool) {
	m := Match{Score: 300}
	last := -1
	i := 0
	for k := 0; k < len(query); k++ {
		qc := query[k]
		if qc == ' ' {
			continue
		}
		found := -1
		for ; i < len(name); i++ {
			if lower(name[i]) == lower(qc) {
				found = i
				break
			}
		}
		if found < 0 {
			return Match{}, false
		}
		if WordStart(name, found) {
			m.Score += 12
		}
		if last >= 0 {
			if found == last+1 {
				m.Score += 8
			} else {
				m.Score -= min(found-last, 20)
			}
		} else {
			m.Score -= min(found, 20)
		}
		m.add(found)
		last = found
		i = found + 1
	}
	m.Score -= min(len(name), 60) / 4
	return m, true
}

// Ranked is a ranked candidate: which one (an index the caller keeps) and
// its match.
type Ranked struct {
	Index int
	Match Match
}

// Rank returns the best n of names for query, best first, with boost (how
// often each was opened) adding min(count×25, 250) to its score.
func Rank(query string, names []string, boost []uint16, n int) []Ranked {
	out := make([]Ranked, 0, n)
	if n <= 0 {
		return out
	}
	for i, name := range names {
		m, ok := Score(query, name)
		if !ok {
			continue
		}
		if i < len(boost) {
			m.Score += min(int(boost[i])*25, 250)
		}
		r := Ranked{Index: i, Match: m}
		if len(out) < n {
			out = append(out, r)
			continue
		}
		// Full: replace the worst kept one if this is better.
		worst := 0
		for j := 1; j < len(out); j++ {
			if out[j].Match.Score < out[worst].Match.Score {
				worst = j
			}
		}
		if m.Score > out[worst].Match.Score {
			out[worst] = r
		}
	}
	// Zig's pdq sort is an insertion sort at this size, so stable.
	slices.SortStableFunc(out, func(a, b Ranked) int { return b.Match.Score - a.Match.Score })
	return out
}
