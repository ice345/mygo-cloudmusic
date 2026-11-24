package player

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"netease-music-mygo/internal/music"
)

// Cache keeps the songs played on disk, so that they play again at once,
// and without the network: each as <key>-<level>.mp3 or .flac, named for its
// source and its ID at the source (see cacheKey), with what its stream was
// beside it in <key>-<level>.json. Once they take more than the limit, those
// played longest ago go first.
type Cache struct {
	dir string

	mu    sync.Mutex
	limit int64 // in bytes; 0 keeps nothing
}

// NewCache returns a cache in dir of limit bytes.
func NewCache(dir string, limit int64) *Cache {
	return &Cache{dir: dir, limit: limit}
}

// SetLimit changes how much the cache may take, and drops what is over.
func (c *Cache) SetLimit(limit int64) {
	c.mu.Lock()
	c.limit = limit
	c.mu.Unlock()
	c.prune("")
}

// legacySource is NetEase's ID among the sources: its songs keep the names
// they had before there were sources, <id>-<level>, so that what was kept
// plays still, and is not fetched again. An episode's audio is the
// "<audio>@<program>" ID's first part.
const legacySource = "netease"

// cacheKey names a song's files, whatever the source's IDs look like: they
// are the source's, and may hold anything, a path included.
func cacheKey(source, id, level string) string {
	if source == legacySource {
		if audio, _, _ := strings.Cut(id, "@"); audio != "" && strings.Trim(audio, "0123456789") == "" {
			return audio + "-" + level
		}
	}
	sum := sha256.Sum256([]byte(source + "\x00" + id))
	return hex.EncodeToString(sum[:16]) + "-" + level
}

// songTypes are the formats that are kept.
var songTypes = []string{"mp3", "flac"}

// keepable reports whether a song of level and type ext may be kept: they
// name its file, so only those known, which keep it in the cache's folder.
// levels are the source's, the worst first.
func keepable(levels []string, level, ext string) bool {
	return slices.Contains(levels, level) && slices.Contains(songTypes, ext)
}

// keptMeta is what is kept of a song's stream beside it. Its names are those it
// has always had.
type keptMeta struct {
	Size  int64   `json:"size"`
	Type  string  `json:"type"`
	Level string  `json:"level"`
	Gain  float64 `json:"gain"`
	Peak  float64 `json:"peak"`
}

func (m keptMeta) stream() *music.Stream {
	return &music.Stream{Size: m.Size, Type: m.Type, Quality: m.Level, Gain: m.Gain, Peak: m.Peak}
}

// lookup returns a song kept at level or a better one, open to read, with
// its stream. levels are the source's, the worst first.
func (c *Cache) lookup(source, id, level string, levels []string) (*download, *music.Stream, bool) {
	if c == nil {
		return nil, nil, false
	}
	at := max(slices.Index(levels, level), 0)
	for _, lv := range levels[at:] {
		key := cacheKey(source, id, lv)
		raw, err := os.ReadFile(filepath.Join(c.dir, key+".json"))
		if err != nil {
			continue
		}
		var m keptMeta
		if json.Unmarshal(raw, &m) != nil || !keepable(levels, lv, m.Type) {
			continue
		}
		path := filepath.Join(c.dir, key+"."+m.Type)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		if info, err := f.Stat(); err != nil || info.Size() != m.Size {
			f.Close()
			continue
		}
		// Played now: the last to go.
		now := time.Now()
		os.Chtimes(path, now, now)
		return opened(f, m.Size), m.stream(), true
	}
	return nil, nil, false
}

// download fetches the song of the stream st into a file, to keep once it
// has all come in; or into memory, if the cache would not keep it.
func (c *Cache) download(source, id string, st *music.Stream, levels []string) *download {
	part := c.part(st)
	if part == nil {
		return startDownload(st.URL, st.Size, nil, nil)
	}
	return startDownload(st.URL, st.Size, part, func(path string, n int64) (string, bool) {
		return c.keep(source, id, st, levels, path, n)
	})
}

// part makes a file for a song of the stream st to come into, or returns
// nil if the cache would not keep it. Each has a name of its own: the same
// song played again while it still comes in has another.
func (c *Cache) part(st *music.Stream) *os.File {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	limit := c.limit
	c.mu.Unlock()
	// A song of more than a quarter of it would push out too many.
	if limit <= 0 || st.Size > limit/4 {
		return nil
	}
	if os.MkdirAll(c.dir, 0o700) != nil {
		return nil
	}
	f, err := os.CreateTemp(c.dir, "*.part")
	if err != nil {
		return nil
	}
	return f
}

// keep keeps the song of the stream st, n bytes, from the file part, which
// is closed, and drops what is over the limit then. It returns where the
// song is: kept, or still at part.
func (c *Cache) keep(source, id string, st *music.Stream, levels []string, part string, n int64) (string, bool) {
	c.mu.Lock()
	limit := c.limit
	c.mu.Unlock()
	if n > limit/4 {
		return part, false
	}
	kept := keptMeta{Size: n, Type: st.Type, Level: st.Quality, Gain: st.Gain, Peak: st.Peak}
	if kept.Type == "" {
		kept.Type = "mp3"
	}
	if !keepable(levels, kept.Level, kept.Type) {
		return part, false
	}
	key := cacheKey(source, id, kept.Level)
	raw, _ := json.Marshal(kept)
	path := filepath.Join(c.dir, key+"."+kept.Type)
	if os.Rename(part, path) != nil {
		return part, false
	}
	os.WriteFile(filepath.Join(c.dir, key+".json"), raw, 0o600)
	c.prune(path)
	return path, true
}

// Size returns how many bytes the songs kept take.
func (c *Cache) Size() int64 {
	var n int64
	for _, f := range c.songs() {
		n += f.size
	}
	return n
}

// Clear drops every song kept.
func (c *Cache) Clear() {
	for _, f := range c.songs() {
		c.remove(f.path)
	}
}

type cachedSong struct {
	path string
	size int64
	used time.Time
}

func (c *Cache) songs() []cachedSong {
	entries, _ := os.ReadDir(c.dir)
	var out []cachedSong
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if ext != ".mp3" && ext != ".flac" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, cachedSong{filepath.Join(c.dir, e.Name()), info.Size(), info.ModTime()})
	}
	return out
}

func (c *Cache) remove(path string) {
	os.Remove(path)
	os.Remove(strings.TrimSuffix(path, filepath.Ext(path)) + ".json")
}

// prune drops the songs played longest ago until the rest fit the limit,
// but keep, the song just kept.
func (c *Cache) prune(keep string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dropParts()
	songs := c.songs()
	slices.SortFunc(songs, func(a, b cachedSong) int { return a.used.Compare(b.used) })
	var total int64
	for _, f := range songs {
		total += f.size
	}
	for _, f := range songs {
		if total <= c.limit {
			return
		}
		if f.path != keep {
			c.remove(f.path)
			total -= f.size
		}
	}
}

// dropParts removes the files of songs that never all came in, left by an
// app that quit as they came: one coming in is written to every moment.
func (c *Cache) dropParts() {
	parts, _ := filepath.Glob(filepath.Join(c.dir, "*.part"))
	for _, path := range parts {
		if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) > time.Hour {
			os.Remove(path)
		}
	}
}
