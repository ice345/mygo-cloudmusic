package netease

import (
	"errors"
	"testing"
)

func TestPlayableFollowsPrivilege(t *testing.T) {
	tracks := []Track{{ID: 1, Fee: 1}, {ID: 2, Fee: 8}, {ID: 3, Fee: 8}, {ID: 4, Fee: 1}}
	if tracks[0].Playable() || !tracks[1].Playable() {
		t.Fatal("without privileges, VIP songs do not play and free ones do")
	}
	withPrivileges(tracks, []Privilege{
		{ID: 1, Pl: 320000},           // VIP song, the listener subscribes
		{ID: 2, Pl: 0},                // nothing to play
		{ID: 3, St: -200, Pl: 128000}, // taken down
	})
	want := []bool{true, false, false, false}
	for i, tr := range tracks {
		if tr.Playable() != want[i] {
			t.Errorf("track %d: playable %v", tr.ID, tr.Playable())
		}
	}
}

// The desktop client writes a credit as JSON. It is a timed line, like the web's.
func TestPlainLRCReadsCreditLines(t *testing.T) {
	in := "{\"t\":675,\"c\":[{\"tx\":\"作曲: \"},{\"tx\":\"某人\"}]}\n[00:21.96]一句\n{\"t\":1,\"c\":[{}]}\n"
	got := plainLRC(in)
	want := "[00:00.675]作曲: 某人\n[00:21.96]一句\n"
	if got != want {
		t.Fatalf("%q", got)
	}
}

func swapLyrics(desk, web func(int64) (LyricTexts, error)) func() {
	prevD, prevW := desktopLyrics, webLyrics
	desktopLyrics, webLyrics = desk, web
	return func() { desktopLyrics, webLyrics = prevD, prevW }
}

// The desktop client's answer carries every text, so the web endpoint is not asked.
func TestLyricsUsesTheDesktopAnswer(t *testing.T) {
	asked := false
	defer swapLyrics(
		func(int64) (LyricTexts, error) {
			return LyricTexts{LRC: "l", Translation: "t", Reading: "r", YRC: "y", YTranslation: "yt", YReading: "yr"}, nil
		},
		func(int64) (LyricTexts, error) {
			asked = true
			return LyricTexts{LRC: "web"}, nil
		},
	)()
	got, err := Lyrics(1)
	if err != nil || asked || got != (LyricTexts{LRC: "l", Translation: "t", Reading: "r", YRC: "y", YTranslation: "yt", YReading: "yr"}) {
		t.Fatalf("asked web %v, %+v, %v", asked, got, err)
	}
}

// A desktop call that fails leaves the web's timed lines, with no word-timed text.
func TestLyricsFallsBackWhenTheDesktopCallFails(t *testing.T) {
	defer swapLyrics(
		func(int64) (LyricTexts, error) { return LyricTexts{YRC: "y"}, errors.New("down") },
		func(int64) (LyricTexts, error) {
			return LyricTexts{LRC: "l", Translation: "t", Reading: "r"}, nil
		},
	)()
	got, err := Lyrics(1)
	if err != nil || got != (LyricTexts{LRC: "l", Translation: "t", Reading: "r"}) {
		t.Fatalf("%+v, %v", got, err)
	}
}

// A desktop answer with no timed lines keeps its word-timed text and takes the lines from the web.
func TestLyricsFallsBackWhenTheDesktopCallHasNoLines(t *testing.T) {
	defer swapLyrics(
		func(int64) (LyricTexts, error) {
			return LyricTexts{LRC: " \n", YRC: "y", YTranslation: "yt", YReading: "yr", Reading: "desk"}, nil
		},
		func(int64) (LyricTexts, error) {
			return LyricTexts{LRC: "l", Translation: "t"}, nil
		},
	)()
	got, err := Lyrics(1)
	want := LyricTexts{LRC: "l", Translation: "t", Reading: "desk", YRC: "y", YTranslation: "yt", YReading: "yr"}
	if err != nil || got != want {
		t.Fatalf("%+v, %v", got, err)
	}
}

// Both calls failing reports the desktop client's error, which was asked first.
func TestLyricsReportsTheDesktopError(t *testing.T) {
	defer swapLyrics(
		func(int64) (LyricTexts, error) { return LyricTexts{}, errors.New("down") },
		func(int64) (LyricTexts, error) { return LyricTexts{}, errors.New("web down") },
	)()
	if _, err := Lyrics(1); err == nil || err.Error() != "down" {
		t.Fatalf("%v", err)
	}
}
