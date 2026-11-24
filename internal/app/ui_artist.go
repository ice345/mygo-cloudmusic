package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
)

// artistLinks shows a track's artists as its credits read, each a link to
// their page in color, brightening to hover under the pointer, and then,
// with album, a link to its album. An artist NetEase has no page for,
// credited by name alone, is plain text. An episode's host and podcast,
// in its artists' and album's place, link to the podcast's page.
func (a *app) artistLinks(c *ui.Context, tr music.Track, color, hover ui.Color, album ...bool) ui.Element {
	podcast := route{page: pagePodcast, source: tr.Source, id: tr.ShowID}
	return ui.RichText(c).TextColor(color).Children(func() {
		for i, ar := range tr.Artists {
			if i > 0 {
				ui.Text(c, ", ")
			}
			switch {
			case tr.ShowID != "":
				a.link(c.Key(i), ar.Name, color, hover, podcast)
			case ar.ID != "":
				a.link(c.Key(i), ar.Name, color, hover, route{page: pageArtist, source: tr.Source, id: ar.ID})
			default:
				ui.Text(c, ar.Name)
			}
		}
		if len(album) > 0 && album[0] && tr.Album.Name != "" {
			ui.Text(c, " — ")
			switch {
			case tr.ShowID != "":
				a.link(c.Key("album"), tr.Album.Name, color, hover, podcast)
			case tr.Album.ID != "":
				a.link(c.Key("album"), tr.Album.Name, color, hover, route{page: pageAlbum, source: tr.Source, id: tr.Album.ID})
			default:
				ui.Text(c, tr.Album.Name)
			}
		}
	})
}

// link is text in a RichText that opens a page.
func (a *app) link(c *ui.Context, text string, color, hover ui.Color, r route) {
	link := ui.Link(c, text, "").TextColor(color)
	if link.Hovered() {
		link.TextColor(hover)
	}
	// Once built: a song's row is in a list, which a new page resets.
	link.OnClick(func() { a.openPage(r) })
}

// openPage opens a page in the main window, from wherever a link to it
// shows: over Now Playing, which closes, or the notch.
func (a *app) openPage(r route) {
	a.open(r)
	a.showMain()
	a.redraw()
}

// openArtist opens an artist's page in the main window.
func (a *app) openArtist(source, id string) {
	a.openPage(route{page: pageArtist, source: source, id: id})
}

// These load an artist's newest songs, their albums, which come in pages, and
// their events, which are NetEase's own, from NetEase; tests replace them.
var (
	artistLatestSongs = netease.ArtistLatestSongs
	albumsBy          = netease.ArtistAlbums
	accountEvents     = netease.Events
)

// artistData is what the app has of an artist: who they are, with their most
// popular songs, as their source gave them, and what NetEase says of them
// besides, read from its record once. Their newest songs, albums and events,
// which are NetEase's own, come once asked for.
type artistData struct {
	page          *music.ArtistPage
	rec           netease.ArtistProfile // NetEase's record of them: aliases, counts, their account
	loading       bool
	err           string
	latest        []netease.Track
	latestLoading bool
	latestErr     string
	albums        []netease.AlbumProfile
	albumsPage    paged
	events        []netease.Event
	eventsPage    paged
	eventsBefore  int64 // the time of the last event loaded, the next page's start
	following     bool  // a follow or unfollow is on its way
}

// What an artist's page lists: their songs, the most popular or the
// newest, their albums, or their events.
const (
	artistHot = iota
	artistLatest
	artistAlbums
	artistEvents
)

// latestCount is how many of an artist's newest songs their page lists;
// albumsPage and eventsPage how many albums and events load at a time.
const (
	latestCount = 100
	albumsPage  = 30
	eventsPage  = 20
)

func (a *app) artist(k pageKey) *artistData {
	d := a.artists[k]
	if d == nil {
		d = &artistData{}
		a.artists[k] = d
	}
	return d
}

// ownTabs reports whether the source of the artist has the tabs that are
// NetEase's own: newest songs, albums in pages, and events. Another source's
// artist page lists their most popular songs, and their albums if it gives
// them.
func ownTabs(k pageKey) bool { return k.source == netease.SourceID }

