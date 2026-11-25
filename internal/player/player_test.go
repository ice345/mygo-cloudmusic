package player

import (
	"io/fs"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/egoist/mygo/audio"

	"netease-music-mygo/internal/music"
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
	p.Cue([]music.Track{{ID: "1", Name: "一", Duration: 600000}})
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
	p.Cue([]music.Track{{ID: "1"}, {ID: "2"}})
	asked := 0
	p.AttachRadio("fm:DEFAULT", func(got func([]music.Track)) {
		asked++
		got([]music.Track{{ID: "2"}, {ID: "3"}})
	})
	if asked != 1 || len(p.Queue()) != 3 || p.Queue()[2].ID != "3" {
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
		st   *music.Stream
		want float64
	}{
		{nil, 1},
		{&music.Stream{}, 1},
		{&music.Stream{Gain: -6.6, Peak: 1}, math.Pow(10, -2.6/20)},
		// Quiet, it is lifted, but not past its peak.
		{&music.Stream{Gain: 3, Peak: 0.8}, 1.25},
		{&music.Stream{Gain: -4, Peak: 0}, 1},
	} {
		if g := normGain(c.st); math.Abs(g-c.want) > 1e-9 {
			t.Errorf("%+v: %v, want %v", c.st, g, c.want)
		}
	}
}

// nlevels are NetEase's levels of audio, the worst first, which the tests
// of the cache keep songs by.
var nlevels = []string{"standard", "higher", "exhigh", "lossless", "hires"}

// The cache gives back what it kept, at the level asked for or a better
// one, and drops what was played longest ago once over its limit.
func TestCache(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir, 400)
	store := func(id string, st *music.Stream, n int) {
		f := c.part(st)
		f.Write(make([]byte, n))
		f.Close()
		c.keep("netease", id, st, nlevels, f.Name(), int64(n))
	}
	store("1", &music.Stream{Quality: "lossless", Type: "flac", Gain: -3}, 100)
	dl, st, ok := c.lookup("netease", "1", "exhigh", nlevels)
	if !ok || dl.size != 100 || st.Type != "flac" || st.Gain != -3 || st.Quality != "lossless" {
		t.Fatalf("lookup: %v %+v", ok, st)
	}
	dl.Close()
	if _, _, ok := c.lookup("netease", "1", "hires", nlevels); ok {
		t.Error("found a lossless song for Hi-Res")
	}
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(dir, "1-lossless.flac"), old, old)
	store("2", &music.Stream{Quality: "exhigh", Type: "mp3"}, 100)
	store("3", &music.Stream{Quality: "exhigh", Type: "mp3"}, 100)
	store("4", &music.Stream{Quality: "exhigh", Type: "mp3"}, 100)
	store("5", &music.Stream{Quality: "exhigh", Type: "mp3"}, 100)
	if _, _, ok := c.lookup("netease", "1", "standard", nlevels); ok {
		t.Error("kept the song played longest ago")
	}
	if dl, _, ok := c.lookup("netease", "5", "standard", nlevels); !ok {
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

// NetEase's songs keep the names they had before there were sources, so what
// was kept plays still, and an episode's is its audio's; any other source's
// are named for the source and a hash of the ID, which may hold anything, so
// that nothing it says leaves the cache's folder or meets another source's.
func TestCacheKeyNamesFiles(t *testing.T) {
	for _, c := range []struct{ source, id, want string }{
		{"netease", "1321033921", "1321033921-exhigh"},
		{"netease", "555@777", "555-exhigh"},
	} {
		if got := cacheKey(c.source, c.id, "exhigh"); got != c.want {
			t.Errorf("%s %q: %q, want %q", c.source, c.id, got, c.want)
		}
	}
	for _, c := range []struct{ source, id string }{
		{"netease", "../x"}, {"netease", "@1"}, {"netease", ""}, {"plug", "1"}, {"plug", "../../x"}, {"plug", "file:///a/b.mp3"},
	} {
		if got := cacheKey(c.source, c.id, "high"); strings.ContainsAny(got, `/\.:`) || !strings.HasSuffix(got, "-high") || len(got) != 32+len("-high") {
			t.Errorf("%s %q: %q", c.source, c.id, got)
		}
	}
	if cacheKey("plug", "1", "high") == cacheKey("other", "1", "high") {
		t.Error("two sources' songs have one name")
	}
}

// Leaving a radio keeps the song playing and those before it, drops those
// it queued after, and asks it for no more.
func TestEndRadio(t *testing.T) {
	p := &Player{at: -1}
	p.Cue([]music.Track{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}})
	p.at, p.track = 1, &p.queue[1]
	asked := 0
	p.AttachRadio("heart", func(got func([]music.Track)) { asked++ })
	p.EndRadio()
	if p.Radio() != "" || p.Current().ID != "2" {
		t.Fatalf("radio %q, current %v", p.Radio(), p.Current())
	}
	if q := p.Queue(); len(q) != 2 || q[0].ID != "1" || q[1].ID != "2" {
		t.Fatalf("queue %v", q)
	}
	p.refill()
	if asked != 0 {
		t.Error("asked the radio after leaving it")
	}
}

