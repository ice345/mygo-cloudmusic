package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
)

// The count on the comments' tab holds the last song's while the next
// one's loads, for it to roll from one to the other, and goes once the
// next song turns out to have none.
func TestCommentCountHoldsWhileLoading(t *testing.T) {
	a, tt, _ := coverApp(t)
	a.comments = commentState{track: ntrack(2), loaded: true, total: 128}
	tt.Frame()
	a.pl.Next()
	a.comments = commentState{track: ntrack(3), loading: true}
	tt.Frame()
	if _, ok := tt.Find("评论 128"); !ok {
		t.Fatal("the count went as the next song's loads")
	}
	a.comments.loading, a.comments.loaded, a.comments.total = false, true, 96
	tt.Frame()
	if _, ok := tt.Find("评论 96"); !ok {
		t.Error("the count did not change once loaded")
	}
	a.pl.Next()
	a.comments = commentState{track: ntrack(1), loaded: true}
	tt.Advance(time.Second)
	tt.Frame()
	if _, ok := tt.Find("评论"); !ok {
		t.Error("a song without comments shows a count")
	}
}

// Now Playing shows an episode's comments, the episode's and not its
// audio's, under a tab of their own beside what it is about, which stays
// chosen as the frames go.
func TestEpisodeComments(t *testing.T) {
	asked := make(chan string, 4)
	a := newTestApp()
	a.sources[netease.SourceID] = &commentSource{comments: func(id string, offset int) (*music.CommentPage, error) {
		asked <- id
		return &music.CommentPage{}, nil
	}}
	a.settings = defaultSettings()
	a.cue([]netease.Track{{ID: 13, Name: "第一期", ProgramID: 3, PodcastID: 5, Duration: 600000}})
	a.pl.SetLyrics(nil)
	a.nowPlaying = true
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Advance(time.Second)
	tt.Frame()
	if id := wait(t, asked, "the comments"); id != "13@3" {
		t.Fatalf("asked for the comments of %s, want the episode 13@3", id)
	}
	if a.npPane != paneProgram {
		t.Fatalf("opened on pane %d", a.npPane)
	}
	if err := tt.Click("评论"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if a.npPane != paneComments {
		t.Errorf("on pane %d", a.npPane)
	}
}

// 赞 on a comment likes it at once, counting the listener in where it is
// hot and where it is latest, and asks NetEase to; again, it unlikes it.
func TestLikeComment(t *testing.T) {
	type call struct {
		track, comment string
		like           bool
	}
	calls := make(chan call, 4)
	a, tt, _ := coverApp(t)
	a.sources[netease.SourceID] = &commentSource{
		comments: func(string, int) (*music.CommentPage, error) { return &music.CommentPage{}, nil },
		like:     func(track, comment string, on bool) error { calls <- call{track, comment, on}; return nil },
	}
	a.account = &music.Account{ID: "1"}
	cm := music.Comment{ID: "9", Text: "好听", Likes: 41}
	a.comments = commentState{track: ntrack(2), loaded: true, total: 1, hot: []music.Comment{cm}, latest: []music.Comment{cm}}
	a.npPane = paneComments
	tt.Frame()
	if err := tt.Click("赞"); err != nil {
		t.Fatal(err)
	}
	if c := wait(t, calls, "the like"); c != (call{"2", "9", true}) {
		t.Errorf("asked NetEase for %+v", c)
	}
	for _, cm := range []music.Comment{a.comments.hot[0], a.comments.latest[0]} {
		if !cm.Liked || cm.Likes != 42 {
			t.Fatalf("liked %v by %d", cm.Liked, cm.Likes)
		}
	}
	tt.Frame()
	if err := tt.Click("取消赞"); err != nil {
		t.Fatal(err)
	}
	if c := wait(t, calls, "the unlike"); c.like {
		t.Error("liked again")
	}
	for _, cm := range []music.Comment{a.comments.hot[0], a.comments.latest[0]} {
		if cm.Liked || cm.Likes != 41 {
			t.Errorf("unliked: liked %v by %d", cm.Liked, cm.Likes)
		}
	}
}

// The comments of the track playing are asked of its source, a page at a time
// from where the last ended; the hot ones come on the first page only, and a
// comment that shifted onto the next page is not listed twice.
func TestCommentsLoadByPagesFromTheSource(t *testing.T) {
	type ask struct {
		id     string
		offset int
	}
	asks := make(chan ask, 4)
	pages := []*music.CommentPage{
		{Total: 3, More: true,
			Hot:      []music.Comment{{ID: "h", Text: "热评"}},
			Comments: []music.Comment{{ID: "1", Text: "一"}, {ID: "2", Text: "二"}}},
		{Total: 3, Comments: []music.Comment{{ID: "2", Text: "二"}, {ID: "3", Text: "三"}}},
	}
	a, _, _ := coverApp(t)
	landed := make(chan func(), 4)
	a.onPost = func(fn func()) { landed <- fn }
	a.sources[netease.SourceID] = &commentSource{comments: func(id string, offset int) (*music.CommentPage, error) {
		asks <- ask{id, offset}
		if offset == 0 {
			return pages[0], nil
		}
		return pages[1], nil
	}}
	a.comments = commentState{}
	a.commentsFor()
	wait(t, landed, "the first page")()
	if got := wait(t, asks, "the first ask"); got != (ask{"2", 0}) {
		t.Errorf("asked %+v", got)
	}
	cs := &a.comments
	if !cs.loaded || len(cs.hot) != 1 || len(cs.latest) != 2 || cs.total != 3 || !cs.more {
		t.Fatalf("after the first page: %+v", cs)
	}
	a.loadComments()
	wait(t, landed, "the second page")()
	if got := wait(t, asks, "the second ask"); got != (ask{"2", 2}) {
		t.Errorf("asked %+v, want the next page from 2", got)
	}
	if len(cs.latest) != 3 || cs.latest[2].ID != "3" || len(cs.hot) != 1 || cs.more {
		t.Errorf("after the second page: %+v", cs)
	}
}

// A page that arrives after the song changed is not shown among the next
// song's comments.
func TestCommentsOfTheLastSongAreNotShown(t *testing.T) {
	release := make(chan struct{})
	a, _, _ := coverApp(t)
	landed := make(chan func(), 4)
	a.onPost = func(fn func()) { landed <- fn }
	a.sources[netease.SourceID] = &commentSource{comments: func(id string, offset int) (*music.CommentPage, error) {
		<-release
		return &music.CommentPage{Total: 1, Comments: []music.Comment{{ID: "x", Text: "上一首的"}}}, nil
	}}
	a.comments = commentState{}
	a.commentsFor()
	a.pl.Next()
	a.comments = commentState{track: ntrack(3)}
	close(release)
	wait(t, landed, "the late page")()
	if len(a.comments.latest) != 0 || a.comments.loaded {
		t.Errorf("the last song's comments are the next's: %+v", a.comments)
	}
}

// A song of a source with no comments has no tab for them, and Now Playing
// left on them goes to the queue; a source that has them but cannot like
// them shows how many liked a comment, with nothing to press.
func TestSourceWithoutCommentsOrLikes(t *testing.T) {
	a, tt, _ := coverApp(t)
	a.sources[netease.SourceID] = &commentSource{}
	a.comments = commentState{}
	a.npPane = paneComments
	tt.Frame()
	tt.Frame()
	if _, ok := tt.Find("评论"); ok {
		t.Error("a source with no comments has a tab for them")
	}
	if a.npPane != paneQueue {
		t.Errorf("left on pane %d", a.npPane)
	}

	a.sources[netease.SourceID] = &commentSource{comments: func(string, int) (*music.CommentPage, error) { return &music.CommentPage{}, nil }}
	cm := music.Comment{ID: "9", Text: "好听", Likes: 41}
	a.comments = commentState{track: ntrack(2), loaded: true, total: 1, latest: []music.Comment{cm}}
	a.npPane = paneComments
	tt.Frame()
	tt.Frame()
	if _, ok := tt.Find("赞"); ok {
		t.Error("a thumb to press on a source that cannot like comments")
	}
	if _, ok := tt.Find("好听"); !ok {
		t.Error("the comment is not listed")
	}
}
