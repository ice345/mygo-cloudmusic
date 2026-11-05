package lyrics

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// A Line is a line of timed lyrics, with its translation if any, and
// when each of its words is sung.
type Line struct {
	At          time.Duration
	End         time.Duration
	Text        string
	Translation string
	Words       []Word
	// Reading is how the line is read aloud, where NetEase sends one that
	// says more than the line. A reading of a line in Latin, or one that
	// only repeats the line, is left empty.
	Reading string
	// Timed says the words carry their own times, as NetEase's word-timed
	// lyrics give them; else they share the line's time evenly.
	Timed bool
}

// A Word is a word of a line, or a character of Chinese or Japanese,
// with when it is sung.
type Word struct {
	Text       string
	Start, End time.Duration
}

var lrcTime = regexp.MustCompile(`\[(\d+):(\d+)(?:[.:](\d+))?\]`)

// parseLRC reads lyrics in the LRC format, as NetEase serves them: a line
// may carry several times, and credits come as lines of their own.
func parseLRC(lrc string) []Line {
	var out []Line
	for _, raw := range strings.Split(lrc, "\n") {
		times := lrcTime.FindAllStringSubmatchIndex(raw, -1)
		if len(times) == 0 {
			continue
		}
		text := strings.TrimSpace(raw[times[len(times)-1][1]:])
		for _, m := range times {
			min, _ := strconv.Atoi(raw[m[2]:m[3]])
			sec, _ := strconv.Atoi(raw[m[4]:m[5]])
			var frac time.Duration
			if m[6] >= 0 {
				f := raw[m[6]:m[7]]
				n, _ := strconv.Atoi(f)
				frac = time.Duration(n) * time.Second
				for range len(f) {
					frac /= 10
				}
			}
			at := time.Duration(min)*time.Minute + time.Duration(sec)*time.Second + frac
			out = append(out, Line{At: at, Text: text})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

var (
	yrcLine = regexp.MustCompile(`^\[(\d+),(\d+)\]`)
	yrcWord = regexp.MustCompile(`\((\d+),(\d+),\d+\)([^(]*)`)
)

// parseYRC reads NetEase's word-timed lyrics: a line is
// "[start,length](start,length,0)word(start,length,0)word…", in
// milliseconds. Credits come as lines of JSON, which it leaves out.
func parseYRC(yrc string) []Line {
	ms := func(s string) time.Duration {
		n, _ := strconv.Atoi(s)
		return time.Duration(n) * time.Millisecond
	}
	var out []Line
	for _, raw := range strings.Split(yrc, "\n") {
		m := yrcLine.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		l := Line{At: ms(m[1]), Timed: true}
		l.End = l.At + ms(m[2])
		for _, w := range yrcWord.FindAllStringSubmatch(raw[len(m[0]):], -1) {
			start := ms(w[1])
			word := Word{Text: w[3], Start: start, End: start + ms(w[2])}
			if strings.TrimSpace(word.Text) == "" {
				// A space between words: it belongs to the word before.
				if n := len(l.Words); n > 0 {
					l.Words[n-1].Text += " "
				}
				continue
			}
			l.Words = append(l.Words, word)
		}
		for _, w := range l.Words {
			l.Text += w.Text
		}
		l.Text = strings.TrimSpace(l.Text)
		if l.Text != "" {
			out = append(out, l)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

// Source is a song's lyrics as NetEase serves them (netease.LyricTexts).
type Source struct {
	// LRC is the timed lines; Translation and Reading are timed by them.
	LRC, Translation, Reading string
	// YRC is the word-timed lines. They may merge or split LRC's lines;
	// YTranslation and YReading are timed by YRC's own.
	YRC, YTranslation, YReading string
}

// Build makes the lines to show from what NetEase serves: the
// word-timed lyrics where there are some, else the timed lines, with the
// translation and the reading of each. A translation written against a
// timed line still reaches the word-timed line that sings it, and two
// that one line sings are joined, in the order they were written.
func Build(s Source) []Line {
	var lines []Line
	wordTimed := false
	if s.YRC != "" {
		lines = parseYRC(s.YRC)
		wordTimed = len(lines) > 0
	}
	if len(lines) == 0 {
		for _, l := range parseLRC(s.LRC) {
			if l.Text != "" {
				lines = append(lines, l)
			}
		}
		for i := range lines {
			end := lines[i].At + 6*time.Second
			if i+1 < len(lines) {
				end = min(end, lines[i+1].At)
			}
			lines[i].End = end
			lines[i].Words = SpreadWords(lines[i].Text, lines[i].At, end)
		}
	}
	// On word-timed lines the texts timed by them come first. A line they
	// leave empty takes the text of the timed lines it sings. A Han-only
	// line keeps the timed lines' reading where it has one: for Cantonese
	// that is the Jyutping, and the word-timed reading is a different spelling.
	var tr, rd []string
	if wordTimed {
		tr, rd = extras(lines, s.YTranslation), extras(lines, s.YReading)
	}
	ltr, lrd := extras(lines, s.Translation), extras(lines, s.Reading)
	for i := range lines {
		t, r := "", ""
		if wordTimed {
			t, r = tr[i], rd[i]
		}
		if t == "" {
			t = ltr[i]
		}
		if hanOnly(lines[i].Text) {
			if lrd[i] != "" {
				r = lrd[i]
			}
		} else if r == "" {
			r = lrd[i]
		}
		if t != "" && !echoes(lines[i].Text, t) {
			lines[i].Translation = t
		}
		if r != "" && spoken(lines[i].Text) && !echoes(lines[i].Text, r) {
			lines[i].Reading = r
		}
	}
	return lines
}

// echoes reports whether reading says no more than text: the same letters
// and digits, whatever their case, spacing and punctuation, as NetEase's
// reading of an English line often is.
func echoes(text, reading string) bool {
	return foldLetters(text) == foldLetters(reading)
}

// foldLetters returns the letters and digits of s, in lower case.
func foldLetters(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// spoken reports whether text has letters a reading can say: any outside
// the Latin script.
func spoken(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) && !unicode.Is(unicode.Latin, r) {
			return true
		}
	}
	return false
}

// hanOnly reports whether text's letters are Han characters and nothing else.
func hanOnly(text string) bool {
	han := false
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			han = true
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			return false
		}
	}
	return han
}

// belongs returns the line an extra line at at belongs to, or -1: of the
// two lines starting nearest it, no later than half a second after it,
// the nearer that is still sung then. Word-timed lines start a few
// hundred milliseconds off the timed lines extras are written against,
// and some of them sing two.
func belongs(lines []Line, at time.Duration) int {
	i := sort.Search(len(lines), func(i int) bool { return lines[i].At > at+500*time.Millisecond }) - 1
	best, bestGap := -1, time.Duration(0)
	for _, k := range []int{i, i - 1} {
		if k < 0 || k >= len(lines) {
			continue
		}
		if lines[k].At-500*time.Millisecond > at || at >= lines[k].End+200*time.Millisecond {
			continue
		}
		gap := lines[k].At - at
		if gap < 0 {
			gap = -gap
		}
		if best < 0 || gap < bestGap {
			best, bestGap = k, gap
		}
	}
	return best
}

// extras returns, for each of lines, the text of extra's timed lines that
// belong to it, joined by spaces in their order. Lines of no letters or
// digits, as "//", are none.
func extras(lines []Line, extra string) []string {
	out := make([]string, len(lines))
	for _, t := range parseLRC(extra) {
		if !textual(t.Text) {
			continue
		}
		i := belongs(lines, t.At)
		if i < 0 {
			continue
		}
		if out[i] != "" {
			out[i] += " "
		}
		out[i] += t.Text
	}
	return out
}

// textual reports whether s has a letter or a digit.
func textual(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// mergeLyrics pairs timed lines with their translations, without words'
// own times.
func mergeLyrics(lrc, translation string) []Line {
	return Build(Source{LRC: lrc, Translation: translation})
}

// SpreadWords splits text into words, each Chinese, Japanese or Korean
// character one of them, and shares the time from start to end among them
// by their lengths, for lines whose words have no times of their own.
func SpreadWords(text string, start, end time.Duration) []Word {
	var words []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			if cur.Len() > 0 {
				cur.WriteRune(' ')
				flush()
			}
		case isCJK(r):
			flush()
			words = append(words, string(r))
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	if len(words) == 0 {
		return nil
	}
	total := 0
	for _, w := range words {
		total += utf8.RuneCountInString(strings.TrimSpace(w))
	}
	// Singing takes most of the line, not the pause before the next.
	span := time.Duration(float64(end-start) * 0.85)
	out := make([]Word, len(words))
	at := start
	for i, w := range words {
		d := span * time.Duration(utf8.RuneCountInString(strings.TrimSpace(w))) / time.Duration(max(total, 1))
		out[i] = Word{Text: w, Start: at, End: at + d}
		at += d
	}
	return out
}

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

// Index returns the line sung at pos, or -1 before the first.
func Index(lines []Line, pos time.Duration) int {
	i := sort.Search(len(lines), func(i int) bool { return lines[i].At > pos })
	return i - 1
}

// A Quiet is a stretch of a song with nothing sung: its intro, an
// interlude, or its outro.
type Quiet struct {
	From, To time.Duration
}

// Quiets returns the stretches of a song length long, of at least least,
// in which none of lines is sung. Credits, as 作词 and 作曲, are not sung.
func Quiets(lines []Line, length, least time.Duration) []Quiet {
	var out []Quiet
	add := func(from, to time.Duration) {
		if to-from >= least {
			out = append(out, Quiet{from, to})
		}
	}
	at := time.Duration(0)
	sung := false
	for _, l := range lines {
		if IsCredit(l.Text) {
			continue
		}
		add(at, l.At)
		at, sung = max(at, l.End), true
	}
	if sung && length > 0 {
		add(at, length)
	}
	return out
}

// creditNames are what credit lines begin with.
var creditNames = []string{"作词", "作曲", "编曲", "制作", "监制", "混音", "录音", "母带", "和声", "吉他", "贝斯", "鼓", "键盘", "弦乐", "出品", "发行", "OP", "SP", "词", "曲", "Lyrics", "Music", "Composer", "Arranger", "Producer"}

// IsCredit reports whether a line names who made the song, as "作词 :
// 林夕", rather than being sung.
func IsCredit(text string) bool {
	i := strings.IndexAny(text, ":：")
	if i <= 0 || utf8.RuneCountInString(text[:i]) > 12 {
		return false
	}
	head := strings.TrimSpace(text[:i])
	for _, n := range creditNames {
		// A name of one character, as 词, is the whole of the head.
		if head == n || utf8.RuneCountInString(n) > 1 && strings.HasPrefix(head, n) {
			return true
		}
	}
	return false
}
