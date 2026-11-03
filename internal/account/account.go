package account

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rabden/ultra-light/internal/store"
)

const (
	Gateway  = "https://gateway.codegraff.com"
	TermsURL = "https://codegraff.com/terms"
	// Service and Key name the secret, as the Mac's Keychain item does.
	Service = "dev.nanoui.nanolight"
	Key     = "codegraff"

	pullEvery = 5 * time.Minute
	timeout   = 15 * time.Second
)

// Shortened by tests.
var (
	pushAfter = 2 * time.Second
	second    = time.Second
)

// State is where signing in is.
type State int

const (
	SignedOut State = iota
	Starting
	Waiting
	SignedIn
)

// Secrets keeps the key (the keyring, else a 0600 file).
type Secrets interface {
	Get() (string, bool)
	Set(key string) bool
	Delete()
}

// Hooks is what the account asks of the app.
type Hooks interface {
	// Snapshot is this computer's settings now.
	Snapshot() Settings
	// Apply takes the account's settings merged with this computer's.
	Apply(s Settings)
	// Changed: signed in or out, or waiting on the browser.
	Changed()
	// OpenURL opens the approval page in the browser.
	OpenURL(u string)
}

// Account is the Codegraff account, on the main thread: HTTP runs on
// goroutines and hands its answers back through Post.
type Account struct {
	Base    string
	KV      *store.Store
	Secrets Secrets
	Hooks   Hooks
	// Post runs fn on the main thread.
	Post func(fn func())
	// Now is time.Now, for tests.
	Now    func() time.Time
	Client *http.Client

	state      State
	key        string
	email      string
	deviceCode string
	interval   time.Duration
	deadline   time.Time
	lastPull   time.Time
	pulling    bool
	pushes     int
	gen        int
}

func (a *Account) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func (a *Account) State() State  { return a.state }
func (a *Account) Email() string { return a.email }
func (a *Account) base() string {
	if a.Base != "" {
		return a.Base
	}
	return Gateway
}

// Load: signed in already if a key is kept, and then synced (at launch).
func (a *Account) Load() {
	k, ok := a.Secrets.Get()
	if !ok || k == "" {
		return
	}
	a.key = k
	a.email, _ = a.KV.Get(store.KeyAccountEmail)
	a.state = SignedIn
	a.Pull()
}

type response struct {
	status int
	body   []byte
}

// fetch runs a request on a goroutine; done gets its answer on the main
// thread (status 0: no answer).
func (a *Account) fetch(method, path string, body []byte, auth bool, done func(response)) {
	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, a.base()+path, rd)
	if err != nil {
		a.Post(func() { done(response{}) })
		return
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+a.key)
	}
	go func() {
		var r response
		if resp, err := client.Do(req); err == nil {
			r.status = resp.StatusCode
			r.body, _ = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
		}
		a.Post(func() { done(r) })
	}()
}

// ----- signing in -----

// SignIn starts the device flow: the browser opens codegraff.com to approve
// it, and the key it gives is kept.
func (a *Account) SignIn() {
	if a.state != SignedOut {
		return
	}
	a.state = Starting
	a.gen++
	gen := a.gen
	a.Hooks.Changed()
	a.fetch("POST", "/v1/device/start", []byte(`{"device_label":"nanolight"}`), false, func(r response) {
		if gen == a.gen {
			a.started(r)
		}
	})
}

func (a *Account) started(r response) {
	if a.state != Starting {
		return
	}
	var s struct {
		DeviceCode string `json:"device_code"`
		URL        string `json:"verification_uri_complete"`
		Interval   *int   `json:"interval"`
		ExpiresIn  *int   `json:"expires_in"`
	}
	if r.status != 200 || json.Unmarshal(r.body, &s) != nil || len(s.DeviceCode) != 64 || !strings.HasPrefix(s.URL, "https://") {
		a.fail()
		return
	}
	interval, expires := 5, 600
	if s.Interval != nil {
		interval = *s.Interval
	}
	if s.ExpiresIn != nil {
		expires = *s.ExpiresIn
	}
	a.deviceCode = s.DeviceCode
	a.interval = time.Duration(min(max(interval, 2), 30)) * second
	a.deadline = a.now().Add(time.Duration(expires) * second)
	a.state = Waiting
	a.Hooks.OpenURL(s.URL)
	a.Hooks.Changed()
	a.pollLater()
}

func (a *Account) fail() {
	a.state = SignedOut
	a.Hooks.Changed()
}

func (a *Account) pollLater() {
	gen := a.gen
	time.AfterFunc(a.interval, func() {
		a.Post(func() {
			if gen == a.gen {
				a.poll()
			}
		})
	})
}

