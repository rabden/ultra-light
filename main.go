// Nanolight for Linux: a launcher for apps, math, Google, Grok and
// Raycast extensions, drawn with mygo's native UI. A port of the Zig app in
// ../src; see internal/app for how it starts.
//
//	nanolight            start, or show the running copy's panel
//	nanolight --toggle   show or hide the running copy's panel (bind it to
//	                     a key where the desktop can't give one to apps)
//	nanolight --version  print the version
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/egoist/mygo"

	"github.com/rabden/ultra-light/internal/app"
)

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" {
			fmt.Printf("nanolight %s\n", app.Version())
			return
		}
	}
	mygo.App.SetName("Nanolight")
	// The Google page draws with WebKitGTK's DMA-BUF renderer where the
	// host can (Wayland, a Mesa GPU), as Epiphany does: its fallback breaks
	// up on hover on GNOME with AMD graphics.
	mygo.App.SetDMABUFRenderer(mygo.DMABUFRendererAuto)
	if !mygo.App.RequestSingleInstanceLock() {
		return // the running copy shows or hides its panel (OnSecondInstance)
	}
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
