package app

import (
	"image"
	"image/draw"
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

// Choosing a style from the stage's menu sets its lyrics in it, and
// choosing 流光 goes back to the classic.
func TestStageStyleMenu(t *testing.T) {
	a, tt, _ := playingApp()
	a.nowPlaying, a.npPane = true, paneStage
	a.pl.Seek(2 * time.Second)
	tt.Frame()
	for _, c := range []struct{ item, want string }{{"云阶", "partita"}, {"倾诉", "tilt"}, {"流光", ""}} {
		if err := tt.Click("律动效果"); err != nil {
			t.Fatal(err)
		}
		if err := tt.ChooseMenuItem(c.item); err != nil {
			t.Fatalf("%v; the menu shows %q", err, tt.Menu())
		}
		tt.Frame()
		if a.settings.StageStyle != c.want {
			t.Errorf("chose %s: style %q, want %q", c.item, a.settings.StageStyle, c.want)
		}
	}
}

// 火花 in the stage's menu turns the sparks on, and again off, whatever
// the style.
func TestStageSparksMenu(t *testing.T) {
	a, tt, _ := playingApp()
	a.nowPlaying, a.npPane = true, paneStage
	a.settings.StageStyle = "tilt"
	tt.Frame()
	for _, want := range []bool{true, false} {
		if err := tt.Click("律动效果"); err != nil {
			t.Fatal(err)
		}
		if err := tt.ChooseMenuItem("火花"); err != nil {
			t.Fatalf("%v; the menu shows %q", err, tt.Menu())
		}
		tt.Frame()
		if a.settings.StageSparks != want || a.settings.StageStyle != "tilt" {
			t.Errorf("sparks %v in %q, want %v in tilt", a.settings.StageSparks, a.settings.StageStyle, want)
		}
	}
}

// Turning the translation and the reading off and on moves none of the
// stage's words, in any style: under the line, they lifted it as they came.
func TestStageWordsStayWhenTheCaptionChanges(t *testing.T) {
	for _, style := range []string{"", "partita", "tilt"} {
		a := newTestApp()
		a.settings = defaultSettings()
		a.settings.LyricRoman = true
		a.settings.StageStyle = style
		tr := netease.Track{ID: 3, Name: "風", Artists: []netease.Artist{{Name: "誰"}}, Duration: 60000}
		a.pl.Cue([]netease.Track{tr})
		a.pl.SetLyrics(lyrics.Build(lyrics.Source{
			LRC:         "[00:10.00]風が廊下を渡る\n[00:18.00]遠くで息をする",
			Translation: "[00:10.00]风穿过走廊\n[00:18.00]远处有人呼吸",
			Reading:     "[00:10.00]kaze ga fuku\n[00:18.00]tooku de iki o suru",
		}))
		a.nowPlaying, a.npPane = true, paneStage
		a.pl.Hold(12 * time.Second) // the first word has settled
		tt := ui.NewTester(a.view, 1200, 800)
		tt.Frame()
		r0, ok := tt.Find("風")
		if !ok {
			t.Fatalf("style %q: 風 is not shown", style)
		}
		a.settings.LyricTranslation, a.settings.LyricRoman = false, false
		tt.Frame()
		r1, ok := tt.Find("風")
		if !ok {
			t.Fatalf("style %q: 風 went away", style)
		}
		a.settings.LyricTranslation, a.settings.LyricRoman = true, true
		tt.Frame()
		r2, ok := tt.Find("風")
		if !ok {
			t.Fatalf("style %q: 風 went away", style)
		}
		for _, r := range []ui.Rect{r1, r2} {
			if math.Abs(float64(r.X-r0.X)) > 0.5 || math.Abs(float64(r.Y-r0.Y)) > 0.5 ||
				math.Abs(float64(r.W-r0.W)) > 0.5 || math.Abs(float64(r.H-r0.H)) > 0.5 {
				t.Errorf("style %q: 風 moved from %v to %v", style, r0, r)
			}
		}
	}
}

// With NM_FRAMES set, saves one frame of each stage style with a reading
// and a translation along the foot.
func TestStageCaptionFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, s := range stageStyles {
		name := "caption.png"
		if s.id != "" {
			name = "caption-" + s.id + ".png"
		}
		a := newTestApp()
		a.settings = defaultSettings()
		a.settings.LyricRoman = true
		a.settings.StageStyle = s.id
		tr := netease.Track{ID: 3, Name: "風", Artists: []netease.Artist{{Name: "誰"}}, Duration: 60000}
		a.pl.Cue([]netease.Track{tr})
		a.pl.SetLyrics(lyrics.Build(lyrics.Source{
			LRC:         "[00:10.00]風が廊下を渡る\n[00:18.00]遠くで息をする",
			Translation: "[00:10.00]风穿过长长的走廊，带着夜里的声音，一直走到听不见的地方\n[00:18.00]远处有人呼吸",
			Reading:     "[00:10.00]kaze ga rouka o wataru yoru no oto o tsurete\n[00:18.00]tooku de iki o suru",
		}))
		a.nowPlaying, a.npPane = true, paneStage
		a.pl.Hold(12500 * time.Millisecond)
		tt := ui.NewTester(a.view, 1200, 800)
		tt.SetDark(true)
		tt.Advance(time.Second)
		tt.Frame()
		img := tt.Image()
		b := img.Bounds()
		crop := image.Rect(b.Min.X, b.Min.Y+b.Dy()/5, b.Max.X, b.Max.Y-b.Dy()/8)
		out := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
		draw.Draw(out, out.Bounds(), img, crop.Min, draw.Src)
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, out); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}
}

