package app

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/rabden/ultra-light/internal/account"
	"github.com/rabden/ultra-light/internal/hotkey"
	"github.com/rabden/ultra-light/internal/store"
	"github.com/rabden/ultra-light/internal/view"
)

// gateway replaces Codegraff's address in test builds (-ldflags -X).
var gateway string

// startAccount loads the Codegraff account, if one is signed in, and pulls.
func (a *App) startAccount() {
	sec := newSecret()
	sec.migrate()
	a.acct = &account.Account{Base: gateway, KV: a.kv, Secrets: sec, Hooks: accountHooks{a}, Post: a.win.Update}
	a.acct.Load()
}

func (a *App) maybePull() {
	if a.acct != nil {
		a.acct.MaybePull()
	}
}

// touched: something that syncs changed; keyOrSuggest marks when (the key
// and the suggestions go with whichever computer changed them last).
func (a *App) touched(keyOrSuggest bool) {
	if keyOrSuggest {
		a.kv.Set(store.KeySyncAt, strconv.FormatInt(time.Now().Unix(), 10))
	}
	if a.acct != nil {
		a.acct.Touched()
	}
}

func (a *App) accountState() view.AccountState {
	if a.acct == nil {
		return view.SignedOut
	}
	switch a.acct.State() {
	case account.Starting:
		return view.Starting
	case account.Waiting:
		return view.Waiting
	case account.SignedIn:
		return view.SignedIn
	}
	return view.SignedOut
}

func (a *App) accountEmail() string {
	if a.acct == nil {
		return ""
	}
	return a.acct.Email()
}

func (a *App) signIn() {
	if a.acct != nil {
		a.acct.SignIn()
	}
}

func (a *App) signOut() {
	if a.acct != nil {
		a.acct.SignOut()
	}
}

// accountClick is the tray's and Settings' one account button.
func (a *App) accountClick() {
	switch a.accountState() {
	case view.SignedOut:
		a.signIn()
	case view.Waiting, view.SignedIn:
		a.signOut()
	}
}

type accountHooks struct{ a *App }

// Snapshot is this computer's settings: the key, the suggestions, when they
// changed, the launch counts above 0.
func (h accountHooks) Snapshot() account.Settings {
	a := h.a
	at, _ := strconv.ParseInt(must(a.kv.Get(store.KeySyncAt)), 10, 64)
	s := account.Settings{V: 1, Hotkey: a.choice.Name(), Suggest: a.panel.SuggestOn, At: at}
	for i, e := range a.index.Entries {
		if n := a.index.Launches[i]; n > 0 {
			s.Launches = append(s.Launches, account.Launch{Name: e.Name, Count: n})
		}
	}
	return s
}

// Apply takes the account's settings, merged: the key, the suggestions,
// the counts (each the higher).
func (h accountHooks) Apply(s account.Settings) {
	a := h.a
	a.kv.Set(store.KeySyncAt, strconv.FormatInt(s.At, 10))
	if c, ok := hotkey.Parse(s.Hotkey); ok && c != a.choice {
		a.kv.Set(store.KeyHotkey, c.Name())
		a.useKey(c)
	}
	if s.Suggest != a.panel.SuggestOn {
		a.panel.SuggestOn = s.Suggest
		a.kv.SetBool(store.KeySuggest, s.Suggest)
	}
	for _, l := range s.Launches {
		if i := a.index.Find(l.Name); i >= 0 {
			a.index.Launches[i] = max(a.index.Launches[i], l.Count)
		}
	}
	a.kv.Set(store.KeyLaunches, a.index.EncodeLaunches())
	a.panel.Refresh()
	a.settingsChanged()
}

func (h accountHooks) Changed()         { h.a.settingsChanged() }
func (h accountHooks) OpenURL(u string) { h.a.OpenURL(u) }

// secret keeps the Codegraff key in the desktop's keyring (Secret
// Service), the Mac's service and account; without one, in the Zig build's
// 0600 file.
type secret struct {
	ring mygo.SecretsModule
	file string
}

func newSecret() *secret {
	return &secret{
		ring: mygo.Secrets.Service(account.Service),
		file: filepath.Join(store.DataDir(), "secret-"+account.Service+"-"+account.Key),
	}
}

// migrate moves a key the Zig build left in its file into the keyring.
func (s *secret) migrate() {
	b, err := os.ReadFile(s.file)
	if err != nil || len(b) == 0 {
		return
	}
	if s.ring.Set(account.Key, b) == nil {
		os.Remove(s.file)
	}
}

func (s *secret) Get() (string, bool) {
	b, err := s.ring.Get(account.Key)
	if err == nil {
		return string(b), len(b) > 0
	}
	if !errors.Is(err, mygo.ErrSecretNotFound) && !errors.Is(err, errors.ErrUnsupported) {
		log.Printf("nanolight: the keyring: %v", err)
	}
	b, err = os.ReadFile(s.file)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), len(b) > 0
}

func (s *secret) Set(key string) bool {
	if err := s.ring.Set(account.Key, []byte(key)); err == nil {
		return true
	} else {
		log.Printf("nanolight: the keyring: %v; keeping the key in a file only you can read", err)
	}
	f, err := os.OpenFile(s.file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return false
	}
	defer f.Close()
	f.Chmod(0o600)
	_, err = f.WriteString(key)
	return err == nil
}

func (s *secret) Delete() {
	s.ring.Delete(account.Key)
	os.Remove(s.file)
}
