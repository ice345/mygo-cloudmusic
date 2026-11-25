package app

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
)

// The listener's own playlists: making them, renaming them, deleting them,
// and adding songs to them or taking songs out.

// editErr does an edit with the source, or fails as unsupported if there is
// none.
func (a *app) editErr(src music.Source, do func() error) error {
	if src == nil {
		return music.ErrUnsupported
	}
	return do()
}

// nid is a NetEase number as a source's ID.
func nid(n int64) string { return strconv.FormatInt(n, 10) }

// nameDialog asks for a playlist's name: a new one's, or one's new name.
type nameDialog struct {
	open   bool
	name   string
	rename string       // the playlist renamed, or "" to make one
	add    *music.Track // the song to add to the playlist made, if any
	busy   bool         // the source is making the playlist
	src    string       // the source whose playlist it is
}

// deleteDialog asks whether to delete one of the listener's playlists.
type deleteDialog struct {
	open bool
	src  string // the source whose playlist it is
	id   string
	name string
}

// editable returns the listener's playlist id at the source if they may edit its
// songs and name, or nil: the liked songs' are their hearts'.
func (a *app) editable(src, id string) *music.Playlist {
	mine, _ := a.listsOf(src)
	for i := range *mine {
		if (*mine)[i].ID == id && !(*mine)[i].Fixed {
			return &(*mine)[i]
		}
	}
	return nil
}

// isSavedID reports whether the listener saved the playlist id at the source,
// as its page or the sidebar says.
func (a *app) isSavedID(src, id string, saved bool) bool {
	_, list := a.listsOf(src)
	return saved || slices.ContainsFunc(*list, func(s music.Playlist) bool { return s.ID == id })
}

// isSaved reports whether the listener saved the playlist pl shows.
func (a *app) isSaved(pl *music.Playlist) bool { return a.isSavedID(pl.Source, pl.ID, pl.Saved) }

// savable reports whether the listener may save playlist pl: anyone's but
// their own, which NetEase's record says and a plugin marks. Signed out,
// saving asks them to sign in.
func (a *app) savable(pl *music.Playlist) bool {
	if pl.Source != netease.SourceID {
		return !pl.Mine
	}
	return a.account == nil || playlistRecord(pl).Creator.UserID != a.uid()
}

// toggleSaved saves playlist pl for the listener, or unsaves it, at once
// in the app and then with its source, going back if the source refuses.
// Signed out, it asks the listener to sign in.
func (a *app) toggleSaved(pl music.Playlist) {
	srcID := pl.Source
	if !a.signedInTo(srcID) {
		a.signInTo(srcID)
		return
	}
	id := pl.ID
	if a.savingPl[id] {
		return
	}
	if a.savingPl == nil {
		a.savingPl = map[string]bool{}
	}
	a.savingPl[id] = true
	save := !a.isSavedID(srcID, id, pl.Saved)
	at := a.setSaved(pl, save, 0)
	who := a.accountID(srcID)
	src := a.source(srcID)
	go func() {
		err := a.editErr(src, func() error { return src.SavePlaylist(id, save) })
		a.post(func() {
			delete(a.savingPl, id)
			switch {
			case srcID == netease.SourceID && a.accountID(srcID) != who:
				// Signed out or in as someone else meanwhile.
			case err != nil:
				a.setSaved(pl, !save, max(at, 0))
				if !a.signInNeeded(srcID, err) {
					a.toast = "操作失败，请稍后再试"
				}
			case save:
				a.toast = "已收藏"
			default:
				a.toast = "已取消收藏"
			}
		})
	}()
}

