package core

import (
	"encoding/json"
	"strings"
	"time"
)

// Google's completions for what is typed: the endpoint Firefox uses, no
// cookies kept, a short answer or none. Nothing that reads as an address
// goes out, and an answer counts only for the words it says it answers.
// Answers are kept (SuggestCache), and while a new one is on its way the
// answer to fewer letters stands in for it. Port of suggest.zig.

const (
	// SuggestMax is how many completions are listed at most.
	SuggestMax = 4
	// SuggestMaxLen: a query or a completion longer than this is left out.
	SuggestMaxLen = 120
	// SuggestMaxBytes: a longer answer is not read.
	SuggestMaxBytes = 32 * 1024
	// SuggestPause is how long typing pauses before asking.
	SuggestPause = 90 * time.Millisecond
	// SuggestTimeout bounds a request.
	SuggestTimeout = 4 * time.Second
)

const blank = " \t\r\n"

// Eligible reports whether typed may be sent to Google at all.
func Eligible(typed string) bool {
	t := strings.Trim(typed, blank)
	if len(t) < 2 || len(t) > SuggestMaxLen {
		return false
	}
	if hasPrefixFold(t, "www.") || hasPrefixFold(t, "localhost") {
		return false
	}
	for i := 0; i < len(t); i++ {
		switch ch := t[i]; {
		case ch <= 0x1f, ch == 0x7f, ch == ':', ch == '@', ch == '\\', ch == '/', ch == '.':
			return false
		}
	}
	return true
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// HasPrefixFold reports whether s starts with prefix, ASCII case folded.
func HasPrefixFold(s, prefix string) bool { return startsWithFold(s, prefix) }

// SuggestURL is the address that asks for typed's completions.
func SuggestURL(typed string) string {
	return "https://suggestqueries.google.com/complete/search?client=firefox&q=" + Escape(typed)
}

// Plain returns s as words to compare: ASCII lowered, runs of spaces made
// one, none at either end, at most SuggestMaxLen bytes.
func Plain(s string) string {
	var b []byte
	space := false
	t := strings.Trim(s, blank)
	for i := 0; i < len(t); i++ {
		if len(b) == SuggestMaxLen {
			break
		}
		ch := t[i]
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			space = true
			continue
		}
		if space {
			b = append(b, ' ')
			space = false
			if len(b) == SuggestMaxLen {
				break
			}
		}
		b = append(b, lower(ch))
	}
	return string(b)
}

// Same reports whether a and b are the same words.
func Same(a, b string) bool { return Plain(a) == Plain(b) }

func hasSame(l []string, s string) bool {
	for _, x := range l {
		if Same(x, s) {
			return true
		}
	}
	return false
}

// DecodeSuggestions reads Google's answer (["typed", ["completion", …], …])
// as completions of typed: false when it isn't one, or answers other words.
// The words typed themselves, repeats, and long ones are left out.
func DecodeSuggestions(body []byte, typed string) ([]string, bool) {
	if len(body) > SuggestMaxBytes {
		return nil, false
	}
	var v []json.RawMessage
	if json.Unmarshal(body, &v) != nil || len(v) < 2 {
		return nil, false
	}
	var echoed string
	var choices []json.RawMessage
	if json.Unmarshal(v[0], &echoed) != nil || json.Unmarshal(v[1], &choices) != nil || choices == nil || !Same(echoed, typed) {
		return nil, false
	}
	out := []string{}
	for _, raw := range choices {
		var choice string
		if json.Unmarshal(raw, &choice) != nil {
			continue
		}
		text := strings.Trim(choice, blank)
		if text == "" || len(text) > SuggestMaxLen || Same(text, typed) || hasSame(out, text) {
			continue
		}
		out = append(out, text)
		if len(out) == SuggestMax {
			break
		}
	}
	return out, true
}

// SuggestCache keeps the last 64 answers, by the words they answer; the
// oldest goes first once full.
type SuggestCache struct {
	keys  []string
	lists [][]string
	next  int
}

const cacheSlots = 64

// Put keeps Google's answer for typed.
func (c *SuggestCache) Put(typed string, l []string) {
	key := Plain(typed)
	l = append([]string(nil), l...)
	if i := c.find(key); i >= 0 {
		c.lists[i] = l
		return
	}
	if len(c.keys) < cacheSlots {
		c.keys = append(c.keys, key)
		c.lists = append(c.lists, l)
	} else {
		c.keys[c.next], c.lists[c.next] = key, l
	}
	c.next = (c.next + 1) % cacheSlots
}

func (c *SuggestCache) find(key string) int {
	for i, k := range c.keys {
		if k == key {
			return i
		}
	}
	return -1
}

// Lookup returns what to show for typed now: Google's answer to it (true:
// no need to ask), or its answer to fewer of the letters, kept to what
// still begins with them (false).
func (c *SuggestCache) Lookup(typed string) ([]string, bool) {
	key := Plain(typed)
	if key == "" {
		return nil, false
	}
	if i := c.find(key); i >= 0 {
		return append([]string(nil), c.lists[i]...), true
	}
	for n := len(key) - 1; n >= 1; n-- {
		i := c.find(key[:n])
		if i < 0 {
			continue
		}
		var out []string
		for _, s := range c.lists[i] {
			if p := Plain(s); len(p) > len(key) && strings.HasPrefix(p, key) {
				out = append(out, s)
			}
		}
		return out, false
	}
	return nil, false
}
