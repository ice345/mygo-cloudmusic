package netease

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"netease-music-mygo/internal/music"
)

func TestQRLoginLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	// A device of the test's own, out of the keychain.
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	key, err := QRKey()
	if err != nil {
		t.Fatal(err)
	}
	code, msg, _, _, err := QRCheck(key)
	if err != nil || code != QRWaiting {
		t.Fatalf("check: %d %q %v", code, msg, err)
	}
	t.Logf("key %s: %d %s", key, code, msg)
}

func TestSongURLEapiLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	var res map[string]any
	err := eapiPost("/api/song/enhance/player/url/v1", map[string]any{"ids": "[2652820720]", "level": "exhigh", "encodeType": "mp3"}, &res)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(res)
	t.Log(string(b)[:min(400, len(b))])
}

func TestPrivilegesLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	pl, err := PlaylistDetail(3778678)
	if err != nil {
		t.Fatal(err)
	}
	with := 0
	for _, tr := range pl.Tracks {
		if tr.Privilege != nil {
			with++
		}
	}
	daily, err := DailySongs()
	if err != nil {
		t.Fatal(err)
	}
	dailyWith := 0
	for _, tr := range daily {
		if tr.Privilege != nil {
			dailyWith++
		}
	}
	t.Logf("playlist: %d of %d tracks with privileges; daily: %d of %d", with, len(pl.Tracks), dailyWith, len(daily))
	if with == 0 || dailyWith == 0 {
		t.Fatal("no privileges")
	}
	st, err := SongURL(2652820720, "exhigh")
	if err != nil || st.URL == "" || st.Size == 0 || st.Type != "mp3" {
		t.Fatalf("stream %+v %v", st, err)
	}
}

func TestRecentPlaysLive(t *testing.T) {
	// The session of a development build, which has the listener's plays.
	path := os.Getenv("NM_SESSION")
	if os.Getenv("LIVE") == "" || path == "" {
		t.Skip()
	}
	useSession(t, path)
	plays, err := RecentPlays(20)
	if err != nil || len(plays) == 0 {
		t.Fatalf("%d plays, %v", len(plays), err)
	}
	for _, p := range plays[:min(5, len(plays))] {
		t.Logf("%s %s %q playable %v", p.At.Format("01-02 15:04"), p.Device, p.Track.Name, p.Track.Playable())
	}
	if plays[0].Track.ID == 0 || plays[0].Device == "" || plays[0].At.Year() < 2020 {
		t.Fatalf("latest play %+v", plays[0])
	}
}

var lrcStamp = regexp.MustCompile(`\[(\d+):(\d+)(?:\.(\d+))?\]`)

// sameTimedLines reports whether two lyric answers sing the same lines.
// The desktop client writes a credit's time with three decimals and no
// space before its colon; the web writes two or three, and spaces around
// the colon. Neither changes the line.
func sameTimedLines(a, b string) bool {
	as, bs := canonLines(a), canonLines(b)
	if len(as) != len(bs) {
		return false
	}
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

func canonLines(s string) []string {
	var out []string
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		stamps := lrcStamp.FindAllStringSubmatchIndex(line, -1)
		if len(stamps) == 0 {
			out = append(out, line)
			continue
		}
		var b strings.Builder
		for _, m := range stamps {
			min, _ := strconv.Atoi(line[m[2]:m[3]])
			sec, _ := strconv.Atoi(line[m[4]:m[5]])
			ms := (min*60 + sec) * 1000
			if m[6] >= 0 {
				f := line[m[6]:m[7]]
				n, _ := strconv.Atoi(f)
				scale := 1000
				for range len(f) {
					scale /= 10
				}
				ms += n * scale
			}
			b.WriteString(strconv.Itoa(ms))
			b.WriteByte(' ')
		}
		text := strings.TrimSpace(line[stamps[len(stamps)-1][1]:])
		b.WriteString(strings.Join(strings.Fields(text), ""))
		out = append(out, b.String())
	}
	return out
}

// useSession signs in with the session kept at path.
func useSession(t *testing.T, path string) {
	deviceOnce.Do(func() {
		deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123"
		// NM_DEVICE is the app's own device-id file, for the device NetEase
		// knows the session by.
		if b, err := os.ReadFile(os.Getenv("NM_DEVICE")); err == nil && len(b) == 52 {
			deviceID = string(b)
		}
	})
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cookies []*http.Cookie
	for _, part := range strings.Split(strings.TrimSpace(string(b)), ";") {
		if name, value, ok := strings.Cut(strings.TrimSpace(part), "="); ok {
			cookies = append(cookies, &http.Cookie{Name: name, Value: value, Path: "/"})
		}
	}
	httpClient.Jar.SetCookies(siteURL, cookies)
}

