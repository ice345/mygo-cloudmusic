package app

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

type saveCall struct {
	id   int64
	save bool
}

// nextSave waits for the playlist saved or unsaved next, failing if none
// is asked for.
func nextSave(t *testing.T, calls chan saveCall) saveCall {
	t.Helper()
	select {
	case c := <-calls:
		return c
	case <-time.After(2 * time.Second):
		t.Fatal("NetEase was not asked to save or unsave a playlist")
		return saveCall{}
	}
}

// Clicking 收藏 on someone else's playlist saves it with NetEase, and lists
// it first among the sidebar's saved playlists at once.
func TestSavePlaylist(t *testing.T) {
	calls := make(chan saveCall, 1)
	a := editTestApp()
	edits(t, a).save = func(id string, save bool) error { calls <- saveCall{numericID(id), save}; return nil }
	pl := netease.Playlist{ID: 30, Name: "华语经典", Tracks: netease.FromPlaylist(*a.playlists[nkey(10)]).Tracks}
	pl.Creator.UserID = 2
	a.playlists[nkey(30)] = loadedPlaylist(pl)
	a.route = nroute(pagePlaylist, 30)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("收藏"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := nextSave(t, calls); got != (saveCall{30, true}) {
		t.Errorf("asked NetEase for %+v, want playlist 30 saved", got)
	}
	if len(a.saved) != 2 || a.saved[0].ID != 30 || a.saved[0].Tracks != nil {
		t.Errorf("saved playlists are %+v, want playlist 30 first, without its songs", a.saved)
	}
}

// The listener's own playlist offers no saving.
func TestOwnPlaylistNotSavable(t *testing.T) {
	a := editTestApp()
	pl := a.playlists[nkey(10)]
	rec := netease.PlaylistRecord(*pl)
	rec.Creator.UserID = a.uid()
	pl.Extra, _ = json.Marshal(rec)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if _, ok := tt.Find("收藏"); ok {
		t.Error("offered to save the listener's own playlist")
	}
}

// Choosing 取消收藏 from a saved playlist's menu in the sidebar unsaves it,
// and its page, once loaded, no longer reads saved.
func TestUnsavePlaylistFromSidebar(t *testing.T) {
	calls := make(chan saveCall, 1)
	a := editTestApp()
	edits(t, a).save = func(id string, save bool) error { calls <- saveCall{numericID(id), save}; return nil }
	a.saved[0].Subscribed = true
	a.playlists[nkey(20)] = loadedPlaylist(netease.Playlist{ID: 20, Name: "飙升榜", Subscribed: true})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("飙升榜"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("取消收藏"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
	tt.Frame()
	if got := nextSave(t, calls); got != (saveCall{20, false}) {
		t.Errorf("asked NetEase for %+v, want playlist 20 unsaved", got)
	}
	if len(a.saved) != 0 || a.isSaved(a.playlists[nkey(20)]) {
		t.Errorf("playlist 20 still reads saved: %+v", a.saved)
	}
}
