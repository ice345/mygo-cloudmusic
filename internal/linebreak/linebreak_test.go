package linebreak

import (
	"reflect"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// chars measures a row as one unit a character, spaces too.
func chars(s string) float32 { return float32(utf8.RuneCountInString(s)) }

// BudouX's own examples come out in the phrases its parser gives, so the
// port scores a gap as the original does.
func TestPhrasesAsBudouX(t *testing.T) {
	for _, c := range []struct {
		s, lang string
		want    []string
	}{
		{"今日は天気です。", "ja", []string{"今日は", "天気です。"}},
		{"今天是晴天。", "", []string{"今天", "是", "晴天。"}},
	} {
		if got := Phrases(c.s, c.lang); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %q, want %q", c.s, got, c.want)
		}
	}
}

// A line too long for one row breaks between phrases, at any width: never
// between a kanji and the kana that end its word, as 続 / いていく did,
// never inside a word of letters, and Korean only at its spaces.
func TestRowsNeverBreakInsideAWord(t *testing.T) {
	for _, c := range []struct{ s, lang string }{
		{"風が長い廊下を渡って夜の向こうまで続いていく", "ja"},
		{"思い出す日々を数えて", "ja"},
		{"君の名前を呼んでいた", "ja"},
		{"Loveの歌を聴かせて", "ja"},
		{"너의 모든 걸 원해 지금 이대로", "ko"},
		{"across the quiet water", ""},
	} {
		pieces := Phrases(c.s, c.lang)
		for max := float32(4); max <= 16; max++ {
			rows := Rows(pieces, 2, max, chars)
			for i := 1; i < len(rows); i++ {
				end, _ := utf8.DecodeLastRuneInString(rows[i-1])
				start, _ := utf8.DecodeRuneInString(rows[i])
				spaced := strings.Contains(c.s, rows[i-1]+" "+string(start))
				switch {
				case unicode.Is(unicode.Han, end) && unicode.Is(unicode.Hiragana, start):
					t.Errorf("%s at %v: %q breaks a word before its kana", c.s, max, rows)
				case (letter(end) || unicode.Is(unicode.Hangul, end)) && !spaced:
					t.Errorf("%s at %v: %q breaks a word where it has no space", c.s, max, rows)
				}
			}
			if got := strings.Join(rows, ""); strings.ReplaceAll(got, " ", "") != strings.ReplaceAll(c.s, " ", "") {
				t.Errorf("%s at %v: rows %q lose or add text", c.s, max, rows)
			}
		}
	}
}

// A line that fits stays one row; one that does not breaks into rows as
// even as its phrases allow, rather than filling the first.
func TestRowsAreEven(t *testing.T) {
	pieces := Phrases("across the quiet water", "")
	if rows := Rows(pieces, 2, 22, chars); len(rows) != 1 {
		t.Errorf("fits in 22: %q", rows)
	}
	if rows := Rows(pieces, 2, 16, chars); !reflect.DeepEqual(rows, []string{"across the", "quiet water"}) {
		t.Errorf("in 16: %q, want across the / quiet water", rows)
	}
}

// A translation too long for one row breaks after its comma when that
// leaves the rows nearly even, rather than in the middle of a phrase.
func TestRowsBreakAfterPunctuation(t *testing.T) {
	pieces := Phrases("风穿过长长的走廊，带着夜里的声音，一直走到听不见的地方", "")
	want := []string{"风穿过长长的走廊，带着夜里的声音，", "一直走到听不见的地方"}
	if rows := Rows(pieces, 2, 20, chars); !reflect.DeepEqual(rows, want) {
		t.Errorf("%q, want %q", rows, want)
	}
}
