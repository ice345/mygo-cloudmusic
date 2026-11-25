package app

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
)

// switcherApp makes an app with one plugin, of the manifest and module given,
// and draws its window. What lands off the main thread waits in landed.
func switcherApp(t *testing.T, manifest, js string) (*app, *ui.Tester, chan func()) {
	t.Helper()
	a := newTestApp()
	a.settings = defaultSettings()
	landed := make(chan func(), 16)
	a.onPost = func(fn func()) { landed <- fn }
	code := t.TempDir()
	putPlugin(t, code, "x", manifest, js)
	a.usePlugins(code, t.TempDir())
	t.Cleanup(a.closePlugins)
	if len(a.plugins) != 1 {
		t.Fatal("the plugin did not load")
	}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	return a, tt, landed
}

// landN lets in the n things the app was waiting to post, and draws again.
func landN(t *testing.T, tt *ui.Tester, landed chan func(), n int) {
	t.Helper()
	for range n {
		wait(t, landed, "an answer of the plugin")()
	}
	tt.Frame()
}

// pickPlugin chooses the plugin in the switcher, as the listener does.
func pickPlugin(t *testing.T, tt *ui.Tester, from, plugin string) {
	t.Helper()
	if err := tt.Click("音源：" + from); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(plugin); err != nil {
		t.Fatalf("no %s in the switcher: %v", plugin, err)
	}
	tt.Frame()
}

const siteManifest = `{"id":"x","name":"测试音源","navigation":[
	{"title":"首页","builtin":"home","icon":"home"},{"title":"新歌","page":"new","icon":"note"}]}`

const siteJS = `
const HOME = {
  title: "测试首页",
  sections: [
    { title: "推荐歌曲", tracks: [{ id: "t1", name: "歌一", artists: [{ id: "a1", name: "歌手甲" }], album: { id: "al1", name: "专辑甲" }, duration: 200000 }] },
    { title: "精选歌单", playlists: [{ id: "p9", name: "精选一", trackCount: 3 }], morePage: "more" },
    { title: "入口", links: [{ title: "热门榜", subtitle: "每天更新", page: "hot" }] },
  ],
};
export function home() { return HOME; }
export function browse({ page }) { return { title: "页面：" + page, sections: [] }; }
export function search({ query }) { return { tracks: [{ id: "s1", name: "搜到的歌：" + query, duration: 1000 }], total: 1 }; }
`

// Without a plugin the window has no switcher, and with one it offers NetEase
// and the plugin: choosing the plugin shows its own sidebar and the page its
// Home makes, none of NetEase's; choosing NetEase brings its own back.
func TestSwitcherSwitchesSourcesAndTheirSidebars(t *testing.T) {
	plain := newTestApp()
	plain.settings = defaultSettings()
	pt := ui.NewTester(plain.view, 1200, 800)
	pt.Frame()
	if err := pt.Click("音源：网易云音乐"); err == nil {
		t.Error("a switcher with nothing to switch to")
	}

	a, tt, landed := switcherApp(t, siteManifest, siteJS)
	if !tt.HasText("每日推荐") {
		t.Fatal("NetEase's sidebar is not shown at first")
	}
	pickPlugin(t, tt, "网易云音乐", "测试音源")
	if a.activeID() != "x" || a.route != homeRoute("x") {
		t.Fatalf("active %q, page %+v", a.activeID(), a.route)
	}
	landN(t, tt, landed, 1) // its home
	for _, want := range []string{"首页", "新歌", "测试首页", "推荐歌曲", "歌一", "精选歌单", "精选一"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %v", want, tt.Texts())
		}
	}
	for _, gone := range []string{"每日推荐", "私人 FM", "排行榜"} {
		if tt.HasText(gone) {
			t.Errorf("NetEase's %q beside a plugin's pages", gone)
		}
	}

	pickPlugin(t, tt, "测试音源", "网易云音乐")
	if a.activeID() != netease.SourceID || a.route != homeRoute(netease.SourceID) {
		t.Fatalf("back: active %q, page %+v", a.activeID(), a.route)
	}
	if !tt.HasText("每日推荐") || tt.HasText("新歌") {
		t.Errorf("NetEase's sidebar is not back: %v", tt.Texts())
	}
}

