package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

// Where playback was left is kept in session.json in the user data
// directory: on quitting, and every little while as it plays, in case the
// app does not get to quit.

func sessionPath() (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "session.json"), nil
}

// saveSession keeps where playback is, if it moved on since last kept.
func (a *app) saveSession() {
	s, ok := a.pl.Session()
	if !ok {
		return
	}
	b, err := json.Marshal(s)
	if err != nil || string(b) == a.savedSession {
		return
	}
	path, err := sessionPath()
	if err != nil {
		return
	}
	if os.WriteFile(path+".part", b, 0o600) == nil && os.Rename(path+".part", path) == nil {
		a.savedSession = string(b)
	}
}

// sessionEvery is how long after playback changed, and how often as it
// plays, the session is kept.
const sessionEvery = 30 * time.Second

// keepSession saves the session a while after playback changed, and every
// while again as it plays. Paused, nothing is due, and the app sleeps.
func (a *app) keepSession() {
	if a.sessionDue {
		return
	}
	a.sessionDue = true
	sessionAfter(sessionEvery, func() {
		a.sessionDue = false
		a.saveSession()
		if a.pl.Playing() {
			a.keepSession()
		}
	})
}

// sessionAfter runs fn on the main thread after d, without redrawing a
// window as Update would.
var sessionAfter = func(d time.Duration, fn func()) {
	time.AfterFunc(d, func() { mygo.RunOnMain(fn) })
}

// decodeSession reads a session as it was kept, whichever release kept it: its
// tracks are in the shape of the day, with a source, or NetEase's own, as
// they were before there were sources, which are made the app's tracks as
// they come. The next time it is kept, it is kept in the new shape.
func decodeSession(b []byte) (player.Session, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil {
		return player.Session{}, false
	}
	var raws []json.RawMessage
	if json.Unmarshal(fields["queue"], &raws) != nil {
		return player.Session{}, false
	}
	delete(fields, "queue")
	rest, _ := json.Marshal(fields)
	var s player.Session
	if json.Unmarshal(rest, &s) != nil {
		return player.Session{}, false
	}
	for _, raw := range raws {
		var probe struct {
			Source string `json:"source"`
		}
		json.Unmarshal(raw, &probe)
		if probe.Source != "" {
			var mt music.Track
			if json.Unmarshal(raw, &mt) != nil {
				return player.Session{}, false
			}
			s.Queue = append(s.Queue, mt)
			continue
		}
		var nt netease.Track
		if json.Unmarshal(raw, &nt) != nil {
			return player.Session{}, false
		}
		s.Queue = append(s.Queue, netease.ToTrack(nt))
	}
	return s, true
}

// restoreSession picks up playback where it was left, paused, once the
// listener's session is back: the songs they may play depend on it.
func (a *app) restoreSession() {
	if a.restored {
		return
	}
	a.restored = true
	path, err := sessionPath()
	if err != nil {
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	s, ok := decodeSession(b)
	if !ok {
		return
	}
	a.savedSession = string(b)
	a.pl.Restore(s)
	if s.Radio != "" && a.current() != nil {
		a.attachRadio(s.Radio, a.radioFeed(s.Radio))
	}
}
