package plugin

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"netease-music-mygo/internal/music"
)

// writePlugin makes a plugin folder holding plugin.json and, if given, other
// files, and answers its path.
func writePlugin(t *testing.T, manifest string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "plugin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// A plugin.json leaves out what has a sensible default (the name is the ID, the
// playback a stream, the main file index.js) and says what is wrong with one
// that can't be run: no ID, an ID that leads out of the plugin's data folder, a
// main file outside its own, a version that is not one, or one of an app newer
// than this.
func TestManifestDefaultsAndChecks(t *testing.T) {
	defer SetAppVersion("")
	SetAppVersion("0.1.28")
	m, err := readManifest(writePlugin(t, `{"id":"demo"}`, nil))
	if err != nil || m.Name != "demo" || m.Playback != music.PlaybackStream || m.Main != "index.js" {
		t.Fatalf("defaults: %+v, %v", m, err)
	}
	for name, c := range map[string]struct{ json, want string }{
		"no id":       {`{"name":"x"}`, "needs an id"},
		"dotdot":      {`{"id":".."}`, "can't be called"},
		"slash":       {`{"id":"a/b"}`, "can't be called"},
		"main out":    {`{"id":"x","main":"../x.js"}`, "main must be a file"},
		"bad version": {`{"id":"x","minAppVersion":"soon"}`, "isn't a version"},
		"bad process": {`{"id":"x","permissions":{"process":3}}`, "process is a list"},
	} {
		if _, err := readManifest(writePlugin(t, c.json, nil)); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", name, err, c.want)
		}
	}
	_, err = readManifest(writePlugin(t, `{"id":"x","name":"新插件","minAppVersion":"0.2.0"}`, nil))
	var newer *NeedsNewerApp
	if !errors.As(err, &newer) || newer.Plugin != "新插件" || newer.Version != "0.2.0" {
		t.Errorf("a plugin of a newer app: %v", err)
	}
	if _, err := readManifest(writePlugin(t, `{"id":"x","minAppVersion":"0.1.28"}`, nil)); err != nil {
		t.Errorf("a plugin of this very version: %v", err)
	}
	SetAppVersion("")
	if _, err := readManifest(writePlugin(t, `{"id":"x","minAppVersion":"9.0.0"}`, nil)); err != nil {
		t.Errorf("a development build holds nothing back: %v", err)
	}
}

// Tags that are numbers compare as such, so that a release taken back does not
// read as new; any other tag has no order, and one that differs is new.
func TestIsNewer(t *testing.T) {
	for _, c := range []struct {
		latest, installed string
		want              bool
	}{
		{"v1.2.0", "v1.1.9", true},
		{"1.10", "1.9", true},
		{"v1.2", "v1.2.0", false},
		{"v1.1.0", "v1.2.0", false},
		{"v1.2.0", "v1.2.0", false},
		{"abc123", "def456", true},
		{"abc123", "abc123", false},
		{"1.2.3-beta", "1.2.2", true},
	} {
		if got := IsNewer(c.latest, c.installed); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.latest, c.installed, got, c.want)
		}
	}
}

