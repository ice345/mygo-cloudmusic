package app

import (
	"context"
	"errors"
	"runtime"
	"time"

	"github.com/egoist/mygo/ui"
	"rsc.io/qr"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
)

// loginStage is where a sign-in is: its code loading, waiting to be scanned,
// scanned and waiting to be confirmed on the phone, or run out.
type loginStage int

const (
	loginLoading loginStage = iota
	loginWaiting
	loginScanned
	loginExpired
)

// loginState is a QR code sign-in under way.
type loginState struct {
	open    bool
	gen     int // counts sign-ins, to ignore the ends of closed ones
	code    *qr.Code
	qrText  string // what the code shows
	stage   loginStage
	scanner string // who scanned the code, and their picture
	avatar  string
	err     string
	// finished says they confirmed, and the dialog is greeting them.
	finished bool
	// running says a sign-in is waiting, to be given up if the dialog closes.
	running bool
}

// uid is the NetEase number of who is signed in, or 0.
func (a *app) uid() int64 {
	if a.account == nil {
		return 0
	}
	return numericID(a.account.ID)
}

// accountSource is the source whose account the app shows: NetEase, so far.
func (a *app) accountSource() music.Source { return a.source(netease.SourceID) }

// boot signs in with the session in the keychain, if any, then loads the
// home page for whoever is listening.
func (a *app) boot() {
	a.homeLoading = true
	src := a.accountSource()
	go func() {
		var acc *music.Account
		if src != nil {
			// A session that could not be asked about, offline, is nobody's yet.
			acc, _ = src.Account()
		}
		a.post(func() {
			a.signedIn(acc)
		})
	}()
}

// signedIn shows what is personal to who, or to nobody.
func (a *app) signedIn(who *music.Account) {
	a.account = who
	// Playback picks up where it was left, once the session that lets
	// the listener play their songs is back.
	a.restoreSession()
	a.mine, a.saved, a.recPlaylists, a.liked = nil, nil, nil, nil
	a.likeAsked = nil
	// What the listener may play, and follows, changed: load the pages
	// again.
	a.playlists = map[pageKey]*music.Playlist{}
	a.plErr = map[pageKey]string{}
	a.artists, a.albums, a.users, a.podcasts = map[pageKey]*artistData{}, map[pageKey]*albumData{}, map[int64]*userData{}, map[int64]*podcastData{}
	a.podcastTab = &podcastTab{}
	a.searches = map[route]*searchData{}
	a.load(a.route)
	a.loadHome()
	if who == nil {
		return
	}
	a.loadLiked(a.uid())
	a.loadMine(a.uid())
}

// openLogin starts a sign-in, giving up the one under way: the source shows
// the code and who scanned it through the prompt, and returns who confirmed.
func (a *app) openLogin() {
	src := a.accountSource()
	a.giveUpLogin()
	a.login = loginState{open: true, gen: a.login.gen + 1, running: src != nil}
	gen := a.login.gen
	if src == nil {
		a.login.err = "无法登录"
		return
	}
	if p, ok := src.(music.Prompter); ok {
		p.OnSignInPrompt(func(prompt music.SignInPrompt) {
			a.post(func() { a.loginPrompt(gen, prompt) })
		})
	}
	go func() {
		who, err := src.SignIn()
		a.post(func() { a.loginEnded(gen, who, err) })
	}()
}

// giveUpLogin stops the sign-in waiting, if any.
func (a *app) giveUpLogin() {
	if !a.login.running {
		return
	}
	a.login.running = false
	if c, ok := a.accountSource().(music.Canceler); ok {
		go c.CancelSignIn()
	}
}

// loginPrompt shows what the source says of the sign-in: its code, and who
// scanned it.
func (a *app) loginPrompt(gen int, p music.SignInPrompt) {
	l := &a.login
	if gen != l.gen || !l.open {
		return
	}
	if p.QR != "" && p.QR != l.qrText {
		code, err := qr.Encode(p.QR, qr.M)
		if err != nil {
			l.err = "无法获取二维码，请检查网络"
			return
		}
		l.code, l.qrText = code, p.QR
	}
	l.stage = loginWaiting
	if p.Name != "" {
		l.stage, l.scanner, l.avatar = loginScanned, p.Name, p.ImageURL
	}
}

