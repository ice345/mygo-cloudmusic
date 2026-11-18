package app

import (
	"image"
	"image/color"
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
	for _, style := range []string{"", "partita", "tilt", "aobird"} {
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
		// 青鸟 draws the line as one piece, so the word is not its own text.
		mark := "風"
		if style == "aobird" {
			mark = "風が廊下を渡る"
		}
		r0, ok := tt.Find(mark)
		if !ok {
			t.Fatalf("style %q: %s is not shown", style, mark)
		}
		a.settings.LyricTranslation, a.settings.LyricRoman = false, false
		tt.Frame()
		r1, ok := tt.Find(mark)
		if !ok {
			t.Fatalf("style %q: %s went away", style, mark)
		}
		a.settings.LyricTranslation, a.settings.LyricRoman = true, true
		tt.Frame()
		r2, ok := tt.Find(mark)
		if !ok {
			t.Fatalf("style %q: %s went away", style, mark)
		}
		for _, r := range []ui.Rect{r1, r2} {
			if math.Abs(float64(r.X-r0.X)) > 0.5 || math.Abs(float64(r.Y-r0.Y)) > 0.5 ||
				math.Abs(float64(r.W-r0.W)) > 0.5 || math.Abs(float64(r.H-r0.H)) > 0.5 {
				t.Errorf("style %q: 風 moved from %v to %v", style, r0, r)
			}
		}
	}
}

