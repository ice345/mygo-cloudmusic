package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
)

// A picture the NetEase source gives is fetched, at each size the views show
// it, at the very URL it always was, so that covers find what the cache holds
// and nothing is fetched twice; a view that has not moved over, with
// NetEase's own picture URL, asks for the same, and no picture asks for none.
func TestPicturesAreFetchedAtTheURLsTheyAlwaysWere(t *testing.T) {
	for _, pic := range []string{"http://p1.music.126.net/a.jpg", "https://p2.music.126.net/b/c.jpg"} {
		given := netease.ToAlbum(netease.AlbumProfile{Album: netease.Album{PicURL: pic}}).ImageURL
		for _, px := range []int{40, 96, 160, 240, 400, 600} {
			want := netease.Sized(pic, px)
			if got := sizedImage(given, px); got != want {
				t.Errorf("%s at %d: %q, want %q", pic, px, got, want)
			}
			if got := sizedImage(pic, px); got != want {
				t.Errorf("%s as a view holds it, at %d: %q, want %q", pic, px, got, want)
			}
		}
	}
	if sizedImage("", 96) != "" || netease.ToAlbum(netease.AlbumProfile{}).ImageURL != "" {
		t.Error("a picture from nothing")
	}
}

// A page is opened by NetEase's numeric ID, which a source's ID names only if
// it is one: anything else opens nothing, rather than the page of ID 0.
func TestNumericIDOpensOnlyWhatIsOne(t *testing.T) {
	for id, want := range map[string]int64{"42": 42, "0": 0, "-3": 0, "": 0, "x": 0, "42@7": 0} {
		if got := numericID(id); got != want {
			t.Errorf("%q: %d, want %d", id, got, want)
		}
	}
}

// A page is named by its source and the ID there, and loads as NetEase's only
// if NetEase is its source: another source's "8" is not NetEase's 8.
func TestRoutesNameTheirSource(t *testing.T) {
	if r := nroute(pageAlbum, 8); r.source != netease.SourceID || r.id != "8" || r.number() != 8 {
		t.Errorf("NetEase's album 8: %+v, number %d", r, r.number())
	}
	if n := (route{page: pageAlbum, source: "plug", id: "8"}).number(); n != 0 {
		t.Errorf("another source's 8 loads as NetEase's %d", n)
	}
	if n := (route{page: pageAlbum, id: "8"}).number(); n != 0 {
		t.Errorf("an ID with no source loads as NetEase's %d", n)
	}
}

// A card opens the page of the source it came from, with its ID there.
func TestCardsOpenTheirOwnSourcesPage(t *testing.T) {
	a := catalogApp()
	r := route{page: pageSearch, query: "MyGO", kind: music.SearchAlbums}
	a.searches[r] = &searchData{
		res:  music.SearchResult{Albums: []music.Album{{ID: "a-1", Source: "plug", Name: "别处的专辑"}}},
		page: paged{loaded: true},
	}
	a.open(r)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("别处的专辑"); err != nil {
		t.Fatal(err)
	}
	if a.route != (route{page: pageAlbum, source: "plug", id: "a-1"}) {
		t.Errorf("opened %+v", a.route)
	}
	// Its page says it can't open yet, and asks NetEase for nothing.
	tt.Frame()
	if _, ok := tt.Find("暂不支持打开这个来源的页面"); !ok {
		t.Error("the page does not say it is not supported")
	}
	if len(a.albums) != 1 { // catalogApp's own album 8
		t.Errorf("%d albums loaded or asked for", len(a.albums))
	}
}
