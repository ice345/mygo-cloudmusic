package speech

import (
	"net/http"
	"os"
	"testing"
)

// Every library and the voice detector, on every platform, are where the
// app downloads them from. They were on upstream's release, which went
// with it, and every download failed as if the network were down.
func TestNativesLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip("set LIVE to ask GitHub")
	}
	var files []nativeFile
	for _, fs := range libFiles {
		files = append(files, fs...)
	}
	files = append(files, vadFile)
	for _, f := range files {
		res, err := http.Head(nativeURL(f))
		if err != nil {
			t.Errorf("%s: %v", f.asset, err)
			continue
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK || res.ContentLength != f.gzSize {
			t.Errorf("%s: %s, %d bytes, want %d", f.asset, res.Status, res.ContentLength, f.gzSize)
		}
	}
}
