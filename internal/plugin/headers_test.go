package plugin

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// A plugin's requestHeaders may only name hosts its network permission reaches,
// or the host of a setting, and not * or a host with a path or port; they may
// not set a header that is the request's own, or be for a kind that is none.
func TestRequestHeadersAreChecked(t *testing.T) {
	for name, c := range map[string]struct{ json, want string }{
		"covered":        {`{"id":"x","permissions":{"network":["api.example.com"]},"requestHeaders":[{"hosts":["api.example.com"],"headers":{"x":"1"}}]}`, ""},
		"subdomain":      {`{"id":"x","permissions":{"network":["*.example.com"]},"requestHeaders":[{"hosts":["a.example.com","*.b.example.com"],"headers":{"x":"1"}}]}`, ""},
		"setting host":   {`{"id":"x","requestHeaders":[{"hosts":["{setting:server}"],"headers":{"x":"1"}}]}`, ""},
		"outside":        {`{"id":"x","permissions":{"network":["api.example.com"]},"requestHeaders":[{"hosts":["evil.test"],"headers":{"x":"1"}}]}`, "outside the network permission"},
		"star":           {`{"id":"x","permissions":{"network":["*"]},"requestHeaders":[{"hosts":["*"],"headers":{"x":"1"}}]}`, "isn't a host"},
		"port":           {`{"id":"x","permissions":{"network":["a.com"]},"requestHeaders":[{"hosts":["a.com:80"],"headers":{"x":"1"}}]}`, "isn't a host"},
		"bad setting":    {`{"id":"x","requestHeaders":[{"hosts":["{setting:}"],"headers":{"x":"1"}}]}`, "neither a host"},
		"host header":    {`{"id":"x","permissions":{"network":["a.com"]},"requestHeaders":[{"hosts":["a.com"],"headers":{"Host":"b.com"}}]}`, "isn't a header it can set"},
		"kind":           {`{"id":"x","permissions":{"network":["a.com"]},"requestHeaders":[{"hosts":["a.com"],"kinds":["video"],"headers":{"x":"1"}}]}`, "isn't image or audio"},
		"nothing":        {`{"id":"x","requestHeaders":[{"hosts":[],"headers":{}}]}`, "needs hosts and headers"},
		"wider than all": {`{"id":"x","permissions":{"network":["a.example.com"]},"requestHeaders":[{"hosts":["*.example.com"],"headers":{"x":"1"}}]}`, "outside the network permission"},
	} {
		var m Manifest
		if err := json.Unmarshal([]byte(c.json), &m); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		err := checkRequestHeaders(&m)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: %v", name, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: %v, want %q", name, err, c.want)
		}
	}
}

// What a plugin's requestHeaders give goes with what the app fetches of its
// hosts, of the kinds named, with {setting:key} and {storage:key} as they are
// when the request goes out; a header whose placeholder is of nothing is left
// out, and a host that is the plugin's server only counts while the network
// permission reaches it.
func TestHeadersForFillsPlaceholders(t *testing.T) {
	p, err := newPlugin(writePlugin(t, `{"id":"x","permissions":{"network":["api.example.com","*.cdn.example.com","nas.local"]},
		"requestHeaders":[
			{"hosts":["api.example.com","*.cdn.example.com"],"kinds":["image"],"headers":{"X-Token":"t-{storage:token}","X-Plain":"yes"}},
			{"hosts":["{setting:server}"],"headers":{"Cookie":"session={storage:session}"}}
		]}`, nil), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	image := mustURL(t, "https://img.cdn.example.com/a.jpg")
	if got := p.HeadersFor(image, KindImage); got["X-Plain"] != "yes" || len(got) != 1 {
		t.Errorf("a token not yet kept leaves its header out, the rest stay: %v", got)
	}
	p.mu.Lock()
	p.storage["token"] = json.RawMessage(`"abc"`)
	p.mu.Unlock()
	if got := p.HeadersFor(image, KindImage); got["X-Token"] != "t-abc" {
		t.Errorf("a stored token: %v", got)
	}
	if got := p.HeadersFor(image, KindAudio); got != nil {
		t.Errorf("headers for images went with audio: %v", got)
	}
	if got := p.HeadersFor(mustURL(t, "https://evil.test/a.jpg"), KindImage); got != nil {
		t.Errorf("headers went to a host that is not the plugin's: %v", got)
	}
	if got := p.HeadersFor(mustURL(t, "ftp://api.example.com/a"), KindImage); got != nil {
		t.Errorf("headers for a request that is no web request: %v", got)
	}

	p.keepSetting("server", "http://nas.local:5666")
	p.mu.Lock()
	p.storage["session"] = json.RawMessage(`"s1"`)
	p.mu.Unlock()
	if got := p.HeadersFor(mustURL(t, "http://nas.local:5666/cover"), KindImage); got["Cookie"] != "session=s1" {
		t.Errorf("the server's: %v", got)
	}
	// A server the permission does not reach gets nothing, though it is the
	// one the listener named.
	p.keepSetting("server", "http://other.local")
	if got := p.HeadersFor(mustURL(t, "http://other.local/cover"), KindImage); got != nil {
		t.Errorf("a server outside the network permission got %v", got)
	}
	// A stored value with a line break could end the header, and add another.
	p.keepSetting("server", "nas.local")
	p.mu.Lock()
	p.storage["session"] = json.RawMessage(`"s1\r\nX-Evil: 1"`)
	p.mu.Unlock()
	if got := p.HeadersFor(mustURL(t, "http://nas.local/cover"), KindImage); got != nil {
		t.Errorf("a header with a line break went: %v", got)
	}
}
