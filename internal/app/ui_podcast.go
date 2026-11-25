package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
	"netease-music-mygo/internal/netease"
)

// programsPage is how many of a podcast's episodes load at a time.
const programsPage = 30

// podcastData is what the app has of a podcast: what it is, and its
// episodes, the newest first, as far as they have loaded.
type podcastData struct {
	show        *music.Show
	loading     bool
	err         string
	episodes    []music.Track
	page        paged
	subscribing bool // a subscription or its end is on its way
	thenPlay    bool // its card's play button waits for it to load
}

// loadPodcast loads a podcast and its first page of episodes, asked of the
// source its route names, unless it has.
func (a *app) loadPodcast(k pageKey) {
	d := a.podcasts[k]
	if d == nil {
		d = &podcastData{}
		a.podcasts[k] = d
	}
	if d.show != nil || d.loading {
		return
	}
	src := a.source(k.source)
	if src == nil {
		d.err = "暂不支持打开这个来源的页面"
		return
	}
	d.loading, d.err = true, ""
	go func() {
		page, err := src.Show(k.id, 0)
		a.post(func() {
			d.loading = false
			if err != nil {
				d.err, d.thenPlay = "播客加载失败", false
				return
			}
			show := page.Show
			if show.Source == "" {
				show.Source = k.source
			}
			d.show, d.episodes = &show, page.Episodes
			d.page = paged{loaded: true, more: page.More && len(page.Episodes) > 0}
			a.askLikes(page.Episodes)
			a.playLoaded(d)
		})
	}()
}

// loadPrograms loads the next page of a podcast's episodes.
func (a *app) loadPrograms(k pageKey) {
	d := a.podcasts[k]
	src := a.source(k.source)
	if d == nil || src == nil || d.page.loading {
		return
	}
	d.page.loading, d.page.err = true, ""
	offset := len(d.episodes)
	go func() {
		page, err := src.Show(k.id, offset)
		a.post(func() {
			d.page.loading = false
			if err != nil {
				d.page.err, d.thenPlay = "节目加载失败", false
				return
			}
			d.episodes = append(d.episodes, page.Episodes...)
			d.page.loaded, d.page.more = true, page.More && len(page.Episodes) > 0
			// Which the listener likes, for their hearts.
			a.askLikes(page.Episodes)
			a.playLoaded(d)
		})
	}()
}

// playProgram plays the episodes loaded, from the one at i: those with
// audio to play.
func (a *app) playProgram(d *podcastData, i int) {
	var queue []music.Track
	at := 0
	for j := range d.episodes {
		if d.episodes[j].Unavailable {
			continue
		}
		if j <= i {
			at = len(queue)
		}
		queue = append(queue, d.episodes[j])
	}
	if len(queue) > 0 {
		a.pl.PlayQueue(queue, at)
	}
}

// playPodcast plays a podcast's episodes from the newest, from its card,
// once it and they have loaded.
func (a *app) playPodcast(k pageKey) {
	a.loadPodcast(k)
	d := a.podcasts[k]
	d.thenPlay = true
	a.playLoaded(d)
}

// playLoaded plays a podcast its card asked to play, once it can.
func (a *app) playLoaded(d *podcastData) {
	if d.thenPlay && d.show != nil && d.page.loaded {
		d.thenPlay = false
		a.playProgram(d, 0)
	}
}

// toggleSubscribe subscribes the listener to a podcast, or unsubscribes
// them, at once, going back if its source refuses. Signed out, it asks the
// listener to sign in.
func (a *app) toggleSubscribe(d *podcastData) {
	if d.show == nil || d.subscribing {
		return
	}
	if !a.signedInTo(d.show.Source) {
		a.signInTo(d.show.Source)
		return
	}
	show := d.show
	src := a.source(show.Source)
	tab := a.podcastTab
	sub := !show.Subscribed
	show.Subscribed, d.subscribing = sub, true
	if sub {
		show.Subscribers++
	} else {
		show.Subscribers = max(show.Subscribers-1, 0)
	}
	go func() {
		err := music.ErrUnsupported
		if src != nil {
			err = src.SubscribeShow(show.ID, sub)
		}
		a.post(func() {
			d.subscribing = false
			switch {
			case err != nil:
				show.Subscribed = !sub
				if sub {
					show.Subscribers = max(show.Subscribers-1, 0)
				} else {
					show.Subscribers++
				}
				if !a.signInNeeded(show.Source, err) {
					a.toast = "操作失败，请稍后再试"
				}
			case sub:
				tab.subscribed(show)
				a.toast = "已订阅"
			default:
				tab.unsubscribed(pageKey{show.Source, show.ID})
				a.toast = "已取消订阅"
			}
		})
	}()
}

