package netease

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// A song's comments are NetEase's thread of the song and an episode's its
// program's; the first page brings the hot ones too, each with who wrote it,
// from where, what it answers and whether the listener liked it, and a later
// page brings only the latest, from where the last ended.
func TestSourceComments(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, r.URL.Path+"?"+r.URL.Query().Encode())
		mu.Unlock()
		w.Write([]byte(`{"code":200,"total":128,"more":true,
			"hotComments":[{"commentId":9,"user":{"nickname":"阿青","avatarUrl":"http://p/a.jpg"},"content":"好听","time":1700000000000,"likedCount":42,"liked":true,"ipLocation":{"location":"上海"},
				"beReplied":[{"user":{"nickname":"小明"},"content":"同感"}]}],
			"comments":[{"commentId":10,"user":{"nickname":"路人"},"content":"在听","time":1700000001000,"likedCount":0}]}`))
	}))
	defer srv.Close()
	prev := apiBase
	apiBase = srv.URL
	defer func() { apiBase = prev }()

	src := Source{}
	page, err := src.Comments("42", 0)
	if err != nil || page.Total != 128 || !page.More || len(page.Hot) != 1 || len(page.Comments) != 1 {
		t.Fatalf("%+v, %v", page, err)
	}
	hot := page.Hot[0]
	if hot.ID != "9" || hot.Author != "阿青" || hot.Text != "好听" || hot.Likes != 42 || !hot.Liked || hot.Location != "上海" ||
		hot.Time != 1700000000000 || hot.Reply == nil || hot.Reply.Author != "小明" || hot.Reply.Text != "同感" ||
		!strings.HasPrefix(hot.AvatarURL, "https://p/a.jpg") {
		t.Errorf("hot comment %+v", hot)
	}
	if c := page.Comments[0]; c.ID != "10" || c.Reply != nil || c.Liked {
		t.Errorf("latest comment %+v", c)
	}
	if _, err := src.Comments("7@99", 60); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(asked) != 2 || !strings.HasPrefix(asked[0], "/api/v1/resource/comments/R_SO_4_42?") || !strings.Contains(asked[0], "offset=0") ||
		!strings.HasPrefix(asked[1], "/api/v1/resource/comments/A_DJ_1_99?") || !strings.Contains(asked[1], "offset=60") {
		t.Errorf("asked %q", asked)
	}
	if _, err := src.Comments("not-a-track", 0); err == nil {
		t.Error("asked for the comments of a track that is none")
	}
}

// Liking a comment asks for the thread of the track it is on, by the comment's
// number, and unliking undoes it; a comment that is no number is refused
// before NetEase is asked.
func TestSourceLikeComment(t *testing.T) {
	seen, done := serveByPath(t, map[string]string{
		"/api/v1/comment/like":   `{"code":200}`,
		"/api/v1/comment/unlike": `{"code":200}`,
	})
	defer done()
	src := Source{}
	if err := src.LikeComment("42", "9", true); err != nil {
		t.Fatal(err)
	}
	if err := src.LikeComment("7@99", "10", false); err != nil {
		t.Fatal(err)
	}
	got := seen()
	if len(got) != 2 ||
		got[0].path != "/api/v1/comment/like" || got[0].data["threadId"] != "R_SO_4_42" || got[0].data["commentId"] != "9" ||
		got[1].path != "/api/v1/comment/unlike" || got[1].data["threadId"] != "A_DJ_1_99" || got[1].data["commentId"] != "10" {
		t.Errorf("asked %+v", got)
	}
	if err := src.LikeComment("42", "x", true); err == nil || len(seen()) != 2 {
		t.Errorf("a comment that is no number: %v", err)
	}
}
