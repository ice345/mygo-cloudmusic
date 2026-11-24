package netease

import (
	"errors"
	"reflect"
	"testing"

	"netease-music-mygo/internal/music"
)

// A song is liked as a song and an episode through its program, each by its
// own call: an episode's audio is a song's ID, and liking it as one filled
// the listener's liked songs with a podcast's.
func TestSourceLikesSongsAndEpisodesEachItsOwnWay(t *testing.T) {
	ask, done := serveByPath(t, map[string]string{
		"/api/radio/like":      `{"code":200}`,
		"/api/resource/like":   `{"code":200}`,
		"/api/resource/unlike": `{"code":200}`,
	})
	defer done()
	src := Source{}
	if err := src.Like("12", true); err != nil {
		t.Fatal(err)
	}
	if err := src.LikeEpisode("12@34", true); err != nil {
		t.Fatal(err)
	}
	if err := src.LikeEpisode("12@34", false); err != nil {
		t.Fatal(err)
	}
	got := ask()
	if len(got) != 3 || got[0].path != "/api/radio/like" || got[0].data["trackId"] != "12" || got[0].data["like"] != "true" {
		t.Fatalf("a song: %+v", got)
	}
	if got[1].path != "/api/resource/like" || got[1].data["threadId"] != "A_DJ_1_34" || got[2].path != "/api/resource/unlike" {
		t.Errorf("an episode: %+v %+v", got[1], got[2])
	}
	// Each the other's way is not done, and asks NetEase nothing.
	if err := src.Like("12@34", true); !errors.Is(err, music.ErrUnsupported) {
		t.Errorf("an episode liked as a song: %v", err)
	}
	if err := src.LikeEpisode("12", true); !errors.Is(err, music.ErrUnsupported) {
		t.Errorf("a song liked as an episode: %v", err)
	}
	if len(ask()) != 3 {
		t.Errorf("%d requests, want 3", len(ask()))
	}
}

// Which tracks are liked: songs by the listener's liked songs, which need them
// signed in, and episodes by asking after each; a list of episodes alone does
// not ask for the songs.
func TestSourceSaysWhichAreLiked(t *testing.T) {
	ask, done := serveByPath(t, map[string]string{
		"/api/nuser/account/get":         `{"code":200,"profile":{"userId":9,"nickname":"我"}}`,
		"/api/song/like/get":             `{"code":200,"ids":[12,99]}`,
		"/api/resource/commentInfo/list": `{"code":200,"data":[{"resourceId":34,"liked":true},{"resourceId":35,"liked":false}]}`,
	})
	defer done()
	got, err := Source{}.IsLiked([]string{"12", "13", "12@34", "14@35"})
	want := map[string]bool{"12": true, "13": false, "12@34": true, "14@35": false}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("liked %v, %v", got, err)
	}
	if programs := ask()[len(ask())-1]; programs.path != "/api/resource/commentInfo/list" || programs.data["resourceIds"] != "[34,35]" {
		t.Errorf("asked after the episodes: %+v", programs)
	}

	before := len(ask())
	got, err = Source{}.IsLiked([]string{"12@34"})
	if err != nil || !got["12@34"] || len(ask()) != before+1 {
		t.Errorf("episodes alone: %v, %v, %d more requests", got, err, len(ask())-before)
	}
}

// Songs liked, signed out, say to sign in rather than that none is liked.
func TestSourceNeedsSignInToSayWhichSongsAreLiked(t *testing.T) {
	_, done := serveByPath(t, map[string]string{"/api/nuser/account/get": `{"code":200,"profile":null}`})
	defer done()
	if _, err := (Source{}).IsLiked([]string{"12"}); !errors.Is(err, music.ErrSignIn) {
		t.Errorf("signed out: %v", err)
	}
}