func (a *app) podcastPage(c *ui.Context) {
	p := a.pal
	k := a.route.key()
	d := a.podcasts[k]
	if d == nil {
		return
	}
	if d.show == nil {
		if !d.loading {
			a.centered(c, d.err, func() { a.loadPodcast(k) })
			return
		}
		s := a.pageShimmer(c)
		a.trackSkeleton(c, s, true, func(c *ui.Context) { a.playlistHeaderSkeleton(c, s) })
		return
	}
	pod := d.show
	cur := a.current()
	a.scrollPage(c, func() {
		ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 8, 0).Children(func() {
			ui.Box(c).Size(212, 212).Radius(14).Shadow(0, 18, 40, -10, p.cardShade).Children(func() {
				a.coverFill(c, pod.ImageURL, 600, 14)
			})
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
				kind := "播客"
				if pod.Category != "" {
					kind += " · " + pod.Category
				}
				ui.Text(c, kind).FontSize(12).FontWeight(600).TextColor(p.accent)
				ui.Text(c, pod.Name).FontSize(30).FontWeight(800).LetterSpacing(-0.6).MaxLines(2).LineHeight(1.2)
				ui.RichText(c).FontSize(13).TextColor(p.muted).Children(func() {
					if uid := hostOf(pod); uid != 0 {
						a.link(c.Key("dj"), pod.Publisher, p.muted, p.text, nroute(pageUser, uid))
						ui.Text(c, " · ")
					} else if pod.Publisher != "" {
						ui.Text(c, pod.Publisher)
						ui.Text(c, " · ")
					}
					meta := fmt.Sprintf("%d 期", pod.EpisodeCount)
					if pod.Subscribers > 0 {
						meta += " · " + count(pod.Subscribers) + " 人订阅"
					}
					ui.Text(c, meta)
				})
				if desc := strings.TrimSpace(pod.Description); desc != "" {
					ui.Text(c, strings.ReplaceAll(desc, "\n", " ")).FontSize(12.5).TextColor(p.muted).MaxLines(2).LineHeight(1.45).Margin(2, 0, 0, 0)
				}
				ui.Row(c).Gap(10).Margin(10, 0, 0, 0).Children(func() {
					if pillButton(c.Key("play"), iconPlay, "播放", p.accent, p.onAccent).Clicked() {
						a.playProgram(d, 0)
					}
					if src := a.source(pod.Source); src != nil && src.Can("subscribeShow") {
						icon, label := iconPlus, "订阅"
						if pod.Subscribed {
							icon, label = iconCheck, "已订阅"
						}
						if pillButton(c.Key("subscribe"), icon, label, p.hover.Over(p.bg), p.text).Clicked() {
							a.toggleSubscribe(d)
						}
					}
				})
			})
		})
		a.section(c, "节目")
		for i := range d.episodes {
			a.programRow(c.Key(d.episodes[i].ID), d, i, cur)
		}
		a.pageFoot(c, &d.page, func() { a.loadPrograms(k) })
		if d.page.loaded && len(d.episodes) == 0 {
			a.empty(c, "暂无节目")
		}
	})
}