// fakeSource is a source whose stream and lyrics are what a test says.
type fakeSource struct {
	music.Unsupported
	stream func(id, quality string) (*music.Stream, error)
	lyrics func(id string) (*music.Lyrics, error)
}

func (*fakeSource) Info() music.Info { return music.Info{ID: "fake", Name: "测试音源"} }
func (f *fakeSource) Stream(id, quality string) (*music.Stream, error) {
	return f.stream(id, quality)
}
func (f *fakeSource) Lyrics(id string) (*music.Lyrics, error) {
	if f.lyrics == nil {
		return nil, music.ErrUnsupported
	}
	return f.lyrics(id)
}

// offline is a player of three tracks whose source's stream is f, for the
// tests of tracks that fail to load; settle runs what it posts until the
// current track has loaded or failed, and until until done says so.
func offline(t *testing.T, f func(string, string) (*music.Stream, error)) (p *Player, settle func(), until func(done func() bool)) {
	t.Helper()
	p, _, until = offlineCh(t, f)
	return p, func() { t.Helper(); until(func() bool { return !p.loading }) }, until
}

// offlineCh is offline with the channel what is posted comes on, and with
// no automatic retries, which the tests of them set.
func offlineCh(t *testing.T, f func(string, string) (*music.Stream, error)) (p *Player, ch chan func(), until func(done func() bool)) {
	t.Helper()
	prevRetries := refusalRetries
	refusalRetries = nil
	t.Cleanup(func() { refusalRetries = prevRetries })
	ch = make(chan func(), 64)
	p = New(func(fn func()) { ch <- fn })
	src := &fakeSource{stream: f}
	p.Source = func(id string) music.Source {
		if id == "fake" {
			return src
		}
		return nil
	}
	p.queue = []music.Track{{ID: "1", Source: "fake", Name: "一"}, {ID: "2", Source: "fake", Name: "二"}, {ID: "3", Source: "fake", Name: "三"}}
	p.order = []int{0, 1, 2}
	until = func(done func() bool) {
		t.Helper()
		for !done() {
			next(t, ch)
		}
	}
	return p, ch, until
}

// runFor runs what is posted, for d.
func runFor(ch chan func(), d time.Duration) {
	end := time.After(d)
	for {
		select {
		case fn := <-ch:
			fn()
		case <-end:
			return
		}
	}
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
	p, settle, _ := offline(t, func(string, string) (*music.Stream, error) {
		asked++
		return nil, dialError()
	})
	p.Restore(Session{Queue: p.queue, Order: p.order, At: 1, Position: 95000})
	settle()
	s, _ := p.Session()
	if p.Current().ID != "2" || s.At != 1 || s.Position != 95000 || p.Playing() || p.Err() == "" {
		t.Fatalf("offline: track %s, at %d, %dms, playing %v, err %q", p.Current().ID, s.At, s.Position, p.Playing(), p.Err())
	}

	p.TogglePlay()
	settle()
	if asked != 2 || p.Current().ID != "2" || p.Position() != 95*time.Second {
		t.Fatalf("play again: asked %d times, track %s at %v", asked, p.Current().ID, p.Position())
	}
}

