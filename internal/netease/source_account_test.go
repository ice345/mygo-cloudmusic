package netease

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"netease-music-mygo/internal/music"
)

var (
	_ music.Prompter = Source{}
	_ music.Canceler = Source{}
)

// quickSignIn makes a sign-in poll at once, and puts back what a test
// replaced of the keychain and of the session.
func quickSignIn(t *testing.T) (saved *int) {
	t.Helper()
	n := 0
	prevEvery, prevSave, prevRestore, prevClear, prevLogout := signInEvery, saveSession, restoreSession, clearSession, logout
	signInEvery = time.Millisecond
	saveSession = func() error { n++; return nil }
	restoreSession = func() bool { return false }
	clearSession = func() {}
	logout = func() {}
	t.Cleanup(func() {
		signInEvery, saveSession, restoreSession, clearSession, logout = prevEvery, prevSave, prevRestore, prevClear, prevLogout
		acct.mu.Lock()
		acct.show, acct.cancel, acct.me = nil, nil, nil
		acct.mu.Unlock()
		restoreOnce, restored = sync.Once{}, false
	})
	return &n
}

// Signing in by QR shows the code, then who scanned it, and returns who
// confirmed once they do, with the session kept once and the account asked for
// after.
func TestSourceSignsInByQR(t *testing.T) {
	saved := quickSignIn(t)
	checks := []string{
		`{"code":801}`, `{"code":801}`,
		`{"code":802,"nickname":"小明","avatarUrl":"http://p/me.jpg"}`,
		`{"code":803}`,
	}
	_, done := serveByPathFn(t, func(path string, n int) (string, bool) {
		switch path {
		case "/api/login/qrcode/unikey":
			return `{"code":200,"unikey":"KEY-1"}`, true
		case "/api/login/qrcode/client/login":
			return checks[min(n, len(checks))-1], true
		case "/api/nuser/account/get":
			return `{"code":200,"profile":{"userId":5,"nickname":"小明","avatarUrl":"http://p/me.jpg"}}`, true
		}
		return "", false
	})
	defer done()
	var prompts []music.SignInPrompt
	var mu sync.Mutex
	src := Source{}
	src.OnSignInPrompt(func(p music.SignInPrompt) { mu.Lock(); prompts = append(prompts, p); mu.Unlock() })
	acc, err := src.SignIn()
	if err != nil || acc.ID != "5" || acc.Name != "小明" || acc.ImageURL != "http://p/me.jpg" {
		t.Fatalf("signed in %+v, %v", acc, err)
	}
	if *saved != 1 {
		t.Errorf("the session was kept %d times", *saved)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(prompts) != 2 || prompts[0].QR != LoginURL("KEY-1") || prompts[0].Name != "" ||
		prompts[1].QR != LoginURL("KEY-1") || prompts[1].Name != "小明" || prompts[1].ImageURL != "http://p/me.jpg" {
		t.Errorf("prompts %+v", prompts)
	}
}

// A code that runs out is ErrExpired, one NetEase refuses is ErrSignIn in its
// own words, and a poll that fails to reach NetEase is tried again.
func TestSourceSignInEnds(t *testing.T) {
	for name, c := range map[string]struct {
		check string
		want  error
		why   string
	}{
		"expired": {`{"code":800}`, music.ErrExpired, ""},
		"refused": {`{"code":8888,"message":"账号异常"}`, music.ErrSignIn, "账号异常"},
	} {
		quickSignIn(t)
		_, done := serveByPathFn(t, func(path string, n int) (string, bool) {
			switch path {
			case "/api/login/qrcode/unikey":
				return `{"code":200,"unikey":"K"}`, true
			case "/api/login/qrcode/client/login":
				if n == 1 {
					return `not json`, true // the network failed: asked again
				}
				return c.check, true
			}
			return "", false
		})
		_, err := Source{}.SignIn()
		done()
		if !errors.Is(err, c.want) || music.Reason(err) != c.why {
			t.Errorf("%s: %v (%q)", name, err, music.Reason(err))
		}
	}
}

// Giving up a sign-in while it waits ends it as cancelled.
func TestSourceSignInCanBeGivenUp(t *testing.T) {
	quickSignIn(t)
	signInEvery = 5 * time.Millisecond
	_, done := serveByPathFn(t, func(path string, n int) (string, bool) {
		switch path {
		case "/api/login/qrcode/unikey":
			return `{"code":200,"unikey":"K"}`, true
		case "/api/login/qrcode/client/login":
			return `{"code":801}`, true
		}
		return "", false
	})
	defer done()
	result := make(chan error, 1)
	go func() { _, err := Source{}.SignIn(); result <- err }()
	time.Sleep(30 * time.Millisecond)
	Source{}.CancelSignIn()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("ended with %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the sign-in went on after it was given up")
	}
}

// The first time it is asked, the account brings the session back, and a session
// that has run out is forgotten, once; signed out, it is nobody, not an error.
func TestSourceAccountRestoresAndForgetsAnExpiredSession(t *testing.T) {
	quickSignIn(t)
	restores, clears := 0, 0
	restoreSession = func() bool { restores++; return true }
	clearSession = func() { clears++ }
	_, done := serveByPath(t, map[string]string{"/api/nuser/account/get": `{"code":200,"profile":null}`})
	defer done()
	for range 2 {
		acc, err := Source{}.Account()
		if acc != nil || err != nil {
			t.Fatalf("expired session: %+v, %v", acc, err)
		}
	}
	if restores != 1 || clears != 1 {
		t.Errorf("restored %d times and forgot %d, want once each", restores, clears)
	}
}

// The listener's playlists are those they made, the liked songs' first and
// marked the source's own, and then those they saved.
func TestSourcePlaylistsAreMadeThenSaved(t *testing.T) {
	quickSignIn(t)
	_, done := serveByPath(t, map[string]string{
		"/api/nuser/account/get": `{"code":200,"profile":{"userId":5,"nickname":"小明"}}`,
		"/api/user/playlist": `{"code":200,"playlist":[` +
			`{"id":1,"name":"小明喜欢的音乐","specialType":5,"trackCount":30},` +
			`{"id":2,"name":"通勤","trackCount":10},` +
			`{"id":3,"name":"飙升榜","subscribed":true,"trackCount":100}]}`,
	})
	defer done()
	got, err := Source{}.Playlists()
	if err != nil || len(got) != 3 {
		t.Fatalf("%+v, %v", got, err)
	}
	if got[0].ID != "1" || !got[0].Fixed || !got[0].Mine || got[1].ID != "2" || !got[1].Mine || got[1].Fixed {
		t.Errorf("made: %+v %+v", got[0], got[1])
	}
	if got[2].ID != "3" || !got[2].Saved || got[2].Mine {
		t.Errorf("saved: %+v", got[2])
	}
}
