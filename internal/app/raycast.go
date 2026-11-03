package app

import (
	"bufio"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/rabden/ultra-light/internal/apps"
	"github.com/rabden/ultra-light/internal/raycast"
	"github.com/rabden/ultra-light/internal/store"
	"github.com/rabden/ultra-light/resources"
)

// Raycast extensions run in the panel: `node host.js run <dir> <cmd>` (or
// `store <extensions dir>`), whose JSON lines draw the command's screen
// until it ends or Escape leaves it (raycast.zig). Script commands run
// without Node (script.go). One of the two runs at a time.

// startRaycast finds Node (20 or later) and writes the host where it can
// run it; the rows follow.
func (a *App) startRaycast() {
	go func() {
		node := raycast.FindNode(os.Getenv)
		why := ""
		host := ""
		switch {
		case node == "":
			why = "Raycast extensions need Node.js: install it from nodejs.org."
		case raycast.NodeMajor(node) < raycast.MinNode:
			why = "Raycast extensions need Node.js 20 or later: install it from nodejs.org."
			node = ""
		default:
			var err error
			if host, err = raycast.WriteHost(store.CacheDir(), resources.RaycastHost, os.Getenv); err != nil {
				log.Printf("nanolight: writing the Raycast host: %v", err)
				why = "The Raycast host couldn't be written to Nanolight's cache."
				host = ""
			}
		}
		a.win.Update(func() {
			a.ext.node, a.ext.host, a.ext.why = node, host, why
			a.rebuildIndex()
		})
	}()
}

// raycastRows: installed commands and the store's row only where Node
// runs them; script commands everywhere.
func (a *App) raycastRows() []apps.Entry {
	var rows []apps.Entry
	if a.ext.node != "" {
		rows = append(rows, raycast.Scan(store.DataDir())...)
	}
	return append(rows, raycast.ScanScripts(store.DataDir())...)
}

// openRaycast runs a command's row in the panel, which stays up; the
// scripts' folder opens in the file manager.
func (a *App) openRaycast(e apps.Entry) bool {
	if e.Kind == apps.ScriptsFolder {
		if err := mygo.Shell.OpenPath(raycast.ScriptsDir(store.DataDir())); err != nil {
			log.Printf("nanolight: opening the scripts' folder: %v", err)
		}
		return false
	}
	icon := a.Icon(e)
	if e.Kind == apps.Script {
		a.ext.stop()
		a.script.start(e.Path, e.Name, icon)
	} else {
		a.script.stop()
		a.ext.start(e.Path, e.Name, icon)
	}
	return true
}

// extRunner runs the Node host for one command at a time.
type extRunner struct {
	a          *App
	node, host string
	why        string // why there is no Node, or ""
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	gen        int
	shown      bool // the screen is this runner's
}

func (r *extRunner) start(path, title string, icon *ui.Bitmap) {
	r.stop()
	ext := &r.a.panel.Ext
	if r.node == "" || r.host == "" {
		why := r.why
		if why == "" {
			why = "Raycast extensions need Node.js: install it from nodejs.org."
		}
		r.fail(title, why)
		return
	}
	args := []string{"--no-deprecation", r.host}
	if dir, cmd, ok := raycast.Split(path); ok {
		args = append(args, "run", dir, cmd)
	} else {
		args = append(args, "store", raycast.Dir(store.DataDir()))
	}
	ext.Start(title, icon, r.send)
	r.shown = true
	cmd := exec.Command(r.node, args...)
	cmd.Stderr = os.Stderr
	in, err1 := cmd.StdinPipe()
	out, err2 := cmd.StdoutPipe()
	if err1 != nil || err2 != nil || cmd.Start() != nil {
		r.fail(title, "Node.js couldn't be started.")
		return
	}
	r.cmd, r.stdin = cmd, in
	r.gen++
	gen := r.gen
	go func() {
		sc := bufio.NewScanner(out)
		sc.Buffer(make([]byte, 64<<10), 16<<20)
		for sc.Scan() {
			line := append([]byte(nil), sc.Bytes()...)
			r.a.win.Update(func() {
				if gen == r.gen {
					r.handle(line)
				}
			})
		}
		cmd.Wait()
		r.a.win.Update(func() {
			if gen == r.gen {
				r.stop()
			}
		})
	}()
	r.a.win.Invalidate()
}

// fail shows why a command can't run, in its screen and its footer.
func (r *extRunner) fail(title, why string) {
	ext := &r.a.panel.Ext
	ext.Start(title, nil, nil)
	r.shown = true
	ext.Note(why, true)
	ext.Show(&raycast.View{Kind: "detail", Title: title, Markdown: &why})
	r.a.win.Invalidate()
}

// stop ends the command running, if one is, and leaves its screen.
func (r *extRunner) stop() {
	r.gen++
	if r.stdin != nil {
		r.stdin.Close()
		r.stdin = nil
	}
	if r.cmd != nil && r.cmd.Process != nil {
		r.cmd.Process.Kill()
	}
	r.cmd = nil
	if ext := &r.a.panel.Ext; ext.Open && r.shown {
		ext.Stop()
		r.a.win.Invalidate()
	}
	r.shown = false
}

func (r *extRunner) send(line []byte) {
	if r.stdin == nil {
		return
	}
	if _, err := r.stdin.Write(line); err != nil {
		r.stop()
	}
}

func (r *extRunner) handle(line []byte) {
	m, err := raycast.Parse(line)
	if err != nil {
		return
	}
	a := r.a
	ext := &a.panel.Ext
	switch m.Type {
	case "render":
		if m.View != nil {
			ext.Show(m.View)
		}
	case "toast", "error":
		t := deref(m.Title)
		if t == "" {
			t = deref(m.Message)
		}
		if m.Title != nil && deref(m.Message) != "" {
			t += ": " + *m.Message
		}
		ext.Note(t, m.Type == "error" || m.Style == "failure")
	case "hud":
		ext.Note(deref(m.Title), false)
		a.win.Hide()
	case "open":
		if strings.HasPrefix(m.Target, "https://") || strings.HasPrefix(m.Target, "http://") {
			a.OpenURL(m.Target)
		} else if m.Target != "" {
			if err := mygo.Shell.OpenPath(m.Target); err != nil {
				log.Printf("nanolight: opening %s: %v", m.Target, err)
			}
		}
	case "copy", "paste":
		if m.Text != nil {
			a.Copy(*m.Text)
		}
		ext.Note("Copied to the clipboard", false)
		if m.Type == "paste" {
			a.win.Hide() // no keystroke into the other app (decision 10)
		}
	case "close":
		a.win.Hide()
	case "installed":
		a.rebuildIndex()
	case "pop_to_root", "exit":
		r.stop()
	case "request":
		switch m.Method {
		case "confirm":
			// A confirmation is yes: the person chose the action that asks.
			r.send(raycast.Reply(m.ID, true))
		case "clipboard.readText":
			r.send(raycast.Reply(m.ID, mygo.Clipboard.ReadText()))
		default:
			r.send(raycast.Reply(m.ID, nil))
		}
	}
	a.win.Invalidate()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