// A track that can't play is still skipped, and so is one its source won't
// stream; a refusal, like the network, stops on the track it reached, and
// says why in the source's words: the reason, as a risk it saw in the
// network, was dropped for a sentence that said only to try again.
func TestUnplayableSkipsAndRefusalStays(t *testing.T) {
	p, settle, _ := offline(t, func(id string, _ string) (*music.Stream, error) {
		if id == "1" {
			return nil, music.Fail(music.ErrUnavailable, "这首歌需要 VIP 才能播放")
		}
		return nil, music.Fail(music.ErrRefused, "检测到您的网络环境存在风险，请稍后再试")
	})
	p.load(0)
	settle()
	if p.Current().ID != "2" || p.Playing() || !strings.Contains(p.Err(), "测试音源拒绝了播放：检测到您的网络环境存在风险") {
		t.Fatalf("track %s, playing %v, err %q", p.Current().ID, p.Playing(), p.Err())
	}
}

// A track that can't play says why in its source's words, as a song for
// VIPs or an episode to buy, or plainly if the source gave none; one whose
// source is gone is skipped, saying so.
func TestUnavailableSaysWhy(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want string
	}{
		"with a reason": {music.Fail(music.ErrUnavailable, "这期节目需要购买才能收听"), "这期节目需要购买才能收听"},
		"without":       {music.ErrUnavailable, "这首歌无法播放"},
	} {
		p, settle, _ := offline(t, func(string, string) (*music.Stream, error) { return nil, c.err })
		p.queue = p.queue[:1]
		p.order = p.order[:1]
		p.load(0)
		settle()
		if p.Err() != c.want || p.Playing() {
			t.Errorf("%s: err %q, playing %v", name, p.Err(), p.Playing())
		}
	}
	p, settle, _ := offline(t, nil)
	p.Source = func(string) music.Source { return nil }
	p.queue, p.order = p.queue[:1], p.order[:1]
	p.load(0)
	settle()
	if p.Err() != "找不到这首歌的音源" {
		t.Errorf("no source: err %q", p.Err())
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
	p, settle, _ := offline(t, func(id string, _ string) (*music.Stream, error) {
		path := "/cut"
		if id == "1" {
			path = "/gone"
		}
		return &music.Stream{URL: srv.URL + path, Size: 100000}, nil
	})
	p.load(0)
	settle()
	if p.Current().ID != "2" || p.Playing() || p.Err() == "" {
		t.Fatalf("track %s, playing %v, err %q", p.Current().ID, p.Playing(), p.Err())
	}
}