// loginEnded takes how a sign-in ended: who confirmed, a code that ran out, or
// a refusal.
func (a *app) loginEnded(gen int, who *music.Account, err error) {
	l := &a.login
	if gen != l.gen {
		return
	}
	l.running = false
	switch {
	case !l.open || errors.Is(err, context.Canceled):
		return
	case errors.Is(err, music.ErrExpired):
		l.stage = loginExpired
	case err != nil:
		switch {
		case l.code == nil:
			l.err = "无法获取二维码，请检查网络"
		case music.Reason(err) != "":
			l.err = music.Reason(err)
		default:
			l.err = "登录失败，请稍后再试"
		}
	default:
		l.finished = true
		a.signedIn(who)
		a.pickUp(a.uid())
		a.closeLoginSoon(gen)
	}
}

// closeLoginSoon closes the dialog a moment after it showed the welcome.
func (a *app) closeLoginSoon(gen int) {
	go func() {
		time.Sleep(1200 * time.Millisecond)
		a.post(func() {
			if gen == a.login.gen {
				a.login.open = false
			}
		})
	}()
}

func (a *app) signOut() {
	a.giveUpLogin()
	a.login.gen++
	a.signedIn(nil)
	if src := a.accountSource(); src != nil {
		go src.SignOut()
	}
}

// loginDialog shows the QR code to scan with the NetEase app on a phone.
func (a *app) loginDialog(c *ui.Context) {
	p := a.pal
	l := &a.login
	// Closed while it waited: the sign-in is given up.
	if !l.open {
		a.giveUpLogin()
	}
	ui.DialogBase(c.Key("login"), &l.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(340).Padding(28, 28, 26, 28).Radius(22).Gap(6).AlignItems(ui.Center).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.94}, Exit: &ui.Motion{Scale: 0.96}})
		title, sub := "扫码登录", "打开网易云音乐 App，扫一扫登录"
		switch {
		case l.finished:
			title, sub = "登录成功", "欢迎回来，"+a.profileName()
		case l.err != "":
			sub = l.err
		case l.stage == loginScanned:
			title, sub = "扫描成功", "请在手机上确认登录"
		case l.stage == loginExpired:
			sub = "二维码已失效"
		}
		ui.Text(c, title).FontSize(20).FontWeight(700)
		ui.Text(c, sub).FontSize(13).TextColor(p.muted).TextAlign(ui.Center).Margin(0, 0, 14, 0)

		ui.Box(c).Size(212, 212).Radius(16).Background(ui.RGB(255, 255, 255)).Border(1, p.hairline).Children(func() {
			if l.code != nil {
				code := l.code
				opacity := float32(1)
				if l.stage == loginScanned || l.stage == loginExpired || l.finished || l.err != "" {
					opacity = 0.12
				}
				ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Opacity(opacity).
					Draw(func(pt *ui.Painter, r ui.Rect) { drawQR(pt, r, code, 14) })
			} else if l.err == "" {
				// On white whatever the appearance, as the code is.
				s := shimmer{base: ui.RGBA(0, 0, 0, 0.08), sheen: ui.RGBA(0, 0, 0, 0.03)}.in(c)
				qrSkeleton(c, s, 14).Absolute().Top(0).Left(0).Right(0).Bottom(0)
			}
			ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Gap(10).Children(func() {
				switch {
				case l.finished || l.stage == loginScanned:
					if bmp := bitmap(a.art.get(sizedImage(l.avatar, 160), false)); bmp != nil {
						pic := ui.Box(c).Size(72, 72).Radius(36).Shadow(0, 6, 16, -4, ui.RGBA(0, 0, 0, 0.3))
						in := artIn(pic, bmp)
						pic.Children(func() { artImage(c, bmp, 36, in) })
					}
					if l.scanner != "" {
						ui.Text(c, l.scanner).FontSize(14).FontWeight(600).TextColor(ui.Hex("#1d1d1f"))
					}
				case l.stage == loginExpired || l.err != "":
					if pillButton(c.Key("refresh"), iconRepeat, "刷新二维码", p.accent, p.onAccent).Clicked() {
						a.openLogin()
					}
				}
			})
		})
		ui.Text(c, "登录后，每日推荐与歌单将根据你的口味生成").FontSize(11.5).TextColor(p.faint).Margin(16, 0, 0, 0)
	})
}

