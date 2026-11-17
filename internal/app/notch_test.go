package app

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/audio"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// The notch, turned off, shows itself as the first song plays, and only
// then; its button keeps it on.
func TestNotchGreetsOnce(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	n := &notch{a: a, w0: 200, h0: 32, winW: notchOpenW + 60, winH: 240}
	a.notch = n
	a.pl.Cue([]netease.Track{{ID: 7, Name: "宝船", Duration: 200000}})
	a.syncWindows()
	if n.hello {
		t.Fatal("greeted before the song played")
	}
	out := audio.NewPlayer(&silence{})
	out.SetVolume(0)
	a.pl.Attach(out)
	defer out.Close()
	a.syncWindows()
	if !n.hello || !a.settings.NotchGreeted {
		t.Fatal("did not greet as the song played")
	}

	tt := ui.NewTester(n.view, int(n.winW), int(n.winH))
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.Click("开启刘海播放器"); err != nil {
		t.Fatal(err)
	}
	if !a.settings.Notch || n.hello {
		t.Fatalf("kept: notch %v, greeting %v", a.settings.Notch, n.hello)
	}

	// Turned off again, it does not greet the next song.
	a.settings.Notch = false
	a.pl.Cue([]netease.Track{{ID: 8, Name: "下一首", Duration: 200000}})
	a.pl.Attach(out)
	a.syncWindows()
	if n.hello {
		t.Error("greeted twice")
	}
}

// The pointer resting on the greeting leaves it as it is, so its button can
// be clicked. It used to open the player, which hid the button, and leaving
// brought it back only to open again as the pointer came back to it.
func TestNotchGreetingKeptUnderPointer(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	n := &notch{a: a, real: true, w0: 185, h0: 32, winW: notchOpenW + 60, winH: 240}
	a.notch = n
	a.pl.Cue([]netease.Track{{ID: 7, Name: "宝船", Duration: 200000}})
	n.greet()
	if !n.hello {
		t.Fatal("did not greet")
	}

	defer func(f func() mygo.Point) { cursorPoint = f }(cursorPoint)
	cursorPoint = func() mygo.Point {
		return mygo.Point{X: int(n.screenX + n.winW/2 + 120), Y: int(n.screenY + n.h0 + 28)}
	}
	n.poll()
	n.enteredAt = n.lastInside.Add(-time.Second) // as if it had rested there
	n.poll()
	if n.open {
		t.Fatal("opened the player over the greeting")
	}

	tt := ui.NewTester(n.view, int(n.winW), int(n.winH))
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.Click("开启刘海播放器"); err != nil {
		t.Fatal(err)
	}
	if !a.settings.Notch || n.hello {
		t.Fatalf("kept: notch %v, greeting %v", a.settings.Notch, n.hello)
	}
}

// With another app at the notch, the notch neither greets the first song
// nor opens under a resting pointer; once that app quits, it opens again.
func TestNotchMakesWay(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	n := &notch{a: a, real: true, w0: 200, h0: 32, winW: notchOpenW + 60, winH: 240, neighbor: "Atoll"}
	a.notch = n
	defer func(f func() string) { notchNeighbor = f }(notchNeighbor)
	defer func(f func() mygo.Point) { cursorPoint = f }(cursorPoint)
	notchNeighbor = func() string { return "Atoll" }

	a.pl.Cue([]netease.Track{{ID: 7, Name: "宝船", Duration: 200000}})
	a.syncWindows()
	out := audio.NewPlayer(&silence{})
	out.SetVolume(0)
	a.pl.Attach(out)
	defer out.Close()
	a.syncWindows()
	if n.hello || a.settings.NotchGreeted {
		t.Fatalf("greeted: hello %v, greeted %v", n.hello, a.settings.NotchGreeted)
	}

	// The listener has it on. The other app still has the spot.
	a.settings.Notch = true
	cursorPoint = func() mygo.Point {
		return mygo.Point{X: int(n.screenX + n.winW/2), Y: int(n.screenY + n.h0/2)}
	}
	n.poll()
	n.enteredAt = n.lastInside.Add(-time.Second)
	n.poll()
	if n.open {
		t.Fatal("opened under the other app")
	}

	notchNeighbor = func() string { return "" }
	n.lookAround()
	n.poll()
	n.enteredAt = n.lastInside.Add(-time.Second)
	n.poll()
	if !n.open {
		t.Fatal("did not open once the other app quit")
	}
}

