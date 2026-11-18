package app

import (
	"runtime"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// 青鸟 leaves the stage quiet. A line forms whole out of a mist and stays
// still: no per-word light, no scale, no sparks. Its reading sits under it,
// and its translation answers late, off to the right. In a break a hairline
// holds the place. Hot comments never take this style, and it draws no foot
// and no next line: the empty space is the point. The release notes should
// say the same.

// birdFontJA and birdFontZH are the serif families for a Japanese line and
// for everything else. A test points them at a family that does not exist.
var birdFontJA, birdFontZH = birdFonts(runtime.GOOS)

// birdFonts returns the serif families goos installs, for a Japanese line
// and for everything else: macOS's own, Windows's, and Noto's elsewhere.
// Each system is asked only for its own. Asked for YuMincho, which it only
// offers as a download, macOS can wait on its font catalog: a Mac in CI
// waited ten minutes.
func birdFonts(goos string) (ja, zh string) {
	switch goos {
	case "darwin":
		return "Hiragino Mincho ProN", "Songti SC"
	case "windows":
		return "Yu Mincho", "SimSun"
	}
	return "Noto Serif CJK JP", "Noto Serif CJK SC"
}

var (
	birdInk    = ui.Hex("#EEF4F6")
	birdBreath = white.Alpha(0.46)
	birdAnswer = ui.Hex("#F3EEDF").Alpha(0.62)
)

// birdRest is the hairline: a pale teal, half there, pulled a little
// toward the cover's accent.
func birdRest(accent ui.Color) ui.Color {
	return ui.Hex("#B0CECC").Mix(accent, 0.15).Alpha(0.5)
}

// birdStage shows the lyrics of tr in 青鸟, as a function of pos alone.
func (a *app) birdStage(c *ui.Context, tr *netease.Track, lines []lyrics.Line, loaded bool, pos time.Duration, w, h float32, accent ui.Color) {
	_ = tr
	_ = h
	box := ui.Box(c.Key("stage")).Grow(1)
	size := birdSize(w)
	if len(lines) == 0 {
		box.Children(func() {
			ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Children(func() {
				if !loaded {
					stageSkeleton(c, npShimmer(c), size, w)
					return
				}
				ui.Text(c, "纯音乐，请欣赏").Font(birdFontZH).FontSize(max(14, size*0.38)).FontWeight(400).
					LetterSpacing(size * 0.38 * 0.14).TextColor(birdAnswer)
			})
		})
		return
	}
	reduce := c.Preferences().ReduceMotion
	if a.pl.Playing() {
		a.birdNextFrame(c, lines, pos, reduce)
	}
	cur := lyrics.Index(lines, pos)
	active := birdActive(lines, cur, pos, reduce)
	box.Children(func() {
		if frac, _ := birdHair(lines, cur, pos, reduce); frac > 0 {
			ui.Column(c.Key("rest")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Children(func() {
				ui.Box(c).Width(w * 0.32 * frac).Height(1).Background(birdRest(accent))
			})
		}
		if active > 0 {
			a.birdLine(c, &lines[active-1], &lines[active], pos, w, reduce)
		}
		if active >= 0 {
			var next *lyrics.Line
			if active+1 < len(lines) {
				next = &lines[active+1]
			}
			a.birdLine(c, &lines[active], next, pos, w, reduce)
		}
	})
}

// birdSize is the line's size: below 流光's, and never so small it disappears.
func birdSize(w float32) float32 { return min(56, max(32, w*0.04)) }

// birdActive is the line forming or sung at pos. Condensing starts before
// the line's time, so the incoming line is current once it has begun.
func birdActive(lines []lyrics.Line, cur int, pos time.Duration, reduce bool) int {
	next := cur + 1
	if next >= 0 && next < len(lines) {
		start, _ := birdClockOf(&lines[next], reduce).condense()
		if pos >= start {
			return next
		}
	}
	return cur
}

// birdLine draws l, and next the line after it, as they are at pos. The
// line stays in the middle; its reading and its translation hang below
// that, so turning them on or off does not move the words. Before a long
// break the line leaves with the break, and otherwise as the next one
// forms. The line before the one sung used to be timed as if no break had
// come, and came back, faint, as the next line formed.
func (a *app) birdLine(c *ui.Context, l, next *lyrics.Line, pos time.Duration, w float32, reduce bool) {
	clock := birdClockOf(l, reduce)
	long := next == nil || next.At-l.End > 4*time.Second
	text := l.Text
	cjk := birdCJK(text)
	font := birdFontZH
	if l.Lang == "ja" {
		font = birdFontJA
	}
	avail := w * 0.8
	size := birdSize(w)
	size, spacing, lineW, two := birdFit(c, text, font, size, avail, cjk)
	op, blur, dy, gone := birdLook(clock, l, next, pos, size, reduce, long)
	if op < 0.004 {
		return
	}
	ui.Box(c.Key(l.At)).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Children(func() {
		words := ui.Column(c.Key("words")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().
			Translate(0, dy).Opacity(op)
		if !reduce && blur > 0.05 {
			words.Blur(blur)
		}
		words.Children(func() {
			ui.Text(c, text).Font(font).Lang(l.Lang).FontSize(size).FontWeight(400).
				LetterSpacing(spacing).LineHeight(1.3).MaxLines(2).MaxWidth(avail).
				TextAlign(ui.Center).TextColor(birdInk)
		})
		a.birdCaptions(c, l, pos, clock, size, lineW, dy, blur, op, gone, two, reduce)
	})
}

// birdCaptions hangs l's reading and translation under the line, the
// reading with the line and the translation late, from the right.
func (a *app) birdCaptions(c *ui.Context, l *lyrics.Line, pos time.Duration, clock birdClock, size, lineW, dy, blur, op, gone float32, two, reduce bool) {
	reading, translation := a.lyricReading(*l), a.lyricTranslation(*l)
	if reading == "" && translation == "" {
		return
	}
	half := size * 1.3 / 2
	if two {
		half = size * 1.3
	}
	gap := size * 0.6
	block := float32(0)
	if reading != "" {
		gap = size * 0.35
		block += max(13, size*0.32) * 1.2
	}
	if translation != "" {
		if reading != "" {
			block += 4
		}
		block += max(14, size*0.38) * 1.3
	}
	k2, ax, ablur := birdAnswerMotion(clock, pos, reduce)
	ui.Column(c.Key("cap")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().
		Translate(0, half+gap+block/2+dy).Children(func() {
		if reading != "" {
			rd := ui.Text(c, reading).FontSize(max(13, size*0.32)).FontWeight(400).
				TextAlign(ui.Center).TextColor(birdBreath).MaxWidth(lineW).Opacity(op)
			if !reduce && blur > 0.05 {
				rd.Blur(blur)
			}
		}
		if translation != "" {
			// The answer's left edge sits in from the line's left.
			ui.Row(c).Width(max(lineW, 1)).Children(func() {
				ui.Box(c).Width(lineW * 0.14)
				tr := ui.Text(c, translation).Font(birdFontZH).FontSize(max(14, size*0.38)).FontWeight(400).
					LetterSpacing(max(14, size*0.38)*0.14).LineHeight(1.3).MaxLines(2).MaxWidth(lineW*0.86).
					TextColor(birdAnswer).Opacity(k2*(1-gone)).Translate(ax, 0)
				if !reduce && ablur > 0.05 {
					tr.Blur(ablur)
				}
			})
		}
	})
}

// birdLook is how far l has formed, and how far it has left, at pos.
// opacity, blur and dy are the line's; gone is how far the leaving has run.
func birdLook(clock birdClock, l, next *lyrics.Line, pos time.Duration, size float32, reduce, long bool) (opacity, blur, dy, gone float32) {
	start, end := clock.condense()
	k := birdSmooth(birdSpan(pos, start, end))
	if reduce {
		k = birdSpan(pos, start, end)
	}
	if ls, le, ok := birdLeave(l, next, long, reduce); ok {
		gone = easeOutCubic(birdSpan(pos, ls, le))
	}
	opacity = k * (1 - gone)
	if reduce || gone > 0 {
		dy = 0
		if !reduce {
			blur = 4 * gone
		}
		return opacity, blur, dy, gone
	}
	blur = 9 * (1 - k) * (size / 48)
	dy = 3 * (1 - k)
	return opacity, blur, dy, gone
}

func birdAnswerMotion(clock birdClock, pos time.Duration, reduce bool) (k, x, blur float32) {
	start, _ := clock.condense()
	start += 400 * time.Millisecond
	dur := 900 * time.Millisecond
	if reduce {
		dur = 300 * time.Millisecond
	}
	t := birdSpan(pos, start, start+dur)
	k = easeOutCubic(t)
	if reduce {
		return k, 0, 0
	}
	return k, 36 * (1 - k), 1.5 * (1 - k)
}

// birdClock is when a line forms: s the first word, e the line's end,
// D how long it condenses, L how early it begins.
type birdClock struct {
	s, e, D, L time.Duration
}

func birdClockOf(l *lyrics.Line, reduce bool) birdClock {
	s := l.At
	if len(l.Words) > 0 {
		s = l.Words[0].Start
	}
	span := l.End - s
	if span < 0 {
		span = 0
	}
	d := time.Duration(float64(span) * 0.45)
	d = clampDur(d, 600*time.Millisecond, 1800*time.Millisecond)
	lead := min(300*time.Millisecond, d/3)
	if reduce {
		d = 300 * time.Millisecond
	}
	return birdClock{s: s, e: l.End, D: d, L: lead}
}

func (b birdClock) condense() (start, end time.Duration) {
	start = b.s - b.L
	return start, start + b.D
}

// birdLeave is when l fades out: over the first of a long break, or as the
// next line begins to form. ok is false when it never leaves.
func birdLeave(l, next *lyrics.Line, long, reduce bool) (start, end time.Duration, ok bool) {
	dur := 450 * time.Millisecond
	if reduce {
		dur = 300 * time.Millisecond
	}
	if long {
		start = l.End + 800*time.Millisecond
		if reduce {
			dur = 300 * time.Millisecond
		} else {
			dur = 700 * time.Millisecond
		}
		return start, start + dur, true
	}
	if next == nil {
		return 0, 0, false
	}
	at, _ := birdClockOf(next, reduce).condense()
	start = at - 150*time.Millisecond
	return start, start + dur, true
}

// birdHair is how far the break's hairline has grown, from 0 to 1, and
// whether it is still growing or fading. A short gap has none.
func birdHair(lines []lyrics.Line, cur int, pos time.Duration, reduce bool) (float32, bool) {
	if len(lines) == 0 {
		return 0, false
	}
	if cur < 0 {
		return introHair(pos, &lines[0], reduce)
	}
	if cur+1 < len(lines) && lines[cur+1].At-lines[cur].End <= 4*time.Second {
		return 0, false
	}
	var next *lyrics.Line
	if cur+1 < len(lines) {
		next = &lines[cur+1]
	}
	return breakHair(&lines[cur], next, pos, reduce)
}

func introHair(pos time.Duration, first *lyrics.Line, reduce bool) (float32, bool) {
	growEnd := 800 * time.Millisecond
	g := float32(1)
	moving := false
	if pos < growEnd {
		g = easeOutCubic(float32(pos) / float32(growEnd))
		moving = true
	}
	end, _ := birdClockOf(first, reduce).condense()
	start := end - 400*time.Millisecond
	if pos >= start {
		g *= 1 - birdSpan(pos, start, end)
		moving = pos < end
	}
	return g, moving
}

func breakHair(l, next *lyrics.Line, pos time.Duration, reduce bool) (float32, bool) {
	_, left, _ := birdLeave(l, nil, true, reduce)
	growStart := left + 400*time.Millisecond
	growEnd := growStart + 800*time.Millisecond
	if pos < growStart {
		return 0, false
	}
	g := float32(1)
	moving := false
	if pos < growEnd {
		g = easeOutCubic(birdSpan(pos, growStart, growEnd))
		moving = true
	}
	if next != nil {
		end, _ := birdClockOf(next, reduce).condense()
		start := end - 400*time.Millisecond
		if pos >= start {
			g *= 1 - birdSpan(pos, start, end)
			moving = pos < end
		}
	}
	return g, moving
}

// birdNextFrame asks for a frame while 青鸟 is changing, and otherwise
// waits until the next of those moments. A pause asks for nothing.
func (a *app) birdNextFrame(c *ui.Context, lines []lyrics.Line, pos time.Duration, reduce bool) {
	busy, wait := a.birdSchedule(lines, pos, reduce)
	if busy {
		c.AnimationFrame()
		return
	}
	if wait > 0 {
		c.After(wait)
	}
}

func (a *app) birdSchedule(lines []lyrics.Line, pos time.Duration, reduce bool) (busy bool, wait time.Duration) {
	wait = -1
	note := func(t time.Duration) {
		if t <= pos {
			return
		}
		if d := t - pos; wait < 0 || d < wait {
			wait = d
		}
	}
	mark := func(start, end time.Duration) {
		if end <= start {
			return
		}
		if pos >= start && pos < end {
			busy = true
		}
		note(start)
		note(end)
	}
	if len(lines) > 0 && pos < lines[0].At {
		mark(0, 800*time.Millisecond)
		end, _ := birdClockOf(&lines[0], reduce).condense()
		mark(end-400*time.Millisecond, end)
	}
	for i := range lines {
		l := &lines[i]
		clock := birdClockOf(l, reduce)
		cs, ce := clock.condense()
		mark(cs, ce)
		if a.lyricTranslation(*l) != "" {
			as, _ := clock.condense()
			as += 400 * time.Millisecond
			dur := 900 * time.Millisecond
			if reduce {
				dur = 300 * time.Millisecond
			}
			mark(as, as+dur)
		}
		var next *lyrics.Line
		if i+1 < len(lines) {
			next = &lines[i+1]
		}
		long := next == nil || next.At-l.End > 4*time.Second
		if ls, le, ok := birdLeave(l, next, long, reduce); ok {
			mark(ls, le)
		}
		if long {
			_, left, _ := birdLeave(l, nil, true, reduce)
			gs := left + 400*time.Millisecond
			mark(gs, gs+800*time.Millisecond)
			if next != nil {
				end, _ := birdClockOf(next, reduce).condense()
				mark(end-400*time.Millisecond, end)
			}
		}
	}
	if wait < 0 {
		wait = 0
	}
	return busy, wait
}

func birdFit(c *ui.Context, text, font string, size, avail float32, cjk bool) (fit, spacing, width float32, two bool) {
	em := float32(0.01)
	if cjk {
		em = 0.10
	}
	spacing = size * em
	tw, _ := c.MeasureText(0, ui.Span{Text: text, Font: font, Size: size, Weight: 400, LetterSpacing: spacing})
	if tw > avail*2 && tw > 0 {
		size *= max(0.5, avail*2/tw)
		spacing = size * em
		tw, _ = c.MeasureText(0, ui.Span{Text: text, Font: font, Size: size, Weight: 400, LetterSpacing: spacing})
	}
	two = tw > avail
	if tw > avail {
		tw = avail
	}
	return size, spacing, max(tw, 1), two
}

func birdCJK(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			return true
		}
	}
	return false
}

func birdSpan(pos, start, end time.Duration) float32 {
	if end <= start {
		if pos >= end {
			return 1
		}
		return 0
	}
	return clamp01(float32(pos-start) / float32(end-start))
}

func clampDur(d, lo, hi time.Duration) time.Duration {
	if d < lo {
		return lo
	}
	if d > hi {
		return hi
	}
	return d
}

func birdSmooth(t float32) float32 {
	t = clamp01(t)
	return t * t * (3 - 2*t)
}
