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

0.1.28 did not ship until each of these was done and its result written here, but for two that were not done (an external display, an Intel Mac): see Decisions, 2026-11-24. They were skipped for 0.1.27.

| Check | Where | State |
|---|---|---|
| The notch on a Mac with a notch: card, greeting, lyrics under the notch (with 0.1.28's rows and window), intro and outro, Atoll making way, an external display | [`notch.md`](notch.md#on-a-mac-before-a-release), steps 1-6 | done 2026-11-24 for 1, 2, 4 (outro), 5 and 3's rows with a translation; two bugs fixed (Atoll's quit and launch); making way for Atoll and taking the notch back after it quits checked again by hand on MyGo v0.10.0. Then, by hand, by the listener, 2026-11-24: a reading under the notch (3 行 with 音译), a click through the lyrics onto what is under them, and a song with a long intro: all pass. Step 6, an external display, was skipped and not done |
| Podcast captions on a Mac: the model and libraries download, and an episode is transcribed | [`lyrics.md`](lyrics.md#podcast-captions) | Apple silicon, 2026-11-24: libraries and model download, load and transcribe a minute in 24 s (`TestTranscribe`, English speech; its punctuation check wants Chinese); `TestNativesLive` passes. Then, by hand, by the listener, 2026-11-24: 下载模型 in settings finishes, an episode's captions appear in its 字幕 tab and follow playback, and the model is not downloaded again after quitting: all pass. Intel not done |
| 0.1.27's macOS build opens, after the quarantine step its notes give | the release page | done 2026-11-24, by the listener: it opens |
| The `NM_FRAMES` frames looked at on macOS, with its fonts: 青鸟 in Hiragino Mincho, AppleMyungjo and Songti, and the line breaks there | [`stage.md`](stage.md#looking-at-it) | done 2026-11-24, Apple silicon, `LIVE` set: 106 frames. The three fonts are installed and show as themselves (a Mincho 風が吹く, Myungjo Hangul, Songti Chinese), the sans fallback works, a long Japanese line breaks in two even rows between phrases, a one-character line keeps its translation on one row, one glass shows whatever the cover, and 和鸣 holds its line, reading, translation and gold word over six real covers. Small things, none blocking: the translation's last glyph sits 4 to 11 px past the line's (tracking at the end of the line), and a line's translation lingers, faint and blurred, under the next line for a moment, as the table has it. Motion is judged in a window, not in stills |
| `TestLyricTextsLive` signed in, for the one lyrics request (`7c8f425`) | `internal/netease/live_test.go` | done 2026-11-24: passes signed in |
| A song that isn't cached plays, signed in: the eapi's User-Agent now matches its headers, and `SongURL` asks again after a -460, then the web's endpoint | `eapi.go`, `netease.go`, `player.go` | done 2026-11-24, `3fff848`. Signed in, the same day, side by side: the old shape was refused 30 times in 30 and the new one never, nor in about 170 more requests (song addresses at three levels, in bursts, among web calls, and every read-only eapi call); `TestSongURLSignedInLive`. A refusal that still gets through is asked for again by itself after 3, 10 and 30 s while the listener wants the track, and the log says which try was refused (`netease: song … refused (…)`). The -460 seen in the app afterwards was an old build launched by mistake, not the fix. |

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
| `settings.json` written to a temporary file and renamed | P2 | `settings.go`, `save` | done, `7033602` |
| A cover that failed on the network is asked for again after a while | P2 | `artwork.go`, `get`: after a minute | done, `7033602` |
| Downloads stop when their track does, and give up when stalled | P2 | `player.go`, `stop`; `stream.go` | stalls give up after 30 s: done, `7033602`. Closing at `stop` left: `out.Close` may already close the stream; unverified |
| The notch's levels at 30 frames a second | P2 | `ui_notch.go`, `ui_pages.go`, `barFrame` | done, `7033602` |

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

## 6. Plugins

Sources of music as plugins, from [the plugins page](https://mygo-music.egoist.dev/plugins), with NetEase built in as the first source. The plan, what is ported from `egoist/mygo-music` and the decisions are in [`plugins.md`](plugins.md). Not in 0.1.28: they are for the next release, whose notes are drafted in `CHANGELOG.md` as 0.2.0 (the number is a proposal; `mygo.json` is not bumped until the release commit, [`RELEASING.md`](../RELEASING.md)). All four stages are on `feat/plugins`, pushed, not merged to `main`.

| Stage | State |
|---|---|
| 1.1 MyGo v0.10.0 (from a v0.7 snapshot) | done 2026-11-24: builds, `go test ./internal/...` passes, the window draws as before, and the notch making way for Atoll and taking the notch back when it quits was checked by hand on it |
| 1.2 `internal/music`: types, `Source`, errors, readings, `ErrRefused`, `Unsupported` | done 2026-11-24, ported from upstream; the app runs on it since 1.3 |
| 1.3 The views on `music.Track`, a slice at a time: player, lists and search, library pages, account and likes and comments, podcasts and stations, NetEase's extras | **done** 2026-11-24, on `feat/plugins` (not pushed): the player, search, the playlist, album and artist pages and their editing, the account and sign-in, comments, podcasts, the radios, Now Playing and every view of what plays run on `music.Track` and a source's IDs, through `netease.Source`; a song of another source shows its cover, opens its artist and plays through its own source. What stays NetEase's by design: its home page, users and events, suggestions, an artist's newest songs, albums and events, and the recent plays a login picks up ([`plugins.md`](plugins.md)). The tests, the stable frames, `mygo vet` and the six cross-builds are green; the frames the clock does not move are drawn as before |
| 2 The plugin host (`internal/plugin`, moejs), source settings | **done** 2026-11-25, on `feat/plugins` (not pushed): the host runs upstream's demo plugin as made, on moejs `v0.1.0-alpha.11` (Apache-2.0, no runtime dependency of its own, about 12 MB more in the binary, builds for all six targets), with its sandbox pinned by tests; the player plays what a plugin sends (Ogg Vorbis, WAV, ALAC, AAC, `file://`, a stream's headers) and the app registers the plugins of `plugins/` as sources, `Player.Source` finding them, with a settings group for each, its sign-in (a web page in an isolated session, or the prompt's words), a source switcher with the plugin's own sidebar, home and pages, search, hearts, comments and playlist edits through the plugin; stations, shows, saved albums and suggestions for a plugin are drawn when there is a place for them |
| 3 Install from GitHub, zip, folder or `mygo-music://` link; updates; a plugins list | **done** 2026-11-25, pushed: the installer (`internal/plugin/install.go`) and the app's flow: a plugins group in the settings with a + that installs from GitHub, a folder or a zip, browses the plugins page and shows the folder; what comes is asked about first (where from, what it may reach, the programs it asks to run); a plugin is a source at once, and an update replaces it live, asking again if it wants more than was allowed and taking back the programs the old was let run; removal to the trash, its data staying; a daily check for those from GitHub; `mygo-music://install-plugin?repo=owner/repo` (and `mygo-music-dev://`), which only opens the dialog. Against GitHub's real API: a repository's release and a missing one answer as the tests' stand-in does |
| 4 README, release notes | **done** 2026-11-25, local: the README's plugin section, this table, `plugins.md`, and 0.2.0's notes in `CHANGELOG.md`, which say plugins run with the permissions the listener allows |

Checked by hand, by the listener, on a Mac, 2026-11-25: with upstream's demo plugin copied into the development app's `plugins/` (light and dark; the switcher; the plugin's group in the settings with its version, permissions, sign-in and a setting; browsing; switching sources while NetEase plays; signing in to the demo), which found four things, all fixed and checked again (`ac4fa3f`: a missing `file://` song said the network failed and `#`, `?` and `%` in a folder's name broke the path; the home's entry had no label; large covers with no picture were blank; the search field listed podcasts for a plugin); then a real install from GitHub of `VenusLuna/mygo-music-netease-plus` (the hosts it may reach, its repository and version, its sign-in, with no restart), update checks, removal to the trash, a folder and a zip, and a `mygo-music-dev://install-plugin?repo=…` link. The daily check, which runs by itself, was not among them: the listener's checks used each plugin's menu, and the daily one was found never to have been started, while the release notes were drafted, and is started now as the window comes up (`windowUp`). **Not checked by hand:** that daily check on its own, Windows and Linux (the builds pass for all six targets, and the tests run off macOS; the sign-in window's web view, the native dialogs, the trash and the link's hand-over to the app need each system), and the two checks 0.1.28 left open (§2). Whether that blocks the release is for the release's review.

## 7. Later

- **Structure.** Split the notch's state into an explicit mode; give `app` a constructor that tests share; one `lyrics.Source(texts)` conversion; caption sizes in one place.
- **Tests.** Seams read before goroutines start, so `-race` passes; the player's network behind seams, so the unit run makes no requests; frame tests that need NetEase behind `LIVE` (with it set on 2026-11-24, `TestBackdropDriftFrames` and `TestSmokeFrames` failed: a signed-out search for 春日影 MyGO found no songs); the wall tests shorter. One `internal/app` run of about a dozen failed on Linux on 2026-11-17, its output not kept: likely a late goroutine from a test seam. Keep the output the next time it happens.
- **Dependencies.** MyGo is v0.10.0, a tagged release (2026-11-24; `go tool mygo migrate-ui` had nothing to change). The README asks for Go 1.27.3.
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
| 2026-11-24 | 0.1.28 ships without two checks, the notch on an external display set as the main one and podcast captions on an Intel Mac: neither was done. Its notes say so. They stay open, for a later release. Plugins are not part of 0.1.28: they are on `feat/plugins`, unreleased. |
| 2026-11-24 | 0.1.28 released: tag `v0.1.28` at `8d2b75a`, its Test and Release runs passed on Linux, macOS and Windows, and the update feed answers it with a delta from 0.1.27. Its notes name nothing of what 和鸣 is after, and say nothing of plugins. |
| 2026-11-25 | Plugins stay on `feat/plugins` until the listener reviews them; the next release's notes are drafted as 0.2.0, a proposal. |
| 2026-11-25 | The host's installer does not look for the listener's GitHub token (`GITHUB_TOKEN`, `GH_TOKEN`, then `gh auth token`, as upstream does): they never gave this app one, and public repositories install without it. The cost is GitHub's 60 requests an hour without one; a private repository does not install. Kept as decided, 2026-11-25. |
| 2026-11-25 | A `mygo-music://install-plugin` link opens the dialog and fetches nothing, where upstream downloads at once to ask: a page can make the app show a question, not download. Kept as decided. |
| 2026-11-25 | An update that adds programs a plugin may run takes back the listener's allowing of programs, so that they allow the new list again; upstream keeps it. Kept as decided. |
| 2026-11-25 | Plugins from GitHub are checked for a new version once a day, with no switch to turn it off, one request each to GitHub's API; installing one is the only thing that makes the app ask GitHub about it. Removing a plugin keeps what it stored. Both kept as decided. |
| 2026-11-25 | The licences accepted for plugins: moejs (Apache-2.0, alpha, about 12 MB in the binary) and, for Ogg Vorbis and AAC, `jfreymuth/oggvorbis` (MIT) and `tphakala/go-aac` (LGPL-2.1-or-later, ported from FFmpeg, as MyGo's own `audio/aac` links it), which the README credits. |