// loadArtist loads an artist's page, and what it lists.
func (a *app) loadArtist(k pageKey) {
	if k.id == "" || k.id == "0" {
		return
	}
	d := a.artist(k)
	if n := numericID(k.id); ownTabs(k) && n != 0 {
		switch a.artistTab {
		case artistLatest:
			a.loadLatest(n)
		case artistAlbums:
			if !d.albumsPage.loaded {
				a.loadAlbums(n)
			}
		}
	}
	if d.page != nil || d.loading {
		if ownTabs(k) && a.artistTab == artistEvents && !d.eventsPage.loaded {
			a.loadEvents(k)
		}
		return
	}
	src := a.source(k.source)
	if src == nil {
		d.err = "暂不支持打开这个来源的页面"
		return
	}
	d.loading, d.err = true, ""
	go func() {
		page, err := src.Artist(k.id)
		a.post(func() {
			d.loading = false
			if err != nil {
				d.err = "歌手加载失败"
				return
			}
			d.page = page
			if ownTabs(k) {
				d.rec = netease.FromArtist(page.Artist)
			}
			// Their events are their account's, which their profile names.
			if ownTabs(k) && a.artistTab == artistEvents && a.route.key() == k && a.route.page == pageArtist {
				a.loadEvents(k)
			}
		})
	}()
}

func (a *app) loadLatest(id int64) {
	d := a.artist(nkey(id))
	if d.latest != nil || d.latestLoading {
		return
	}
	d.latestLoading, d.latestErr = true, ""
	go func() {
		songs, err := artistLatestSongs(id, latestCount)
		a.post(func() {
			d.latestLoading = false
			if err != nil {
				d.latestErr = "歌曲加载失败"
				return
			}
			d.latest = songs
		})
	}()
}

// loadAlbums loads the next page of an artist's albums.
func (a *app) loadAlbums(id int64) {
	if id == 0 {
		return
	}
	d := a.artist(nkey(id))
	if d.albumsPage.loading {
		return
	}
	d.albumsPage.loading, d.albumsPage.err = true, ""
	offset := len(d.albums)
	go func() {
		albums, more, err := albumsBy(id, albumsPage, offset)
		a.post(func() {
			d.albumsPage.loading = false
			if err != nil {
				d.albumsPage.err = "专辑加载失败"
				return
			}
			d.albums = append(d.albums, albums...)
			d.albumsPage.loaded, d.albumsPage.more = true, more && len(albums) > 0
		})
	}()
}

// loadEvents loads the next page of an artist's events, once their profile
// says whose they are.
func (a *app) loadEvents(k pageKey) {
	d := a.artist(k)
	if d.page == nil || d.rec.AccountID == 0 || d.eventsPage.loading {
		return
	}
	d.eventsPage.loading, d.eventsPage.err = true, ""
	uid, before := d.rec.AccountID, d.eventsBefore
	if !d.eventsPage.loaded {
		before = -1
	}
	go func() {
		events, last, more, err := accountEvents(uid, eventsPage, before)
		a.post(func() {
			d.eventsPage.loading = false
			if err != nil {
				d.eventsPage.err = "动态加载失败"
				return
			}
			d.events = append(d.events, events...)
			d.eventsPage.loaded = true
			// A page that starts no earlier would load forever.
			d.eventsPage.more = more && len(events) > 0 && (before == -1 || last < before)
			d.eventsBefore = last
		})
	}()
}

// toggleFollow follows an artist, or unfollows them, at once, going back if
// their source refuses. Signed out, it asks the listener to sign in.
func (a *app) toggleFollow(k pageKey) {
	d := a.artist(k)
	src := a.source(k.source)
	if d.page == nil || d.following || src == nil {
		return
	}
	if a.account == nil {
		a.openLogin()
		return
	}
	ar := &d.page.Artist
	follow := !ar.Following
	ar.Following, d.following = follow, true
	go func() {
		err := src.FollowArtist(k.id, follow)
		a.post(func() {
			d.following = false
			switch {
			case err != nil:
				ar.Following = !follow
				a.toast = "操作失败，请稍后再试"
			case follow:
				a.toast = "已关注"
			default:
				a.toast = "已取消关注"
			}
		})
	}()
}

