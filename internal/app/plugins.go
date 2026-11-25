// headerPlugins is ported from internal/app of egoist/mygo-music (AGPL-3.0, as
// this repository is).

package app

import (
	"log"
	"net/url"
	"path/filepath"
	"sync/atomic"

	"github.com/egoist/mygo"

	"netease-music-mygo/internal/plugin"
)

// pluginDirs are where the plugins are, a folder to each, and where each keeps
// its settings and what it stores, beside them. Both are in the user data
// directory.
func pluginDirs() (code, data string, ok bool) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", "", false
	}
	return filepath.Join(dir, "plugins"), filepath.Join(dir, "plugin-data"), true
}

// usePlugins makes each plugin found in code a source, beside those built in.
// A plugin whose ID is one that is there already, NetEase's or that of another
// found first, is left out: it can't stand in for a source, nor can two be
// told apart. The registry is behind a lock (see sourcesMu), as a plugin
// installed while the app runs changes it under those who read it.
func (a *app) usePlugins(code, data string) {
	for _, p := range plugin.Discover(code, data) {
		if !a.addSource(p) {
			log.Printf("plugin %s: there is a source with that ID already", p.Info().ID)
			continue
		}
		a.plugins = append(a.plugins, p)
	}
	if a.headers == nil {
		a.headers = &headerPlugins{}
	}
	a.headers.set(a.plugins)
}

// closePlugins stops the plugins, and the programs they started.
func (a *app) closePlugins() {
	for _, p := range a.plugins {
		p.Close()
	}
}

// headerPlugins are the plugins that are on, for the pictures loading off
// the main thread to ask for their requestHeaders.
type headerPlugins struct {
	on atomic.Pointer[[]*plugin.Plugin]
}

// set takes the plugins that are on.
func (h *headerPlugins) set(on []*plugin.Plugin) { h.on.Store(&on) }

// image answers the headers to fetch the picture at u with: those of the
// first plugin on whose requestHeaders are for its host.
func (h *headerPlugins) image(u *url.URL) map[string]string {
	if h == nil {
		return nil
	}
	on := h.on.Load()
	if on == nil {
		return nil
	}
	for _, p := range *on {
		if headers := p.HeadersFor(u, plugin.KindImage); headers != nil {
			return headers
		}
	}
	return nil
}
