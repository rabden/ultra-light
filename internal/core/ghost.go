package core

// Ghost returns what the query is the start of, drawn faintly after the
// caret; → or Tab takes it. The top app's name when one is listed first
// ("saf" → "ari"), else Google's first completion that goes on from the
// words ("liquid gl" → "ass"). Nothing when the query is empty, nothing is
// listed, or the query is still the one shown as the panel opened
// (revealed). Return never takes it. Port of view.zig completion().
func Ghost(query string, revealed bool, rows []Row, appName func(int) string, web []string) string {
	if query == "" || len(rows) == 0 || revealed {
		return ""
	}
	r := rows[0]
	switch r.Kind {
	case KindApp:
		name := appName(r.App)
		if len(name) <= len(query) || !startsWithFold(name, query) {
			return ""
		}
		return name[len(query):]
	case KindSearchWeb:
		for _, w := range web {
			if len(w) > len(query) && startsWithFold(w, query) {
				return w[len(query):]
			}
		}
	}
	return ""
}

// HighlightRuns turns matched byte indices into runs [start, end) of
// consecutive bytes, as the row draws them semibold.
func HighlightRuns(hits []int) [][2]int {
	var runs [][2]int
	for k := 0; k < len(hits); {
		end := k + 1
		for end < len(hits) && hits[end] == hits[end-1]+1 {
			end++
		}
		runs = append(runs, [2]int{hits[k], hits[end-1] + 1})
		k = end
	}
	return runs
}
