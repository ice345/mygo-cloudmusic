package app

import (
	"sync"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
)

// sourcesMu guards the registry of sources, which the goroutines of the player,
// the pages and the settings read as one installed while the app runs changes
// it. There is one app, so there is one lock; a test that fills a registry
// before it reads it needs none.
var sourcesMu sync.RWMutex

// source finds a source by its ID, or nil.
func (a *app) source(id string) music.Source {
	sourcesMu.RLock()
	defer sourcesMu.RUnlock()
	return a.sources[id]
}

// addSource registers a source, unless there is one by its ID already.
func (a *app) addSource(src music.Source) bool {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	id := src.Info().ID
	if _, taken := a.sources[id]; taken {
		return false
	}
	a.sources[id] = src
	return true
}

// replaceSource puts src in the place of the source that has its ID, as a plugin
// updated while the app runs is, and registers it if there is none.
func (a *app) replaceSource(src music.Source) {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	a.sources[src.Info().ID] = src
}

// removeSource takes the source of the ID out of the registry.
func (a *app) removeSource(id string) {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	delete(a.sources, id)
}

// activeID is the ID of the source whose music the app shows: NetEase's, until
// the listener switches to another's.
func (a *app) activeID() string {
	if a.active == "" {
		return netease.SourceID
	}
	return a.active
}

// activeSource is the source whose music the app shows.
func (a *app) activeSource() music.Source { return a.source(a.activeID()) }

// homeRoute is the first page of a source: NetEase's own home, or a Browse
// page of the source's, which is the page its Home makes.
func homeRoute(id string) route {
	if id == netease.SourceID {
		return route{page: pageHome}
	}
	return route{page: pageBrowse, source: id}
}

// switchSource shows the music of the source, from its first page, with
// nothing of the other's to go back to. The listener's account, playlists and
// pages are the source's own, each asked of it as they are first wanted.
func (a *app) switchSource(id string) {
	if id == a.activeID() || a.source(id) == nil {
		return
	}
	a.sourcesOpen = false
	a.accountOpen = false
	a.active = id
	if id == netease.SourceID {
		a.active = ""
	}
	a.nowPlaying = false
	a.history, a.forward = nil, nil
	a.searched, a.query = route{}, ""
	a.dismissSuggestions()
	a.show(homeRoute(id))
	if id != netease.SourceID {
		a.enterPlugin(a.source(id))
	}
	a.redraw()
}

// sourceSwitcher is the button over the search field that says whose music the
// app shows, and opens the sources to switch to. It shows only where there is
// a plugin to switch to.
func (a *app) sourceSwitcher(c *ui.Context) {
	if len(a.plugins) == 0 {
		return
	}
	p := a.pal
	name := a.sourceName(a.activeID())
	b := ui.ButtonBase(c.Key("source")).Height(32).PaddingX(10).Gap(8).Radius(9).AlignItems(ui.Center).
		Justify(ui.Start).FocusRing(false).Margin(0, 0, 8, 0)
	if b.Hovered() || a.sourcesOpen {
		b.Background(p.hover)
	}
	b.Label("音源：" + name).Expanded(a.sourcesOpen).Children(func() {
		ui.Icon(c, iconNote).Size(14, 14).TextColor(p.muted)
		ui.Text(c, name).FontSize(13).FontWeight(600).SingleLine().Grow(1).Shrink(1).MinWidth(0)
		ui.Icon(c, iconUpDown).Size(13, 13).TextColor(p.faint)
	})
	if b.Clicked() {
		a.sourcesOpen = !a.sourcesOpen
	}
	a.sourceMenu(c, b)
}

// sourceMenu drops down from the switcher, as wide as it: NetEase, then the
// plugins, the one shown marked.
func (a *app) sourceMenu(c *ui.Context, row ui.Element) {
	pop := popoverColors(a.pal.dark)
	ui.PopoverBase(c.Key("source-menu"), row, &a.sourcesOpen, func(panel ui.Element) {
		panel.AttachTo(row, ui.AnchorBottomStart, ui.AnchorTopStart).Margin(4, 0, 0, 0).Padding(5).Gap(1).
			Radius(12).Background(pop.fill).Border(0.5, pop.edge).Shadow(0, 8, 24, -4, ui.RGBA(0, 0, 0, 0.2)).
			Role(ui.RoleMenu).FocusGroup(ui.Vertical).Label("音源").TransformOrigin(0.5, 0).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.96, Y: -4}, Exit: &ui.Motion{Scale: 0.98}})
		if w := row.Bounds().W; w > 0 {
			panel.Width(w)
		}
		ids := []string{netease.SourceID}
		for _, pl := range a.plugins {
			ids = append(ids, pl.Info().ID)
		}
		for i, id := range ids {
			icon := iconNote
			if id == a.activeID() {
				icon = iconCheck
			}
			item := a.menuItem(c, icon, a.sourceName(id), "")
			if i == 0 {
				item.AutoFocus()
			}
			if item.Clicked() {
				a.switchSource(id)
			}
		}
	})
}
