// Ported in part from internal/app/ui_source_settings.go of egoist/mygo-music
// (AGPL-3.0, as this repository is).

package app

import (
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/plugin"
)

// settingEdit is one setting of a source as the listener has it in front of
// them: what they typed or chose, and what was last kept. Only the main
// thread changes it; a save in flight disables the control until its posted
// end arrives.
type settingEdit struct {
	value  string
	saved  string
	saving bool
	failed bool
}

// grantDialog asks whether a plugin may start programs, which can do what
// the listener can.
type grantDialog struct {
	open   bool
	plugin *plugin.Plugin
}

// pluginGroups draws a group of the settings window for each plugin: what it
// may reach, whether it may start programs, and the settings it asks for.
func (a *app) pluginGroups(c *ui.Context, p palette, group func(string, func())) {
	for _, pl := range a.plugins {
		info := pl.Info()
		title := "音源：" + info.Name
		group(title, func() {
			a.pluginRows(c, p, pl, info)
		})
	}
}

// settingRow lays out a row of the settings window: what it is, its help and
// what comes after the help, to the left of its control.
func settingRow(c *ui.Context, p palette, label, help string, more func(), control func()) {
	ui.Row(c).Padding(12, 16).Gap(16).AlignItems(ui.Center).MinHeight(48).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, label).FontSize(13.5).FontWeight(500)
			if help != "" {
				ui.Text(c, help).FontSize(12).TextColor(p.muted).LineHeight(1.4)
			}
			if more != nil {
				more()
			}
		})
		if control != nil {
			control()
		}
	})
}

func (a *app) pluginRows(c *ui.Context, p palette, pl *plugin.Plugin, info music.Info) {
	id := info.ID
	name := info.Name
	if info.Version != "" {
		name += " " + info.Version
	}
	settingRow(c, p, name, permissionText(pl.Permissions()), nil, nil)
	if err := a.sourceErr[id]; err != "" {
		settingRow(c, p, err, "", nil, nil)
	}
	if pl.Can("signIn") || pl.Can("account") {
		a.pluginAccountRow(c, p, pl, info)
	}
	if len(pl.Permissions().Process) > 0 {
		settingRow(c, p, "允许启动程序", "开启后，这个音源可以运行："+programsText(pl.Permissions().Process)+"。这些程序能做你能做的一切，只允许你信任的音源。", nil, func() {
			on := pl.Granted()
			if ui.Switch(c.Key(id+"/grant"), &on).Label("允许 " + info.Name + " 启动程序").Changed() {
				if on {
					a.granting = grantDialog{open: true, plugin: pl}
				} else {
					a.grant(pl, false)
				}
			}
		})
	}
	for _, st := range info.Settings {
		a.settingControl(c, p, pl, id, st)
	}
}

// permissionText says what a plugin may reach, for the listener to read.
func permissionText(perm plugin.Permissions) string {
	var parts []string
	switch {
	case slices.Contains(perm.Network, "*"):
		parts = append(parts, "可访问任意网站")
	case len(perm.Network) > 0:
		parts = append(parts, "可访问网站："+strings.Join(perm.Network, "、"))
	}
	if len(perm.Process) > 0 {
		parts = append(parts, "可启动程序："+programsText(perm.Process))
	}
	if len(parts) == 0 {
		return "不访问网络，也不启动程序"
	}
	return strings.Join(parts, "；")
}

// programsText names the programs a plugin may start.
func programsText(ps plugin.Programs) string {
	if ps.Any() {
		return "任意程序"
	}
	return strings.Join(ps, "、")
}

// draftOf returns the listener's edit of the setting, made from what the
// source has when there is none, and made again from it while nothing of the
// listener's is waiting, so that a change from elsewhere shows.
func (a *app) draftOf(src music.Source, id, key string) *settingEdit {
	if a.settingEdits == nil {
		a.settingEdits = map[string]*settingEdit{}
	}
	now := src.Setting(key)
	k := id + "\x00" + key
	e := a.settingEdits[k]
	if e == nil || !e.saving && !e.failed && e.value == e.saved && now != e.saved {
		e = &settingEdit{value: now, saved: now}
		a.settingEdits[k] = e
	}
	return e
}

