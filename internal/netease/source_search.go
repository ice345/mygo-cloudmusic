package netease

import (
	"strconv"
	"time"

	"netease-music-mygo/internal/music"
)

// searchPageSize is how many results a page of a search holds.
const searchPageSize = 60

// Search looks for what query names, of a kind, from offset: a page of
// searchPageSize. NetEase also searches for users, which no other source has:
// that is SearchPage's.
func (Source) Search(query string, kind music.SearchKind, offset int) (*music.SearchResult, error) {
	var of SearchType
	switch kind {
	case music.SearchTracks:
		of = SearchSongs
	case music.SearchAlbums:
		of = SearchAlbums
	case music.SearchArtists:
		of = SearchArtists
	case music.SearchPlaylists:
		of = SearchPlaylists
	case music.SearchShows:
		of = SearchPodcasts
	default:
		return nil, music.ErrUnsupported
	}
	res, err := SearchPage(query, of, searchPageSize, offset)
	if err != nil {
		return nil, err
	}
	out := &music.SearchResult{
		Tracks:  ToTracks(res.Songs),
		Albums:  ToAlbums(res.Albums),
		Artists: ToArtists(res.Artists),
		Shows:   ToShows(res.Podcasts),
		Total:   res.Total,
		// A server can claim more while sending none.
		More: res.Len() > 0 && offset+res.Len() < res.Total,
	}
	for i := range res.Playlists {
		out.Playlists = append(out.Playlists, ToPlaylist(res.Playlists[i]))
	}
	return out, nil
}

// ToArtist is an artist as the app has them: their square photo, or else the
// wide one.
func ToArtist(a ArtistProfile) music.Artist {
	pic := a.Avatar
	if pic == "" {
		pic = a.PicURL
	}
	return music.Artist{ID: strconv.FormatInt(a.ID, 10), Name: a.Name, ImageURL: imageURL(pic), Following: a.Followed}
}

// ToArtists is ToArtist of each.
func ToArtists(as []ArtistProfile) []music.Artist {
	var out []music.Artist
	for _, a := range as {
		out = append(out, ToArtist(a))
	}
	return out
}

// ToAlbum is an album as the app has it, released as the date NetEase gives.
func ToAlbum(al AlbumProfile) music.Album {
	out := music.Album{ID: strconv.FormatInt(al.ID, 10), Name: al.Name, ImageURL: imageURL(al.PicURL)}
	for _, a := range al.Artists {
		out.Artists = append(out.Artists, music.Artist{ID: strconv.FormatInt(a.ID, 10), Name: a.Name})
	}
	if al.PublishTime > 0 {
		out.Released = time.UnixMilli(al.PublishTime).Format("2006-01-02")
	}
	return out
}

// ToAlbums is ToAlbum of each.
func ToAlbums(als []AlbumProfile) []music.Album {
	var out []music.Album
	for _, al := range als {
		out = append(out, ToAlbum(al))
	}
	return out
}

// ToPlaylist is a playlist as the app has it, without its tracks.
func ToPlaylist(pl Playlist) music.Playlist {
	return music.Playlist{
		ID: strconv.FormatInt(pl.ID, 10), Name: pl.Name, ImageURL: imageURL(pl.Cover()),
		Description: pl.Description, Owner: pl.Creator.Nickname,
		TrackCount: pl.TrackCount, PlayCount: int64(pl.PlayCount),
	}
}

// ToShow is a podcast as the app has it.
func ToShow(p Podcast) music.Show {
	return music.Show{
		ID: strconv.FormatInt(p.ID, 10), Name: p.Name, ImageURL: imageURL(p.PicURL),
		Description: p.Description, Publisher: p.DJ.Nickname, EpisodeCount: p.ProgramCount, Subscribed: p.Subed,
	}
}

// ToShows is ToShow of each.
func ToShows(ps []Podcast) []music.Show {
	var out []music.Show
	for _, p := range ps {
		out = append(out, ToShow(p))
	}
	return out
}
