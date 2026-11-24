package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
)

// foreignTrack is a song of a source other than NetEase's, with a cover whose
// URL says where its size goes, and an artist of its own.
func foreignTrack() music.Track {
	return music.Track{
		ID: "t1", Source: "plug", Name: "外来的歌", Duration: 200000,
		Artists: []music.Artist{{ID: "a9", Name: "外来歌手"}},
		Album:   music.Album{ID: "al1", Name: "外来专辑", ImageURL: "https://plug.invalid/cover/{size}.jpg"},
	}
}

// Now Playing shows a song of another source with that source's own cover, its
// size filled in where its URL says, and its artist's name opens that artist at
// that source, not NetEase's; with no comments to show, it has no tab for them.
func TestNowPlayingShowsASongOfAnotherSource(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.sources["plug"] = &artistSource{id: "plug"}
	a.pl.Cue([]music.Track{foreignTrack()})
	a.pl.SetLyrics(nil)
	a.nowPlaying = true
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()

	asked := 0
	for _, keys := range []map[string]bool{a.art.busy} {
		for key := range keys {
			asked++
			if !strings.HasPrefix(key, "https://plug.invalid/cover/") || strings.Contains(key, "param=") || strings.Contains(key, "{") {
				t.Errorf("a cover was asked for at %q", key)
			}
		}
	}
	for key := range a.art.art {
		asked++
		if !strings.HasPrefix(key, "https://plug.invalid/cover/") || strings.Contains(key, "param=") {
			t.Errorf("a cover was kept at %q", key)
		}
	}
	if asked == 0 {
		t.Error("no cover was asked for")
	}
	if _, ok := tt.Find("评论"); ok {
		t.Error("a source with no comments has their tab")
	}
	if err := tt.Click("外来歌手"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.nowPlaying || a.route != (route{page: pageArtist, source: "plug", id: "a9"}) {
		t.Errorf("Now Playing open %v, page %+v", a.nowPlaying, a.route)
	}
}
