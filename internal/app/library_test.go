package app

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
)

// playlistSource is a source other than NetEase that serves playlists by ID,
// and says which it was asked for.
type playlistSource struct {
	music.Unsupported
	id        string
	playlists map[string]*music.Playlist
	asked     []string
}

func (s *playlistSource) Info() music.Info     { return music.Info{ID: s.id, Name: "别处"} }
func (s *playlistSource) Can(what string) bool { return what == "playlist" }
func (s *playlistSource) Playlist(id string) (*music.Playlist, error) {
	s.asked = append(s.asked, id)
	if pl := s.playlists[id]; pl != nil {
		return pl, nil
	}
	return nil, errors.New("no such playlist")
}

// A playlist of a source other than NetEase loads through that source, shows
// its songs, and plays them through it: the page is asked of the source its
// route names, by the ID there.
func TestPlaylistPageOfAnotherSource(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	landed := make(chan func(), 2)
	a.onPost = func(fn func()) { landed <- fn }
	src := &playlistSource{id: "plug", playlists: map[string]*music.Playlist{"p1": {
		ID: "p1", Name: "别处的歌单", Owner: "某人", PlayCount: 4200,
		Tracks: []music.Track{{ID: "t1", Source: "plug", Name: "别处的歌", Duration: 60000}, {ID: "t2", Source: "plug", Name: "另一首", Duration: 60000}},
	}}}
	a.sources["plug"] = src

	a.open(route{page: pagePlaylist, source: "plug", id: "p1"})
	wait(t, landed, "the playlist")()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if len(src.asked) != 1 || src.asked[0] != "p1" {
		t.Fatalf("asked %v", src.asked)
	}
	if _, ok := tt.Find("别处的歌"); !ok {
		t.Fatal("the songs of the playlist are not listed")
	}
	if err := tt.Click("另一首"); err != nil {
		t.Fatal(err)
	}
	if cur := a.pl.Current(); cur == nil || cur.Source != "plug" || cur.ID != "t2" || len(a.pl.Queue()) != 2 {
		t.Errorf("playing %+v of %d", cur, len(a.pl.Queue()))
	}
}

// A playlist of a source that is gone says it cannot open, rather than asking
// NetEase for it.
func TestPlaylistOfAGoneSourceSaysSo(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.open(route{page: pagePlaylist, source: "gone", id: "1"})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if _, ok := tt.Find("暂不支持打开这个来源的页面"); !ok {
		t.Error("the page does not say it cannot open")
	}
}

// A NetEase chart's page says when it is updated, which only NetEase says and
// a playlist carries in its record, and a playlist of anyone's carries no such
// label.
func TestPlaylistHeaderReadsWhatOnlyNetEaseSays(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.playlists[nkey(3)] = loadedPlaylist(netease.Playlist{ID: 3, Name: "飙升榜", UpdateFreq: "刚刚更新", Tracks: []netease.Track{{ID: 1, Name: "一", Duration: 1000}}})
	a.playlists[nkey(4)] = loadedPlaylist(netease.Playlist{ID: 4, Name: "我的歌单", Tracks: []netease.Track{{ID: 1, Name: "一", Duration: 1000}}})
	a.open(nroute(pagePlaylist, 3))
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if _, ok := tt.Find("榜单 · 刚刚更新"); !ok {
		t.Error("a chart's update label is not shown")
	}
	a.open(nroute(pagePlaylist, 4))
	tt.Frame()
	if _, ok := tt.Find("榜单 · 刚刚更新"); ok {
		t.Error("another playlist shows a chart's label")
	}
}

// albumSource is a source that serves albums by ID, and fails the others, as
// being offline does.
type albumSource struct {
	music.Unsupported
	id     string
	albums map[string]*music.AlbumPage
	asked  []string
}

func (s *albumSource) Info() music.Info     { return music.Info{ID: s.id, Name: "专辑音源"} }
func (s *albumSource) Can(what string) bool { return what == "album" }
func (s *albumSource) Album(id string) (*music.AlbumPage, error) {
	s.asked = append(s.asked, id)
	if page := s.albums[id]; page != nil {
		return page, nil
	}
	return nil, errors.New("offline")
}

