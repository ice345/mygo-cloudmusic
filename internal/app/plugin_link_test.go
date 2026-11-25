package app

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"netease-music-mygo/internal/plugin"
)

// linkEnv is an app whose settings window is a count of its being opened, and
// whose way to GitHub says if it was asked.
func linkEnv(t *testing.T) (*app, *int, *int) {
	t.Helper()
	a := newTestApp()
	a.settings = defaultSettings()
	opened, fetched := new(int), new(int)
	was, wasFetch := showSettings, fetchPlugin
	showSettings = func(*app) { *opened++ }
	fetchPlugin = func(string, string) (*plugin.Pending, error) { *fetched++; return nil, plugin.ErrNoRepo }
	t.Cleanup(func() { showSettings, fetchPlugin = was, wasFetch })
	return a, opened, fetched
}

// A link to install a plugin opens the settings and the dialog that installs from
// GitHub with its repository in the field, in either form of the link, whatever
// the case of its scheme; it fetches nothing, so that a page cannot make the app
// download, and it replaces the question of an earlier plugin.
func TestInstallLinkOpensTheDialogAndFetchesNothing(t *testing.T) {
	for _, raw := range []string{
		linkScheme + "://install-plugin?repo=o/tone",
		linkScheme + ":install-plugin?repo=o/tone",
		strings.ToUpper(linkScheme) + "://install-plugin?repo=o%2Ftone",
		linkScheme + "://install-plugin/?repo=%20o/tone%20",
	} {
		a, opened, fetched := linkEnv(t)
		a.installer.pending = &plugin.Pending{}
		a.openLink(raw)
		in := a.installer
		if !in.open || !in.github || in.repo != "o/tone" || *opened != 1 || in.pending != nil {
			t.Errorf("%s: dialog %+v, settings opened %d times", raw, in, *opened)
		}
		if *fetched != 0 {
			t.Errorf("%s: fetched from GitHub", raw)
		}
	}
}

// Links of another scheme, of another action, or naming no repository, or one
// too long to read in the field, leave the app as it was.
func TestOtherLinksAreLeftAlone(t *testing.T) {
	for _, raw := range []string{
		"https://install-plugin?repo=o/tone",
		"other-scheme://install-plugin?repo=o/tone",
		linkScheme + "://open?repo=o/tone",
		linkScheme + "://install-plugin",
		linkScheme + "://install-plugin?repo=",
		linkScheme + "://install-plugin?repo=" + strings.Repeat("a", maxLinkRepo+1),
		linkScheme + "://%zz",
		"",
	} {
		a, opened, _ := linkEnv(t)
		a.openLink(raw)
		if a.installer.open || *opened != 0 || a.installer.repo != "" {
			t.Errorf("%q: dialog %+v, settings opened %d times", raw, a.installer, *opened)
		}
	}
}

// A link the app was started with arrives before its window is up and waits for
// it, opening once; those that come after open at once.
func TestLinkThatStartedTheAppWaitsForItsWindow(t *testing.T) {
	a, opened, _ := linkEnv(t)
	a.linkOpened(linkScheme + "://install-plugin?repo=o/first")
	if a.installer.open || *opened != 0 {
		t.Fatalf("opened before the window: %+v", a.installer)
	}
	a.linksReady()
	if !a.installer.open || a.installer.repo != "o/first" || *opened != 1 {
		t.Fatalf("not opened once ready: %+v, %d times", a.installer, *opened)
	}
	a.linksReady()
	if *opened != 1 {
		t.Errorf("a link opened twice: %d", *opened)
	}
	a.linkOpened(linkScheme + "://install-plugin?repo=o/second")
	if a.installer.repo != "o/second" || *opened != 2 {
		t.Errorf("a link once ready: %+v, %d times", a.installer, *opened)
	}
}

// The app declares both of its schemes, the installed app's and the development
// app's, which the system hands links by: without them a link opens nothing.
func TestConfigDeclaresTheLinkSchemes(t *testing.T) {
	b, err := os.ReadFile("../../mygo.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		URLSchemes []string `json:"urlSchemes"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"mygo-music", "mygo-music-dev"} {
		found := false
		for _, s := range cfg.URLSchemes {
			found = found || s == want
		}
		if !found {
			t.Errorf("mygo.json does not declare %q: %v", want, cfg.URLSchemes)
		}
	}
}
