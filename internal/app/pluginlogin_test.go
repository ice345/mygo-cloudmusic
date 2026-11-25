package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
)

// plugLogin is a plugin's source whose sign-in the test controls, as
// loginSource is NetEase's, and which knows who it signed in.
type plugLogin struct {
	loginSource
	gone chan struct{} // it was given up
}

func (*plugLogin) Info() music.Info { return music.Info{ID: "plug", Name: "插件音源"} }
func (*plugLogin) Can(what string) bool {
	return what == "signIn"
}
func (s *plugLogin) CancelSignIn() { s.gone <- struct{}{} }

// loginAppWithPlugin is loginApp with a plugin's source beside NetEase's.
func loginAppWithPlugin() (*app, *loginSource, *plugLogin, chan func()) {
	a, netSrc, landed := loginApp()
	plug := &plugLogin{loginSource: loginSource{ends: make(chan loginEnd, 1)}, gone: make(chan struct{}, 4)}
	a.sources["plug"] = plug
	return a, netSrc, plug, landed
}

// A sign-in to a plugin shows its words, from its prompt, in a dialog of its
// name, with no code where it showed none; who signed in is the plugin's
// account, and NetEase's is left alone.
func TestPluginLoginShowsWhatThePluginSays(t *testing.T) {
	a, _, plug, landed := loginAppWithPlugin()
	tt := ui.NewTester(a.view, 1200, 800)
	a.openLoginFor(plug)
	tt.Frame()
	if !tt.HasText("登录 插件音源") || !tt.HasText("请按提示完成登录") || !tt.HasText("取消") {
		t.Fatalf("opening shows %v", tt.Texts())
	}
	if tt.HasText("打开网易云音乐 App，扫一扫登录") {
		t.Error("NetEase's words in another source's dialog")
	}

	plug.show(music.SignInPrompt{Text: "在打开的页面里登录"})
	wait(t, landed, "the prompt")()
	tt.Frame()
	if !tt.HasText("在打开的页面里登录") {
		t.Errorf("the plugin's words are not shown: %v", tt.Texts())
	}

	plug.ends <- loginEnd{who: &music.Account{ID: "7", Name: "小明"}}
	wait(t, landed, "the end")()
	tt.Frame()
	if who := a.accountOf("plug"); who == nil || who.Name != "小明" {
		t.Fatalf("the plugin's account: %+v", who)
	}
	if a.account != nil {
		t.Errorf("NetEase's account became %+v", a.account)
	}
	if !a.login.finished || !tt.HasText("欢迎回来，小明") {
		t.Errorf("finished %v, shows %v", a.login.finished, tt.Texts())
	}
}

// A plugin's sign-in that fails says why in the plugin's words, with a way to
// try again, not the NetEase code that couldn't be had; giving the dialog up
// gives up the plugin's sign-in, and not NetEase's.
func TestPluginLoginFailsAndIsGivenUp(t *testing.T) {
	a, netSrc, plug, landed := loginAppWithPlugin()
	tt := ui.NewTester(a.view, 1200, 800)
	a.openLoginFor(plug)
	plug.ends <- loginEnd{err: music.Fail(music.ErrSignIn, "账号被锁定")}
	wait(t, landed, "the end")()
	tt.Frame()
	if a.login.err != "账号被锁定" || !tt.HasText("账号被锁定") || !tt.HasText("重试") {
		t.Fatalf("error %q, shows %v", a.login.err, tt.Texts())
	}

	gen := a.login.gen
	if err := tt.Click("重试"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.login.gen != gen+1 || a.login.err != "" || a.login.source != "plug" {
		t.Fatalf("trying again: %+v", a.login)
	}

	// Closed while it waits.
	a.login.open = false
	tt.Frame()
	wait(t, plug.gone, "the plugin's sign-in to be given up")
	if n := netSrc.cancelled.Load(); n != 0 {
		t.Errorf("NetEase's sign-in was given up %d times", n)
	}
}

// pluginAccountJS is a plugin that signs in and out, as an account is kept.
const pluginAccountJS = `
let who = null;
export function account() { return who; }
export function signIn() { who = { id: "1", name: "小明" }; return who; }
export function signOut() { who = null; }
`

// The settings window says who is signed in to a plugin, asked of the plugin
// when the window first shows it; signing in is offered where nobody is, which
// opens the dialog for that plugin, and signing out takes the account away.
func TestPluginAccountInSettings(t *testing.T) {
	a, _, tt, landed := pluginSettingsJS(t, `{"id":"x","name":"测试音源"}`, pluginAccountJS, t.TempDir())
	// Asked as the window showed it, and answered off the main thread.
	land(t, tt, landed)
	if !tt.HasText("未登录") {
		t.Fatalf("nobody is signed in: %v", tt.Texts())
	}
	if err := tt.Click("登录"); err != nil {
		t.Fatal(err)
	}
	if !a.login.open || a.login.source != "x" {
		t.Fatalf("the dialog: %+v", a.login)
	}
	land(t, tt, landed) // who signed in
	if who := a.accountOf("x"); who == nil || who.Name != "小明" {
		t.Fatalf("signed in: %+v", who)
	}
	if !tt.HasText("小明") || !tt.HasText("已登录 测试音源") {
		t.Errorf("who is signed in isn't shown: %v", tt.Texts())
	}

	if err := tt.Click("退出登录"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.accountOf("x") != nil || !tt.HasText("未登录") {
		t.Errorf("signed out, but %+v, shows %v", a.accountOf("x"), tt.Texts())
	}
}

// A plugin that asks for a setting before it can say who is signed in says so
// where the account would be, in its own words, and offers no sign-in until
// it is set up.
func TestPluginThatNeedsSettingUpSaysWhy(t *testing.T) {
	_, _, tt, landed := pluginSettingsJS(t, `{"id":"x","name":"测试音源","signIn":true}`, `
export function account() { throw Object.assign(new Error("先填写服务器地址"), { code: "setup" }); }
export function signIn() { return { id: "1", name: "x" }; }
`, t.TempDir())
	land(t, tt, landed)
	if !tt.HasText("需要先设置") || !tt.HasText("先填写服务器地址") {
		t.Fatalf("does not say what it needs: %v", tt.Texts())
	}
	if err := tt.Click("登录"); err == nil {
		t.Error("offers to sign in to a plugin that is not set up")
	}
}
