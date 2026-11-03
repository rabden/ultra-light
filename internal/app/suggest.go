package app

import (
	"io"
	"net/http"
	"time"

	"github.com/rabden/ultra-light/internal/core"
)

// suggester asks Google for completions as typing pauses (90 ms), only the
// last question of a burst going out, without cookies, within 4 s.
type suggester struct {
	gen    int
	timer  *time.Timer
	client *http.Client
}

// Suggest asks for query's completions once typing pauses.
func (a *App) Suggest(query string) {
	s := &a.sugg
	s.gen++
	gen := s.gen
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(core.SuggestPause, func() {
		a.win.Update(func() {
			// Still what is typed, and still wanted.
			if gen != s.gen || !a.panel.SuggestOn || !core.Same(query, a.panel.Query()) {
				return
			}
			go a.fetchSuggestions(query)
		})
	})
}

func (a *App) fetchSuggestions(query string) {
	s := &a.sugg
	if s.client == nil {
		s.client = &http.Client{Timeout: core.SuggestTimeout} // no cookie jar
	}
	req, err := http.NewRequest("GET", core.SuggestURL(query), nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, core.SuggestMaxBytes+1))
	if err != nil {
		return
	}
	list, ok := core.DecodeSuggestions(body, query)
	if !ok {
		return
	}
	a.win.Update(func() { a.panel.Answered(query, list) })
}
