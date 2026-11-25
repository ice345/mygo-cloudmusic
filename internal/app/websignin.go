package app

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/plugin"
)

// errSignInPageClosed is how a page the listener closed ends a sign-in.
var errSignInPageClosed = errors.New("登录页面在登录完成前被关闭")

// showWebSignIn shows the page a plugin signs in on, in a window of its own,
// and answers the cookies it leaves once the ones the plugin named are there.
// The page lives in a session that keeps nothing: apart from every other page,
// the listener's browser's too, and forgotten when the window closes. It is
// closed when the listener gives the sign-in up (ctx ends) and when it is done;
// closed by the listener, it ends the sign-in.
func (a *app) showWebSignIn(ctx context.Context, o plugin.WebSignIn) (string, error) {
	session := mygo.NewEphemeralSession()
	var wv *mygo.WebView
	var wvMu sync.Mutex
	site := o.CookiesOf
	if u, err := url.Parse(o.CookiesOf); err == nil {
		site = u.Hostname()
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:           "登录 " + o.Name,
		Width:           900,
		Height:          720,
		MinWidth:        520,
		MinHeight:       420,
		DisableMinimize: true,
		BackgroundColor: "light-dark(#ffffff, #141416)",
		Content: ui.View(func(c *ui.Context) {
			t := *c.Theme()
			p := paletteFor(&t)
			c.Root().Background(p.bg)
			ui.Column(c).Fill().Children(func() {
				ui.Text(c, "“"+o.Name+"”想让你在 "+site+" 登录。登录后窗口会自动关闭，只有这个网站的登录状态会交给它。").
					FontSize(12).TextColor(p.muted).LineHeight(1.4).Padding(10, 16)
				wvMu.Lock()
				page := wv
				wvMu.Unlock()
				if page != nil {
					ui.WebView(c, page).Grow(1)
				}
			})
		}),
	})
	closed := make(chan struct{})
	var once sync.Once
	win.OnClosed(func() { once.Do(func() { close(closed) }) })
	page, err := win.NewWebView(mygo.WebViewOptions{
		URL:  o.URL,
		Page: mygo.PageOptions{Session: session, UserAgent: o.UserAgent},
	})
	if err != nil {
		win.Close()
		return "", err
	}
	wvMu.Lock()
	wv = page
	wvMu.Unlock()
	win.Invalidate()
	defer win.Close()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	return awaitCookies(ctx, func() ([]mygo.Cookie, error) {
		return session.Cookies.List(mygo.CookieFilter{URL: o.CookiesOf})
	}, o.Until, ticker.C, closed)
}

// awaitCookies asks list for the cookies, once and then at each tick, until
// all of those named by until are there with a value. It answers them all as a
// Cookie header: those a request to the address would send. It gives up with
// ctx's error when ctx ends, and when closed does, with errSignInPageClosed.
// A list that fails is tried again at the next tick.
func awaitCookies(ctx context.Context, list func() ([]mygo.Cookie, error), until []string, tick <-chan time.Time, closed <-chan struct{}) (string, error) {
	for {
		if cookies, err := list(); err == nil {
			if header, ok := cookieHeader(cookies, until); ok {
				return header, nil
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-closed:
			return "", errSignInPageClosed
		case <-tick:
		}
	}
}

// cookieHeader is the Cookie header of cookies, and whether each of the names
// in until is among them with a value.
func cookieHeader(cookies []mygo.Cookie, until []string) (string, bool) {
	have := map[string]bool{}
	pairs := make([]string, 0, len(cookies))
	for _, c := range cookies {
		pairs = append(pairs, c.Name+"="+c.Value)
		if c.Value != "" {
			have[c.Name] = true
		}
	}
	for _, name := range until {
		if !have[name] {
			return "", false
		}
	}
	return strings.Join(pairs, "; "), true
}
