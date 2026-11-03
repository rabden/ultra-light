package core

import "testing"

func section(t *testing.T, r *Results, i int) string {
	t.Helper()
	s, ok := r.Section(i)
	if !ok {
		return "<none>"
	}
	return s
}

func TestResults(t *testing.T) {
	names := []string{"Safari", "System Settings", "Calculator"}
	var r Results
	r.Build("saf", names, nil, nil, nil)
	if r.Rows[0].Kind != KindApp || r.Rows[0].App != 0 {
		t.Fatalf("saf: %+v", r.Rows[0])
	}
	if r.Rows[len(r.Rows)-3].Kind != KindSearchWeb {
		t.Error("Google search must come third from last")
	}
	for i, want := range []string{"Applications", "Google", "Ask & Search", "<none>"} {
		if got := section(t, &r, i); got != want {
			t.Errorf("section %d = %q, want %q", i, got, want)
		}
	}

	r.Build("2^10 * 3", names, nil, nil, []string{"ignored"})
	if r.Rows[0].Kind != KindCalc || r.Answer != "3,072" || r.Hex != "0xC00" || len(r.Rows) != 1 {
		t.Fatalf("math: %+v %q %q", r.Rows, r.Answer, r.Hex)
	}

	r.Build("", names, nil, []int{2, 0}, nil)
	if len(r.Rows) != 2 || section(t, &r, 0) != "Suggestions" {
		t.Fatalf("empty: %+v", r.Rows)
	}

	r.Build("liquid glass", names, nil, nil, nil)
	if len(r.Rows) != 3 || r.Rows[0].Kind != KindSearchWeb || section(t, &r, 0) != "Google" || section(t, &r, 1) != "Ask & Search" {
		t.Fatalf("no app: %+v", r.Rows)
	}

	r.Build("liquid gl", names, nil, nil, []string{"liquid glass", "liquid glue"})
	if len(r.Rows) != 5 || r.Rows[0].Kind != KindSearchWeb || r.Rows[1].Kind != KindSuggest || r.Rows[2].Web != 1 || r.Rows[3].Kind != KindAskGrok {
		t.Fatalf("completions: %+v", r.Rows)
	}
	if section(t, &r, 1) != "<none>" || section(t, &r, 2) != "<none>" || section(t, &r, 3) != "Ask & Search" {
		t.Error("completions must share Google's section")
	}

	r.Build("settings", names, nil, nil, nil)
	if r.Rows[0].Kind != KindApp || r.Rows[0].App != 1 || r.Rows[1].Kind != KindSettings {
		t.Fatalf("settings: %+v", r.Rows)
	}

	many := []string{"Safari", "Mail", "Maps", "Pages", "Calendar", "Calculator", "FaceTime"}
	r.Build("a", many, nil, nil, []string{"a1", "a2", "a3", "a4"})
	if len(r.Rows) != MaxRows || r.Rows[MaxApps].Kind != KindSearchWeb || r.Rows[MaxApps+1].Kind != KindSuggest || r.Rows[MaxRows-1].Kind != KindSearchX {
		t.Fatalf("full: %d rows %+v", len(r.Rows), r.Rows)
	}
}

func TestURL(t *testing.T) {
	if got := URL(KindSearchX, " liquid glass & UI "); got != "https://x.com/search?f=live&q=liquid+glass+%26+UI" {
		t.Error(got)
	}
	if got := URL(KindSearchWeb, "a/b"); got != "https://www.google.com/search?q=a%2Fb" {
		t.Error(got)
	}
	if got := URL(KindApp, "x"); got != "" {
		t.Error(got)
	}
}
