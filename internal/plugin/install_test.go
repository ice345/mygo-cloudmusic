package plugin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

// zipOf makes a zip of the files given, by name, with the modes in modes where
// it has one, and a link for each name in links.
func zipOf(t *testing.T, files map[string]string, modes map[string]os.FileMode, links map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0o644)
		if m, ok := modes[name]; ok {
			h.SetMode(m)
		}
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(body))
	}
	for name, target := range links {
		h := &zip.FileHeader{Name: name}
		h.SetMode(os.ModeSymlink | 0o777)
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(target))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// pluginZip is a zip of a plugin of the ID and version, under the folder top as
// GitHub's archives have it, or at its top if there is none.
func pluginZip(t *testing.T, id, version, top, code string) []byte {
	t.Helper()
	manifest := fmt.Sprintf(`{"id":%q,"name":%q,"version":%q}`, id, strings.ToUpper(id), version)
	files := map[string]string{"plugin.json": manifest, "index.js": code}
	if top != "" {
		for name, body := range files {
			delete(files, name)
			files[top+"/"+name] = body
		}
	}
	return zipOf(t, files, nil, nil)
}

const searchJS = `export function search() { return { tracks: [] }; }`

// repoStub is what a repository on the stand-in for GitHub offers.
type repoStub struct {
	tag    string            // of its release, or none
	commit string            // the latest commit's, if there is no release
	zip    []byte            // what its release or commit downloads as
	assets map[string][]byte // what else its release offers, by name
	status int               // what its API answers instead, if set
}

// fakeGitHub stands in for GitHub's API and its downloads, with the repositories
// given, and counts the requests it gets.
func fakeGitHub(t *testing.T, repos map[string]*repoStub) (srv *httptest.Server, asked *atomic.Int32) {
	t.Helper()
	asked = new(atomic.Int32)
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		switch {
		case len(parts) == 5 && parts[0] == "repos" && parts[3] == "releases" && parts[4] == "latest":
			stub := repos[parts[1]+"/"+parts[2]]
			switch {
			case stub != nil && stub.status != 0:
				w.WriteHeader(stub.status)
			case stub == nil || stub.tag == "":
				http.NotFound(w, r)
			default:
				type asset struct {
					Name string `json:"name"`
					URL  string `json:"browser_download_url"`
				}
				var assets []asset
				for name := range stub.assets {
					assets = append(assets, asset{name, srv.URL + "/asset/" + parts[1] + "/" + parts[2] + "/" + name})
				}
				json.NewEncoder(w).Encode(map[string]any{
					"tag_name": stub.tag, "zipball_url": srv.URL + "/zip/" + parts[1] + "/" + parts[2], "assets": assets,
				})
			}
		case len(parts) == 5 && parts[0] == "repos" && parts[3] == "commits":
			stub := repos[parts[1]+"/"+parts[2]]
			if stub == nil || stub.commit == "" {
				http.NotFound(w, r)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"sha": stub.commit})
		case len(parts) == 5 && parts[0] == "repos" && parts[3] == "zipball":
			w.Write(repos[parts[1]+"/"+parts[2]].zip)
		case len(parts) == 3 && parts[0] == "zip":
			w.Write(repos[parts[1]+"/"+parts[2]].zip)
		case len(parts) == 4 && parts[0] == "asset":
			w.Write(repos[parts[1]+"/"+parts[2]].assets[parts[3]])
		default:
			http.NotFound(w, r)
		}
	}))
	was := githubAPI
	githubAPI = srv.URL
	t.Cleanup(func() { githubAPI = was; srv.Close() })
	return srv, asked
}