func words(texts ...string) []lyrics.Word {
	out := make([]lyrics.Word, len(texts))
	for i, s := range texts {
		out[i] = lyrics.Word{Text: s, Start: time.Duration(i) * time.Second, End: time.Duration(i+1) * time.Second}
	}
	return out
}

// A line breaks at its spaces and punctuation, punctuation staying with
// the word before it and a contraction whole, and a long run of
// characters in pieces.
func TestStageUnits(t *testing.T) {
	for _, c := range []struct {
		words []lyrics.Word
		want  [][2]int
	}{
		{words("从", "不 ", "主", "动 ", "示", "弱"), [][2]int{{0, 2}, {2, 4}, {4, 6}}},
		{words("Hello", ", ", "It", "’", "s ", "me"), [][2]int{{0, 2}, {2, 5}, {5, 6}}},
		{words("I ", "never ", "show"), [][2]int{{0, 1}, {1, 2}, {2, 3}}},
		{words("曾", "经", "的", "那", "些", "发", "生", "过"), [][2]int{{0, 4}, {4, 8}}},
	} {
		got := stageUnits(c.words)
		if len(got) != len(c.want) {
			t.Errorf("%v: units %v, want %v", c.words, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%v: units %v, want %v", c.words, got, c.want)
				break
			}
		}
	}
}

// 倾诉 splits a line into parts that follow one another, none empty, and
// rather after punctuation than nearest the middle.
func TestTiltSplitsAtPunctuation(t *testing.T) {
	ws := words("夜", "空", "中", "最", "亮", "的", "星，", "能", "否", "听", "清")
	units := stageUnits(ws)
	for n := 1; n <= min(4, len(units)); n++ {
		parts := splitParts(ws, units, n)
		if len(parts) != n || parts[0].from != 0 || parts[n-1].to != len(ws) {
			t.Fatalf("%d parts: %+v", n, parts)
		}
		for i := 1; i < n; i++ {
			if parts[i].from != parts[i-1].to || parts[i].from >= parts[i].to {
				t.Fatalf("%d parts: %+v", n, parts)
			}
		}
	}
	if parts := splitParts(ws, units, 2); parts[0].to != 7 {
		t.Errorf("split at %d, not after 星，", parts[0].to)
	}
}