// A plugin's page opens the pages it links to, by a section's link to more and
// by a card, and goes back to where it was.
func TestPluginPagesLinkToEachOther(t *testing.T) {
	a, tt, landed := switcherApp(t, siteManifest, siteJS)
	pickPlugin(t, tt, "网易云音乐", "测试音源")
	landN(t, tt, landed, 1)

	if err := tt.Click("更多"); err != nil {
		t.Fatal(err)
	}
	want := route{page: pageBrowse, source: "x", id: "more"}
	if a.route != want {
		t.Fatalf("opened %+v, want %+v", a.route, want)
	}
	landN(t, tt, landed, 1)
	if !tt.HasText("页面：more") {
		t.Errorf("the linked page is not shown: %v", tt.Texts())
	}

	a.back()
	tt.Frame()
	if err := tt.Click("热门榜"); err != nil {
		t.Fatal(err)
	}
	if want := (route{page: pageBrowse, source: "x", id: "hot"}); a.route != want {
		t.Fatalf("opened %+v, want %+v", a.route, want)
	}
	landN(t, tt, landed, 1)

	if err := tt.Click("新歌"); err != nil {
		t.Fatal(err)
	}
	landN(t, tt, landed, 1)
	if !tt.HasText("页面：new") {
		t.Errorf("the sidebar's page is not shown: %v", tt.Texts())
	}

	a.back()
	a.back()
	a.back()
	tt.Frame()
	if a.route != homeRoute("x") || !tt.HasText("推荐歌曲") {
		t.Errorf("back to %+v", a.route)
	}
}

// Searching while a plugin is shown asks the plugin, and its results are the
// plugin's songs, with the tabs a plugin can search and none for users, which
// only NetEase has; a search made before the switch is not the new one's.
func TestSearchFollowsTheSourceShown(t *testing.T) {
	a, tt, landed := switcherApp(t, siteManifest, siteJS)
	a.search("先前的搜索")
	if a.route.source != "" || a.searched.query != "先前的搜索" {
		t.Fatalf("NetEase's search: %+v", a.route)
	}

	pickPlugin(t, tt, "网易云音乐", "测试音源")
	landN(t, tt, landed, 1)
	if a.searched.query != "" {
		t.Errorf("the last search of another source is %q", a.searched.query)
	}
	// What NetEase would suggest is not asked for another source's music.
	a.suggest("歌")
	if a.suggestions.loading {
		t.Error("NetEase's suggestions are asked for while a plugin is shown")
	}
	a.search("歌")
	want := route{page: pageSearch, source: "x", query: "歌", kind: music.SearchTracks}
	if a.route != want {
		t.Fatalf("searched %+v, want %+v", a.route, want)
	}
	landN(t, tt, landed, 1)
	if !tt.HasText("搜到的歌：歌") {
		t.Errorf("the plugin's results are not shown: %v", tt.Texts())
	}
	if tt.HasText("用户") || !tt.HasText("单曲") {
		t.Errorf("the tabs of a plugin's search: %v", tt.Texts())
	}
}

// A plugin whose page wants a sign-in or a setting says so where the page
// would be, and offers what to do: sign in, or go to the settings; any other
// failure offers to try again.
func TestPluginPageThatCannotBeHad(t *testing.T) {
	for _, c := range []struct {
		name, js string
		texts    []string
		button   string
	}{
		{"sign in", `export function home() { throw Object.assign(new Error("先登录账号"), { code: "signIn" }); }`,
			[]string{"需要先登录 测试音源", "先登录账号"}, "登录"},
		{"setup", `export function home() { throw Object.assign(new Error("填写服务器"), { code: "setup" }); }`,
			[]string{"需要先设置 测试音源", "填写服务器"}, "打开设置"},
		{"other", `export function home() { throw new Error("boom"); }`,
			[]string{"加载失败，请稍后再试"}, "重试"},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, tt, landed := switcherApp(t, `{"id":"x","name":"测试音源"}`, c.js+`
export function signIn() { return { id: "1", name: "x" }; }`)
			pickPlugin(t, tt, "网易云音乐", "测试音源")
			landN(t, tt, landed, 1)
			for _, want := range c.texts {
				if !tt.HasText(want) {
					t.Errorf("no %q in %v", want, tt.Texts())
				}
			}
			if !tt.HasText(c.button) {
				t.Fatalf("no %q to press in %v", c.button, tt.Texts())
			}
			switch c.name {
			case "sign in":
				if tt.HasText("重试") {
					t.Error("offers to try again what needs a sign-in")
				}
				if err := tt.Click("登录"); err != nil {
					t.Fatal(err)
				}
				if !a.login.open || a.login.source != "x" {
					t.Errorf("the dialog: %+v", a.login)
				}
			case "other":
				if err := tt.Click("重试"); err != nil {
					t.Fatal(err)
				}
				landN(t, tt, landed, 1) // asked again
			}
		})
	}
}

