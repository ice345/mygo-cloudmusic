package player

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"netease-music-mygo/internal/music"
)

// A song a source points to on this machine, as a file://, plays from where
// it is: it is not fetched, nor copied into the cache, and a file that isn't
// there is the song's failure, not a request for the network to answer.
func TestOpenPlaysAFileWhereItIs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tone.wav")
	const frames = 4410
	if err := os.WriteFile(path, wavFile(1, 2, 16, 44100, make([]byte, frames*4)), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := NewCache(filepath.Join(dir, "cache"), 1<<30)
	p := &Player{Cache: cache}
	asked := 0
	src := &fakeSource{stream: func(id, quality string) (*music.Stream, error) {
		asked++
		return &music.Stream{URL: music.FileURL(filepath.Join(dir, id)), Type: "wav", Quality: quality}, nil
	}}

	info, s, err := p.open(src, music.Track{ID: "tone.wav", Source: "fake"}, "high")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if info.Type != "wav" || s.Len() != frames || s.SampleRate() != 44100 {
		t.Errorf("type %q, %d frames at %d", info.Type, s.Len(), s.SampleRate())
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "cache")); len(entries) != 0 || cache.Size() != 0 {
		t.Errorf("the cache took %v", entries)
	}

	if _, _, err := p.open(src, music.Track{ID: "gone.wav", Source: "fake"}, "high"); !os.IsNotExist(err) {
		t.Errorf("a file that isn't there opened with %v", err)
	}
	if asked != 2 {
		t.Errorf("asked the source %d times", asked)
	}
}

// A source that answers a track with no stream at all, rather than an error,
// leaves it unavailable, instead of the player reading from nothing.
func TestOpenWithoutAStreamIsUnavailable(t *testing.T) {
	p := &Player{}
	src := &fakeSource{stream: func(id, quality string) (*music.Stream, error) { return nil, nil }}
	if _, _, err := p.open(src, music.Track{ID: "1", Source: "fake"}, "high"); err != music.ErrUnavailable {
		t.Errorf("opened with %v", err)
	}
}

// A song is asked for with the headers its source says it needs, and a
// redirect to another host goes without them, so that what a source asks
// with, such as its token, reaches no one else.
func TestDownloadSendsItsHeadersOnlyToTheirHost(t *testing.T) {
	var mu sync.Mutex
	got := map[string]string{} // the header each host's server saw
	serve := func(name string, next func(r *http.Request) string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			got[name] = r.Header.Get("X-Token")
			mu.Unlock()
			if to := next(r); to != "" {
				http.Redirect(w, r, to, http.StatusFound)
				return
			}
			w.Write([]byte("the song"))
		}))
	}
	other := serve("other", func(*http.Request) string { return "" })
	defer other.Close()
	u, err := url.Parse(other.URL)
	if err != nil {
		t.Fatal(err)
	}
	port := u.Port()
	home := serve("home", func(*http.Request) string { return "http://localhost:" + port + "/song" })
	defer home.Close()

	// 127.0.0.1 and localhost are other hosts, as far as headers go.
	dl := startDownload(home.URL+"/song", 0, map[string]string{"X-Token": "secret"}, nil, nil)
	defer dl.Close()
	if n := dl.waitFor(1 << 40); n != int64(len("the song")) {
		t.Fatalf("got %d bytes, failed with %v", n, dl.failed())
	}
	mu.Lock()
	defer mu.Unlock()
	if got["home"] != "secret" {
		t.Errorf("the source's own server saw %q", got["home"])
	}
	if got["other"] != "" {
		t.Errorf("the redirect took the header along: %q", got["other"])
	}
}

// A song of any type the player plays is kept in the cache and found again
// by its ID, whichever case its source spelled the type in, and counts
// toward what the cache holds. Only MP3 and FLAC were kept: the others
// played, and were fetched again each time.
func TestCacheKeepsEveryTypeItPlays(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir, 1<<20)
	types := []string{"mp3", "flac", "ogg", "wav", "M4A", "aac"}
	for i, typ := range types {
		st := &music.Stream{Quality: "exhigh", Type: typ}
		f := c.part(st)
		f.Write(make([]byte, 100))
		f.Close()
		c.keep("plug", strconv.Itoa(i), st, nlevels, f.Name(), 100)
	}
	for i, typ := range types {
		dl, st, ok := c.lookup("plug", strconv.Itoa(i), "exhigh", nlevels)
		if !ok {
			t.Errorf("%s: not found", typ)
			continue
		}
		dl.Close()
		if want := strings.ToLower(typ); st.Type != want {
			t.Errorf("%s: kept as %q", typ, st.Type)
		}
	}
	if n := c.Size(); n != int64(len(types))*100 {
		t.Errorf("takes %d bytes, though %d songs of 100 are kept", n, len(types))
	}
}