func TestVIPSongPlayableLive(t *testing.T) {
	// A subscriber's VIP songs play: the web API called Stan taken down
	// (st -100) for a subscriber, and greyed it out, though the eapi
	// streamed it whole. NM_SESSION must be a subscriber's.
	path := os.Getenv("NM_SESSION")
	if os.Getenv("LIVE") == "" || path == "" {
		t.Skip()
	}
	useSession(t, path)
	songs, err := SongDetails([]int64{17572422})
	if err != nil || len(songs) != 1 {
		t.Fatalf("%d songs, %v", len(songs), err)
	}
	if !songs[0].Playable() {
		t.Fatalf("Stan not playable: %+v", songs[0].Privilege)
	}
}

func TestArtistDetailLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	ar, err := ArtistDetail(12138269)
	if err != nil {
		t.Fatal(err)
	}
	if ar.Name == "" || ar.Avatar == "" || len(ar.Songs) == 0 || ar.Songs[0].Privilege == nil {
		t.Fatalf("artist %+v with %d songs", ar.Artist, len(ar.Songs))
	}
	t.Logf("%s: %d songs, %d albums, first %q", ar.Name, ar.MusicSize, ar.AlbumSize, ar.Songs[0].Name)
}

func TestArtistLatestSongsLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	// The brilliant green's come in the old shape, 毛不易's in the new.
	for _, id := range []int64{21823, 12138269} {
		songs, err := ArtistLatestSongs(id, 100)
		if err != nil || len(songs) == 0 {
			t.Fatalf("artist %d: %d songs, %v", id, len(songs), err)
		}
		s := songs[0]
		if len(s.Artists) == 0 || s.Album.PicURL == "" || s.Duration == 0 || s.Privilege == nil {
			t.Fatalf("artist %d: newest song %+v", id, s)
		}
		t.Logf("artist %d: %d songs, newest %q", id, len(songs), s.Name)
	}
}

// Each kind of search finds something for a common query, and says how
// many it found in all; podcasts come 10 at most, on one page.
func TestSearchPageLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	for _, kind := range []SearchType{SearchSongs, SearchAlbums, SearchArtists, SearchPlaylists, SearchUsers, SearchPodcasts} {
		res, err := SearchPage("音乐", kind, 5, 0)
		if err != nil || res.Len() == 0 || res.Total < res.Len() {
			t.Fatalf("kind %d: %d of %d, %v", kind, res.Len(), res.Total, err)
		}
	}
	s, err := SearchSuggest("周杰")
	if err != nil || len(s.Songs) == 0 || len(s.Songs[0].Artists) == 0 || len(s.Albums) == 0 || len(s.Albums[0].Artists) == 0 {
		t.Fatalf("suggestions %+v, %v", s, err)
	}
}

// An album's page has its artists and songs in the shape Track reads; an
// artist's albums page on.
func TestAlbumLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	al, err := AlbumDetail(32311)
	if err != nil || al.Name == "" || len(al.Artists) == 0 || al.PublishTime == 0 || len(al.Songs) == 0 {
		t.Fatalf("album %+v, %v", al, err)
	}
	if s := al.Songs[0]; len(s.Artists) == 0 || s.Album.PicURL == "" || s.Duration == 0 || s.Privilege == nil {
		t.Fatalf("first song %+v", s)
	}
	albums, more, err := ArtistAlbums(6452, 5, 0)
	if err != nil || len(albums) != 5 || !more || albums[0].PicURL == "" {
		t.Fatalf("%d albums, more %v, %v", len(albums), more, err)
	}
}

