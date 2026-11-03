package raycast

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rabden/ultra-light/internal/apps"
)

// Raycast Script Commands (github.com/raycast/script-commands), run by
// Nanolight itself, no Node: any script whose comments say what it is,
//
//	# @raycast.schemaVersion 1
//	# @raycast.title Generate UUID
//	# @raycast.mode silent
//	# @raycast.packageName Developer Utilities
//	# @raycast.argument1 { "type": "text", "placeholder": "Name" }
//
// dropped in <data dir>/script-commands (or a folder down).

const (
	// ScriptPrefix starts a script's row path: script:<file>.
	ScriptPrefix = "script:"
	// FolderPath is the row that opens the scripts' folder.
	FolderPath = "script-folder:"
)

type Mode int

const (
	FullOutput Mode = iota
	Silent
	Compact
	Inline
)

type Argument struct {
	// Type is "text", "password" or "dropdown".
	Type           string `json:"type"`
	Placeholder    string `json:"placeholder"`
	Optional       bool   `json:"optional"`
	PercentEncoded bool   `json:"percentEncoded"`
	Data           []struct {
		Title string `json:"title"`
		Value string `json:"value"`
	} `json:"data"`
}

// Script is a script command's header.
type Script struct {
	Title, Package, Icon, Description, Cwd string
	Mode                                   Mode
	Confirm                                bool
	Args                                   [3]*Argument
	schema                                 bool
}

// ArgCount is how many arguments it takes.
func (s *Script) ArgCount() int {
	n := 0
	for _, a := range s.Args {
		if a != nil {
			n++
		}
	}
	return n
}

// ParseScript reads the @raycast. keys in a script's comments (#, //, --,
// ;) in its first 200 lines; false when it isn't a script command (no
// schemaVersion or no title).
func ParseScript(src string) (Script, bool) {
	var s Script
	for n, raw := range strings.Split(src, "\n") {
		if n >= 200 {
			break
		}
		line := strings.Trim(raw, " \t\r")
		var body string
		found := false
		for _, c := range []string{"#", "//", "--", ";"} {
			if strings.HasPrefix(line, c) {
				body = strings.TrimLeft(line[len(c):], " \t#/-;")
				found = true
				break
			}
		}
		if !found {
			continue
		}
		at := strings.Index(body, "@raycast.")
		if at < 0 {
			continue
		}
		kv := body[at+len("@raycast."):]
		sp := strings.IndexAny(kv, " \t")
		if sp < 0 {
			sp = len(kv)
		}
		key, value := kv[:sp], strings.Trim(kv[sp:], " \t")
		switch {
		case key == "schemaVersion":
			s.schema = true
		case key == "title":
			s.Title = value
		case key == "packageName":
			s.Package = value
		case key == "icon":
			s.Icon = value
		case key == "description":
			s.Description = value
		case key == "currentDirectoryPath":
			s.Cwd = value
		case key == "needsConfirmation":
			s.Confirm = value == "true"
		case key == "mode":
			switch value {
			case "silent":
				s.Mode = Silent
			case "compact":
				s.Mode = Compact
			case "inline":
				s.Mode = Inline
			default:
				s.Mode = FullOutput
			}
		case len(key) == 9 && strings.HasPrefix(key, "argument") && key[8] >= '1' && key[8] <= '3':
			a := &Argument{Type: "text"}
			if json.Unmarshal([]byte(value), a) == nil {
				s.Args[key[8]-'1'] = a
			}
		}
	}
	if !s.schema || s.Title == "" {
		return s, false
	}
	return s, true
}

// ScriptsDir is <data dir>/script-commands, made if missing.
func ScriptsDir(data string) string {
	d := filepath.Join(data, "script-commands")
	os.MkdirAll(d, 0o755)
	return d
}

// ScanScripts lists the folder's row and each script command in the folder
// and a folder down; dotfiles are passed over, the header read from the
// first 8 KB.
func ScanScripts(data string) []apps.Entry {
	out := []apps.Entry{{Name: "Script Commands Folder", Folder: "Raycast", Path: FolderPath, Kind: apps.ScriptsFolder}}
	return scanDir(out, ScriptsDir(data), 1)
}

func scanDir(out []apps.Entry, dir string, depth int) []apps.Entry {
	list, _ := os.ReadDir(dir)
	for _, e := range list {
		if e.Name() == "" || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if depth > 0 {
				out = scanDir(out, full, depth-1)
			}
			continue
		}
		head := readHead(full, 8192)
		s, ok := ParseScript(head)
		if !ok {
			continue
		}
		folder := s.Package
		if folder == "" {
			folder = "Script Command"
		}
		out = append(out, apps.Entry{Name: s.Title, Folder: folder, Path: ScriptPrefix + full, Kind: apps.Script})
	}
	return out
}

func readHead(path string, n int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	b, _ := io.ReadAll(io.LimitReader(f, n))
	return string(b)
}

// Command is how to run file: its shebang's words (#!/usr/bin/env python3),
// else /bin/sh, then the file.
func Command(src, file string) []string {
	var argv []string
	if strings.HasPrefix(src, "#!") {
		first, _, _ := strings.Cut(src[2:], "\n")
		argv = strings.Fields(strings.Trim(first, " \t\r"))
		if len(argv) > 7 {
			argv = argv[:7]
		}
	}
	if len(argv) == 0 {
		argv = []string{"/bin/sh"}
	}
	return append(argv, file)
}

// Encode percent-encodes an argument (percentEncoded).
func Encode(v string) string {
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
