package app

import (
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/linebreak"
	"netease-music-mygo/internal/lyrics"
)

// 和鸣 sets a line as an ensemble plays: hard and bright against the dark,
// lit from behind (backdrop_backlit.go), each line pulled into focus as the
// one before goes out of it, one brass gold on the word being sung, and its
// reading and translation in unison with it, under it on its axis. It is
// 青鸟's sibling: there the translation answers late and off to one side;
// here every part sounds together. It shares the stage, with its hot
// comments, interludes and next line, and sets its own captions.

// 和鸣's colors. The captions read at 4.5:1 or more over its backdrop
// (TestCaptionsReadOverTheBackdrop).
var (
	chorusInk     = white
	chorusGold    = ui.Hex("#E8BC5A")
	chorusReading = white.Alpha(0.84)
	chorusAnswer  = ui.Hex("#FFF3E2").Alpha(0.90)
)

const (
	// chorusLead is how long before its first word a line starts to come
	// into focus, and chorusPull how long the focus takes to come to it.
	chorusLead = 150 * time.Millisecond
	chorusPull = 450 * time.Millisecond
	// A word turns gold over chorusGoldIn as it is sung, and back over
	// chorusGoldOut once it has been.
	chorusGoldIn  = 120 * time.Millisecond
	chorusGoldOut = 300 * time.Millisecond
)

// chorusSize is a line's size on a stage w wide.
func chorusSize(w float32) float32 { return min(60, max(34, w*0.046)) }

// chorusIn is when l comes into focus: from a little before its first word.
func chorusIn(l *lyrics.Line) (start, end time.Duration) {
	s := l.At
	if len(l.Words) > 0 {
		s = l.Words[0].Start
	}
	start = s - chorusLead
	return start, start + chorusPull
}

// chorusWordEnd is when the i-th word of l stops being the word sung: as
// it ends, held for at least 0.15 s, and no later than the next one starts.
func chorusWordEnd(l *lyrics.Line, i int) time.Duration {
	w := l.Words[i]
	end := max(w.End, w.Start+150*time.Millisecond)
	if i+1 < len(l.Words) && l.Words[i+1].Start > w.Start {
		end = min(end, l.Words[i+1].Start)
	}
	return end
}

// chorusGoldOf is how gold each word of l is at pos: the word being sung,
// on a line timed word by word; none on a line timed only as a whole, whose
// words' times are guesses.
func chorusGoldOf(l *lyrics.Line, pos time.Duration, reduce bool) []float32 {
	gold := make([]float32, len(l.Words))
	if !l.Timed {
		return gold
	}
	for i, w := range l.Words {
		end := chorusWordEnd(l, i)
		switch {
		case pos < w.Start || strings.TrimSpace(w.Text) == "":
		case pos < end:
			gold[i] = 1
			if !reduce {
				gold[i] = clamp01(float32(pos-w.Start) / float32(chorusGoldIn))
			}
		case !reduce:
			gold[i] = 1 - clamp01(float32(pos-end)/float32(chorusGoldOut))
		}
	}
	return gold
}

// chorusRun is a run of a line's text in one style: a word of it, or a
// part of one where a row breaks inside it, or the break (word -1).
type chorusRun struct {
	text string
	word int
}

// chorusRuns cuts words into runs at starts, where rows start in their
// text, putting a break there and leaving out the spaces before it, so
// that each row centres on its words.
func chorusRuns(words []lyrics.Word, starts []int) []chorusRun {
	var runs []chorusRun
	at, next := 0, 0 // where the word starts in the text, and the next start
	for i, w := range words {
		text, from := w.Text, at
		at += len(w.Text)
		for next < len(starts) && starts[next] < at {
			cut := max(0, starts[next]-from)
			runs = append(runs, chorusRun{text[:cut], i}, chorusRun{"\n", -1})
			text, from = text[cut:], from+cut
			next++
		}
		runs = append(runs, chorusRun{text, i})
	}
	for i := range runs {
		if runs[i].word >= 0 || runs[i].text != "\n" {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			runs[j].text = strings.TrimRightFunc(runs[j].text, unicode.IsSpace)
			if runs[j].text != "" {
				break
			}
		}
	}
	out := runs[:0]
	for _, r := range runs {
		if r.text != "" {
			out = append(out, r)
		}
	}
	return out
}

