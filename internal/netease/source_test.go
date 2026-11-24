package netease

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"netease-music-mygo/internal/music"
)

// A track goes to the app's shape and back whole, and through the JSON the
// queue and the session keep it in, with what the app's shape has no place
// for: the fee, the privilege, the program of an episode and its podcast.
func TestTrackSurvivesTheAppsShape(t *testing.T) {
	for name, want := range map[string]Track{
		"song": {
			ID: 1321033921, Name: "So Hot", Duration: 164000, Fee: 8, Alias: []string{"alias"},
			Artists:   []Artist{{ID: 7, Name: "A"}, {ID: 8, Name: "B"}},
			Album:     Album{ID: 9, Name: "Alb", PicURL: "http://p1.music.126.net/x.jpg"},
			Privilege: &Privilege{ID: 1321033921, St: 0, Pl: 320000},
			Reason:    "like it",
		},
		"episode": {
			ID: 555, ProgramID: 777, PodcastID: 99, Name: "第一期", Duration: 600000,
			Description: "about it", Album: Album{Name: "电台", PicURL: "https://p2.music.126.net/y.jpg"},
		},
	} {
		mt := ToTrack(want)
		b, err := json.Marshal(mt)
		if err != nil {
			t.Fatal(err)
		}
		var kept music.Track
		if err := json.Unmarshal(b, &kept); err != nil {
			t.Fatal(err)
		}
		if got := FromTrack(kept); !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got  %+v\n want %+v", name, got, want)
		}
		if mt.Source != SourceID || mt.Name != want.Name || mt.Duration != want.Duration {
			t.Errorf("%s: the app's track says %+v", name, mt)
		}
	}
}

// An episode is told from a song by its ID, which holds the program's too,
// and a cover asks the image CDN for the size the app shows it at.
func TestEpisodeIDAndCover(t *testing.T) {
	ep := ToTrack(Track{ID: 555, ProgramID: 777, PodcastID: 99, Album: Album{PicURL: "http://p/x.jpg"}})
	if ep.ID != "555@777" || ep.ShowID != "99" {
		t.Errorf("episode %q of show %q", ep.ID, ep.ShowID)
	}
	if got := music.SizedImage(ep.Album.ImageURL, 600); got != "https://p/x.jpg?param=600y600" {
		t.Errorf("cover %q", got)
	}
	for id, want := range map[string][2]int64{"12": {12, 0}, "555@777": {555, 777}} {
		a, p, err := parseTrackID(id)
		if err != nil || a != want[0] || p != want[1] {
			t.Errorf("%q: %d %d %v", id, a, p, err)
		}
	}
	for _, id := range []string{"", "x", "0", "-5", "5@", "5@x", "@5"} {
		if _, _, err := parseTrackID(id); err == nil {
			t.Errorf("%q is a track", id)
		}
	}
}

// What NetEase says of a track's stream reaches the app as the kinds it acts
// on: a refusal stays, in NetEase's words; a song that cannot play says why,
// as a song or as an episode; the stream is NetEase's, level and all.
func TestSourceStreamSaysWhatHappened(t *testing.T) {
	_, done := serveNetEase(t, func(int) string { return songBody }, func(int) string { return noURLBody })
	s, err := Source{}.Stream("7", "exhigh")
	done()
	if err != nil || s.URL != "https://m701.music.126.net/7.mp3" || s.Type != "mp3" || s.Quality != "exhigh" {
		t.Fatalf("stream %+v, %v", s, err)
	}

	_, done = serveNetEase(t, func(int) string { return riskBody }, func(int) string { return noURLBody })
	_, err = Source{}.Stream("7", "exhigh")
	done()
	if !errors.Is(err, music.ErrRefused) || music.Reason(err) != "检测到您的网络环境存在风险，请稍后再试" {
		t.Errorf("refusal: %v (%q)", err, music.Reason(err))
	}

	_, done = serveNetEase(t, func(int) string { return noURLBody }, nil)
	_, song := Source{}.Stream("7", "exhigh")
	_, episode := Source{}.Stream("7@8", "exhigh")
	done()
	if !errors.Is(song, music.ErrUnavailable) || !errors.Is(episode, music.ErrUnavailable) || music.Reason(song) == music.Reason(episode) || music.Reason(song) == "" {
		t.Errorf("unavailable: %q, %q", music.Reason(song), music.Reason(episode))
	}
}

// A track's lyrics come as lines built from NetEase's texts.
func TestSourceLyricsAreBuiltFromNetEasesTexts(t *testing.T) {
	defer swapLyrics(
		func(id int64) (LyricTexts, error) {
			if id != 12 {
				t.Errorf("asked for %d", id)
			}
			return LyricTexts{LRC: "[00:01.00]一\n[00:04.00]二", Translation: "[00:01.00]one"}, nil
		},
		func(int64) (LyricTexts, error) { return LyricTexts{}, errors.New("not asked") },
	)()
	l, err := Source{}.Lyrics("12")
	if err != nil || len(l.Lines) != 2 || l.Lines[0].Text != "一" || l.Lines[0].Translation != "one" || l.Lines[0].Start != 1000 {
		t.Fatalf("lyrics %+v, %v", l, err)
	}
}

