package plugin

import (
	"crypto/aes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"netease-music-mygo/internal/music"
)

// inline loads a plugin of the manifest and the module a test gives, with its
// data in a folder of the test's.
func inline(t *testing.T, manifest, js string) *Plugin {
	t.Helper()
	dir := writePlugin(t, manifest, map[string]string{"index.js": js})
	p, err := Load(dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

// A plugin that throws says what it is by a code, which the app acts on by its
// kind, with the plugin's own words as the reason; any other error shows its
// message, and an async export answers as a sync one does.
func TestPluginErrorsKeepTheirKind(t *testing.T) {
	p := inline(t, `{"id":"e"}`, `
const fail = (code, message) => Object.assign(new Error(message), { code });
export function suggest({ query }) {
  switch (query) {
    case "signIn": throw fail("signIn", "log in at the site");
    case "setup": throw fail("setup", "fill in the server");
    case "unavailable": throw fail("unavailable", "removed");
    case "unsupported": throw fail("unsupported");
    case "plain": throw new Error("it broke");
    case "string": throw "just text";
    case "silent": throw new Error("");
  }
  return [query];
}
export async function recent() {
  await new Promise((resolve) => setTimeout(resolve, 20));
  throw fail("signIn", "later");
}
export async function search({ query }) {
  await new Promise((resolve) => setTimeout(resolve, 20));
  return { tracks: [{ id: query, name: "late" }] };
}`)
	for query, c := range map[string]struct {
		kind   error
		reason string
	}{
		"signIn":      {music.ErrSignIn, "log in at the site"},
		"setup":       {music.ErrSetup, "fill in the server"},
		"unavailable": {music.ErrUnavailable, "removed"},
		"unsupported": {music.ErrUnsupported, ""},
	} {
		_, err := p.Suggest(query)
		if !errors.Is(err, c.kind) || music.Reason(err) != c.reason {
			t.Errorf("%s: %v (%q)", query, err, music.Reason(err))
		}
	}
	for query, want := range map[string]string{"plain": "it broke", "string": "just text", "silent": "the plugin failed"} {
		_, err := p.Suggest(query)
		if err == nil || err.Error() != want || errors.Is(err, music.ErrSignIn) {
			t.Errorf("%s: %v, want %q", query, err, want)
		}
	}
	if _, err := p.Recent(); !errors.Is(err, music.ErrSignIn) || music.Reason(err) != "later" {
		t.Errorf("an async export's failure: %v", err)
	}
	if r, err := p.Search("x1", music.SearchTracks, 0); err != nil || len(r.Tracks) != 1 || r.Tracks[0].ID != "x1" || r.Tracks[0].Source != "e" {
		t.Errorf("an async export's answer: %+v, %v", r, err)
	}
	if _, err := p.Playlists(); !errors.Is(err, music.ErrUnsupported) {
		t.Errorf("what it does not export: %v", err)
	}
}

// fetch reaches the hosts the network permission names, and no others, a
// redirect to one it does not too; with no permission it reaches none.
func TestPluginReachesOnlyTheHostsItWasGiven(t *testing.T) {
	var secret atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "hello") })
	mux.HandleFunc("/secret", func(w http.ResponseWriter, r *http.Request) { secret.Add(1); fmt.Fprint(w, "secret") })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	mux.HandleFunc("/jump", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://localhost:"+port+"/secret", http.StatusFound)
	})
	module := `export async function suggest({ query }) {
  try { return ["ok:" + await (await fetch(query)).text()]; } catch (e) { return ["failed:" + e.message]; }
}`
	allowed := inline(t, `{"id":"net","permissions":{"network":["127.0.0.1"]}}`, module)
	if got, err := allowed.Suggest(srv.URL + "/hello"); err != nil || len(got) != 1 || got[0] != "ok:hello" {
		t.Errorf("a host it was given: %v, %v", got, err)
	}
	for name, url := range map[string]string{
		"a host it was not given":            "http://localhost:" + port + "/secret",
		"a redirect to one it was not given": srv.URL + "/jump",
	} {
		got, err := allowed.Suggest(url)
		if err != nil || len(got) != 1 || !strings.HasPrefix(got[0], "failed:") {
			t.Errorf("%s: %v, %v", name, got, err)
		}
	}
	if secret.Load() != 0 {
		t.Errorf("a host the plugin was not given was asked %d times", secret.Load())
	}
	none := inline(t, `{"id":"none"}`, module)
	if got, _ := none.Suggest(srv.URL + "/hello"); len(got) != 1 || !strings.HasPrefix(got[0], "failed:") {
		t.Errorf("with no network permission: %v", got)
	}
}

