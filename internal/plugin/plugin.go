package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/MoeForge/moejs"

	"netease-music-mygo/internal/music"
)

// Plugin is a music.Source in JavaScript.
type Plugin struct {
	manifest Manifest
	dir      string // where plugin.json is
	data     string // the plugin's own data, and ours about it
	mod      *moejs.Module
	exports  map[string]bool
	locales  map[string]map[string]string // by language, the English to what it is in it

	// starting is held while the plugin starts, and by Close, so one start
	// runs at a time and none outlives a Close. It isn't mu: the module's top
	// level runs as it starts, and the host functions it calls take mu.
	starting sync.Mutex

	mu       sync.Mutex // guards what follows
	settings map[string]string
	storage  map[string]json.RawMessage
	granted  bool // the listener let it start programs
	vm       *vm
	prompt   func(music.SignInPrompt)
}

// newPlugin reads the plugin in dir, keeping its data in dataDir, without
// reading its code: what it keeps from the last time, and what it says of
// itself.
func newPlugin(dir, dataDir string) (*Plugin, error) {
	m, err := readManifest(dir)
	if err != nil {
		return nil, err
	}
	p := &Plugin{
		manifest: m,
		dir:      dir,
		data:     filepath.Join(dataDir, m.ID),
		exports:  map[string]bool{},
		settings: map[string]string{},
		storage:  map[string]json.RawMessage{},
	}
	if p.locales, err = readLocales(dir); err != nil {
		return nil, err
	}
	readJSON(filepath.Join(p.data, "settings.json"), &p.settings)
	readJSON(filepath.Join(p.data, "storage.json"), &p.storage)
	readJSON(filepath.Join(p.data, "granted.json"), &p.granted)
	return p, nil
}

// OnSignInPrompt says what to do with what the plugin wants shown as it signs in.
func (p *Plugin) OnSignInPrompt(show func(music.SignInPrompt)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.prompt = show
}

// t is the text in the listener's language, if the plugin translated it.
func (p *Plugin) t(text string) string {
	if tr := p.locales[Language()][text]; tr != "" {
		return tr
	}
	return text
}

func readJSON(path string, into any) {
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, into)
	}
}

// writeJSON replaces the file whole, so that a crash leaves the last one.
func (p *Plugin) writeJSON(name string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.data, 0o700); err != nil {
		return err
	}
	path := filepath.Join(p.data, name)
	if err := os.WriteFile(path+".part", b, 0o600); err != nil {
		return err
	}
	return os.Rename(path+".part", path)
}

// Permissions are what plugin.json asks for.
func (p *Plugin) Permissions() Permissions { return p.manifest.Permissions }

// Granted reports whether the listener let the plugin start its programs.
func (p *Plugin) Granted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.granted
}

// keepGranted keeps whether the listener let the plugin start programs.
func (p *Plugin) keepGranted(on bool) error {
	p.mu.Lock()
	p.granted = on
	p.mu.Unlock()
	return p.writeJSON("granted.json", on)
}

// Info is what plugin.json says of the plugin, in the listener's language as
// far as the plugin translated it: its name, settings and sidebar.
func (p *Plugin) Info() music.Info {
	info := p.manifest.Info
	if len(p.locales[Language()]) == 0 {
		return info
	}
	info.Name = p.t(info.Name)
	info.Settings = slices.Clone(info.Settings)
	for i, st := range info.Settings {
		info.Settings[i].Label, info.Settings[i].Help = p.t(st.Label), p.t(st.Help)
		info.Settings[i].Options = slices.Clone(st.Options)
		for j, option := range st.Options {
			info.Settings[i].Options[j].Label = p.t(option.Label)
		}
	}
	info.Navigation = slices.Clone(info.Navigation)
	for i, n := range info.Navigation {
		info.Navigation[i].Title = p.t(n.Title)
	}
	return info
}

// Can reports whether the plugin exports the function. It's known from
// compiling it, without running anything, so the views can ask at any time.
func (p *Plugin) Can(what string) bool { return p.exports[what] }

func (p *Plugin) Setting(key string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.settings[key]
}

// keepSetting keeps the setting, and answers whether it is kept for a running
// plugin to be told of.
func (p *Plugin) keepSetting(key, value string) error {
	p.mu.Lock()
	p.settings[key] = value
	settings := make(map[string]string, len(p.settings))
	for k, s := range p.settings {
		settings[k] = s
	}
	p.mu.Unlock()
	return p.writeJSON("settings.json", settings)
}