// The process permission is a list of programs, or true for any, from before
// they were named; a path only matches a path and a name a name, a pattern does
// not cross a directory, and a path from the plugin's directory is under it.
func TestProgramsAllow(t *testing.T) {
	var perms Permissions
	if err := json.Unmarshal([]byte(`{"process":true}`), &perms); err != nil || !perms.Process.Any() {
		t.Fatalf("true: %+v, %v", perms, err)
	}
	if err := json.Unmarshal([]byte(`{"process":false}`), &perms); err != nil || len(perms.Process) != 0 {
		t.Fatalf("false: %+v, %v", perms, err)
	}
	dir := filepath.Join(string(filepath.Separator), "plugins", "demo")
	programs := Programs{"bin/helper-*", "ffmpeg"}
	for _, c := range []struct {
		command string
		want    bool
	}{
		{filepath.Join(dir, "bin", "helper-arm64"), true},
		{filepath.Join(dir, "bin", "other"), false},
		{filepath.Join(dir, "bin", "sub", "helper-x"), false},
		{filepath.Join(string(filepath.Separator), "usr", "bin", "helper-x"), false},
		{"ffmpeg", true},
		{"ffprobe", false},
		{filepath.Join(string(filepath.Separator), "usr", "bin", "ffmpeg"), false},
	} {
		if got := programs.Allows(dir, c.command); got != c.want {
			t.Errorf("Allows(%q) = %v, want %v", c.command, got, c.want)
		}
	}
	if !(Programs{"*"}).Allows(dir, "anything") || (Programs{}).Allows(dir, "ffmpeg") {
		t.Error("* allows any, and none allows none")
	}
	if runtime.GOOS == "windows" && !(Programs{"ffmpeg"}).Allows(dir, "FFmpeg.exe") {
		t.Error("a program needn't say .exe, or its case, on Windows")
	}
}

// A plugin keeps its settings, its storage and whether the listener let it
// start programs, in its own data folder, for the next time; one plugin's data
// is not another's.
func TestPluginKeepsItsState(t *testing.T) {
	data := t.TempDir()
	dir := writePlugin(t, `{"id":"demo"}`, nil)
	p, err := newPlugin(dir, data)
	if err != nil {
		t.Fatal(err)
	}
	if p.Setting("server") != "" || p.Granted() {
		t.Fatalf("a new plugin has %q, granted %v", p.Setting("server"), p.Granted())
	}
	if err := p.keepSetting("server", "https://nas.local"); err != nil {
		t.Fatal(err)
	}
	if err := p.keepGranted(true); err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	p.storage["token"] = json.RawMessage(`"abc"`)
	err = p.writeJSON("storage.json", p.storage)
	p.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}

	again, err := newPlugin(dir, data)
	if err != nil {
		t.Fatal(err)
	}
	if again.Setting("server") != "https://nas.local" || !again.Granted() || string(again.storage["token"]) != `"abc"` {
		t.Errorf("not kept: server %q, granted %v, token %s", again.Setting("server"), again.Granted(), again.storage["token"])
	}
	other, err := newPlugin(writePlugin(t, `{"id":"other"}`, nil), data)
	if err != nil || other.Setting("server") != "" || other.Granted() {
		t.Errorf("another plugin has this one's: %q, %v, %v", other.Setting("server"), other.Granted(), err)
	}
}

// A plugin that translated itself shows its name, settings and sidebar in the
// listener's language, and as it is where it did not; a locale that is not
// JSON fails the plugin, rather than going unread.
func TestPluginInfoIsTranslated(t *testing.T) {
	defer SetLanguage("zh-CN")
	manifest := `{"id":"demo","name":"Demo","settings":[{"key":"server","label":"Server","help":"Where it runs"}],"navigation":[{"title":"Library"}]}`
	dir := writePlugin(t, manifest, map[string]string{"locales/zh-CN.json": `{"Demo":"演示","Server":"服务器","Library":"曲库"}`})
	p, err := newPlugin(dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	info := p.Info()
	if info.Name != "演示" || info.Settings[0].Label != "服务器" || info.Settings[0].Help != "Where it runs" || info.Navigation[0].Title != "曲库" {
		t.Errorf("in Chinese: %+v", info)
	}
	SetLanguage("en")
	if info := p.Info(); info.Name != "Demo" || info.Settings[0].Label != "Server" {
		t.Errorf("in English: %+v", info)
	}
	if p.manifest.Settings[0].Label != "Server" {
		t.Error("translating changed the plugin's own manifest")
	}
	bad := writePlugin(t, `{"id":"bad"}`, map[string]string{"locales/zh-CN.json": `not json`})
	if _, err := newPlugin(bad, t.TempDir()); err == nil {
		t.Error("a locale that is not JSON was ignored")
	}
}
