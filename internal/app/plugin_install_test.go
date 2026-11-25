package app

import (
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/plugin"
)

// installEnv is an app, with its settings window drawn, whose plugins are in a
// folder of the test's, and whose way to GitHub, the disk's dialogs, the trash
// and the question of removing are the test's.
type installEnv struct {
	a       *app
	tt      *ui.Tester
	landed  chan func()
	code    string // the plugins' folder
	data    string
	trashed []string
	repos   []string // what was asked of GitHub, in order
	pages   []string // the web pages opened
	picks   string   // what the file dialog is to choose
	removed bool     // the listener says yes to removing
}

func newInstallEnv(t *testing.T) *installEnv {
	t.Helper()
	e := &installEnv{landed: make(chan func(), 16), removed: true}
	e.code, e.data = filepath.Join(t.TempDir(), "plugins"), filepath.Join(t.TempDir(), "plugin-data")
	a := newTestApp()
	a.settings = defaultSettings()
	a.onPost = func(fn func()) { e.landed <- fn }
	e.a = a

	was := [...]any{pluginDirs, fetchPlugin, fetchRelease, latestPlugin, openPlugin, pickPlugin, confirmRemovePlugin, trashItem, revealFolder}
	pluginDirs = func() (string, string, bool) { return e.code, e.data, true }
	pickPlugin = func(_ *app, _ bool, got func(string)) { got(e.picks) }
	confirmRemovePlugin = func(_ *app, _ string, yes func()) {
		if e.removed {
			yes()
		}
	}
	trashItem = func(path string) error { e.trashed = append(e.trashed, path); return os.RemoveAll(path) }
	revealFolder = func(string) {}
	wasOpen := openWebPage
	openWebPage = func(url string) error { e.pages = append(e.pages, url); return nil }
	t.Cleanup(func() {
		openWebPage = wasOpen
		pluginDirs, fetchPlugin, fetchRelease = was[0].(func() (string, string, bool)), was[1].(func(string, string) (*plugin.Pending, error)), was[2].(func(*plugin.Release, string) (*plugin.Pending, error))
		latestPlugin, openPlugin, pickPlugin = was[3].(func(string) (*plugin.Release, error)), was[4].(func(string, string) (*plugin.Pending, error)), was[5].(func(*app, bool, func(string)))
		confirmRemovePlugin, trashItem, revealFolder = was[6].(func(*app, string, func())), was[7].(func(string) error), was[8].(func(string))
		a.closePlugins()
	})
	e.tt = ui.NewTester(a.settingsView, 560, 3000)
	e.tt.Frame()
	return e
}

// land lets in what the app was waiting to post, and draws again.
func (e *installEnv) land(t *testing.T) {
	t.Helper()
	wait(t, e.landed, "an answer")()
	e.tt.Frame()
}

