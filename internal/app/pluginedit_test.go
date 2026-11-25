package app

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
)

// plugEdits is a plugin's source whose playlist edits are what a test says, and
// which does only what it says it can.
type plugEdits struct {
	editSource
	can    map[string]bool
	listed chan struct{} // it was asked for the listener's playlists
}

func (*plugEdits) Info() music.Info { return music.Info{ID: "plug", Name: "插件音源"} }
func (s *plugEdits) Can(what string) bool {
	return s.can[what]
}

// Playlists is asked for once an edit changed them, and cannot answer.
func (s *plugEdits) Playlists() ([]music.Playlist, error) {
	if s.listed != nil {
		s.listed <- struct{}{}
	}
	return nil, errors.New("offline")
}

// editsOf makes the plugin's playlist edits those of a fresh plugEdits that can
// do what is named, in an app that shows the plugin, signed in to it, with
// playlists of its own and one of them open.
func pluginEditApp(t *testing.T, can ...string) (*app, *plugEdits, chan func(), *ui.Tester) {
	t.Helper()
	a := newTestApp()
	a.settings = defaultSettings()
	landed := make(chan func(), 8)
	a.onPost = func(fn func()) { landed <- fn }
	src := &plugEdits{editSource: editSource{t: t}, can: map[string]bool{"signIn": true, "playlists": true}, listed: make(chan struct{}, 4)}
	for _, c := range can {
		src.can[c] = true
	}
	a.sources["plug"] = src
	a.active = "plug"
	st := a.stateOf("plug")
	st.asked, st.listsAsked = true, true
	st.account = &music.Account{ID: "7", Name: "小明"}
	st.mine = []music.Playlist{
		{ID: "m1", Source: "plug", Name: "通勤", Mine: true},
		{ID: "m2", Source: "plug", Name: "睡前", Mine: true},
	}
	st.saved = []music.Playlist{{ID: "s1", Source: "plug", Name: "别人的歌单", Saved: true}}
	a.playlists[pageKey{"plug", "m1"}] = &music.Playlist{ID: "m1", Source: "plug", Name: "通勤", Mine: true, Tracks: []music.Track{
		{ID: "t1", Source: "plug", Name: "一首歌", Duration: 200000},
		{ID: "t2", Source: "plug", Name: "另一首", Duration: 200000},
	}}
	a.playlists[pageKey{"plug", "s1"}] = &music.Playlist{ID: "s1", Source: "plug", Name: "别人的歌单", Saved: true, Tracks: []music.Track{
		{ID: "t3", Source: "plug", Name: "收藏里的歌", Duration: 200000},
	}}
	a.route = route{page: pagePlaylist, source: "plug", id: "m1"}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	return a, src, landed, tt
}