// Resting the pointer on the lyrics under the notch opens nothing, and
// clicks pass through to the window under them. Resting on the housing
// still opens the player.
func TestNotchLyricsPassThePointer(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.Notch = true
	a.settings.NotchLyrics = 1
	n := &notch{a: a, real: true, w0: 200, h0: 32, winW: notchOpenW + 60, winH: 240}
	a.notch = n
	a.pl.Cue([]netease.Track{{ID: 7, Name: "风", Duration: 60000}})
	a.pl.SetLyrics(lyrics.Build(lyrics.Source{LRC: "[00:00.00]风在吹\n[00:10.00]下一句"}))
	a.pl.Hold(time.Second)
	defer func(f func() mygo.Point) { cursorPoint = f }(cursorPoint)
	// 14 is the middle of the line. 2 is its top, where the housing's hit
	// area used to reach and open the player.
	for _, dy := range []float32{14, 2} {
		cursorPoint = func() mygo.Point {
			return mygo.Point{X: int(n.screenX + n.winW/2), Y: int(n.screenY + n.h0 + dy)}
		}
		n.open = false
		n.poll()
		n.enteredAt = n.lastInside.Add(-time.Second)
		n.poll()
		if n.open || !n.passing || !n.under {
			t.Fatalf("at %v under the housing: open %v, passing %v, over the lyrics %v", dy, n.open, n.passing, n.under)
		}
	}

	cursorPoint = func() mygo.Point {
		return mygo.Point{X: int(n.screenX + n.winW/2), Y: int(n.screenY + n.h0/2)}
	}
	n.open = false
	n.poll()
	n.enteredAt = n.lastInside.Add(-time.Second)
	n.poll()
	if !n.open || n.passing || n.under {
		t.Fatalf("housing: open %v, passing %v, over the lyrics %v", n.open, n.passing, n.under)
	}
}

// The open card keeps the pointer as it moves from the housing onto the
// card's controls, and so does the greeting. Since the lyrics came under
// the notch, only the housing counted: on the card, clicks went to the
// window under it, and the card closed.
func TestNotchCardKeepsThePointer(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.Notch = true
	a.settings.NotchLyrics = 2
	n := &notch{a: a, real: true, w0: 200, h0: 32, winW: notchOpenW + 60, winH: 240}
	a.notch = n
	a.pl.Cue([]netease.Track{{ID: 7, Name: "风", Duration: 60000}})
	a.pl.SetLyrics(lyrics.Build(lyrics.Source{LRC: "[00:00.00]风在吹\n[00:10.00]下一句"}))
	a.pl.Hold(time.Second)
	defer func(f func() mygo.Point) { cursorPoint = f }(cursorPoint)
	at := func(dx, dy float32) {
		cursorPoint = func() mygo.Point {
			return mygo.Point{X: int(n.screenX + n.winW/2 + dx), Y: int(n.screenY + n.h0 + dy)}
		}
	}

	at(0, -n.h0/2)
	n.poll()
	n.enteredAt = n.lastInside.Add(-time.Second)
	n.poll()
	if !n.open {
		t.Fatal("did not open on the housing")
	}
	// Down to the controls, and a second there.
	at(0, 130)
	n.poll()
	n.lastInside = n.lastInside.Add(-time.Second)
	n.poll()
	if !n.open || n.passing {
		t.Fatalf("on the card: open %v, passing %v", n.open, n.passing)
	}

	// The greeting, its button under the pointer.
	a.settings.Notch = false
	n.open, n.hello = false, true
	at(120, 28)
	n.poll()
	if n.open || n.passing {
		t.Fatalf("on the greeting: open %v, passing %v", n.open, n.passing)
	}
}

