package app

import (
	"strconv"
	"testing"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

func newTestApp() *app {
	a := &app{
		playlists: map[pageKey]*music.Playlist{}, loadingPl: map[pageKey]bool{}, plErr: map[pageKey]string{},
		artists: map[pageKey]*artistData{}, albums: map[pageKey]*albumData{}, users: map[int64]*userData{},
		podcasts: map[pageKey]*podcastData{}, podcastTab: &podcastTab{}, searches: map[route]*searchData{},
		lyricAt: -1, queueAt: -1,
		sources: map[string]music.Source{netease.SourceID: netease.Source{}},
	}
	a.art = newArtCache(func(func()) {})
	a.pl = player.New(func(func()) {})
	a.pl.Source = a.source
	a.pl.SetVolume(0.5)
	return a
}

// cue makes tracks the player's queue, in order, with the first current.
func (a *app) cue(tracks []netease.Track) { a.pl.Cue(netease.ToTracks(tracks)) }

// loadedPlaylist is a NetEase playlist as the playlist page holds it once
// loaded.
func loadedPlaylist(pl netease.Playlist) *music.Playlist {
	mp := netease.ToPlaylist(pl)
	return &mp
}

// editSource is a source whose playlist edits are what a test says; an edit it
// was not told of fails.
type editSource struct {
	music.Unsupported
	t      *testing.T
	create func(name string) (*music.Playlist, error)
	rename func(id, name string) error
	del    func(id string) error
	add    func(id string, tracks []string) error
	remove func(id string, tracks []string) error
	save   func(id string, on bool) error
}

func (*editSource) Info() music.Info { return music.Info{ID: netease.SourceID, Name: "编辑音源"} }
func (s *editSource) CreatePlaylist(name string) (*music.Playlist, error) {
	if s.create == nil {
		s.t.Error("made a playlist")
		return nil, music.ErrUnsupported
	}
	return s.create(name)
}
func (s *editSource) RenamePlaylist(id, name string) error {
	if s.rename == nil {
		s.t.Error("renamed a playlist")
		return music.ErrUnsupported
	}
	return s.rename(id, name)
}
func (s *editSource) DeletePlaylist(id string) error {
	if s.del == nil {
		s.t.Error("deleted a playlist")
		return music.ErrUnsupported
	}
	return s.del(id)
}
func (s *editSource) AddToPlaylist(id string, tracks []string) error {
	if s.add == nil {
		s.t.Error("added to a playlist")
		return music.ErrUnsupported
	}
	return s.add(id, tracks)
}
func (s *editSource) RemoveFromPlaylist(id string, tracks []string) error {
	if s.remove == nil {
		s.t.Error("removed from a playlist")
		return music.ErrUnsupported
	}
	return s.remove(id, tracks)
}
func (s *editSource) SavePlaylist(id string, on bool) error {
	if s.save == nil {
		s.t.Error("saved a playlist")
		return music.ErrUnsupported
	}
	return s.save(id, on)
}

// edits makes a's playlist edits those of a fresh editSource, which the test
// sets what it expects of.
func edits(t *testing.T, a *app) *editSource {
	src := &editSource{t: t}
	a.sources[netease.SourceID] = src
	return src
}

// loadedAlbum is a NetEase album as the album page holds it once loaded.
func loadedAlbum(al netease.AlbumProfile) *music.AlbumPage {
	return &music.AlbumPage{Album: netease.ToAlbum(al), Tracks: netease.ToTracks(al.Songs)}
}

// loadedArtist is a NetEase artist as the artist page holds it once loaded.
func loadedArtist(ar netease.ArtistProfile) *artistData {
	return &artistData{page: &music.ArtistPage{Artist: netease.ToArtist(ar), Tracks: netease.ToTracks(ar.Songs)}, rec: ar}
}

// mineLists are NetEase playlists as the sidebar lists the listener's own.
func mineLists(pls ...netease.Playlist) []music.Playlist {
	var out []music.Playlist
	for _, pl := range pls {
		mp := netease.ToPlaylist(pl)
		mp.Mine = true
		out = append(out, mp)
	}
	return out
}

// savedLists are NetEase playlists as the sidebar lists those the listener saved.
func savedLists(pls ...netease.Playlist) []music.Playlist {
	var out []music.Playlist
	for _, pl := range pls {
		mp := netease.ToPlaylist(pl)
		mp.Saved = true
		out = append(out, mp)
	}
	return out
}

// ntrack names a NetEase song or episode in the comments, as the app keeps them.
func ntrack(n int64) trackKey { return trackKey{netease.SourceID, strconv.FormatInt(n, 10)} }

// commentSource is a source whose comments, and likes of them, are what a test
// says; one it was not told of is unsupported.
type commentSource struct {
	music.Unsupported
	comments func(trackID string, offset int) (*music.CommentPage, error)
	like     func(trackID, commentID string, on bool) error
}

func (*commentSource) Info() music.Info {
	return music.Info{ID: netease.SourceID, Name: "评论音源"}
}
func (s *commentSource) Can(what string) bool {
	return what == "comments" && s.comments != nil || what == "likeComment" && s.like != nil
}
func (s *commentSource) Comments(id string, offset int) (*music.CommentPage, error) {
	if s.comments == nil {
		return nil, music.ErrUnsupported
	}
	return s.comments(id, offset)
}
func (s *commentSource) LikeComment(id, comment string, on bool) error {
	if s.like == nil {
		return music.ErrUnsupported
	}
	return s.like(id, comment, on)
}

// loadedPodcast is a NetEase podcast and its episodes as the podcast page holds
// them once they have loaded.
func loadedPodcast(pod netease.Podcast, programs ...netease.Program) *podcastData {
	show := netease.ToShow(pod)
	d := &podcastData{show: &show, page: paged{loaded: true}}
	for _, p := range programs {
		d.episodes = append(d.episodes, netease.ToEpisode(p, &pod))
	}
	return d
}

// picksSource is a source with podcasts to pick from, and those the listener
// subscribes to, as a test says.
type picksSource struct {
	music.Unsupported
	subs  []music.Show
	picks func() ([]music.Show, error)
	hot   func(offset int) ([]music.Show, bool, error)
}

func (*picksSource) Info() music.Info { return music.Info{ID: netease.SourceID, Name: "播客音源"} }
func (*picksSource) Can(what string) bool {
	return what == "shows"
}
func (s *picksSource) Shows() ([]music.Show, error)        { return s.subs, nil }
func (s *picksSource) PodcastPicks() ([]music.Show, error) { return s.picks() }
func (s *picksSource) HotPodcasts(offset int) ([]music.Show, bool, error) {
	return s.hot(offset)
}
