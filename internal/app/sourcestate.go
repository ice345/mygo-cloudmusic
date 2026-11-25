package app

import (
	"errors"
	"strings"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/plugin"
)

// sourceState is what the app holds of a source that is a plugin: who is signed
// in to it, and why it can't be used yet. NetEase's is the app's own (account
// and the rest), as it was before there were plugins. Only the main thread
// reads or changes it.
type sourceState struct {
	account *music.Account
	asked   bool // who is signed in was asked of the source
	asking  bool // and is being
	// needsSetup says the source asked for a setting to be filled in, and
	// setupReason what it said it is for.
	needsSetup  bool
	setupReason string
	// mine and saved are the listener's playlists at the plugin, made and
	// saved; listsAsked says they were asked for, and the collapsed ones are
	// shut in the sidebar.
	mine, saved                   []music.Playlist
	listsAsked, listsBusy         bool
	mineCollapsed, savedCollapsed bool
}

// stateOf is the state of the source, made when it is first wanted.
func (a *app) stateOf(id string) *sourceState {
	if a.states == nil {
		a.states = map[string]*sourceState{}
	}
	s := a.states[id]
	if s == nil {
		s = &sourceState{}
		a.states[id] = s
	}
	return s
}

// accountOf is who is signed in to the source, as far as it is known.
func (a *app) accountOf(id string) *music.Account {
	if id == netease.SourceID {
		return a.account
	}
	return a.stateOf(id).account
}

// sourceName is how the source is called, or its ID if it is gone.
func (a *app) sourceName(id string) string {
	if src := a.source(id); src != nil {
		return src.Info().Name
	}
	return id
}

// askAccount asks the plugin who is signed in to it, once, off the main
// thread. It starts the plugin, so it is asked when the account is wanted, not
// as the app starts.
func (a *app) askAccount(src music.Source) {
	s := a.stateOf(src.Info().ID)
	if s.asked || s.asking || !src.Can("account") {
		return
	}
	s.asking = true
	go func() {
		who, err := src.Account()
		a.post(func() {
			s.asking, s.asked = false, true
			// A source that could not say is nobody's yet, as NetEase is.
			s.account = who
			s.needsSetup = errors.Is(err, music.ErrSetup)
			s.setupReason = ""
			if s.needsSetup {
				s.setupReason = music.Reason(err)
			}
		})
	}()
}

// pluginSignedIn takes who signed in to the plugin.
func (a *app) pluginSignedIn(id string, who *music.Account) {
	s := a.stateOf(id)
	s.account, s.asked, s.needsSetup, s.setupReason = who, true, false, ""
	// What the listener sees at the plugin is the account's: ask again.
	delete(a.browses, pageKey{id, ""})
	if src := a.source(id); src != nil {
		a.loadPluginLists(src, true)
		if a.route == homeRoute(id) {
			a.loadBrowse(pageKey{id, ""}, true)
		}
	}
}

// signOutOf signs out of the plugin: at once for the listener, then with the
// source.
func (a *app) signOutOf(src music.Source) {
	id := src.Info().ID
	if a.login.source == id {
		a.giveUpLogin()
		a.login.gen++
	}
	s := a.stateOf(id)
	s.account, s.asked = nil, true
	s.mine, s.saved, s.listsAsked = nil, nil, false
	a.browses = map[pageKey]*browseData{}
	go src.SignOut()
	if a.route.page == pageBrowse && a.route.source == id {
		a.loadBrowse(a.route.key(), true)
	}
}

// enterPlugin asks a plugin just shown for what the sidebar shows of it: who
// is signed in, and their playlists.
func (a *app) enterPlugin(src music.Source) {
	a.askAccount(src)
	a.loadPluginLists(src, false)
}