// drawQR draws a QR code in r, as crisp squares on white, inside a quiet
// zone of pad.
func drawQR(p *ui.Painter, r ui.Rect, code *qr.Code, pad float32) {
	n := code.Size
	cell := (min(r.W, r.H) - 2*pad) / float32(n)
	x0 := r.X + (r.W-cell*float32(n))/2
	y0 := r.Y + (r.H-cell*float32(n))/2
	ink := ui.Hex("#141416")
	for y := range n {
		for x := range n {
			if code.Black(x, y) {
				// A hair larger, so neighbors join without seams.
				p.Fill(ui.Rect{X: x0 + float32(x)*cell, Y: y0 + float32(y)*cell, W: cell + 0.3, H: cell + 0.3}, ink, 0)
			}
		}
	}
}

func (a *app) profileName() string {
	if a.account == nil {
		return ""
	}
	return a.account.Name
}

// accountRow sits at the foot of the sidebar: who is signed in, with a
// dropdown to open their page or the settings or sign out, or a way to
// sign in.
func (a *app) accountRow(c *ui.Context) {
	p := a.pal
	b := ui.ButtonBase(c.Key("account")).Height(40).PaddingX(8).Gap(10).Radius(10).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false)
	if b.Hovered() || a.accountOpen && a.account != nil {
		b.Background(p.hover)
	}
	if a.account == nil {
		a.accountOpen = false
		b.Label("登录").Children(func() {
			ui.Box(c).Size(26, 26).Radius(13).Background(p.press).Center().Children(func() {
				ui.Icon(c, iconUser).Size(14, 14).TextColor(p.muted)
			})
			ui.Text(c, "登录网易云音乐").FontSize(13).FontWeight(500).TextColor(p.muted)
		})
		if b.Clicked() {
			a.openLogin()
		}
		return
	}
	prof := a.account
	b.Label(prof.Name).Expanded(a.accountOpen).Children(func() {
		a.avatar(c, prof.ImageURL, 26, p.press)
		ui.Text(c, prof.Name).FontSize(13).FontWeight(600).SingleLine().Grow(1).Shrink(1).MinWidth(0)
		ui.Icon(c, iconUpDown).Size(13, 13).TextColor(p.faint)
	})
	if b.Clicked() {
		a.accountOpen = !a.accountOpen
	}
	a.accountMenu(c, b)
}

// accountMenu drops up from the account row, as wide as it, in the colors
// of the volume popover: the listener's own page and the settings, then
// signing out.
func (a *app) accountMenu(c *ui.Context, row ui.Element) {
	pop := popoverColors(a.pal.dark)
	ui.PopoverBase(c.Key("account-menu"), row, &a.accountOpen, func(panel ui.Element) {
		panel.AttachTo(row, ui.AnchorTopStart, ui.AnchorBottomStart).Margin(0, 0, 6, 0).Padding(5).Gap(1).
			Radius(12).Background(pop.fill).Border(0.5, pop.edge).Shadow(0, 8, 24, -4, ui.RGBA(0, 0, 0, 0.2)).
			Role(ui.RoleMenu).FocusGroup(ui.Vertical).Label("账户").TransformOrigin(0.5, 1).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.96, Y: 4}, Exit: &ui.Motion{Scale: 0.98}})
		if w := row.Bounds().W; w > 0 {
			panel.Width(w)
		}
		shortcut := "Ctrl+,"
		if runtime.GOOS == "darwin" {
			shortcut = "⌘,"
		}
		if a.menuItem(c, iconUser, "我的主页", "").AutoFocus().Clicked() {
			a.accountOpen = false
			a.open(nroute(pageUser, a.uid()))
		}
		if a.menuItem(c, iconSettings, "设置…", shortcut).Clicked() {
			a.accountOpen = false
			a.openSettings()
		}
		ui.Box(c).Height(0.5).Margin(4, 8).Background(pop.edge)
		if a.menuItem(c, iconSignOut, "退出登录", "").Clicked() {
			a.accountOpen = false
			a.signOut()
		}
	})
}

