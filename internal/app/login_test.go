package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
)

// loginSource is a source whose sign-in the test controls: it shows what
// prompts it is given, ends as the test says, and counts being given up.
type loginSource struct {
	music.Unsupported
	show      func(music.SignInPrompt)
	ends      chan loginEnd
	cancelled atomic.Int32
}

type loginEnd struct {
	who *music.Account
	err error
}

func (*loginSource) Info() music.Info { return music.Info{ID: netease.SourceID, Name: "登录音源"} }
func (*loginSource) Can(what string) bool {
	return what == "signIn"
}
func (s *loginSource) OnSignInPrompt(show func(music.SignInPrompt)) { s.show = show }
func (s *loginSource) CancelSignIn()                                { s.cancelled.Add(1) }
func (s *loginSource) SignIn() (*music.Account, error) {
	e := <-s.ends
	return e.who, e.err
}

// loginApp is an app whose NetEase is a loginSource, with what is posted
// coming on landed for the test to run.
func loginApp() (*app, *loginSource, chan func()) {
	a := newTestApp()
	a.settings = defaultSettings()
	landed := make(chan func(), 16)
	a.onPost = func(fn func()) { landed <- fn }
	src := &loginSource{ends: make(chan loginEnd, 1)}
	a.sources[netease.SourceID] = src
	return a, src, landed
}

// The sign-in dialog follows the source: its code once the source shows one,
// who scanned it once someone has, and a code that runs out is refreshed by
// asking again, the old sign-in's late end ignored.
func TestLoginDialogFollowsTheSource(t *testing.T) {
	a, src, landed := loginApp()
	a.openLogin()
	if !a.login.open || a.login.code != nil || a.login.stage != loginLoading {
		t.Fatalf("opening: %+v", a.login)
	}
	src.show(music.SignInPrompt{QR: "https://music.163.com/login?codekey=ONE"})
	wait(t, landed, "the code")()
	if a.login.code == nil || a.login.stage != loginWaiting {
		t.Fatalf("code shown: %+v", a.login)
	}
	src.show(music.SignInPrompt{QR: "https://music.163.com/login?codekey=ONE", Name: "小明", ImageURL: "http://p/me.jpg"})
	wait(t, landed, "who scanned")()
	if a.login.stage != loginScanned || a.login.scanner != "小明" || a.login.avatar != "http://p/me.jpg" {
		t.Fatalf("scanned: %+v", a.login)
	}

	first := a.login.gen
	src.ends <- loginEnd{err: music.ErrExpired}
	wait(t, landed, "the code to run out")()
	if a.login.stage != loginExpired {
		t.Fatalf("expired: %+v", a.login)
	}
	a.openLogin()
	if a.login.gen != first+1 || a.login.stage != loginLoading || a.login.code != nil {
		t.Errorf("refreshing: %+v", a.login)
	}
}

// A sign-in NetEase refuses says why in its words, one that never got a code
// says the code could not be had, and a refusal after the dialog closed
// shows nothing.
func TestLoginRefusalsAreShown(t *testing.T) {
	a, src, landed := loginApp()
	a.openLogin()
	src.ends <- loginEnd{err: context.DeadlineExceeded}
	wait(t, landed, "the end")()
	if a.login.err != "无法获取二维码，请检查网络" {
		t.Errorf("no code: %q", a.login.err)
	}

	a.openLogin()
	src.show(music.SignInPrompt{QR: "https://x/y"})
	wait(t, landed, "the code")()
	src.ends <- loginEnd{err: music.Fail(music.ErrSignIn, "账号异常")}
	wait(t, landed, "the refusal")()
	if a.login.err != "账号异常" {
		t.Errorf("refused: %q", a.login.err)
	}

	a.openLogin()
	src.show(music.SignInPrompt{QR: "https://x/z"})
	wait(t, landed, "the code")()
	src.ends <- loginEnd{err: music.ErrSignIn}
	wait(t, landed, "a refusal in no words")()
	if a.login.err != "登录失败，请稍后再试" {
		t.Errorf("refused in no words: %q", a.login.err)
	}

	a.openLogin()
	a.login.open = false
	src.ends <- loginEnd{err: music.Fail(music.ErrSignIn, "迟来的拒绝")}
	wait(t, landed, "the late end")()
	if a.login.err != "" {
		t.Errorf("a closed dialog shows %q", a.login.err)
	}
}

// Closing the dialog while the code waits gives the sign-in up, once, and
// signing out does too.
func TestClosingTheLoginDialogGivesUpTheSignIn(t *testing.T) {
	a, src, _ := loginApp()
	tt := ui.NewTester(a.view, 1200, 800)
	a.openLogin()
	tt.Frame()
	if n := src.cancelled.Load(); n != 0 {
		t.Fatalf("given up at once: %d", n)
	}
	a.login.open = false
	tt.Frame()
	tt.Frame()
	time.Sleep(20 * time.Millisecond)
	if n := src.cancelled.Load(); n != 1 {
		t.Errorf("closed: given up %d times, want once", n)
	}

	a.openLogin()
	a.signOut()
	time.Sleep(20 * time.Millisecond)
	if n := src.cancelled.Load(); n != 2 {
		t.Errorf("signed out: given up %d times in all, want 2", n)
	}
}
