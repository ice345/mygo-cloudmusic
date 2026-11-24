// Ported from internal/plugin of egoist/mygo-music (AGPL-3.0, as this
// repository is), but for the engine, which is not here yet.

// Package plugin runs sources as JavaScript or TypeScript, each in a runtime
// of its own inside the app. A plugin is a directory with a plugin.json:
//
//	{
//	  "id": "example", "name": "Example", "main": "index.js", "playback": "stream",
//	  "minAppVersion": "0.1.8",
//	  "permissions": {"network": ["api.example.com"], "process": ["bin/helper-*", "ffmpeg"]}
//	}
//
// and one module that exports a function for each thing it does, by the names
// of music.Source's, such as search or playlist. Each gets one object of
// parameters, and answers a value or a promise of one. What it doesn't export,
// it doesn't do. A plugin that throws an error with a code of signIn, setup,
// unavailable or unsupported says that of what was asked.
//
// A plugin reaches nothing but the JavaScript standard library, timers and the
// host global, unless it asks: fetch reaches the hosts its network permission
// names, and host.spawn, for those with the process permission that the
// listener granted, starts the programs it names, which can do anything the
// listener can.
package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"netease-music-mygo/internal/music"
)

// Permissions are what a plugin asks for beyond its own sandbox.
type Permissions struct {
	// Network lists the hosts fetch can reach, *.example.com for those below
	// it, and * for any.
	Network []string `json:"network,omitempty"`
	// Process lists the programs it may start, once the listener granted it.
	Process Programs `json:"process,omitempty"`
}

// Programs are what a plugin may start: each a path from its directory or
// absolute, such as bin/helper-*, or a name looked up on PATH, such as ffmpeg,
// and * for any. A pattern matches as filepath.Match does, so * doesn't cross
// a directory. true is short for any, from before programs were named.
type Programs []string

func (ps *Programs) UnmarshalJSON(b []byte) error {
	var all bool
	if err := json.Unmarshal(b, &all); err == nil {
		*ps = nil
		if all {
			*ps = Programs{"*"}
		}
		return nil
	}
	var names []string
	if err := json.Unmarshal(b, &names); err != nil {
		return errors.New("process is a list of programs, or true for any")
	}
	*ps = names
	return nil
}

// Any reports whether it may start any program.
func (ps Programs) Any() bool { return slices.Contains(ps, "*") }

// Allows reports whether command, as host.spawn has it after resolving a path
// from dir, is one of the programs. A path only matches a path, and a name a
// name, so a program that's somewhere else, but called the same, isn't one.
func (ps Programs) Allows(dir, command string) bool {
	bare := !strings.ContainsAny(command, `/\`)
	if !bare {
		command = filepath.Clean(command)
	}
	for _, pattern := range ps {
		if pattern == "*" {
			return true
		}
		pattern = filepath.FromSlash(pattern)
		if strings.ContainsAny(pattern, `/\`) != !bare {
			continue
		}
		if !bare && !filepath.IsAbs(pattern) {
			pattern = filepath.Join(dir, pattern)
		}
		name := command
		if runtime.GOOS == "windows" {
			// Names and paths there are any case, and a program needn't say .exe.
			pattern, name = strings.ToLower(pattern), strings.ToLower(name)
			if bare && filepath.Ext(pattern) == "" {
				name = strings.TrimSuffix(name, ".exe")
			}
		}
		if ok, _ := filepath.Match(pattern, name); ok {
			return true
		}
	}
	return false
}

// Manifest is a plugin's plugin.json.
type Manifest struct {
	music.Info
	Main string `json:"main,omitempty"`
	// MinAppVersion is the oldest version of the app the plugin works with,
	// such as 0.1.8. An older one won't install it, or load it.
	MinAppVersion  string           `json:"minAppVersion,omitempty"`
	Permissions    Permissions      `json:"permissions"`
	RequestHeaders []RequestHeaders `json:"requestHeaders,omitempty"`
}

// appVersion is the app's version, which the app sets. With none, as when
// it's built for development, any plugin loads.
var appVersion string

// SetAppVersion says which version of the app this is.
func SetAppVersion(v string) { appVersion = v }

// NeedsNewerApp is why a plugin doesn't load: it works with a newer version of
// the app than this.
type NeedsNewerApp struct {
	Plugin  string // its name
	Version string // the oldest version it works with
}

func (e *NeedsNewerApp) Error() string {
	return fmt.Sprintf("%s needs MyGo 云音乐 %s or later", e.Plugin, e.Version)
}

var pluginName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// readManifest reads and checks the plugin.json in dir, and fills in what it
// leaves out: the name is the ID, the playback a stream, the main file
// index.js.
func readManifest(dir string) (Manifest, error) {
	var m Manifest
	b, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("%s: %w", dir, err)
	}
	if m.ID == "" {
		return m, fmt.Errorf("%s: plugin.json needs an id", dir)
	}
	// The id names the plugin's data folder, so it may not lead out of it
	// into another plugin's, as ".." would.
	if !pluginName.MatchString(m.ID) {
		return m, fmt.Errorf("%s: a plugin can't be called %q", dir, m.ID)
	}
	if m.Name == "" {
		m.Name = m.ID
	}
	if m.Playback == "" {
		m.Playback = music.PlaybackStream
	}
	if m.Main == "" {
		m.Main = "index.js"
	}
	if m.MinAppVersion != "" {
		if _, ok := versionNumbers(m.MinAppVersion); !ok {
			return m, fmt.Errorf("%s: minAppVersion isn't a version such as 0.1.8: %q", dir, m.MinAppVersion)
		}
		if _, ok := versionNumbers(appVersion); ok && IsNewer(m.MinAppVersion, appVersion) {
			return m, &NeedsNewerApp{Plugin: m.Name, Version: m.MinAppVersion}
		}
	}
	if err := checkRequestHeaders(&m); err != nil {
		return m, fmt.Errorf("%s: %w", dir, err)
	}
	if !filepath.IsLocal(m.Main) {
		return m, fmt.Errorf("%s: main must be a file in the plugin's folder: %q", dir, m.Main)
	}
	return m, nil
}

// IsNewer reports whether latest is newer than installed. Tags that are
// version numbers compare as such, for a release taken back not to read as new.
// Anything else has no order: a commit is just the latest, so one that's
// different is new.
func IsNewer(latest, installed string) bool {
	if latest == installed {
		return false
	}
	a, okA := versionNumbers(latest)
	b, okB := versionNumbers(installed)
	if !okA || !okB {
		return true
	}
	for i := range max(len(a), len(b)) {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// versionNumbers reads a tag such as v1.2.3, or 1.2. A tag with a label, such as
// 1.2.3-beta, isn't one: its order isn't only in its numbers.
func versionNumbers(tag string) ([]int, bool) {
	parts := strings.Split(strings.TrimPrefix(tag, "v"), ".")
	numbers := make([]int, len(parts))
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nil, false
		}
		numbers[i] = n
	}
	return numbers, true
}