// artistPage shows who an artist is over their songs, the most popular or
// the newest, their albums or their events.
func (a *app) artistPage(c *ui.Context) {
	k := a.route.key()
	d := a.artist(k)
	if d.page == nil {
		if !d.loading {
			a.centered(c, d.err, func() { a.loadArtist(k) })
			return
		}
		s := a.pageShimmer(c)
		a.trackSkeleton(c, s, true, func(c *ui.Context) { a.artistHeaderSkeleton(c, s) })
		return
	}
	header := func(c *ui.Context) { a.artistHeader(c, k, d) }
	tab := a.artistTabOf(k, d)
	switch tab {
	case artistAlbums:
		if !ownTabs(k) {
			a.scrollPage(c, func() {
				header(c)
				a.albumGrid(c, d.page.Albums)
				if len(d.page.Albums) == 0 {
					a.empty(c, "暂无专辑")
				}
			})
			return
		}
		a.scrollPage(c, func() {
			header(c)
			a.albumGrid(c, netease.ToAlbums(d.albums))
			a.pageFoot(c, &d.albumsPage, func() { a.loadAlbums(numericID(k.id)) })
			if d.albumsPage.loaded && len(d.albums) == 0 {
				a.empty(c, "暂无专辑")
			}
		})
		return
	case artistEvents:
		a.scrollPage(c, func() {
			header(c)
			a.artistEvents(c, k, d)
		})
		return
	}
	songs := listOf(d.page.Tracks)
	if tab == artistLatest {
		switch {
		case d.latest != nil:
			songs = neteaseList(d.latest)
		case d.latestLoading:
			a.trackSkeleton(c, a.pageShimmer(c), true, header)
			return
		default:
			songs = listOf(nil)
			header = func(c *ui.Context) {
				a.artistHeader(c, k, d)
				ui.Column(c).Padding(40, 0).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Text(c, d.latestErr).TextColor(a.pal.muted).FontSize(13)
					if ui.Button(c.Key("retry"), "重试").Clicked() {
						a.loadLatest(numericID(k.id))
					}
				})
			}
		}
	}
	a.trackRows(c, songs, true, header)
}

// scrollPage scrolls a page of cards and rows, inset as one of tracks is:
// a tab of a page that lists tracks puts its header in the same place.
func (a *app) scrollPage(c *ui.Context, content func()) {
	top, bottom := a.insets(c)
	ui.Scroll(c.Key(a.route)).TrackScroll(&a.pageScroll).Grow(1).Padding(top+pageGap, pagePad-12, bottom+32, pagePad-12).ScrollbarInsets(top, 0, bottom, 0).Children(func() {
		a.startMark(c)
		defer a.endMark(c)
		content()
	})
}

// empty says a list has nothing in it.
func (a *app) empty(c *ui.Context, msg string) {
	ui.Text(c, msg).FontSize(13).TextColor(a.pal.muted).Padding(40, 0).AlignSelf(ui.Center)
}

// artistPhotoSize is the width of an artist's round photo on their page.
const artistPhotoSize = 212

// artistTabOf is the tab an artist's page shows: the one chosen, if their
// source has it. Another source's has their most popular songs, and their
// albums if it gives them.
func (a *app) artistTabOf(k pageKey, d *artistData) int {
	switch {
	case ownTabs(k):
		return a.artistTab
	case a.artistTab == artistAlbums && len(d.page.Albums) > 0:
		return artistAlbums
	}
	return artistHot
}

// artistTabs are the tabs of an artist's page, and what they are called.
func artistTabs(k pageKey, d *artistData) ([]int, []string) {
	switch {
	case ownTabs(k):
		return []int{artistHot, artistLatest, artistAlbums, artistEvents}, []string{"热门", "最新", "专辑", "动态"}
	case len(d.page.Albums) > 0:
		return []int{artistHot, artistAlbums}, []string{"热门", "专辑"}
	}
	return []int{artistHot}, []string{"热门"}
}