// loadPluginLists loads the listener's playlists at the plugin, off the main
// thread, unless they have been, or again. A plugin that wants a sign-in or a
// setting first leaves them empty: it is said where the listener goes next.
func (a *app) loadPluginLists(src music.Source, again bool) {
	s := a.stateOf(src.Info().ID)
	if !src.Can("playlists") || s.listsBusy || s.listsAsked && !again {
		return
	}
	s.listsBusy = true
	go func() {
		lists, err := src.Playlists()
		a.post(func() {
			s.listsBusy, s.listsAsked = false, true
			// What was had stays when the plugin can't say, as NetEase's does.
			if err != nil {
				return
			}
			s.mine, s.saved = nil, nil
			for _, pl := range lists {
				if pl.Saved {
					s.saved = append(s.saved, pl)
				} else {
					s.mine = append(s.mine, pl)
				}
			}
		})
	}()
}

// listsOf are the playlists the listener made and saved at the source: NetEase's
// are the app's own, a plugin's are in its state.
func (a *app) listsOf(src string) (mine, saved *[]music.Playlist) {
	if src == netease.SourceID {
		return &a.mine, &a.saved
	}
	st := a.stateOf(src)
	return &st.mine, &st.saved
}

// signedInTo reports whether to go on with what needs the listener's account at
// the source. A plugin that has no sign-in has no account to need, and one
// whose account was not asked yet is gone on with: it says if the listener has
// to sign in (music.ErrSignIn), which is what asks them to.
func (a *app) signedInTo(src string) bool {
	if src == netease.SourceID {
		return a.account != nil
	}
	s := a.source(src)
	if s == nil {
		return false
	}
	if !s.Can("signIn") {
		return true
	}
	st := a.stateOf(src)
	if !st.asked {
		a.askAccount(s)
		return true
	}
	return st.account != nil
}

// signInTo asks the listener to sign in to the source.
func (a *app) signInTo(src string) {
	a.showMain()
	if src == netease.SourceID {
		a.openLogin()
		return
	}
	a.openLoginFor(a.source(src))
}

// canDo reports whether the source does what is named. NetEase's always did:
// it is the one whose menus were made for it.
func (a *app) canDo(src, what string) bool {
	if src == netease.SourceID {
		return true
	}
	s := a.source(src)
	return s != nil && s.Can(what)
}

// accountID is who is signed in at the source, to tell them from another, or
// empty.
func (a *app) accountID(src string) string {
	if who := a.accountOf(src); who != nil {
		return who.ID
	}
	return ""
}

// reloadLists asks the source again for the listener's playlists, which an
// edit changed.
func (a *app) reloadLists(src string) {
	if src == netease.SourceID {
		if a.account != nil {
			a.loadMine(a.uid())
		}
		return
	}
	if s := a.source(src); s != nil {
		a.loadPluginLists(s, true)
	}
}

// signInNeeded takes an error of the source, and asks the listener to sign
// in if that is what it says, as a plugin does of a listener it does not know.
func (a *app) signInNeeded(src string, err error) bool {
	if src == netease.SourceID || !errors.Is(err, music.ErrSignIn) {
		return false
	}
	a.signInTo(src)
	return true
}

// pluginByID finds the installed plugin of the ID, or nil.
func (a *app) pluginByID(id string) *plugin.Plugin {
	for _, p := range a.plugins {
		if p.Info().ID == id {
			return p
		}
	}
	return nil
}

// forgetSource drops what the app holds of a source's answers, its account, the
// pages it made and the pages of its songs, albums, artists and shows: the
// source was replaced or taken away, and what it said may be so no longer.
func (a *app) forgetSource(id string) {
	delete(a.states, id)
	for k := range a.settingEdits {
		if strings.HasPrefix(k, id+"\x00") {
			delete(a.settingEdits, k)
		}
	}
	for k := range a.browses {
		if k.source == id {
			delete(a.browses, k)
		}
	}
	for k := range a.playlists {
		if k.source == id {
			delete(a.playlists, k)
		}
	}
	for k := range a.artists {
		if k.source == id {
			delete(a.artists, k)
		}
	}
	for k := range a.albums {
		if k.source == id {
			delete(a.albums, k)
		}
	}
	for k := range a.podcasts {
		if k.source == id {
			delete(a.podcasts, k)
		}
	}
	for r := range a.searches {
		if r.source == id {
			delete(a.searches, r)
		}
	}
	if a.searched.source == id {
		a.searched = route{}
	}
}
