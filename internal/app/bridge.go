package app

import (
	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
)

// The player speaks music.Track, whichever source a track is of; most views
// still hold netease.Track, until each is moved over (docs/plugins.md, stage
// 1.3). These carry one into the other at the player's edge, and shrink as
// views move: a track's NetEase record travels whole in its Extra, so nothing
// is lost either way.

// source finds a source by its ID: NetEase, so far.
func (a *app) source(id string) music.Source {
	if id == netease.SourceID {
		return netease.Source{}
	}
	return nil
}

// trackKey names a track among all sources'.
type trackKey struct{ source, id string }

// fromMusic is the NetEase track of one the player has, decoded from its
// record once, however often a view asks: they ask every frame.
func (a *app) fromMusic(mt music.Track) *netease.Track {
	k := trackKey{mt.Source, mt.ID}
	if t, ok := a.neteaseTracks[k]; ok {
		return t
	}
	if a.neteaseTracks == nil || len(a.neteaseTracks) >= 4096 {
		a.neteaseTracks = make(map[trackKey]*netease.Track)
	}
	t := netease.FromTrack(mt)
	a.neteaseTracks[k] = &t
	return &t
}

// current is the track playing or loading, or nil.
func (a *app) current() *netease.Track {
	if mt := a.pl.Current(); mt != nil {
		return a.fromMusic(*mt)
	}
	return nil
}

// queued is the tracks queued, in the order they were given.
func (a *app) queued() []netease.Track {
	q := a.pl.Queue()
	out := make([]netease.Track, len(q))
	for i := range q {
		out[i] = *a.fromMusic(q[i])
	}
	return out
}

// playQueue replaces the queue with tracks and plays the one at start.
func (a *app) playQueue(tracks []netease.Track, start int) {
	a.pl.PlayQueue(netease.ToTracks(tracks), start)
}

// feedOf is a radio's feed, as the player asks it.
func feedOf(feed func(got func([]netease.Track))) func(got func([]music.Track)) {
	if feed == nil {
		return nil
	}
	return func(got func([]music.Track)) {
		feed(func(ts []netease.Track) { got(netease.ToTracks(ts)) })
	}
}

func (a *app) playRadio(name string, tracks []netease.Track, feed func(got func([]netease.Track))) {
	a.pl.PlayRadio(name, netease.ToTracks(tracks), feedOf(feed))
}

func (a *app) radioFrom(name string, tracks []netease.Track, feed func(got func([]netease.Track))) {
	a.pl.RadioFrom(name, netease.ToTracks(tracks), feedOf(feed))
}

func (a *app) attachRadio(name string, feed func(got func([]netease.Track))) {
	a.pl.AttachRadio(name, feedOf(feed))
}
