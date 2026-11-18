package player

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/egoist/mygo/audio"

	"netease-music-mygo/internal/netease"
)

func TestPlayEmptyQueue(t *testing.T) {
	p := &Player{at: -1}
	p.PlayQueue(nil, 0) // an empty playlist's play button
	if p.Current() != nil {
		t.Error("playing from an empty queue")
	}
}

// silence is a long song of nothing.
type silence struct{}

func (silence) SampleRate() int               { return 48000 }
func (silence) Channels() int                 { return 2 }
func (silence) Len() int64                    { return 48000 * 600 }
func (silence) SetPosition(int64) error       { return nil }
func (silence) Read(b []float32) (int, error) { clear(b); return len(b), nil }

// posted collects what the player posts to the main thread, for the test
// to run there.
func posted() (chan func(), *Player) {
	ch := make(chan func(), 16)
	p := New(func(fn func()) { ch <- fn })
	p.Cue([]netease.Track{{ID: 1, Name: "一", Duration: 600000}})
	out := audio.NewPlayer(silence{})
	out.SetVolume(0)
	out.Play()
	p.Attach(out)
	p.fade.attach(out, 1)
	return ch, p
}

func next(t *testing.T, ch chan func()) {
	t.Helper()
	select {
	case fn := <-ch:
		fn()
	case <-time.After(2 * time.Second):
		t.Fatal("nothing posted")
	}
}

// Pausing fades out before the music stops; playing again fades it in.
func TestPauseFades(t *testing.T) {
	ch, p := posted()
	defer p.out.Close()
	p.TogglePlay()
	if p.Playing() || !p.out.Playing() {
		t.Fatal("paused at once, or not at all")
	}
	next(t, ch)
	if p.out.Playing() {
		t.Fatal("still playing after fading out")
	}
	if !p.fade.silent() {
		t.Error("faded out to something")
	}
	p.TogglePlay()
	if !p.Playing() || !p.out.Playing() {
		t.Fatal("did not play again")
	}
	time.Sleep(fadeIn + 50*time.Millisecond)
	p.fade.mu.Lock()
	level := p.fade.level(time.Now())
	p.fade.mu.Unlock()
	if level != 1 {
		t.Errorf("faded in to %v", level)
	}
}

// A seek playing pauses, which fades out over a few milliseconds, and
// plays again once there, its position the one asked for meanwhile.
func TestSeekWithoutClick(t *testing.T) {
	ch, p := posted()
	defer p.out.Close()
	p.Seek(90 * time.Second)
	if p.out.Playing() {
		t.Error("jumped while playing")
	}
	if p.Position() != 90*time.Second {
		t.Errorf("position %v while seeking", p.Position())
	}
	next(t, ch)
	if !p.out.Playing() || p.out.Position() < 90*time.Second {
		t.Errorf("after the seek: playing %v at %v", p.out.Playing(), p.out.Position())
	}
}

// Seeks one after the other, as the progress bar asks when let go, play
// on from the last.
func TestSeekTwicePlaysOn(t *testing.T) {
	ch, p := posted()
	defer p.out.Close()
	p.Seek(90 * time.Second)
	p.Seek(95 * time.Second)
	next(t, ch)
	for len(ch) > 0 {
		next(t, ch)
	}
	time.Sleep(seekGap * 2)
	for len(ch) > 0 {
		next(t, ch)
	}
	if !p.out.Playing() || p.out.Position() < 95*time.Second {
		t.Errorf("after two seeks: playing %v at %v", p.out.Playing(), p.out.Position())
	}
}

// A radio asks for more as its queue nears its end, and adds what is not
// in it already.
func TestRadioRefills(t *testing.T) {
	p := &Player{at: -1}
	p.Cue([]netease.Track{{ID: 1}, {ID: 2}})
	asked := 0
	p.AttachRadio("fm:DEFAULT", func(got func([]netease.Track)) {
		asked++
		got([]netease.Track{{ID: 2}, {ID: 3}})
	})
	if asked != 1 || len(p.Queue()) != 3 || p.Queue()[2].ID != 3 {
		t.Fatalf("asked %d, queue %v", asked, p.Queue())
	}
	if p.Radio() != "fm:DEFAULT" {
		t.Errorf("radio %q", p.Radio())
	}
	// Playing something else ends the radio.
	p.endRadio()
	if p.Radio() != "" || p.feed != nil {
		t.Error("radio still on")
	}
}

