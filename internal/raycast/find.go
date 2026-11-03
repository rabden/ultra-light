package raycast

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/rabden/ultra-light/internal/apps"
)

const (
	// Prefix starts an installed command's row path: raycast:<dir>#<cmd>.
	Prefix = "raycast:"
	// StorePath is "Install Raycast Extension…"'s.
	StorePath = "raycast-store:"
)

// Dir is <data dir>/extensions, made if missing.
func Dir(data string) string {
	d := filepath.Join(data, "extensions")
	os.MkdirAll(d, 0o755)
	return d
}

type pkg struct {
	Name     string `json:"name"`
	Title    string `json:"title"`
	Icon     string `json:"icon"`
	Commands []struct {
		Name  string `json:"name"`
		Title string `json:"title"`
	} `json:"commands"`
}

func readPkg(dir string) (pkg, bool) {
	var p pkg
	b, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil || json.Unmarshal(b, &p) != nil {
		return p, false
	}
	return p, true
}

// Scan lists the store's row and each installed command: its title (or
// name), its folder the extension's title (or name), its icon the
// extension's (assets/<icon>).
func Scan(data string) []apps.Entry {
	out := []apps.Entry{{Name: "Install Raycast Extension…", Folder: "Raycast", Path: StorePath, Kind: apps.RaycastStore}}
	root := Dir(data)
	list, _ := os.ReadDir(root)
	for _, e := range list {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, e.Name())
		p, ok := readPkg(dir)
		if !ok {
			continue
		}
		folder := p.Title
		if folder == "" {
			folder = p.Name
		}
		icon := ""
		if p.Icon != "" {
			icon = filepath.Join(dir, "assets", p.Icon)
		}
		for _, c := range p.Commands {
			name := c.Title
			if name == "" {
				name = c.Name
			}
			out = append(out, apps.Entry{Name: name, Folder: folder, Path: Prefix + dir + "#" + c.Name, Kind: apps.RaycastCmd, Icon: icon})
		}
	}
	return out
}

// Split is a command row's extension directory and command name.
func Split(path string) (dir, cmd string, ok bool) {
	rest, found := strings.CutPrefix(path, Prefix)
	if !found {
		return "", "", false
	}
	h := strings.LastIndexByte(rest, '#')
	if h < 0 {
		return "", "", false
	}
	return rest[:h], rest[h+1:], true
}

// MinNode is the oldest Node the host runs on (raycast/package.json).
const MinNode = 20

// FindNode finds Node where it usually is: $NANOLIGHT_NODE, the PATH,
// /usr/local/bin, /usr/bin, Volta, Bun, the newest nvm version (an app
// started from the desktop gets a short PATH).
func FindNode(getenv func(string) string) string {
	if p := getenv("NANOLIGHT_NODE"); p != "" {
		return p
	}
	for _, d := range filepath.SplitList(getenv("PATH")) {
		if p := filepath.Join(d, "node"); isFile(p) {
			return p
		}
	}
	home := getenv("HOME")
	for _, p := range []string{"/usr/local/bin/node", "/usr/bin/node", filepath.Join(home, ".volta", "bin", "node"), filepath.Join(home, ".bun", "bin", "node")} {
		if isFile(p) {
			return p
		}
	}
	nvm := filepath.Join(home, ".nvm", "versions", "node")
	list, _ := os.ReadDir(nvm)
	var names []string
	for _, e := range list {
		names = append(names, e.Name())
	}
	sort.Slice(names, func(i, j int) bool { return versionLess(names[j], names[i]) })
	for _, n := range names {
		if p := filepath.Join(nvm, n, "bin", "node"); isFile(p) {
			return p
		}
	}
	return ""
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

var versionRE = regexp.MustCompile(`(\d+)(?:\.(\d+))?(?:\.(\d+))?`)

func versionParts(s string) [3]int {
	var v [3]int
	m := versionRE.FindStringSubmatch(s)
	for i := 1; i < len(m) && i <= 3; i++ {
		v[i-1], _ = strconv.Atoi(m[i])
	}
	return v
}

// versionLess orders "v18.2.0" before "v20.11.1".
func versionLess(a, b string) bool {
	va, vb := versionParts(a), versionParts(b)
	for i := range va {
		if va[i] != vb[i] {
			return va[i] < vb[i]
		}
	}
	return a < b
}

// NodeMajor runs node --version and returns its major version, 0 if it
// doesn't run.
func NodeMajor(node string) int {
	out, err := exec.Command(node, "--version").Output()
	if err != nil {
		return 0
	}
	return versionParts(strings.TrimSpace(string(out)))[0]
}

// WriteHost puts the built-in host script in the cache, once a version
// (named by its hash, so an update never runs an older one), and returns
// its path; $NANOLIGHT_RAYCAST_HOST wins.
func WriteHost(cache string, js []byte, getenv func(string) string) (string, error) {
	if p := getenv("NANOLIGHT_RAYCAST_HOST"); p != "" {
		return p, nil
	}
	h := fnv.New64a()
	h.Write(js)
	p := filepath.Join(cache, fmt.Sprintf("raycast-host-%x.js", h.Sum64()))
	if isFile(p) {
		return p, nil
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return "", err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, js, 0o644); err != nil {
		return "", err
	}
	return p, os.Rename(tmp, p)
}