// setSaved marks playlist pl saved or not, on its page and in the sidebar,
// where a saved one goes at index at, and returns where it was there, or
// -1.
func (a *app) setSaved(pl music.Playlist, save bool, at int) int {
	if loaded := a.playlists[pageKey{pl.Source, pl.ID}]; loaded != nil {
		loaded.Saved = save
	}
	_, list := a.listsOf(pl.Source)
	// A new slice, not shifting the one the sidebar may be listing.
	saved := slices.Clone(*list)
	was := slices.IndexFunc(saved, func(s music.Playlist) bool { return s.ID == pl.ID })
	if was >= 0 {
		saved = slices.Delete(saved, was, was+1)
	}
	if save {
		pl.Saved = true
		pl.Tracks = nil
		saved = slices.Insert(saved, min(at, len(saved)), pl)
	}
	*list = saved
	return was
}

// loadMine loads the playlists the listener uid made and saved.
func (a *app) loadMine(uid int64) {
	src := a.accountSource()
	if src == nil {
		return
	}
	go func() {
		lists, err := src.Playlists()
		a.post(func() {
			if err != nil || a.account == nil || a.uid() != uid {
				return
			}
			var made, saved []music.Playlist
			for _, pl := range lists {
				if pl.Saved {
					saved = append(saved, pl)
				} else {
					made = append(made, pl)
				}
			}
			a.mine, a.saved = made, saved
		})
	}()
}

// playlistChanged forgets what was loaded of playlist id at the source, which
// it changed: loaded anew if shown, and the sidebar's covers with it.
func (a *app) playlistChanged(src, id string) {
	k := pageKey{src, id}
	delete(a.playlists, k)
	if a.route.page == pagePlaylist && a.route.key() == k {
		a.loadPlaylist(k, false)
	}
	a.reloadLists(src)
}

// newPlaylist asks for the name of a playlist to make, at the source of tr, which
// it adds to it, or at the source shown if there is none. Signed out, it asks
// the listener to sign in.
func (a *app) newPlaylist(tr *music.Track) {
	src := a.activeID()
	if tr != nil {
		src = tr.Source
	}
	if !a.signedInTo(src) {
		a.signInTo(src)
		return
	}
	a.naming = nameDialog{open: true, add: tr, src: src}
}

// startRename asks for a new name for the listener's playlist pl.
func (a *app) startRename(pl *music.Playlist) {
	a.naming = nameDialog{open: true, name: pl.Name, rename: pl.ID, src: pl.Source}
}

// submitName makes the playlist named, or renames one: at once in the app,
// as its source may refuse, when renaming.
func (a *app) submitName() {
	d := &a.naming
	name := strings.TrimSpace(d.name)
	if name == "" || d.busy {
		return
	}
	srcID := d.src
	if d.rename != "" {
		id := d.rename
		d.open = false
		pl := a.editable(srcID, id)
		if pl == nil || pl.Name == name {
			return
		}
		old := pl.Name
		a.setPlaylistName(srcID, id, name)
		src := a.source(srcID)
		go func() {
			err := a.editErr(src, func() error { return src.RenamePlaylist(id, name) })
			a.post(func() {
				if err != nil {
					a.setPlaylistName(srcID, id, old)
					if !a.signInNeeded(srcID, err) {
						a.toast = "重命名失败，请稍后再试"
					}
				}
			})
		}()
		return
	}
	d.busy = true
	add := d.add
	src := a.source(srcID)
	go func() {
		var pl *music.Playlist
		err := a.editErr(src, func() (err error) { pl, err = src.CreatePlaylist(name); return })
		a.post(func() {
			a.naming.busy = false
			if err != nil {
				if !a.signInNeeded(srcID, err) {
					a.toast = "创建歌单失败，请稍后再试"
				}
				return
			}
			a.naming.open = false
			// Under the liked songs, as NetEase lists the newest.
			mine, _ := a.listsOf(srcID)
			at := 0
			if len(*mine) > 0 && (*mine)[0].Fixed {
				at = 1
			}
			*mine = slices.Insert(*mine, at, *pl)
			pl.Tracks = nil
			a.playlists[pageKey{pl.Source, pl.ID}] = pl
			a.open(route{page: pagePlaylist, source: pl.Source, id: pl.ID})
			if add != nil {
				a.addTo(*add, pl.ID)
			}
		})
	}()
}

