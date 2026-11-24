package app

import (
	"strconv"
	"strings"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
)

// The player speaks music.Track, whichever source a track is of; most views
// still hold netease.Track, until each is moved over (docs/plugins.md, stage
// 1.3). These carry one into the other at the player's edge, and shrink as
// views move: a track's NetEase record travels whole in its Extra, so nothing
// is lost either way.

// source finds a source by its ID, or nil.
func (a *app) source(id string) music.Source { return a.sources[id] }

// searchSource is the source the search field searches: NetEase, until the
// listener can switch between sources.
func (a *app) searchSource() music.Source { return a.source(netease.SourceID) }

// nroute is the page of kind of the NetEase item numbered id.
func nroute(page pageKind, id int64) route {
	return route{page: page, source: netease.SourceID, id: strconv.FormatInt(id, 10)}
}

// number is the NetEase number of the item the page shows, or 0: the pages
// load their data by it, as only NetEase's can be loaded so far.
func (r route) number() int64 {
	if r.source != netease.SourceID {
		return 0
	}
	return numericID(r.id)
}

// numericID is the NetEase ID a source's ID of a page names, or 0: pages are
// still opened by NetEase's numeric IDs, until the views that open them move
// over (docs/plugins.md, stage 1.3).
func numericID(id string) int64 {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// sizedImage is the URL to fetch the picture at url with, to show it px
// pixels across: one a source gives, which says where its size goes, or
// NetEase's, as the views that have not moved over hold them.
func sizedImage(url string, px int) string {
	if strings.Contains(url, "{size}") || strings.Contains(url, "%7Bsize%7D") {
		return music.SizedImage(url, px)
	}
	return netease.Sized(url, px)
}

// musicOf is a NetEase track as the app's, for the views that have not moved
// over to call what has. It costs about two microseconds.
func musicOf(tr *netease.Track) music.Track { return netease.ToTrack(*tr) }

// trackList is the tracks a list shows: those a source gave, or those of a
// page that still holds NetEase's, which are made the app's one row at a time
// as the list builds them, and all at once only to play them.
type trackList struct {
	n   int
	at  func(i int) music.Track
	all func() []music.Track
}

// listOf is a list of the tracks a source gave.
func listOf(tracks []music.Track) trackList {
	return trackList{len(tracks), func(i int) music.Track { return tracks[i] }, func() []music.Track { return tracks }}
}

// neteaseList is a list of NetEase's tracks.
func neteaseList(tracks []netease.Track) trackList {
	return trackList{
		len(tracks),
		func(i int) music.Track { return netease.ToTrack(tracks[i]) },
		func() []music.Track { return netease.ToTracks(tracks) },
	}
}

// pageKey names what a page shows among all sources': the playlist, album or
// artist with the ID id at the source.
type pageKey struct{ source, id string }

// nkey is the key of the NetEase item numbered n.
func nkey(n int64) pageKey { return pageKey{netease.SourceID, strconv.FormatInt(n, 10)} }

// key is the key of what the page of r shows.
func (r route) key() pageKey { return pageKey{r.source, r.id} }

// playlistRecord is the NetEase record of a loaded playlist, for what only
// NetEase says of one; decoded each time, which costs a few microseconds.
func playlistRecord(pl *music.Playlist) netease.Playlist { return netease.PlaylistRecord(*pl) }

// firstAvailable is where a list of tracks is first played from: its first
// track the listener can play, or its first.
func firstAvailable(tracks []music.Track) int {
	for i := range tracks {
		if !tracks[i].Unavailable {
			return i
		}
	}
	return 0
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
