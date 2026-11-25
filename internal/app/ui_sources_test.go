package app

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/plugin"
)

// pluginSettings makes an app with one plugin, of the manifest given and
// keeping its data in data, and draws the settings window with it. What lands
// off the main thread waits in landed, for the test to let it in.
func pluginSettings(t *testing.T, manifest, data string) (*app, *plugin.Plugin, *ui.Tester, chan func()) {
	t.Helper()
	return pluginSettingsJS(t, manifest, `export function search() { return { tracks: [] }; }`, data)
}

// pluginSettingsJS is pluginSettings of a plugin whose module is js.
func pluginSettingsJS(t *testing.T, manifest, js, data string) (*app, *plugin.Plugin, *ui.Tester, chan func()) {
	t.Helper()
	a := newTestApp()
	a.settings = defaultSettings()
	landed := make(chan func(), 4)
	a.onPost = func(fn func()) { landed <- fn }
	code := t.TempDir()
	putPlugin(t, code, "x", manifest, js)
	a.usePlugins(code, data)
	t.Cleanup(a.closePlugins)
	if len(a.plugins) != 1 {
		t.Fatal("the plugin did not load")
	}
	tt := ui.NewTester(a.settingsView, 560, 2400)
	tt.Frame()
	return a, a.plugins[0], tt, landed
}

// land lets in what the app was waiting to post, and draws again.
func land(t *testing.T, tt *ui.Tester, landed chan func()) {
	t.Helper()
	fn := <-landed
	fn()
	tt.Frame()
}

// rowEnd is where a row's control is, at the right end of the window: a
// switch is that wide, a menu wider.
func rowEnd(tt *ui.Tester, title string, from float32) (x, y float32) {
	r, ok := tt.Find(title)
	if !ok {
		return 0, 0
	}
	return 560 - 28 - 16 - from, r.Y + r.H/2
}

const textSetting = `{"id":"x","name":"测试音源","settings":[{"key":"server","label":"服务器地址","help":"你的服务器"}]}`

