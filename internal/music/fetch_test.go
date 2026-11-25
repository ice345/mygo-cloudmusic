package music

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

// useImageHeaders sets ImageHeaders for a test, putting back what was there.
func useImageHeaders(t *testing.T, f func(*url.URL) map[string]string) {
	was := ImageHeaders
	ImageHeaders = f
	t.Cleanup(func() { ImageHeaders = was })
}

// A picture is asked for with the headers its source gives for it, such as
// the cookie of the session it keeps its covers behind.
func TestFetchWithImageHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "music-token=abc" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte("cover"))
	}))
	defer srv.Close()
	useImageHeaders(t, func(u *url.URL) map[string]string {
		return map[string]string{"Cookie": "music-token=abc"}
	})
	if b, err := Fetch(srv.URL + "/cover?id=1"); err != nil || string(b) != "cover" {
		t.Fatalf("got %q, %v", b, err)
	}
}

// Headers given for a host go no further than it: a redirect to another is
// asked for without them, so a source's token never reaches a stranger.
// Go itself only kept Cookie and Authorization back.
func TestHeadersStayOnTheirHost(t *testing.T) {
	var leaked string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("X-Token")
		w.Write([]byte("cover"))
	}))
	defer other.Close()
	// 127.0.0.1 and localhost are other hosts, as far as headers go.
	port := must(url.Parse(other.URL)).Port()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://localhost:"+port+"/c.jpg", http.StatusFound)
	}))
	defer srv.Close()
	useImageHeaders(t, func(u *url.URL) map[string]string {
		if u.Hostname() != "127.0.0.1" {
			return nil
		}
		return map[string]string{"X-Token": "secret"}
	})
	if _, err := Fetch(srv.URL + "/c.jpg"); err != nil {
		t.Fatal(err)
	}
	if leaked != "" {
		t.Errorf("the redirect took the header along: %q", leaked)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// A picture whose URL says where its size goes, with {size}, as a source
// gives it or escaped, is asked for the size it shows; one shown at no size
// in particular is asked for at the most the app shows one at, and one
// without it as it is.
func TestSizedImageFillsItsPlaceholder(t *testing.T) {
	for _, c := range []struct {
		in   string
		px   int
		want string
	}{
		{"https://img.example.com/a.jpg?param={size}y{size}", 96, "https://img.example.com/a.jpg?param=96y96"},
		{"https://img.example.com/%7Bsize%7D/a.jpg", 240, "https://img.example.com/240/a.jpg"},
		{"https://img.example.com/a.jpg?w={size}", 0, "https://img.example.com/a.jpg?w=600"},
		{"https://img.example.com/a.jpg?size=big", 96, "https://img.example.com/a.jpg?size=big"},
	} {
		if got := SizedImage(c.in, c.px); got != c.want {
			t.Errorf("SizedImage(%q, %d) = %q, want %q", c.in, c.px, got, c.want)
		}
	}
}

// A file:// URL names the path after it, as a plugin that joins the folder it
// is in to a file's name makes it, raw, and as FileURL makes it, escaped: both
// give the same path, whatever the folders are called. A # or ? in a folder's
// name ended the path there, and a % that began no escape failed the URL, which
// then went to the network as a request, and its failure was called the
// network's.
func TestPathOfTakesWhatFollowsFileAsThePath(t *testing.T) {
	dir := "/Users/小明/Library/Application Support/MyGo 云音乐 Dev/plugins/demo"
	for _, name := range []string{"tone.mp3", "C# demos/100% 真的?/a b.mp3", "a%20b/x.mp3"} {
		path := dir + "/" + name
		for how, u := range map[string]string{"raw": "file://" + path, "escaped": FileURL(path)} {
			if how == "raw" && strings.Contains(name, "%20") {
				// What reads as an escape is one.
				continue
			}
			if got, ok := PathOf(u); !ok || got != filepath.FromSlash(path) {
				t.Errorf("%s %q: %q, %v", how, u, got, ok)
			}
		}
	}
	for u, want := range map[string]string{
		"file:///a/b.mp3":               "/a/b.mp3",
		"file:/a/b.mp3":                 "/a/b.mp3",
		"file://localhost/a/b.mp3":      "/a/b.mp3",
		"file:///a/b%20c.mp3":           "/a/b c.mp3",
		"file:///a/%E4%BA%91.mp3":       "/a/云.mp3",
		`file://C:\Music\demo/tone.mp3`: `C:\Music\demo/tone.mp3`,
	} {
		if got, ok := PathOf(u); !ok || got != filepath.FromSlash(want) && got != want {
			t.Errorf("%q: %q, %v", u, got, ok)
		}
	}
	for _, u := range []string{"https://a/b.mp3", "file:", "file://host", "file:relative/a.mp3", "/a/b.mp3"} {
		if got, ok := PathOf(u); ok {
			t.Errorf("%q is no file URL, but was %q", u, got)
		}
	}
}
