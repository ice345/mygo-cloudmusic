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