// settingControl draws one setting a source asks for, as its type says: a
// switch or a menu keep at once what is chosen; text and passwords are kept
// with Save, so that what is half typed is not.
func (a *app) settingControl(c *ui.Context, p palette, src music.Source, id string, st music.Setting) {
	e := a.draftOf(src, id, st.Key)
	more := func() {
		if st.Link != "" {
			ui.RichText(c).FontSize(11.5).Children(func() { ui.Link(c, st.Link, st.Link) })
		}
		if e.failed {
			ui.Text(c, "没能保存").FontSize(11.5).TextColor(ui.Hex("#ff8a80"))
		}
	}
	key := c.Key(id + "/" + st.Key)
	switch {
	case st.Type == "boolean" && (e.value == "" || e.value == "true" || e.value == "false"):
		settingRow(c, p, st.Label, st.Help, more, func() {
			on := e.value == "true"
			if ui.Switch(key, &on).Label(st.Label).Disabled(e.saving).Changed() {
				e.value = strconv.FormatBool(on)
				a.keepSetting(src, st.Key, e)
			}
		})
	case st.Type == "select" && validSettingOptions(st.Options):
		settingRow(c, p, st.Label, st.Help, more, func() {
			names := make([]string, 0, len(st.Options)+1)
			chosen := ""
			for _, option := range st.Options {
				names = append(names, option.Label)
				if option.Value == e.value {
					chosen = option.Label
				}
			}
			if chosen == "" {
				// A value that is none of the options, or none yet, stays, shown,
				// until the listener chooses one.
				chosen = fmt.Sprintf("当前值 [%s]", e.value)
				if e.value == "" {
					chosen = "未选择"
				}
				for slices.Contains(names, chosen) {
					chosen += " "
				}
				names = append([]string{chosen}, names...)
			}
			if ui.Select(key, &chosen, names).Label(st.Label).Width(180).Disabled(e.saving).Changed() {
				for _, option := range st.Options {
					if chosen == option.Label {
						e.value = option.Value
						a.keepSetting(src, st.Key, e)
						break
					}
				}
			}
		})
	default:
		settingRow(c, p, st.Label, st.Help, more, func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				input := ui.TextInput(key, &e.value).Width(200).Label(st.Label).FontSize(13).Disabled(e.saving)
				if st.Type == "password" {
					input.Password()
				}
				dirty := e.value != e.saved
				if ui.Button(c.Key(id+"/"+st.Key+"/save"), "保存").Disabled(e.saving || !dirty).Clicked() {
					a.keepSetting(src, st.Key, e)
				}
			})
		})
	}
}

// validSettingOptions says whether a select's options can be told apart:
// ambiguous or incomplete ones are edited as text, which never changes a value.
func validSettingOptions(options []music.SettingOption) bool {
	if len(options) == 0 {
		return false
	}
	values, labels := map[string]bool{}, map[string]bool{}
	for _, option := range options {
		if option.Label == "" || values[option.Value] || labels[option.Label] {
			return false
		}
		values[option.Value], labels[option.Label] = true, true
	}
	return true
}

// keepSetting gives the source the setting as edited. The source may take a
// while (a running plugin hears of it), so it is asked off the main thread.
func (a *app) keepSetting(src music.Source, key string, e *settingEdit) {
	if e.saving || e.value == e.saved && !e.failed {
		return
	}
	e.saving = true
	value := e.value
	go func() {
		err := src.SetSetting(key, value)
		a.post(func() {
			e.saving, e.failed = false, err != nil
			if err == nil {
				e.saved = value
			} else {
				log.Printf("source setting %s: %v", key, err)
			}
		})
	}()
}

// grant lets a plugin start programs, or stops it. It takes the plugin's
// runtime down, and ends the programs it started, so it is done off the main
// thread.
func (a *app) grant(pl *plugin.Plugin, on bool) {
	id := pl.Info().ID
	delete(a.sourceErr, id)
	go func() {
		err := pl.Grant(on)
		a.post(func() {
			if err != nil {
				if a.sourceErr == nil {
					a.sourceErr = map[string]string{}
				}
				a.sourceErr[id] = "没能保存这项选择：" + err.Error()
			}
		})
	}()
}

// grantDialogView asks whether the plugin may start programs.
func (a *app) grantDialogView(c *ui.Context, p palette) {
	d := &a.granting
	ui.DialogBase(c.Key("grant"), &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(380).Padding(24).Radius(18).Gap(8).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.94}, Exit: &ui.Motion{Scale: 0.96}})
		name, programs := "", ""
		if d.plugin != nil {
			name, programs = d.plugin.Info().Name, programsText(d.plugin.Permissions().Process)
		}
		ui.Text(c, "允许启动程序").FontSize(17).FontWeight(700)
		ui.Text(c, "“"+name+"”想在你的电脑上运行："+programs+"。这些程序能做你能做的一切，包括读写你的文件。只允许你信任的音源。").
			FontSize(13).TextColor(p.muted).LineHeight(1.45)
		ui.Row(c).Gap(10).Justify(ui.End).Margin(8, 0, 0, 0).Children(func() {
			if pillButton(c.Key("cancel"), nil, "取消", p.hover.Over(p.bg), p.text).Clicked() {
				d.open = false
			}
			if pillButton(c.Key("ok"), nil, "允许", p.accent, p.onAccent).Clicked() {
				d.open = false
				if d.plugin != nil {
					a.grant(d.plugin, true)
				}
			}
		})
	})
}

// pluginAccountRow shows who is signed in to the plugin, with a way to sign out,
// or to sign in, or says the plugin needs setting up first.
func (a *app) pluginAccountRow(c *ui.Context, p palette, pl *plugin.Plugin, info music.Info) {
	a.askAccount(pl)
	st := a.stateOf(info.ID)
	switch {
	case st.account != nil:
		settingRow(c, p, st.account.Name, "已登录 "+info.Name, nil, func() {
			if pl.Can("signOut") && ui.Button(c.Key(info.ID+"/signout"), "退出登录").Clicked() {
				a.signOutOf(pl)
			}
		})
	case st.needsSetup:
		reason := st.setupReason
		if reason == "" {
			reason = "先填写下面的设置，才能登录"
		}
		settingRow(c, p, "需要先设置", reason, nil, nil)
	default:
		settingRow(c, p, "未登录", "登录后，可以使用你在 "+info.Name+" 的歌单与收藏", nil, func() {
			if pl.Can("signIn") && ui.PrimaryButton(c.Key(info.ID+"/signin"), "登录").Clicked() {
				a.showMain()
				a.openLoginFor(pl)
				a.redraw()
			}
		})
	}
}
