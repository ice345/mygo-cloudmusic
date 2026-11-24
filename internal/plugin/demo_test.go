package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"netease-music-mygo/internal/music"
)

// demo loads upstream's demo plugin, which does everything a plugin can, with
// made-up music, keeping its data in a folder of the test's.
func demo(t *testing.T) *Plugin {
	t.Helper()
	SetLanguage("zh-CN")
	p, err := Load(filepath.Join("testdata", "demo"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

// The demo plugin, which was made for upstream's app, is a source here: it says
// what it can do from its exports alone, in the listener's language, and every
// answer comes as the types the views know, with the plugin as its source.
func TestDemoPluginIsASource(t *testing.T) {
	p := demo(t)
	info := p.Info()
	if info.ID != "demo" || info.Name != "演示" || info.Playback != music.PlaybackStream || info.Settings[0].Label != "问候语" {
		t.Errorf("info %+v", info)
	}
	for _, what := range []string{"search", "playlist", "album", "artist", "comments", "lyrics", "stream", "shows", "stations", "signIn", "remotePlay"} {
		if !p.Can(what) {
			t.Errorf("the demo cannot %s", what)
		}
	}
	if p.Can("nosuchthing") {
		t.Error("Can says yes to what is not exported")
	}

	found, err := p.Search("song", music.SearchTracks, 0)
	if err != nil || len(found.Tracks) != 3 || found.Tracks[0].Source != "demo" || found.Tracks[0].Album.Source != "demo" || found.Tracks[0].Artists[0].Source != "demo" {
		t.Fatalf("search: %+v, %v", found, err)
	}
	if next, _ := p.Search("song", music.SearchTracks, 3); len(next.Tracks) != 0 {
		t.Errorf("a page past the end: %d", len(next.Tracks))
	}
	if shows, err := p.Search("show", music.SearchShows, 0); err != nil || len(shows.Shows) != 1 || shows.Shows[0].Source != "demo" {
		t.Errorf("shows: %+v, %v", shows, err)
	}

	home, err := p.Home()
	if err != nil || home.Title != "欢迎" || len(home.Sections) != 4 || home.Sections[0].Tracks[0].Source != "demo" ||
		home.Sections[1].Albums[0].Source != "demo" || home.Sections[2].Shows[0].Source != "demo" || home.Sections[0].MorePage != "discover" {
		t.Fatalf("home: %+v, %v", home, err)
	}
	if err := p.SetSetting("greeting", "你好"); err != nil {
		t.Fatal(err)
	}
	if home, _ := p.Home(); home.Title != "你好" {
		t.Errorf("the home page ignored the setting: %q", home.Title)
	}
	page, err := p.Browse("discover")
	if err != nil || page.Title != "发现" || len(page.Sections[0].Artists) != 1 || page.Sections[0].Artists[0].Source != "demo" {
		t.Errorf("browse: %+v, %v", page, err)
	}
	if _, err := p.Browse("nowhere"); !errors.Is(err, music.ErrUnavailable) || music.Reason(err) != "no such page" {
		t.Errorf("a page that is none: %v (%q)", err, music.Reason(err))
	}

	al, err := p.Album("al1")
	if err != nil || al.Album.Source != "demo" || al.Album.Artists[0].Source != "demo" || len(al.Tracks) != 3 {
		t.Errorf("album: %+v, %v", al, err)
	}
	ar, err := p.Artist("a1")
	if err != nil || ar.Artist.Source != "demo" || ar.Albums[0].Source != "demo" {
		t.Errorf("artist: %+v, %v", ar, err)
	}
	lyrics, err := p.Lyrics("t1")
	if err != nil || lyrics.LRC == "" || lyrics.Translation == "" {
		t.Errorf("lrc lyrics: %+v, %v", lyrics, err)
	}
	words, err := p.Lyrics("t2")
	if err != nil || len(words.Lines) != 1 || len(words.Lines[0].Words) != 3 || words.Lines[0].Words[1].Text != "by " {
		t.Errorf("lyrics by word: %+v, %v", words, err)
	}
	stream, err := p.Stream("t1", "high")
	if err != nil || stream.Type != "mp3" || stream.Quality != "high" || stream.URL != "file://"+filepath.Join("testdata", "demo")+"/tone.mp3" {
		t.Errorf("stream: %+v, %v", stream, err)
	}
	if _, err := p.Stream("nosuch", "high"); !errors.Is(err, music.ErrUnavailable) {
		t.Errorf("a stream of a track that is none: %v", err)
	}
	comments, err := p.Comments("t1", 0)
	if err != nil || len(comments.Comments) != 1 || comments.Comments[0].Likes != 2 || comments.Total != 1 {
		t.Errorf("comments: %+v, %v", comments, err)
	}
	if err := p.LikeComment("t1", "c1", true); err != nil {
		t.Errorf("liking a comment: %v", err)
	}
}

// What the listener changes in the demo plugin, it keeps with host.storage: it
// is there after the plugin starts again, from a file in its own data folder,
// and no other plugin's.
func TestDemoPluginKeepsWhatTheListenerChanges(t *testing.T) {
	data := t.TempDir()
	load := func() *Plugin {
		p, err := Load(filepath.Join("testdata", "demo"), data)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(p.Close)
		return p
	}
	p := load()
	if acc, err := p.Account(); err != nil || acc != nil {
		t.Fatalf("before signing in: %+v, %v", acc, err)
	}
	if acc, err := p.SignIn(); err != nil || acc.ID != "me" || acc.Name != "Demo Listener" {
		t.Fatalf("sign in: %+v, %v", acc, err)
	}
	if err := p.Like("t2", true); err != nil {
		t.Fatal(err)
	}
	if err := p.SubscribeShow("s1", true); err != nil {
		t.Fatal(err)
	}
	made, err := p.CreatePlaylist("夜里听")
	if err != nil || made.Name != "夜里听" || made.Source != "demo" || !made.Mine {
		t.Fatalf("create: %+v, %v", made, err)
	}
	if err := p.AddToPlaylist(made.ID, []string{"t3", "t1", "t2"}); err != nil {
		t.Fatal(err)
	}
	if err := p.MovePlaylistTrack(made.ID, 2, 0); err != nil {
		t.Fatal(err)
	}
	if err := p.RemoveFromPlaylist(made.ID, []string{"t1"}); err != nil {
		t.Fatal(err)
	}
	if err := p.RenamePlaylist(made.ID, "夜里再听"); err != nil {
		t.Fatal(err)
	}
	p.Close()

	again := load()
	if acc, _ := again.Account(); acc == nil || acc.Name != "Demo Listener" {
		t.Errorf("the account was forgotten: %+v", acc)
	}
	liked, err := again.IsLiked([]string{"t1", "t2"})
	if err != nil || liked["t1"] || !liked["t2"] {
		t.Errorf("liked %v, %v", liked, err)
	}
	if likedPage, _ := again.Liked(0); len(likedPage.Tracks) != 1 || likedPage.Tracks[0].ID != "t2" || likedPage.Tracks[0].Source != "demo" {
		t.Errorf("liked songs: %+v", likedPage)
	}
	if shows, _ := again.Shows(); len(shows) != 1 || !shows[0].Subscribed {
		t.Errorf("subscriptions: %+v", shows)
	}
	pl, err := again.Playlist(made.ID)
	var ids []string
	for _, tr := range pl.Tracks {
		ids = append(ids, tr.ID)
	}
	if err != nil || pl.Name != "夜里再听" || !slices.Equal(ids, []string{"t2", "t3"}) {
		t.Errorf("playlist: %+v (%v), %v", pl, ids, err)
	}
	all, _ := again.Playlists()
	if len(all) != 2 || all[0].Source != "demo" {
		t.Errorf("playlists: %+v", all)
	}
	if err := again.DeletePlaylist(made.ID); err != nil {
		t.Fatal(err)
	}
	if all, _ := again.Playlists(); len(all) != 1 {
		t.Errorf("after deleting: %d playlists", len(all))
	}
	if err := again.SignOut(); err != nil {
		t.Fatal(err)
	}
	if acc, _ := again.Account(); acc != nil {
		t.Errorf("signed out, still %+v", acc)
	}
	if _, err := os.Stat(filepath.Join(data, "demo", "storage.json")); err != nil {
		t.Errorf("no storage file in the plugin's data folder: %v", err)
	}
}

// A plugin that was made for a remote player says what it plays, and is told
// what to do.
func TestDemoPluginPlaysRemotely(t *testing.T) {
	p := demo(t)
	if st, err := p.RemoteState(); err != nil || st != nil {
		t.Fatalf("before playing: %+v, %v", st, err)
	}
	if err := p.RemotePlay([]string{"t1", "t2"}, 1, 500); err != nil {
		t.Fatal(err)
	}
	if err := p.RemoteSeek(900); err != nil {
		t.Fatal(err)
	}
	if err := p.RemoteVolume(40); err != nil {
		t.Fatal(err)
	}
	if err := p.RemotePause(); err != nil {
		t.Fatal(err)
	}
	st, err := p.RemoteState()
	if err != nil || st == nil || st.TrackID != "t2" || st.Playing || st.Position != 900 || st.Volume != 40 {
		t.Errorf("state %+v, %v", st, err)
	}
	if devices, err := p.RemoteDevices(); err != nil || len(devices) != 1 || !devices[0].Active {
		t.Errorf("devices %+v, %v", devices, err)
	}
}
