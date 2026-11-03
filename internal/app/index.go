package app

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/rabden/ultra-light/internal/apps"
	"github.com/rabden/ultra-light/internal/store"
)

// scan lists the installed apps on a goroutine (GIO, as the desktop's menu
// does) and rebuilds the index with Raycast's rows.
func (a *App) scan() {
	if a.scanning {
		return
	}
	a.scanning = true
	a.scanAt = time.Now()
	go func() {
		list, err := mygo.Shell.InstalledApps()
		if err != nil {
			log.Printf("nanolight: listing apps: %v", err)
		}
		a.win.Update(func() {
			a.scanning = false
			if err == nil {
				a.installed = list
			}
			a.rebuildIndex()
		})
	}()
}

// maybeRescan looks again on show when the last look is older than 60 s,
// so new apps appear without a restart.
func (a *App) maybeRescan() {
	if time.Since(a.scanAt) > 60*time.Second {
		a.scan()
	}
}

// rebuildIndex makes the index from the apps and Raycast's rows: each name
// once, sorted, with the launch counts kept.
func (a *App) rebuildIndex() {
	ix := a.index
	ix.Entries, ix.Names, ix.Launches = nil, nil, nil
	for _, app := range a.installed {
		ix.Add(apps.Entry{Name: app.Name, Folder: "Application", Path: app.Path, ID: app.ID, Kind: apps.App, Ref: app})
	}
	a.extra = a.raycastRows()
	for _, e := range a.extra {
		ix.Add(e)
	}
	ix.Sort()
	ix.LoadLaunches(must(a.kv.Get(store.KeyLaunches)))
	a.panel.Refresh()
	a.win.Invalidate()
}

// isSelf: Nanolight's own desktop entry re-shows the panel rather than
// starting a copy.
func isSelf(e apps.Entry) bool {
	id := strings.TrimSuffix(strings.ToLower(e.ID), ".desktop")
	return id == "nanolight" || strings.HasSuffix(id, ".nanolight")
}

// Open opens an entry: an app is launched (the panel hides first, so the
// app the user was in keeps the keys until the new one takes them).
func (a *App) Open(e apps.Entry) bool {
	switch e.Kind {
	case apps.App:
		if isSelf(e) {
			a.panel.Reveal()
			a.keepOpen = true
			return true
		}
		a.win.Hide()
		app, _ := e.Ref.(mygo.InstalledApp)
		if err := mygo.Shell.LaunchApp(app); err != nil {
			log.Printf("nanolight: launching %s: %v", e.Name, err)
		}
		return false
	}
	return a.openRaycast(e)
}

// Icon draws an entry's picture the first time its row shows, for the
// sharpest display, and keeps it (nil included).
func (a *App) Icon(e apps.Entry) *ui.Bitmap {
	key := e.Path
	if e.Icon != "" {
		key = e.Icon
	}
	if b, ok := a.icons[key]; ok {
		return b
	}
	var b *ui.Bitmap
	switch {
	case e.Icon != "":
		if data, err := os.ReadFile(e.Icon); err == nil {
			b, _ = ui.DecodeBitmap(data)
		}
	case e.Kind == apps.App:
		scale := 1.0
		for _, d := range mygo.Screen.Displays() {
			scale = max(scale, d.ScaleFactor)
		}
		if data, err := mygo.Shell.FileIcon(e.Path, int(26*scale+0.5)); err == nil {
			b, _ = ui.DecodeBitmap(data)
		}
	}
	a.icons[key] = b
	return b
}
