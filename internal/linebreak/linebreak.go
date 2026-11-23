// Package linebreak breaks a line of lyrics, or its translation, into a
// few even rows, never inside a word. MyGo breaks text wherever it runs
// out of room, which in Japanese and Chinese is between any two
// characters, so a row could end inside a word: 続 / いていく.
//
// Phrases finds where a row may end: at spaces, which is the only place
// Korean and the languages spaced into words break, and between the
// phrases of Japanese and Chinese, as BudouX finds them. Rows then picks
// from those the breaks that make the rows most even, preferring a break
// after punctuation.
package linebreak

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Phrases splits s into the pieces a row may break between, each keeping
// the spaces after it. lang is the line's language, "ja" for Japanese; a
// run with kana is Japanese whatever lang says, and other Chinese
// characters are read as Chinese.
func Phrases(s, lang string) []string {
	jaModel, zhModel := models()
	var out []string
	s = strings.TrimLeftFunc(s, unicode.IsSpace)
	for s != "" {
		// A run up to its spaces, and those spaces.
		end := strings.IndexFunc(s, unicode.IsSpace)
		if end < 0 {
			end = len(s)
		}
		word, rest := s[:end], strings.TrimLeftFunc(s[end:], unicode.IsSpace)
		spaces := s[end : len(s)-len(rest)]
		s = rest
		pieces := []string{word}
		if script := han(word); script != none {
			m := zhModel
			if script == kana || lang == "ja" {
				m = jaModel
			}
			pieces = keep(m.phrases([]rune(word)))
		}
		pieces[len(pieces)-1] += spaces
		out = append(out, pieces...)
	}
	return out
}

type script int

const (
	none script = iota
	hanzi
	kana
)

// han says whether s has Chinese characters, and whether kana with them.
func han(s string) script {
	found := none
	for _, r := range s {
		switch {
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			return kana
		case unicode.Is(unicode.Han, r):
			found = hanzi
		}
	}
	return found
}

// keep joins pieces a row must not break between: a word of letters or
// digits the model split, as in Loveの, and a piece that may not start a
// row, as a closing mark or a small kana, onto the one before. An opening
// mark goes onto the piece after it.
func keep(pieces []string) []string {
	out := pieces[:0:0]
	for _, p := range pieces {
		if n := len(out); n > 0 {
			last, _ := utf8.DecodeLastRuneInString(out[n-1])
			first, _ := utf8.DecodeRuneInString(p)
			if letter(last) && letter(first) || noStart(first) || noEnd(last) {
				out[n-1] += p
				continue
			}
		}
		out = append(out, p)
	}
	return out
}

// letter is a letter or digit of a script spaced into words, as Latin.
func letter(r rune) bool {
	return (unicode.IsLetter(r) || unicode.IsDigit(r)) &&
		!unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

// noStart is a character a row may not start with, as Japanese sets it:
// closing marks, the iteration and length marks, and small kana.
func noStart(r rune) bool {
	return strings.ContainsRune("、。，．,.！？!?：；:;」』）)】〕〉》”’…‥ー〜～々ゝゞヽヾぁぃぅぇぉっゃゅょゎゕゖァィゥェォッャュョヮヵヶ・", r)
}

// noEnd is a character a row may not end with: opening marks.
func noEnd(r rune) bool {
	return strings.ContainsRune("「『（(【〔〈《“‘", r)
}

// Rows breaks pieces, as Phrases gives them, into at most n rows, each no
// wider than limit where they can be, measured by width: one row if the
// whole fits, else as few as fit, as even as can be. A break after
// punctuation or a space counts as an eighth of the whole narrower, so
// rows end there rather than mid-phrase when it costs little evenness. A
// piece wider than limit on its own is broken between its characters. Rows
// lose the spaces they end with.
func Rows(pieces []string, n int, limit float32, width func(string) float32) []string {
	if len(pieces) == 0 {
		return nil
	}
	if n < 1 {
		n = 1
	}
	var split []string
	for _, p := range pieces {
		if width(strings.TrimRightFunc(p, unicode.IsSpace)) <= limit || utf8.RuneCountInString(p) < 2 {
			split = append(split, p)
			continue
		}
		var chars []string
		for _, r := range p {
			chars = append(chars, string(r))
		}
		split = append(split, keep(chars)...)
	}
	pieces = split

	// The widths of pieces with their spaces, and without them as they
	// end a row: a row is about as wide as its pieces.
	k := len(pieces)
	full, end := make([]float32, k), make([]float32, k)
	strong := make([]bool, k)
	for i, p := range pieces {
		trimmed := strings.TrimRightFunc(p, unicode.IsSpace)
		full[i], end[i] = width(p), width(trimmed)
		last, _ := utf8.DecodeLastRuneInString(trimmed)
		strong[i] = trimmed != p || unicode.IsPunct(last)
	}
	before := make([]float32, k+1)
	for i := range k {
		before[i+1] = before[i] + full[i]
	}
	rowWidth := func(from, to int) float32 { return before[to-1] - before[from] + end[to-1] }
	bonus := rowWidth(0, k) / 8

	// Every way of breaking into rows rows, kept if it fits, or, when none
	// does at n rows, the narrowest.
	best, bestCost := []int(nil), float32(0)
	var try func(cuts []int, from, rows int)
	try = func(cuts []int, from, rows int) {
		if rows == 1 {
			widest, cost := rowWidth(from, k), float32(0)
			prev := 0
			for _, c := range cuts {
				widest = max(widest, rowWidth(prev, c))
				if strong[c-1] {
					cost -= bonus
				}
				prev = c
			}
			if widest > limit {
				cost += 4 * (widest - limit) // over limit only when nothing fits
			}
			cost += widest
			if best == nil || cost < bestCost {
				best, bestCost = append([]int{}, cuts...), cost
			}
			return
		}
		for c := from + 1; c <= k-rows+1; c++ {
			try(append(cuts, c), c, rows-1)
		}
	}
	for rows := 1; rows <= min(n, k); rows++ {
		best = nil
		try(nil, 0, rows)
		widest, prev := float32(0), 0
		for _, c := range append(best, k) {
			widest = max(widest, rowWidth(prev, c))
			prev = c
		}
		if widest <= limit {
			break
		}
	}

	out := make([]string, 0, len(best)+1)
	prev := 0
	for _, c := range append(best, k) {
		out = append(out, strings.TrimRightFunc(strings.Join(pieces[prev:c], ""), unicode.IsSpace))
		prev = c
	}
	return out
}
