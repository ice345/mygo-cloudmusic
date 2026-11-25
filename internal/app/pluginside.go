package app

import (
	"runtime"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
)

// navIcons are the icons a plugin's navigation may name, of the app's.
var navIcons = map[string]*ui.SVG{
	"home":     iconHome,
	"note":     iconNote,
	"album":    iconNote,
	"artist":   iconUser,
	"playlist": iconQueue,
	"radio":    iconRadio,
	"podcast":  iconPodcast,
	"chart":    iconCharts,
	"calendar": iconDaily,
	"heart":    iconHeart,
	"search":   iconSearch,
	"star":     iconHeart,
}

// pluginNav draws the sidebar of the plugin shown: the entries its manifest
// gives, or its home where it gives none and has one, then the last search
// and the listener's playlists at the plugin.
func (a *app) pluginNav(c *ui.Context, src music.Source) {
	info := src.Info()
	items := info.Navigation
	if len(items) == 0 && (src.Can("home") || src.Can("browse")) {
		items = []music.NavItem{{Title: "首页", Builtin: "home", Icon: "home"}}
	}
	for _, it := range items {
		icon := navIcons[it.Icon]
		if icon == nil {
			icon = iconHome
		}
		switch {
		case it.Page != "":
			a.navItem(c, icon, it.Title, route{page: pageBrowse, source: info.ID, id: it.Page})
		case it.Builtin == "home" && (src.Can("home") || src.Can("browse")):
			a.navItem(c, icon, it.Title, homeRoute(info.ID))
		}
		// The other builtins (liked songs, saved albums, stations and shows)
		// are not drawn yet: a plugin's liked songs are in its playlists, as
		// the one it marks fixed.
	}
	if a.searched.query != "" {
		a.navItem(c, iconSearch, "搜索结果", a.searched)
	}
	st := a.stateOf(info.ID)
	a.myPlaylists(c, "我的歌单", st.mine, true, &st.mineCollapsed)
	a.myPlaylists(c, "收藏的歌单", st.saved, false, &st.savedCollapsed)
}

// sidebarAccount sits at the foot of the sidebar while a plugin is shown: who
// is signed in to it, with a dropdown for the settings and signing out, or a
// way to sign in where the plugin has one. A plugin with no accounts has none.
func (a *app) sidebarAccount(c *ui.Context, src music.Source) {
	info := src.Info()
	if !src.Can("signIn") && !src.Can("account") {
		return
	}
	p := a.pal
	st := a.stateOf(info.ID)
	b := ui.ButtonBase(c.Key("account")).Height(40).PaddingX(8).Gap(10).Radius(10).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false)
	if b.Hovered() || a.accountOpen && st.account != nil {
		b.Background(p.hover)
	}
	if st.account == nil {
		a.accountOpen = false
		if !src.Can("signIn") {
			return
		}
		b.Label("登录").Children(func() {
			ui.Box(c).Size(26, 26).Radius(13).Background(p.press).Center().Children(func() {
				ui.Icon(c, iconUser).Size(14, 14).TextColor(p.muted)
			})
			ui.Text(c, "登录 "+info.Name).FontSize(13).FontWeight(500).TextColor(p.muted).SingleLine().Shrink(1).MinWidth(0)
		})
		if b.Clicked() {
			a.openLoginFor(src)
		}
		return
	}
	who := st.account
	b.Label(who.Name).Expanded(a.accountOpen).Children(func() {
		a.avatar(c, who.ImageURL, 26, p.press)
		ui.Text(c, who.Name).FontSize(13).FontWeight(600).SingleLine().Grow(1).Shrink(1).MinWidth(0)
		ui.Icon(c, iconUpDown).Size(13, 13).TextColor(p.faint)
	})
	if b.Clicked() {
		a.accountOpen = !a.accountOpen
	}
	pop := popoverColors(p.dark)
	ui.PopoverBase(c.Key("account-menu"), b, &a.accountOpen, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorTopStart, ui.AnchorBottomStart).Margin(0, 0, 6, 0).Padding(5).Gap(1).
			Radius(12).Background(pop.fill).Border(0.5, pop.edge).Shadow(0, 8, 24, -4, ui.RGBA(0, 0, 0, 0.2)).
			Role(ui.RoleMenu).FocusGroup(ui.Vertical).Label("账户").TransformOrigin(0.5, 1).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.96, Y: 4}, Exit: &ui.Motion{Scale: 0.98}})
		if w := b.Bounds().W; w > 0 {
			panel.Width(w)
		}
		shortcut := "Ctrl+,"
		if runtime.GOOS == "darwin" {
			shortcut = "⌘,"
		}
		if a.menuItem(c, iconSettings, "设置…", shortcut).AutoFocus().Clicked() {
			a.accountOpen = false
			a.openSettings()
		}
		if src.Can("signOut") {
			ui.Box(c).Height(0.5).Margin(4, 8).Background(pop.edge)
			if a.menuItem(c, iconSignOut, "退出登录", "").Clicked() {
				a.accountOpen = false
				a.signOutOf(src)
			}
		}
	})
}
