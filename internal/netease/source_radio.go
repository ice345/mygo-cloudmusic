package netease

import (
	"fmt"
	"strconv"

	"netease-music-mygo/internal/music"
)

// fmStations are the modes of the personal FM, as stations.
var fmStations = []music.Station{
	{ID: FMDefault, Name: "推荐", Description: "按你的口味推荐"},
	{ID: FMFamiliar, Name: "熟悉", Description: "与你熟悉的歌相似的歌"},
	{ID: FMExplore, Name: "探索", Description: "你还没听过的歌"},
}

// Stations lists the modes of the personal FM, which plays on without end.
func (Source) Stations() ([]music.Station, error) {
	return append([]music.Station(nil), fmStations...), nil
}

// StationNext answers the next few songs of the personal FM in a mode, those
// the listener may play and not.
func (Source) StationNext(id string) ([]music.Track, error) {
	known := false
	for _, st := range fmStations {
		known = known || st.ID == id
	}
	if !known {
		return nil, fmt.Errorf("netease: no such station %q", id)
	}
	tracks, err := PersonalFM(id)
	if err != nil {
		return nil, fmt.Errorf("netease: %w", err)
	}
	return ToTracks(tracks), nil
}

// HeartMode answers songs like last, in the heart mode that starts from the
// song start of the playlist: NetEase's own, which no other source has.
func (Source) HeartMode(last, start, playlist string) ([]music.Track, error) {
	l, _, err := parseTrackID(last)
	if err != nil {
		return nil, err
	}
	s, _, err := parseTrackID(start)
	if err != nil {
		return nil, err
	}
	pid, err := strconv.ParseInt(playlist, 10, 64)
	if err != nil || pid <= 0 {
		return nil, fmt.Errorf("netease: no such playlist %q", playlist)
	}
	tracks, err := HeartMode(l, s, pid)
	if err != nil {
		return nil, fmt.Errorf("netease: %w", err)
	}
	return ToTracks(tracks), nil
}

// TrashFM tells the personal FM the listener does not like a song, which it
// plays no more.
func (Source) TrashFM(id string) error {
	audio, _, err := parseTrackID(id)
	if err != nil {
		return err
	}
	return FMTrash(audio)
}
