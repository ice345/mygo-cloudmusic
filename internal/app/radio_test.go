package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
)

func TestCommentTime(t *testing.T) {
	now := time.Date(2026, 10, 25, 22, 0, 0, 0, time.Local)
	for _, c := range []struct {
		t    time.Time
		want string
	}{
		{now.Add(-20 * time.Second), "刚刚"},
		{now.Add(-42 * time.Minute), "42分钟前"},
		{now.Add(-5 * time.Hour), "17:00"},
		{now.Add(-23 * time.Hour), "昨天 23:00"},
		{time.Date(2026, 3, 8, 9, 0, 0, 0, time.Local), "3月8日"},
		{time.Date(2021, 4, 10, 9, 0, 0, 0, time.Local), "2021年4月10日"},
	} {
		if got := commentTime(c.t, now); got != c.want {
			t.Errorf("%v: %q, want %q", c.t, got, c.want)
		}
	}
}

// Heart mode playing, its card and Now Playing offer to leave it, which
// keeps the song playing.
func TestExitHeartMode(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.account = &music.Account{ID: "1", Name: "听众"}
	a.cue([]netease.Track{{ID: 1, Name: "晚风"}, {ID: 2, Name: "推荐的歌"}})
	a.attachRadio(heartRadio, func(got func([]music.Track)) {})
	a.route = route{page: pageFM}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("退出心动模式"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.pl.Radio() != "" || numericID(a.current().ID) != 1 || len(a.queued()) != 1 {
		t.Fatalf("radio %q, current %v, queue %d", a.pl.Radio(), a.current(), len(a.queued()))
	}
	if !tt.HasText("开启心动模式") {
		t.Errorf("card still on: %q", tt.Texts())
	}

	a.attachRadio(heartRadio, func(got func([]music.Track)) {})
	a.nowPlaying = true
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.Click("退出心动模式"); err != nil {
		t.Fatal(err)
	}
	if a.pl.Radio() != "" || numericID(a.current().ID) != 1 {
		t.Fatalf("from Now Playing: radio %q, current %v", a.pl.Radio(), a.current())
	}
}

// radioSource is a source with a personal FM and a heart mode, whose songs
// and whose FM trash are what a test says.
type radioSource struct {
	music.Unsupported
	stations []string
	next     func(station string) ([]music.Track, error)
	heart    func(last, start, playlist string) ([]music.Track, error)
	trashed  chan string
}

func (*radioSource) Info() music.Info { return music.Info{ID: netease.SourceID, Name: "电台音源"} }
func (*radioSource) Can(what string) bool {
	return what == "stations" || what == "stationNext"
}
func (s *radioSource) StationNext(id string) ([]music.Track, error) { return s.next(id) }
func (s *radioSource) HeartMode(last, start, playlist string) ([]music.Track, error) {
	return s.heart(last, start, playlist)
}
func (s *radioSource) TrashFM(id string) error { s.trashed <- id; return nil }

// The personal FM asks its source for the next songs of the mode chosen, drops
// those the listener cannot play and asks again if none is left, and plays what
// comes as a radio that asks for more.
func TestPersonalFMAsksItsSource(t *testing.T) {
	asked := make(chan string, 8)
	var calls int
	a := newTestApp()
	a.settings = defaultSettings()
	landed := make(chan func(), 4)
	a.onPost = func(fn func()) { landed <- fn }
	a.sources[netease.SourceID] = &radioSource{next: func(station string) ([]music.Track, error) {
		asked <- station
		calls++
		if calls == 1 {
			return []music.Track{{ID: "1", Source: netease.SourceID, Name: "VIP 的歌", Unavailable: true}}, nil
		}
		return []music.Track{{ID: "2", Source: netease.SourceID, Name: "能听的歌", Duration: 1000}, {ID: "3", Source: netease.SourceID, Name: "也能听", Duration: 1000}}, nil
	}}
	a.fmMode = 1
	a.playFM()
	wait(t, landed, "the songs")()
	if got := wait(t, asked, "the first ask"); got != netease.FMFamiliar {
		t.Errorf("asked for the station %q, want the mode chosen", got)
	}
	if got := wait(t, asked, "the second ask"); got != netease.FMFamiliar {
		t.Errorf("asked again for %q", got)
	}
	if a.pl.Radio() != fmRadio(netease.FMFamiliar) || a.current() == nil || a.current().ID != "2" || len(a.queued()) != 2 {
		t.Errorf("radio %q, playing %+v of %d", a.pl.Radio(), a.current(), len(a.queued()))
	}
}

// Heart mode starts from the song playing, if the listener likes it, asks its
// source for songs like it from their liked songs, and plays it first; with no
// liked songs it says why, and a song skipped on the FM is told to its source.
func TestHeartModeAndFMTrashGoThroughTheSource(t *testing.T) {
	type ask struct{ last, start, playlist string }
	asked := make(chan ask, 4)
	src := &radioSource{trashed: make(chan string, 2), heart: func(last, start, playlist string) ([]music.Track, error) {
		asked <- ask{last, start, playlist}
		return []music.Track{{ID: "9", Source: netease.SourceID, Name: "相似的歌", Duration: 1000}}, nil
	}}
	a := newTestApp()
	a.settings = defaultSettings()
	landed := make(chan func(), 4)
	a.onPost = func(fn func()) { landed <- fn }
	a.sources[netease.SourceID] = src
	a.account = &music.Account{ID: "1", Name: "听众"}

	a.startHeartMode()
	if a.toast != "喜欢一些歌曲后，即可开启心动模式" {
		t.Errorf("with no liked songs: %q", a.toast)
	}
	a.mine = []music.Playlist{{ID: "5", Source: netease.SourceID, Name: "喜欢的音乐", Mine: true, Fixed: true}}
	a.playlists[nkey(5)] = &music.Playlist{ID: "5", Source: netease.SourceID, Tracks: []music.Track{{ID: "4", Source: netease.SourceID, Name: "喜欢的歌", Duration: 1000}}}
	a.startHeartMode()
	wait(t, landed, "the songs")()
	if got := wait(t, asked, "the heart mode"); got != (ask{"4", "4", "5"}) {
		t.Errorf("asked %+v, want a liked song from the liked songs", got)
	}
	if a.pl.Radio() != heartRadio || len(a.queued()) != 2 || a.current().ID != "4" {
		t.Errorf("radio %q, playing %+v of %d", a.pl.Radio(), a.current(), len(a.queued()))
	}

	a.trashFM()
	if got := wait(t, src.trashed, "the trash"); got != "4" {
		t.Errorf("told the source %q was not liked, want 4", got)
	}
	if a.current().ID != "9" {
		t.Errorf("the FM stayed on %q", a.current().ID)
	}
}
