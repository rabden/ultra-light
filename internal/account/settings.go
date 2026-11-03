// Package account signs in to Codegraff and syncs Nanolight's settings
// through it to every computer on the same account (account.zig,
// settings.zig). Optional: nothing leaves the computer until someone signs
// in, and signing out stops it.
package account

import (
	"encoding/json"
	"sort"
)

// Launch is how often an app was opened.
type Launch struct {
	Name  string `json:"name"`
	Count uint16 `json:"count"`
}

// Settings is what syncs, as one record: the key that opens Nanolight,
// whether Google's suggestions are on, and the launch counts. JSON, the
// same as the Mac's and the Zig build's.
type Settings struct {
	V int `json:"v"`
	// Hotkey is the preset's name ("cmd_space"), or a Mac's recorded key.
	Hotkey  string `json:"hotkey"`
	Suggest bool   `json:"suggest"`
	// At is when Hotkey or Suggest last changed, on whichever computer
	// (unix seconds).
	At       int64    `json:"at"`
	Launches []Launch `json:"launches"`
}

// Encode is a record's body.
func Encode(s Settings) []byte {
	s.V = 1
	if s.Launches == nil {
		s.Launches = []Launch{}
	}
	b, _ := json.Marshal(s)
	return b
}

// Decode reads a record's body; false when it isn't one of ours.
func Decode(body []byte) (Settings, bool) {
	var s Settings
	if json.Unmarshal(body, &s) != nil || s.V != 1 || s.Hotkey == "" || len(s.Hotkey) > 32 {
		return Settings{}, false
	}
	return s, true
}

// Merge merges two computers' settings, never overwriting: the key and the
// suggestions go with whichever changed them last (preferRemote: this
// computer's first sync, where the account's win over a fresh install's),
// and each app keeps the higher count of the two, sorted by name.
func Merge(local, remote Settings, preferRemote bool) Settings {
	out := Settings{V: 1, Hotkey: local.Hotkey, Suggest: local.Suggest, At: local.At}
	if preferRemote || remote.At > local.At {
		out.Hotkey, out.Suggest, out.At = remote.Hotkey, remote.Suggest, remote.At
	}
	idx := map[string]int{}
	for _, side := range [][]Launch{local.Launches, remote.Launches} {
		for _, l := range side {
			if l.Name == "" || len(l.Name) > 255 {
				continue
			}
			if i, ok := idx[l.Name]; ok {
				out.Launches[i].Count = max(out.Launches[i].Count, l.Count)
				continue
			}
			idx[l.Name] = len(out.Launches)
			out.Launches = append(out.Launches, l)
		}
	}
	sort.SliceStable(out.Launches, func(i, j int) bool { return out.Launches[i].Name < out.Launches[j].Name })
	return out
}

// Same reports whether two say the same.
func Same(a, b Settings) bool {
	if a.Hotkey != b.Hotkey || a.Suggest != b.Suggest || a.At != b.At || len(a.Launches) != len(b.Launches) {
		return false
	}
	for i := range a.Launches {
		if a.Launches[i] != b.Launches[i] {
			return false
		}
	}
	return true
}
