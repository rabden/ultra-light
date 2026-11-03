package account

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rabden/ultra-light/internal/store"
)

func TestMerge(t *testing.T) {
	here := Settings{Hotkey: "cmd_space", Suggest: true, At: 100, Launches: []Launch{{"Safari", 5}, {"Notes", 1}}}
	there := Settings{Hotkey: "opt_space", Suggest: false, At: 200, Launches: []Launch{{"Notes", 4}, {"Mail", 2}}}
	m := Merge(here, there, false)
	if m.Hotkey != "opt_space" || m.Suggest || len(m.Launches) != 3 || m.Launches[0].Name != "Mail" || m.Launches[1].Count != 4 || m.Launches[2].Count != 5 {
		t.Fatalf("%+v", m)
	}
	// Merged either way round, the same.
	if !Same(m, Merge(there, here, false)) {
		t.Fatal("symmetric")
	}
	// This computer changed its key last: it stays; a first sync takes the
	// account's.
	later := Settings{Hotkey: "ctrl_space", Suggest: true, At: 300}
	if Merge(later, there, false).Hotkey != "ctrl_space" || Merge(later, there, true).Hotkey != "opt_space" {
		t.Fatal("who wins")
	}
}

func TestRecord(t *testing.T) {
	s := Settings{Hotkey: "cmd_space", Suggest: true, At: 1793600000, Launches: []Launch{{`Visual "Studio" Code`, 3}}}
	back, ok := Decode(Encode(s))
	if !ok || !Same(s, back) {
		t.Fatal("round trip")
	}
	for _, b := range []string{`{"v":2,"hotkey":"x","suggest":true,"at":1}`, "not json"} {
		if _, ok := Decode([]byte(b)); ok {
			t.Errorf("%s taken", b)
		}
	}
	// Fields a later version adds are passed over.
	if _, ok := Decode([]byte(`{"v":1,"hotkey":"cmd_space","suggest":false,"at":5,"theme":"dark"}`)); !ok {
		t.Fatal("later fields")
	}
	// A Zig-written record reads.
	if s, ok := Decode([]byte(`{"v":1,"hotkey":"opt_space","suggest":true,"at":7,"launches":[{"name":"Files","count":2}]}`)); !ok || s.Launches[0].Count != 2 {
		t.Fatal("zig record")
	}
}

// gateway is a fake Codegraff gateway.
type gateway struct {
	mu       sync.Mutex
	polls    int
	pending  int
	record   string
	pushed   []string
	revoked  int
	authFail bool
}

func (g *gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	switch r.URL.Path {
	case "/v1/device/start":
		if !strings.Contains(string(body), `"device_label":"nanolight"`) {
			w.WriteHeader(400)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"device_code": strings.Repeat("d", 64), "verification_uri_complete": "https://codegraff.com/device?c=1", "interval": 1, "expires_in": 600})
	case "/v1/device/poll":
		g.polls++
		if g.polls <= g.pending {
			json.NewEncoder(w).Encode(map[string]any{"status": "pending"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "api_key": "k-123", "email": "ring@example.com"})
	case "/v1/sync/nanolight":
		if g.authFail || r.Header.Get("Authorization") != "Bearer k-123" {
			w.WriteHeader(401)
			return
		}
		if r.Method == "POST" {
			var p struct {
				Records []struct{ ID, Body string } `json:"records"`
			}
			json.Unmarshal(body, &p)
			g.pushed = append(g.pushed, p.Records[0].Body)
			g.record = p.Records[0].Body
			return
		}
		recs := []map[string]any{}
		if g.record != "" {
			recs = append(recs, map[string]any{"id": "settings", "body": g.record})
		}
		json.NewEncoder(w).Encode(map[string]any{"records": recs})
	case "/v1/keys/revoke":
		g.revoked++
	default:
		w.WriteHeader(404)
	}
}

type memSecret struct{ v string }

func (m *memSecret) Get() (string, bool) { return m.v, m.v != "" }
func (m *memSecret) Set(k string) bool   { m.v = k; return true }
func (m *memSecret) Delete()             { m.v = "" }

type hooks struct {
	local   Settings
	applied []Settings
	changes int
	opened  []string
}

func (h *hooks) Snapshot() Settings { return h.local }
func (h *hooks) Apply(s Settings)   { h.applied = append(h.applied, s); h.local = s }
func (h *hooks) Changed()           { h.changes++ }
func (h *hooks) OpenURL(u string)   { h.opened = append(h.opened, u) }

