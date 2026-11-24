package netease

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeNetEase serves the song endpoints of both APIs from one server, and
// points the package at it. eapi and web answer each request with a body;
// the request's header is kept for the test to read.
type fakeNetEase struct {
	eapiCalls, webCalls atomic.Int32
	eapiHeader          atomic.Pointer[http.Header]
}

func serveNetEase(t *testing.T, eapi, web func(call int) string) (*fakeNetEase, func()) {
	t.Helper()
	f := &fakeNetEase{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/eapi/"):
			h := r.Header.Clone()
			f.eapiHeader.Store(&h)
			w.Write([]byte(eapi(int(f.eapiCalls.Add(1)))))
		case strings.HasPrefix(r.URL.Path, "/api/"):
			w.Write([]byte(web(int(f.webCalls.Add(1)))))
		default:
			http.NotFound(w, r)
		}
	}))
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	prevEapi, prevBase, prevWait := eapiDomain, apiBase, songRetryWait
	eapiDomain, apiBase, songRetryWait = srv.URL, srv.URL, 0
	return f, func() {
		eapiDomain, apiBase, songRetryWait = prevEapi, prevBase, prevWait
		srv.Close()
	}
}

const (
	riskBody  = `{"code":-460,"message":"检测到您的网络环境存在风险，请稍后再试"}`
	songBody  = `{"code":200,"data":[{"id":7,"url":"http://m701.music.126.net/7.mp3","br":320000,"size":9,"type":"mp3","level":"exhigh","code":200}]}`
	noURLBody = `{"code":200,"data":[{"id":7,"url":null,"code":404}]}`
)

// The eapi's User-Agent names the same client as the headers it sends: it was
// an iPhone's beside a Windows PC's headers, and NetEase refused every song
// address (-460) as a client lying about itself.
func TestEapiSaysOneClient(t *testing.T) {
	f, done := serveNetEase(t, func(int) string { return songBody }, nil)
	defer done()
	if _, err := SongURL(7, "exhigh"); err != nil {
		t.Fatal(err)
	}
	h := f.eapiHeader.Load()
	ua, cookie := h.Get("User-Agent"), h.Get("Cookie")
	if strings.Contains(ua, "iPhone") || !strings.Contains(ua, "Windows") || !strings.Contains(ua, "NeteaseMusicDesktop/"+eapiClient["appver"]) {
		t.Errorf("User-Agent %q is not the %s client %s", ua, eapiClient["os"], eapiClient["appver"])
	}
	if !strings.Contains(cookie, "os=pc") || !strings.Contains(cookie, "appver="+url.QueryEscape(eapiClient["appver"])) {
		t.Errorf("the header's cookie %q does not say the same client", cookie)
	}
}

// A song NetEase turned away as a risk plays when asked again a moment later.
func TestSongURLAsksAgainAfterARiskRefusal(t *testing.T) {
	f, done := serveNetEase(t, func(call int) string {
		if call == 1 {
			return riskBody
		}
		return songBody
	}, func(int) string { return noURLBody })
	defer done()
	s, err := SongURL(7, "exhigh")
	if err != nil {
		t.Fatal(err)
	}
	if f.eapiCalls.Load() != 2 || f.webCalls.Load() != 0 {
		t.Errorf("asked the eapi %d times and the web %d", f.eapiCalls.Load(), f.webCalls.Load())
	}
	if s.URL != "https://m701.music.126.net/7.mp3" {
		t.Errorf("address %q", s.URL)
	}
}

// With the eapi turning it away twice, the web's whole answer is taken.
func TestSongURLFallsBackToTheWeb(t *testing.T) {
	f, done := serveNetEase(t, func(int) string { return riskBody }, func(int) string { return songBody })
	defer done()
	s, err := SongURL(7, "exhigh")
	if err != nil {
		t.Fatal(err)
	}
	if f.eapiCalls.Load() != 2 || f.webCalls.Load() != 1 || s.URL == "" {
		t.Errorf("eapi %d, web %d, address %q", f.eapiCalls.Load(), f.webCalls.Load(), s.URL)
	}
}

// The web's silence is not the song's: it gives no address for songs the eapi
// streams to a signed-in listener, so a track the eapi refused stays, as
// refused, rather than being skipped as one that can't play.
func TestSongURLKeepsTheRefusalWhenTheWebHasNoAddress(t *testing.T) {
	_, done := serveNetEase(t, func(int) string { return riskBody }, func(int) string { return noURLBody })
	defer done()
	_, err := SongURL(7, "exhigh")
	var refused *RefusedError
	if !errors.As(err, &refused) || refused.Code != -460 || errors.Is(err, ErrUnavailable) {
		t.Fatalf("got %v, want a refusal of -460", err)
	}
}

// A refusal that isn't a risk (a session run out) is not asked again.
func TestSongURLDoesNotRetryOtherRefusals(t *testing.T) {
	f, done := serveNetEase(t, func(int) string { return `{"code":301,"message":"需要登录"}` }, func(int) string { return songBody })
	defer done()
	_, err := SongURL(7, "exhigh")
	if !errors.Is(err, ErrRefused) || f.eapiCalls.Load() != 1 || f.webCalls.Load() != 0 {
		t.Fatalf("%v after %d eapi and %d web calls", err, f.eapiCalls.Load(), f.webCalls.Load())
	}
}
