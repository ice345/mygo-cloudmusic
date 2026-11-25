// Ported in part from internal/app/plugin_install.go of egoist/mygo-music
// (AGPL-3.0, as this repository is).

package app

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"slices"
	"time"

	"github.com/egoist/mygo"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/plugin"
)

// Plugins install from GitHub repositories, and update from where they came,
// or from a folder or a zip on this machine; a link on the web installs one too
// (see openLink). Nothing installs without the listener allowing it: what a
// plugin may reach, and the programs it asks to run, are said first.

// These reach GitHub, the disk and the system; tests replace them.
var (
	fetchPlugin  = plugin.Fetch
	fetchRelease = plugin.FetchRelease
	latestPlugin = plugin.Latest
	openPlugin   = plugin.Open
	trashItem    = mygo.Shell.TrashItem
	openWebPage  = mygo.Shell.OpenExternal
	revealFolder = func(path string) {
		if err := mygo.Shell.OpenPath(path); err != nil {
			log.Printf("showing %s: %v", path, err)
		}
	}
)

// pluginsPage lists plugins to install: repositories with the topic
// mygo-music-plugin, as upstream's page does.
const pluginsPage = "https://mygo-music.egoist.dev/plugins"

// pickPlugin asks the listener for a plugin's folder, or a zip of one, with a
// native dialog, on a goroutine, for it blocks; tests replace it.
var pickPlugin = func(a *app, folder bool, got func(string)) {
	go func() {
		opts := mygo.OpenDialogOptions{Title: "安装插件", ButtonLabel: "安装", Directory: folder}
		if !folder {
			opts.Filters = []mygo.FileFilter{{Name: "Zip 压缩包", Extensions: []string{"zip"}}}
		}
		paths, err := mygo.Dialog.Open(opts)
		if err == nil && len(paths) > 0 {
			a.post(func() { got(paths[0]) })
		}
	}()
}

// confirmRemovePlugin asks the listener with a native dialog whether to remove a
// plugin, on a goroutine, for it blocks, calling yes if they say so; tests
// replace it.
var confirmRemovePlugin = func(a *app, name string, yes func()) {
	parent := a.settingsWin
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:             parent,
			Type:               mygo.MessageWarning,
			Message:            "移除 " + name + "？",
			Detail:             "它的文件夹会移到废纸篓。它保存的设置会留着，以备你再装上它。",
			Buttons:            []string{"取消", "移除"},
			DefaultButton:      1,
			DestructiveButtons: []int{1},
		})
		if err == nil && res.Button == 1 {
			a.post(yes)
		}
	}()
}

// pluginCheckEvery is how often plugins are checked for new versions: once a
// day, as the app is. GitHub's API allows few requests without an account.
const pluginCheckEvery = 24 * time.Hour

// pluginInstaller is what's going on with installing plugins.
type pluginInstaller struct {
	// open is whether the dialog shows: asking for a repository, or about a
	// plugin on its way, or what went wrong with one.
	open, github bool
	repo         string          // what's written in the field
	busy         bool            // one's downloading
	err          string          // what went wrong last
	pending      *plugin.Pending // one downloaded, for the listener to allow
	updating     map[string]bool
	updateErr    map[string]string // what went wrong updating a plugin, by ID
	// checked is what checking a plugin for a new version came to, when the
	// listener asked, by ID.
	checked map[string]checkResult
	// found is the new version checking found of a plugin, by ID, for updating
	// to install without asking GitHub again; after the app opens again, it asks.
	found map[string]*plugin.Release
}

// checkResult is what checking a plugin for a new version came to.
type checkResult int

const (
	checking checkResult = iota + 1
	upToDate
	checkFailed
)

// askForRepo opens the dialog that asks for a repository to install from.
func (a *app) askForRepo() {
	in := &a.installer
	a.cancelInstall()
	in.open, in.github, in.err = true, true, ""
}

// installFromRepo downloads what's written in the field, to ask about it.
func (a *app) installFromRepo() {
	in := &a.installer
	repo, err := plugin.ParseRepo(in.repo)
	if err != nil {
		in.err = "请写成 owner/repo，或它在 GitHub 上的地址"
		return
	}
	code, _, ok := pluginDirs()
	if !ok {
		in.err = "找不到存放插件的文件夹"
		return
	}
	in.busy, in.err = true, ""
	a.cancelInstall()
	go func() {
		pending, err := fetchPlugin(repo, code)
		a.post(func() {
			in.busy = false
			if err != nil {
				in.err = installError(err)
				return
			}
			in.pending = pending
		})
	}()
}

