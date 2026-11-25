package app

import (
	"errors"
	"strconv"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
)

// browseData is a page a plugin makes up, as far as it has come in: what it
// answered, or why it could not.
type browseData struct {
	page    *music.Browse
	loading bool
	// err is what the page says in place of itself; need is what the
	// listener is to do about it: sign in, or set the plugin up.
	err    string
	need   error
	reason string
}

// loadBrowse loads the page of the plugin that k names: its Home for an empty
// page, or the Browse page of that name. A page asked for twice is loaded
// once, unless again.
func (a *app) loadBrowse(k pageKey, again bool) {
	if d := a.browses[k]; d != nil && !again && (d.page != nil || d.loading) {
		return
	}
	if a.browses == nil {
		a.browses = map[pageKey]*browseData{}
	}
	d := &browseData{loading: true}
	a.browses[k] = d
	src := a.source(k.source)
	go func() {
		var page *music.Browse
		var err error
		switch {
		case src == nil:
			err = music.ErrUnavailable
		case k.id == "":
			page, err = src.Home()
		default:
			page, err = src.Browse(k.id)
		}
		a.post(func() {
			d.loading = false
			d.page, d.err, d.need, d.reason = page, "", nil, ""
			switch {
			case err == nil && page == nil:
				d.err = "这个页面是空的"
			case err == nil:
			case errors.Is(err, music.ErrSignIn):
				d.need, d.reason = music.ErrSignIn, music.Reason(err)
				d.err = "需要先登录 " + a.sourceName(k.source)
			case errors.Is(err, music.ErrSetup):
				d.need, d.reason = music.ErrSetup, music.Reason(err)
				d.err = "需要先设置 " + a.sourceName(k.source)
			case errors.Is(err, music.ErrUnsupported):
				d.err = "这个音源没有这个页面"
			default:
				d.err = "加载失败，请稍后再试"
			}
		})
	}()
}

// browsePage draws a page a plugin makes up: its title, and its sections one
// under another; or, where it could not be had, what to do about it.
func (a *app) browsePage(c *ui.Context) {
	k := a.route.key()
	d := a.browses[k]
	switch {
	case d == nil || d.loading && d.page == nil:
		a.scrollPage(c, func() {
			a.playlistCardsSkeleton(c, a.pageShimmer(c), a.columns(c, 156), 156, false)
		})
		return
	case d.page == nil:
		a.browseFailed(c, k, d)
		return
	}
	page := d.page
	a.scrollPage(c, func() {
		title := page.Title
		if title == "" {
			title = a.sourceName(k.source)
		}
		a.heading(c, title, page.Subtitle)
		if page.Description != "" {
			ui.Text(c, page.Description).FontSize(13).TextColor(a.pal.muted).LineHeight(1.5).Margin(-6, 0, 12, 0)
		}
		for i := range page.Sections {
			ui.Column(c.Key(strconv.Itoa(i))).Children(func() { a.browseSection(c, k, &page.Sections[i]) })
		}
		if len(page.Sections) == 0 {
			a.empty(c, "这个页面没有内容")
		}
	})
}

// browseFailed says a plugin's page could not be had, with what the listener
// can do about it.
func (a *app) browseFailed(c *ui.Context, k pageKey, d *browseData) {
	top, bottom := a.insets(c)
	p := a.pal
	ui.Column(c).Grow(1).Padding(top, 0, bottom, 0).Center().Gap(12).Children(func() {
		ui.Text(c, d.err).FontSize(15).FontWeight(600)
		if d.reason != "" {
			ui.Text(c, d.reason).FontSize(13).TextColor(p.muted).TextAlign(ui.Center).MaxWidth(420)
		}
		switch d.need {
		case music.ErrSignIn:
			if pillButton(c.Key("signin"), nil, "登录", p.accent, p.onAccent).Clicked() {
				a.openLoginFor(a.source(k.source))
			}
		case music.ErrSetup:
			if pillButton(c.Key("setup"), iconSettings, "打开设置", p.accent, p.onAccent).Clicked() {
				a.openSettings()
			}
		default:
			if ui.Button(c.Key("retry"), "重试").Clicked() {
				a.loadBrowse(k, true)
			}
		}
	})
}

