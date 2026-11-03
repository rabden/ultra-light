package app

import (
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The results page: Google's page in the panel, in a WebKitGTK web view
// made on first use and kept; stopped and emptied on hide, and let go
// after the panel has been hidden 5 minutes (its web process holds 100-200
// MB). Without WebKitGTK, searches go to the browser.

// userAgent is a desktop Safari's, as the Mac's WKWebView tells Google, so
// it serves its full page.
const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15"

const pageRelease = 5 * time.Minute

type pageState struct {
	view    *mygo.WebView
	failed  bool
	release *time.Timer
	app     *App
}

// ShowPage loads url in the panel's web view.
func (a *App) ShowPage(url string) bool {
	p := &a.page
	p.app = a
	if p.failed {
		return false
	}
	if p.view == nil || p.view.IsDestroyed() {
		v, err := a.win.NewWebView(mygo.WebViewOptions{
			Page: mygo.PageOptions{
				Untrusted:       true, // Google's page never reaches the app's Go
				UserAgent:       userAgent,
				SwipeNavigation: true,
				Session:         mygo.NewSession("results"), // consent and sign-in kept
			},
			BackgroundColor: "transparent",
			Transparent:     true,
		})
		if err != nil {
			log.Printf("nanolight: no results page in the panel: %v", err)
			p.failed = true
			a.panel.SetPageEnabled(false)
			a.panel.Note = "Install WebKitGTK to see results in the panel"
			a.settingsChanged()
			return false
		}
		page := v.Page()
		page.SetWindowOpenHandler(func(r mygo.WindowOpenRequest) *mygo.WindowOptions {
			page.LoadURL(r.URL) // a new window's link loads in place
			return nil
		})
		loaded := func() {
			if !a.panel.PageLoaded {
				a.panel.PageLoaded = true
				a.win.Invalidate()
			}
		}
		page.OnDidNavigate(func(string) { loaded() })
		page.OnDidFinishLoad(loaded)
		p.view = v
	}
	p.cancelRelease()
	if err := p.view.Page().LoadURL(url); err != nil {
		log.Printf("nanolight: loading %s: %v", url, err)
	}
	return true
}

// HidePage stops the page and empties it: no sound left playing, no old
// page under the next one.
func (a *App) HidePage() {
	if v := a.page.view; v != nil && !v.IsDestroyed() {
		v.Page().Stop()
		v.Page().LoadHTML("", "")
	}
}

// Page is the web view the panel shows, or nil.
func (a *App) Page() ui.NativeWebView {
	if v := a.page.view; v != nil && !v.IsDestroyed() {
		return v
	}
	return nil
}

// PageURL is the address the page is at now.
func (a *App) PageURL() string {
	if v := a.page.view; v != nil && !v.IsDestroyed() {
		return v.Page().URL()
	}
	return ""
}

// PageBack goes back a page.
func (a *App) PageBack() {
	if v := a.page.view; v != nil && !v.IsDestroyed() && v.Page().CanGoBack() {
		v.Page().GoBack()
	}
}

func (p *pageState) releaseLater() {
	if p.view == nil {
		return
	}
	p.cancelRelease()
	p.release = time.AfterFunc(pageRelease, func() {
		p.app.win.Update(func() {
			if p.view != nil && !p.app.win.IsVisible() {
				p.view.Destroy()
				p.view = nil
			}
		})
	})
}

func (p *pageState) cancelRelease() {
	if p.release != nil {
		p.release.Stop()
		p.release = nil
	}
}