// Lyrics that fail to come are asked for once more, a little later, and
// then the player says they failed, rather than that the song has none;
// asking again fetches them. A failure used to read as a song without
// lyrics, 纯音乐，请欣赏, until the song was played again.
func TestLyricsFailureIsNotNone(t *testing.T) {
	asked, back := 0, false
	prevAgain := lyricsAgain
	defer func() { lyricsAgain = prevAgain }()
	lyricsAgain = time.Millisecond
	p, _, until := offline(t, func(string, string) (*music.Stream, error) { return nil, dialError() })
	p.Source("fake").(*fakeSource).lyrics = func(string) (*music.Lyrics, error) {
		asked++
		if back {
			return &music.Lyrics{LRC: "[00:01.00]回来了"}, nil
		}
		return nil, dialError()
	}
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

// A source with no lyrics at all gives none, as a song without them: not a
// failure to ask again for and offer a retry of.
func TestSourceWithoutLyricsIsNoFailure(t *testing.T) {
	p, _, until := offline(t, func(string, string) (*music.Stream, error) { return nil, dialError() })
	p.load(0)
	until(func() bool { _, ok := p.Lyrics(); return ok })
	if lines, _ := p.Lyrics(); p.LyricsFailed() || len(lines) != 0 {
		t.Fatalf("failed %v, %d lines", p.LyricsFailed(), len(lines))
	}
}

// A seek on a track whose audio has not loaded still tells the app, so
// that the notch's lyrics move to the line there: they stayed on the
// credits, though the progress bar had moved.
func TestSeekBeforeLoadTellsTheApp(t *testing.T) {
	p := &Player{at: -1}
	changed := 0
	p.Changed = func() { changed++ }
	p.Seek(41 * time.Second)
	if changed != 1 || p.pending != 41*time.Second {
		t.Errorf("changed %d times, pending %v", changed, p.pending)
	}
}

// A refusal its source says to try again after is asked again by itself, as many
// times as there are waits, and then the track stays, paused, as it did
// before: the listener had to press play, and the refusals came and went.
func TestRefusalIsAskedForAgainByItself(t *testing.T) {
	var asked atomic.Int32
	p, ch, until := offlineCh(t, func(string, string) (*music.Stream, error) {
		asked.Add(1)
		return nil, music.Fail(music.ErrRefused, "检测到您的网络环境存在风险，请稍后再试")
	})
	refusalRetries = []time.Duration{time.Millisecond, time.Millisecond}
	p.load(0)
	until(func() bool { return asked.Load() == 3 && !p.loading })
	runFor(ch, 40*time.Millisecond)
	if asked.Load() != 3 || p.Current().ID != "1" || p.Playing() || strings.Contains(p.Err(), "自动重试") || !strings.Contains(p.Err(), "网络环境存在风险") {
		t.Fatalf("asked %d times, track %s, playing %v, err %q", asked.Load(), p.Current().ID, p.Playing(), p.Err())
	}
}

// A track restored paused, refused as the app opens, is not asked for by
// itself: nobody wanted it to play.
func TestRestoredTrackIsNotAskedForAfterARefusal(t *testing.T) {
	var asked atomic.Int32
	p, ch, until := offlineCh(t, func(string, string) (*music.Stream, error) {
		asked.Add(1)
		return nil, music.Fail(music.ErrRefused, "检测到您的网络环境存在风险，请稍后再试")
	})
	refusalRetries = []time.Duration{time.Millisecond}
	p.Restore(Session{Queue: p.queue, Order: p.order, At: 1, Position: 95000})
	until(func() bool { return asked.Load() == 1 && !p.loading })
	runFor(ch, 40*time.Millisecond)
	if asked.Load() != 1 || p.Playing() {
		t.Fatalf("asked %d times, playing %v", asked.Load(), p.Playing())
	}
}

// A file a source points to that isn't there says so, and is skipped as a
// song that can't play, not kept as the network's failure, which said to check
// the network for a song that would never play.
func TestMissingFileIsNotTheNetwork(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "不在这里", "tone.mp3")
	p, settle, _ := offline(t, func(string, string) (*music.Stream, error) {
		return &music.Stream{URL: music.FileURL(gone), Type: "mp3"}, nil
	})
	p.queue, p.order = p.queue[:1], p.order[:1]
	p.load(0)
	settle()
	if p.Err() != "找不到这首歌的音频文件" || p.Playing() {
		t.Errorf("err %q, playing %v", p.Err(), p.Playing())
	}
	if stays(&fs.PathError{Op: "open", Path: gone, Err: fs.ErrNotExist}) {
		t.Error("a file that isn't there stays, as a network failure does")
	}
	if !stays(dialError()) {
		t.Error("the network's failure no longer stays")
	}
}