// A plugin that never gives control back is ended, and starts over the next
// time it is asked, where one that only waits is let wait.
func TestPluginThatNeverReturnsIsRestarted(t *testing.T) {
	p := inline(t, `{"id":"stuck"}`, `
let calls = 0;
export function suggest({ query }) {
  calls++;
  if (query === "spin") { for (;;) {} }
  return [query + ":" + calls];
}
export function search() { return new Promise((resolve) => setTimeout(() => resolve({}), 300)); }`)
	if got, err := p.Suggest("a"); err != nil || got[0] != "a:1" {
		t.Fatalf("%v, %v", got, err)
	}
	err := p.callWithin(200*time.Millisecond, "suggest", map[string]string{"query": "spin"}, nil)
	if err == nil || !strings.Contains(err.Error(), "didn't answer suggest in time") {
		t.Fatalf("a plugin that spins: %v", err)
	}
	if got, err := p.Suggest("b"); err != nil || got[0] != "b:1" {
		t.Errorf("after it spun, the plugin starts over: %v, %v", got, err)
	}
	// Slow, not stuck: it goes on, and is not restarted.
	if err := p.callWithin(100*time.Millisecond, "search", map[string]any{"query": "q"}, nil); err == nil {
		t.Error("a slow answer arrived in time")
	}
	if got, _ := p.Suggest("c"); got[0] != "c:2" {
		t.Errorf("a plugin that only waited was restarted: %v", got)
	}
}

// A setting is kept for the next time, read by a running plugin through
// host.setting, and told to it if it exports setSetting.
func TestSettingsReachARunningPlugin(t *testing.T) {
	p := inline(t, `{"id":"s","settings":[{"key":"server","label":"Server"}]}`, `
let told = [];
export function setSetting({ key, value }) { told.push(key + "=" + value); }
export function suggest() { return [host.setting("server"), ...told]; }`)
	if got, _ := p.Suggest(""); len(got) != 1 || got[0] != "" {
		t.Fatalf("before: %v", got)
	}
	if err := p.SetSetting("server", "https://nas.local"); err != nil {
		t.Fatal(err)
	}
	if got, _ := p.Suggest(""); len(got) != 2 || got[0] != "https://nas.local" || got[1] != "server=https://nas.local" {
		t.Errorf("a running plugin: %v", got)
	}
	if p.Setting("server") != "https://nas.local" {
		t.Errorf("Setting says %q", p.Setting("server"))
	}
}

// A plugin starts a program only if its process permission names it and the
// listener let it; taking the permission back ends what it started.
func TestPluginStartsOnlyWhatItMayAndWasGranted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the programs here are of a shell")
	}
	module := `export async function suggest({ query }) {
  try {
    const pr = host.spawn(query, ["-c", "echo hello; read x; echo got-$x"]);
    const first = await pr.readLine();
    pr.write("42\n");
    const second = await pr.readLine();
    const code = await pr.wait();
    return [first, second, String(code)];
  } catch (e) { return ["failed:" + e.message]; }
}`
	none := inline(t, `{"id":"p0"}`, module)
	if got, _ := none.Suggest("sh"); len(got) != 1 || !strings.Contains(got[0], "needs the process permission") {
		t.Errorf("with no process permission: %v", got)
	}
	p := inline(t, `{"id":"p1","permissions":{"process":["sh"]}}`, module)
	if got, _ := p.Suggest("sh"); len(got) != 1 || !strings.Contains(got[0], "hasn't allowed") {
		t.Errorf("not granted: %v", got)
	}
	if err := p.Grant(true); err != nil || !p.Granted() {
		t.Fatalf("grant: %v", err)
	}
	if got, err := p.Suggest("sh"); err != nil || len(got) != 3 || got[0] != "hello" || got[1] != "got-42" || got[2] != "0" {
		t.Errorf("granted: %v, %v", got, err)
	}
	if got, _ := p.Suggest("bash"); len(got) != 1 || !strings.Contains(got[0], "isn't one of the programs") {
		t.Errorf("a program not named: %v", got)
	}
	if err := p.Grant(false); err != nil || p.Granted() {
		t.Fatalf("take back: %v", err)
	}
	if got, _ := p.Suggest("sh"); len(got) != 1 || !strings.Contains(got[0], "hasn't allowed") {
		t.Errorf("taken back: %v", got)
	}
}