// programRow is an episode in its podcast's list, which a click plays.
func (a *app) programRow(c *ui.Context, d *podcastData, i int, cur *music.Track) {
	p := a.pal
	ep := &d.episodes[i]
	playing := cur != nil && keyOf(cur) == keyOf(ep)
	row := ui.ButtonBase(c).Height(60).PaddingX(12).Gap(14).Radius(8).AlignItems(ui.Center).FocusRing(false).
		Label(ep.Name).Disabled(ep.Unavailable)
	switch {
	case playing:
		row.Background(p.accent.Alpha(0.08))
	case row.Hovered():
		row.Background(p.hover)
	}
	row.Children(func() {
		cover := ep.Album.ImageURL
		if cover == "" {
			cover = d.show.ImageURL
		}
		a.cover(c, cover, 96, 40, 6)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			title := ui.Text(c, ep.Name).FontSize(13.5).FontWeight(500).SingleLine()
			if playing {
				title.TextColor(p.accent)
			}
			ui.Text(c, ep.Published).FontSize(12).TextColor(p.muted)
		})
		if playing {
			a.nowPlayingBars(c)
		}
		// A liked episode's heart shows; the others' as the pointer is on them.
		ui.Box(c).Size(32, 32).Center().Children(func() {
			if a.isLiked(*ep) || row.Hovered() {
				a.likeButton(c.Key("like"), *ep, 15, p.muted)
			}
		})
		ui.Text(c, clock(ep.Length())).FontSize(12).FontFeatures("tnum").TextColor(p.muted)
	})
	if row.Clicked() {
		a.playProgram(d, i)
	}
}

// podcastRow is a podcast in a list, opening its page.
func (a *app) podcastRow(c *ui.Context, pod *music.Show) {
	p := a.pal
	row := ui.ButtonBase(c).Padding(10, 12).Gap(14).Radius(10).AlignItems(ui.Center).FocusRing(false).Label(pod.Name)
	if row.Hovered() {
		row.Background(p.hover)
	}
	row.Children(func() {
		a.cover(c, pod.ImageURL, 160, 64, 8)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, pod.Name).FontSize(14).FontWeight(600).SingleLine()
			meta := []string{fmt.Sprintf("%d 期", pod.EpisodeCount)}
			if pod.Publisher != "" {
				meta = append([]string{pod.Publisher}, meta...)
			}
			ui.Text(c, strings.Join(meta, " · ")).FontSize(12.5).TextColor(p.muted).SingleLine()
			if pod.Description != "" {
				ui.Text(c, pod.Description).FontSize(12.5).TextColor(p.muted).SingleLine()
			}
		})
	})
	if row.Clicked() {
		a.open(route{page: pagePodcast, source: pod.Source, id: pod.ID})
	}
}

// programPane is what the episode playing is about, in Now Playing, in
// place of a song's lyrics.
func (a *app) programPane(c *ui.Context, tr *music.Track) {
	ui.Scroll(c.Key(keyOf(tr))).Grow(1).Padding(4, 12, 48, 0).Children(func() {
		desc := strings.TrimSpace(tr.Description)
		if desc == "" {
			desc = "这期节目没有简介"
		}
		ui.Text(c, desc).FontSize(15).LineHeight(1.7).TextColor(white.Alpha(0.75))
	})
}

// A podcastPicker is a source with podcasts of its own to pick from, besides
// those the listener subscribes to: NetEase's picks and its most popular.
type podcastPicker interface {
	PodcastPicks() ([]music.Show, error)
	HotPodcasts(offset int) ([]music.Show, bool, error)
}

// podcastTab is what the podcasts tab lists: the podcasts the listener
// subscribes to, the source's picks, and the most popular, as far as they
// have loaded.
type podcastTab struct {
	subs    []music.Show // the latest subscribed first
	picks   []music.Show
	hot     []music.Show
	hotPage paged
	hotAt   int // how far into the source's list the next page starts
	loading bool
	err     string
}

func (a *app) loadPodcastTab() {
	t := a.podcastTab
	if t.hotPage.loaded || t.loading {
		return
	}
	src := a.accountSource()
	if src == nil {
		t.err = "暂不支持打开这个来源的页面"
		return
	}
	t.loading, t.err = true, ""
	signedIn := a.account != nil
	go func() {
		var subs []music.Show
		if signedIn {
			subs, _ = src.Shows()
		}
		var picks, hot []music.Show
		var more bool
		var err1, err2 error
		if pk, ok := src.(podcastPicker); ok {
			picks, err1 = pk.PodcastPicks()
			hot, more, err2 = pk.HotPodcasts(0)
		}
		a.post(func() {
			t.loading = false
			if err1 != nil && err2 != nil {
				t.err = "无法连接网易云音乐，请检查网络"
				return
			}
			t.subs, t.picks = subs, picks
			t.addHot(hot)
			t.hotPage = paged{loaded: true, more: more && len(hot) > 0}
			if err2 != nil {
				t.hotPage.err = "热门播客加载失败"
			}
		})
	}()
}