// menuItem is an item of a dropdown: an icon, its label, and the shortcut
// that does the same, if any.
func (a *app) menuItem(c *ui.Context, icon *ui.SVG, label, shortcut string) ui.Element {
	p := a.pal
	b := ui.ButtonBase(c.Key(label)).Role(ui.RoleMenuItem).Label(label).Height(30).PaddingX(8).Gap(9).Radius(7).
		AlignItems(ui.Center).Justify(ui.Start).FocusRing(false)
	if b.Hovered() || b.FocusVisible() {
		b.Background(p.press)
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(15, 15).TextColor(p.muted)
		ui.Text(c, label).FontSize(13).Grow(1)
		if shortcut != "" {
			ui.Text(c, shortcut).FontSize(12).TextColor(p.faint)
		}
	})
	return b
}

// myPlaylists lists the listener's playlists under a header that opens
// and closes them: those they made, if mine, with a button to make one
// and a menu to rename them, or else with a menu to unsave them.
func (a *app) myPlaylists(c *ui.Context, title string, lists []music.Playlist, mine bool, collapsed *bool) {
	if len(lists) == 0 {
		return
	}
	p := a.pal
	ui.Row(c.Key(title)).AlignItems(ui.Center).Margin(18, 4, 6, 6).Children(func() {
		b := ui.ButtonBase(c.Key("toggle")).Grow(1).Height(26).PaddingX(4).Gap(6).Radius(6).
			AlignItems(ui.Center).Justify(ui.Start).Role(ui.RoleDisclosure).Label(title)
		if b.Clicked() {
			*collapsed = !*collapsed
		}
		b.Expanded(!*collapsed)
		if b.Hovered() {
			b.Background(p.hover)
		}
		b.Children(func() {
			ui.Text(c, title).FontSize(11.5).FontWeight(600).TextColor(p.faint).Grow(1)
			arrow := iconDown
			if *collapsed {
				arrow = iconForward
			}
			ui.Icon(c, arrow).Size(12, 12).TextColor(p.faint)
		})
		if mine && iconButton(c.Key("new"), iconPlus, 13, p.faint, "新建歌单").Size(26, 26).Radius(6).Clicked() {
			a.newPlaylist(nil)
		}
	})
	if *collapsed {
		return
	}
	for i := range lists {
		pl := &lists[i]
		selected := a.route.page == pagePlaylist && a.route.key() == (pageKey{pl.Source, pl.ID}) && !a.nowPlaying
		b := ui.ButtonBase(c.Key(pl.ID)).Height(34).PaddingX(8).Gap(10).Radius(8).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(pl.Name)
		switch {
		case selected:
			b.Background(p.press)
		case b.Hovered():
			b.Background(p.hover)
		}
		b.Children(func() {
			a.cover(c, pl.ImageURL, 80, 22, 5)
			ui.Text(c, pl.Name).FontSize(13).SingleLine().Shrink(1).MinWidth(0)
		})
		switch {
		case mine && !pl.Fixed:
			b.ContextMenu(func(m *ui.Menu) {
				if m.Item("重命名…").Chosen() {
					a.startRename(pl)
				}
				m.Separator()
				if m.Item("删除歌单…").Chosen() {
					a.confirmDelete(pl)
				}
			})
		case !mine:
			b.ContextMenu(func(m *ui.Menu) {
				if m.Item("取消收藏").Chosen() {
					a.toggleSaved(*pl)
				}
			})
		}
		if b.Clicked() {
			a.open(route{page: pagePlaylist, source: pl.Source, id: pl.ID})
		}
	}
}