// folder makes a plugin of the ID, version and extra manifest, whose search
// answers answer, in a folder of its own, and returns the folder.
func folder(t *testing.T, id, version, extra, answer string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "src-"+id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{"id":%q,"name":%q,"version":%q%s}`, id, strings.ToUpper(id), version, extra)
	js := fmt.Sprintf(`export function search() { return { tracks: [{ id: "1", name: %q }] }; }`, answer)
	for name, body := range map[string]string{"plugin.json": manifest, "index.js": js} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// fromGitHub makes fetchPlugin and fetchRelease answer with the folder, as if it
// were repo's release of version, recording what was asked of GitHub.
func (e *installEnv) fromGitHub(t *testing.T, repo, version, dir string) {
	t.Helper()
	pending := func(into string) (*plugin.Pending, error) {
		p, err := plugin.Open(dir, into)
		if err != nil {
			return nil, err
		}
		p.Repo, p.Version = repo, version
		return p, nil
	}
	fetchPlugin = func(got, into string) (*plugin.Pending, error) {
		e.repos = append(e.repos, got)
		if got != repo {
			return nil, plugin.ErrNoRepo
		}
		return pending(into)
	}
	fetchRelease = func(rel *plugin.Release, into string) (*plugin.Pending, error) {
		e.repos = append(e.repos, rel.Repo)
		return pending(into)
	}
}

// A plugin chosen from this machine is asked about before anything is installed:
// where it is from, what it may reach and the programs it asks to run are said,
// and nothing is in the plugins' folder or among the sources until the listener
// allows it; then it is one, at once, with its settings in the window.
func TestInstallFromAFolderAsksFirst(t *testing.T) {
	e := newInstallEnv(t)
	src := folder(t, "tone", "1.0.0", `,"permissions":{"network":["api.example.com"],"process":["ffmpeg"]}`, "x")
	e.picks = src
	e.a.installFromDisk(true)
	e.land(t)
	if !e.a.installer.open || e.a.installer.pending == nil {
		t.Fatalf("not asked: %+v", e.a.installer)
	}
	for _, want := range []string{"来自 " + src, "可访问网站：api.example.com", "可启动程序：ffmpeg", "它想运行 ffmpeg"} {
		if !e.tt.HasText(want) {
			t.Errorf("no %q in %v", want, e.tt.Texts())
		}
	}
	if e.a.source("tone") != nil || len(e.a.plugins) != 0 {
		t.Error("a source before the listener allowed it")
	}
	if _, err := os.Stat(filepath.Join(e.code, "tone")); err == nil {
		t.Error("in the plugins' folder before the listener allowed it")
	}

	if err := e.tt.Click("安装"); err != nil {
		t.Fatal(err)
	}
	e.land(t)
	pl := e.a.source("tone")
	if pl == nil || len(e.a.plugins) != 1 || e.a.installer.open {
		t.Fatalf("not installed: source %v, %d plugins, dialog %v, error %q", pl, len(e.a.plugins), e.a.installer.open, e.a.installer.err)
	}
	if _, err := os.Stat(filepath.Join(e.code, "tone", "plugin.json")); err != nil {
		t.Errorf("not in the plugins' folder: %v", err)
	}
	if !e.tt.HasText("音源：TONE") || !e.tt.HasText("来自这台电脑") {
		t.Errorf("its group is not in the settings: %v", e.tt.Texts())
	}
	// It is a source to switch to, with no need to open the app again.
	if got := e.a.headers.on.Load(); got == nil || len(*got) != 1 {
		t.Error("the plugin's request headers are not asked for")
	}

	// Not wanted: nothing of it installs.
	e.picks = folder(t, "other", "1.0.0", "", "y")
	e.a.installFromDisk(true)
	e.land(t)
	if err := e.tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if e.a.installer.pending != nil || e.a.source("other") != nil {
		t.Errorf("a plugin the listener refused: pending %v", e.a.installer.pending)
	}
	if _, err := os.Stat(filepath.Join(e.code, "other")); err == nil {
		t.Error("a plugin the listener refused is in the plugins' folder")
	}
}

// A repository written as the listener may, or its address from its page, is what
// GitHub is asked for, and what comes back is asked about as one from the disk
// is; what is no repository is said so and asks GitHub nothing; one that is not
// there, or needs a newer app, says so.
func TestInstallFromGitHubTakesWhatIsWritten(t *testing.T) {
	e := newInstallEnv(t)
	e.fromGitHub(t, "o/tone", "v1.0.0", folder(t, "tone", "1.0.0", "", "x"))
	e.a.askForRepo()
	e.tt.Frame()

	e.tt.Type("nonsense")
	e.tt.Frame()
	if err := e.tt.Click("安装"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if !e.tt.HasText("请写成 owner/repo") || len(e.repos) != 0 {
		t.Errorf("a bad repository: %v, asked %v", e.tt.Texts(), e.repos)
	}

	e.a.installer.repo = ""
	e.tt.Frame()
	e.tt.Type("https://github.com/o/tone/tree/main")
	e.tt.Frame()
	if err := e.tt.Click("安装"); err != nil {
		t.Fatal(err)
	}
	e.land(t)
	if len(e.repos) != 1 || e.repos[0] != "o/tone" {
		t.Fatalf("asked GitHub for %v", e.repos)
	}
	if !e.tt.HasText("来自 o/tone") || !e.tt.HasText("安装 TONE v1.0.0？") {
		t.Fatalf("not asked about it: %v", e.tt.Texts())
	}
	if err := e.tt.Click("安装"); err != nil {
		t.Fatal(err)
	}
	e.land(t)
	pl := e.a.pluginByID("tone")
	if pl == nil || pl.Origin() == nil || pl.Origin().Repo != "o/tone" || pl.Origin().Version != "v1.0.0" {
		t.Fatalf("installed %v", pl)
	}
	if !e.tt.HasText("来自 GitHub：o/tone · v1.0.0") {
		t.Errorf("where it is from is not said: %v", e.tt.Texts())
	}

	// What can't be had says why.
	for name, c := range map[string]struct {
		err  error
		want string
	}{
		"missing": {plugin.ErrNoRepo, "GitHub 上没有这个公开仓库"},
		"newer":   {&plugin.NeedsNewerApp{Plugin: "Tone", Version: "9.0.0"}, "Tone 需要 MyGo 云音乐 9.0.0 或更新的版本"},
		"other":   {errors.New("GitHub answered 500"), "GitHub answered 500"},
	} {
		fetchPlugin = func(string, string) (*plugin.Pending, error) { return nil, c.err }
		e.a.askForRepo()
		e.a.installer.repo = "o/else"
		e.a.installFromRepo()
		e.land(t)
		if !e.tt.HasText(c.want) {
			t.Errorf("%s: no %q in %v", name, c.want, e.tt.Texts())
		}
	}
}

// A plugin of an ID that another has, as one dropped in does, is not installed
// over it, and the dialog says so.
func TestInstallOverAPluginOfTheSameIDIsRefused(t *testing.T) {
	e := newInstallEnv(t)
	putPlugin(t, e.code, "my-copy", `{"id":"dup","name":"DUP"}`, searchJSForTests)
	e.a.usePlugins(e.code, e.data)
	e.picks = folder(t, "dup", "2.0.0", "", "new")
	e.a.installFromDisk(true)
	e.land(t)
	if err := e.tt.Click("安装"); err != nil {
		t.Fatal(err)
	}
	e.land(t)
	if !e.tt.HasText("already installed") || len(e.a.plugins) != 1 {
		t.Errorf("%v, %d plugins", e.tt.Texts(), len(e.a.plugins))
	}
}

const searchJSForTests = `export function search() { return { tracks: [] }; }`

// A plugin installed from GitHub is checked for a new version, the daily check
// asking only for those that came from GitHub and only once a day; one found is
// offered, and updating takes the new in place of the old without the app opening
// again, the old stopped, what the app held of it let go, and the new one the
// source, as it was.
func TestAPluginIsUpdatedInPlace(t *testing.T) {
	e := newInstallEnv(t)
	e.fromGitHub(t, "o/tone", "v1.0.0", folder(t, "tone", "1.0.0", "", "old"))
	e.a.installer.pending, _ = fetchPlugin("o/tone", e.code)
	e.a.confirmInstall()
	e.land(t)
	old := e.a.pluginByID("tone")
	if old == nil {
		t.Fatal("not installed")
	}
	putPlugin(t, e.code, "dropped", `{"id":"dropped"}`, searchJSForTests)
	e.a.usePlugins(e.code, e.data)
	e.a.switchSource("tone")
	e.land(t) // its home, which it has none of
	if tracks, err := old.Search("q", "tracks", 0); err != nil || tracks.Tracks[0].Name != "old" {
		t.Fatalf("%v, %v", tracks, err)
	}

	// The daily check: one plugin has an origin.
	var asked []string
	latestPlugin = func(repo string) (*plugin.Release, error) {
		asked = append(asked, repo)
		return &plugin.Release{Repo: repo, Version: "v1.1.0"}, nil
	}
	e.a.checkPluginUpdates()
	e.land(t)
	if len(asked) != 1 || asked[0] != "o/tone" || e.a.settings.PluginUpdates["tone"] != "v1.1.0" {
		t.Fatalf("asked %v, found %v", asked, e.a.settings.PluginUpdates)
	}
	e.a.checkPluginUpdates()
	if len(asked) != 1 {
		t.Errorf("asked again within a day: %v", asked)
	}
	e.a.settings.PluginsChecked = time.Now().Add(-25 * time.Hour)
	e.a.checkPluginUpdates()
	e.land(t)
	if len(asked) != 2 {
		t.Errorf("not asked a day on: %v", asked)
	}
	if !e.tt.HasText("更新到 v1.1.0") {
		t.Fatalf("the update is not offered: %v", e.tt.Texts())
	}

	// Updating. What the app held of the old, asked of it, is let go.
	e.a.stateOf("tone").asked = true
	e.fromGitHub(t, "o/tone", "v1.1.0", folder(t, "tone", "1.1.0", "", "new"))
	if err := e.tt.Click("更新到 v1.1.0"); err != nil {
		t.Fatal(err)
	}
	e.land(t)
	now := e.a.pluginByID("tone")
	if now == nil || now == old || e.a.source("tone") != any(now) {
		t.Fatalf("not replaced: %v", now)
	}
	if tracks, err := now.Search("q", "tracks", 0); err != nil || tracks.Tracks[0].Name != "new" {
		t.Errorf("answers %v, %v", tracks, err)
	}
	if now.Origin().Version != "v1.1.0" || e.a.settings.PluginUpdates["tone"] != "" || len(e.a.plugins) != 2 {
		t.Errorf("origin %+v, updates %v, %d plugins", now.Origin(), e.a.settings.PluginUpdates, len(e.a.plugins))
	}
	if e.a.activeID() != "tone" {
		t.Errorf("the source shown became %q", e.a.activeID())
	}
	if e.a.stateOf("tone").asked {
		t.Error("what the old plugin said of its account is held for the new")
	}
	if e.tt.HasText("更新到 v1.1.0") {
		t.Error("still offered what was installed")
	}
}

// An update that asks for more than the listener allowed, a program or a site
// the old did not, is asked about first, and is not installed until they say
// so; the programs the old was let run are not the new's, so what it is allowed
// starts again.
func TestUpdateThatAsksForMoreAsksFirst(t *testing.T) {
	e := newInstallEnv(t)
	e.fromGitHub(t, "o/tone", "v1.0.0", folder(t, "tone", "1.0.0", `,"permissions":{"process":["ffmpeg"]}`, "old"))
	e.a.installer.pending, _ = fetchPlugin("o/tone", e.code)
	e.a.confirmInstall()
	e.land(t)
	old := e.a.pluginByID("tone")
	if err := old.Grant(true); err != nil {
		t.Fatal(err)
	}
	e.a.settings.PluginUpdates = map[string]string{"tone": "v2.0.0"}
	e.a.installer.found = map[string]*plugin.Release{"tone": {Repo: "o/tone", Version: "v2.0.0"}}

	e.fromGitHub(t, "o/tone", "v2.0.0", folder(t, "tone", "2.0.0", `,"permissions":{"process":["ffmpeg","curl"]}`, "new"))
	e.tt.Frame()
	if err := e.tt.Click("更新到 v2.0.0"); err != nil {
		t.Fatal(err)
	}
	e.land(t)
	if e.a.pluginByID("tone") != old || !e.a.installer.open || e.a.installer.pending == nil {
		t.Fatalf("installed without asking: %+v", e.a.installer)
	}
	for _, want := range []string{"这个新版本要求的权限比你允许过的更多", "可启动程序：ffmpeg、curl"} {
		if !e.tt.HasText(want) {
			t.Errorf("no %q in %v", want, e.tt.Texts())
		}
	}

	if err := e.tt.Click("安装"); err != nil {
		t.Fatal(err)
	}
	e.land(t)
	now := e.a.pluginByID("tone")
	if now == old || now == nil {
		t.Fatalf("not installed: %v", now)
	}
	if now.Granted() {
		t.Error("the new version runs the programs the old was allowed, with others among them")
	}

	// One that asks for nothing new needs no asking, and keeps what was allowed.
	if err := now.Grant(true); err != nil {
		t.Fatal(err)
	}
	e.a.settings.PluginUpdates = map[string]string{"tone": "v2.1.0"}
	e.a.installer.found = map[string]*plugin.Release{"tone": {Repo: "o/tone", Version: "v2.1.0"}}
	e.fromGitHub(t, "o/tone", "v2.1.0", folder(t, "tone", "2.1.0", `,"permissions":{"process":["ffmpeg"]}`, "newer"))
	e.a.updatePlugin(now)
	e.land(t)
	if latest := e.a.pluginByID("tone"); latest == now || !latest.Granted() || e.a.installer.pending != nil {
		t.Errorf("a plain update: replaced %v, still allowed %v, asked %v", latest != now, latest.Granted(), e.a.installer.pending != nil)
	}
}

// Removing a plugin asks first; once the listener says so it is gone from the
// sources and the switcher, the app shows NetEase's music if it was the one shown,
// its folder goes to the trash, and what it kept stays; if they do not say so,
// nothing changes.
func TestRemovingAPlugin(t *testing.T) {
	e := newInstallEnv(t)
	putPlugin(t, e.code, "tone", `{"id":"tone","name":"TONE"}`, searchJSForTests)
	e.a.usePlugins(e.code, e.data)
	pl := e.a.pluginByID("tone")
	if err := pl.SetSetting("k", "kept"); err != nil {
		t.Fatal(err)
	}
	e.a.switchSource("tone")
	e.tt.Frame()

	e.removed = false
	if err := e.tt.Click("更多：tone"); err != nil {
		t.Fatal(err)
	}
	if err := e.tt.ChooseMenuItem("移除…"); err != nil {
		t.Fatalf("%v; the menu shows %q", err, e.tt.Menu())
	}
	if e.a.pluginByID("tone") == nil || len(e.trashed) != 0 {
		t.Fatal("removed without the listener's yes")
	}

	e.removed = true
	e.tt.Frame()
	if err := e.tt.Click("更多：tone"); err != nil {
		t.Fatal(err)
	}
	if err := e.tt.ChooseMenuItem("移除…"); err != nil {
		t.Fatal(err)
	}
	if e.a.pluginByID("tone") != nil || e.a.source("tone") != nil || len(e.a.plugins) != 0 {
		t.Error("still a source")
	}
	if e.a.activeID() != netease.SourceID {
		t.Errorf("shows %q", e.a.activeID())
	}
	// Its folder goes in the background.
	deadline := time.After(2 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(e.code, "tone")); err != nil {
			break
		}
		select {
		case <-deadline:
			t.Fatal("its folder is still there")
		case <-time.After(5 * time.Millisecond):
		}
	}
	if len(e.trashed) != 1 || e.trashed[0] != filepath.Join(e.code, "tone") {
		t.Errorf("trashed %v", e.trashed)
	}
	if b, _ := os.ReadFile(filepath.Join(e.data, "tone", "settings.json")); !strings.Contains(string(b), "kept") {
		t.Errorf("what it kept is gone: %s", b)
	}
	e.tt.Frame()
	if e.tt.HasText("音源：TONE") {
		t.Error("its group is still in the settings")
	}
}

// The plugins' group is in the settings with none installed, as it is where the
// first one is, with the ways to install one; the plugins' folder can be shown.
func TestSettingsOfferToInstallPlugins(t *testing.T) {
	e := newInstallEnv(t)
	if !e.tt.HasText("音源插件") {
		t.Fatalf("no group for plugins: %v", e.tt.Texts())
	}
	shown := ""
	revealFolder = func(path string) { shown = path }
	if err := e.tt.Click("安装插件"); err != nil {
		t.Fatal(err)
	}
	for _, item := range []string{"从 GitHub 安装…", "从文件夹安装…", "从 Zip 安装…", "浏览插件…", "显示插件文件夹"} {
		if !contains(e.tt.Menu(), item) {
			t.Errorf("no %q in %q", item, e.tt.Menu())
		}
	}
	if err := e.tt.ChooseMenuItem("显示插件文件夹"); err != nil {
		t.Fatal(err)
	}
	if shown != e.code {
		t.Errorf("showed %q, want %q", shown, e.code)
	}
	e.tt.Frame()
	// The page that lists plugins is the browser's, not a window of the app's.
	if err := e.tt.Click("安装插件"); err != nil {
		t.Fatal(err)
	}
	if err := e.tt.ChooseMenuItem("浏览插件…"); err != nil {
		t.Fatal(err)
	}
	if len(e.pages) != 1 || e.pages[0] != "https://mygo-music.egoist.dev/plugins" {
		t.Errorf("opened %v", e.pages)
	}
	e.tt.Frame()
	if err := e.tt.Click("安装插件"); err != nil {
		t.Fatal(err)
	}
	if err := e.tt.ChooseMenuItem("从 GitHub 安装…"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if !e.a.installer.open || !e.a.installer.github || !e.tt.HasText("写下有插件的仓库") {
		t.Errorf("asks for no repository: %+v", e.a.installer)
	}
}

func contains(items []string, s string) bool {
	for _, it := range items {
		if it == s {
			return true
		}
	}
	return false
}

// With NM_FRAMES set, saves the settings window's plugins, light and dark: with
// none installed, with the demo, and the question asked of a plugin that wants
// the network and programs.
func TestInstallFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	for _, dark := range []bool{false, true} {
		suffix := ""
		if dark {
			suffix = "-dark"
		}
		e := newInstallEnv(t)
		e.tt.SetDark(dark)
		e.tt.SetSize(560, 1500)
		save := func(name string) {
			e.tt.Frame()
			f, err := os.Create(filepath.Join(dir, name+suffix+".png"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			png.Encode(f, e.tt.Image())
		}
		save("plugins-none")
		e.a.usePlugins(filepath.Join("..", "plugin", "testdata"), e.data)
		save("plugins-demo")
		e.picks = folder(t, "tone", "1.0.0", `,"permissions":{"network":["api.example.com"],"process":["ffmpeg"]}`, "x")
		e.a.installFromDisk(true)
		// What the demo asked of its account comes first.
		for e.a.installer.pending == nil {
			e.land(t)
		}
		e.tt.Advance(400 * time.Millisecond)
		save("plugins-install-ask")
	}
}

// As the window comes up the app opens the links that came before it and checks
// the plugins from GitHub for new versions, which it had never done by itself:
// the check was only made when the listener asked of one plugin.
func TestWindowUpChecksPluginsAndOpensLinks(t *testing.T) {
	e := newInstallEnv(t)
	e.fromGitHub(t, "o/tone", "v1.0.0", folder(t, "tone", "1.0.0", "", "x"))
	e.a.installer.pending, _ = fetchPlugin("o/tone", e.code)
	e.a.confirmInstall()
	e.land(t)
	var asked []string
	latestPlugin = func(repo string) (*plugin.Release, error) {
		asked = append(asked, repo)
		return &plugin.Release{Repo: repo, Version: "v1.1.0"}, nil
	}
	wasShow := showSettings
	showSettings = func(*app) {}
	defer func() { showSettings = wasShow }()
	e.a.linkOpened(linkScheme + "://install-plugin?repo=o/waiting")

	e.a.windowUp()
	e.land(t)
	if len(asked) != 1 || asked[0] != "o/tone" || e.a.settings.PluginUpdates["tone"] != "v1.1.0" {
		t.Errorf("asked %v, found %v", asked, e.a.settings.PluginUpdates)
	}
	if !e.a.installer.open || e.a.installer.repo != "o/waiting" {
		t.Errorf("the link that came first was not opened: %+v", e.a.installer)
	}
}
