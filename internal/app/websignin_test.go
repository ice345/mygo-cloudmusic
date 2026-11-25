package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

// awaiting runs awaitCookies with the lists given in turn, one for the start
// and one for each tick the test sends; it answers what ended it.
type awaiting struct {
	tick   chan time.Time
	closed chan struct{}
	asked  chan struct{}
	done   chan result
}

type result struct {
	header string
	err    error
}

func awaitCookiesOf(ctx context.Context, until []string, lists ...[]mygo.Cookie) *awaiting {
	w := &awaiting{tick: make(chan time.Time), closed: make(chan struct{}), asked: make(chan struct{}, len(lists)+1), done: make(chan result, 1)}
	n := 0
	list := func() ([]mygo.Cookie, error) {
		defer func() { w.asked <- struct{}{} }()
		if n >= len(lists) {
			return nil, errors.New("no more cookies")
		}
		n++
		return lists[n-1], nil
	}
	go func() {
		header, err := awaitCookies(ctx, list, until, w.tick, w.closed)
		w.done <- result{header, err}
	}()
	<-w.asked // the first, at the start
	return w
}

// next lets a tick in and waits for the cookies asked for after it.
func (w *awaiting) next() {
	w.tick <- time.Time{}
	<-w.asked
}

// A sign-in page's cookies are the plugin's once each of the ones it named
// is there with a value, all of the address's as a Cookie header; not while
// one is missing or has none, as a page sets it before signing in.
func TestWebSignInEndsWhenTheCookiesAreThere(t *testing.T) {
	w := awaitCookiesOf(context.Background(), []string{"sid", "uid"},
		[]mygo.Cookie{{Name: "tracking", Value: "x"}},
		[]mygo.Cookie{{Name: "tracking", Value: "x"}, {Name: "sid", Value: "1"}},
		[]mygo.Cookie{{Name: "tracking", Value: "x"}, {Name: "sid", Value: "1"}, {Name: "uid", Value: ""}},
		[]mygo.Cookie{{Name: "tracking", Value: "x"}, {Name: "sid", Value: "1"}, {Name: "uid", Value: "2"}},
	)
	for range 2 {
		w.next()
		select {
		case r := <-w.done:
			t.Fatalf("ended with %+v before the cookies named were there", r)
		default:
		}
	}
	w.tick <- time.Time{}
	r := <-w.done
	if r.err != nil || r.header != "tracking=x; sid=1; uid=2" {
		t.Errorf("%+v", r)
	}
}

// A sign-in given up, or a page the listener closed, ends the wait: the first
// with what ended it, the second as a page closed before signing in.
func TestWebSignInEndsWhenGivenUpOrClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	w := awaitCookiesOf(ctx, []string{"sid"}, nil)
	cancel()
	if r := <-w.done; !errors.Is(r.err, context.Canceled) {
		t.Errorf("given up: %+v", r)
	}

	w = awaitCookiesOf(context.Background(), []string{"sid"}, nil)
	close(w.closed)
	if r := <-w.done; !errors.Is(r.err, errSignInPageClosed) {
		t.Errorf("closed: %+v", r)
	}
}

// Cookies that can't be listed for a moment don't end the wait: the next tick
// asks again.
func TestWebSignInTriesAgainWhenTheCookiesCannotBeListed(t *testing.T) {
	var asked int
	tick := make(chan time.Time)
	done := make(chan result, 1)
	go func() {
		header, err := awaitCookies(context.Background(), func() ([]mygo.Cookie, error) {
			asked++
			if asked == 1 {
				return nil, errors.New("not ready")
			}
			return []mygo.Cookie{{Name: "sid", Value: "1"}}, nil
		}, []string{"sid"}, tick, nil)
		done <- result{header, err}
	}()
	tick <- time.Time{}
	if r := <-done; r.err != nil || r.header != "sid=1" {
		t.Errorf("%+v", r)
	}
}