func (a *app) artistHeader(c *ui.Context, k pageKey, d *artistData) {
	p := a.pal
	ar := d.page.Artist
	tab := a.artistTabOf(k, d)
	// What the buttons play: the songs listed, or else the most popular.
	songs := func() []music.Track {
		if tab == artistLatest && d.latest != nil {
			return netease.ToTracks(d.latest)
		}
		return d.page.Tracks
	}
	ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 8, 0).Children(func() {
		ui.Box(c).Size(artistPhotoSize, artistPhotoSize).Radius(artistPhotoSize/2).Shadow(0, 18, 40, -10, p.cardShade).Children(func() {
			a.coverFill(c, ar.ImageURL, 600, artistPhotoSize/2)
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
			ui.Text(c, "歌手").FontSize(12).FontWeight(600).TextColor(p.accent)
			ui.Text(c, ar.Name).FontSize(30).FontWeight(800).LetterSpacing(-0.6).MaxLines(2).LineHeight(1.2)
			var meta []string
			// What only NetEase says of them: other names, and counts.
			if ownTabs(k) {
				if aka := distinct(append([]string{d.rec.Trans}, d.rec.Alias...)); len(aka) > 0 {
					meta = append(meta, strings.Join(aka, " / "))
				}
				if d.rec.MusicSize > 0 {
					meta = append(meta, fmt.Sprintf("%d 首歌曲", d.rec.MusicSize))
				}
				if d.rec.AlbumSize > 0 {
					meta = append(meta, fmt.Sprintf("%d 张专辑", d.rec.AlbumSize))
				}
			}
			ui.Text(c, strings.Join(meta, " · ")).FontSize(13).TextColor(p.muted)
			if desc := strings.TrimSpace(ar.Description); desc != "" {
				ui.Text(c, strings.ReplaceAll(desc, "\n", " ")).FontSize(12.5).TextColor(p.muted).MaxLines(2).LineHeight(1.45).Margin(2, 0, 0, 0)
			}
			ui.Row(c).Gap(10).Margin(10, 0, 0, 0).Children(func() {
				if pillButton(c.Key("play"), iconPlay, "播放", p.accent, p.onAccent).Clicked() {
					if a.pl.Shuffle() {
						a.pl.ToggleShuffle()
					}
					list := songs()
					a.pl.PlayQueue(list, firstAvailable(list))
				}
				if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() {
					if !a.pl.Shuffle() {
						a.pl.ToggleShuffle()
					}
					list := songs()
					a.pl.PlayQueue(list, firstAvailable(list))
				}
				if a.source(k.source) != nil && a.source(k.source).Can("followArtist") {
					icon, label := iconPlus, "关注"
					if ar.Following {
						icon, label = iconCheck, "已关注"
					}
					if pillButton(c.Key("follow"), icon, label, p.hover.Over(p.bg), p.text).Clicked() {
						a.toggleFollow(k)
					}
				}
			})
		})
	})
	tabs, labels := artistTabs(k, d)
	ui.Row(c).AlignItems(ui.Center).Margin(34, 0, 14, 0).Children(func() {
		ui.Text(c, artistTabTitles[tab]).FontSize(19).FontWeight(700).LetterSpacing(-0.3)
		ui.Spacer(c)
		if len(labels) > 1 {
			at := max(slices.Index(tabs, tab), 0)
			if segmented(c.Key("tab"), &at, labels...) {
				a.artistTab = tabs[at]
				a.loadArtist(k)
			}
		}
	})
}

var artistTabTitles = [...]string{"歌曲", "歌曲", "专辑", "动态"}

func (a *app) artistHeaderSkeleton(c *ui.Context, s shimmer) {
	ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 8, 0).Children(func() {
		s.bone(c, artistPhotoSize/2).Size(artistPhotoSize, artistPhotoSize)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
			s.text(c, 12, 1.3).Width(24)
			s.text(c, 30, 1.2).WidthPercent(32)
			s.text(c, 13, 1.3).Width(160)
			ui.Column(c).Margin(2, 0, 0, 0).Children(func() {
				s.text(c, 12.5, 1.45).WidthPercent(78)
				s.text(c, 12.5, 1.45).WidthPercent(52)
			})
			ui.Row(c).Gap(10).Margin(10, 0, 0, 0).Children(func() {
				s.bone(c, 18).Size(92, 36)
				s.bone(c, 18).Size(120, 36)
			})
		})
	})
	a.section(c, "歌曲")
}

// distinct returns ss without empty strings, nor any after its first: an
// artist's translated name is often among their aliases too.
func distinct(ss []string) []string {
	var out []string
	for _, s := range ss {
		if s != "" && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}