// The sidebar of a plugin has the playlists the listener made and saved there,
// each opening its page at the plugin, and who is signed in at its foot, or a
// way to sign in; all asked of the plugin as it is switched to.
func TestPluginSidebarHasItsPlaylistsAndAccount(t *testing.T) {
	a, tt, landed := switcherApp(t, siteManifest, siteJS+`
let who = null;
export function account() { return who; }
export function signIn() { who = { id: "1", name: "小明" }; return who; }
export function playlists() {
  return who ? [{ id: "m1", name: "通勤路上", mine: true }, { id: "s1", name: "别人的歌单", saved: true }] : [];
}
`)
	pickPlugin(t, tt, "网易云音乐", "测试音源")
	landN(t, tt, landed, 3) // account, playlists, home
	if !tt.HasText("登录 测试音源") || tt.HasText("通勤路上") {
		t.Fatalf("signed out: %v", tt.Texts())
	}

	if err := tt.Click("登录 测试音源"); err != nil {
		t.Fatal(err)
	}
	if !a.login.open || a.login.source != "x" {
		t.Fatalf("the dialog: %+v", a.login)
	}
	landN(t, tt, landed, 1) // who signed in, which asks for what is theirs
	landN(t, tt, landed, 2) // their playlists, and the home again
	for _, want := range []string{"小明", "我的歌单", "通勤路上", "收藏的歌单", "别人的歌单"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %v", want, tt.Texts())
		}
	}

	// The dialog greets them for a moment; it closes.
	a.login.open = false
	tt.Frame()
	if err := tt.Click("通勤路上"); err != nil {
		t.Fatal(err)
	}
	if want := (route{page: pagePlaylist, source: "x", id: "m1"}); a.route != want {
		t.Errorf("opened %+v, want %+v", a.route, want)
	}
}

// The registry of sources can be changed while it is read, as it will be by
// a plugin installed as the app runs: under the race detector, this is where an
// unguarded map is found out.
func TestSourceRegistryIsSafeToChangeWhileRead(t *testing.T) {
	a := newTestApp()
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					a.source(netease.SourceID)
					a.source("late")
				}
			}
		}()
	}
	for i := range 50 {
		a.addSource(&idSource{id: fmt.Sprintf("s%d", i)})
	}
	close(stop)
	wg.Wait()
	if a.source("s49") == nil {
		t.Error("a registered source is not found")
	}
	if a.addSource(&idSource{id: "s0"}) {
		t.Error("a second source of one ID was registered")
	}
}

// idSource is a source of nothing, known by its ID.
type idSource struct {
	music.Unsupported
	id string
}

func (s *idSource) Info() music.Info { return music.Info{ID: s.id, Name: s.id} }

// With NM_FRAMES set, saves a plugin's pages, light and dark: the switcher
// open, its home, and a search of it.
func TestPluginFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	for _, dark := range []bool{false, true} {
		suffix := ""
		if dark {
			suffix = "-dark"
		}
		a, tt, landed := switcherApp(t, siteManifest, siteJS+`
export function account() { return { id: "1", name: "小明" }; }
export function playlists() { return [{ id: "m1", name: "通勤路上", mine: true }, { id: "s1", name: "别人的歌单", saved: true }]; }
`)
		tt.SetDark(dark)
		tt.Frame()
		save := func(name string) {
			f, err := os.Create(filepath.Join(dir, name+suffix+".png"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			png.Encode(f, tt.Image())
		}
		if err := tt.Click("音源：网易云音乐"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		// The menu comes in with a motion.
		tt.Advance(400 * time.Millisecond)
		tt.Frame()
		save("plugin-switcher")
		if err := tt.Click("测试音源"); err != nil {
			t.Fatal(err)
		}
		landN(t, tt, landed, 3)
		tt.Advance(400 * time.Millisecond) // the menu goes out
		tt.Frame()
		save("plugin-home")
		a.search("歌")
		landN(t, tt, landed, 1)
		save("plugin-search")
	}
}
