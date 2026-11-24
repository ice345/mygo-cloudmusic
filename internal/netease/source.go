package netease

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/music"
)

// NetEase as a music.Source, the first of the sources docs/plugins.md plans:
// what the player and the views ask of any source, answered from this
// package. It does what the stages so far have moved over (stream and
// lyrics), and says so in Can; the rest is music.Unsupported until its stage.

// SourceID is NetEase's ID among the sources.
const SourceID = "netease"

// Source is NetEase.
type Source struct {
	music.Unsupported
}

var _ music.Source = Source{}
var _ music.Leveler = Source{}

func (Source) Info() music.Info {
	return music.Info{ID: SourceID, Name: "网易云音乐", Playback: music.PlaybackStream}
}

func (Source) Can(what string) bool {
	switch what {
	case "stream", "lyrics", "search", "like", "likeEpisode", "isLiked":
		return true
	}
	return false
}

// Levels are the levels of audio NetEase serves, the worst first.
func (Source) Levels() []string { return Levels }

// TrackID is the ID the app knows a track by: the song's, or, for an episode
// of a podcast, "<audio>@<program>", since its comments, its likes and its
// page are the program's, and its audio is another ID.
func TrackID(t Track) string {
	if t.ProgramID != 0 {
		return fmt.Sprintf("%d@%d", t.ID, t.ProgramID)
	}
	return strconv.FormatInt(t.ID, 10)
}

// parseTrackID reads what TrackID made: the audio's ID, and the program's if
// it is an episode.
func parseTrackID(id string) (audio, program int64, err error) {
	a, p, episode := strings.Cut(id, "@")
	if audio, err = strconv.ParseInt(a, 10, 64); err != nil || audio <= 0 {
		return 0, 0, fmt.Errorf("netease: no such track %q", id)
	}
	if episode {
		if program, err = strconv.ParseInt(p, 10, 64); err != nil || program <= 0 {
			return 0, 0, fmt.Errorf("netease: no such track %q", id)
		}
	}
	return audio, program, nil
}

// sizeTemplate is where a picture of the image CDN says its size goes.
const sizeTemplate = "?param={size}y{size}"

// imageURL is a picture of the image CDN as the app is given it: asking for
// the size it shows it at, by {size}.
func imageURL(pic string) string {
	if pic == "" {
		return ""
	}
	return strings.Replace(pic, "http://", "https://", 1) + sizeTemplate
}

// ToTrack makes the app's track of one of NetEase's, which it keeps whole in
// Extra for FromTrack, so that nothing of it is lost on its way through the
// queue and the session.
func ToTrack(t Track) music.Track {
	mt := music.Track{
		ID:          TrackID(t),
		Source:      SourceID,
		Name:        t.Name,
		Duration:    t.Duration,
		URL:         t.URL(),
		Unavailable: !t.Playable(),
		Alias:       t.Alias,
		Reason:      t.Reason,
		Description: t.Description,
		Album:       music.Album{Name: t.Album.Name},
	}
	if t.Album.ID != 0 {
		mt.Album.ID = strconv.FormatInt(t.Album.ID, 10)
	}
	mt.Album.ImageURL = imageURL(t.Album.PicURL)
	// An episode says which show it is of; one whose show NetEase did not
	// say is of show "0", which opens nothing, rather than a song.
	if t.PodcastID != 0 {
		mt.ShowID = strconv.FormatInt(t.PodcastID, 10)
	} else if t.ProgramID != 0 {
		mt.ShowID = "0"
	}
	// An artist credited by name alone, with no page, has no ID.
	for _, a := range t.Artists {
		ar := music.Artist{Name: a.Name}
		if a.ID != 0 {
			ar.ID = strconv.FormatInt(a.ID, 10)
		}
		mt.Artists = append(mt.Artists, ar)
	}
	mt.Extra, _ = json.Marshal(t)
	return mt
}

// ToTracks is ToTrack of each.
func ToTracks(ts []Track) []music.Track {
	out := make([]music.Track, len(ts))
	for i, t := range ts {
		out[i] = ToTrack(t)
	}
	return out
}

// FromTrack is the NetEase track of one the app has: its own record, whole,
// or, for one that has none, what the app's track says of it.
func FromTrack(mt music.Track) Track {
	var t Track
	if len(mt.Extra) > 0 && json.Unmarshal(mt.Extra, &t) == nil {
		return t
	}
	audio, program, _ := parseTrackID(mt.ID)
	t = Track{ID: audio, ProgramID: program, Name: mt.Name, Duration: mt.Duration, Description: mt.Description}
	t.Album.Name = mt.Album.Name
	t.Album.PicURL = strings.TrimSuffix(mt.Album.ImageURL, sizeTemplate)
	t.Album.ID, _ = strconv.ParseInt(mt.Album.ID, 10, 64)
	t.PodcastID, _ = strconv.ParseInt(mt.ShowID, 10, 64)
	for _, a := range mt.Artists {
		id, _ := strconv.ParseInt(a.ID, 10, 64)
		t.Artists = append(t.Artists, Artist{ID: id, Name: a.Name})
	}
	return t
}

// Stream says where to stream a track from, at level, as SongURL does, with
// its failures as the app reads them: a refusal NetEase says to try again
// after is music.ErrRefused, in NetEase's own words; a song that cannot play
// is music.ErrUnavailable, saying why; anything else, such as the network,
// as it was.
func (Source) Stream(id, level string) (*music.Stream, error) {
	audio, program, err := parseTrackID(id)
	if err != nil {
		return nil, music.Fail(music.ErrUnavailable, err.Error())
	}
	s, err := SongURL(audio, level)
	if err != nil {
		var refused *RefusedError
		switch {
		case errors.As(err, &refused):
			return nil, music.Fail(music.ErrRefused, refused.Message)
		case errors.Is(err, ErrRefused):
			return nil, music.ErrRefused
		case errors.Is(err, ErrUnavailable) && program != 0:
			return nil, music.Fail(music.ErrUnavailable, "这期节目需要购买才能收听")
		case errors.Is(err, ErrUnavailable):
			return nil, music.Fail(music.ErrUnavailable, "这首歌需要 VIP 才能播放")
		}
		return nil, err
	}
	return &music.Stream{URL: s.URL, Type: s.Type, Quality: s.Level, Size: s.Size, Peak: s.Peak, Gain: s.Gain}, nil
}

// Lyrics gives a track's lyrics as lines, built from NetEase's texts as the
// views always built them: word-timed where it has them, with the
// translation and the reading of each.
func (Source) Lyrics(id string) (*music.Lyrics, error) {
	audio, _, err := parseTrackID(id)
	if err != nil {
		return nil, err
	}
	texts, err := Lyrics(audio)
	if err != nil {
		return nil, err
	}
	return lyrics.ToMusic(lyrics.Build(lyrics.Source{
		LRC: texts.LRC, Translation: texts.Translation, Reading: texts.Reading,
		YRC: texts.YRC, YTranslation: texts.YTranslation, YReading: texts.YReading,
	})), nil
}
