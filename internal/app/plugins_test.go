package app

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"netease-music-mygo/internal/netease"
)

// putPlugin writes a plugin of the manifest and module given into a folder of
// its own in code.
func putPlugin(t *testing.T, code, folder, manifest, js string) {
	t.Helper()
	dir := filepath.Join(code, folder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"plugin.json": manifest, "index.js": js} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// The plugins in the plugins folder become sources beside NetEase, which the
// player finds a track's by its ID. One that would stand in for NetEase, or
// for a plugin found before it, or that doesn't load, is left out without
// taking the others with it.
func TestPluginsBecomeSources(t *testing.T) {
	a := newTestApp()
	builtIn := a.source(netease.SourceID)
	code := t.TempDir()
	const js = `export function search({ query }) { return { tracks: [] }; }`
	putPlugin(t, code, "a-tone", `{"id":"tone","name":"Tone"}`, js)
	putPlugin(t, code, "b-fake", `{"id":"netease","name":"Not NetEase"}`, js)
	putPlugin(t, code, "c-twin", `{"id":"tone","name":"Twin"}`, js)
	putPlugin(t, code, "d-broken", `{"id":`, js)
	putPlugin(t, code, "e-other", `{"id":"other"}`, `export function lyrics() { return null; }`)
	t.Cleanup(a.closePlugins)

	a.usePlugins(code, t.TempDir())

	tone := a.source("tone")
	if tone == nil || tone.Info().Name != "Tone" || !tone.Can("search") || tone.Can("lyrics") {
		t.Fatalf("tone: %v", tone)
	}
	if a.pl.Source("tone") != tone {
		t.Error("the player can't find the plugin's source")
	}
	if got := a.source(netease.SourceID); got != builtIn {
		t.Errorf("a plugin stood in for NetEase: %v", got.Info())
	}
	if other := a.source("other"); other == nil || !other.Can("lyrics") {
		t.Errorf("a plugin found after the ones left out: %v", other)
	}
	if len(a.plugins) != 2 || len(a.sources) != 3 {
		t.Errorf("%d plugins, %d sources", len(a.plugins), len(a.sources))
	}
}

// A picture from a plugin's own host is fetched with the headers its manifest
// gives, even when a plugin before it has none; a picture from another host is
// fetched with none, and before any plugin is found there are none to ask.
func TestPluginsGiveTheirPicturesHeaders(t *testing.T) {
	a := newTestApp()
	pic := func(host string) *url.URL { return &url.URL{Scheme: "https", Host: host, Path: "/a.jpg"} }
	if a.headers.image(pic("img.example.com")) != nil {
		t.Error("headers before any plugin")
	}
	code := t.TempDir()
	putPlugin(t, code, "a-quiet", `{"id":"quiet"}`, `export function search() { return { tracks: [] }; }`)
	putPlugin(t, code, "pics", `{"id":"pics","permissions":{"network":["img.example.com"]},
		"requestHeaders":[{"hosts":["img.example.com"],"kinds":["image"],"headers":{"Referer":"https://img.example.com/"}}]}`,
		`export function search() { return { tracks: [] }; }`)
	t.Cleanup(a.closePlugins)

	a.usePlugins(code, t.TempDir())

	if got := a.headers.image(pic("img.example.com")); got["Referer"] != "https://img.example.com/" {
		t.Errorf("the plugin's own host: %v", got)
	}
	if got := a.headers.image(pic("elsewhere.example.com")); got != nil {
		t.Errorf("another host: %v", got)
	}
}