func TestNormGain(t *testing.T) {
	for _, c := range []struct {
		st   *netease.Stream
		want float64
	}{
		{nil, 1},
		{&netease.Stream{}, 1},
		{&netease.Stream{Gain: -6.6, Peak: 1}, math.Pow(10, -2.6/20)},
		// Quiet, it is lifted, but not past its peak.
		{&netease.Stream{Gain: 3, Peak: 0.8}, 1.25},
		{&netease.Stream{Gain: -4, Peak: 0}, 1},
	} {
		if g := normGain(c.st); math.Abs(g-c.want) > 1e-9 {
			t.Errorf("%+v: %v, want %v", c.st, g, c.want)
		}
	}
}

// The cache gives back what it kept, at the level asked for or a better
// one, and drops what was played longest ago once over its limit.
func TestCache(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir, 400)
	store := func(id int64, st *netease.Stream, n int) {
		f := c.part(st)
		f.Write(make([]byte, n))
		f.Close()
		c.keep(id, st, f.Name(), int64(n))
	}
	store(1, &netease.Stream{Level: "lossless", Type: "flac", Gain: -3}, 100)
	dl, st, ok := c.lookup(1, "exhigh")
	if !ok || dl.size != 100 || st.Type != "flac" || st.Gain != -3 {
		t.Fatalf("lookup: %v %+v", ok, st)
	}
	dl.Close()
	if _, _, ok := c.lookup(1, "hires"); ok {
		t.Error("found a lossless song for Hi-Res")
	}
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(dir, "1-lossless.flac"), old, old)
	store(2, &netease.Stream{Level: "exhigh", Type: "mp3"}, 100)
	store(3, &netease.Stream{Level: "exhigh", Type: "mp3"}, 100)
	store(4, &netease.Stream{Level: "exhigh", Type: "mp3"}, 100)
	store(5, &netease.Stream{Level: "exhigh", Type: "mp3"}, 100)
	if _, _, ok := c.lookup(1, "standard"); ok {
		t.Error("kept the song played longest ago")
	}
	if dl, _, ok := c.lookup(5, "standard"); !ok {
		t.Error("dropped the song just kept")
	} else {
		dl.Close()
	}
	if n := c.Size(); n > 400 {
		t.Errorf("takes %d of 400", n)
	}
	c.Clear()
	if n := c.Size(); n != 0 {
		t.Errorf("cleared to %d", n)
	}
}

// Leaving a radio keeps the song playing and those before it, drops those
// it queued after, and asks it for no more.
func TestEndRadio(t *testing.T) {
	p := &Player{at: -1}
	p.Cue([]netease.Track{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}})
	p.at, p.track = 1, &p.queue[1]
	asked := 0
	p.AttachRadio("heart", func(got func([]netease.Track)) { asked++ })
	p.EndRadio()
	if p.Radio() != "" || p.Current().ID != 2 {
		t.Fatalf("radio %q, current %v", p.Radio(), p.Current())
	}
	if q := p.Queue(); len(q) != 2 || q[0].ID != 1 || q[1].ID != 2 {
		t.Fatalf("queue %v", q)
	}
	p.refill()
	if asked != 0 {
		t.Error("asked the radio after leaving it")
	}
}

// offline is a player of three tracks whose songURL is f, for the tests
// of tracks that fail to load; settle runs what it posts until the
// current track has loaded or failed, and until until done says so.
func offline(t *testing.T, f func(int64, string) (*netease.Stream, error)) (p *Player, settle func(), until func(done func() bool)) {
	t.Helper()
	prev := songURL
	songURL = f
	t.Cleanup(func() { songURL = prev })
	ch := make(chan func(), 64)
	p = New(func(fn func()) { ch <- fn })
	p.queue = []netease.Track{{ID: 1, Name: "一"}, {ID: 2, Name: "二"}, {ID: 3, Name: "三"}}
	p.order = []int{0, 1, 2}
	until = func(done func() bool) {
		t.Helper()
		for !done() {
			next(t, ch)
		}
	}
	return p, func() { t.Helper(); until(func() bool { return !p.loading }) }, until
}

// dialError is how a request fails with no network.
func dialError() error {
	return &url.Error{Op: "Post", URL: "https://interfacepc.music.163.com/eapi/song/enhance/player/url/v1",
		Err: &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}}
}

