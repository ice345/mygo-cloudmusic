package netease

import (
	"errors"
	"testing"

	"netease-music-mygo/internal/music"
)

// A playlist comes with its songs, each with its own record, and keeps what
// only NetEase says of it: the liked songs' playlist is the source's own, one
// the listener saved is theirs saved, a chart's update label and its maker's
// number are read back from the playlist's record, and that record has not the
// songs in it, which carry theirs.
func TestSourcePlaylistKeepsItsRecord(t *testing.T) {
	_, done := serveByPath(t, map[string]string{
		"/api/v6/playlist/detail": `{"code":200,"playlist":{"id":7,"name":"飙升榜","coverImgUrl":"http://p/c.jpg","specialType":5,"subscribed":true,` +
			`"updateFrequency":"刚刚更新","playCount":1234,"creator":{"userId":3,"nickname":"他"},"description":"说明",` +
			`"tracks":[{"id":1,"name":"一","dt":1000,"fee":0},{"id":2,"name":"二","dt":2000,"fee":1,"alia":["Two"]}],"trackIds":[{"id":1},{"id":2}]},"privileges":[]}`,
	})
	defer done()
	pl, err := Source{}.Playlist("7")
	if err != nil {
		t.Fatal(err)
	}
	if pl.ID != "7" || pl.Source != SourceID || !pl.Fixed || !pl.Saved || pl.Owner != "他" || pl.PlayCount != 1234 || pl.TrackCount != 2 || len(pl.Tracks) != 2 {
		t.Fatalf("playlist %+v", pl)
	}
	if pl.Tracks[1].ID != "2" || !pl.Tracks[1].Unavailable || pl.Tracks[1].Alias[0] != "Two" {
		t.Errorf("a song: %+v", pl.Tracks[1])
	}
	back := FromPlaylist(*pl)
	if back.UpdateFreq != "刚刚更新" || back.Creator.UserID != 3 || back.SpecialType != 5 || len(back.Tracks) != 2 || back.Tracks[1].Fee != 1 {
		t.Errorf("read back: %+v", back)
	}
	if len(pl.Extra) > 400 {
		t.Errorf("the record holds the songs: %d bytes", len(pl.Extra))
	}
	if _, err := (Source{}).Playlist("x"); err == nil {
		t.Error("a playlist that is no number")
	}
}

// An album comes with its songs, its release date and what it says of itself,
// and an artist with their most popular songs, whether the listener follows
// them, and their NetEase record, with the account their events are of.
func TestSourceAlbumAndArtist(t *testing.T) {
	_, done := serveByPath(t, map[string]string{
		"/api/v1/album/8": `{"code":200,"album":{"id":8,"name":"专辑","picUrl":"http://p/a.jpg","publishTime":1700000000000,"description":"介绍","artists":[{"id":1,"name":"手"}]},` +
			`"songs":[{"id":11,"name":"甲","dt":1000},{"id":12,"name":"乙","dt":1000}]}`,
		"/api/v1/artist/9": `{"code":200,"artist":{"id":9,"name":"某人","img1v1Url":"http://p/sq.jpg","briefDesc":"简介","followed":true,"accountId":77,"musicSize":120,"alias":["Mou"]},` +
			`"hotSongs":[{"id":21,"name":"热一","dt":1000}]}`,
	})
	defer done()
	al, err := Source{}.Album("8")
	if err != nil || al.Album.ID != "8" || al.Album.Description != "介绍" || al.Album.Released[:4] != "2023" || len(al.Tracks) != 2 || al.Tracks[0].ID != "11" || al.Album.Artists[0].Name != "手" {
		t.Fatalf("album %+v, %v", al, err)
	}
	if FromAlbum(al.Album).Description != "介绍" {
		t.Error("the album's record was lost")
	}
	ar, err := Source{}.Artist("9")
	if err != nil || ar.Artist.ID != "9" || !ar.Artist.Following || ar.Artist.Description != "简介" || len(ar.Tracks) != 1 || ar.Tracks[0].ID != "21" {
		t.Fatalf("artist %+v, %v", ar, err)
	}
	if rec := FromArtist(ar.Artist); rec.AccountID != 77 || rec.MusicSize != 120 || rec.Alias[0] != "Mou" {
		t.Errorf("the artist's record: %+v", rec)
	}
}

// Playlists are edited each its own call with its own arguments, a song that is
// in the playlist already is music.ErrDuplicate, and an episode is no song to
// put in one, asking NetEase nothing.
func TestSourceEditsPlaylists(t *testing.T) {
	answers := map[string]string{
		"/api/playlist/create":            `{"code":200,"id":50,"playlist":{"id":50,"name":"新歌单"}}`,
		"/api/playlist/update/name":       `{"code":200}`,
		"/api/playlist/remove":            `{"code":200}`,
		"/api/playlist/manipulate/tracks": `{"code":200}`,
		"/api/playlist/subscribe":         `{"code":200}`,
		"/api/playlist/unsubscribe":       `{"code":200}`,
		"/api/artist/sub":                 `{"code":200}`,
	}
	ask, done := serveByPath(t, answers)
	defer done()
	src := Source{}
	made, err := src.CreatePlaylist("新歌单")
	if err != nil || made.ID != "50" || made.Name != "新歌单" {
		t.Fatalf("created %+v, %v", made, err)
	}
	for _, err := range []error{
		src.RenamePlaylist("50", "改名"), src.DeletePlaylist("50"),
		src.AddToPlaylist("50", []string{"1", "2"}), src.RemoveFromPlaylist("50", []string{"3"}),
		src.SavePlaylist("60", true), src.SavePlaylist("60", false),
		src.FollowArtist("9", true), // by the web's API, which the eapi's record does not show
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	got := ask()
	want := []struct{ path, key, val string }{
		{"/api/playlist/create", "name", "新歌单"},
		{"/api/playlist/update/name", "name", "改名"},
		{"/api/playlist/remove", "ids", "[50]"},
		{"/api/playlist/manipulate/tracks", "trackIds", "[1,2]"},
		{"/api/playlist/manipulate/tracks", "op", "del"},
		{"/api/playlist/subscribe", "id", "60"},
		{"/api/playlist/unsubscribe", "id", "60"},
	}
	if len(got) != len(want) {
		t.Fatalf("%d requests: %+v", len(got), got)
	}
	for i, w := range want {
		if got[i].path != w.path || got[i].data[w.key] != w.val {
			t.Errorf("request %d: %s %v, want %s %s=%s", i, got[i].path, got[i].data, w.path, w.key, w.val)
		}
	}
	if got[3].data["op"] != "add" || got[3].data["pid"] != "50" {
		t.Errorf("an add: %+v", got[3].data)
	}

	if err := src.AddToPlaylist("50", []string{"4@5"}); !errors.Is(err, music.ErrUnsupported) {
		t.Errorf("an episode in a playlist: %v", err)
	}
	if len(ask()) != len(want) {
		t.Error("an episode asked NetEase for something")
	}

	answers["/api/playlist/manipulate/tracks"] = `{"code":502}`
	if err := src.AddToPlaylist("50", []string{"1"}); !errors.Is(err, music.ErrDuplicate) {
		t.Errorf("a song in the playlist already: %v", err)
	}
}
