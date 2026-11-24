package app

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// 翻译 starts on and 音译 starts off. Turning 音译 on shows a reading the
// song has, and turning 翻译 off takes that line away. A line with no
// reading gains none. The menu names this song's reading 罗马音.
func TestLyricScript(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	tr := netease.Track{ID: 1, Name: "風", Artists: []netease.Artist{{Name: "誰"}}, Duration: 60000}
	a.cue([]netease.Track{tr})
	// Two lines carry kana, so the song is Japanese and the reading is 罗马音.
	a.pl.SetLyrics(lyrics.Build(lyrics.Source{
		LRC:         "[00:10.00]風が吹く\n[00:15.00]また\n[00:20.00]I love you\n[00:30.00]รักนะ",
		Translation: "[00:10.00]风在吹",
		Reading:     "[00:10.00]kaze ga fuku\n[00:20.00]I love you",
	}))
	a.nowPlaying, a.npPane = true, paneLyrics
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if !tt.HasText("风在吹") {
		t.Fatal("the translation is not shown")
	}
	if tt.HasText("kaze ga fuku") {
		t.Fatal("the reading is shown before it is asked for")
	}
	if err := tt.Click("歌词显示"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("音译（罗马音）"); err != nil {
		t.Fatalf("%v; menu %q", err, tt.Menu())
	}
	tt.Frame()
	if !a.settings.LyricRoman || !tt.HasText("kaze ga fuku") {
		t.Fatalf("roman %v, texts %q", a.settings.LyricRoman, tt.Texts())
	}
	if err := tt.Click("歌词显示"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("翻译"); err != nil {
		t.Fatalf("%v; menu %q", err, tt.Menu())
	}
	tt.Frame()
	if a.settings.LyricTranslation || tt.HasText("风在吹") {
		t.Fatalf("translation %v still shows %q", a.settings.LyricTranslation, tt.Texts())
	}
	if !tt.HasText("風が吹く") || !tt.HasText("รักนะ") {
		t.Fatalf("the lines themselves: %q", tt.Texts())
	}
}

// Turning 翻译 off keeps the lit line a third of the way down, where it
// was: the lines above it lost their translations, and it rose with them.
func TestLyricRowsKeepTheLitLine(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	var lrc, trans string
	for i := range 12 {
		sec := (i + 1) * 10
		stamp := fmt.Sprintf("[%02d:%02d.00]", sec/60, sec%60)
		text := fmt.Sprintf("第%d句", i+1)
		lrc += stamp + text + "\n"
		trans += stamp + "译" + text + "\n"
	}
	tr := netease.Track{ID: 1, Name: "歌", Artists: []netease.Artist{{Name: "人"}}, Duration: 180000}
	a.cue([]netease.Track{tr})
	a.pl.SetLyrics(lyrics.Build(lyrics.Source{LRC: lrc, Translation: trans}))
	a.pl.Hold(80 * time.Second) // the 8th line, 「第8句」
	a.nowPlaying, a.npPane = true, paneLyrics
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	r0, ok := tt.Find("第8句")
	if !ok {
		t.Fatal("the lit line is not shown")
	}
	if err := tt.Click("歌词显示"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("翻译"); err != nil {
		t.Fatalf("%v; menu %q", err, tt.Menu())
	}
	tt.Advance(time.Second)
	tt.Frame()
	r1, ok := tt.Find("第8句")
	if !ok {
		t.Fatal("the lit line went away")
	}
	if d := math.Abs(float64(r1.Y - r0.Y)); d > 4 {
		t.Fatalf("the lit line moved by %.1f", d)
	}
}

// With NM_FRAMES set, saves 歌词 with a Japanese song's reading and
// translation on, to look at: the line in Japanese forms (直, 骨, 角), the
// translation in Chinese ones.
func TestLyricListFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.LyricRoman = true
	a.cue([]netease.Track{{ID: 3, Name: "歌", Artists: []netease.Artist{{Name: "誰"}}, Duration: 60000}})
	a.pl.SetLyrics(lyrics.Build(lyrics.Source{
		LRC:         "[00:10.00]真っ直ぐな骨の角まで\n[00:16.00]風が吹く\n[00:22.00]遠くで息をする",
		Translation: "[00:10.00]直到骨头的棱角\n[00:16.00]风在吹\n[00:22.00]远处有人呼吸",
		Reading:     "[00:10.00]ma s su gu na ho ne no ka do ma de\n[00:16.00]ka ze ga fu ku\n[00:22.00]to o ku de i ki wo su ru",
	}))
	a.nowPlaying, a.npPane = true, paneLyrics
	a.pl.Hold(17 * time.Second)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.SetDark(true)
	tt.Frame()
	tt.Advance(2 * time.Second)
	tt.Frame()
	f, err := os.Create(filepath.Join(dir, "lyrics-ja.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}
