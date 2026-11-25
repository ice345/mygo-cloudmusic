package app

import (
	"errors"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
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
	go src.SignOut()
}
