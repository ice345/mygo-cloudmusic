// Ported in part from internal/app/plugin_install.go of egoist/mygo-music
// (AGPL-3.0, as this repository is).

package app

import (
	"fmt"
	"log"
	"strings"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/plugin"
)

// pluginsGroup is the group of the settings that holds the plugins' install:
// what plugins are, and how to install one. It shows with none installed, as it
// is where the first is.
func (a *app) pluginsGroup(c *ui.Context, p palette, group func(string, func())) {
	group("插件", func() {
		desc := "插件让你在边栏的音源按钮里切换到别的音乐来源。只安装你信任的插件：它们按你允许的权限访问网站，也可能运行程序。"
		if n := len(a.plugins); n > 0 {
			desc = fmt.Sprintf("已安装 %d 个。只安装你信任的插件：它们按你允许的权限访问网站，也可能运行程序。", n)
		}
		settingRow(c, p, "音源插件", desc, nil, func() { a.installMenu(c, p) })
	})
}

// installMenu is the + of the plugins' group: the ways to install one.
func (a *app) installMenu(c *ui.Context, p palette) {
	in := &a.installer
	add := iconButton(c.Key("install-plugin"), iconPlus, 16, p.muted, "安装插件")
	add.Menu(func(m *ui.Menu) {
		if m.Item("从 GitHub 安装…").Disabled(in.busy).Chosen() {
			a.askForRepo()
		}
		if m.Item("从文件夹安装…").Disabled(in.busy).Chosen() {
			a.installFromDisk(true)
		}
		if m.Item("从 Zip 安装…").Disabled(in.busy).Chosen() {
			a.installFromDisk(false)
		}
		m.Separator()
		if m.Item("浏览插件…").Chosen() {
			if err := openWebPage(pluginsPage); err != nil {
				log.Printf("opening %s: %v", pluginsPage, err)
			}
		}
		if m.Item("显示插件文件夹").Chosen() {
			if code, _, ok := pluginDirs(); ok {
				revealFolder(code)
			}
		}
	})
}

// installDialog asks for a repository to install from, and what's asked about a
// plugin on its way, before it's in. Put away, it forgets the plugin.
func (a *app) installDialog(c *ui.Context, p palette) {
	in := &a.installer
	if !in.open && in.pending != nil {
		a.cancelInstall()
	}
	if !in.open && !in.busy {
		in.err = ""
	}
	ui.DialogBase(c.Key("install-plugin-dialog"), &in.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(400).Padding(24).Radius(18).Gap(10).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.94}, Exit: &ui.Motion{Scale: 0.96}})
		buttons := func(action string, disabled bool, do func()) {
			ui.Row(c).Gap(10).Justify(ui.End).Margin(8, 0, 0, 0).Children(func() {
				if pillButton(c.Key("cancel"), nil, "取消", p.hover.Over(p.bg), p.text).Clicked() {
					in.open = false
				}
				if action == "" {
					return
				}
				ok := pillButton(c.Key("ok"), nil, action, p.accent, p.onAccent)
				if disabled {
					ok.Disabled(true).Opacity(0.5)
				}
				if ok.Clicked() && !disabled {
					do()
				}
			})
		}
		problem := func() {
			if in.err != "" {
				ui.Text(c, in.err).FontSize(12.5).TextColor(p.accent).LineHeight(1.4)
			}
		}
		switch pending := in.pending; {
		case pending != nil:
			ui.Text(c, "安装 "+strings.TrimSpace(pending.Manifest.Name+" "+pending.Version)+"？").FontSize(17).FontWeight(700)
			from := pending.Repo
			if from == "" {
				from = pending.Path
			}
			ui.Text(c, "来自 "+from).FontSize(13).TextColor(p.muted).LineHeight(1.45)
			perms := pending.Manifest.Permissions
			ui.Text(c, permissionText(perms)).FontSize(13).TextColor(p.muted).LineHeight(1.45)
			if len(perms.Process) > 0 {
				ui.Text(c, "它想运行 "+programsText(perms.Process)+"。这些程序能做你能做的一切。装好后，要在设置里单独允许它启动程序，它才能运行。").
					FontSize(13).TextColor(p.muted).LineHeight(1.45)
			}
			if a.updatingPending(pending) {
				ui.Text(c, "这个新版本要求的权限比你允许过的更多。").FontSize(13).FontWeight(600).TextColor(p.accent).LineHeight(1.45)
			}
			problem()
			buttons("安装", in.busy, a.confirmInstall)
		case in.github:
			ui.Text(c, "从 GitHub 安装").FontSize(17).FontWeight(700)
			ui.Text(c, "写下有插件的仓库，如 owner/repo，或它在 GitHub 上的地址").FontSize(13).TextColor(p.muted).LineHeight(1.45)
			field := ui.TextInput(c.Key("repo"), &in.repo).Label("从 GitHub 安装").Placeholder("owner/repo 或网址").FontSize(14).AutoFocus()
			problem()
			action := "安装"
			if in.busy {
				action = "正在下载…"
			}
			empty := strings.TrimSpace(in.repo) == ""
			if field.Submitted() && !in.busy && !empty {
				a.installFromRepo()
			}
			buttons(action, in.busy || empty, a.installFromRepo)
		default:
			// One from this computer on its way in, or what went wrong with it.
			ui.Text(c, "安装插件").FontSize(17).FontWeight(700)
			if in.busy {
				ui.Text(c, "正在打开插件…").FontSize(13).TextColor(p.muted)
			}
			problem()
			buttons("", false, nil)
		}
	})
}