// With NM_FRAMES set, saves the lyrics under the notch: one line filling,
// two and three lines, the next line during a break, and the strip faded
// under the pointer.
func TestNotchLyricStripFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	save := func(name string, n *notch) {
		tt := ui.NewTester(n.view, int(n.winW), int(n.winH))
		tt.SetDark(true)
		tt.Advance(time.Second)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, tt.Image())
		f.Close()
	}
	lines := []lyrics.Line{
		{At: 0, End: 3 * time.Second, Text: "上一句"},
		{
			At: 4 * time.Second, End: 8 * time.Second, Text: "风在吹", Lang: "zh",
			Reading: "feng zai chui", Translation: "the wind blows",
			Words: []lyrics.Word{
				{Text: "风", Start: 4 * time.Second, End: 5 * time.Second},
				{Text: "在", Start: 5 * time.Second, End: 6 * time.Second},
				{Text: "吹", Start: 6 * time.Second, End: 7 * time.Second},
			},
		},
		{At: 16 * time.Second, End: 20 * time.Second, Text: "下一句"},
	}
	for _, rows := range []int{1, 2, 3} {
		a := newTestApp()
		a.settings = defaultSettings()
		a.settings.Notch = true
		a.settings.NotchLyrics = rows
		a.settings.LyricRoman = true
		n := &notch{a: a, real: true, w0: 200, h0: 32, winW: notchOpenW + 60, winH: 240}
		a.notch = n
		a.pl.Cue([]netease.Track{{ID: 7, Name: "风", Duration: 30000}})
		a.pl.SetLyrics(lines)
		a.pl.Hold(5500 * time.Millisecond)
		save(fmt.Sprintf("notch-lyrics-%d", rows), n)
	}
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.Notch = true
	a.settings.NotchLyrics = 1
	n := &notch{a: a, real: true, w0: 200, h0: 32, winW: notchOpenW + 60, winH: 240}
	a.notch = n
	a.pl.Cue([]netease.Track{{ID: 7, Name: "风", Duration: 30000}})
	a.pl.SetLyrics(lines)
	a.pl.Hold(9 * time.Second)
	save("notch-lyrics-break", n)
	n.under = true
	save("notch-lyrics-under", n)
}

// With NM_FRAMES set, saves the notch as it greets, growing and grown, and
// the player bar holding a song a login picked up.
func TestFirstUseFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	save := func(name string, tt *ui.Tester) {
		f, _ := os.Create(filepath.Join(dir, name+".png"))
		png.Encode(f, tt.Image())
		f.Close()
	}
	a := newTestApp()
	a.settings = defaultSettings()
	n := &notch{a: a, real: true, w0: 185, h0: 32, winW: notchOpenW + 60, winH: 240}
	a.notch = n
	a.pl.Cue([]netease.Track{{ID: 7, Name: "NAINAINAI", Artists: []netease.Artist{{Name: "ATARASHII GAKKO!"}}, Duration: 193877}})
	main := ui.NewTester(a.view, 1200, 800) // for the app's colors
	main.Frame()
	tt := ui.NewTester(n.view, int(n.winW), int(n.winH))
	tt.Frame()
	n.greet()
	for _, ms := range []int{120, 600} {
		tt.Advance(time.Duration(ms) * time.Millisecond)
		tt.Frame()
		save(fmt.Sprintf("notch-hello-%d", ms), tt)
	}

	a.picked = pickedUp{id: 7, heard: "3分钟前在手机上听过"}
	main.Advance(time.Second)
	main.Frame()
	save("pickup-bar", main)
}
