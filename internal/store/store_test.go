package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	s.Set("hotkey", "ctrl_space")
	s.Set("quote", "a \"b\"\n\tc")
	s.Set("gone", "x")
	s.Remove("gone")
	s = Open(dir)
	if v, _ := s.Get("hotkey"); v != "ctrl_space" {
		t.Errorf("hotkey %q", v)
	}
	if v, _ := s.Get("quote"); v != "a \"b\"\n\tc" {
		t.Errorf("quote %q", v)
	}
	if s.Has("gone") {
		t.Error("gone kept")
	}
	os.WriteFile(filepath.Join(dir, "kv.json"), []byte("{not json"), 0o644)
	if s = Open(dir); len(s.m) != 0 {
		t.Error("broken file must start empty")
	}
}

// A kv.json the Zig build wrote loads unchanged.
func TestZigFile(t *testing.T) {
	s := Open("testdata")
	for k, want := range map[string]string{
		KeyHotkey: "opt_space", KeyOnboarded: "1", KeySuggest: "0",
		KeyLaunches: "Firefox\t3\nFiles\t1\n", KeyUpdated: "0.1.3",
	} {
		if v, _ := s.Get(k); v != want {
			t.Errorf("%s = %q, want %q", k, v, want)
		}
	}
	if s.Bool(KeySuggest, true) || s.Bool(KeySearchesOut, false) || !s.Bool(KeyOnboarded, false) {
		t.Error("bools")
	}
}
