package netease

import (
	"errors"
	"fmt"
	"strconv"

	"netease-music-mygo/internal/music"
)

// number reads a playlist's, album's or artist's ID, which is NetEase's number.
func number(id string) (int64, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("netease: no such page %q", id)
	}
	return n, nil
}

// Playlist is a playlist with its songs.
func (Source) Playlist(id string) (*music.Playlist, error) {
	n, err := number(id)
	if err != nil {
		return nil, err
	}
	pl, err := PlaylistDetail(n)
	if err != nil {
		return nil, err
	}
	out := ToPlaylist(*pl)
	return &out, nil
}

// Album is an album with its songs.
func (Source) Album(id string) (*music.AlbumPage, error) {
	n, err := number(id)
	if err != nil {
		return nil, err
	}
	al, err := AlbumDetail(n)
	if err != nil {
		return nil, err
	}
	return &music.AlbumPage{Album: ToAlbum(*al), Tracks: ToTracks(al.Songs)}, nil
}

// Artist is an artist with their most popular songs. Their newest songs,
// their albums, which come in pages, and their events are NetEase's own.
func (Source) Artist(id string) (*music.ArtistPage, error) {
	n, err := number(id)
	if err != nil {
		return nil, err
	}
	ar, err := ArtistDetail(n)
	if err != nil {
		return nil, err
	}
	return &music.ArtistPage{Artist: ToArtist(*ar), Tracks: ToTracks(ar.Songs)}, nil
}

// CreatePlaylist makes a playlist, which is empty.
func (Source) CreatePlaylist(name string) (*music.Playlist, error) {
	pl, err := CreatePlaylist(name)
	if err != nil {
		return nil, err
	}
	out := ToPlaylist(*pl)
	return &out, nil
}

func (Source) RenamePlaylist(id, name string) error {
	n, err := number(id)
	if err != nil {
		return err
	}
	return RenamePlaylist(n, name)
}

func (Source) DeletePlaylist(id string) error {
	n, err := number(id)
	if err != nil {
		return err
	}
	return DeletePlaylist(n)
}

// songNumbers reads the IDs of songs, which an episode is not.
func songNumbers(trackIDs []string) ([]int64, error) {
	out := make([]int64, len(trackIDs))
	for i, id := range trackIDs {
		audio, program, err := parseTrackID(id)
		if err != nil {
			return nil, err
		}
		if program != 0 {
			return nil, music.ErrUnsupported
		}
		out[i] = audio
	}
	return out, nil
}

// AddToPlaylist adds songs to a playlist; a song that is in it already is
// music.ErrDuplicate.
func (Source) AddToPlaylist(id string, trackIDs []string) error {
	n, err := number(id)
	if err != nil {
		return err
	}
	songs, err := songNumbers(trackIDs)
	if err != nil {
		return err
	}
	if err := AddToPlaylist(n, songs...); errors.Is(err, ErrInPlaylist) {
		return music.ErrDuplicate
	} else if err != nil {
		return err
	}
	return nil
}

func (Source) RemoveFromPlaylist(id string, trackIDs []string) error {
	n, err := number(id)
	if err != nil {
		return err
	}
	songs, err := songNumbers(trackIDs)
	if err != nil {
		return err
	}
	return RemoveFromPlaylist(n, songs...)
}

// SavePlaylist saves someone else's playlist for the listener, or lets it go.
func (Source) SavePlaylist(id string, on bool) error {
	n, err := number(id)
	if err != nil {
		return err
	}
	return SubscribePlaylist(n, on)
}

// FollowArtist follows an artist, or stops.
func (Source) FollowArtist(id string, on bool) error {
	n, err := number(id)
	if err != nil {
		return err
	}
	return FollowArtist(n, on)
}