func (a *Account) poll() {
	if a.state != Waiting {
		return
	}
	if a.now().After(a.deadline) {
		a.fail()
		return
	}
	gen := a.gen
	body, _ := json.Marshal(map[string]string{"device_code": a.deviceCode})
	a.fetch("POST", "/v1/device/poll", body, false, func(r response) {
		if gen == a.gen {
			a.polled(r)
		}
	})
}

func (a *Account) polled(r response) {
	if a.state != Waiting {
		return
	}
	var p struct {
		Status string  `json:"status"`
		APIKey *string `json:"api_key"`
		Email  *string `json:"email"`
	}
	json.Unmarshal(r.body, &p)
	// Offline for a moment, or still waiting: ask again.
	if r.status == 0 || p.Status == "pending" {
		a.pollLater()
		return
	}
	if p.Status != "ok" || p.APIKey == nil || !a.Secrets.Set(*p.APIKey) {
		a.fail()
		return
	}
	a.key = *p.APIKey
	if p.Email != nil {
		a.email = *p.Email
		a.KV.Set(store.KeyAccountEmail, a.email)
	}
	a.state = SignedIn
	a.Hooks.Changed()
	a.Pull()
}

// SignOut revokes the key and forgets it; this computer's settings stay.
// While waiting on the browser, it stops waiting.
func (a *Account) SignOut() {
	if a.key != "" {
		a.fetch("POST", "/v1/keys/revoke", nil, true, func(response) {})
	}
	a.forget()
}

func (a *Account) forget() {
	a.gen++
	a.key = ""
	a.Secrets.Delete()
	a.KV.Remove(store.KeyAccountEmail)
	a.KV.Remove(store.KeyAccountSynced)
	a.email = ""
	a.state = SignedOut
	a.Hooks.Changed()
}

// ----- syncing -----

// MaybePull: when the panel opens, the account's record if the last look
// is five minutes old.
func (a *Account) MaybePull() {
	if a.state == SignedIn && a.now().Sub(a.lastPull) > pullEvery {
		a.Pull()
	}
}

// Pull reads the account's record, merges it, and sends back what differs.
func (a *Account) Pull() {
	if a.state != SignedIn || a.pulling {
		return
	}
	a.pulling = true
	a.lastPull = a.now()
	gen := a.gen
	a.fetch("GET", "/v1/sync/nanolight?since=0&limit=10", nil, true, func(r response) {
		a.pulling = false
		if gen == a.gen {
			a.pulled(r)
		}
	})
}

func (a *Account) pulled(r response) {
	if a.state != SignedIn {
		return
	}
	// The key was revoked (signed out elsewhere, or deleted on the web).
	if r.status == 401 || r.status == 403 {
		a.forget()
		return
	}
	if r.status != 200 {
		return
	}
	var page struct {
		Records []struct {
			ID      string  `json:"id"`
			Body    *string `json:"body"`
			Deleted bool    `json:"deleted"`
		} `json:"records"`
	}
	if json.Unmarshal(r.body, &page) != nil {
		return
	}
	var remote *Settings
	for _, rec := range page.Records {
		if rec.ID == "settings" && !rec.Deleted && rec.Body != nil {
			if s, ok := Decode([]byte(*rec.Body)); ok {
				remote = &s
			}
		}
	}
	here := a.Hooks.Snapshot()
	if remote == nil {
		a.send(here)
		return
	}
	merged := Merge(here, *remote, !a.KV.Has(store.KeyAccountSynced))
	a.Hooks.Apply(merged)
	a.KV.Set(store.KeyAccountSynced, "1")
	if !Same(merged, *remote) {
		a.send(merged)
	}
}

// Touched: this computer's settings changed; they go once changes pause.
func (a *Account) Touched() {
	if a.state != SignedIn {
		return
	}
	a.pushes++
	time.AfterFunc(pushAfter, func() {
		a.Post(func() {
			a.pushes--
			if a.pushes > 0 || a.state != SignedIn {
				return
			}
			a.send(a.Hooks.Snapshot())
		})
	})
}

func (a *Account) send(s Settings) {
	body, _ := json.Marshal(map[string]any{"records": []map[string]string{{"id": "settings", "body": string(Encode(s))}}})
	gen := a.gen
	a.fetch("POST", "/v1/sync/nanolight", body, true, func(r response) {
		if gen != a.gen {
			return
		}
		if r.status == 200 {
			a.KV.Set(store.KeyAccountSynced, "1")
		}
		if r.status == 401 || r.status == 403 {
			a.forget()
		}
	})
}