// Choosing 青鸟 leaves 火花 as it was, and it can't be turned on there.
func TestBirdKeepsSparks(t *testing.T) {
	a, tt, _ := playingApp()
	a.settings.StageSparks = true
	a.nowPlaying, a.npPane = true, paneStage
	tt.Frame()
	if err := tt.Click("律动效果"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("青鸟"); err != nil {
		t.Fatalf("%v; menu %q", err, tt.Menu())
	}
	tt.Frame()
	if a.settings.StageStyle != "aobird" || !a.settings.StageSparks {
		t.Fatalf("style %q, sparks %v", a.settings.StageStyle, a.settings.StageSparks)
	}
	if err := tt.Click("律动效果"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("火花"); err == nil || !a.settings.StageSparks {
		t.Fatalf("火花 in 青鸟: err %v, sparks %v, menu %q", err, a.settings.StageSparks, tt.Menu())
	}
	if err := tt.Click("律动效果"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("流光"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.settings.StageStyle != "" || !a.settings.StageSparks {
		t.Fatalf("back to 流光: style %q, sparks %v", a.settings.StageStyle, a.settings.StageSparks)
	}
}

// With NM_FRAMES set, saves 青鸟 at the moments and covers the style is
// reviewed with: mist, half-formed, formed, the answer, leaving, the
// hairline, three covers, the toggles, a fast line, a long line, an
// English line, and the sans fallback.
func TestBirdFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := func(text string, at, end time.Duration, lang, tr, rd string) lyrics.Line {
		return lyrics.Line{
			At: at, End: end, Text: text, Lang: lang, Translation: tr, Reading: rd,
			Words: []lyrics.Word{{Text: text, Start: at, End: at + (end-at)*3/4}},
		}
	}
	save := func(name string, lines []lyrics.Line, pos time.Duration, roman, trans bool, tint ui.Color, font string) {
		t.Helper()
		a := newTestApp()
		a.settings = defaultSettings()
		a.settings.StageStyle = "aobird"
		a.settings.LyricRoman = roman
		a.settings.LyricTranslation = trans
		if font != "" {
			prevJA, prevZH := birdFontJA, birdFontZH
			birdFontJA, birdFontZH = font, font
			defer func() { birdFontJA, birdFontZH = prevJA, prevZH }()
		}
		tr := netease.Track{ID: 3, Name: "青鸟", Artists: []netease.Artist{{Name: "誰"}}, Duration: 120000, Album: netease.Album{PicURL: name}}
		if tint != (ui.Color{}) {
			img := image.NewRGBA(image.Rect(0, 0, 48, 48))
			draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{tint.R, tint.G, tint.B, 255}}, image.Point{}, draw.Src)
			art := &artwork{bmp: ui.NewBitmap(img), backdrop: ui.NewBitmap(blurImage(img)), tint: tint}
			url := netease.Sized(name, 600)
			a.art.art[url] = art
			a.art.art[url+"#backdrop"] = art
		}
		a.pl.Cue([]netease.Track{tr})
		a.pl.SetLyrics(lines)
		a.pl.Hold(pos)
		a.nowPlaying, a.npPane = true, paneStage
		tt := ui.NewTester(a.view, 1200, 800)
		tt.SetDark(true)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}
	ja := line("風が吹く", 10*time.Second, 18*time.Second, "ja", "风在吹", "kaze ga fuku")
	next := line("遠くで", 16*time.Second, 22*time.Second, "ja", "在远处", "tooku de")
	save("bird-mist", []lyrics.Line{ja}, 10150*time.Millisecond, true, true, ui.Color{}, "")
	save("bird-half", []lyrics.Line{ja}, 10600*time.Millisecond, true, true, ui.Color{}, "")
	save("bird-formed", []lyrics.Line{ja}, 12*time.Second, true, true, ui.Color{}, "")
	save("bird-answer", []lyrics.Line{ja}, 10700*time.Millisecond, true, true, ui.Color{}, "")
	save("bird-leaving", []lyrics.Line{ja, next}, 15780*time.Millisecond, true, true, ui.Color{}, "")
	far := line("次の", 40*time.Second, 48*time.Second, "ja", "", "")
	save("bird-hair", []lyrics.Line{line("風が吹く", 10*time.Second, 14*time.Second, "ja", "", ""), far}, 18*time.Second, false, false, ui.Color{}, "")
	save("bird-red", []lyrics.Line{ja}, 12*time.Second, true, true, ui.RGB(180, 40, 40), "")
	save("bird-green", []lyrics.Line{ja}, 12*time.Second, true, true, ui.RGB(40, 140, 70), "")
	save("bird-grey", []lyrics.Line{ja}, 12*time.Second, true, true, ui.RGB(140, 140, 140), "")
	save("bird-no-translation", []lyrics.Line{ja}, 12*time.Second, true, false, ui.Color{}, "")
	save("bird-reading", []lyrics.Line{ja}, 12*time.Second, true, true, ui.Color{}, "")
	fast := line("風", 10*time.Second, 11200*time.Millisecond, "ja", "风", "kaze")
	save("bird-fast", []lyrics.Line{fast}, 11*time.Second, false, true, ui.Color{}, "")
	long := line("風が長い廊下を渡って夜の向こうまで続いていく", 10*time.Second, 20*time.Second, "ja", "风穿过长长的走廊，带着夜里的声音，一直走到听不见的地方", "kaze ga nagai rouka o watatte")
	save("bird-long", []lyrics.Line{long}, 14*time.Second, true, true, ui.Color{}, "")
	en := line("across the quiet water", 10*time.Second, 18*time.Second, "", "越过安静的水面", "")
	save("bird-english", []lyrics.Line{en}, 12*time.Second, false, true, ui.Color{}, "")
	save("bird-sans", []lyrics.Line{ja}, 12*time.Second, true, true, ui.Color{}, "NoSuchFamily")
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

// After a long break the line before stays gone as the next line comes in,
// in every style, and so does its translation; with no break it still
// fades as the next comes. It used to come back for a moment, faint, as
// the next line began: in 流光, 云阶 and 倾诉 its words for 0.4 s and its
// translation for 0.12 s, and in 青鸟 the whole line at about a third.
func TestLineBeforeStaysGoneAfterABreak(t *testing.T) {
	for _, gap := range []string{"break", "none"} {
		next := "[00:40.00]"
		if gap == "none" {
			next = "[00:16.00]"
		}
		lines := lyrics.Build(lyrics.Source{
			LRC:         "[00:10.00]風が吹く\n" + next + "遠くで息をする\n[00:58.00]終わり",
			Translation: "[00:10.00]风在吹\n" + next + "远处有人呼吸",
		})
		for _, style := range []string{"", "partita", "tilt", "aobird"} {
			a := newTestApp()
			a.settings = defaultSettings()
			a.settings.StageStyle = style
			a.pl.Cue([]netease.Track{{ID: 3, Name: "歌", Artists: []netease.Artist{{Name: "誰"}}, Duration: 60000}})
			a.pl.SetLyrics(lines)
			a.nowPlaying, a.npPane = true, paneStage
			// Just as the next line comes in: in 青鸟 as it starts to form.
			pos := lines[1].At + 50*time.Millisecond
			mark := "風"
			if style == "aobird" {
				pos, _ = birdClockOf(&lines[1], false).condense()
				mark = "風が吹く"
			}
			a.pl.Hold(pos)
			tt := ui.NewTester(a.view, 1200, 800)
			tt.Frame()
			_, words := tt.Find(mark)
			_, translation := tt.Find("风在吹")
			if gap == "break" && (words || translation) {
				t.Errorf("style %q after a break: the line before is back (words %v, translation %v)", style, words, translation)
			}
			if gap == "none" && !words {
				t.Errorf("style %q with no break: the line before is gone at once", style)
			}
		}
	}
}

// Two lines sung at one time, as a duet's are, both show on the stage, in
// every style. Their layers were keyed by time alone, so the two shared
// one, which MyGo stops on in tests.
func TestStageLinesAtOneTime(t *testing.T) {
	lines := lyrics.Build(lyrics.Source{YRC: "[10000,4000](10000,2000,0)Hello(12000,2000,0) world\n" +
		"[10000,4000](10000,2000,0)Bonjour(12000,2000,0) monde\n[20000,2000](20000,2000,0)Bye"})
	for _, style := range []string{"", "partita", "tilt", "aobird"} {
		a := newTestApp()
		a.settings = defaultSettings()
		a.settings.StageStyle = style
		a.pl.Cue([]netease.Track{{ID: 3, Name: "歌", Artists: []netease.Artist{{Name: "誰"}}, Duration: 60000}})
		a.pl.SetLyrics(lines)
		a.nowPlaying, a.npPane = true, paneStage
		// The first still fading as the second comes in, in the styles
		// where one line fades as the next comes.
		a.pl.Hold(10100 * time.Millisecond)
		tt := ui.NewTester(a.view, 1200, 800)
		tt.Frame()
		marks := map[string][]string{"": {"Hello", "Bonjour"}, "partita": {"Hello", "Bonjour"}, "tilt": {"H", "j"}, "aobird": {"Bonjour monde"}}[style]
		for _, mark := range marks {
			if _, ok := tt.Find(mark); !ok {
				t.Errorf("style %q: %s is not shown", style, mark)
			}
		}
	}
}
