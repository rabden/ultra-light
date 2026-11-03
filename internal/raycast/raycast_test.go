package raycast

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rabden/ultra-light/internal/apps"
)

func TestParseRender(t *testing.T) {
	m, err := Parse([]byte(`{"type":"render","view":{"kind":"list","loading":false,"sections":[{"title":"","items":[{"id":"a","title":"Story","subtitle":"pg","icon":{"text":"🔥"},"accessories":["3","0"],"actions":[{"id":"o","title":"Open in Browser","shortcut":null},{"id":"c","title":"Copy Link","shortcut":"cmd+."}]}]}],"extra":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	v := m.View
	if !v.IsList() || v.Count() != 1 || v.ActionsAt(0)[1].Title != "Copy Link" || *v.Item(0).Icon.Text != "🔥" {
		t.Fatalf("%+v", v)
	}
	if got := Plain("# Hello **world**\n![x](y.png)See [docs](https://a.b) and `code`."); got != "Hello world\nSee docs and code." {
		t.Errorf("plain %q", got)
	}
	if got := string(Say("search", "text", "hn")); got != "{\"type\":\"search\",\"text\":\"hn\"}\n" {
		t.Errorf("say %q", got)
	}
	if got := string(Say("pop", "", nil)); got != "{\"type\":\"pop\"}\n" {
		t.Errorf("say %q", got)
	}
	if got := string(FieldValue("arg1", "Rach")); got != `{"type":"field","id":"arg1","value":"Rach"}`+"\n" {
		t.Errorf("field %q", got)
	}
	r, _ := Parse([]byte(`{"type":"request","id":7,"method":"confirm"}`))
	if got := string(Reply(r.ID, true)); got != `{"type":"reply","id":7,"result":true}`+"\n" {
		t.Errorf("reply %q", got)
	}
}

func TestSplit(t *testing.T) {
	dir, cmd, ok := Split("raycast:/home/me/.local/share/Nanolight/extensions/hacker-news#frontpage")
	if !ok || dir != "/home/me/.local/share/Nanolight/extensions/hacker-news" || cmd != "frontpage" {
		t.Fatal(dir, cmd)
	}
	if _, _, ok := Split("/usr/share/applications/x.desktop"); ok {
		t.Fatal("an app is no command")
	}
}

const wiki = `#!/usr/bin/env bash
# Required parameters:
# @raycast.schemaVersion 1
# @raycast.title Search Wikipedia
# @raycast.mode silent
# @raycast.packageName Web Searches
# @raycast.icon 📖
# @raycast.argument1 { "type": "text", "placeholder": "Query", "percentEncoded": true }
# @raycast.argument2 { "type": "dropdown", "placeholder": "Lang", "data": [{"title": "English", "value": "en"}] }
open "https://$2.wikipedia.org/w/index.php?search=$1"
`

func TestScriptHeader(t *testing.T) {
	s, ok := ParseScript(wiki)
	if !ok || s.Title != "Search Wikipedia" || s.Mode != Silent || s.Package != "Web Searches" || s.ArgCount() != 2 ||
		!s.Args[0].PercentEncoded || s.Args[1].Data[0].Value != "en" {
		t.Fatalf("%+v", s)
	}
	if _, ok := ParseScript("#!/bin/sh\necho hi\n"); ok {
		t.Fatal("not a script command")
	}
	if s, _ := ParseScript("-- @raycast.schemaVersion 1\n-- @raycast.title A\n"); s.Title != "A" {
		t.Fatal("AppleScript comments")
	}
	if s, _ := ParseScript("// @raycast.schemaVersion 1\n// @raycast.title B\n"); s.Title != "B" {
		t.Fatal("JavaScript comments")
	}
	argv := Command(wiki, "/s/wiki.sh")
	if argv[0] != "/usr/bin/env" || argv[1] != "bash" || argv[2] != "/s/wiki.sh" {
		t.Fatal(argv)
	}
	if Command("echo", "x")[0] != "/bin/sh" {
		t.Fatal("sh")
	}
	if Encode("a b&c") != "a%20b%26c" {
		t.Fatal(Encode("a b&c"))
	}
}

func TestScan(t *testing.T) {
	data := t.TempDir()
	ext := filepath.Join(Dir(data), "hacker-news")
	os.MkdirAll(ext, 0o755)
	os.WriteFile(filepath.Join(ext, "package.json"), []byte(`{"name":"hacker-news","title":"Hacker News","icon":"icon.png","commands":[{"name":"frontpage","title":"Front Page"},{"name":"best"}]}`), 0o644)
	rows := Scan(data)
	if len(rows) != 3 || rows[0].Kind != apps.RaycastStore || rows[1].Name != "Front Page" || rows[1].Folder != "Hacker News" ||
		rows[2].Name != "best" || rows[1].Path != "raycast:"+ext+"#frontpage" || rows[1].Icon != filepath.Join(ext, "assets", "icon.png") {
		t.Fatalf("%+v", rows)
	}
	sd := ScriptsDir(data)
	os.MkdirAll(filepath.Join(sd, "web"), 0o755)
	os.WriteFile(filepath.Join(sd, "web", "wiki.sh"), []byte(wiki), 0o755)
	os.WriteFile(filepath.Join(sd, ".hidden.sh"), []byte(wiki), 0o755)
	os.WriteFile(filepath.Join(sd, "plain.sh"), []byte("echo hi\n"), 0o755)
	srows := ScanScripts(data)
	if len(srows) != 2 || srows[0].Kind != apps.ScriptsFolder || srows[1].Name != "Search Wikipedia" || srows[1].Folder != "Web Searches" {
		t.Fatalf("%+v", srows)
	}
}

func TestFindNodeAndHost(t *testing.T) {
	home := t.TempDir()
	for _, v := range []string{"v18.20.1", "v20.11.0", "v9.0.0"} {
		os.MkdirAll(filepath.Join(home, ".nvm", "versions", "node", v, "bin"), 0o755)
		os.WriteFile(filepath.Join(home, ".nvm", "versions", "node", v, "bin", "node"), nil, 0o755)
	}
	env := map[string]string{"HOME": home, "PATH": t.TempDir()}
	got := FindNode(func(k string) string { return env[k] })
	if _, err := os.Stat("/usr/local/bin/node"); err == nil {
		t.Skip("a node in /usr/local/bin comes first")
	}
	if _, err := os.Stat("/usr/bin/node"); err == nil {
		t.Skip("a node in /usr/bin comes first")
	}
	if got != filepath.Join(home, ".nvm", "versions", "node", "v20.11.0", "bin", "node") {
		t.Fatal(got)
	}
	cache := t.TempDir()
	p, err := WriteHost(cache, []byte("host"), func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "host" {
		t.Fatal("host written")
	}
}
