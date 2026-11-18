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
| The notch on a Mac with a notch: card, greeting, lyrics under the notch, intro and outro, Atoll making way, an external display | [`notch.md`](notch.md#on-a-mac-before-a-release), steps 1-6 | pending |
| Podcast captions on a Mac: the model and libraries download, and an episode is transcribed | [`lyrics.md`](lyrics.md#podcast-captions) | pending |
| 0.1.27's macOS build opens, after the quarantine step its notes give | the release page | pending |
| The `NM_FRAMES` frames looked at on macOS, with its fonts | [`stage.md`](stage.md#looking-at-it) | pending |
| `TestLyricTextsLive` signed in, for the one lyrics request (`7c8f425`) | `internal/netease/live_test.go` | pending |

## 3. Lyrics, for 0.1.28

| Item | Pri | Where |
|---|---|---|
| Word-timed lines keep words in parentheses, "(oh" and "(Ah)" | P1 | `lyrics.go:75` |
| Lines without word timing keep the space after Han and Hangul once lit | P1 | `SpreadWords` |
| The line before doesn't come back after a long break, in every style | P1 | `ui_stage.go:139`, `:174`; `ui_stage_bird.go:105` |
| The lyrics under the notch come in with the first line and go with the last | P1 | `ui_notch.go:628` |
| 青鸟's translation arrives after its line, and answers from the right | P1 | `ui_stage_bird.go:136-219` |
| Two LRC lines at one time: one line with its translation, and keys by index | P2 | `lyrics.Build`, `ui_stage.go:280` |
| `Lang` on the line's own text in 歌词, not on its row | P2 | `ui_nowplaying.go:554` |
| A lyrics request that fails says so, with 重试, and is tried again once | P2 | `player.go:604` |
| `settings.json` written to a temporary file and renamed | P2 | `settings.go:144` |
| A cover that failed on the network is asked for again after a while | P2 | `artwork.go:129` |
| Downloads stop when their track does, and give up when stalled | P2 | `player.go:560`, `stream.go:100` |
| The notch's levels at 30 frames a second | P2 | `ui_notch.go:513`, `ui_pages.go:449` |

## 4. Typography and 青鸟

- One hierarchy for the reading and the translation on every surface, by role ([`lyrics.md`](lyrics.md#where-they-show)).
- A line breaker in the app: balanced rows, never inside a word, Korean at spaces.
- A registry of stage styles that the menu, `stage()` and Now Playing read, in place of comparing ids ([`stage.md`](stage.md#adding-a-style)).
- 青鸟 v2: one glass for every cover, captions at 4.5:1, the answer flush right and timed from the first word, a Korean serif.

## 5. A style after 『響け！ユーフォニアム』

A sibling of 青鸟 on the same engine, from what the makers said rather than from impressions: high contrast, a backlight behind the line, rack focus as a line comes in, one gold on the word sung, the translation in unison on the line's axis. Look at its frames on real covers before it ships. Never anything from the works themselves: see the review's §5.

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
