package app

import (
	"testing"

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
