package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/music"
)

// albumData is what the app has of an album.
type albumData struct {
	page    *music.AlbumPage
	loading bool
	err     string
}

// loadAlbum asks the source for the album k names, unless it has it or is
// asking.
func (a *app) loadAlbum(k pageKey) {
	src := a.source(k.source)
	if k.id == "" || k.id == "0" {
		return
	}
	d := a.albums[k]
	if d == nil {
		d = &albumData{}
		a.albums[k] = d
	}
	if d.page != nil || d.loading {
		return
	}
	if src == nil {
		d.err = "暂不支持打开这个来源的页面"
		return
	}
	d.loading, d.err = true, ""
	go func() {
		page, err := src.Album(k.id)
		a.post(func() {
			d.loading = false
			if err != nil {
				d.err = "专辑加载失败"
				return
			}
			d.page = page
		})
	}()
}

func (a *app) albumPage(c *ui.Context) {
	k := a.route.key()
	d := a.albums[k]
	if d == nil {
		d = &albumData{}
		a.albums[k] = d
	}
	if d.page == nil {
		if !d.loading {
			a.centered(c, d.err, func() { a.loadAlbum(k) })
			return
		}
		s := a.pageShimmer(c)
		a.trackSkeleton(c, s, false, func(c *ui.Context) { a.playlistHeaderSkeleton(c, s) })
		return
	}
	a.trackRows(c, listOf(d.page.Tracks), false, func(c *ui.Context) { a.albumHeader(c, k.source, d.page) })
}

func (a *app) albumHeader(c *ui.Context, source string, page *music.AlbumPage) {
	p := a.pal
	al := &page.Album
	ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 26, 0).Children(func() {
		ui.Box(c).Size(212, 212).Radius(14).Shadow(0, 18, 40, -10, p.cardShade).Children(func() {
			a.coverFill(c, al.ImageURL, 600, 14)
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
			ui.Text(c, "专辑").FontSize(12).FontWeight(600).TextColor(p.accent)
			ui.Text(c, al.Name).FontSize(30).FontWeight(800).LetterSpacing(-0.6).MaxLines(2).LineHeight(1.2)
			ui.RichText(c).FontSize(13).TextColor(p.muted).Children(func() {
				for i, ar := range al.Artists {
					if i > 0 {
						ui.Text(c, ", ")
					}
					if ar.ID == "" {
						ui.Text(c, ar.Name)
						continue
					}
					a.link(c.Key(i), ar.Name, p.muted, p.text, route{page: pageArtist, source: source, id: ar.ID})
				}
				meta := fmt.Sprintf(" · %d 首", len(page.Tracks))
				if al.Released != "" {
					meta = " · " + al.Released + meta
				}
				ui.Text(c, meta)
			})
			if d := strings.TrimSpace(al.Description); d != "" {
				ui.Text(c, strings.ReplaceAll(d, "\n", " ")).FontSize(12.5).TextColor(p.muted).MaxLines(2).LineHeight(1.45).Margin(2, 0, 0, 0)
			}
			ui.Row(c).Gap(10).Margin(10, 0, 0, 0).Children(func() {
				if pillButton(c.Key("play"), iconPlay, "播放", p.accent, p.onAccent).Clicked() {
					if a.pl.Shuffle() {
						a.pl.ToggleShuffle()
					}
					a.pl.PlayQueue(page.Tracks, firstAvailable(page.Tracks))
				}
			})
		})
	})
}

// albumGrid shows albums as cards, each opening its page.
func (a *app) albumGrid(c *ui.Context, albums []music.Album) {
	ui.Grid(c).Columns(a.columns(c, 156)).GapX(20).GapY(28).Children(func() {
		for i := range albums {
			a.albumCard(c.Key(albums[i].ID), &albums[i])
		}
	})
}

// albumCard is an album's cover, lifting under the pointer, over its name
// and the year it came out.
func (a *app) albumCard(c *ui.Context, al *music.Album) {
	p := a.pal
	card := ui.ButtonBase(c).Column().Gap(10).AlignItems(ui.Stretch).FocusRing(false).Label(al.Name)
	hovered := card.Hovered()
	card.Children(func() {
		art := ui.Box(c).AspectRatio(1).Radius(10)
		lift := ui.Pose{}
		if hovered {
			lift = ui.Pose{Y: -3}
			art.Shadow(0, 12, 28, -6, p.cardShade)
		} else {
			art.Shadow(0, 2, 8, -2, p.cardShade)
		}
		art.AnimatePose(lift, ui.Snappy)
		art.Children(func() { a.coverFill(c, al.ImageURL, 400, 10) })
		ui.Text(c, al.Name).FontSize(13).FontWeight(500).MaxLines(2).LineHeight(1.35)
		var meta []string
		if a.route.page != pageArtist {
			meta = append(meta, al.ArtistNames())
		}
		if len(al.Released) >= 4 {
			meta = append(meta, al.Released[:4])
		}
		ui.Text(c, strings.Join(meta, " · ")).FontSize(12).TextColor(p.muted).SingleLine().Margin(-6, 0, 0, 0)
	})
	if card.Clicked() {
		a.open(route{page: pageAlbum, source: al.Source, id: al.ID})
	}
}