// A browser sign-in comes back to a port on this machine only, with what it
// was called with, and the plugin can show the listener a code on the way.
func TestPluginSignsInThroughARedirect(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	p := inline(t, `{"id":"r","permissions":{"network":["accounts.example.com"]}}`, fmt.Sprintf(`
export async function signIn() {
  host.signInPrompt({ qr: "https://accounts.example.com/go", text: "Open it on your phone" });
  const q = await host.waitForRedirect({ port: %d, path: "/callback", timeout: 5000 });
  return { id: q.code, name: "Signed in" };
}`, port))
	prompts := make(chan music.SignInPrompt, 1)
	p.OnSignInPrompt(func(sp music.SignInPrompt) { prompts <- sp })
	done := make(chan *music.Account, 1)
	go func() {
		acc, err := p.SignIn()
		if err != nil {
			t.Error(err)
		}
		done <- acc
	}()
	select {
	case sp := <-prompts:
		if sp.QR != "https://accounts.example.com/go" || sp.Text != "Open it on your phone" {
			t.Errorf("prompt %+v", sp)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no prompt was shown")
	}
	var res *http.Response
	for range 50 {
		if res, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/callback?code=abc123", port)); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	select {
	case acc := <-done:
		if acc == nil || acc.ID != "abc123" {
			t.Errorf("signed in as %+v", acc)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the sign-in did not come back")
	}
}

// A web sign-in is only of a secure page, for cookies of a host the plugin may
// reach, and names the cookies that say it is done; the app's window answers
// with a Cookie header.
func TestPluginWebSignInIsChecked(t *testing.T) {
	var asked WebSignIn
	SetWebSignIn(func(w WebSignIn) (string, error) { asked = w; return "sid=1; uid=2", nil })
	defer SetWebSignIn(func(WebSignIn) (string, error) { return "", errors.New("can't show a sign-in page here") })
	p := inline(t, `{"id":"w","name":"Web","permissions":{"network":["*.example.com"]}}`, `
export async function suggest({ query }) {
  const [url, cookiesOf, until] = query.split("|");
  try {
    const r = await host.webSignIn({ url, cookiesOf, until: until ? until.split(",") : [] });
    return [r.cookie];
  } catch (e) { return ["failed:" + e.message]; }
}`)
	for name, query := range map[string]string{
		"not secure":         "http://login.example.com/|https://api.example.com|sid",
		"cookies elsewhere":  "https://login.example.com/|https://evil.test|sid",
		"cookies not named":  "https://login.example.com/|https://api.example.com|",
		"cookies insecurely": "https://login.example.com/|http://api.example.com|sid",
	} {
		if got, _ := p.Suggest(query); len(got) != 1 || !strings.HasPrefix(got[0], "failed:") {
			t.Errorf("%s: %v", name, got)
		}
	}
	if asked.URL != "" {
		t.Errorf("a window was shown for a sign-in that was refused: %+v", asked)
	}
	got, err := p.Suggest("https://login.example.com/|https://api.example.com|sid,uid")
	if err != nil || len(got) != 1 || got[0] != "sid=1; uid=2" || asked.Plugin != "w" || asked.Name != "Web" || len(asked.Until) != 2 {
		t.Errorf("%v, %v, asked %+v", got, err, asked)
	}
}

// The host's small tools: a plugin may remove only folders of its own data,
// open only web pages, and has a md5 and an AES that agree with Go's.
func TestHostTools(t *testing.T) {
	var opened []string
	SetOpenURL(func(u string) error { opened = append(opened, u); return nil })
	defer SetOpenURL(func(string) error { return errors.New("can't open pages here") })
	p := inline(t, `{"id":"tools"}`, `
export function suggest({ query }) {
  const [what, arg] = query.split("|");
  try {
    switch (what) {
      case "md5": return [host.crypto.md5(arg)];
      case "aes": return [host.crypto.aesEcbEncrypt("0123456789abcdef", arg)];
      case "open": host.openURL(arg); return ["opened"];
      case "remove": host.removeDataDir(arg); return ["removed"];
      case "dir": return [host.dataDir];
    }
  } catch (e) { return ["failed:" + e.message]; }
}`)
	ask := func(q string) string {
		got, err := p.Suggest(q)
		if err != nil || len(got) != 1 {
			t.Fatalf("%s: %v, %v", q, got, err)
		}
		return got[0]
	}
	sum := md5.Sum([]byte("abc"))
	if got := ask("md5|abc"); got != hex.EncodeToString(sum[:]) {
		t.Errorf("md5 %s", got)
	}
	want, _ := aesECB([]byte("0123456789abcdef"), []byte("hello"))
	block, _ := aes.NewCipher([]byte("0123456789abcdef"))
	padded := append([]byte("hello"), make([]byte, 11)...)
	for i := 5; i < 16; i++ {
		padded[i] = 11
	}
	direct := make([]byte, 16)
	block.Encrypt(direct, padded)
	if got := ask("aes|hello"); got != hex.EncodeToString(want) || got != hex.EncodeToString(direct) {
		t.Errorf("aes %s, want %x", got, direct)
	}
	if ask("open|https://example.com/x") != "opened" || len(opened) != 1 {
		t.Errorf("opened %v", opened)
	}
	for _, bad := range []string{"file:///etc/passwd", "javascript:alert(1)", "mailto:a@b.c"} {
		if got := ask("open|" + bad); !strings.HasPrefix(got, "failed:") {
			t.Errorf("opened %s: %s", bad, got)
		}
	}
	if len(opened) != 1 {
		t.Errorf("opened %v", opened)
	}
	dir := ask("dir|")
	os.MkdirAll(filepath.Join(dir, "cache"), 0o700)
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}"), 0o600)
	if got := ask("remove|cache"); got != "removed" {
		t.Errorf("removing its own cache: %s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "cache")); !os.IsNotExist(err) {
		t.Errorf("the folder is still there: %v", err)
	}
	for _, bad := range []string{"..", "../other", ".", "/etc", ""} {
		if got := ask("remove|" + bad); !strings.HasPrefix(got, "failed:") {
			t.Errorf("removed %q: %s", bad, got)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); err != nil {
		t.Errorf("a plugin removed what is ours: %v", err)
	}
}

// Discovery loads every folder that holds a plugin, leaves out one that does
// not load, a folder of a dot, and what is no folder, and takes none of the
// others with it.
func TestDiscoverLeavesOutWhatDoesNotLoad(t *testing.T) {
	root := t.TempDir()
	add := func(name, manifest, js string) {
		dir := filepath.Join(root, name)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644)
		if js != "" {
			os.WriteFile(filepath.Join(dir, "index.js"), []byte(js), 0o644)
		}
	}
	add("good", `{"id":"good"}`, `export function suggest() { return []; }`)
	add("broken", `{"id":"broken"}`, `export function (`)
	add("nomanifest", `not json`, ``)
	add(".partial", `{"id":"partial"}`, `export function suggest() {}`)
	os.WriteFile(filepath.Join(root, "readme.txt"), []byte("hi"), 0o644)
	found := Discover(root, t.TempDir())
	if len(found) != 1 || found[0].Info().ID != "good" {
		t.Fatalf("found %d: %+v", len(found), found)
	}
	if !found[0].Can("suggest") || found[0].Can("search") {
		t.Error("what it can is not what it exports")
	}
}
