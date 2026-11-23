package linebreak

import (
	_ "embed"
	"encoding/json"
	"sync"
)

// The phrases of Japanese and Chinese are BudouX's
// (github.com/google/budoux, Apache-2.0): a model of which characters
// around a gap mark the end of a phrase, scored as its parser.py does.
// models/ holds its Japanese and Simplified Chinese models as v0.10.4
// ships them, with its licence.

//go:embed models/ja.json
var jaModel []byte

//go:embed models/zh-hans.json
var zhModel []byte

// A model scores a gap between two characters by the characters around it:
// one at a time (UW), two (BW) and three (TW), at each offset.
type model struct {
	uw    [6]map[string]int
	bw    [3]map[string]int
	tw    [4]map[string]int
	total int // the sum of every score, which the parser halves as its bias
}

var (
	loadOnce sync.Once
	ja, zh   *model
)

func models() (*model, *model) {
	loadOnce.Do(func() {
		ja, zh = parseModel(jaModel), parseModel(zhModel)
	})
	return ja, zh
}

func parseModel(data []byte) *model {
	var groups map[string]map[string]int
	if err := json.Unmarshal(data, &groups); err != nil {
		panic("linebreak: a model does not parse: " + err.Error())
	}
	m := &model{}
	for name, g := range groups {
		for _, v := range g {
			m.total += v
		}
		i := int(name[2] - '1')
		switch name[:2] {
		case "UW":
			m.uw[i] = g
		case "BW":
			m.bw[i] = g
		case "TW":
			m.tw[i] = g
		}
	}
	return m
}

// phrases splits r into BudouX's phrases.
func (m *model) phrases(r []rune) []string {
	if len(r) == 0 {
		return nil
	}
	at := func(g map[string]int, from, to int) int {
		if from < 0 || to > len(r) {
			return 0
		}
		return g[string(r[from:to])]
	}
	var out []string
	start := 0
	for i := 1; i < len(r); i++ {
		score := 0
		for k := range 6 {
			score += at(m.uw[k], i-3+k, i-2+k)
		}
		for k := range 3 {
			score += at(m.bw[k], i-2+k, i+k)
		}
		for k := range 4 {
			score += at(m.tw[k], i-3+k, i+k)
		}
		// The parser breaks where the score passes half the sum of all.
		if 2*score > m.total {
			out = append(out, string(r[start:i]))
			start = i
		}
	}
	return append(out, string(r[start:]))
}