// An artist with an account of their own has its events, a page at a
// time, and a podcast has its episodes, each with audio to play.
func TestEventsAndPodcastLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	ar, err := ArtistDetail(5781)
	if err != nil || ar.AccountID == 0 {
		t.Fatalf("artist %+v, %v", ar, err)
	}
	events, last, _, err := Events(ar.AccountID, 5, -1)
	if err != nil || len(events) == 0 || last == 0 || events[0].User.Nickname == "" {
		t.Fatalf("%d events, last %d, %v", len(events), last, err)
	}
	res, err := SearchPage("故事", SearchPodcasts, 1, 0)
	if err != nil || len(res.Podcasts) == 0 {
		t.Fatalf("podcasts %+v, %v", res, err)
	}
	pod, err := PodcastDetail(res.Podcasts[0].ID)
	if err != nil || pod.Name == "" || pod.ProgramCount == 0 {
		t.Fatalf("podcast %+v, %v", pod, err)
	}
	programs, _, err := PodcastPrograms(pod.ID, 5, 0)
	if err != nil || len(programs) == 0 || programs[0].MainSong.ID == 0 {
		t.Fatalf("%d episodes, %v", len(programs), err)
	}
}

// The podcasts tab's lists come a page at a time, with names and covers.
func TestPodcastListsLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	rec, err := RecommendedPodcasts()
	if err != nil || len(rec) == 0 || rec[0].Name == "" {
		t.Fatalf("%d recommended, %v", len(rec), err)
	}
	hot, more, err := HotPodcasts(5, 0)
	if err != nil || len(hot) != 5 || !more || hot[0].PicURL == "" {
		t.Fatalf("%d hot, more %v, %v", len(hot), more, err)
	}
	next, _, err := HotPodcasts(5, 5)
	if err != nil || len(next) == 0 || next[0].ID == hot[0].ID {
		t.Fatalf("next page %+v, %v", next, err)
	}
}

// An episode has comments of its own, not its audio's, and NetEase answers
// of each episode asked about whether the listener likes it.
func TestProgramCommentsLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	res, err := SearchPage("故事", SearchPodcasts, 1, 0)
	if err != nil || len(res.Podcasts) == 0 {
		t.Fatalf("podcasts %+v, %v", res, err)
	}
	pod := &res.Podcasts[0]
	programs, _, err := PodcastPrograms(pod.ID, 2, 0)
	if err != nil || len(programs) < 2 {
		t.Fatalf("%d episodes, %v", len(programs), err)
	}
	tr := programs[0].Track(pod)
	page, err := ThreadComments(tr.Thread(), 0, 5)
	if err != nil || page.Total == 0 || len(page.Latest) == 0 {
		t.Fatalf("comments %+v, %v", page, err)
	}
	ids := []int64{programs[0].ID, programs[1].ID}
	liked, err := ProgramsLiked(ids)
	if err != nil || len(liked) != 2 {
		t.Fatalf("liked %v, %v", liked, err)
	}
	if _, ok := liked[ids[1]]; !ok {
		t.Errorf("no answer of %d: %v", ids[1], liked)
	}
}

// Credit lines match across the two answers when only the spaces and the
// decimals differ, and a changed line does not.
func TestSameTimedLinesIgnoresCreditSpacing(t *testing.T) {
	web := "[00:00.00] 作词 : 某人\n[00:23.64]一句\n"
	desk := "[00:00.000]作词: 某人\n[00:23.64]一句\n"
	if !sameTimedLines(web, desk) {
		t.Fatal("the credit did not match")
	}
	if sameTimedLines(web, "[00:00.000]作词: 某人\n[00:23.64]另一句\n") {
		t.Fatal("a changed line matched")
	}
}

// The desktop client's timed lines and translation match the web's, for
// the songs whose readings the app relies on. NM_SESSION, when set, is a
// signed-in session file, as the other live tests take it.
func TestLyricTextsLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	if path := os.Getenv("NM_SESSION"); path != "" {
		useSession(t, path)
	}
	for _, id := range []int64{536622304, 2120465975, 65766, 3427209483} {
		web, werr := fetchWebLyrics(id)
		desk, derr := fetchDesktopLyrics(id)
		if werr != nil || derr != nil {
			t.Errorf("%d: web %v, desktop %v", id, werr, derr)
			continue
		}
		if web.Translation != desk.Translation || !sameTimedLines(web.LRC, desk.LRC) {
			t.Errorf("%d: desktop timed lines or translation differ from the web", id)
		}
		if web.Reading != desk.Reading {
			t.Errorf("%d: desktop reading differs from the web (desktop %d bytes, web %d)", id, len(desk.Reading), len(web.Reading))
		}
	}
	for _, id := range []int64{536622304, 2120465975, 65766} {
		texts, err := Lyrics(id)
		if err != nil || strings.TrimSpace(texts.Reading) == "" {
			t.Errorf("%d: reading %q, %v", id, texts.Reading, err)
		}
	}
	texts, err := Lyrics(3427209483)
	if err != nil || strings.TrimSpace(texts.YReading) == "" {
		t.Errorf("3427209483: y reading %q, %v", texts.YReading, err)
	}
}

