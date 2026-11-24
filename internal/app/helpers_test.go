package app

import (
	"testing"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

func newTestApp() *app {
	a := &app{
		playlists: map[pageKey]*music.Playlist{}, loadingPl: map[pageKey]bool{}, plErr: map[pageKey]string{},
		artists: map[int64]*artistData{}, albums: map[int64]*albumData{}, users: map[int64]*userData{},
		podcasts: map[int64]*podcastData{}, podcastTab: &podcastTab{}, searches: map[route]*searchData{},
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
