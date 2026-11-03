package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/rabden/ultra-light/internal/raycast"
)

// A Raycast script command, run by Nanolight itself (script_run.zig): its
// arguments asked for first (a form in the panel), then the script started
// with them, what it prints read as it comes and shown by its mode: silent
// hides the panel and says the last line in the search's footer, compact
// and inline show the last line, fullOutput everything, live.

const maxScriptOutput = 256 << 10

type scriptRunner struct {
	a     *App
	file  string
	title string
	src   string
	meta  raycast.Script
	args  [3]string
	cmd   *exec.Cmd
	out   []byte
	shown bool
	gen   int
}

func (r *scriptRunner) start(path, title string, icon *ui.Bitmap) {
	r.stop()
	r.file = strings.TrimPrefix(path, raycast.ScriptPrefix)
	r.title = title
	b, _ := os.ReadFile(r.file)
	r.src = string(b)
	if m, ok := raycast.ParseScript(r.src); ok {
		r.meta = m
	} else {
		r.meta = raycast.Script{Title: title}
	}
	r.args = [3]string{}
	r.a.panel.Ext.Start(title, icon, r.said)
	r.shown = true
	if r.meta.ArgCount() > 0 || r.meta.Confirm {
		r.ask()
		return
	}
	r.run()
}

var runAction = []raycast.Action{{ID: "run", Title: "Run"}}

// ask shows the arguments' form, or with none "Run X?"; Run runs.
func (r *scriptRunner) ask() {
	var fields []raycast.Field
	for i, arg := range r.meta.Args {
		if arg == nil {
			continue
		}
		var opts []raycast.Option
		for _, d := range arg.Data {
			opts = append(opts, raycast.Option{Value: d.Value, Title: d.Title})
		}
		dropdown := arg.Type == "dropdown"
		// A dropdown starts on its first choice.
		if dropdown && len(arg.Data) > 0 && r.args[i] == "" {
			r.args[i] = arg.Data[0].Value
		}
		kind := "text"
		switch {
		case dropdown:
			kind = "dropdown"
		case arg.Type == "password":
			kind = "password"
		}
		title := arg.Placeholder
		if title == "" {
			title = "Argument"
		}
		ph := ""
		if arg.Optional {
			ph = "Optional"
		}
		fields = append(fields, raycast.Field{ID: fmt.Sprintf("arg%d", i+1), Type: kind, Title: title, Placeholder: ph, Value: r.args[i], Options: opts})
	}
	var v *raycast.View
	if len(fields) > 0 {
		v = &raycast.View{Kind: "form", Title: r.title, Fields: fields, Actions: runAction}
	} else {
		md := fmt.Sprintf("# Run %s?\n%s", r.title, r.meta.Description)
		v = &raycast.View{Kind: "detail", Title: r.title, Markdown: &md, Actions: runAction}
	}
	r.a.panel.Ext.Show(v)
	r.a.win.Invalidate()
}

// run starts the script with its arguments, its errors joined to its
// output through sh.
func (r *scriptRunner) run() {
	r.stop()
	r.out = nil
	argv := append([]string{"-c", `exec "$@" 2>&1`, "sh"}, raycast.Command(r.src, r.file)...)
	for i, arg := range r.meta.Args {
		if arg == nil {
			continue
		}
		v := r.args[i]
		if v == "" && !arg.Optional {
			r.fail("Fill in every argument that isn't optional.")
			return
		}
		if arg.PercentEncoded {
			v = raycast.Encode(v)
		}
		argv = append(argv, v)
	}
	cmd := exec.Command("/bin/sh", argv...)
	cmd.Dir = r.workDir()
	cmd.Stderr = os.Stderr
	out, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		r.fail("The script couldn't be started (is its interpreter installed?).")
		return
	}
	r.cmd = cmd
	r.gen++
	gen := r.gen
	go func() {
		buf := make([]byte, 16<<10)
		for {
			n, err := out.Read(buf)
			if n > 0 {
				chunk := append([]byte(nil), buf[:n]...)
				r.a.win.Update(func() {
					if gen == r.gen {
						r.feed(chunk)
					}
				})
			}
			if err != nil {
				break
			}
		}
		werr := cmd.Wait()
		r.a.win.Update(func() {
			if gen == r.gen {
				r.ended(werr)
			}
		})
	}()
	if r.meta.Mode == raycast.Silent {
		r.shown = false
		r.a.panel.Ext.Stop()
		r.a.win.Hide()
		return
	}
	r.showOutput(false)
}