// A song of a plugin goes in the playlists the listener made at that plugin,
// with the menu offering those and not the one the song is listed in, and the
// plugin adds it; the sidebar's lists are asked for again after.
func TestPluginSongGoesToItsOwnPlaylists(t *testing.T) {
	type add struct {
		pid   string
		songs []string
	}
	calls := make(chan add, 1)
	a, src, landed, tt := pluginEditApp(t, "addToPlaylist", "createPlaylist")
	src.add = func(pid string, songs []string) error { calls <- add{pid, songs}; return nil }
	if err := tt.RightClick("另一首"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("添加到歌单", "通勤"); err == nil {
		t.Error("offered to add the song to the playlist it is in")
	}
	if err := tt.ChooseMenuItem("添加到歌单", "睡前"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	got := wait(t, calls, "the song to be added")
	if got.pid != "m2" || len(got.songs) != 1 || got.songs[0] != "t2" {
		t.Errorf("added %+v, want song t2 to playlist m2", got)
	}
	wait(t, landed, "the end of the edit")()
	if a.toast != "已添加到歌单" {
		t.Errorf("toast %q", a.toast)
	}
	// The sidebar's lists are asked for again, and kept as they are while the
	// plugin cannot say.
	wait(t, src.listed, "the playlists to be asked for again")
	wait(t, landed, "the end of asking again")()
	if n := len(a.stateOf("plug").mine); n != 2 {
		t.Errorf("the sidebar has %d playlists after the plugin failed to list them", n)
	}
}

// What a plugin cannot do the menus do not offer: a plugin that adds to
// playlists but makes none has no "new playlist", and one that can do neither
// has no playlists in a song's menu at all.
func TestPluginMenusOfferOnlyWhatItCan(t *testing.T) {
	_, src, _, tt := pluginEditApp(t, "addToPlaylist")
	src.add = func(string, []string) error { return nil }
	if err := tt.RightClick("另一首"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("添加到歌单", "新建歌单…"); err == nil {
		t.Error("offered to make a playlist at a plugin that cannot")
	}
	if err := tt.ChooseMenuItem("添加到歌单", "睡前"); err != nil {
		t.Errorf("not offered the playlists it can add to: %v", err)
	}

	_, _, _, none := pluginEditApp(t)
	if err := none.RightClick("另一首"); err != nil {
		t.Fatal(err)
	}
	if err := none.ChooseMenuItem("添加到歌单", "睡前"); err == nil {
		t.Error("offered playlists at a plugin that cannot add to them")
	}
	for _, item := range none.Menu() {
		if item == "添加到歌单" {
			t.Errorf("an empty submenu of playlists at a plugin that has none to offer: %q", none.Menu())
		}
	}
}

// A playlist the listener made at a plugin is deleted from the sidebar's menu
// once they say yes, by its plugin and with its ID, and is gone from the
// sidebar at once; and a plugin that cannot delete offers no way to.
func TestPluginPlaylistIsDeletedAtItsPlugin(t *testing.T) {
	calls := make(chan string, 1)
	_, src, landed, tt := pluginEditApp(t, "deletePlaylist")
	src.del = func(id string) error { calls <- id; return nil }
	if err := tt.RightClick("睡前"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("重命名…"); err == nil {
		t.Error("offered to rename at a plugin that cannot")
	}
	if err := tt.ChooseMenuItem("删除歌单…"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Frame()
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	if id := wait(t, calls, "the playlist to be deleted"); id != "m2" {
		t.Errorf("deleted %q", id)
	}
	wait(t, landed, "the end of the edit")()
	tt.Frame()
	if tt.HasText("睡前") || !tt.HasText("已删除歌单") {
		t.Errorf("deleted, but the window shows %v", tt.Texts())
	}

	_, _, _, none := pluginEditApp(t)
	if err := none.RightClick("睡前"); err == nil && len(none.Menu()) > 0 {
		t.Errorf("a menu for a playlist that can't be edited: %q", none.Menu())
	}
}

// Someone else's playlist at a plugin is saved from its page, with the plugin,
// and the listener's own is not offered to them; a plugin that cannot save has
// no button.
func TestPluginPlaylistIsSavedAtItsPlugin(t *testing.T) {
	calls := make(chan bool, 1)
	a, src, landed, tt := pluginEditApp(t, "savePlaylist")
	src.save = func(id string, on bool) error {
		if id != "other" {
			t.Errorf("saved %q", id)
		}
		calls <- on
		return nil
	}
	a.playlists[pageKey{"plug", "other"}] = &music.Playlist{ID: "other", Source: "plug", Name: "路人的歌单", Tracks: []music.Track{{ID: "t9", Source: "plug", Name: "某首歌", Duration: 1000}}}
	a.route = route{page: pagePlaylist, source: "plug", id: "other"}
	tt.Frame()
	if err := tt.Click("收藏"); err != nil {
		t.Fatal(err)
	}
	if on := wait(t, calls, "the playlist to be saved"); !on {
		t.Error("asked to unsave")
	}
	wait(t, landed, "the end of the edit")()
	if a.toast != "已收藏" || len(a.stateOf("plug").saved) != 2 {
		t.Errorf("toast %q, saved %d", a.toast, len(a.stateOf("plug").saved))
	}

	a.route = route{page: pagePlaylist, source: "plug", id: "m1"}
	tt.Frame()
	if err := tt.Click("收藏"); err == nil {
		t.Error("offers to save the listener's own playlist")
	}
	_, _, _, none := pluginEditApp(t)
	none.Frame()
	if err := none.Click("收藏"); err == nil {
		t.Error("a button to save at a plugin that cannot")
	}
}

// heartSource is a plugin's source whose hearts are what a test says.
type heartSource struct {
	music.Unsupported
	like func(id string, on bool) error
}

func (*heartSource) Info() music.Info { return music.Info{ID: "plug", Name: "插件音源"} }
func (s *heartSource) Can(what string) bool {
	return what == "like" || what == "signIn"
}
func (s *heartSource) Like(id string, on bool) error { return s.like(id, on) }

// A heart on a plugin's song is the plugin's: signed in, it likes the song with
// the plugin; known to be signed out, it asks the listener to sign in to the
// plugin, not to NetEase, and likes nothing; and where the plugin was not asked
// who is signed in, and says it needs a sign-in, the heart goes back and the
// listener is asked, with no toast of a failure.
func TestHeartOfAPluginsSong(t *testing.T) {
	song := music.Track{ID: "t1", Source: "plug", Name: "一首歌"}
	key := likeKey(song)
	calls := make(chan bool, 2)
	var answer error
	setup := func() (*app, chan func()) {
		a := newTestApp()
		a.settings = defaultSettings()
		landed := make(chan func(), 4)
		a.onPost = func(fn func()) { landed <- fn }
		a.sources["plug"] = &heartSource{like: func(id string, on bool) error { calls <- on; return answer }}
		return a, landed
	}

	a, landed := setup()
	a.stateOf("plug").asked = true
	a.stateOf("plug").account = &music.Account{ID: "7", Name: "小明"}
	a.toggleLike(song)
	if on := wait(t, calls, "the like"); !on || !a.liked[key] {
		t.Fatalf("liked: asked %v, shown %v", on, a.liked[key])
	}
	wait(t, landed, "its end")()

	a, _ = setup()
	a.stateOf("plug").asked = true
	a.toggleLike(song)
	if !a.login.open || a.login.source != "plug" || a.liked[key] {
		t.Fatalf("signed out: the dialog %+v, liked %v", a.login, a.liked[key])
	}
	select {
	case <-calls:
		t.Error("liked a song while signed out")
	default:
	}

	a, landed = setup()
	answer = music.ErrSignIn
	a.toggleLike(song)
	wait(t, calls, "the like")
	wait(t, landed, "its refusal")()
	if a.liked[key] || !a.login.open || a.login.source != "plug" || a.toast != "" {
		t.Errorf("refused: liked %v, dialog %+v, toast %q", a.liked[key], a.login, a.toast)
	}
}