// loadHotPodcasts loads the next page of the most popular podcasts.
func (a *app) loadHotPodcasts() {
	t := a.podcastTab
	pk, ok := a.accountSource().(podcastPicker)
	if !ok || t.hotPage.loading {
		return
	}
	t.hotPage.loading, t.hotPage.err = true, ""
	offset := t.hotAt
	go func() {
		hot, more, err := pk.HotPodcasts(offset)
		a.post(func() {
			t.hotPage.loading = false
			if err != nil {
				t.hotPage.err = "热门播客加载失败"
				return
			}
			t.addHot(hot)
			t.hotPage.more = more && len(hot) > 0
		})
	}()
}

// addHot lists a page of the most popular podcasts after those listed,
// but those already listed: NetEase's pages overlap, a podcast often
// coming again a page or two later.
func (t *podcastTab) addHot(page []music.Show) {
	t.hotAt += len(page)
	for _, pod := range page {
		if !slices.ContainsFunc(t.hot, func(p music.Show) bool { return p.Source == pod.Source && p.ID == pod.ID }) {
			t.hot = append(t.hot, pod)
		}
	}
}

// subscribed lists a podcast the listener just subscribed to first, once
// the tab has loaded what they subscribe to.
func (t *podcastTab) subscribed(pod *music.Show) {
	if !t.hotPage.loaded {
		return
	}
	t.unsubscribed(pageKey{pod.Source, pod.ID})
	t.subs = append([]music.Show{*pod}, t.subs...)
}

// unsubscribed takes a podcast out of the listener's subscriptions.
func (t *podcastTab) unsubscribed(k pageKey) {
	t.subs = slices.DeleteFunc(t.subs, func(p music.Show) bool { return p.Source == k.source && p.ID == k.id })
}

func (a *app) podcastsPage(c *ui.Context) {
	t := a.podcastTab
	if !t.hotPage.loaded && !t.loading {
		a.centered(c, t.err, a.loadPodcastTab)
		return
	}
	top, bottom := a.insets(c)
	ui.Scroll(c.Key("podcasts")).TrackScroll(&a.pageScroll).Grow(1).Padding(top+pageGap, pagePad, bottom+48, pagePad).ScrollbarInsets(top, 0, bottom, 0).Children(func() {
		a.startMark(c)
		defer a.endMark(c)
		a.heading(c, "播客", "")
		cols := a.columns(c, 150)
		if !t.hotPage.loaded {
			s := a.pageShimmer(c)
			a.section(c, "精选播客").Margin(0, 0, 14, 0)
			a.playlistCardsSkeleton(c, s, cols, 150, true)
			a.section(c, "热门播客")
			a.playlistCardsSkeleton(c, s, cols*2, 150, true)
			return
		}
		// The first list starts right under the heading.
		first := true
		list := func(title string, pods []music.Show) {
			if len(pods) == 0 {
				return
			}
			if s := a.section(c, title); first {
				s.Margin(0, 0, 14, 0)
			}
			first = false
			ui.Grid(c.Key(title)).Columns(cols).GapX(20).GapY(28).Children(func() {
				for i := range pods {
					a.podcastCard(c.Key(pods[i].ID), &pods[i])
				}
			})
		}
		list("我的订阅", t.subs)
		list("精选播客", t.picks)
		list("热门播客", t.hot)
		a.pageFoot(c, &t.hotPage, a.loadHotPodcasts)
	})
}

// podcastCard is a podcast's cover over its name and, for those the
// listener subscribes to, its newest episode; for the others, why to
// listen.
func (a *app) podcastCard(c *ui.Context, pod *music.Show) {
	sub := pod.Latest
	if sub == "" {
		sub = pod.Reason
	}
	if sub == "" {
		sub = pod.Publisher
	}
	k := pageKey{pod.Source, pod.ID}
	if a.coverCard(c, pod.Name, pod.ImageURL, float64(pod.PlayCount), sub, func() { a.playPodcast(k) }) {
		a.open(route{page: pagePodcast, source: pod.Source, id: pod.ID})
	}
}

// hostOf is the number of the page of who hosts a NetEase podcast, which only
// NetEase says, or 0.
func hostOf(pod *music.Show) int64 {
	if pod.Source != netease.SourceID {
		return 0
	}
	return netease.PodcastRecord(*pod).DJ.UserID
}
