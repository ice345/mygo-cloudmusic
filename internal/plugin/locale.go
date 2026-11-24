package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// languages are those the app speaks besides English, each with a
// locales/<code>.json in a plugin that translated itself: of the English text to
// what it is in that language. The app speaks Chinese.
var languages = []string{"zh-CN"}

var (
	languageMu sync.Mutex
	language   = "zh-CN"
)

// Language is the listener's, as the app speaks it.
func Language() string {
	languageMu.Lock()
	defer languageMu.Unlock()
	return language
}

// SetLanguage says which language the listener has: a plugin that translated
// its texts into it shows them so.
func SetLanguage(code string) {
	languageMu.Lock()
	language = code
	languageMu.Unlock()
}

// readLocales reads the plugin's translations: for each language the app
// speaks but English, locales/<language>.json, such as locales/zh-CN.json, of
// the English text to what it is in that language, as the app's own are.
func readLocales(dir string) (map[string]map[string]string, error) {
	locales := map[string]map[string]string{}
	for _, code := range languages {
		path := filepath.Join(dir, "locales", code+".json")
		b, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var texts map[string]string
		if err := json.Unmarshal(b, &texts); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		locales[code] = texts
	}
	return locales, nil
}
