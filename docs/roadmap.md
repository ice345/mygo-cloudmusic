# Roadmap

What comes next, in order, and where each item stands. Update it when an item lands or changes, and at each release. Older plans and reviews are in [`archive/`](archive/); the last one is [`archive/2026-11-after-0.1.26/review-2026-11-17.md`](archive/2026-11-after-0.1.26/review-2026-11-17.md), which gives the evidence for the items below.

Priorities: **P0** blocks a release. **P1** wrong in normal use. **P2** worth doing next. **P3** later, or only once verified.

## 1. The fork's first release, 0.1.27

0.1.27 was cut on 2026-11-18 without the macOS checks, because no Mac was available. Its release notes say so. Those checks block 0.1.28 (§2).

| Item | Pri | State |
|---|---|---|
| The open notch card and the greeting keep the pointer (regression in `c49756f`) | P0 | done in code (`c122424`), **pending macOS verification**: [the checklist in `notch.md`](notch.md#on-a-mac-before-a-release) |
| Updates are signed in CI: `MYGO_UPDATER_PRIVATE_KEY`, and a release fails early without it | P0 | done, `3441e4d`: a new key, the secret set 2026-11-17 |
| `main` pushed, and the repository public, so the update feed and the speech libraries download | P0 | done 2026-11-17. The feed answers once a release is published. |
| CI runs the tests on push and pull request, and before a release; the notch tests pass off macOS | P1 | done, `ea9ebb8`, `4db5d73` |
| A network error leaves the listener on their track, rather than skipping the queue | P1 | done, `bf1b752`, `4a72f15` |
| The speech libraries for podcast captions are hosted on this repository | P1 | done in code (`b57aaa2`, the `sherpa-onnx-2.0.1` prerelease), **pending macOS verification**: [the checks in `lyrics.md`](lyrics.md#podcast-captions). Downloads were checked on Linux, against their digests. |

## 2. Before 0.1.28: blocks the release

0.1.28 does not ship until each of these is done and its result written here. They were skipped for 0.1.27.

| Check | Where | State |
|---|---|---|
| The notch on a Mac with a notch: card, greeting, lyrics under the notch (with 0.1.28's rows and window), intro and outro, Atoll making way, an external display | [`notch.md`](notch.md#on-a-mac-before-a-release), steps 1-6 | done 2026-11-24 for 1, 2, 4 (outro), 5 and 3's rows with a translation; two bugs fixed (Atoll's quit and launch); Left: a reading under the notch, a click through the lyrics, a long intro, 6 (skipped) |
| Podcast captions on a Mac: the model and libraries download, and an episode is transcribed | [`lyrics.md`](lyrics.md#podcast-captions) | Apple silicon, 2026-11-24: libraries and model download, load and transcribe a minute in 24 s (`TestTranscribe`, English speech; its punctuation check wants Chinese); `TestNativesLive` passes. Not yet: the settings button, Intel, and the app's own 字幕 tab |
| 0.1.27's macOS build opens, after the quarantine step its notes give | the release page | pending |
| The `NM_FRAMES` frames looked at on macOS, with its fonts: 青鸟 in Hiragino Mincho, AppleMyungjo and Songti, and the line breaks there | [`stage.md`](stage.md#looking-at-it) | pending |
| `TestLyricTextsLive` signed in, for the one lyrics request (`7c8f425`) | `internal/netease/live_test.go` | pending: needs a signed-in session |
| A song that isn't cached plays, signed in: the eapi's User-Agent now matches its headers, and `SongURL` asks again after a -460, then the web's endpoint | `eapi.go`, `netease.go` | done in code 2026-11-24, **pending a signed-in listener**: signed out, only the old shape (iPhone User-Agent, Windows headers) came back -460, on every song and try; the dev build here has no NetEase session to play one |

## 3. Lyrics, for 0.1.28

| Item | Pri | Where | State |
|---|---|---|---|
| Word-timed lines keep words in parentheses, "(oh" and "(Ah)" | P1 | `lyrics.go`, `parseYRC` | done, `4d420a3` |
| Lines without word timing keep the space after Han and Hangul once lit | P1 | `SpreadWords` | done, `4672687` |
| The line before doesn't come back after a long break, in every style | P1 | `ui_stage.go`, `ui_stage_bird.go` | done, `e465dbd` |
| The lyrics under the notch come in with the first line and go with the last | P1 | `ui_notch.go`, `lyricStrip` | done, `dcddf48`; on the Mac checklist (§2) |
| Two LRC lines at one time: one line with its translation, and stage keys that don't collide | P2 | `lyrics.Build`, `lineKey` | done, `b9818c5`, `bb8e33e` |
| `Lang` on the line's own text in 歌词, not on its row | P2 | `ui_nowplaying.go`, `timedLines` | done, `5bd50ea` |
| A lyrics request that fails says so, with 重试, and is tried again once | P2 | `player.go`, `loadLyrics`; `lyricsFailedNote` | done, `bce8059` |
| 青鸟's translation arrives after its line, and answers from the right | P1 | `ui_stage_bird.go`, `birdCaptions` | done, `aee62b8` |
| `settings.json` written to a temporary file and renamed | P2 | `settings.go`, `save` | done |
| A cover that failed on the network is asked for again after a while | P2 | `artwork.go`, `get`: after a minute | done |
| Downloads stop when their track does, and give up when stalled | P2 | `player.go`, `stop`; `stream.go` | stalls give up after 30 s: done. Closing at `stop` left: `out.Close` may already close the stream; unverified |
| The notch's levels at 30 frames a second | P2 | `ui_notch.go`, `ui_pages.go`, `barFrame` | done |

## 4. Typography and 青鸟, for 0.1.28

| Item | Where | State |
|---|---|---|
| A registry of stage styles, read by the menu, `stage()` and Now Playing; the backdrop's tone a value, not a bool | `stageStyle`, `backdropTone` ([`stage.md`](stage.md#adding-a-style)) | done, `9db559b` |
| A line breaker: even rows, never inside a word, Korean at spaces; on 青鸟 and the foot band | `internal/linebreak`, `breakRows` ([`stage.md`](stage.md#breaking-lines)) | done, `78ca913` |
| 青鸟 v2: one glass for every cover, captions at 4.5:1, the answer at the line's right edge and 0.4 s after the first word, a Korean serif | `ui_stage_bird.go`, `backdrop.go` | done, `aee62b8` |
| One hierarchy by role for the reading and the translation, at 4.5:1, the translation `zh-Hans`: the foot band and 歌词 | `stageCaption`, `timedLines` ([`lyrics.md`](lyrics.md#where-they-show)) | done, `29942ff` |
| Under the notch: the translation whenever on; three rows with both; a window over the word sung | `stripRows`, `stripWindow` ([`notch.md`](notch.md)) | done, `f5db54e`; on the Mac checklist (§2) |
| The 歌词 list broken between phrases too | `timedLines` | open: its width is known only after layout |
| The captions over the fog, which is lighter than 4.5:1 allows | `fogColors` | open, P3: the fog is an experiment, off by default |

## 5. 和鸣, a style after 『響け！ユーフォニアム』, for 0.1.28

Done (`ui_stage_chorus.go`, `backdrop_backlit.go`): a sibling of 青鸟 on the shared stage, from what the makers said rather than from impressions, and nothing from the works themselves (the review's §5). High contrast, a backlight behind the line with lights out of focus, a rack focus as each line comes in, one brass gold on the word sung on word-timed lines, the translation in unison on the line's axis; hot comments stay. Its frames were looked at on six real covers on Linux ([`stage.md`](stage.md#和鸣)); they are on the Mac checklist (§2) with the other styles'. The release notes must not name the works.

## 6. Later

- **Structure.** Split the notch's state into an explicit mode; give `app` a constructor that tests share; one `lyrics.Source(texts)` conversion; caption sizes in one place.
- **Tests.** Seams read before goroutines start, so `-race` passes; the player's network behind seams, so the unit run makes no requests; frame tests that need NetEase behind `LIVE`; the wall tests shorter. One `internal/app` run of about a dozen failed on Linux on 2026-11-17, its output not kept: likely a late goroutine from a test seam. Keep the output the next time it happens.
- **Dependencies.** Upgrade MyGo to a tagged release (v0.9.x), with `go tool mygo migrate-ui`. The README asks for Go 1.27.3.
- **Platforms.** Decide whether Windows and Linux get a compact lyrics surface, or say the notch is macOS only. The notch picks the display with a notch (`safeAreaInsets`), not `screens[0]`.
- **Accessibility.** A real disabled state for the notch-lyrics control; Reduce Motion in the foot band.
- **The website.** Point it at this repository, or remove it.
- **P3, once verified.** Publishing the playback rate during a stall; the Cantonese rule on more songs; podcast captions under the notch; signing with this repository's own Developer ID.

## Decisions

| Date | Decision |
|---|---|
| 2026-11-05 | The menu says 音译, named for the song as 罗马音, 粤拼 or 拼音. |
| 2026-11-05 | The lyrics under the notch are off by default: they cover the window under them. |
| 2026-11-05 | 流光, 云阶 and 倾诉 share one foot band for the reading and translation; 青鸟 sets its own and shows no hot comments or sparks. |
| 2026-11-17 | The repository is public: the update feed and the speech libraries are its release assets, and the AGPL asks for the source anyway. |
| 2026-11-17 | Updates are signed with a new key, kept as the `MYGO_UPDATER_PRIVATE_KEY` secret ([`CONTRIBUTING.md`](../CONTRIBUTING.md#releases)). |
| 2026-11-18 | 0.1.27 ships without the macOS checks, which no Mac was available for; they block 0.1.28. |
| 2026-11-23 | Line breaks come from BudouX's Japanese and Simplified Chinese models (v0.10.4, Apache-2.0), kept in the repository, read by a port of its parser rather than a Go dependency, whose newest release is v0.1.0's model. |
| 2026-11-23 | 青鸟's answer ends at the line's right edge; one wider than the line sets out from the line's middle, to the right. Both captions read at 4.5:1 over the lightest glass: the glass went from value 0.40 to 0.36 at the top for it. |
| 2026-11-23 | The Euphonium sibling is named 和鸣 ("sounding together"), not 余晖 ("afterglow"), which is elegiac: id `chorus`. No sparks, as the gold is its one accent. |
| 2026-11-23 | Captions by role: in the foot band the translation is 18/600/72% always and the reading 16/500/70%; in 歌词 the reading is closer to its line than the translation. This departs from Folia's "the first row is louder". |
| 2026-11-24 | The -460 on every song's address (检测到您的网络环境存在风险) was the eapi's User-Agent, an iPhone's, beside headers saying a Windows PC: measured signed out, that shape was refused on every try, and a Windows User-Agent of the same version, or an iPhone's with iPhone headers, or the web's endpoint, was not. It is now built from the headers' `appver`, so the two can't drift apart. |
