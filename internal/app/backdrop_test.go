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

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

// The drift eases in as the music plays and glides to a stop as it
// pauses; the bass swells and settles.
func TestBackdropDrifts(t *testing.T) {
	var bd backdropCache
	now := time.Unix(0, 0)
	var quiet [player.Bands]float32
	for range 120 {
		now = now.Add(backdropFrame)
		bd.step(now, true, quiet, true)
	}
	if bd.speed != 1 || bd.drift <= 0 {
		t.Fatalf("playing: speed %v, drift %v", bd.speed, bd.drift)
	}
	var loud [player.Bands]float32
	for i := range loud {
		loud[i] = 1
	}
	now = now.Add(backdropFrame)
	bd.step(now, true, loud, true)
	if bd.level.group(0, 6) <= 0.2 {
		t.Errorf("bass %v on a beat", bd.level.group(0, 6))
	}
	moving := true
	for range 120 {
		now = now.Add(backdropFrame)
		moving = bd.step(now, false, quiet, false)
	}
	if moving || bd.speed != 0 {
		t.Errorf("paused: still moving, speed %v", bd.speed)
	}
	at := bd.drift
	now = now.Add(time.Second)
	bd.step(now, false, quiet, false)
	if bd.drift != at {
		t.Error("drifted while paused")
	}
}

// With NM_FRAMES set, saves the fog of a few covers as it drifts,
// and on a beat, one row a cover.
func TestBackdropDriftFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	const w, h = 300, 420
	var rows []*image.RGBA
	for _, q := range []string{"春日影 MyGO", "晴天 周杰伦", "夜に駆ける"} {
		hits, err := netease.Search(q, 1)
		if err != nil || len(hits) == 0 {
			t.Fatal(q, err)
		}
		img, err := fetchImage(netease.Sized(hits[0].Album.PicURL, 600))
		if err != nil {
			t.Fatal(err)
		}
		art := &artwork{tint: dominantColor(img)}
		shots := []struct{ drift, bass float32 }{{0, 0}, {2, 0}, {4, 0}, {4, 0.9}}
		row := image.NewRGBA(image.Rect(0, 0, w*len(shots), h))
		for i, s := range shots {
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) { drawFog(p, r, art.tint, s.drift, s.bass, toneDeep) })
			}, w, h)
			tt.Frame()
			shot := tt.Image()
			draw.Draw(row, image.Rect(i*w, 0, (i+1)*w, h), shot, shot.Bounds().Min, draw.Src)
		}
		rows = append(rows, row)
	}
	out := image.NewRGBA(image.Rect(0, 0, rows[0].Bounds().Dx(), h*len(rows)))
	for i, r := range rows {
		draw.Draw(out, image.Rect(0, i*h, r.Bounds().Dx(), (i+1)*h), r, image.Point{}, draw.Src)
	}
	f, _ := os.Create(filepath.Join(dir, "drift.png"))
	defer f.Close()
	png.Encode(f, out)
}

// Each surface's reading and translation read at 4.5:1 or more over the
// lightest still backdrop where they sit: a pale cover of any hue, its
// wash white. 青鸟's were about 3:1 and 3.7:1 over its glass, and the
// reading under the line sung in 歌词 about 3:1.
func TestCaptionsReadOverTheBackdrop(t *testing.T) {
	type caption struct {
		name  string
		color ui.Color
		least float64 // the contrast it needs, if not 4.5:1
	}
	surfaces := []struct {
		name   string
		tone   backdropTone
		y0, y1 float64 // where they sit, as parts of the window's height
		caps   []caption
	}{
		{"青鸟", toneGlass, 0.35, 0.7, []caption{{"reading", birdBreath, 0}, {"translation", birdAnswer, 0}}},
		// The foot band, from the shortest window's to the tallest's.
		{"the foot band", toneDeep, 0.62, 0.88, []caption{{"reading", footReading, 0}, {"translation", footTranslation, 0}}},
		// Under the line sung, a third of the way down the list.
		{"歌词", toneDeep, 0.3, 0.7, []caption{{"reading", listReading, 0}, {"translation", listTranslation, 0}}},
		// Under the line, in the light behind it.
		{"和鸣", toneBacklit, 0.42, 0.75, []caption{{"reading", chorusReading, 0}, {"translation", chorusAnswer, 0}}},
		// The line itself, in the middle of the light: large and bold,
		// its gold word needs 3:1.
		{"和鸣's line", toneBacklit, 0.36, 0.58, []caption{{"gold word", chorusGold, 3}}},
	}
	cover := image.NewRGBA(image.Rect(0, 0, 48, 48))
	draw.Draw(cover, cover.Bounds(), &image.Uniform{color.RGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)
	blurred := blurImage(cover)
	art := &artwork{backdrop: ui.NewBitmap(blurred), grey: ui.NewBitmap(greyImage(blurred))}
	var tints []ui.Color
	for hue := 0.0; hue < 360; hue += 30 {
		for _, sat := range []float64{0.2, 0.6, 0.95} {
			tints = append(tints, hsvColor(hue, sat, 0.85))
		}
	}
	tints = append(tints, ui.RGB(200, 200, 200))
	for _, sf := range surfaces {
		worst := map[string]float64{}
		for _, tint := range tints {
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Draw(func(p *ui.Painter, r ui.Rect) {
					drawBackdrop(p, r, art, tint, sf.tone)
				})
			}, 300, 200)
			tt.Frame()
			img := tt.Image()
			b := img.Bounds()
			for y := b.Min.Y + int(sf.y0*float64(b.Dy())); y < b.Min.Y+int(sf.y1*float64(b.Dy())); y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					r, g, bl, _ := img.At(x, y).RGBA()
					bg := [3]float64{float64(r) / 65535, float64(g) / 65535, float64(bl) / 65535}
					for _, cp := range sf.caps {
						a := float64(cp.color.A) / 255
						fg := [3]float64{}
						for i, v := range []uint8{cp.color.R, cp.color.G, cp.color.B} {
							fg[i] = float64(v)/255*a + bg[i]*(1-a)
						}
						ratio := (luminance(fg) + 0.05) / (luminance(bg) + 0.05)
						if w, ok := worst[cp.name]; !ok || ratio < w {
							worst[cp.name] = ratio
						}
					}
				}
			}
		}
		for _, cp := range sf.caps {
			t.Logf("%s %s %.2f", sf.name, cp.name, worst[cp.name])
			least := cp.least
			if least == 0 {
				least = 4.5
			}
			if worst[cp.name] < least {
				t.Errorf("%s: the %s reads at %.2f:1 over the lightest backdrop, under %.1f:1", sf.name, cp.name, worst[cp.name], least)
			}
		}
	}
}

// luminance is an sRGB color's relative luminance, as WCAG defines it.
func luminance(c [3]float64) float64 {
	lin := func(v float64) float64 {
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c[0]) + 0.7152*lin(c[1]) + 0.0722*lin(c[2])
}
