package netease

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const podcastJSON = `{"id":5,"name":"某播客","picUrl":"http://p/pod.jpg","desc":"说明","category":"文化","dj":{"userId":9,"nickname":"主播"},` +
	`"programCount":42,"subCount":1200,"playCount":99999,"subed":true,"lastProgramName":"最新一期","rcmdtext":"值得一听"}`

// A podcast comes with a page of its episodes, the newest first, as tracks: an
// episode is told by its audio and its program, says which show it is of and the
// day it came out, and one with no audio is unavailable; the show keeps what
// only NetEase says of it, and says whether there is more.
func TestSourceShowAndItsEpisodes(t *testing.T) {
	var mu sync.Mutex
	var offsets []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/djradio/v2/get":
			w.Write([]byte(`{"code":200,"data":` + podcastJSON + `}`))
		case "/api/dj/program/byradio":
			mu.Lock()
			offsets = append(offsets, r.URL.Query().Get("offset"))
			mu.Unlock()
			w.Write([]byte(`{"code":200,"more":true,"programs":[` +
				`{"id":3,"name":"第三期","coverUrl":"","duration":600000,"createTime":1700000000000,"description":"讲了些事","mainSong":{"id":13}},` +
				`{"id":2,"name":"第二期","coverUrl":"http://p/ep.jpg","duration":500000,"createTime":1690000000000,"mainSong":{"id":0}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	prev := apiBase
	apiBase = srv.URL
	defer func() { apiBase = prev }()
	page, err := Source{}.Show("5", 30)
	if err != nil || len(page.Episodes) != 2 || !page.More {
		t.Fatalf("%+v, %v", page, err)
	}
	sh := page.Show
	if sh.ID != "5" || sh.Source != SourceID || sh.Name != "某播客" || sh.Publisher != "主播" || sh.EpisodeCount != 42 || !sh.Subscribed ||
		sh.Category != "文化" || sh.Subscribers != 1200 || sh.PlayCount != 99999 || sh.Latest != "最新一期" || sh.Reason != "值得一听" {
		t.Errorf("show %+v", sh)
	}
	if PodcastRecord(sh).DJ.UserID != 9 {
		t.Errorf("the host's number was lost: %+v", PodcastRecord(sh).DJ)
	}
	e := page.Episodes[0]
	if e.ID != "13@3" || e.ShowID != "5" || e.Name != "第三期" || e.Description != "讲了些事" || e.Duration != 600000 || e.Unavailable ||
		len(e.Published) != 10 || !strings.HasPrefix(e.Published, "2023-11-1") || e.Album.Name != "某播客" || len(e.Artists) != 1 || e.Artists[0].Name != "主播" {
		t.Errorf("episode %+v", e)
	}
	if !strings.HasPrefix(e.Album.ImageURL, "https://p/pod.jpg") {
		t.Errorf("an episode with no cover shows %q, want the show's", e.Album.ImageURL)
	}
	if !strings.HasPrefix(page.Episodes[1].Album.ImageURL, "https://p/ep.jpg") || !page.Episodes[1].Unavailable {
		t.Errorf("an episode with no audio: %+v", page.Episodes[1])
	}
	mu.Lock()
	defer mu.Unlock()
	if len(offsets) != 1 || offsets[0] != "30" {
		t.Errorf("episodes asked from %v, want 30", offsets)
	}
	if _, err := (Source{}).Show("x", 0); err == nil {
		t.Error("a podcast that is no number")
	}
}

// The listener's subscriptions are their shows, and subscribing to one, or
// ending it, goes to NetEase by its number.
func TestSourceSubscriptions(t *testing.T) {
	asked, done := serveByPath(t, map[string]string{
		"/api/djradio/get/subed": `{"code":200,"hasMore":false,"djRadios":[` + podcastJSON + `]}`,
		"/api/djradio/sub":       `{"code":200}`,
		"/api/djradio/unsub":     `{"code":200}`,
	})
	defer done()
	shows, err := Source{}.Shows()
	if err != nil || len(shows) != 1 || shows[0].ID != "5" || shows[0].Latest != "最新一期" {
		t.Fatalf("%+v, %v", shows, err)
	}
	if err := (Source{}).SubscribeShow("5", true); err != nil {
		t.Fatal(err)
	}
	if err := (Source{}).SubscribeShow("5", false); err != nil {
		t.Fatal(err)
	}
	if err := (Source{}).SubscribeShow("x", true); err == nil {
		t.Error("subscribed to a podcast that is no number")
	}
	var paths []string
	for _, a := range asked() {
		paths = append(paths, a.path)
	}
	if !strings.Contains(strings.Join(paths, " "), "/api/djradio/sub") || !strings.Contains(strings.Join(paths, " "), "/api/djradio/unsub") {
		t.Errorf("asked %v", paths)
	}
}