// updatingPending reports whether the plugin waiting is a new version of one
// installed, which asks for more than that was allowed.
func (a *app) updatingPending(pending *plugin.Pending) bool {
	old := a.pluginByID(pending.Manifest.ID)
	return old != nil && asksForMore(old.Permissions(), pending.Manifest.Permissions)
}

// pluginManageRow says where a plugin came from, with what can be done with it:
// updating to a new version, checking for one, showing its folder, removing it.
func (a *app) pluginManageRow(c *ui.Context, p palette, pl *plugin.Plugin, id string) {
	in := &a.installer
	from, o := "来自这台电脑", pl.Origin()
	if o != nil {
		from = "来自 GitHub：" + o.Repo
		if o.Version != "" {
			from += " · " + o.Version
		}
	}
	note := ""
	if msg := in.updateErr[id]; msg != "" {
		note = msg
	} else {
		switch in.checked[id] {
		case checking:
			note = "正在检查更新…"
		case upToDate:
			note = "已是最新版本"
		case checkFailed:
			note = "没能检查更新"
		}
	}
	settingRow(c, p, from, note, nil, func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if version, ok := a.settings.PluginUpdates[id]; ok && o != nil && plugin.IsNewer(version, o.Version) {
				label := "更新到 " + version
				if in.updating[id] {
					label = "正在更新…"
				}
				if ui.Button(c.Key(id+"/update"), label).Disabled(in.updating[id]).Clicked() {
					a.updatePlugin(pl)
				}
			}
			more := iconButton(c.Key(id+"/plugin-more"), iconMore, 16, p.muted, "更多："+id)
			more.Menu(func(m *ui.Menu) {
				if o != nil {
					busy := in.checked[id] == checking || in.updating[id]
					if m.Item("检查更新").Disabled(busy).Chosen() {
						a.checkPluginUpdate(pl)
					}
				}
				if m.Item("在文件夹中显示").Chosen() {
					revealFolder(pl.Dir())
				}
				m.Separator()
				if m.Item("移除…").Disabled(in.updating[id]).Chosen() {
					confirmRemovePlugin(a, pl.Info().Name, func() { a.removePlugin(pl) })
				}
			})
		})
	})
}