// A signed-in listener is given a free song's address every time they ask,
// and the audio it names answers: NetEase turned the request away as a risk
// (-460) when the eapi's User-Agent and headers named two clients. NM_SESSION
// is a signed-in session file, and NM_DEVICE the device-id file beside it.
func TestSongURLSignedInLive(t *testing.T) {
	if os.Getenv("LIVE") == "" || os.Getenv("NM_SESSION") == "" {
		t.Skip()
	}
	useSession(t, os.Getenv("NM_SESSION"))
	found, err := Search("轻音乐 钢琴", 30)
	if err != nil {
		t.Fatal(err)
	}
	var song *Track
	for i := range found {
		if found[i].Fee == 0 || found[i].Fee == 8 {
			song = &found[i]
			break
		}
	}
	if song == nil {
		t.Skip("no free song in the search")
	}
	var stream *Stream
	for i := range 5 {
		stream, err = SongURL(song.ID, "exhigh")
		if errors.Is(err, ErrUnavailable) {
			t.Skipf("song %d is not for this account", song.ID)
		}
		if err != nil {
			t.Fatalf("ask %d: %v", i, err)
		}
		time.Sleep(time.Second)
	}
	req, _ := http.NewRequest("GET", stream.URL, nil)
	req.Header.Set("Range", "bytes=0-1023")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
		t.Fatalf("the audio answered %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "audio") && !strings.Contains(ct, "octet-stream") {
		t.Errorf("the audio's type is %q", ct)
	}
}

// The source searches every kind it does, and a second page continues from
// the first: what came as the app's own shapes has what the views draw.
func TestSourceSearchLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	src := Source{}
	for _, kind := range []music.SearchKind{music.SearchTracks, music.SearchArtists, music.SearchAlbums, music.SearchPlaylists, music.SearchShows} {
		res, err := src.Search("音乐", kind, 0)
		if err != nil || res.Len() == 0 || res.Total < res.Len() {
			t.Fatalf("%s: %d of %d, %v", kind, res.Len(), res.Total, err)
		}
		first := res.Tracks
		if kind != music.SearchTracks {
			continue
		}
		if first[0].ID == "" || first[0].Name == "" || len(first[0].Artists) == 0 || first[0].Album.ImageURL == "" {
			t.Errorf("a song with nothing to show: %+v", first[0])
		}
		next, err := src.Search("音乐", kind, res.Len())
		if err != nil || len(next.Tracks) == 0 || next.Tracks[0].ID == first[0].ID {
			t.Errorf("the next page: %d songs, %v", len(next.Tracks), err)
		}
	}
	if res, err := src.Search("音乐", music.SearchArtists, 0); err == nil && (res.Artists[0].Name == "" || res.Artists[0].ImageURL == "") {
		t.Errorf("an artist with nothing to show: %+v", res.Artists[0])
	}
}

// The source loads a playlist, an album and an artist that a search found, each
// with songs to play and what its page shows.
func TestSourcePagesLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	deviceOnce.Do(func() { deviceID = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123" })
	src := Source{}
	found := func(kind music.SearchKind) *music.SearchResult {
		res, err := src.Search("周杰伦", kind, 0)
		if err != nil || res.Len() == 0 {
			t.Fatalf("%s: %v", kind, err)
		}
		return res
	}
	pl, err := src.Playlist(found(music.SearchPlaylists).Playlists[0].ID)
	if err != nil || len(pl.Tracks) == 0 || pl.Name == "" || pl.ImageURL == "" {
		t.Errorf("playlist: %d songs, %v", len(pl.Tracks), err)
	}
	al, err := src.Album(found(music.SearchAlbums).Albums[0].ID)
	if err != nil || len(al.Tracks) == 0 || al.Album.Name == "" || len(al.Album.Artists) == 0 {
		t.Errorf("album: %+v, %v", al, err)
	}
	ar, err := src.Artist(found(music.SearchArtists).Artists[0].ID)
	if err != nil || len(ar.Tracks) == 0 || ar.Artist.Name == "" {
		t.Errorf("artist: %+v, %v", ar, err)
	}
}
