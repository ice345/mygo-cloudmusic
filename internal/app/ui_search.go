package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
)

// searchSuggest asks NetEase what a search may be after, and searchUsers for
// the users it finds, which no other source has; tests replace them.
var (
	searchSuggest = netease.SearchSuggest
	searchUsers   = func(query string, offset int) ([]netease.User, int, error) {
		res, err := netease.SearchPage(query, netease.SearchUsers, searchLimit, offset)
		return res.Users, res.Total, err
	}
)

// searchLimit is how many users load at a time; the source decides how many
// of the rest.
const searchLimit = 60

// kindUsers is the search for users, which only NetEase has: a kind of its
// own, beside those of music.SearchKind.
const kindUsers music.SearchKind = "users"

// searchKindTab is a kind of search, as its tab shows it.
type searchKindTab struct {
	kind        music.SearchKind
	label, unit string
}

// searchKinds are the kinds of search, as their tabs show them.
var searchKinds = []searchKindTab{
	{music.SearchTracks, "单曲", "首单曲"},
	{music.SearchArtists, "歌手", "位歌手"},
	{music.SearchAlbums, "专辑", "张专辑"},
	{music.SearchPlaylists, "歌单", "个歌单"},
	{kindUsers, "用户", "位用户"},
	{music.SearchShows, "播客", "个播客"},
}

// searchData is what a search found, as far as its pages have loaded.
type searchData struct {
	res music.SearchResult
	// users are of the one kind a source cannot search for.
	users []netease.User
	total int
	page  paged
}

// len is how many a search has found so far.
func (d *searchData) len() int { return d.res.Len() + len(d.users) }

// search searches for q, of the kind last searched for.
func (a *app) search(q string) {
	q = strings.TrimSpace(q)
	if q == "" {
		return
	}
	kind := a.searched.kind
	if kind == "" {
		kind = music.SearchTracks
	}
	a.searchFor(a.searchRoute(q, kind))
}

// searchRoute is the route of a search of the source shown: NetEase's names no
// source, as it never did.
func (a *app) searchRoute(q string, kind music.SearchKind) route {
	r := route{page: pageSearch, query: q, kind: kind}
	if a.activeID() != netease.SourceID {
		r.source = a.activeID()
	}
	return r
}

// searchSourceID is the source a search page searched: the one its route names,
// or NetEase.
func searchSourceID(r route) string {
	if r.source == "" {
		return netease.SourceID
	}
	return r.source
}

// kindsOf are the kinds a search of the source may look for: users are
// NetEase's alone.
func kindsOf(r route) []searchKindTab {
	if r.source == "" {
		return searchKinds
	}
	var kinds []searchKindTab
	for _, k := range searchKinds {
		if k.kind != kindUsers {
			kinds = append(kinds, k)
		}
	}
	return kinds
}

// searchFor opens the page of a search, remembering its query. Searched
// for again, it loads anew.
func (a *app) searchFor(r route) {
	a.rememberSearch(r.query)
	a.dismissSuggestions()
	if r == a.route {
		a.nowPlaying = false
		a.loadSearch(r, true)
		return
	}
	a.open(r)
}

// loadSearch loads the first page of a search, unless it has, or again.
func (a *app) loadSearch(r route, again bool) {
	if a.searches[r] != nil && !again {
		return
	}
	d := &searchData{}
	a.searches[r] = d
	a.moreResults(r, d)
}

// moreResults loads the next page of a search. A search done again has
// new data, so the last one's pages land where nothing shows them.
func (a *app) moreResults(r route, d *searchData) {
	if d.page.loading {
		return
	}
	d.page.loading, d.page.err = true, ""
	offset := d.len()
	src := a.source(searchSourceID(r))
	go func() {
		var res *music.SearchResult
		var users []netease.User
		var total int
		var err error
		switch {
		case r.kind == kindUsers && r.source == "":
			users, total, err = searchUsers(r.query, offset)
		case src == nil:
			err = music.ErrUnsupported
		default:
			res, err = src.Search(r.query, r.kind, offset)
		}
		a.post(func() {
			d.page.loading = false
			if err != nil {
				d.page.err = "搜索失败，请稍后再试"
				return
			}
			if res != nil {
				d.res.Append(*res)
				d.total, d.page.loaded, d.page.more = res.Total, true, res.More
				return
			}
			d.users = append(d.users, users...)
			d.total = total
			// A server can claim more while sending none.
			d.page.loaded, d.page.more = true, len(users) > 0 && len(d.users) < total
		})
	}()
}

// searchHistoryLimit is how many searches the history keeps.
const searchHistoryLimit = 20

// rememberSearch puts q first in the history of searches, once.
func (a *app) rememberSearch(q string) {
	history := []string{q}
	for _, s := range a.settings.SearchHistory {
		if s != q && len(history) < searchHistoryLimit {
			history = append(history, s)
		}
	}
	a.settings.SearchHistory = history
	a.settings.save()
}

