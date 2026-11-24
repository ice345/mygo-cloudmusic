# Plugins

Where this app is going: sources of music as plugins, installed from [the plugins page](https://mygo-music.egoist.dev/plugins) as [`egoist/mygo-music`](https://github.com/egoist/mygo-music) does, with NetEase staying built in as the first source. This file is the plan and its decisions; the state of each stage is in [`roadmap.md`](roadmap.md#6-plugins).

This is not a fix for NetEase's -460: that was the eapi's User-Agent, and is fixed in the built-in client ([`roadmap.md`](roadmap.md#decisions), 2026-11-24). The `mygo-music-netease-plus` plugin sends the same iPhone User-Agent beside Windows headers, and only gets past -460 by asking the web's endpoint first.

## What upstream is

`egoist/mygo-music` (AGPL-3.0, as this repository is) is this app's successor: the archived `egoist/mygo-cloudmusic` rewritten as a player that knows no service. NetEase, QQ Music, Spotify, Bilibili and the rest are plugins, [about thirty](https://github.com/topics/mygo-music-plugin) of them. Its pieces, which we port, not depend on:

| Upstream | What it does | Size |
|---|---|---|
| `internal/music` | the types a view knows (`Track`, `Album`, `Browse` …) and `Source`, the 50 things a source can do, with `Can(what)` for the views to hide what it can't | 480 lines |
| `internal/plugin` | the host: reads `plugin.json`, runs one JS file in [moejs](https://github.com/MoeForge/moejs) (a pure-Go engine, Apache-2.0, alpha), gives it `fetch`, `host.storage`, `host.crypto`, `host.webSignIn`, `host.spawn`, enforces permissions, makes a plugin a `Source` | 2,400 lines |
| `internal/app/sources.go`, `plugin_install.go`, `ui_source_settings.go`, `request_headers.go` | a library of answers per source, the switcher, install from GitHub, a zip, a folder or a `mygo-music://install-plugin?repo=owner/repo` link, and a plugin's settings | 1,200 lines |
| `examples/demo-plugin` | every export, with made-up data | 300 lines |

The plugin contract is in upstream's README (the exports and their shapes). We keep it as it is, so a plugin made for upstream runs here, and add to it only where NetEase needs more (below).

## Why it is a rewrite of the data layer

Upstream's views know `music.Track{ID string}`. Ours know `netease.Track{ID int64}`, with NetEase's fields: 653 references in 31 files of `internal/app` and `internal/player`, and about as many in tests. Plugins can only be sources if the views stop knowing NetEase. So the work is in the views, not the host; the host is mostly a port.

## Decisions

**IDs are opaque strings.** A track is `{Source, ID}`; the app never reads inside an ID. NetEase's are the decimal song ID, and a podcast episode is `<audioID>@<programID>`, since its comments, likes and page are the program's and its audio is another ID.

**NetEase is a `Source` written in Go**, in `internal/netease` beside the client, and the only one that doesn't need the host. Its ID is `netease`. What `Source` has no method for (events, a listener's page, the daily mix as a card, 私人 FM's trash, search for users) stays NetEase's own, through optional interfaces the views ask for (`src.(netease.Extras)`), and shows only while NetEase is the source. Nothing NetEase can do today is dropped.

**`music` adds what NetEase needs.** `LyricLine.Reading` and `Lyrics.Reading` carry 音译, which upstream has no place for; a plugin that leaves them out has none. NetEase's own source turns its texts into lines with `lyrics.Build`, so word-timed lyrics, merged lines and translations keep the logic that is tested; `lyrics.FromMusic` turns `music.Lyrics` into `lyrics.Line` again. A line's words go across only where they carry their own times, and the others share the line's time as they always did, so the round trip is exact (`TestLinesSurviveTheSourcesShape`). `Leveler` lets a source name its levels of audio (NetEase has five; upstream's `Qualities` has three), which the cache keeps songs by.

**A track carries its source's record.** `Track.Extra` holds the source's own record of a track (NetEase's whole `netease.Track`: the fee, the privilege, an episode's program and podcast), which the app never reads, and the queue and the session keep. While views still hold `netease.Track`, `internal/app/bridge.go` turns one into the other at the player's edge, and nothing is lost. The bridge shrinks as views move, and `Extra` stays for what only NetEase's own pages need.

**A refusal is its own kind.** `music.ErrRefused` says the source turned the request away for now, as -460: the track stays, paused, and the reason (`music.Reason`) is the source's own words. It is what `netease.ErrRefused` is today, so nothing about how a refusal plays changes.

**Sessions and the cache keep working.** `session.json`'s queue is read in both shapes (the old `netease.Track` JSON becomes `{Source: "netease"}` tracks, `decodeSession`) and written in the new one. The cache names a song for its source and a hash of its ID, which may hold anything, a path included; NetEase's keep the names they have, `<id>-<level>`, so nothing is downloaded again (`TestCacheKeyNamesFiles`).

**Licence.** Code ported from upstream keeps its licence (AGPL-3.0, ours too) and says so in the file's head. The plugins page lists repositories with the topic `mygo-music-plugin`; the host does not run what isn't installed.

## Stages

1. **The seam** (this stage; MyGo first, then the views):
   1. MyGo v0.10.0, from a v0.7 snapshot. Done 2026-11-24: it builds, tests pass, and the window draws as before. The v0.8 changes that mattered: windows' accents fade when inactive (`InactiveAccent` keeps ours), and `HandleInput` sees every key (our handlers pick by kind).
   2. `internal/music`: the types, `Source`, errors, an embeddable `Unsupported`.
   3. The views, a slice at a time, each one green: the player (queue, stream, lyrics, cover, session, cache), then lists and search, then the library pages (playlist, album, artist), then account, likes and comments, then podcasts and stations; NetEase's extras last. **The player is done** (2026-11-24, on `feat/plugins`): it knows `music.Track`, finds a track's source through `Player.Source`, and imports nothing of NetEase's; `netease.Source` does stream and lyrics (`Can` says so); the views read it through `bridge.go`. **Search is done** (2026-11-24): `Source.Search` serves tracks, albums, artists, playlists and shows, 60 to a page; the search page holds `music` results and draws the artist, album, playlist and show cards from them, which the artist and user pages use too. What stays NetEase's, for the slices that move what it needs: the shared song rows (they read the like state, routes by numeric ID, the song menu, `Alias` and `Reason`, and are fed by pages that still hold NetEase's tracks, so search hands them its songs through the bridge, made once as each page comes in), the suggestions under the search field (NetEase suggests songs, artists, albums and playlists to open, which a source's `Suggest` of keywords has no place for), and the search for users, which no other source has. **The library pages' IDs, likes and song rows are done** (2026-11-24): a route is `{source, id}` as strings, and a card or a row link opens the page of its item's own source; the likes are one map by source and track ID, liked through `Source.Like`, `LikeEpisode` and `IsLiked`; and the shared song rows, with their artist links, heart and menu, take `music.Track` (`Alias` and `Reason` became optional fields of it), fed lazily by `neteaseList` from pages that still hold NetEase's tracks, a row at a time. What is left of the library pages: their data. A playlist, an album, an artist, a user and a podcast are still loaded by NetEase's number, from NetEase's functions, into maps keyed by that number, and a page of another source says it cannot open yet (`route.number`); they move to `Source.Playlist`, `Album`, `Artist`, `Show` with their headers, the playlist editing and the other likes on them.
2. **The host.** `internal/plugin` from upstream, with `plugin.json`, permissions, `fetch` and `host`; a plugin becomes a `Source`; `Player.Source(id)` finds it. Source settings and the switcher.
3. **Installing.** From GitHub, a zip or a folder, `mygo-music://install-plugin` (the scheme goes in `mygo.json`'s `urlSchemes`), updates, removal, a plugins list in settings.
4. **Release.** The README's plugin section, `plugins/` in the user data directory, release notes that say plugins run with the permissions the listener allowed.

## Risks

- moejs is alpha (`v0.1.0-alpha.11`). Upstream ships on it, but its behaviour can change.
- A plugin with `network: ["*"]` or `process` can do what the listener can. Upstream asks before installing and before allowing a program; we keep both.
- Every view that reads a track changes in stage 1.3, so the frame and perf tests are the net: they must stay green at every slice, and the Mac checks in [`roadmap.md`](roadmap.md#2-before-0128-blocks-the-release) are run again before a release.