// installFromDisk asks for a plugin's folder, or a zip of one, and copies it in
// to ask about it, as one from GitHub is asked about.
func (a *app) installFromDisk(folder bool) {
	in := &a.installer
	pickPlugin(a, folder, func(path string) {
		code, _, ok := pluginDirs()
		if !ok {
			return
		}
		a.cancelInstall()
		in.open, in.github, in.busy, in.err = true, false, true, ""
		go func() {
			pending, err := openPlugin(path, code)
			a.post(func() {
				in.busy = false
				if err != nil {
					in.err = installError(err)
					return
				}
				in.pending = pending
			})
		}()
	})
}

// installError says what went wrong, in words for the listener.
func installError(err error) string {
	if errors.Is(err, plugin.ErrNoRepo) {
		return "GitHub 上没有这个公开仓库"
	}
	var needs *plugin.NeedsNewerApp
	if errors.As(err, &needs) {
		return fmt.Sprintf("%s 需要 MyGo 云音乐 %s 或更新的版本，请先更新应用。", needs.Plugin, needs.Version)
	}
	return err.Error()
}

// confirmInstall installs the plugin the listener allowed.
func (a *app) confirmInstall() {
	in := &a.installer
	pending := in.pending
	if pending == nil {
		return
	}
	code, _, ok := pluginDirs()
	if !ok {
		return
	}
	in.pending, in.busy = nil, true
	go func() {
		err := pending.Install(code)
		a.post(func() {
			in.busy = false
			if err != nil {
				in.err = installError(err)
				return
			}
			in.repo = ""
			if err := a.pluginInstalled(pending.Manifest.ID); err != nil {
				in.err = err.Error()
				return
			}
			in.open = false
		})
	}()
}

// cancelInstall forgets the plugin the listener didn't want.
func (a *app) cancelInstall() {
	in := &a.installer
	if in.pending != nil {
		go in.pending.Discard()
		in.pending = nil
	}
}

// pluginInstalled takes in what was just installed: a new plugin, or a new
// version of one, which takes its place without opening the app again.
func (a *app) pluginInstalled(id string) error {
	code, data, ok := pluginDirs()
	if !ok {
		return errors.New("找不到存放插件的文件夹")
	}
	in := &a.installer
	delete(a.settings.PluginUpdates, id)
	a.settings.save()
	delete(in.updating, id)
	delete(in.found, id)
	delete(in.checked, id)
	p, err := plugin.Load(filepath.Join(code, id), data)
	if err != nil {
		return err
	}
	old := a.pluginByID(id)
	if old == nil {
		if !a.addSource(p) {
			p.Close()
			return fmt.Errorf("已经有一个叫 %s 的音源了", id)
		}
		a.plugins = append(a.plugins, p)
		a.syncHeaders()
		a.redraw()
		return nil
	}
	// The listener let the old one run the programs it asked for; a new
	// version that asks for others is not let, until they say so again.
	if p.Granted() && more(old.Permissions().Process, p.Permissions().Process) {
		if err := p.Grant(false); err != nil {
			log.Printf("taking back what %s was allowed: %v", id, err)
		}
	}
	// The new version in the old one's place. Its code is gone from under the
	// old one: it stops, and what the app held of it, which the new may say
	// otherwise of, is let go.
	a.replaceSource(p)
	at := slices.Index(a.plugins, old)
	a.plugins[at] = p
	a.syncHeaders()
	if a.login.source == id {
		a.giveUpLogin()
		a.login.open = false
	}
	a.forgetSource(id)
	go old.Close()
	if a.activeID() == id {
		a.enterPlugin(p)
		a.load(a.route)
	}
	a.redraw()
	return nil
}

// removePlugin takes a plugin out, what it keeps of its own staying for if it
// comes back. If it is the source shown, NetEase's is shown instead.
func (a *app) removePlugin(p *plugin.Plugin) {
	id := p.Info().ID
	if a.login.source == id {
		a.giveUpLogin()
		a.login.open = false
	}
	if a.activeID() == id {
		a.switchSource(netease.SourceID)
	}
	a.removeSource(id)
	a.plugins = slices.DeleteFunc(slices.Clone(a.plugins), func(q *plugin.Plugin) bool { return q == p })
	a.syncHeaders()
	a.forgetSource(id)
	delete(a.settings.PluginUpdates, id)
	delete(a.installer.found, id)
	delete(a.installer.checked, id)
	a.settings.save()
	go func() {
		if err := p.Remove(trashItem); err != nil {
			log.Printf("removing plugin %s: %v", id, err)
		}
	}()
	a.redraw()
}

// windowUp does what waits for the window: the links that came before it, and
// the plugins' check for new versions, which then goes on as the days come.
func (a *app) windowUp() {
	a.linksReady()
	a.keepCheckingPlugins()
}