// An album of a source other than NetEase's loads through that source: its
// page names its artists as links to their pages at that source, shows its
// songs and its date, and plays them through their own source.
func TestAlbumPageOfAnotherSource(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	landed := make(chan func(), 2)
	a.onPost = func(fn func()) { landed <- fn }
	src := &albumSource{id: "plug", albums: map[string]*music.AlbumPage{"al1": {
		Album:  music.Album{ID: "al1", Name: "别处的专辑", Released: "2024-03-01", Description: "介绍", Artists: []music.Artist{{ID: "ar1", Name: "某人"}, {Name: "无页面的人"}}},
		Tracks: []music.Track{{ID: "t1", Source: "plug", Name: "专辑第一首", Duration: 60000}},
	}}}
	a.sources["plug"] = src

	a.open(route{page: pageAlbum, source: "plug", id: "al1"})
	wait(t, landed, "the album")()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if len(src.asked) != 1 || src.asked[0] != "al1" {
		t.Fatalf("asked %v", src.asked)
	}
	for _, text := range []string{"别处的专辑", "专辑第一首", "某人"} {
		if _, ok := tt.Find(text); !ok {
			t.Errorf("%q is not shown", text)
		}
	}
	if err := tt.Click("某人"); err != nil {
		t.Fatal(err)
	}
	if a.route != (route{page: pageArtist, source: "plug", id: "ar1"}) {
		t.Errorf("the artist's link opened %+v", a.route)
	}
}

// artistSource is a source that serves artists by ID, fails the others as being
// offline does, and follows artists as a test says.
type artistSource struct {
	music.Unsupported
	id      string
	artists map[string]*music.ArtistPage
	follow  func(id string, on bool) error
	asked   []string
}

func (s *artistSource) Info() music.Info { return music.Info{ID: s.id, Name: "歌手音源"} }
func (s *artistSource) Can(what string) bool {
	return what == "artist" || (what == "followArtist" && s.follow != nil)
}
func (s *artistSource) Artist(id string) (*music.ArtistPage, error) {
	s.asked = append(s.asked, id)
	if page := s.artists[id]; page != nil {
		return page, nil
	}
	return nil, errors.New("offline")
}
func (s *artistSource) FollowArtist(id string, on bool) error { return s.follow(id, on) }