// chorusLine draws l on 和鸣 as it is at.pos: coming into focus, sung, or,
// as at.fade falls, going out of focus as the next one comes.
func (a *app) chorusLine(c *ui.Context, l *lyrics.Line, at stageAt) {
	reduce := c.Preferences().ReduceMotion
	words := l.Words
	if len(words) == 0 {
		words = []lyrics.Word{{Text: l.Text, Start: l.At, End: l.End}}
	}
	var text strings.Builder
	for _, w := range words {
		text.WriteString(w.Text)
	}
	size := chorusSize(at.w)
	avail := at.w * 0.8
	measure := func(size float32) ([]int, float32, int) {
		span := ui.Span{Size: size, Weight: 700, Lang: l.Lang}
		width := func(s string) float32 {
			span.Text = s
			w, _ := c.MeasureText(0, span)
			return w
		}
		pieces := linebreak.Phrases(text.String(), l.Lang)
		rows := linebreak.Rows(pieces, 2, avail, width)
		widest := float32(0)
		for _, r := range rows {
			widest = max(widest, width(r))
		}
		return linebreak.Starts(pieces, 2, avail, width), widest, len(rows)
	}
	starts, widest, rows := measure(size)
	if widest > avail {
		size *= max(0.6, avail/widest)
		starts, widest, rows = measure(size)
	}

	inStart, inEnd := chorusIn(l)
	k := easeOutCubic(birdSpan(at.pos, inStart, inEnd))
	out := 1 - at.fade // as the next line comes in
	opacity := k * at.fade
	if opacity < 0.004 {
		return
	}
	blur, scale := float32(0), float32(1)
	if !reduce {
		// The focus comes to it from in front, and leaves it for behind.
		blur = 12 * size / 48 * max(1-k, out)
		scale = 1 + 0.04*(1-k) - 0.04*out
	}
	gold := chorusGoldOf(&lyrics.Line{Words: words, Timed: l.Timed}, at.pos, reduce)
	ui.Box(c.Key(lineKey(l))).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Children(func() {
		line := ui.Column(c.Key("words")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().
			Opacity(opacity).Scale(scale, scale)
		if blur > 0.05 {
			line.Blur(blur)
		}
		line.Children(func() {
			ui.RichText(c).Lang(l.Lang).FontSize(size).FontWeight(700).LineHeight(1.25).NoWrap().
				TextAlign(ui.Center).Label(l.Text).Children(func() {
				for i, r := range chorusRuns(words, starts) {
					color := chorusInk
					if r.word >= 0 && gold[r.word] > 0 {
						color = chorusInk.Mix(chorusGold, gold[r.word])
					}
					ui.Text(c.Key(i), r.text).TextColor(color)
				}
			})
		})
		a.chorusCaptions(c, l, size, float32(rows)*size*1.25/2, at.w, opacity, blur)
	})
}

// chorusCaptions sets l's reading and translation under its line, half
// high from the middle, as the line is: in focus with it, on its axis, the
// reading close under it and the translation a group of its own.
func (a *app) chorusCaptions(c *ui.Context, l *lyrics.Line, size, half, w, opacity, blur float32) {
	reading, translation := a.lyricReading(*l), a.lyricTranslation(*l)
	if reading == "" && translation == "" {
		return
	}
	wide := w * 0.6
	rSize, tSize := max(15, size*0.34), max(18, size*0.38)
	var rRows, tRows string
	block, first := float32(0), size*0.35
	if reading != "" {
		rRows, _ = breakRows(c, reading, "", 2, wide, ui.Span{Size: rSize, Weight: 500})
		block += float32(strings.Count(rRows, "\n")+1) * rSize * 1.2
		first = size * 0.1
	}
	if translation != "" {
		tRows, _ = breakRows(c, translation, "zh-Hans", 2, wide, ui.Span{Size: tSize, Weight: 600})
		if reading != "" {
			block += size * 0.35
		}
		block += float32(strings.Count(tRows, "\n")+1) * tSize * 1.3
	}
	caps := ui.Column(c.Key("cap")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Gap(size*0.35).
		Translate(0, half+first+block/2).Opacity(opacity)
	if blur > 0.05 {
		caps.Blur(blur)
	}
	caps.Children(func() {
		if reading != "" {
			ui.Text(c, rRows).FontSize(rSize).FontWeight(500).LineHeight(1.2).NoWrap().
				TextAlign(ui.Center).TextColor(chorusReading)
		}
		if translation != "" {
			ui.Text(c, tRows).Lang("zh-Hans").FontSize(tSize).FontWeight(600).LineHeight(1.3).NoWrap().
				TextAlign(ui.Center).TextColor(chorusAnswer)
		}
	})
}

// chorusFrames asks for a frame at once while a line comes into focus or
// leaves it, or a word turns gold or back, and otherwise waits for the
// next of those: a word held, or a line formed, asks for none.
func (a *app) chorusFrames(c *ui.Context, lines []lyrics.Line, cur int, pos time.Duration) {
	if len(lines) == 0 {
		return
	}
	// The interlude's dots breathe, a few times a second.
	if cur < 0 && pos < lines[0].At-chorusLead ||
		cur >= 0 && cur+1 < len(lines) && pos > lines[cur].End+time.Second && lines[cur+1].At-lines[cur].End > 4*time.Second {
		c.After(min(100*time.Millisecond, max(time.Millisecond, lines[min(cur+1, len(lines)-1)].At-chorusLead-pos)))
		return
	}
	busy, wait := false, time.Duration(-1)
	note := func(t time.Duration) {
		if t > pos && (wait < 0 || t-pos < wait) {
			wait = t - pos
		}
	}
	mark := func(start, end time.Duration) {
		if pos >= start && pos < end {
			busy = true
		}
		note(start)
		note(end)
	}
	for i := max(cur-1, 0); i <= min(cur+1, len(lines)-1); i++ {
		l := &lines[i]
		mark(chorusIn(l))
		mark(l.At, l.At+400*time.Millisecond) // the line before going
		if !l.Timed {
			continue
		}
		for j, w := range l.Words {
			end := chorusWordEnd(l, j)
			mark(w.Start, w.Start+chorusGoldIn)
			mark(end, end+chorusGoldOut)
		}
	}
	if busy {
		c.AnimationFrame()
	} else if wait > 0 {
		c.After(wait)
	}
}
