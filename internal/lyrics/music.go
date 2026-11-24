package lyrics

import (
	"strings"
	"time"

	"netease-music-mygo/internal/music"
)

// What a source gives, in music.Lyrics, and what the views read, as Lines,
// go from one to the other here.

// FromMusic makes the lines to show of what a source gave: the timed Lines,
// where there are any, else its LRC, with the translation and the reading
// the sources give as LRC too. Lines that carry no words share their time
// evenly, as the lines of an LRC do.
func FromMusic(l *music.Lyrics) []Line {
	if l == nil {
		return nil
	}
	if len(l.Lines) == 0 {
		return Build(Source{LRC: l.LRC, Translation: l.Translation, Reading: l.Reading})
	}
	ms := func(n int64) time.Duration { return time.Duration(n) * time.Millisecond }
	lines := make([]Line, len(l.Lines))
	for i, ml := range l.Lines {
		line := Line{At: ms(ml.Start), End: ms(ml.End), Text: ml.Text, Translation: ml.Translation, Reading: ml.Reading}
		if line.Text == "" {
			var b strings.Builder
			for _, w := range ml.Words {
				b.WriteString(w.Text)
			}
			line.Text = b.String()
		}
		if line.End <= line.At {
			// As an LRC's lines end: with the next, or in six seconds.
			line.End = line.At + 6*time.Second
			if i+1 < len(l.Lines) {
				line.End = min(line.End, ms(l.Lines[i+1].Start))
			}
		}
		if len(ml.Words) > 0 {
			line.Timed = true
			for _, w := range ml.Words {
				line.Words = append(line.Words, Word{Text: w.Text, Start: ms(w.Start), End: ms(w.End)})
			}
		} else {
			line.Words = SpreadWords(line.Text, line.At, line.End)
		}
		lines[i] = line
	}
	setLang(lines)
	return lines
}

// ToMusic is lines as a source gives them: timed Lines, in which the words of
// a line are kept only where they carry their own times, for the views to
// share the time of the others evenly. FromMusic of it is the lines again.
func ToMusic(lines []Line) *music.Lyrics {
	out := &music.Lyrics{Lines: make([]music.LyricLine, len(lines))}
	for i, l := range lines {
		ml := music.LyricLine{
			Start: l.At.Milliseconds(), End: l.End.Milliseconds(),
			Text: l.Text, Translation: l.Translation, Reading: l.Reading,
		}
		if l.Timed {
			for _, w := range l.Words {
				ml.Words = append(ml.Words, music.LyricWord{Start: w.Start.Milliseconds(), End: w.End.Milliseconds(), Text: w.Text})
			}
		}
		out.Lines[i] = ml
	}
	return out
}