// browseSection draws a section of a plugin's page: its title, with a link to
// the rest of it where there is one, over what it lists, of each kind in turn.
func (a *app) browseSection(c *ui.Context, k pageKey, sec *music.Section) {
	p := a.pal
	if sec.Title != "" {
		ui.Row(c).AlignItems(ui.Center).Margin(34, 0, 14, 0).Children(func() {
			ui.Text(c, sec.Title).FontSize(19).FontWeight(700).LetterSpacing(-0.3).Grow(1)
			if sec.MorePage != "" {
				b := ui.ButtonBase(c.Key("more")).Height(26).PaddingX(8).Radius(6).AlignItems(ui.Center).FocusRing(false).Label("更多")
				if b.Hovered() {
					b.Background(p.hover)
				}
				b.Children(func() { ui.Text(c, "更多").FontSize(12.5).TextColor(p.muted) })
				if b.Clicked() {
					a.open(route{page: pageBrowse, source: k.source, id: sec.MorePage})
				}
			}
		})
	}
	empty := len(sec.Tracks)+len(sec.Albums)+len(sec.Artists)+len(sec.Playlists)+len(sec.Shows)+len(sec.Links) == 0
	if empty && sec.Empty != "" {
		ui.Text(c, sec.Empty).FontSize(13).TextColor(p.muted).Margin(0, 0, 8, 0)
	}
	if len(sec.Tracks) > 0 {
		list := listOf(sec.Tracks)
		w, _ := c.Size()
		wide := w-sidebarWidth > 820
		ui.Column(c).Children(func() {
			for i := range sec.Tracks {
				a.songRow(c, list, i, true, wide, "")
			}
		})
	}
	if len(sec.Albums) > 0 {
		a.albumGrid(c, sec.Albums)
	}
	if len(sec.Artists) > 0 {
		ui.Grid(c).Columns(a.columns(c, 132)).GapX(20).GapY(28).Children(func() {
			for i := range sec.Artists {
				a.artistCard(c.Key(sec.Artists[i].ID), &sec.Artists[i])
			}
		})
	}
	if len(sec.Playlists) > 0 {
		ui.Grid(c).Columns(a.columns(c, 156)).GapX(20).GapY(28).Children(func() {
			for i := range sec.Playlists {
				a.foundPlaylistCard(c.Key(sec.Playlists[i].ID), &sec.Playlists[i])
			}
		})
	}
	for i := range sec.Shows {
		a.podcastRow(c.Key(sec.Shows[i].ID), &sec.Shows[i])
	}
	if len(sec.Links) > 0 {
		ui.Grid(c).Columns(a.columns(c, 156)).GapX(20).GapY(28).Children(func() {
			for i := range sec.Links {
				a.linkCard(c.Key(strconv.Itoa(i)), k, &sec.Links[i])
			}
		})
	}
}

// linkCard is a link to another page of a plugin's: its picture over its title,
// and what it says of it under that.
func (a *app) linkCard(c *ui.Context, k pageKey, l *music.Link) {
	p := a.pal
	card := ui.ButtonBase(c).Column().Gap(10).AlignItems(ui.Stretch).Justify(ui.Start).FocusRing(false).Label(l.Title)
	hovered := card.Hovered()
	card.Children(func() {
		art := ui.Box(c).AspectRatio(1).Radius(10)
		if hovered {
			art.Shadow(0, 12, 28, -6, p.cardShade)
		} else {
			art.Shadow(0, 2, 8, -2, p.cardShade)
		}
		art.Children(func() { a.coverFill(c, l.ImageURL, 400, 10) })
		ui.Text(c, l.Title).FontSize(13).FontWeight(500).MaxLines(2).LineHeight(1.35)
		if l.Subtitle != "" {
			ui.Text(c, l.Subtitle).FontSize(12).TextColor(p.muted).SingleLine().Margin(-6, 0, 0, 0)
		}
	})
	if card.Clicked() {
		a.open(route{page: pageBrowse, source: k.source, id: l.Page})
	}
}
