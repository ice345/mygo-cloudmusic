package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// A navigation entry that is the app's own and names no title, as upstream's demo
// plugin writes its home, has the app's title, not a blank beside its icon; one
// of the plugin's own pages with no title is called by its page.
func TestUntitledNavigationEntriesHaveTitles(t *testing.T) {
	_, tt, landed := switcherApp(t, `{"id":"x","name":"测试音源","navigation":[
		{"builtin":"home"},{"builtin":"stations"},{"page":"discover","icon":"star"}]}`,
		`export function home() { return { title: "欢迎", sections: [] }; }`)
	pickPlugin(t, tt, "网易云音乐", "测试音源")
	landN(t, tt, landed, 1)
	for _, want := range []string{"首页", "discover"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %v", want, tt.Texts())
		}
	}
}

// The search field says what it searches: NetEase's songs, artists and
// podcasts, and for a plugin, which may have no podcasts, only that it searches
// there.
func TestSearchPlaceholderNamesTheSourceShown(t *testing.T) {
	a, tt, landed := switcherApp(t, siteManifest, siteJS)
	if got := a.searchPlaceholder(); got != "搜索歌曲、歌手、播客" {
		t.Errorf("NetEase: %q", got)
	}
	pickPlugin(t, tt, "网易云音乐", "测试音源")
	landN(t, tt, landed, 1)
	if got := a.searchPlaceholder(); got != "在 测试音源 中搜索" {
		t.Errorf("plugin: %q", got)
	}
}

// inkOfCover draws a cover of url in a square and counts the pixels of its
// whole that are not the cover's own ground.
func inkOfCover(t *testing.T, url string) int {
	t.Helper()
	a := newTestApp()
	tt := ui.NewTester(func(c *ui.Context) {
		th := *c.Theme()
		a.pal = paletteFor(&th)
		c.Root().Background(a.pal.bg)
		ui.Box(c).Size(200, 200).Margin(10).Children(func() { a.coverFill(c, url, 400, 10) })
	}, 220, 220)
	tt.Frame()
	img := tt.Image()
	ground := img.RGBAAt(110, 30)
	n := 0
	for y := 30; y < 190; y++ {
		for x := 30; x < 190; x++ {
			if img.RGBAAt(x, y) != ground {
				n++
			}
		}
	}
	return n
}

// A large cover with no picture to come, as a source that has none gives,
// holds a note, as the small covers of the rows do, instead of a blank square;
// one whose picture is only on its way is as it was.
func TestEmptyCoverHoldsANote(t *testing.T) {
	if n := inkOfCover(t, ""); n < 50 {
		t.Errorf("a cover with no picture has %d pixels of a note", n)
	}
	if n := inkOfCover(t, "file:///no/such/cover.jpg"); n != 0 {
		t.Errorf("a cover on its way shows %d pixels of something", n)
	}
}
