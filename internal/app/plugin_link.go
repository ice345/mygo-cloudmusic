// Ported in part from internal/app/plugin_install.go of egoist/mygo-music
// (AGPL-3.0, as this repository is).

package app

import (
	"log"
	"net/url"
	"strings"

	"github.com/egoist/mygo"
)

// A link on the web installs a plugin:
//
//	mygo-music://install-plugin?repo=owner/repo
//
// which opens the plugins' settings with the repository written in the field of
// the dialog that installs from GitHub. Nothing is fetched until the listener
// presses Install, and then what comes is asked about as any plugin is: a page
// can make the app show a question, and not make it download or install.

// linkScheme is the scheme of the app's links, declared in mygo.json: the
// development app's is its own, which leaves the installed app's links to it.
var linkScheme = func() string {
	if mygo.IsDev() {
		return "mygo-music-dev"
	}
	return "mygo-music"
}()

// maxLinkRepo is the longest repository a link may name: it is written in a
// field the listener reads.
const maxLinkRepo = 200

// showSettings opens the settings window; tests replace it.
var showSettings = func(a *app) { a.openSettings() }

// linkOpened takes a link the app was asked to open. One that arrives before the
// window is up, as the one the app was started with, waits for it.
func (a *app) linkOpened(raw string) {
	if !a.ready {
		a.links = append(a.links, raw)
		return
	}
	a.openLink(raw)
}

// linksReady marks the app ready for links, and opens those that came before.
func (a *app) linksReady() {
	a.ready = true
	links := a.links
	a.links = nil
	for _, raw := range links {
		a.openLink(raw)
	}
}

// openLink acts on a link of the app's scheme. One to install a plugin shows the
// plugins' settings, and in them the dialog that installs from GitHub, with its
// repository in the field. Others are left alone.
func (a *app) openLink(raw string) {
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, linkScheme) {
		return
	}
	// mygo-music://install-plugin, or mygo-music:install-plugin.
	action := u.Host
	if action == "" {
		action = u.Opaque + u.Path
	}
	if strings.Trim(action, "/") != "install-plugin" {
		log.Printf("unknown link: %s", raw)
		return
	}
	repo := strings.TrimSpace(u.Query().Get("repo"))
	if repo == "" || len(repo) > maxLinkRepo {
		return
	}
	in := &a.installer
	a.cancelInstall()
	in.repo = repo
	in.open, in.github, in.err = true, true, ""
	showSettings(a)
	if a.win != nil {
		// The link was opened in the browser, which is in front.
		mygo.App.Focus()
	}
	a.redraw()
}