// A setting typed into a field is kept with Save, not before: what is half
// typed is not the setting. What was typed is the setting once the save, which
// is the source's to do off the main thread, lands.
func TestPluginSettingTextIsKeptWithSave(t *testing.T) {
	_, pl, tt, landed := pluginSettings(t, textSetting, t.TempDir())
	save, ok := tt.Find("保存")
	if !ok {
		t.Fatal("no Save")
	}
	tt.ClickAt(save.X-8-30, save.Y+save.H/2)
	tt.Type("http://nas.local")
	tt.Frame()
	if got := pl.Setting("server"); got != "" {
		t.Fatalf("kept %q before Save", got)
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	land(t, tt, landed)
	if got := pl.Setting("server"); got != "http://nas.local" {
		t.Errorf("kept %q", got)
	}
	if tt.HasText("没能保存") {
		t.Error("says it could not save")
	}
}

// A setting that can't be kept says so and keeps what was typed, to try
// again, instead of showing it as kept.
func TestPluginSettingThatCannotBeKeptSaysSo(t *testing.T) {
	// A file where the plugin's folder of data should go.
	data := filepath.Join(t.TempDir(), "plugin-data")
	if err := os.WriteFile(data, []byte("not a folder"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, pl, tt, landed := pluginSettings(t, textSetting, data)
	save, _ := tt.Find("保存")
	tt.ClickAt(save.X-8-30, save.Y+save.H/2)
	tt.Type("http://nas.local")
	tt.Frame()
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	land(t, tt, landed)
	if !tt.HasText("没能保存") {
		t.Error("did not say it could not save")
	}
	if got := pl.Setting("server"); got != "" {
		t.Errorf("the plugin reads %q though it could not be kept", got)
	}
	if !tt.HasText("保存") {
		t.Error("nothing to try again with")
	}
}

const choiceSettings = `{"id":"x","name":"测试音源","settings":[
	{"key":"hd","label":"高清模式","type":"boolean"},
	{"key":"tier","label":"画质档位","type":"select","options":[{"value":"low","label":"低"},{"value":"high","label":"高"}]}]}`

// A switch and a menu keep what is chosen at once, and the menu keeps the
// option's value, not the label it is shown by.
func TestPluginSettingChoicesAreKeptAtOnce(t *testing.T) {
	_, pl, tt, landed := pluginSettings(t, choiceSettings, t.TempDir())
	if !tt.HasText("未选择") {
		t.Error("a menu with nothing chosen says so")
	}

	x, y := rowEnd(tt, "高清模式", 12)
	tt.ClickAt(x, y)
	land(t, tt, landed)
	if got := pl.Setting("hd"); got != "true" {
		t.Errorf("turned on, kept %q", got)
	}
	x, y = rowEnd(tt, "高清模式", 12)
	tt.ClickAt(x, y)
	land(t, tt, landed)
	if got := pl.Setting("hd"); got != "false" {
		t.Errorf("turned off, kept %q", got)
	}

	x, y = rowEnd(tt, "画质档位", 90)
	tt.ClickAt(x, y)
	tt.Frame()
	if err := tt.Click("高"); err != nil {
		t.Fatalf("no option 高 after opening the menu: %v", err)
	}
	land(t, tt, landed)
	if got := pl.Setting("tier"); got != "high" {
		t.Errorf("chose 高, kept %q", got)
	}
}

const programSettings = `{"id":"x","name":"测试音源","permissions":{"process":["ffmpeg"]}}`

// grantHelp is the row's help, which makes it as tall as its switch is not.
const grantHelp = "开启后，这个音源可以运行：ffmpeg。这些程序能做你能做的一切，只允许你信任的音源。"

// A plugin may start programs only once the listener, asked what it would
// run, says yes; declining leaves it as it was, and taking it back needs no
// asking.
func TestPluginStartsProgramsOnlyWhenTheListenerAllows(t *testing.T) {
	a, pl, tt, landed := pluginSettings(t, programSettings, t.TempDir())
	flip := func() {
		t.Helper()
		top, ok1 := tt.Find("允许启动程序")
		bottom, ok2 := tt.Find(grantHelp)
		if !ok1 || !ok2 {
			t.Fatal("no row to allow programs")
		}
		tt.ClickAt(560-28-16-12, (top.Y+bottom.Y+bottom.H)/2)
		tt.Frame()
	}
	if !tt.HasText("可启动程序：ffmpeg") {
		t.Error("does not say what it may start")
	}

	flip()
	if !a.granting.open || pl.Granted() {
		t.Fatalf("asked: %v, granted: %v", a.granting.open, pl.Granted())
	}
	if !tt.HasText("取消") || !tt.HasText("允许") {
		t.Fatal("the question has no answers")
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if pl.Granted() || a.granting.open {
		t.Fatalf("declined, but granted: %v, asked: %v", pl.Granted(), a.granting.open)
	}

	flip()
	if err := tt.Click("允许"); err != nil {
		t.Fatal(err)
	}
	land(t, tt, landed)
	if !pl.Granted() {
		t.Fatal("allowed, but not granted")
	}

	flip()
	if a.granting.open {
		t.Error("asked again to take it back")
	}
	land(t, tt, landed)
	if pl.Granted() {
		t.Error("taken back, but still granted")
	}
}

// slowSource is a source that takes a setting only when the test lets it,
// and says if the caller had to wait for that.
type slowSource struct {
	music.Unsupported
	release  chan struct{}
	mu       sync.Mutex
	kept     map[string]string
	timedOut atomic.Bool
}

func (*slowSource) Info() music.Info { return music.Info{ID: "slow", Name: "慢的音源"} }

func (s *slowSource) Setting(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kept[key]
}

func (s *slowSource) SetSetting(key, value string) error {
	select {
	case <-s.release:
	case <-time.After(3 * time.Second):
		// Only a caller that waited gets here: the test lets it go first.
		s.timedOut.Store(true)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.kept == nil {
		s.kept = map[string]string{}
	}
	s.kept[key] = value
	return nil
}

// A setting is given to its source off the main thread: a source that is slow
// to take it, as a running plugin may be, leaves the window answering
// meanwhile instead of holding it.
func TestSettingIsKeptOffTheMainThread(t *testing.T) {
	a := newTestApp()
	landed := make(chan func(), 2)
	a.onPost = func(fn func()) { landed <- fn }
	src := &slowSource{release: make(chan struct{})}
	st := music.Setting{Key: "server", Label: "服务器地址"}
	tt := ui.NewTester(func(c *ui.Context) {
		theme := *c.Theme()
		ui.Column(c).Padding(20).Children(func() { a.settingControl(c, paletteFor(&theme), src, "slow", st) })
	}, 560, 200)
	tt.Frame()
	save, ok := tt.Find("保存")
	if !ok {
		t.Fatal("no Save")
	}
	tt.ClickAt(save.X-8-30, save.Y+save.H/2)
	tt.Type("http://nas.local")
	tt.Frame()
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	// The window is back, with the source still holding the setting.
	if got := src.Setting("server"); got != "" {
		t.Fatalf("the source has %q before it was let", got)
	}
	close(src.release)
	land(t, tt, landed)
	if got := src.Setting("server"); got != "http://nas.local" {
		t.Errorf("the source has %q", got)
	}
	if src.timedOut.Load() {
		t.Error("saving held the caller until the source was done")
	}
}
