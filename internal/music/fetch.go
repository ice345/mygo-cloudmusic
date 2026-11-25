// Ported from internal/music of egoist/mygo-music (AGPL-3.0, as this
// repository is), but for Agent.

package music

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Agent is who the app says it is when fetching what it plays and shows: the
// browser NetEase's servers are used to, as before there were sources
// (upstream's is "MyGoMusic/1.0").
const Agent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15"

// fetchClient gets covers: they're small, and a slow one shouldn't hold a
// slot for long.
var fetchClient = &http.Client{Timeout: 30 * time.Second}

// maxFetch is the most a cover may take.
const maxFetch = 64 << 20

// ImageHeaders, when set, answers the headers to fetch the picture at u with,
// such as the cookie of a source that keeps its covers behind its session.
// It may answer nil.
var ImageHeaders func(u *url.URL) map[string]string

// SizedImage returns the URL to fetch the picture at u with to show it px
// pixels across: u with its size filled in, where it says it goes with
// {size}, else u itself.
func SizedImage(u string, px int) string {
	if px <= 0 {
		px = placeholderSize
	}
	if !strings.Contains(u, "size") {
		return u
	}
	return sizePlaceholder.ReplaceAllLiteralString(u, strconv.Itoa(px))
}

// placeholderSize is the size a picture whose URL says where its size goes
// is fetched at where nothing asks for one: the most the app shows one at.
const placeholderSize = 600

// sizePlaceholder is where a picture's URL says its size goes, in the URL
// as a source gives it or escaped, as a URL parsed and written again has it.
var sizePlaceholder = regexp.MustCompile(`\{size\}|%7[Bb]size%7[Dd]`)

// Fetch returns what's at url, of the web or a file://, such as a cover.
func Fetch(u string) ([]byte, error) {
	if path, ok := PathOf(u); ok {
		return os.ReadFile(path)
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", Agent)
	var headers map[string]string
	if ImageHeaders != nil {
		headers = ImageHeaders(req.URL)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := HeadersStay(fetchClient, req.URL.Hostname(), headers).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New(res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, maxFetch))
}

// HeadersStay returns client, or, with headers to send, one like it that
// asks a redirect to a host other than host, or one below it, without them,
// so that what a source asks with, such as its token, reaches no one else.
// It's the rule Go keeps for Cookie and Authorization, for every header.
func HeadersStay(client *http.Client, host string, headers map[string]string) *http.Client {
	if len(headers) == 0 {
		return client
	}
	host = strings.ToLower(host)
	c := *client
	c.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if to := strings.ToLower(next.URL.Hostname()); to != host && !strings.HasSuffix(to, "."+host) {
			for k := range headers {
				next.Header.Del(k)
			}
		}
		return nil
	}
	return &c
}

// FileURL is the file:// URL of a path.
func FileURL(path string) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path // C:/Music
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

// PathOf returns the path a file:// URL names, and whether u is one.
//
// A plugin often makes one by joining strings, file://${dir}/a.mp3, which leaves
// in it what a URL would have escaped: spaces, other scripts, and a # or ? or %
// in the name of a folder. So what follows file:// is the path, whole, and its
// escapes are decoded where they decode, as they do in what FileURL makes; a %
// that is no escape stays one. (Parsed as a URL, a # ended the path there, a ?
// began a query, and a % that began no escape failed the whole.)
func PathOf(u string) (string, bool) {
	rest, ok := strings.CutPrefix(u, "file:")
	if !ok {
		return "", false
	}
	if after, ok := strings.CutPrefix(rest, "//"); ok {
		switch {
		case len(after) >= 2 && after[1] == ':' && (after[0]|0x20 >= 'a' && after[0]|0x20 <= 'z'):
			// file://C:\Music\a.mp3, a Windows path joined on as it is.
			return filepath.FromSlash(after), true
		case !strings.HasPrefix(after, "/"):
			// file://host/path: the host is this machine's, whatever it says.
			slash := strings.IndexByte(after, '/')
			if slash < 0 {
				return "", false
			}
			after = after[slash:]
		}
		rest = after
	}
	if !strings.HasPrefix(rest, "/") {
		return "", false
	}
	if strings.Contains(rest, "%") {
		if decoded, err := url.PathUnescape(rest); err == nil {
			rest = decoded
		}
	}
	if runtime.GOOS == "windows" {
		rest = strings.TrimPrefix(rest, "/")
	}
	return filepath.FromSlash(rest), true
}
