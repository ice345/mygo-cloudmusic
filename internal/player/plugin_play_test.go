package player

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/plugin"
)

// A song of a plugin plays through the player as any source's: one it names on
// this machine plays from where it is; one it names on the web is fetched with
// the audio headers its manifest gives, is kept in the cache as the type it
// said, and plays from there the next time without asking the web or the
// plugin again.
func TestAPluginsSongPlays(t *testing.T) {
	const frames = 4410
	wav := wavFile(1, 2, 16, 44100, make([]byte, frames*4))

	var mu sync.Mutex
	fetched, token := 0, ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fetched, token = fetched+1, r.Header.Get("X-Token")
		mu.Unlock()
		w.Write(wav)
	}))
	defer srv.Close()

	dir := t.TempDir()
	folder := filepath.Join(dir, "tone")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"plugin.json": `{"id":"tone","permissions":{"network":["127.0.0.1"]},
			"requestHeaders":[{"hosts":["127.0.0.1"],"kinds":["audio"],"headers":{"X-Token":"secret"}}]}`,
		// The track's ID is where its song is.
		"index.js": `export function stream({ id }) { return { url: id, type: "wav" }; }`,
	} {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	local := filepath.Join(dir, "tone.wav")
	if err := os.WriteFile(local, wav, 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := plugin.Load(folder, filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	cache := NewCache(filepath.Join(dir, "cache"), 1<<30)
	p := &Player{Cache: cache}

	// On this machine.
	_, s, err := p.open(src, music.Track{ID: music.FileURL(local), Source: "tone"}, "high")
	if err != nil {
		t.Fatal(err)
	}
	if s.Len() != frames {
		t.Errorf("from the file: %d frames", s.Len())
	}
	s.Close()
	mu.Lock()
	if cache.Size() != 0 || fetched != 0 {
		t.Errorf("a file on this machine took %d bytes of cache, %d requests", cache.Size(), fetched)
	}
	mu.Unlock()

	// On the web, twice.
	web := music.Track{ID: srv.URL + "/song", Source: "tone"}
	for i := range 2 {
		info, s, err := p.open(src, web, "high")
		if err != nil {
			t.Fatal(err)
		}
		if s.Len() != frames || info.Type != "wav" {
			t.Errorf("open %d: %d frames of %q", i, s.Len(), info.Type)
		}
		// Done once it has all come in and the cache has taken it.
		dl, _ := s.source()
		dl.waitFor(1 << 40)
		s.Close()
	}
	mu.Lock()
	defer mu.Unlock()
	if token != "secret" {
		t.Errorf("the song was asked for with the token %q", token)
	}
	if fetched != 1 {
		t.Errorf("the song was fetched %d times", fetched)
	}
	if n := cache.Size(); n != int64(len(wav)) {
		t.Errorf("the cache keeps %d bytes of a song of %d", n, len(wav))
	}
}

// copyDir copies the folder src, and everything in it, to dst.
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Upstream's demo plugin, copied into the folder of an app named as this one
// is when it runs from source, plays its song through the player: its stream
// is a file:// URL its code joins from the folder it is in, spaces and Chinese
// in the path, and the song is the tone its folder holds.
func TestDemoPluginSongPlaysFromAFolderNamedAsTheAppsIs(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "Application Support", "MyGo 云音乐 Dev", "plugins", "demo")
	copyDir(t, filepath.Join("..", "plugin", "testdata", "demo"), folder)
	src, err := plugin.Load(folder, filepath.Join(filepath.Dir(folder), "..", "plugin-data"))
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	p := &Player{Cache: NewCache(filepath.Join(t.TempDir(), "cache"), 1<<30)}
	// An MP3 says nothing of its length: the track's duration is it.
	_, s, err := p.open(src, music.Track{ID: "t1", Source: "demo", Duration: 3000}, "high")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.SampleRate() != 44100 || s.Len() != 3*44100 {
		t.Errorf("%d Hz, %d frames", s.SampleRate(), s.Len())
	}

	// Without the tone, as the demo was copied before it was bundled, it
	// says what is missing, not that the network failed.
	if err := os.Remove(filepath.Join(folder, "tone.mp3")); err != nil {
		t.Fatal(err)
	}
	_, _, err = p.open(src, music.Track{ID: "t1", Source: "demo"}, "high")
	if !errors.Is(err, fs.ErrNotExist) || stays(err) {
		t.Errorf("a missing tone: %v (stays %v)", err, stays(err))
	}
}