// workDir is currentDirectoryPath (with ~), else the script's folder.
func (r *scriptRunner) workDir() string {
	d := r.meta.Cwd
	if d == "" {
		return filepath.Dir(r.file)
	}
	if rest, ok := strings.CutPrefix(d, "~"); ok {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, rest)
	}
	return d
}

func (r *scriptRunner) fail(why string) {
	ext := &r.a.panel.Ext
	// Said in the panel (and the footer), not left at "Starting…".
	if ext.View() == nil {
		md := "# " + r.title + "\n" + why
		ext.Show(&raycast.View{Kind: "detail", Title: r.title, Markdown: &md})
	}
	ext.Note(why, true)
	r.a.win.Invalidate()
}

func (r *scriptRunner) text() string { return strings.TrimRight(string(r.out), " \r\n") }

func lastLine(s string) string {
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}

var doneActions = []raycast.Action{{ID: "copy", Title: "Copy Output"}, {ID: "again", Title: "Run Again"}}

// showOutput shows what it printed so far, as its mode does.
func (r *scriptRunner) showOutput(done bool) {
	if !r.shown {
		return
	}
	v := &raycast.View{Kind: "detail", Title: r.title, Loading: !done}
	if done {
		v.Actions = doneActions
	}
	text := r.text()
	if r.meta.Mode == raycast.FullOutput {
		if text == "" && done {
			text = "(no output)"
		}
		v.Output = &text
	} else {
		last := lastLine(text)
		switch {
		case last != "":
		case done:
			last = "Done"
		default:
			last = "Running…"
		}
		md := "# " + last
		v.Markdown = &md
	}
	r.a.panel.Ext.Show(v)
	r.a.win.Invalidate()
}

func (r *scriptRunner) feed(b []byte) {
	room := maxScriptOutput - len(r.out)
	r.out = append(r.out, b[:max(0, min(len(b), room))]...)
	if r.meta.Mode == raycast.FullOutput {
		r.showOutput(false)
	}
}

func (r *scriptRunner) ended(err error) {
	r.cmd = nil
	last := lastLine(r.text())
	if !r.shown {
		// Silent: the last line, said in the search's footer.
		switch {
		case last != "":
		case err == nil:
			last = r.title
		default:
			last = "Failed"
		}
		r.a.panel.Note = last
		r.a.win.Invalidate()
		return
	}
	r.showOutput(true)
	if err != nil {
		why := "Failed"
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if ee.Exited() {
				why = fmt.Sprintf("Exited with code %d", ee.ExitCode())
			} else {
				why = "Stopped"
			}
		}
		r.a.panel.Ext.Note(why, true)
	}
}

// said is what the screen tells the script: a field's value, an action, a
// pop.
func (r *scriptRunner) said(line []byte) {
	m, err := raycast.Parse(line)
	if err != nil {
		return
	}
	id := strings.Trim(string(m.ID), `"`)
	switch m.Type {
	case "field":
		if len(id) == 4 && id[3] >= '1' && id[3] <= '3' {
			i := id[3] - '1'
			s, _ := m.Value.(string)
			r.args[i] = s
			// A dropdown shows what was picked (text fields keep their own).
			if a := r.meta.Args[i]; a != nil && a.Type == "dropdown" {
				r.ask()
			}
		}
	case "action":
		switch id {
		case "run":
			r.run()
		case "again":
			if r.meta.ArgCount() > 0 || r.meta.Confirm {
				r.ask()
				return
			}
			r.run()
		case "copy":
			r.a.Copy(strings.TrimRight(string(r.out), "\n"))
			r.a.panel.Ext.Note("Copied to the clipboard", false)
		}
	case "pop":
		r.stop()
		r.shown = false
		r.a.panel.Ext.Stop()
	}
	r.a.win.Invalidate()
}

// stop ends a script still running.
func (r *scriptRunner) stop() {
	r.gen++
	if r.cmd != nil && r.cmd.Process != nil {
		r.cmd.Process.Kill()
	}
	r.cmd = nil
}
