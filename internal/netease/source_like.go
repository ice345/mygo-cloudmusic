package netease

import (
	"strconv"
	"strings"

	"netease-music-mygo/internal/music"
)

// Like likes a song, or unlikes it. An episode is liked through its program,
// by LikeEpisode, and never as the song its audio is.
func (Source) Like(id string, on bool) error {
	audio, program, err := parseTrackID(id)
	if err != nil {
		return err
	}
	if program != 0 {
		return music.ErrUnsupported
	}
	return Like(audio, on)
}

// LikeEpisode likes an episode of a podcast, or unlikes it.
func (Source) LikeEpisode(id string, on bool) error {
	_, program, err := parseTrackID(id)
	if err != nil {
		return err
	}
	if program == 0 {
		return music.ErrUnsupported
	}
	return LikeProgram(program, on)
}

// IsLiked says which of the tracks the listener likes: songs by the songs
// they have liked, episodes by asking after each. Songs need them signed in.
func (Source) IsLiked(ids []string) (map[string]bool, error) {
	out := make(map[string]bool, len(ids))
	var songs []string
	var programs []int64
	programOf := map[int64]string{}
	for _, id := range ids {
		_, program, err := parseTrackID(id)
		if err != nil {
			continue
		}
		if program == 0 {
			songs = append(songs, id)
			continue
		}
		programs = append(programs, program)
		programOf[program] = id
	}
	if len(songs) > 0 {
		me, err := Account()
		if err != nil {
			return nil, err
		}
		if me == nil {
			return nil, music.Fail(music.ErrSignIn, "")
		}
		liked, err := LikedIDs(me.UserID)
		if err != nil {
			return nil, err
		}
		set := make(map[string]bool, len(liked))
		for _, l := range liked {
			set[strconv.FormatInt(l, 10)] = true
		}
		for _, id := range songs {
			out[id] = set[strings.TrimSpace(id)]
		}
	}
	if len(programs) > 0 {
		liked, err := ProgramsLiked(programs)
		if err != nil {
			return nil, err
		}
		for _, p := range programs {
			out[programOf[p]] = liked[p]
		}
	}
	return out, nil
}

// LikedSongIDs lists the songs the listener likes, all at once, in no order: the
// hearts of a page need not be asked after one by one.
func (Source) LikedSongIDs() ([]string, error) {
	me, err := Account()
	if err != nil {
		return nil, err
	}
	if me == nil {
		return nil, music.Fail(music.ErrSignIn, "")
	}
	liked, err := LikedIDs(me.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(liked))
	for i, l := range liked {
		out[i] = strconv.FormatInt(l, 10)
	}
	return out, nil
}
