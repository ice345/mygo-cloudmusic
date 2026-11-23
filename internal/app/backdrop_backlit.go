package app

import (
	"image"
	"image/color"
	"math"
	"sync"

	"github.com/egoist/mygo/ui"
)

// 和鸣's backdrop is the cover's hue, deeper than the usual backdrop, so the
// line stands out hard against it, and lit from behind where the line
// stands: a broad light, with a few lights out of focus around it, as on a
// stage seen through a long lens. It is painted with the rest of the
// backdrop, once a song, and crossfades with it.

const (
	// backlitGlow is how bright the light behind the line is at its middle.
	backlitGlow = 0.20
	// backlitWash is how much of the cover, blurred, shows through.
	backlitWash = 0.07
)

// backlitColors returns the top and the foot of 和鸣's backdrop for a cover
// of tint: its hue, rich and deep. A grey cover stays grey.
func backlitColors(tint ui.Color) (top, foot ui.Color) {
	h, s, _ := hsv(float64(tint.R)/255, float64(tint.G)/255, float64(tint.B)/255)
	if s > 0.12 {
		s = min(max(s*1.3, 0.5), 0.75)
	} else {
		s = 0
	}
	return hsvColor(h, s, 0.24), hsvColor(h, min(s*1.05, 0.8), 0.12)
}

// backlitLight is the light from behind: a warm white, a little of the
// cover's hue in it.
func backlitLight(tint ui.Color) ui.Color {
	h, s, _ := hsv(float64(tint.R)/255, float64(tint.G)/255, float64(tint.B)/255)
	warm := ui.Hex("#FFE9C8")
	if s <= 0.12 {
		return warm
	}
	return warm.Mix(hsvColor(h, 0.45, 1), 0.3)
}

// drawBacklit paints 和鸣's backdrop in r.
func drawBacklit(p *ui.Painter, r ui.Rect, art *artwork, tint ui.Color) {
	top, foot := backlitColors(tint)
	p.FillGradient(r, ui.LinearGradient{Angle: 180, Oklab: true, Stops: []ui.GradientStop{
		{Color: top, At: 0}, {Color: foot, At: 1},
	}}, 0)
	if art != nil && art.backdrop != nil {
		p.ImageRegion(art.backdrop, ui.Rect{W: 48, H: 48}, coverRect(r, 48, 48), ui.ImageOptions{Tint: white.Alpha(backlitWash)})
	}
	light := backlitLight(tint)
	// Lights out of focus, away from the line, the same for a cover each
	// time it shows.
	seed := float64(int(tint.R)*65536 + int(tint.G)*256 + int(tint.B))
	for i := range 7 {
		k := float64(i)
		x := lineRandom(seed, k*3+1)
		y := lineRandom(seed, k*3+2) * 0.34
		if i%2 == 1 {
			y = 0.66 + y*0.5 // some low, under the line
		}
		size := r.W * (0.035 + 0.05*lineRandom(seed, k*3+3))
		at := ui.Rect{X: r.X + x*r.W - size/2, Y: r.Y + y*r.H - size/2, W: size, H: size}
		p.ImageRegion(discBitmap(), ui.Rect{}, at, ui.ImageOptions{Tint: light.Alpha(0.05 + 0.06*lineRandom(seed, k+40))})
	}
	// The light behind the line, wider than high.
	cx, cy, rad := r.X+r.W*0.5, r.Y+r.H*0.46, r.W*0.42
	p.ImageRegion(falloffBitmap(), ui.Rect{}, ui.Rect{X: cx - rad, Y: cy - rad*0.6, W: 2 * rad, H: 1.2 * rad},
		ui.ImageOptions{Tint: light.Alpha(backlitGlow)})
	// Its sides fall away into the dark.
	p.FillGradient(r, ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
		{Color: ui.RGBA(0, 0, 0, 0.3), At: 0}, {Color: ui.RGBA(0, 0, 0, 0), At: 0.3},
		{Color: ui.RGBA(0, 0, 0, 0), At: 0.7}, {Color: ui.RGBA(0, 0, 0, 0.3), At: 1},
	}}, 0)
	p.FillGradient(r, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
		{Color: ui.RGBA(0, 0, 0, 0), At: 0.55}, {Color: ui.RGBA(0, 0, 0, 0.25), At: 1},
	}}, 0)
}

// falloffBitmap is a white light, full at its middle and gone at its edge,
// for MyGo has no radial gradient.
var falloffBitmap = sync.OnceValue(func() *ui.Bitmap {
	return radialBitmap(func(d float64) float64 {
		t := max(0, 1-d)
		return t * t * (3 - 2*t)
	})
})

// discBitmap is a light out of focus: a disc, its edge soft, a little
// brighter at its rim.
var discBitmap = sync.OnceValue(func() *ui.Bitmap {
	return radialBitmap(func(d float64) float64 {
		edge := min(1, max(0, (1-d)/0.12))
		return edge * (0.75 + 0.25*min(1, max(0, (d-0.6)/0.3)))
	})
})

// radialBitmap is 64×64 white pixels, at the alpha f gives for their
// distance from the middle, 1 at the edge.
func radialBitmap(f func(d float64) float64) *ui.Bitmap {
	const n = 64
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := range n {
		for x := range n {
			dx, dy := (float64(x)+0.5)/n*2-1, (float64(y)+0.5)/n*2-1
			a := f(math.Sqrt(dx*dx + dy*dy))
			img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(math.Round(255 * min(1, max(0, a))))})
		}
	}
	return ui.NewBitmap(img)
}