// keepCheckingPlugins checks for new versions now if it's time, and from then
// on as the day comes round, for as long as the app runs.
func (a *app) keepCheckingPlugins() {
	a.checkPluginUpdates()
	time.AfterFunc(pluginCheckEvery, func() { a.post(a.keepCheckingPlugins) })
}

// checkPluginUpdates asks where the plugins came from what versions are there
// now, unless it's been done in the last day, what it found then being kept.
// What can't be reached just shows nothing new.
func (a *app) checkPluginUpdates() {
	if !a.settings.PluginsChecked.IsZero() && time.Since(a.settings.PluginsChecked) < pluginCheckEvery {
		return
	}
	origins := map[string]plugin.Origin{}
	for _, p := range a.plugins {
		if o := p.Origin(); o != nil {
			origins[p.Info().ID] = *o
		}
	}
	if len(origins) == 0 {
		return
	}
	a.settings.PluginsChecked = time.Now()
	a.settings.save()
	go func() {
		found, releases := map[string]string{}, map[string]*plugin.Release{}
		for id, o := range origins {
			rel, err := latestPlugin(o.Repo)
			if err != nil {
				log.Printf("checking plugin %s: %v", id, err)
				continue
			}
			if plugin.IsNewer(rel.Version, o.Version) {
				found[id], releases[id] = rel.Version, rel
			}
		}
		a.post(func() {
			a.settings.PluginUpdates = found
			a.installer.found = releases
			a.settings.save()
		})
	}()
}

// checkPluginUpdate checks one plugin for a new version now, as the listener
// asks from its menu, whenever the daily check was. A new one shows its Update
// button; none, or none to be had, is said in its row.
func (a *app) checkPluginUpdate(p *plugin.Plugin) {
	if p.Origin() == nil {
		return
	}
	in := &a.installer
	if in.checked == nil {
		in.checked = map[string]checkResult{}
	}
	id, o := p.Info().ID, *p.Origin()
	if in.checked[id] == checking {
		return
	}
	in.checked[id] = checking
	delete(in.updateErr, id)
	go func() {
		rel, err := latestPlugin(o.Repo)
		a.post(func() {
			switch {
			case err != nil:
				log.Printf("checking plugin %s: %v", id, err)
				in.checked[id] = checkFailed
			case plugin.IsNewer(rel.Version, o.Version):
				delete(in.checked, id)
				if a.settings.PluginUpdates == nil {
					a.settings.PluginUpdates = map[string]string{}
				}
				a.settings.PluginUpdates[id] = rel.Version
				if in.found == nil {
					in.found = map[string]*plugin.Release{}
				}
				in.found[id] = rel
				a.settings.save()
			default:
				in.checked[id] = upToDate
				delete(a.settings.PluginUpdates, id)
				delete(in.found, id)
				a.settings.save()
			}
		})
	}()
}

// updatePlugin installs the new version of a plugin. One that asks for more than
// the listener allowed asks them again, first.
func (a *app) updatePlugin(p *plugin.Plugin) {
	o := p.Origin()
	code, _, ok := pluginDirs()
	if o == nil || !ok {
		return
	}
	in := &a.installer
	if in.updating == nil {
		in.updating = map[string]bool{}
	}
	if in.updateErr == nil {
		in.updateErr = map[string]string{}
	}
	id := p.Info().ID
	in.updating[id] = true
	delete(in.updateErr, id)
	had := p.Permissions()
	// The version the check found, which the Update button names, rather than
	// asking GitHub again for what's there.
	rel := in.found[id]
	if rel != nil && rel.Version != a.settings.PluginUpdates[id] {
		rel = nil
	}
	go func() {
		var pending *plugin.Pending
		var err error
		if rel != nil {
			pending, err = fetchRelease(rel, code)
		} else {
			pending, err = fetchPlugin(o.Repo, code)
		}
		if err == nil && asksForMore(had, pending.Manifest.Permissions) {
			a.post(func() {
				delete(in.updating, id)
				a.cancelInstall()
				in.open, in.github, in.err, in.pending = true, false, "", pending
			})
			return
		}
		if err == nil {
			err = pending.Install(code)
		}
		a.post(func() {
			delete(in.updating, id)
			if err == nil {
				err = a.pluginInstalled(id)
			}
			if err != nil {
				in.updateErr[id] = installError(err)
			}
		})
	}()
}

// asksForMore reports whether a new version asks for anything the old didn't.
func asksForMore(had, wants plugin.Permissions) bool {
	return more(had.Process, wants.Process) || more(had.Network, wants.Network)
}

// more reports whether wants has something had doesn't, which * has all of.
func more(had, wants []string) bool {
	if slices.Contains(had, "*") {
		return false
	}
	for _, w := range wants {
		if !slices.Contains(had, w) {
			return true
		}
	}
	return false
}