// Restoring a session with the network down keeps its track and where it
// was, paused, and play tries that track again. Any load error used to
// skip on, so the player walked the whole queue, stopped on another track
// at 0:00, and saved that over the session.
func TestNetworkErrorKeepsTheTrack(t *testing.T) {
	asked := 0
	p, settle, _ := offline(t, func(int64, string) (*netease.Stream, error) {
		asked++
		return nil, dialError()
	})
	p.Restore(Session{Queue: p.queue, Order: p.order, At: 1, Position: 95000})
	settle()
	s, _ := p.Session()
	if p.Current().ID != 2 || s.At != 1 || s.Position != 95000 || p.Playing() || p.Err() == "" {
		t.Fatalf("offline: track %d, at %d, %dms, playing %v, err %q", p.Current().ID, s.At, s.Position, p.Playing(), p.Err())
	}

	p.TogglePlay()
	settle()
	if asked != 2 || p.Current().ID != 2 || p.Position() != 95*time.Second {
		t.Fatalf("play again: asked %d times, track %d at %v", asked, p.Current().ID, p.Position())
	}
}

// A track that can't play is still skipped, and so is one NetEase won't
// stream; a refusal, like the network, stops on the track it reached.
func TestUnplayableSkipsAndRefusalStays(t *testing.T) {
	p, settle, _ := offline(t, func(id int64, _ string) (*netease.Stream, error) {
		if id == 1 {
			return nil, netease.ErrUnavailable
		}
		return nil, fmt.Errorf("netease: song url: code -460: %w", netease.ErrRefused)
	})
	p.load(0)
	settle()
	if p.Current().ID != 2 || p.Playing() || p.Err() == "" {
		t.Fatalf("track %d, playing %v, err %q", p.Current().ID, p.Playing(), p.Err())
	}
}

// A song that stops coming in before it can be decoded stays, as the
// network's failure; a song its server won't send is skipped. The decoder's
// error used to hide why the song stopped coming in.
func TestCutStreamStays(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gone" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", "100000")
		w.Write([]byte("ID3")) // and then the connection drops
	}))
	defer srv.Close()
	p, settle, _ := offline(t, func(id int64, _ string) (*netease.Stream, error) {
		path := "/cut"
		if id == 1 {
			path = "/gone"
		}
		return &netease.Stream{URL: srv.URL + path, Size: 100000}, nil
	})
	p.load(0)
	settle()
	if p.Current().ID != 2 || p.Playing() || p.Err() == "" {
		t.Fatalf("track %d, playing %v, err %q", p.Current().ID, p.Playing(), p.Err())
	}
}

// Lyrics that fail to come are asked for once more, a little later, and
// then the player says they failed, rather than that the song has none;
// asking again fetches them. A failure used to read as a song without
// lyrics, 纯音乐，请欣赏, until the song was played again.
func TestLyricsFailureIsNotNone(t *testing.T) {
	asked, back := 0, false
	prevFetch, prevAgain := fetchLyrics, lyricsAgain
	defer func() { fetchLyrics, lyricsAgain = prevFetch, prevAgain }()
	fetchLyrics = func(int64) (netease.LyricTexts, error) {
		asked++
		if back {
			return netease.LyricTexts{LRC: "[00:01.00]回来了"}, nil
		}
		return netease.LyricTexts{}, dialError()
	}
	lyricsAgain = time.Millisecond
	p, _, until := offline(t, func(int64, string) (*netease.Stream, error) { return nil, dialError() })
	loaded := func() bool { _, ok := p.Lyrics(); return ok }

	p.load(0)
	until(loaded)
	if lines, _ := p.Lyrics(); asked != 2 || !p.LyricsFailed() || len(lines) != 0 {
		t.Fatalf("asked %d times, failed %v, %d lines", asked, p.LyricsFailed(), len(lines))
	}

	back = true
	p.RetryLyrics()
	if loaded() || p.LyricsFailed() {
		t.Fatal("retrying: still shown as failed")
	}
	until(loaded)
	if lines, _ := p.Lyrics(); asked != 3 || p.LyricsFailed() || len(lines) != 1 {
		t.Fatalf("after retrying: asked %d times, failed %v, %d lines", asked, p.LyricsFailed(), len(lines))
	}
}
