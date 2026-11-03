// Package store keeps Nanolight's settings where the Zig build keeps them,
// so an existing Linux install keeps everything: ~/.local/share/Nanolight
// (or $XDG_DATA_HOME/Nanolight) for kv.json, extensions, script commands
// and the secret's fallback file, ~/.cache/Nanolight for the Raycast host.
//
// kv.json is one JSON object of strings, as nanoui's basics/kv_file.zig
// writes it: each change rewrites the whole file through a temporary one
// renamed over it, so a crash leaves the old file or the new one.
package store

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// The keys Nanolight keeps, by the names the Zig and Mac builds use.
const (
	KeyHotkey        = "nanolight.hotkey"
	KeyOnboarded     = "nanolight.onboarded"
	KeySuggest       = "nanolight.suggest"
	KeySearchesOut   = "nanolight.searches_out"
	KeyMenuBar       = "nanolight.menu_bar" // Show in Tray on Linux
	KeyLaunches      = "nanolight.launches"
	KeySyncAt        = "nanolight.sync.at"
	KeyUpdated       = "nanolight.updated"
	KeyAccountEmail  = "nanolight.account.email"
	KeyAccountSynced = "nanolight.account.synced"
)

// Store is a string-to-string map kept in a kv.json file. Safe for use from
// any goroutine.
type Store struct {
	mu   sync.Mutex
	path string // "" keeps values in memory only
	m    map[string]string
}

// Memory returns a store kept in memory only, for tests.
func Memory() *Store { return &Store{m: map[string]string{}} }

// Open reads <dir>/kv.json if it is there. A file that doesn't parse is left
// alone and the store starts empty (the next change replaces it), as the
// Zig build does.
func Open(dir string) *Store {
	s := &Store{path: filepath.Join(dir, "kv.json"), m: map[string]string{}}
	b, err := os.ReadFile(s.path)
	if err != nil {
		return s
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		log.Printf("nanolight: %s unread: %v", s.path, err)
		return s
	}
	for k, v := range raw {
		if str, ok := v.(string); ok {
			s.m[k] = str
		}
	}
	return s
}

// Get returns the value at key.
func (s *Store) Get(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[key]
	return v, ok
}

// Has reports whether Get would find a value at key.
func (s *Store) Has(key string) bool {
	_, ok := s.Get(key)
	return ok
}

// Bool reads "1" as true and "0" as false, else def.
func (s *Store) Bool(key string, def bool) bool {
	switch v, _ := s.Get(key); v {
	case "1":
		return true
	case "0":
		return false
	}
	return def
}

// SetBool keeps b as "1" or "0".
func (s *Store) SetBool(key string, b bool) {
	v := "0"
	if b {
		v = "1"
	}
	s.Set(key, v)
}

// Set keeps value at key and rewrites the file.
func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.m[key]; ok && old == value {
		return
	}
	s.m[key] = value
	s.save()
}

// Remove drops key, if it is there.
func (s *Store) Remove(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[key]; !ok {
		return
	}
	delete(s.m, key)
	s.save()
}

// save writes the whole store over the old file, as nanoui does: JSON
// indented by one space, through .kv.json.tmp renamed over kv.json.
func (s *Store) save() {
	if s.path == "" {
		return
	}
	b, err := json.MarshalIndent(s.m, "", " ")
	if err != nil {
		return
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("nanolight: %s not written: %v", s.path, err)
		return
	}
	tmp := filepath.Join(dir, ".kv.json.tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		log.Printf("nanolight: %s not written: %v", s.path, err)
		return
	}
	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp)
		log.Printf("nanolight: %s not written: %v", s.path, err)
	}
}

// DataDir is $XDG_DATA_HOME/Nanolight or ~/.local/share/Nanolight, made if
// missing.
func DataDir() string { return appDir("XDG_DATA_HOME", ".local/share") }

// CacheDir is $XDG_CACHE_HOME/Nanolight or ~/.cache/Nanolight, made if
// missing.
func CacheDir() string { return appDir("XDG_CACHE_HOME", ".cache") }

// ConfigDir is $XDG_CONFIG_HOME or ~/.config (not Nanolight's own folder:
// the autostart entry lives under it).
func ConfigDir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(d) {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func appDir(env, fallback string) string {
	base := os.Getenv(env)
	if !filepath.IsAbs(base) {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, fallback)
	}
	d := filepath.Join(base, "Nanolight")
	os.MkdirAll(d, 0o755)
	return d
}
