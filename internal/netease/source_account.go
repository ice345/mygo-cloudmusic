package netease

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"netease-music-mygo/internal/music"
)

// Who is signed in with NetEase, and how: the session is the package's, as the
// cookie jar is, so what a sign-in is doing is too.
var acct struct {
	mu     sync.Mutex
	show   func(music.SignInPrompt)
	cancel chan struct{} // closed to give up the sign-in under way
	me     *Profile      // who it is, once known
}

// signInEvery is how often a sign-in asks whether its code was scanned.
var signInEvery = 2 * time.Second

// restoreSession, saveSession, clearSession and logout touch the keychain and
// NetEase's logout; tests replace them.
var (
	restoreSession = RestoreSession
	saveSession    = SaveSession
	clearSession   = ClearSession
	logout         = Logout

	restoreOnce sync.Once
	restored    bool
)

func toAccount(p *Profile) *music.Account {
	return &music.Account{ID: strconv.FormatInt(p.UserID, 10), Name: p.Nickname, ImageURL: p.AvatarURL}
}

// Account says who is signed in, or nil. The first time it is asked it brings
// the session back from the keychain, and forgets one that has run out.
func (Source) Account() (*music.Account, error) {
	first := false
	restoreOnce.Do(func() { first, restored = true, restoreSession() })
	p, err := Account()
	if err != nil {
		return nil, err
	}
	acct.mu.Lock()
	acct.me = p
	acct.mu.Unlock()
	if p == nil {
		if first && restored {
			clearSession() // expired
		}
		return nil, nil
	}
	return toAccount(p), nil
}

// OnSignInPrompt says where to show the code of a sign-in, and who scanned it.
func (Source) OnSignInPrompt(show func(music.SignInPrompt)) {
	acct.mu.Lock()
	acct.show = show
	acct.mu.Unlock()
}

// CancelSignIn gives up the sign-in under way, if any.
func (Source) CancelSignIn() {
	acct.mu.Lock()
	defer acct.mu.Unlock()
	if acct.cancel != nil {
		close(acct.cancel)
		acct.cancel = nil
	}
}

// SignIn signs in with a QR code to scan with NetEase's app on a phone: it
// shows the code, shows who scanned it, and returns once they confirm. A code
// that runs out is music.ErrExpired; one NetEase refuses is music.ErrSignIn in
// its own words; a network that fails is tried again, as it may come back.
func (Source) SignIn() (*music.Account, error) {
	key, err := QRKey()
	if err != nil {
		return nil, err
	}
	cancel := make(chan struct{})
	acct.mu.Lock()
	if acct.cancel != nil {
		close(acct.cancel) // one at a time
	}
	acct.cancel = cancel
	show := acct.show
	acct.mu.Unlock()
	defer func() {
		acct.mu.Lock()
		if acct.cancel == cancel {
			acct.cancel = nil
		}
		acct.mu.Unlock()
	}()
	prompt := func(p music.SignInPrompt) {
		if show != nil {
			show(p)
		}
	}
	code := LoginURL(key)
	prompt(music.SignInPrompt{QR: code, Text: "打开网易云音乐 App，扫一扫登录"})
	for {
		select {
		case <-cancel:
			return nil, context.Canceled
		case <-time.After(signInEvery):
		}
		state, msg, name, avatar, err := QRCheck(key)
		switch {
		case err != nil:
			// Try again: the network may come back.
		case state == QRConfirmed:
			saveSession()
			p, err := Account()
			if err != nil {
				return nil, err
			}
			if p == nil {
				return nil, music.Fail(music.ErrSignIn, "")
			}
			acct.mu.Lock()
			acct.me = p
			acct.mu.Unlock()
			return toAccount(p), nil
		case state == QRExpired:
			return nil, music.ErrExpired
		case state == QRScanned:
			prompt(music.SignInPrompt{QR: code, Text: "请在手机上确认登录", Name: name, ImageURL: avatar})
		case state == QRWaiting:
		default:
			return nil, music.Fail(music.ErrSignIn, msg)
		}
	}
}

// SignOut ends the session, here and with NetEase.
func (Source) SignOut() error {
	logout()
	acct.mu.Lock()
	acct.me = nil
	acct.mu.Unlock()
	return nil
}

// Playlists lists the listener's own, the liked songs' first, and those they
// saved.
func (s Source) Playlists() ([]music.Playlist, error) {
	acct.mu.Lock()
	me := acct.me
	acct.mu.Unlock()
	if me == nil {
		if _, err := s.Account(); err != nil {
			return nil, err
		}
		acct.mu.Lock()
		me = acct.me
		acct.mu.Unlock()
	}
	if me == nil {
		return nil, music.Fail(music.ErrSignIn, "")
	}
	made, saved, err := UserPlaylists(me.UserID)
	if err != nil {
		return nil, fmt.Errorf("netease: %w", err)
	}
	var out []music.Playlist
	for _, pl := range made {
		mp := ToPlaylist(pl)
		mp.Mine = true
		out = append(out, mp)
	}
	for _, pl := range saved {
		out = append(out, ToPlaylist(pl))
	}
	return out, nil
}