// confirmDelete asks whether to delete the listener's playlist pl.
func (a *app) confirmDelete(pl *music.Playlist) {
	a.deleting = deleteDialog{open: true, src: pl.Source, id: pl.ID, name: pl.Name}
}

// deleteChosen deletes the playlist the dialog asked about: at once from
// the sidebar, leaving its page, and then with its source, which may refuse:
// it comes back then.
func (a *app) deleteChosen() {
	srcID, id := a.deleting.src, a.deleting.id
	a.deleting.open = false
	mine, _ := a.listsOf(srcID)
	i := slices.IndexFunc(*mine, func(pl music.Playlist) bool { return pl.ID == id })
	if i < 0 || (*mine)[i].Fixed {
		return
	}
	pl := (*mine)[i]
	*mine = slices.Delete(slices.Clone(*mine), i, i+1)
	// Leave no way back to its page, and leave it for the page before.
	k := pageKey{srcID, id}
	gone := func(r route) bool { return r.page == pagePlaylist && r.key() == k }
	a.history = slices.DeleteFunc(a.history, gone)
	a.forward = slices.DeleteFunc(a.forward, gone)
	if gone(a.route) {
		r := homeRoute(srcID)
		if n := len(a.history); n > 0 {
			r, a.history = a.history[n-1], a.history[:n-1]
		}
		a.show(r)
	}
	src := a.source(srcID)
	go func() {
		err := a.editErr(src, func() error { return src.DeletePlaylist(id) })
		a.post(func() {
			if err != nil {
				mine, _ := a.listsOf(srcID)
				*mine = slices.Insert(*mine, min(i, len(*mine)), pl)
				if !a.signInNeeded(srcID, err) {
					a.toast = "删除失败，请稍后再试"
				}
				return
			}
			delete(a.playlists, k)
			a.toast = "已删除歌单"
		})
	}()
}

func (a *app) setPlaylistName(src, id, name string) {
	if pl := a.editable(src, id); pl != nil {
		pl.Name = name
	}
	if pl := a.playlists[pageKey{src, id}]; pl != nil {
		pl.Name = name
	}
}

// addTo adds tr to the listener's playlist pid at its source; a toast says how
// it went.
func (a *app) addTo(tr music.Track, pid string) {
	song := tr.ID
	srcID := tr.Source
	src := a.source(srcID)
	go func() {
		err := a.editErr(src, func() error { return src.AddToPlaylist(pid, []string{song}) })
		a.post(func() {
			switch {
			case errors.Is(err, music.ErrDuplicate):
				a.toast = "歌曲已在歌单中"
			case a.signInNeeded(srcID, err):
			case err != nil:
				a.toast = "添加失败，请稍后再试"
			default:
				a.toast = "已添加到歌单"
				a.playlistChanged(srcID, pid)
			}
		})
	}()
}

// removeFrom takes tr out of the listener's playlist pid at its source, at once
// in the app and then with the source, which may refuse: the song comes back
// then.
func (a *app) removeFrom(pid string, tr music.Track) {
	song := tr.ID
	srcID := tr.Source
	k := pageKey{srcID, pid}
	pl := a.playlists[k]
	var before []music.Track
	if pl != nil {
		// A new list: the player may be playing the old one.
		before = pl.Tracks
		pl.Tracks = slices.DeleteFunc(slices.Clone(before), func(t music.Track) bool { return t.ID == song })
	}
	src := a.source(srcID)
	go func() {
		err := a.editErr(src, func() error { return src.RemoveFromPlaylist(pid, []string{song}) })
		a.post(func() {
			if err != nil {
				if pl != nil && a.playlists[k] == pl {
					pl.Tracks = before
				}
				if !a.signInNeeded(srcID, err) {
					a.toast = "移除失败，请稍后再试"
				}
				return
			}
			// The songs listed are right: only the sidebar's covers may
			// have changed.
			a.reloadLists(srcID)
		})
	}()
}

