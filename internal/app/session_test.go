package app

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/egoist/mygo/audio"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

// The session is kept a while after playback changes and every while as
// it plays, and paused nothing more is due. It was kept by a ticker that
// woke the app, and redrew its window, every half minute even at rest.
func TestSessionRestsWhilePaused(t *testing.T) {
	var due []func()
	defer func(after func(time.Duration, func())) { sessionAfter = after }(sessionAfter)
	sessionAfter = func(_ time.Duration, fn func()) { due = append(due, fn) }

	a := newTestApp()
	a.pl.Changed = a.keepSession
	a.cue([]netease.Track{{ID: 7, Name: "宝船", Duration: 200000}})
	out := audio.NewPlayer(&silence{})
	out.SetVolume(0)
	out.Play()
	a.pl.Attach(out)
	defer out.Close()

	a.pl.TogglePlay()
	a.pl.TogglePlay()
	if len(due) != 1 {
		t.Fatalf("%d saves due after pausing and playing", len(due))
	}
	due[0]()
	if len(due) != 2 {
		t.Fatalf("playing, %d saves due after one, want another", len(due)-1)
	}

	a.pl.TogglePlay()
	due[1]()
	if len(due) != 2 {
		t.Errorf("paused, %d more saves due", len(due)-2)
	}
}

// A session kept before there were sources, its tracks in NetEase's own shape
// with their numeric IDs, still restores: its queue, its order and where it
// was. Reading it as one of the new shape found a number where the ID is a
// string, and the listener's queue was dropped without a word.
func TestSessionKeptBeforeSourcesStillRestores(t *testing.T) {
	old := `{"queue":[{"id":1,"name":"一","ar":[{"id":5,"name":"某"}],"al":{"id":6,"name":"专","picUrl":"http://p/x.jpg"},"dt":200000,"fee":8},` +
		`{"id":555,"name":"节目","dt":600000,"programId":777,"podcastId":99}],"order":[1,0],"at":1,"shuffle":true,"repeat":1,"position":95000,"radio":"fm:DEFAULT"}`
	s, ok := decodeSession([]byte(old))
	if !ok || len(s.Queue) != 2 || s.At != 1 || !s.Shuffle || s.Repeat != player.RepeatAll || s.Position != 95000 || s.Radio != "fm:DEFAULT" || s.Order[0] != 1 {
		t.Fatalf("restored %+v, %v", s, ok)
	}
	if s.Queue[0].ID != "1" || s.Queue[1].ID != "555@777" || s.Queue[0].Source != netease.SourceID {
		t.Errorf("tracks %q of %q, %q", s.Queue[0].ID, s.Queue[0].Source, s.Queue[1].ID)
	}
	if ep := netease.FromTrack(s.Queue[1]); ep.ProgramID != 777 || ep.PodcastID != 99 || netease.FromTrack(s.Queue[0]).Album.PicURL != "http://p/x.jpg" {
		t.Errorf("an episode's program and podcast, or a cover, were lost: %+v", ep)
	}

	// Kept again, it is in the new shape, and reads back the same.
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	again, ok := decodeSession(b)
	if !ok || !reflect.DeepEqual(again, s) {
		t.Errorf("kept and read again: %+v, %v", again, ok)
	}
}