// A search comes back in the app's shapes, of the one kind asked for: a song
// with its record, an artist's square photo, an album's release date, a
// playlist's plays and maker, a podcast as a show; its page says whether
// there is another, and a server that claims more while sending none has no
// next page.
func TestSourceSearchFindsEachKind(t *testing.T) {
	// One answer for each search, in the order they are made, as NetEase
	// answers: with the kind searched for alone.
	answers := []string{
		`{"code":200,"result":{"songs":[{"id":5,"name":"歌","ar":[{"id":1,"name":"手"}],"al":{"id":2,"name":"专","picUrl":"http://p/c.jpg"},"dt":1000,"fee":0}],"songCount":130}}`,
		`{"code":200,"result":{"artists":[{"id":7,"name":"某人","img1v1Url":"http://p/sq.jpg","picUrl":"http://p/wide.jpg"},{"id":8,"name":"无方图","picUrl":"http://p/wide2.jpg"}],"artistCount":2}}`,
		`{"code":200,"result":{"albums":[{"id":9,"name":"专辑","picUrl":"http://p/a.jpg","artists":[{"id":1,"name":"手"}],"publishTime":1700000000000}],"albumCount":1}}`,
		`{"code":200,"result":{"playlists":[{"id":11,"name":"单","coverImgUrl":"http://p/pl.jpg","trackCount":52,"playCount":12345,"creator":{"userId":3,"nickname":"他"}}],"playlistCount":1}}`,
		`{"code":200,"result":{"djRadios":[{"id":13,"name":"电台","picUrl":"http://p/r.jpg","desc":"说","programCount":40,"dj":{"userId":4,"nickname":"主播"}}],"djRadiosCount":1}}`,
	}
	_, done := serveNetEase(t, func(int) string { return `{"code":200,"data":[]}` }, func(call int) string { return answers[call-1] })
	defer done()
	src := Source{}

	res, err := src.Search("q", music.SearchTracks, 60)
	if err != nil || len(res.Tracks) != 1 || res.Tracks[0].ID != "5" || res.Tracks[0].Name != "歌" || res.Total != 130 || !res.More {
		t.Fatalf("tracks %+v, %v", res, err)
	}
	if FromTrack(res.Tracks[0]).Artists[0].Name != "手" {
		t.Error("the song's record was lost")
	}
	res, _ = src.Search("q", music.SearchArtists, 0)
	if len(res.Artists) != 2 || res.Artists[0].ImageURL != "https://p/sq.jpg?param={size}y{size}" || res.Artists[1].ImageURL != "https://p/wide2.jpg?param={size}y{size}" {
		t.Errorf("artists %+v", res.Artists)
	}
	res, _ = src.Search("q", music.SearchAlbums, 0)
	if a := res.Albums[0]; a.ID != "9" || a.Artists[0].Name != "手" || len(a.Released) != 10 || a.Released[:4] != "2023" {
		t.Errorf("album %+v", a)
	}
	res, _ = src.Search("q", music.SearchPlaylists, 0)
	if p := res.Playlists[0]; p.ID != "11" || p.Owner != "他" || p.TrackCount != 52 || p.PlayCount != 12345 || p.ImageURL != "https://p/pl.jpg?param={size}y{size}" {
		t.Errorf("playlist %+v", p)
	}
	res, _ = src.Search("q", music.SearchShows, 0)
	if s := res.Shows[0]; s.ID != "13" || s.Publisher != "主播" || s.EpisodeCount != 40 || s.Description != "说" {
		t.Errorf("show %+v", s)
	}
	if _, err := src.Search("q", "users", 0); !errors.Is(err, music.ErrUnsupported) {
		t.Errorf("a kind NetEase's source does not search: %v", err)
	}
}

// A page of a search that comes back empty ends it, whatever total the server
// claims, and the last page that fills the total has no next.
func TestSourceSearchEndsWhereItsPagesDo(t *testing.T) {
	_, done := serveNetEase(t, func(int) string { return `{"code":200,"data":[]}` }, func(int) string {
		return `{"code":200,"result":{"albums":[],"albumCount":500}}`
	})
	res, err := Source{}.Search("q", music.SearchAlbums, 120)
	done()
	if err != nil || res.More || res.Total != 500 {
		t.Errorf("empty page of a claimed 500: %+v, %v", res, err)
	}
	_, done = serveNetEase(t, func(int) string { return `{"code":200,"data":[]}` }, func(int) string {
		return `{"code":200,"result":{"albums":[{"id":1,"name":"a"},{"id":2,"name":"b"}],"albumCount":62}}`
	})
	res, _ = Source{}.Search("q", music.SearchAlbums, 60)
	done()
	if res.More {
		t.Error("a next page after the last")
	}
}