// playlistItems are the items of a song's menu that put it in the
// listener's playlists, or take it out of pid, the one it is listed in, if
// set. The playlists are those of the song's own source: NetEase's, or a
// plugin's that makes them.
func (a *app) playlistItems(m *ui.Menu, tr music.Track, pid string) {
	src := tr.Source
	if !a.signedInTo(src) || !a.canDo(src, "addToPlaylist") && !a.canDo(src, "createPlaylist") {
		return
	}
	// A plugin's playlists are asked for as its songs' menus first want them.
	if s := a.source(src); s != nil && src != netease.SourceID {
		a.loadPluginLists(s, false)
	}
	// The playlist listed is of the source the page is of.
	if a.route.source != src {
		pid = ""
	}
	mine, _ := a.listsOf(src)
	m.Separator()
	m.Submenu("添加到歌单", func(m *ui.Menu) {
		if a.canDo(src, "createPlaylist") && m.Item("新建歌单…").Chosen() {
			a.newPlaylist(&tr)
		}
		var to []*music.Playlist
		if a.canDo(src, "addToPlaylist") {
			for i := range *mine {
				if pl := &(*mine)[i]; !pl.Fixed && pl.ID != pid {
					to = append(to, pl)
				}
			}
		}
		if len(to) > 0 {
			m.Separator()
		}
		for _, pl := range to {
			if m.Item(pl.Name).Chosen() {
				a.addTo(tr, pl.ID)
			}
		}
	})
	if a.editable(src, pid) != nil && a.canDo(src, "removeFromPlaylist") && m.Item("从歌单中移除").Chosen() {
		a.removeFrom(pid, tr)
	}
}

// playlistNameDialog asks for the name of a playlist to make, or a new
// name for one.
func (a *app) playlistNameDialog(c *ui.Context) {
	p := a.pal
	d := &a.naming
	ui.DialogBase(c.Key("playlist-name"), &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(360).Padding(24).Radius(18).Gap(16).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.94}, Exit: &ui.Motion{Scale: 0.96}})
		title, action := "新建歌单", "创建"
		if d.rename != "" {
			title, action = "重命名歌单", "保存"
		}
		if d.busy {
			action = "正在创建…"
		}
		ui.Text(c, title).FontSize(17).FontWeight(700)
		in := ui.TextInput(c.Key("name"), &d.name).Placeholder("歌单名称").Label("歌单名称").FontSize(14).AutoFocus()
		if in.Submitted() {
			a.submitName()
		}
		ui.Row(c).Gap(10).Justify(ui.End).Children(func() {
			if pillButton(c.Key("cancel"), nil, "取消", p.hover.Over(p.bg), p.text).Clicked() {
				d.open = false
			}
			ok := pillButton(c.Key("ok"), nil, action, p.accent, p.onAccent)
			if strings.TrimSpace(d.name) == "" {
				ok.Disabled(true).Opacity(0.5)
			}
			if ok.Clicked() {
				a.submitName()
			}
		})
	})
}

// deletePlaylistDialog asks whether to delete a playlist, which can't be
// undone.
func (a *app) deletePlaylistDialog(c *ui.Context) {
	p := a.pal
	d := &a.deleting
	ui.DialogBase(c.Key("playlist-delete"), &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(360).Padding(24).Radius(18).Gap(8).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.94}, Exit: &ui.Motion{Scale: 0.96}})
		ui.Text(c, "删除歌单").FontSize(17).FontWeight(700)
		ui.Text(c, "确定删除歌单“"+d.name+"”吗？删除后无法恢复。").FontSize(13).TextColor(p.muted).LineHeight(1.45)
		ui.Row(c).Gap(10).Justify(ui.End).Margin(8, 0, 0, 0).Children(func() {
			if pillButton(c.Key("cancel"), nil, "取消", p.hover.Over(p.bg), p.text).Clicked() {
				d.open = false
			}
			if pillButton(c.Key("ok"), nil, "删除", p.accent, p.onAccent).Clicked() {
				a.deleteChosen()
			}
		})
	})
}