func (a *app) clearSearchHistory() {
	a.settings.SearchHistory = nil
	a.settings.save()
}

func (a *app) searchPage(c *ui.Context) {
	r := a.route
	d := a.searches[r]
	header := func(c *ui.Context) { a.searchHeader(c, d) }
	more := func() { a.moreResults(r, d) }
	if r.kind == music.SearchTracks {
		if !d.page.loaded && d.page.loading {
			a.trackSkeleton(c, a.pageShimmer(c), true, header)
			return
		}
		a.trackRows(c, listOf(d.res.Tracks), true, header, func(c *ui.Context) {
			a.pageFoot(c, &d.page, more)
			a.searchEmpty(c, d)
		})
		return
	}
	a.scrollPage(c, func() {
		header(c)
		res := &d.res
		switch r.kind {
		case music.SearchArtists:
			ui.Grid(c).Columns(a.columns(c, 132)).GapX(20).GapY(28).Children(func() {
				for i := range res.Artists {
					a.artistCard(c.Key(res.Artists[i].ID), &res.Artists[i])
				}
			})
		case music.SearchAlbums:
			a.albumGrid(c, res.Albums)
		case music.SearchPlaylists:
			ui.Grid(c).Columns(a.columns(c, 156)).GapX(20).GapY(28).Children(func() {
				for i := range res.Playlists {
					a.foundPlaylistCard(c.Key(res.Playlists[i].ID), &res.Playlists[i])
				}
			})
		case kindUsers:
			for i := range d.users {
				a.userRow(c.Key(d.users[i].UserID), &d.users[i])
			}
		case music.SearchShows:
			for i := range res.Shows {
				a.podcastRow(c.Key(res.Shows[i].ID), &res.Shows[i])
			}
		}
		if !d.page.loaded && d.page.loading {
			a.playlistCardsSkeleton(c, a.pageShimmer(c), a.columns(c, 156), 156, false)
			return
		}
		a.pageFoot(c, &d.page, more)
		a.searchEmpty(c, d)
	})
}

func (a *app) searchEmpty(c *ui.Context, d *searchData) {
	if d.page.loaded && d.len() == 0 {
		a.empty(c, "没有找到相关结果")
	}
}

// searchHeader is what was searched for, how many it found, and the tabs
// of what it may look for instead.
func (a *app) searchHeader(c *ui.Context, d *searchData) {
	r := a.route
	sub := "搜索"
	at := 0
	kinds := kindsOf(r)
	for i, k := range kinds {
		if k.kind == r.kind {
			at = i
			if d.page.loaded {
				sub = fmt.Sprintf("搜索 · 找到 %d %s", d.total, k.unit)
			}
		}
	}
	a.heading(c, "“"+r.query+"”", sub)
	ui.Row(c).Margin(0, 0, 18, 0).Children(func() {
		labels := make([]string, len(kinds))
		for i, k := range kinds {
			labels[i] = k.label
		}
		if segmented(c.Key("kind"), &at, labels...) {
			r.kind = kinds[at].kind
			a.show(r)
		}
	})
}

// artistCard is an artist's round photo over their name, opening their page.
func (a *app) artistCard(c *ui.Context, ar *music.Artist) {
	p := a.pal
	card := ui.ButtonBase(c).Column().Gap(10).AlignItems(ui.Stretch).FocusRing(false).Label(ar.Name)
	hovered := card.Hovered()
	card.Children(func() {
		art := ui.Box(c).AspectRatio(1).Radius(999)
		if hovered {
			art.Shadow(0, 12, 28, -6, p.cardShade)
		} else {
			art.Shadow(0, 2, 8, -2, p.cardShade)
		}
		art.Children(func() { a.coverFill(c, ar.ImageURL, 400, 999) })
		ui.Text(c, ar.Name).FontSize(13).FontWeight(500).SingleLine().AlignSelf(ui.Center)
	})
	if card.Clicked() {
		a.open(route{page: pageArtist, source: ar.Source, id: ar.ID})
	}
}

// foundPlaylistCard is a playlist a search found: its card, with how many
// songs it has and who made it under its name, as strangers' playlists
// are told apart by their makers.
func (a *app) foundPlaylistCard(c *ui.Context, pl *music.Playlist) {
	sub := fmt.Sprintf("%d 首", pl.TrackCount)
	if pl.Owner != "" {
		sub += " · " + pl.Owner
	}
	r := route{page: pagePlaylist, source: pl.Source, id: pl.ID}
	if a.coverCard(c, pl.Name, pl.ImageURL, float64(pl.PlayCount), sub, func() { a.loadPlaylist(r.key(), true) }) {
		a.open(r)
	}
}