// An artist of a source other than NetEase's loads through that source: their
// page lists their most popular songs and, as that source gave them, their
// albums, with no tabs of NetEase's own (newest songs, events), and the songs
// play through their own source.
func TestArtistPageOfAnotherSource(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	landed := make(chan func(), 2)
	a.onPost = func(fn func()) { landed <- fn }
	src := &artistSource{id: "plug", artists: map[string]*music.ArtistPage{"ar1": {
		Artist: music.Artist{ID: "ar1", Name: "别处的歌手", Description: "简介"},
		Tracks: []music.Track{{ID: "t1", Source: "plug", Name: "热门歌", Duration: 60000}},
		Albums: []music.Album{{ID: "al1", Source: "plug", Name: "他们的专辑"}},
	}}}
	a.sources["plug"] = src

	a.open(route{page: pageArtist, source: "plug", id: "ar1"})
	wait(t, landed, "the artist")()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if len(src.asked) != 1 || src.asked[0] != "ar1" {
		t.Fatalf("asked %v", src.asked)
	}
	for _, text := range []string{"别处的歌手", "热门歌", "专辑"} {
		if _, ok := tt.Find(text); !ok {
			t.Errorf("%q is not shown", text)
		}
	}
	for _, text := range []string{"最新", "动态"} {
		if _, ok := tt.Find(text); ok {
			t.Errorf("a tab of NetEase's own, %q, shows", text)
		}
	}
	if err := tt.Click("专辑"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find("他们的专辑"); !ok {
		t.Error("their albums are not shown on the 专辑 tab")
	}
}

// showSource is a source of podcasts whose pages, and subscribing, are what a
// test says.
type showSource struct {
	music.Unsupported
	id        string
	show      func(id string, offset int) (*music.ShowPage, error)
	subscribe func(id string, on bool) error
	signIn    bool // it has accounts to sign in to
}

func (s *showSource) Info() music.Info { return music.Info{ID: s.id, Name: "播客音源"} }
func (s *showSource) Can(what string) bool {
	return what == "show" || what == "subscribeShow" && s.subscribe != nil || what == "signIn" && s.signIn
}
func (s *showSource) Show(id string, offset int) (*music.ShowPage, error) { return s.show(id, offset) }
func (s *showSource) SubscribeShow(id string, on bool) error              { return s.subscribe(id, on) }

// A podcast of a source other than NetEase's loads through that source, a page
// of episodes at a time from where the last ended, and its episodes play through
// their own source, those with no audio left out of the queue; it offers
// subscribing only if the source can, and a source that is gone says so.
func TestPodcastOfAnotherSource(t *testing.T) {
	type ask struct {
		id     string
		offset int
	}
	asks := make(chan ask, 4)
	a := newTestApp()
	a.settings = defaultSettings()
	landed := make(chan func(), 4)
	a.onPost = func(fn func()) { landed <- fn }
	a.sources["plug"] = &showSource{id: "plug", show: func(id string, offset int) (*music.ShowPage, error) {
		asks <- ask{id, offset}
		show := music.Show{ID: "p1", Name: "别处的播客", Publisher: "他们的主播", EpisodeCount: 3}
		if offset == 0 {
			return &music.ShowPage{Show: show, More: true, Episodes: []music.Track{
				{ID: "e3", Source: "plug", ShowID: "p1", Name: "第三期", Duration: 60000, Published: "2026-03-01"},
				{ID: "e2", Source: "plug", ShowID: "p1", Name: "第二期", Duration: 60000, Unavailable: true},
			}}, nil
		}
		return &music.ShowPage{Show: show, Episodes: []music.Track{{ID: "e1", Source: "plug", ShowID: "p1", Name: "第一期", Duration: 60000}}}, nil
	}}

	a.open(route{page: pagePodcast, source: "plug", id: "p1"})
	wait(t, landed, "the podcast")()
	if got := wait(t, asks, "the first ask"); got != (ask{"p1", 0}) {
		t.Fatalf("asked %+v", got)
	}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	for _, text := range []string{"别处的播客", "他们的主播", "第三期", "第二期", "2026-03-01"} {
		if !tt.HasText(text) {
			t.Errorf("%q is not shown", text)
		}
	}
	if _, ok := tt.Find("订阅"); ok {
		t.Error("offered subscribing on a source that cannot")
	}
	d := a.podcasts[pageKey{"plug", "p1"}]
	a.loadPrograms(pageKey{"plug", "p1"})
	wait(t, landed, "the second page")()
	if got := wait(t, asks, "the second ask"); got != (ask{"p1", 2}) {
		t.Errorf("asked %+v, want the next page from 2", got)
	}
	if len(d.episodes) != 3 || d.page.more {
		t.Fatalf("episodes %d, more %v", len(d.episodes), d.page.more)
	}
	a.playProgram(d, 2)
	q := a.queued()
	if len(q) != 2 || q[0].Source != "plug" || q[0].ID != "e3" || q[1].ID != "e1" || a.current().ID != "e1" {
		t.Errorf("queue %+v, playing %+v", q, a.current())
	}

	a.open(route{page: pagePodcast, source: "gone", id: "x"})
	tt.Frame()
	if _, ok := tt.Find("暂不支持打开这个来源的页面"); !ok {
		t.Error("a podcast of a source that is gone does not say so")
	}
}

// Subscribing to a podcast shows at once, with one more subscriber, and asks
// its source; the tab lists it first once it has loaded; if the source refuses,
// it goes back and says so; signed out, it asks to sign in.
func TestSubscribeToPodcast(t *testing.T) {
	calls := make(chan bool, 2)
	var fail bool
	a := newTestApp()
	a.settings = defaultSettings()
	landed := make(chan func(), 4)
	a.onPost = func(fn func()) { landed <- fn }
	a.sources["plug"] = &showSource{id: "plug", signIn: true,
		show: func(id string, offset int) (*music.ShowPage, error) {
			return &music.ShowPage{Show: music.Show{ID: "p1", Name: "别处的播客", Subscribers: 10}}, nil
		},
		subscribe: func(id string, on bool) error {
			calls <- on
			if fail {
				return errors.New("refused")
			}
			return nil
		},
	}
	// Known to be signed out of the plugin, as it said so when asked.
	a.stateOf("plug").asked = true
	a.open(route{page: pagePodcast, source: "plug", id: "p1"})
	wait(t, landed, "the podcast")()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	d := a.podcasts[pageKey{"plug", "p1"}]

	a.sources[netease.SourceID] = &loginSource{ends: make(chan loginEnd, 1)}
	if err := tt.Click("订阅"); err != nil {
		t.Fatal(err)
	}
	if !a.login.open || d.show.Subscribed {
		t.Fatalf("signed out: dialog %v, subscribed %v", a.login.open, d.show.Subscribed)
	}
	if a.login.source != "plug" {
		t.Errorf("asked to sign in to %q", a.login.source)
	}
	// The sign-in under way ends, this plugin having none to offer.
	wait(t, landed, "the sign-in's end")()
	a.login.open = false
	a.stateOf("plug").account = &music.Account{ID: "1"}
	a.podcastTab.hotPage.loaded = true
	tt.Frame()
	if err := tt.Click("订阅"); err != nil {
		t.Fatal(err)
	}
	if !d.show.Subscribed || d.show.Subscribers != 11 {
		t.Fatalf("not shown at once: %+v", d.show)
	}
	if on := wait(t, calls, "the subscription"); !on {
		t.Error("asked to unsubscribe")
	}
	wait(t, landed, "its end")()
	if len(a.podcastTab.subs) != 1 || a.podcastTab.subs[0].ID != "p1" {
		t.Errorf("the tab lists %+v", a.podcastTab.subs)
	}

	fail = true
	tt.Frame()
	if err := tt.Click("已订阅"); err != nil {
		t.Fatal(err)
	}
	wait(t, calls, "the unsubscription")
	wait(t, landed, "its refusal")()
	if !d.show.Subscribed || d.show.Subscribers != 11 || a.toast != "操作失败，请稍后再试" {
		t.Errorf("a refusal was not undone: %+v, toast %q", d.show, a.toast)
	}
}
