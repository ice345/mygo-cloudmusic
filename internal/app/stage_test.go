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
	for _, s := range stageStyles {
		style := s.id
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
		// A style that draws the line as one piece, as 青鸟 does, has no
		// text of the word's own.
		mark := "風"
		r0, ok := tt.Find(mark)
		if !ok {
			mark = "風が廊下を渡る"
			r0, ok = tt.Find(mark)
		}
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
// English line, the sans fallback, a one-character line with a long
// translation, and a Korean line.
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
			prevJA, prevKO, prevZH := birdFontJA, birdFontKO, birdFontZH
			birdFontJA, birdFontKO, birdFontZH = font, font, font
			defer func() { birdFontJA, birdFontKO, birdFontZH = prevJA, prevKO, prevZH }()
		}
		tr := netease.Track{ID: 3, Name: "青鸟", Artists: []netease.Artist{{Name: "誰"}}, Duration: 120000, Album: netease.Album{PicURL: name}}
		if tint != (ui.Color{}) {
			img := image.NewRGBA(image.Rect(0, 0, 48, 48))
			draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{tint.R, tint.G, tint.B, 255}}, image.Point{}, draw.Src)
			art := &artwork{bmp: ui.NewBitmap(img), backdrop: ui.NewBitmap(blurImage(img)), grey: ui.NewBitmap(greyImage(blurImage(img))), tint: tint}
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
	one := line("風", 10*time.Second, 18*time.Second, "ja", "风在吹，吹过整条走廊", "kaze")
	save("bird-one", []lyrics.Line{one}, 12*time.Second, true, true, ui.Color{}, "")
	ko := line("너의 모든 걸 원해 지금 이대로 영원히", 10*time.Second, 18*time.Second, "ko", "想要你的全部，就这样直到永远", "neoui modeun geol wonhae")
	save("bird-korean", []lyrics.Line{ko}, 12*time.Second, true, true, ui.Color{}, "")
	save("bird-answer-late", []lyrics.Line{ja}, 10300*time.Millisecond, true, true, ui.Color{}, "")
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
		for _, s := range stageStyles {
			style := s.id
			a := newTestApp()
			a.settings = defaultSettings()
			a.settings.StageStyle = style
			a.pl.Cue([]netease.Track{{ID: 3, Name: "歌", Artists: []netease.Artist{{Name: "誰"}}, Duration: 60000}})
			a.pl.SetLyrics(lines)
			a.nowPlaying, a.npPane = true, paneStage
			// Just as the next line comes in: in 青鸟 as it starts to form.
			pos, mark := lines[1].At+50*time.Millisecond, "風"
			switch style {
			case "", "partita", "tilt":
			case "aobird":
				pos, _ = birdClockOf(&lines[1], false).condense()
				mark = "風が吹く"
			default:
				t.Fatalf("style %q: say here when its next line comes in", style)
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
	for _, s := range stageStyles {
		style := s.id
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
		marks, ok := map[string][]string{"": {"Hello", "Bonjour"}, "partita": {"Hello", "Bonjour"}, "tilt": {"H", "j"}, "aobird": {"Bonjour monde"}}[style]
		if !ok {
			t.Fatalf("style %q: say here what of each line it shows", style)
		}
		for _, mark := range marks {
			if _, ok := tt.Find(mark); !ok {
				t.Errorf("style %q: %s is not shown", style, mark)
			}
		}
	}
}

// 青鸟's translation comes in 0.4 s after the line's first word, whatever
// the line's length, and goes 0.4 s after the line starts to: it came with
// a long line still forming, 0.1 s after its first word.
func TestBirdAnswersAfterTheLine(t *testing.T) {
	for _, length := range []time.Duration{2 * time.Second, 8 * time.Second} {
		l := lyrics.Line{At: 10 * time.Second, End: 10*time.Second + length, Text: "風が吹く",
			Words: []lyrics.Word{{Text: "風が吹く", Start: 10 * time.Second, End: 10*time.Second + length/2}}}
		next := lyrics.Line{At: l.End + time.Second, End: l.End + 4*time.Second, Text: "遠くで"}
		clock := birdClockOf(&l, false)
		at := func(pos time.Duration) float32 { return birdAnswerShown(clock, &l, &next, pos, false, false).k }
		if k := at(l.At + 390*time.Millisecond); k > 0 {
			t.Errorf("a %v line: its translation is %.2f in 0.39 s after its first word", length, k)
		}
		if k := at(l.At + 1400*time.Millisecond); k < 0.99 {
			t.Errorf("a %v line: its translation is only %.2f in 1.4 s after its first word", length, k)
		}
		leave, _, _ := birdLeave(&l, &next, false, false)
		if op, _, _, _ := birdLook(clock, &l, &next, leave+450*time.Millisecond, 48, false, false); op > 0.01 {
			t.Fatalf("a %v line: still %.2f there once it has left", length, op)
		}
		if k := at(leave + 300*time.Millisecond); k < 0.99 {
			t.Errorf("a %v line: its translation goes with it, %.2f 0.3 s after it starts to leave", length, k)
		}
	}
}

// 青鸟's translation answers from the right: its right edge is the line's,
// and one wider than the line sets out from the line's middle, in one row.
// It started 14% in from the line's left, as wide as the line at most, so
// it sat left of centre and stacked a one-character line's a character a
// row.
func TestBirdAnswersFromTheRight(t *testing.T) {
	for _, c := range []struct{ text, translation string }{
		{"風が廊下を渡る", "风穿过走廊"},
		{"風", "风在吹，吹过整条走廊"},
	} {
		a := newTestApp()
		a.settings = defaultSettings()
		a.settings.StageStyle = "aobird"
		a.pl.Cue([]netease.Track{{ID: 3, Name: "歌", Artists: []netease.Artist{{Name: "誰"}}, Duration: 60000}})
		a.pl.SetLyrics([]lyrics.Line{{At: 10 * time.Second, End: 18 * time.Second, Text: c.text, Lang: "ja", Translation: c.translation,
			Words: []lyrics.Word{{Text: c.text, Start: 10 * time.Second, End: 16 * time.Second}}}})
		a.nowPlaying, a.npPane = true, paneStage
		a.pl.Hold(14 * time.Second)
		tt := ui.NewTester(a.view, 1200, 800)
		tt.Frame()
		line, ok := tt.Find(c.text)
		answer, ok2 := tt.Find(c.translation)
		if !ok || !ok2 {
			t.Fatalf("%s: line shown %v, translation shown %v", c.text, ok, ok2)
		}
		if answer.W <= line.W {
			if d := (answer.X + answer.W) - (line.X + line.W); math.Abs(float64(d)) > 1 {
				t.Errorf("%s: the translation ends %.1f from the line's right edge", c.text, d)
			}
		} else {
			if mid := line.X + line.W/2; math.Abs(float64(answer.X-mid)) > 1 {
				t.Errorf("%s: the translation, wider than the line, starts at %.1f, not its middle %.1f", c.text, answer.X, mid)
			}
			if answer.H > 2*birdAnswerSize(birdSize(1200)) {
				t.Errorf("%s: the translation takes %.0f high, more than a row", c.text, answer.H)
			}
		}
	}
}

// In the foot band the translation keeps its size and place with the
// reading on or off, in every style that has the band: as the second row
// it shrank and dimmed when 音译 came on.
func TestFootTranslationStaysWhenTheReadingChanges(t *testing.T) {
	for _, s := range stageStyles {
		if s.line == nil {
			continue // its own captions, no foot band
		}
		a := newTestApp()
		a.settings = defaultSettings()
		a.settings.StageStyle = s.id
		a.settings.LyricTranslation = true
		a.pl.Cue([]netease.Track{{ID: 3, Name: "歌", Artists: []netease.Artist{{Name: "誰"}}, Duration: 60000}})
		a.pl.SetLyrics(lyrics.Build(lyrics.Source{
			LRC:         "[00:10.00]風が廊下を渡る\n[00:18.00]遠くで息をする",
			Translation: "[00:10.00]风穿过走廊\n[00:18.00]远处有人呼吸",
			Reading:     "[00:10.00]kaze ga rouka o wataru\n[00:18.00]tooku de iki o suru",
		}))
		a.nowPlaying, a.npPane = true, paneStage
		a.pl.Hold(12 * time.Second)
		tt := ui.NewTester(a.view, 1200, 800)
		tt.Frame()
		r0, ok := tt.Find("风穿过走廊")
		if !ok {
			t.Fatalf("style %q: the translation is not shown", s.id)
		}
		a.settings.LyricRoman = true
		tt.Frame()
		r1, ok := tt.Find("风穿过走廊")
		if _, reading := tt.Find("kaze ga rouka o wataru"); !ok || !reading {
			t.Fatalf("style %q: translation shown %v, reading shown %v", s.id, ok, reading)
		}
		if math.Abs(float64(r1.X-r0.X)) > 0.5 || math.Abs(float64(r1.Y-r0.Y)) > 0.5 ||
			math.Abs(float64(r1.W-r0.W)) > 0.5 || math.Abs(float64(r1.H-r0.H)) > 0.5 {
			t.Errorf("style %q: the translation went from %v to %v as the reading came on", s.id, r0, r1)
		}
	}
}