type rig struct {
	t  *testing.T
	g  *gateway
	a  *Account
	h  *hooks
	s  *memSecret
	ch chan func()
}

func newRig(t *testing.T) *rig {
	second = time.Millisecond
	pushAfter = 5 * time.Millisecond
	g := &gateway{}
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	r := &rig{t: t, g: g, h: &hooks{local: Settings{Hotkey: "cmd_space", Suggest: true, At: 100}}, s: &memSecret{}, ch: make(chan func(), 64)}
	r.a = &Account{Base: srv.URL, KV: store.Memory(), Secrets: r.s, Hooks: r.h, Post: func(fn func()) { r.ch <- fn }}
	return r
}

// until runs what comes back to the main thread until ok holds.
func (r *rig) until(ok func() bool) {
	r.t.Helper()
	deadline := time.After(5 * time.Second)
	for !ok() {
		select {
		case fn := <-r.ch:
			fn()
		case <-deadline:
			r.t.Fatal("timed out")
		}
	}
}

func TestSignInPullPush(t *testing.T) {
	r := newRig(t)
	r.g.pending = 2
	r.g.record = string(Encode(Settings{Hotkey: "opt_space", Suggest: false, At: 50, Launches: []Launch{{"Files", 3}}}))
	r.a.SignIn()
	if r.a.State() != Starting {
		t.Fatal("starting")
	}
	r.until(func() bool { return r.a.State() == Waiting })
	if len(r.h.opened) != 1 || r.h.opened[0] != "https://codegraff.com/device?c=1" {
		t.Fatal("the browser opens")
	}
	r.until(func() bool { return len(r.h.applied) == 1 })
	if r.a.State() != SignedIn || r.a.Email() != "ring@example.com" || r.s.v != "k-123" || r.g.polls != 3 {
		t.Fatalf("signed in: %v %q %q %d", r.a.State(), r.a.Email(), r.s.v, r.g.polls)
	}
	// The first sync takes the account's key, though this one's is newer.
	if got := r.h.applied[0]; got.Hotkey != "opt_space" || got.Suggest || got.Launches[0].Name != "Files" {
		t.Fatalf("applied %+v", got)
	}
	if !r.a.KV.Has(store.KeyAccountSynced) {
		t.Fatal("synced")
	}
	// A change goes once changes pause: one push for two touches.
	r.h.local.Hotkey, r.h.local.At = "ctrl_space", 300
	r.a.Touched()
	r.a.Touched()
	r.until(func() bool { return r.a.pushes == 0 })
	time.Sleep(50 * time.Millisecond)
	r.g.mu.Lock()
	n := len(r.g.pushed)
	last := r.g.pushed[n-1]
	r.g.mu.Unlock()
	if s, _ := Decode([]byte(last)); s.Hotkey != "ctrl_space" {
		t.Fatalf("pushed %s", last)
	}
	// Signing out revokes and forgets.
	r.a.SignOut()
	if r.a.State() != SignedOut || r.s.v != "" || r.a.KV.Has(store.KeyAccountSynced) {
		t.Fatal("signed out")
	}
}

func TestNoRecordPushesAndRevokedForgets(t *testing.T) {
	r := newRig(t)
	r.s.v = "k-123"
	r.a.Load()
	if r.a.State() != SignedIn {
		t.Fatal("loaded")
	}
	r.until(func() bool { return r.a.KV.Has(store.KeyAccountSynced) })
	if s, ok := Decode([]byte(r.g.record)); !ok || s.Hotkey != "cmd_space" {
		t.Fatal("no record: this computer's goes up")
	}
	// Revoked elsewhere: the next pull forgets the account.
	r.g.authFail = true
	r.a.lastPull = time.Time{}
	r.a.MaybePull()
	r.until(func() bool { return r.a.State() == SignedOut })
	if r.s.v != "" {
		t.Fatal("key forgotten")
	}
}

func TestBadStart(t *testing.T) {
	r := newRig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"device_code": "short", "verification_uri_complete": "http://x"})
	}))
	defer srv.Close()
	r.a.Base = srv.URL
	r.a.SignIn()
	r.until(func() bool { return r.a.State() == SignedOut })
	if len(r.h.opened) != 0 {
		t.Fatal("nothing opens")
	}
}