// Writing a repository as the listener may, owner/repo or its address from any
// page of it, or to clone it, names the repository; what isn't one, or leads
// elsewhere in an address, is refused.
func TestParseRepo(t *testing.T) {
	for in, want := range map[string]string{
		"egoist/demo":                                "egoist/demo",
		"  egoist/demo  ":                            "egoist/demo",
		"https://github.com/egoist/demo":             "egoist/demo",
		"HTTPS://GitHub.com/egoist/demo/":            "egoist/demo",
		"github.com/egoist/demo/tree/main/sub?x=1#y": "egoist/demo",
		"www.github.com/egoist/demo.git":             "egoist/demo",
		"git@github.com:egoist/demo.git":             "egoist/demo",
		"a/b-c_d.e":                                  "a/b-c_d.e",
	} {
		if got, err := ParseRepo(in); err != nil || got != want {
			t.Errorf("%q: %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "demo", "a/b/c", "../x", "a/..", "a/../b", "https://evil.example/a/b", "a b/c", "a/b?x", "file:///a/b"} {
		if got, err := ParseRepo(in); err == nil {
			t.Errorf("%q is no repository, but was %q", in, got)
		}
	}
}

// What a repository offers is its latest release, as the zip for this platform
// or else one for none, or else its source; without a release, its default
// branch's latest commit; a repository that isn't there says so, one that
// answers it is busy says that, and one named so as to lead elsewhere is not
// asked about at all.
func TestLatestChoosesWhatToInstall(t *testing.T) {
	general := zipOf(t, map[string]string{"plugin.json": `{"id":"x"}`}, nil, nil)
	// Zips for every platform but this one.
	others := map[string][]byte{}
	for _, goos := range []string{"darwin", "linux", "windows"} {
		for _, goarch := range []string{"amd64", "arm64"} {
			if goos != runtime.GOOS || goarch != runtime.GOARCH {
				others["p-"+goos+"-"+goarch+".zip"] = general
			}
		}
	}
	_, asked := fakeGitHub(t, map[string]*repoStub{
		"o/for-me":    {tag: "v2.0.0", zip: general, assets: map[string][]byte{"p-" + runtime.GOOS + "-" + runtime.GOARCH + ".zip": general, "p-other.zip": general}},
		"o/general":   {tag: "v1.0.0", zip: general, assets: map[string][]byte{"plugin.zip": general, "notes.txt": nil}},
		"o/others":    {tag: "v1.1.0", zip: general, assets: others},
		"o/no-tags":   {commit: "0123456789abcdef0123", zip: general},
		"o/busy":      {status: http.StatusForbidden},
		"o/broken":    {status: http.StatusInternalServerError},
		"o/untouched": {tag: "v1"},
	})
	srv := githubAPI
	for repo, want := range map[string]struct{ version, url string }{
		"o/for-me":  {"v2.0.0", "/asset/o/for-me/p-" + runtime.GOOS + "-" + runtime.GOARCH + ".zip"},
		"o/general": {"v1.0.0", "/asset/o/general/plugin.zip"},
		"o/others":  {"v1.1.0", "/zip/o/others"},
		"o/no-tags": {"0123456789ab", "/repos/o/no-tags/zipball/0123456789abcdef0123"},
	} {
		rel, err := Latest(repo)
		if err != nil || rel.Version != want.version || rel.url != srv+want.url {
			t.Errorf("%s: %+v, %v", repo, rel, err)
		}
	}
	if _, err := Latest("o/missing"); !errors.Is(err, ErrNoRepo) {
		t.Errorf("a repository that isn't there: %v", err)
	}
	if _, err := Latest("o/busy"); err == nil || !strings.Contains(err.Error(), "slow down") {
		t.Errorf("a busy GitHub: %v", err)
	}
	if _, err := Latest("o/broken"); err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("a GitHub that failed: %v", err)
	}
	before := asked.Load()
	for _, repo := range []string{"../../etc/passwd", "a/../b", "a", "o/r?x=1", ""} {
		if _, err := Latest(repo); err == nil {
			t.Errorf("%q was asked about", repo)
		}
	}
	if asked.Load() != before {
		t.Error("a repository named to lead elsewhere was sent to GitHub")
	}
}

// A repository's plugin downloads and is checked into a folder of its own,
// out of the way of the plugins that are installed, with nothing of it run;
// installing it puts it in a folder of its ID with where it came from beside,
// and leaves nothing of what it took.
func TestFetchChecksAndInstallsAPlugin(t *testing.T) {
	fakeGitHub(t, map[string]*repoStub{
		"o/tone": {tag: "v1.2.0", zip: pluginZip(t, "tone", "1.2.0", "o-tone-abc123", searchJS)},
	})
	dir := filepath.Join(t.TempDir(), "plugins")
	pending, err := Fetch("o/tone", dir)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Manifest.ID != "tone" || pending.Version != "v1.2.0" || pending.Repo != "o/tone" {
		t.Errorf("pending %+v", pending)
	}
	for _, e := range must(os.ReadDir(dir)) {
		if !strings.HasPrefix(e.Name(), ".") {
			t.Errorf("%s is among the plugins before it was allowed", e.Name())
		}
	}
	if got := Discover(dir, t.TempDir()); len(got) != 0 {
		t.Errorf("a plugin not yet installed was found: %v", got)
	}

	if err := pending.Install(dir); err != nil {
		t.Fatal(err)
	}
	if entries := must(os.ReadDir(dir)); len(entries) != 1 || entries[0].Name() != "tone" {
		t.Errorf("after installing: %v", entries)
	}
	found := Discover(dir, t.TempDir())
	if len(found) != 1 || found[0].Info().ID != "tone" {
		t.Fatalf("found %v", found)
	}
	defer found[0].Close()
	if o := found[0].Origin(); o == nil || o.Repo != "o/tone" || o.Version != "v1.2.0" {
		t.Errorf("origin %+v", o)
	}

	// One not wanted leaves nothing.
	other, err := Fetch("o/tone", dir)
	if err != nil {
		t.Fatal(err)
	}
	other.Discard()
	if entries := must(os.ReadDir(dir)); len(entries) != 1 {
		t.Errorf("after discarding: %v", entries)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// A new version takes the place of the one it came from, and what the plugin
// keeps stays; but a plugin of that name that is from elsewhere, another
// repository's or one just dropped in, is not replaced, nor is a GitHub one by a
// folder of this machine, which is from nowhere.
func TestInstallReplacesOnlyWhatItCameFrom(t *testing.T) {
	codeOf := func(name string) string {
		return fmt.Sprintf(`export function search() { return { tracks: [{ id: "1", name: %q }] }; }`, name)
	}
	repos := map[string]*repoStub{
		"o/tone":    {tag: "v1.0.0", zip: pluginZip(t, "tone", "1.0.0", "", codeOf("old"))},
		"p/tone":    {tag: "v9.0.0", zip: pluginZip(t, "tone", "9.0.0", "", searchJS)},
		"o/dropped": {tag: "v1", zip: pluginZip(t, "dropped", "1", "", searchJS)},
	}
	fakeGitHub(t, repos)
	dir, data := filepath.Join(t.TempDir(), "plugins"), t.TempDir()
	install := func(repo string) error {
		p, err := Fetch(repo, dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Install(dir); err != nil {
			p.Discard()
			return err
		}
		return nil
	}
	read := func() string {
		b, err := os.ReadFile(filepath.Join(dir, "tone", "index.js"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	if err := install("o/tone"); err != nil {
		t.Fatal(err)
	}
	first := Discover(dir, data)[0]
	if err := first.SetSetting("server", "kept"); err != nil {
		t.Fatal(err)
	}
	first.Close()

	// The repository's next release.
	repos["o/tone"].tag, repos["o/tone"].zip = "v1.1.0", pluginZip(t, "tone", "1.1.0", "", codeOf("new"))
	if err := install("o/tone"); err != nil {
		t.Fatalf("the repository's own update was refused: %v", err)
	}
	updated := Discover(dir, data)[0]
	defer updated.Close()
	if !strings.Contains(read(), `"new"`) || updated.Origin().Version != "v1.1.0" {
		t.Errorf("not updated: %s, %+v", read(), updated.Origin())
	}
	if b, _ := os.ReadFile(filepath.Join(data, "tone", "settings.json")); !strings.Contains(string(b), "kept") {
		t.Errorf("what the plugin kept is gone: %s", b)
	}

	// Another repository's plugin of the name does not take its place.
	if err := install("p/tone"); err == nil || !strings.Contains(err.Error(), "already installed") {
		t.Errorf("another repository's plugin of the name: %v", err)
	}
	if !strings.Contains(read(), `"new"`) {
		t.Errorf("what was there was replaced: %s", read())
	}

	// A folder of this machine is from nowhere, and does not replace one from GitHub.
	folder := filepath.Join(t.TempDir(), "mine")
	os.MkdirAll(folder, 0o755)
	os.WriteFile(filepath.Join(folder, "plugin.json"), []byte(`{"id":"tone"}`), 0o644)
	os.WriteFile(filepath.Join(folder, "index.js"), []byte(codeOf("mine")), 0o644)
	local, err := Open(folder, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := local.Install(dir); err == nil {
		t.Error("a folder of this machine replaced a plugin from GitHub")
	}
	local.Discard()

	// A plugin dropped in has no origin, so no repository's replaces it.
	os.MkdirAll(filepath.Join(dir, "dropped"), 0o755)
	os.WriteFile(filepath.Join(dir, "dropped", "plugin.json"), []byte(`{"id":"dropped"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "dropped", "index.js"), []byte(searchJS), 0o644)
	if err := install("o/dropped"); err == nil || !strings.Contains(err.Error(), "already installed") {
		t.Errorf("a repository's plugin over one dropped in: %v", err)
	}
}

// A plugin whose ID a plugin in another folder has is not installed, though no
// folder of its own name is in the way: the two would be one source.
func TestInstallRefusesAnIDHeldInAnotherFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "plugins")
	os.MkdirAll(filepath.Join(dir, "my-copy"), 0o755)
	os.WriteFile(filepath.Join(dir, "my-copy", "plugin.json"), []byte(`{"id":"tone"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "my-copy", "index.js"), []byte(searchJS), 0o644)
	fakeGitHub(t, map[string]*repoStub{"o/tone": {tag: "v1", zip: pluginZip(t, "tone", "1", "", searchJS)}})
	p, err := Fetch("o/tone", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Discard()
	if err := p.Install(dir); err == nil || !strings.Contains(err.Error(), "my-copy") {
		t.Errorf("installed over a plugin of the ID in another folder: %v", err)
	}
}

// An archive that would write outside the plugin, carry a link, or take more
// than it may, is refused whole, with nothing left of it among the plugins and
// nothing written outside them.
func TestArchivesThatCouldDoHarmAreRefused(t *testing.T) {
	good := map[string]string{"plugin.json": `{"id":"tone"}`, "index.js": searchJS}
	with := func(extra map[string]string) map[string]string {
		files := map[string]string{}
		for k, v := range good {
			files[k] = v
		}
		for k, v := range extra {
			files[k] = v
		}
		return files
	}
	cases := map[string][]byte{
		"outside":   zipOf(t, with(map[string]string{"../evil.txt": "x"}), nil, nil),
		"absolute":  zipOf(t, with(map[string]string{"/tmp/evil.txt": "x"}), nil, nil),
		"deep":      zipOf(t, with(map[string]string{"a/../../evil.txt": "x"}), nil, nil),
		"link":      zipOf(t, good, nil, map[string]string{"data": "/etc/passwd"}),
		"not a zip": []byte("PK nothing"),
	}
	for name, archive := range cases {
		root := t.TempDir()
		path, dir := filepath.Join(root, "p.zip"), filepath.Join(root, "plugins")
		os.WriteFile(path, archive, 0o644)
		if _, err := Open(path, dir); err == nil {
			t.Errorf("%s: unpacked", name)
		}
		for _, e := range must(os.ReadDir(dir)) {
			t.Errorf("%s: left %s", name, e.Name())
		}
		if _, err := os.Stat(filepath.Join(root, "evil.txt")); err == nil {
			t.Errorf("%s: wrote outside the plugin", name)
		}
	}

	// Too many files, and too much.
	defer func(f int, u int64) { maxFiles, maxUnpacked = f, u }(maxFiles, maxUnpacked)
	maxFiles, maxUnpacked = 3, 100
	for name, files := range map[string]map[string]string{
		"many":  with(map[string]string{"a": "1", "b": "2", "c": "3"}),
		"large": with(map[string]string{"big.bin": strings.Repeat("x", 200)}),
	} {
		root := t.TempDir()
		path, dir := filepath.Join(root, "p.zip"), filepath.Join(root, "plugins")
		os.WriteFile(path, zipOf(t, files, nil, nil), 0o644)
		if _, err := Open(path, dir); err == nil {
			t.Errorf("%s: unpacked", name)
		}
	}
}

// A plugin's folder, or a zip of one, from this machine is copied in as an archive
// is unpacked: what's hidden, such as .git, stays behind, a link is refused, a
// program stays one the listener can run, and the version is what plugin.json says.
func TestOpenCopiesAFolderOrAZip(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "mine")
	os.MkdirAll(filepath.Join(folder, ".git"), 0o755)
	os.MkdirAll(filepath.Join(folder, "bin"), 0o755)
	write := func(name, body string, mode os.FileMode) {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("plugin.json", `{"id":"mine","version":"3.1.4"}`, 0o644)
	write("index.js", searchJS, 0o644)
	write(".git/config", "secret", 0o644)
	write(".env", "TOKEN=x", 0o644)
	write("bin/helper", "#!/bin/sh\n", 0o755)

	dir := filepath.Join(t.TempDir(), "plugins")
	p, err := Open(folder, dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != "3.1.4" || p.Repo != "" || p.Path != folder {
		t.Errorf("pending %+v", p)
	}
	if err := p.Install(dir); err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{".git", ".env", originFile} {
		if _, err := os.Stat(filepath.Join(dir, "mine", hidden)); err == nil {
			t.Errorf("%s was copied", hidden)
		}
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(filepath.Join(dir, "mine", "bin", "helper")); err != nil || info.Mode()&0o111 == 0 {
			t.Errorf("a program that was runnable is not: %v %v", info, err)
		}
	}
	if o := Discover(dir, t.TempDir())[0].Origin(); o != nil {
		t.Errorf("a plugin from a folder has an origin: %+v", o)
	}

	// A zip of it, with a program.
	archive := filepath.Join(t.TempDir(), "mine.zip")
	os.WriteFile(archive, zipOf(t, map[string]string{"plugin.json": `{"id":"zipped","version":"2"}`, "index.js": searchJS, "bin/run": "x"},
		map[string]os.FileMode{"bin/run": 0o755}, nil), 0o644)
	z, err := Open(archive, dir)
	if err != nil {
		t.Fatal(err)
	}
	if z.Version != "2" {
		t.Errorf("version %q", z.Version)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(filepath.Join(z.root, "bin", "run")); err != nil || info.Mode()&0o111 == 0 {
			t.Errorf("a program of a zip that was runnable is not: %v %v", info, err)
		}
	}
	z.Discard()

	// A link in the folder is refused.
	if runtime.GOOS != "windows" {
		os.Symlink("/etc/passwd", filepath.Join(folder, "link"))
		if _, err := Open(folder, dir); err == nil {
			t.Error("a folder with a link was copied")
		}
	}
}

// plugin.json is found at the top of what was unpacked, or in the one folder
// it holds, as GitHub's archives have it; anything else isn't a plugin.
func TestPluginRootIsFoundAtTheTopOrInTheOneFolder(t *testing.T) {
	tree := func(files ...string) string {
		dir := t.TempDir()
		for _, f := range files {
			os.MkdirAll(filepath.Dir(filepath.Join(dir, f)), 0o755)
			os.WriteFile(filepath.Join(dir, f), []byte("{}"), 0o644)
		}
		return dir
	}
	top := tree("plugin.json")
	if got, err := pluginRoot(top); err != nil || got != top {
		t.Errorf("top: %q, %v", got, err)
	}
	nested := tree("o-r-abc/plugin.json", "o-r-abc/index.js")
	if got, err := pluginRoot(nested); err != nil || got != filepath.Join(nested, "o-r-abc") {
		t.Errorf("nested: %q, %v", got, err)
	}
	for name, dir := range map[string]string{
		"none":     tree("readme.md"),
		"two":      tree("a/plugin.json", "b/plugin.json"),
		"too deep": tree("a/b/plugin.json"),
		"hidden":   tree(".hidden/plugin.json"),
	} {
		if got, err := pluginRoot(dir); err == nil {
			t.Errorf("%s: found %q", name, got)
		}
	}
}

// A plugin can't take the ID of one of the app's own sources, which it would
// stand in for with what they keep, nor an ID that can't be a folder's.
func TestPluginsCannotTakeTheAppsOwnIDs(t *testing.T) {
	for _, id := range []string{"netease", "local"} {
		folder := filepath.Join(t.TempDir(), "p")
		os.MkdirAll(folder, 0o755)
		os.WriteFile(filepath.Join(folder, "plugin.json"), []byte(fmt.Sprintf(`{"id":%q}`, id)), 0o644)
		os.WriteFile(filepath.Join(folder, "index.js"), []byte(searchJS), 0o644)
		if _, err := Open(folder, filepath.Join(t.TempDir(), "plugins")); err == nil || !strings.Contains(err.Error(), "can't be called") {
			t.Errorf("%s: %v", id, err)
		}
	}
}

// A plugin that isn't a secure address's download is not fetched: a repository
// can't lead the app to fetch from anywhere that isn't GitHub's, or without TLS.
func TestOnlySecureDownloadsAreFetched(t *testing.T) {
	_, asked := fakeGitHub(t, nil)
	before := asked.Load()
	for _, url := range []string{"http://example.com/p.zip", "file:///etc/passwd", "ftp://example.com/p.zip", "//example.com/p.zip"} {
		_, err := FetchRelease(&Release{Repo: "o/r", Version: "v1", url: url}, filepath.Join(t.TempDir(), "plugins"))
		if err == nil || !strings.Contains(err.Error(), "secure") {
			t.Errorf("%s: %v", url, err)
		}
	}
	if asked.Load() != before {
		t.Error("a request was sent")
	}
}

// Removing a plugin stops it and moves its folder to the trash, or deletes it
// if there is none; what it kept in its data folder stays, for if it is
// installed again.
func TestRemoveStopsThePluginAndKeepsItsData(t *testing.T) {
	dataDir := t.TempDir()
	for name, trash := range map[string]func(string) error{
		"trash":    nil, // set below
		"no trash": func(string) error { return errors.New("no trash here") },
	} {
		dir := filepath.Join(t.TempDir(), "plugins")
		folder := filepath.Join(dir, "tone")
		os.MkdirAll(folder, 0o755)
		os.WriteFile(filepath.Join(folder, "plugin.json"), []byte(`{"id":"tone"}`), 0o644)
		os.WriteFile(filepath.Join(folder, "index.js"), []byte(searchJS), 0o644)
		p, err := Load(folder, dataDir)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.SetSetting("k", "v"); err != nil {
			t.Fatal(err)
		}
		var trashed string
		if trash == nil {
			trash = func(path string) error {
				trashed = path
				return os.RemoveAll(path)
			}
		}
		if err := p.Remove(trash); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(folder); err == nil {
			t.Errorf("%s: the folder is still there", name)
		}
		if name == "trash" && trashed != folder {
			t.Errorf("trashed %q, want %q", trashed, folder)
		}
		if _, err := os.Stat(filepath.Join(dataDir, "tone", "settings.json")); err != nil {
			t.Errorf("%s: its data went: %v", name, err)
		}
	}
}

// A plugin that was updated in place is read again from its folder, the old one
// stopped, and answers as the new code does.
func TestReloadedReadsTheNewVersion(t *testing.T) {
	dir, data := filepath.Join(t.TempDir(), "plugins"), t.TempDir()
	folder := filepath.Join(dir, "tone")
	os.MkdirAll(folder, 0o755)
	os.WriteFile(filepath.Join(folder, "plugin.json"), []byte(`{"id":"tone","version":"1"}`), 0o644)
	os.WriteFile(filepath.Join(folder, "index.js"), []byte(`export function suggest() { return ["old"]; }`), 0o644)
	old, err := Load(folder, data)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := old.Suggest("x"); err != nil || got[0] != "old" {
		t.Fatalf("%v, %v", got, err)
	}
	os.WriteFile(filepath.Join(folder, "plugin.json"), []byte(`{"id":"tone","version":"2"}`), 0o644)
	os.WriteFile(filepath.Join(folder, "index.js"), []byte(`export function suggest() { return ["new"]; }`), 0o644)
	again, err := old.Reloaded(data)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if got, err := again.Suggest("x"); err != nil || got[0] != "new" || again.Info().Version != "2" {
		t.Errorf("%v, %v, %q", got, err, again.Info().Version)
	}
}
