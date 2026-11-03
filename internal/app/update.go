package app

import (
	"context"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"

	"github.com/rabden/ultra-light/internal/store"
)

// Updates (update_pc.zig): signed releases of rabden/ultra-light tagged
// v…, which mygo's updater checks 20 s after launch and every 6 hours
// and installs as it finds them. A launcher almost never quits, so once one
// is in and the panel has been away 10 s, unseen, Nanolight restarts into
// it; the new copy starts quietly and says "Updated to x" the next time the
// panel opens. Builds without updates (go run, the .deb) leave it off.

const (
	firstCheck  = 20 * time.Second
	checkEvery  = 6 * time.Hour
	restartIdle = 10 * time.Second
)

type updates struct {
	checking bool
	ready    string // the version installed, waiting for a restart
	waits    int
}

func (a *App) updatesOn() bool { return mygo.Updater.Enabled() }

// updateDir replaces the releases with a directory in test builds
// (-ldflags -X).
var updateDir string

// dirSource serves what `mygo build` wrote, from a directory.
type dirSource string

func (d dirSource) Manifest(context.Context, bool) (io.ReadCloser, error) {
	return os.Open(filepath.Join(string(d), "update-linux-amd64.json"))
}

func (d dirSource) Open(_ context.Context, url string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(string(d), path.Base(url)))
}

func (a *App) startUpdates() {
	if updateDir != "" {
		mygo.Updater.SetSource(dirSource(updateDir))
	}
	if !a.updatesOn() {
		return
	}
	var tick func()
	tick = func() {
		a.win.Update(func() {
			a.checkUpdates()
			time.AfterFunc(checkEvery, tick)
		})
	}
	time.AfterFunc(firstCheck, tick)
}

// checkUpdates asks for a newer version and installs it, on a goroutine;
// one installed already restarts into it (the button's second meaning).
func (a *App) checkUpdates() {
	if !a.updatesOn() || a.up.checking {
		return
	}
	if a.up.ready != "" {
		a.restartToUpdate()
		return
	}
	a.up.checking = true
	go func() {
		v := ""
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		u, err := mygo.Updater.Check(ctx)
		if err == nil && u != nil {
			if err = u.Install(ctx, nil); err == nil {
				v = u.Version
			}
		}
		if err != nil {
			log.Printf("nanolight: updates: %v", err)
		}
		a.win.Update(func() {
			a.up.checking = false
			if v == "" {
				return
			}
			a.up.ready = v
			a.settingsChanged()
			a.updateAway()
		})
	}()
}

func (a *App) updateReady() string { return a.up.ready }

// updateAway: the panel went away, or an update is in; 10 s later, still
// away, Nanolight restarts into it. Each hide starts the wait again.
func (a *App) updateAway() {
	if a.up.ready == "" {
		return
	}
	a.up.waits++
	time.AfterFunc(restartIdle, func() {
		a.win.Update(func() {
			a.up.waits--
			if a.up.waits == 0 && !a.win.IsVisible() {
				a.restartToUpdate()
			}
		})
	})
}

// restartToUpdate starts the version installed, the panel saying so the
// next time it opens.
func (a *App) restartToUpdate() {
	if a.up.ready == "" {
		return
	}
	a.kv.Set(store.KeyUpdated, a.up.ready)
	mygo.App.Relaunch()
}
