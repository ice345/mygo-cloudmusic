# Implementation guide

This goes with [`next.md`](next.md), which says what to build and why, with the evidence. This file says how: in which order, on which branch, which files and functions, which tests, and what the commits look like. Section numbers like "§3.3" point into `next.md`.

## 0. Before you start

**The tree isn't clean.** On top of `main` at `3982879` (0.1.26) there is uncommitted work:

- `internal/netease/netease.go`, `internal/player/player.go`
- `internal/lyrics/lyrics.go` and `lyrics_test.go`
- `internal/app/settings.go`, `ui_nowplaying.go`, `ui_stage.go`, `ui_stage_styles.go`
- `internal/app/stage_test.go`, `like_test.go`, `perf_test.go`, and the new `lyric_script_test.go`
- the untracked `docs/`

Keep it on a branch of its own, and start every work package from a clean `main`:

```bash
git switch -c wip/reading-caption
git add internal/
git commit -m "wip: lyrics' reading, 歌词显示 and the stage's caption, as found on 2026-11-05"
git switch main
```

`docs/` stays untracked through this. Commit `docs/next.md`, `docs/implementation.md` and, if wanted, `docs/aobird-preview.html` in a commit of their own. Never commit `docs/aobird-frames/` or `docs/stage-caption/`: `NM_FRAMES` writes them again.

To take a piece of the work into a package: `git restore --source=wip/reading-caption -- <file>` for a whole file, or `git checkout -p wip/reading-caption -- <file>` for hunks.

**Read first:**

- `AGENTS.md` (tests)
- `.agents/skills/mygo-maintenance/SKILL.md` (MyGo lifetime, keys, threads)
- MyGo's `ui/doc.go` at the pinned version (`go list -m -f '{{.Dir}}' github.com/egoist/mygo`)
- `next.md` §4.1, the conventions

**Commands:**

```bash
go test ./internal/...                                          # before every commit
go test ./internal/app -run 'TestName' -count=5                 # every new UI test
go tool mygo vet .                                              # stored elements, goroutine captures
NM_FRAMES=$TMPDIR/frames go test ./internal/app -run 'Frames'   # frames to look at
LIVE=1 go test ./internal/netease -run 'Live'                   # NetEase itself
go tool mygo dev                                                # the app
```

## 1. Conventions you must keep

1. **The stage is a function of `pos`.** Every opacity, blur and offset on the stage is worked out from `pl.Position()` and the lines; no state, no timers. Seeking must land on the frame playing would.
2. **Identity.**
   - A line's layer is keyed `c.Key(l.At)`.
   - Stateful widgets get `c.Key(...)` before construction.
   - Never store an `ui.Element`; keep data, `ui.Handle`s or scroll states.
   - Geometry in a later callback comes from a handle, or from `LayoutBounds`, which is the **previous** frame's layout.
3. **Nothing moves because something else changed.** A toggle, a badge or a missing row must not move what's around it. Anchor at the edge that shouldn't move: the foot band is anchored at `Bottom(24)`, so it grows upward.
4. **Controls.**
   - Pills are `ui.ButtonBase(...).Padding(6, 14).Radius(999)` with 13 pt / 600 text: 55% white at rest, 6% background on hover, 14% when on.
   - Menus use `b.Menu(func(m *ui.Menu))` with `Checked`/`Disabled`.
   - Settings use `row(label, desc, control)` with `ui.Switch` or the app's `segmented`.
   - Don't make a new kind of control.
5. **Settings.** A field with a full-sentence comment; "the listener turns it on" for anything opt-in. `omitempty` for zero defaults. Defaults in `defaultSettings`. `a.settings.save()` right after a change. `loadSettings` decodes onto the defaults, so a new key defaulting to true is safe for people upgrading.
6. **Frames.** Ask with `c.AnimationFrame()` only while something visibly changes. Otherwise use `c.After(d)` until the next change. Budgets live in `perf_test.go`.
7. **Comments.** Full sentences about behaviour in the listener's terms. The function comment starts with its name and names the parameters in prose ("stageCaption sets l's reading…"). Match the density around it. Say why where a reader would ask.
8. **Platform code.** macOS in `*_darwin.go` with `purego/objc`, stubs in `*_other.go`. Test seams are package variables (`cursorPoint`, `likeSong`, `copyText`), swapped in tests and restored with `defer`.
9. **Threads.** Network on goroutines; results come back through `p.post` / `Window.Update`. Never block the UI thread.
10. **Tests** follow `AGENTS.md` exactly:
    - behaviour that can break, not labels or settings defaults;
    - a one-sentence comment first;
    - for a regression, what went wrong;
    - `-count=5` for new UI tests;
    - no wall clock.

## 2. Branches and commits

**Branches.** `main` is the trunk; upstream commits land on it directly, and history is linear (3 merges in 158 commits). Work on one topic branch per work package, cut from `main`. Rebase it on `main` before merging, and merge fast-forward. Delete it after. Names:

| WP | Branch |
|---|---|
| 0 | `chore/fork-release` (the owner's) |
| 1 | `lyrics/texts` |
| 2 | `lyrics/reading-ui` |
| 3 | `stage/caption-band` |
| 4 | `notch/make-way` |
| 5 | `notch/lyrics` |
| 6 | `stage/aobird` |
| 7 | `fix/<topic>`, one each |

An agent's branch may follow upstream's pattern `claude/<topic>-<n>`.

**One commit, one behaviour.** A commit is one thing a listener, or the code's reader, can tell apart. It comes with its tests and passes `go test ./internal/...` by itself. Not one commit per file, not "wip", and no formatting changes mixed into behaviour. A regression fix found along the way is its own commit, before the feature that needs it.

**Message.**

- The subject is one line in English, all lower case except names. It starts with a verb: add, let, keep, show, set, make, move, ask, give, write. Chinese names of UI as they appear (`add 歌单 to search`).
- After a colon, say what went wrong before, or why. Length is fine: upstream's median subject is 150 characters.
- An optional body of `- ` lines lists what else changed (API, files, tests).
- AI-assisted commits end with a `Co-Authored-By:` trailer, as upstream's do.

Good:

```
give a word-timed line the translations of all the timed lines it sings: NetEase's word-timed lyrics merge some of its timed lines, and the second one's translation, starting two seconds after the merged line, reached no line at all
```

Bad: `fix lyrics`, `feat(lyrics): add romanization support`, `WIP`, `update ui_stage.go`.

**CHANGELOG** isn't touched in feature commits. It's written at release, in Chinese, for listeners (`RELEASING.md` step 2), in the commit titled only with the version (`0.1.27`), which also bumps `mygo.json`.

**Never commit** frames (`*.png` from `NM_FRAMES`), `build/` or `.mygo/`.

## 3. Order

| Order | WP | Depends on | Ships in |
|---|---|---|---|
| — | 0. Fork and release | — | before the next release |
| 1 | 1. Lyrics data | — | 0.1.27 |
| 2 | 2. Lyrics list and the pill | 1 | 0.1.27 |
| 3 | 3. The stage's foot band | 1 | 0.1.27 |
| 4 | 4. The notch makes way | — | 0.1.27 |
| 5 | 6. 青鸟 | 3 | 0.1.28 |
| 6 | 5. Lyrics under the notch | 1, 4 | 0.1.28 or later |
| any | 7. Small fixes | — | whenever |

## 4. Work packages

### WP0. Fork and release (owner's decisions; no feature code)

1. Choose the repository's name. Either rename `ice345/cloudmusic` to `mygo-cloudmusic`, or change `mygo.json` `updates.github` to `ice345/cloudmusic`. Until then the updater looks at a 404 (`next.md` §7).
2. `origin` is empty, and its default branch setting is `master`. Push `main` (`git push -u origin main`), then make `main` the default on GitHub.
3. `RELEASING.md`: change the feed URL in step 6 and the website check in step 7 to this repository's, or mark them as upstream's.
4. macOS builds are unsigned now (`c1fb036`). Add a line to the release notes on opening a quarantined app. If you sign later with your own Developer ID, the podcast libraries from upstream's `sherpa-onnx-2.0.1` must be re-signed and hosted here (`RELEASING.md`, "The speech recognition libraries"); `internal/speech/natives.go` `libBase` points at upstream.

Commit: `point updates at this repository's releases: …`, plus a separate one for `RELEASING.md`.

### WP1. Lyrics data (`lyrics/texts`)

**Goal.** Every line gets all its translation and reading, including when YRC merges or splits LRC lines. Readings follow the per-language rules (§3.2–3.3). Lines carry their language.

**Files.**

- `internal/netease/netease.go` (`Lyrics`)
- `internal/lyrics/lyrics.go` and `lyrics_test.go`
- `internal/player/player.go` (`loadSkipping`'s lyrics goroutine)
- every `lyrics.Build` call in tests: `like_test.go`, `perf_test.go`, `stage_test.go`, `lyric_script_test.go`, and `mergeLyrics`

**Steps.** Each numbered step is a commit.

1. **Fix the alignment, on today's two inputs, before anything else.** Replace `align` (nearest start within 1.5 s, first one wins) with "belongs" and joining, as below, for the translation only.
   - Test: the 残響散歌 shape (a merged line and a split line), built from literal strings and no network.
   - Commit: the "give a word-timed line the translations…" example above.
2. **Ask for every text NetEase sends, and return them together.**
   - `netease.Lyrics(id) (LyricTexts, error)` and `lyrics.Build(lyrics.Source) []Line`.
   - Decode `romalrc`, `ytlrc` and `yromalrc`.
   - Take `Line.Reading` from the WIP branch.
   - Commit: `ask NetEase for lyrics' readings with their translations, and for both as timed by the word-timed lines where it has them`.
3. **Prefer the word-timed `y*` texts on YRC lines, and fill the rest from the LRC-timed ones.** Readings are kept only where they say something (`spoken`, `echoes`). Extras with no letters or digits are dropped. For Cantonese, see §3.1 point 4.
   - Commit: `take a word-timed line's translation and reading from those timed by it, then from those of the timed lines it sings: …`.
4. **`Line.Lang` and `ReadingName`.**
   - Commit: `tell Japanese and Korean lines apart, and name a song's reading as 罗马音, 粤拼 or 拼音`.

**Sketch** (adapt the wording to the files' voice):

```go
// internal/netease/netease.go

// LyricTexts are a track's lyrics as NetEase serves them, each in its own
// format; any of them may be empty.
type LyricTexts struct {
	// LRC is the timed lines; Translation and Reading are timed by them.
	LRC, Translation, Reading string
	// YRC is the word-timed lines, which only the desktop client's API
	// serves. They may merge or split LRC's lines; YTranslation and
	// YReading are timed by YRC's own.
	YRC, YTranslation, YReading string
}

// Lyrics returns a track's lyrics. The reading is how its lines are read
// aloud, as NetEase writes it for the song's language: romaji for
// Japanese, its own spelling for Korean, Jyutping for Cantonese, and
// nothing for Mandarin, English and most others.
func Lyrics(id int64) (LyricTexts, error)
```

The web call keeps `lv, tv, rv = 1, -1, -1` and decodes `lrc`, `tlyric` and `romalrc`. The eapi body stays as it is (`rv: 0` already returns `romalrc`) and decodes `yrc`, `ytlrc` and `yromalrc`. Explicit `json:"…"` tags. A failing eapi call still leaves the web texts.

```go
// internal/lyrics/lyrics.go

// Source is a song's lyrics as NetEase serves them (netease.LyricTexts).
type Source struct {
	LRC, Translation, Reading   string
	YRC, YTranslation, YReading string
}

// belongs returns the line an extra line at at belongs to, or -1: of the
// two lines starting nearest it, no later than half a second after it,
// the nearer that is still sung then. Word-timed lines start a few
// hundred milliseconds off the timed lines extras are written against,
// and some of them sing two.
func belongs(lines []Line, at time.Duration) int

// extras returns, for each of lines, the text of extra's timed lines that
// belong to it, joined by spaces in their order. Lines of no letters or
// digits, as "//", are none.
func extras(lines []Line, extra string) []string

// echoes reports whether reading says no more than text: the same letters
// and digits, whatever their case, spacing and punctuation, as NetEase's
// reading of an English line often is.
func echoes(text, reading string) bool

// spoken reports whether text has letters a reading can say: any outside
// the Latin script.
func spoken(text string) bool
```

`belongs`:

1. `i := sort.Search(len(lines), func(i int) bool { return lines[i].At > at+500*time.Millisecond }) - 1`.
2. Take `i` and `i-1` as candidates. A candidate holds `at` if `lines[k].At-500ms <= at && at < lines[k].End+200ms`.
3. Of those that hold it, return the one starting nearer to `at`; -1 if none.

`Build(s Source)`:

1. Lines from YRC, else LRC, unchanged.
2. If they came from YRC: `tr, rd := extras(lines, s.YTranslation), extras(lines, s.YReading)`.
3. For each line, an empty `tr[i]` or `rd[i]` takes `extras(lines, s.Translation)[i]` or `extras(lines, s.Reading)[i]`.
4. A reading is kept only `if spoken(l.Text) && !echoes(l.Text, r)`. A translation is dropped when `echoes(l.Text, t)`.
5. Set `Lang`.

```go
	// Lang is the language the line is drawn in, as a BCP 47 tag, where
	// it is not the app's: "ja" for Japanese, so that its Han characters
	// draw as Japanese ones do, or "ko"; else "".
	Lang string
```

`Lang` is decided for the song:

- If at least `max(2, sung/5)` lines (credits excluded, `IsCredit`) have Hiragana or Katakana, every line with kana or Han is `"ja"`.
- A line with Hangul is `"ko"`.
- Otherwise `""`.

```go
// ReadingName names the reading lines carry, for the menu that turns it
// on: 罗马音 for Japanese and Korean, 粤拼 for Cantonese, 拼音 for
// Mandarin, or "" when no line has one.
func ReadingName(lines []Line) string
```

- Any line with a `Reading` and `Lang` of `ja` or `ko` makes it 罗马音.
- Otherwise, readings on Han-only lines are 粤拼 if at least 5% of their syllables end in p, t or k, contain `aa`, `eo` or `oe`, or start with `ng` plus a vowel. Else 拼音.

**Tests** (logic only, `internal/lyrics`; each starts with its one-sentence comment):

- **Merged.** YRC `[37450,4400]` sings LRC 37.64 and 39.84. The line's translation is both, in order; the next YRC line's is its own. Comment the regression: "the second one reached no line".
- **Split.** YRC `[71290,2000]` and `[73330,1500]` against LRC 71.49. The first takes the LRC translation, and the second takes `yromalrc`'s reading at 73.33.
- **Late.** A YRC line starting 270 ms after its LRC line still gets its texts (富士山下's case). Two LRC lines 300 ms apart keep their own.
- **Echoes.** In a Japanese song, an English line's reading `"Hey! I love you"` against the text `"hey, I love you"` is dropped. A Korean line keeps `zeo gi meol ri from Earth to Mars`. A Thai line with no reading stays empty.
- **Lang.** A Japanese song's Han-only line is `"ja"`. A Mandarin song's lines are `""`. A Korean line is `"ko"`.
- **ReadingName.** It returns 罗马音, 粤拼 (from `jyu joek nei fei ngo bat gaa`), 拼音 (from `ni hao shi jie`) and `""`.
- Add `TestLyricTextsLive` to `internal/netease/live_test.go`, skipped without `LIVE`: for 536622304, 2120465975 and 65766, `Reading` isn't empty; for 3427209483, `YReading` isn't empty.

**Done when:**

- all of the above pass;
- `go vet` and `mygo vet` are clean;
- playing 残響散歌 shows a translation on every sung line, including at 0:37 and 0:51.

### WP2. Lyrics list and the pill (`lyrics/reading-ui`)

**Goal.** The 歌词显示 pill from the WIP branch, with the 音译 naming, per-song names, languages on the lines, and the lit line staying where it is.

**Files.** `internal/app/settings.go` (from WIP), `ui_nowplaying.go`, and `lyric_script_test.go` (from WIP, updated).

**Steps** (commits):

1. **Bring `LyricTranslation`, `LyricRoman`, `lyricReading`, `lyricTranslation` and `lyricScriptButton` over from WIP**, with these changes:
   - The menu items are 翻译 and 音译（name）, where the name comes from `lyrics.ReadingName(lines)` and the plain 音译 is used when there is none. (Check NetEase's current client first; if it says 注音, keep that.)
   - Fix the comments about Korean.
   - The reading row in the list is weight 500.
   - Commit: `let 歌词 and 律动 show each line's reading as well as its translation, chosen from a button beside the tabs: 音译 names the reading the song has, as 罗马音 or 粤拼, and is left as chosen for songs with none`.
2. **Keep the lit line in place.** Turning a row on or off re-aims the lit line using the new layout.
   - Commit: `keep the line sung a third of the way down as its translation or reading is turned on or off: the lines above it grew or shrank, and it went with them`.
3. **Languages on the lines.** `Lang(l.Lang)` on each line's container in `timedLines` (it covers `RichText` and `Text`), and on each stage line's layer (the `c.Key(l.At)` Row/Column in `stageLine`, `partitaLine` and `tiltLine`). `Lang` applies to the elements inside.
   - Commit: `set Japanese and Korean lyrics in their own language, so that their Han characters draw as Japanese ones do rather than as Chinese`.

**Re-aiming, and its trap.** `timedLines` aims when `cur != a.lyricAt`, measuring with `active.LayoutBounds()`, which is the previous frame's layout. Resetting `a.lyricAt` in the click handler would measure the old layout. Instead:

```go
// lyricRows are the rows the lines show besides their own, which change
// their heights: turned on or off, the line sung is aimed at again.
type lyricRows struct{ translation, reading bool }

	if rows := (lyricRows{a.settings.LyricTranslation, a.settings.LyricRoman}); rows != a.lyricRows {
		// Laid out anew in this frame, measured in the next.
		a.lyricRows, a.lyricReaim = rows, true
		c.AnimationFrame()
	} else if a.lyricReaim {
		a.lyricReaim, a.lyricAt = false, -2
	}
```

Look at the result in the app. If the jump before the glide shows, offset the scroll by the change in height above the lit line instead. Either way, the test below decides.

**Tests:**

- `TestLyricScript`, updated: the menu item `音译（罗马音）`, and the 「风在吹」 lines as now.
- New: "Turning 翻译 off keeps the lit line a third of the way down, where it was: the lines above it lost their translations, and it rose with them."
  - Setup: 12 lines, each with a translation; seek to the 8th.
  - `r0, _ := tt.Find(lines[7].Text)`; this finds the row, which is labelled by its text.
  - Click 歌词显示, then choose 翻译; `tt.Advance(time.Second)`; `tt.Frame()`.
  - Check `|r1.Y − r0.Y| ≤ 4`.
  - Run it with `-count=5`.

### WP3. The stage's foot band (`stage/caption-band`)

**Goal.** §4.4. The reading and translation go back to the foot, shared by 流光, 云阶 and 倾诉; the words return to their own centring.

**Files.** `internal/app/ui_stage.go`, `ui_stage_styles.go` and `stage_test.go`.

**Steps** (one commit):

1. `stageLine` and `partitaLine` go back to their `main` form. So does `tiltLine`: a function again, with its `sparks` parameter. None of them calls `stageCaption`.
2. In `stage()`, replace the single next-line `Text` with `a.stageFoot(c, lines, cur, pos, w)`, inside the `sung` box, so it's dimmed under a hot comment.

```go
// stageFoot sets, along the foot of the stage, the reading and the
// translation of the line sung, and under them the line coming next: in
// one place and at one size whatever the style, as Folia's shared
// subtitle does, so that turning them on or off, or a line without them,
// moves none of the words above. One line's are there at a time: the
// last line's go in the first 120 milliseconds of the next, whose come in
// after them, and in a long break they go with their line.
func (a *app) stageFoot(c *ui.Context, lines []lyrics.Line, cur int, pos time.Duration, w float32)

// stageCaption sets l's reading and translation as rows of the stage's
// foot, shown as far in as shown: the first at the size the translation
// always had there, the second a step quieter, as Folia stacks them.
func (a *app) stageCaption(c *ui.Context, l *lyrics.Line, shown, w float32)
```

**`stageFoot`:**

- `ui.Column(c.Key("foot")).Absolute().Left(0).Right(0).Bottom(24).AlignItems(ui.Center).Gap(8).PassThrough()`. The caption comes first and the next line last, so the next line stays put and the caption grows upward.
- `since := pos − lines[cur].At`. The caption shown:
  - `since < 120ms && cur > 0`: the previous line's, at `1 − since/120ms`;
  - in a long break (the same test as the stage's interlude): this line's, at `1 − (pos − End − 1s)/400ms`;
  - otherwise: this line's, at `easeOutCubic((since − 120ms)/250ms)`.
- While one fades and playback runs, `c.AnimationFrame()`.

**`stageCaption`:**

- Its rows are the non-empty ones of `a.lyricReading(*l)` and `a.lyricTranslation(*l)`, in that order.
- `ui.Column(c.Key(l.At)).AlignItems(ui.Center).Gap(4).MaxWidth(w*0.72).Opacity(shown).Translate(0, 6*(1-shown))`.
- The first row: 18 pt, 600, `white.Alpha(0.72)`. The second: 18×0.88, 500, `white.Alpha(0.72×0.82)`. Both `TextAlign(ui.Center).MaxLines(2)`.
- The next line stays as on `main`: 14 pt, 32%, `SingleLine`.

Commit: `set the stage's reading and translation along its foot for every style, where Folia's shared subtitle sets them: under the line, they lifted its words as they came and dropped them as they went, and 流光 set them off to the left`.

**Tests:**

- Replace `TestStageCaptionFollowsTheLine` with: "Turning the translation and the reading off and on moves none of the stage's words, in any style: under the line, they lifted it as they came."
  - For each of `""`, `partita` and `tilt`: lyrics with reading and translation, `a.pl.Hold` at a moment the first word has settled, and `tt.Find("風")`.
  - Set both settings false, `tt.Frame()`, find again; then both true, find again. The boxes match within 0.5 DIP.
  - `-count=5`.
- Keep `TestStageCaptionFrames` and look at its output. The caption is centred in all three styles, sits clear of the next line, and wraps.
- `TestNowPlayingFrameBudget` must still pass unchanged.

### WP4. The notch makes way (`notch/make-way`)

**Goal.** §6.3.

**Files.** `internal/app/notch_darwin.go`, `notch_other.go`, `ui_notch.go`, `ui_settings.go` and `notch_test.go`.

**Sketch:**

```go
// notch_darwin.go

// notchApps are apps that draw at the notch as this one does, by the
// start of their bundle identifiers: a panel at the main menu's level and
// above it, opening as the pointer rests there. With one running, both
// opened on the same spot, one over the other.
var notchApps = []string{
	"com.Ebullioscopic.Atoll", // and its .dev builds
	"theboringteam.boringnotch",
	"wiki.qaq.NotchDrop",
	"iordv.Droppy",
}

// otherNotchApp returns the name of a running app that draws at the
// notch, or "" if none does.
func otherNotchApp() string

// watchApps calls fn on the main thread as apps launch and quit.
func watchApps(fn func())
```

`otherNotchApp` goes through `NSWorkspace.sharedWorkspace.runningApplications`, reading `bundleIdentifier` and `localizedName` as C strings (`UTF8String`).

`watchApps` adds observers to `NSWorkspace.sharedWorkspace.notificationCenter`, with `addObserverForName:object:queue:usingBlock:` on `NSOperationQueue.mainQueue`, for `NSWorkspaceDidLaunchApplicationNotification` and `NSWorkspaceDidTerminateApplicationNotification`. The names are NSString constants. Make them with `NSString stringWithUTF8String:` from their names, and check at run time that a launch really calls back. Keep the blocks retained, as `watchPointer` does.

`notch_other.go`: `func otherNotchApp() string { return "" }` and `func watchApps(fn func()) {}`.

```go
// ui_notch.go

// notchNeighbor finds another app drawing at the notch; tests swap it.
var notchNeighbor = otherNotchApp

	neighbor string // another app drawing at the notch, which this one makes way for
```

**`ui_notch.go`:**

- `newNotch`: `n.neighbor = notchNeighbor()`; `watchApps(n.lookAround)`; and in `OnDidBecomeActive`, also `n.lookAround()`.
- `lookAround`:
  - sets `n.neighbor` again;
  - if it changed: closes the card (`n.open = false`), lets clicks pass (`SetIgnoreMouseEvents(true)`), calls `n.watch()` and `n.poll()`, and invalidates both windows (the settings hint).
- `target`: a first case `case n.neighbor != "": return 0, 0`, so the outline springs into nothing.
- `poll`: return early, passing clicks through, while `n.neighbor != ""`.
- `watch`: `on := (n.a.settings.Notch || n.hello) && n.neighbor == ""`.
- `greet`: return before setting `NotchGreeted` while `notchNeighbor() != ""`.

**`ui_settings.go`:** while `a.notch.neighbor != ""`, the hint of 显示刘海播放器 reads 「<name> 正在使用刘海，刘海播放器暂不显示；正在播放的歌曲会显示在 <name> 中。退出 <name> 后自动恢复。」. No new switch.

Commit: `make way at the notch for another app drawing there, as Atoll and boring.notch do: both opened on the same spot under a resting pointer, one over the other, and the greeting grew over theirs`.

**Tests** (`notch_test.go`, swapping `notchNeighbor` and `cursorPoint` with `defer`):

- "With another app at the notch, the notch neither greets the first song nor opens under a resting pointer; once that app quits, it opens again."
  - Set the neighbor to "Atoll". `a.syncWindows()` after the song plays, as `TestNotchGreetsOnce` does, gives `!n.hello && !a.settings.NotchGreeted`.
  - Put the cursor on the housing; `n.poll()`; `n.enteredAt = n.lastInside.Add(-time.Second)`; `n.poll()`. Then `!n.open`.
  - Set the neighbor to `""` and `n.lookAround()`; repeat the rest; now `n.open`.
- `-count=5`.

Manual check with Atoll installed: launch and quit Atoll with the notch on, and confirm only one island at a time.

### WP5. Lyrics under the notch (`notch/lyrics`)

**Goal.** §6.4.

**Files.** `settings.go`, `ui_settings.go`, `ui_notch.go`, `notch_test.go` and `perf_test.go`.

**Steps:**

1. **The setting.**

   ```go
   	// NotchLyrics is how many lines of the lyrics hang under the notch, 0
   	// to 3: they cover the top of the window under them, so the listener
   	// turns them on.
   	NotchLyrics int `json:"notchLyrics,omitempty"`
   ```

   In 刘海播放器, `row("刘海歌词", "在刘海下方显示正在唱的歌词，不会遮挡菜单栏", func() { segmented(...) })` with 关, 1 行, 2 行 and 3 行, disabled while `!s.Notch`.
2. **The strip.**
   - `n.strip()` returns its height: 0, 28, 50 or 72 DIP. It's 0 while the card is open, while making way, with no current track or lyrics, or in the outro.
   - `target()` adds it to `h` when closed.
   - `n.lyricStrip(c, shown)` draws in `body` under the compact row, clipped by it.
   - The current row is 13.5 pt, weight 600, filled word by word from 0.38 to 1. It shrinks to 11 pt to fit, then ends in an ellipsis.
   - Other rows are 11.5 pt at 55%. Rows are chosen as in §6.4, reading the shared 翻译/音译.
   - In a break of 5 s or more, from 1 s after the last word, the next line shows unfilled at 38%.
   - `Lang(l.Lang)` on the strip.
3. **The pointer passes through.** In `poll`, opening is decided over the housing and wings only (`n.h0` tall), not over the strip. `pass` stays true over the strip. `n.under` (the pointer over the strip) fades the strip to 20%.
4. **Frames.** `c.After(40 * time.Millisecond)` while a word fills, as the lyrics list does; otherwise `c.After` until the next word or line.

Commits:

- `let lyrics hang under the notch, a line to three, from 刘海播放器 in settings: the line sung lit word by word, its reading or translation under it where turned on, as the shell grows down out of the notch and never sideways over the menu bar`
- `let the pointer through the lyrics under the notch, fading them while it is over them: resting there opened the player over whatever window was under them`

Separate the second only if it lands later.

**Tests:**

- "Resting the pointer on the lyrics under the notch opens nothing, and clicks pass through to the window under them."
  - Cursor at `n.screenY + n.h0 + 14`, inside the strip; poll twice, with the rest faked as in `TestNotchGreetingKeptUnderPointer`.
  - Expect `!n.open && n.passing`.
- In `perf_test.go`: "The notch's lyrics ask for no frames between words." With the strip on and the position held between two words, at most 2 frames a second. Build the notch view with `ui.NewTester(n.view, …)`.

### WP6. 青鸟 (`stage/aobird`)

**Goal.** §5.4. It needs WP3, since 青鸟 opts out of the foot band.

**Files:**

- new `internal/app/ui_stage_bird.go`, as `ui_quiet.go` and `ui_wall.go` hold one area each
- `ui_stage.go` and `ui_stage_styles.go`
- `backdrop.go` and `ui_nowplaying.go` (the tone)
- `stage_test.go` and `perf_test.go`

**Steps** (commits):

1. **The backdrop's tone.**
   - Add a `glass bool` to `a.backdrop`, `drawBackdrop` and the cache's key (`bd.glass`), so that a change fades as another song's does.
   - `glassColors(tint) (top, foot)`: the hue moved 70% of the way to 192°; saturation 0.20 at the top and 0.24 at the foot; value 0.40 and 0.25. A cover wash of 0.06. A 135° gradient from white at 8% to none at 55%.
   - In `nowPlaying`, `glass := a.npPane == paneStage && a.settings.StageStyle == "aobird"`. Work `accent` out from the cover's tint as now, before the tone.
   - The fog's colours take the tone the same way (`fogColors`).
   - It ships with step 2: no one can see it before.
2. **The style.**
   - `stageStyles` gains `{"aobird", "青鸟"}`. Fix the comment over it: 青鸟 is the app's own, not Folia's.
   - `stage()` branches early for it: `if a.settings.StageStyle == "aobird" { a.birdStage(c, tr, lines, loaded, pos, w, h, accent); return }`. It doesn't call `quietShown` (no hot comments), draws no foot band, and has its own frames.
   - `birdStage` draws, from `pos` alone:
     - the leaving line, then the current or condensing line;
     - its reading, then its answer (the translation);
     - or the hairline in breaks and the intro;
     - 「纯音乐，请欣赏」 for songs without lyrics.
   - All timings, sizes and tokens are as in §5.4, as named constants with comments. Name the tokens `birdInk`, `birdBreath`, `birdAnswer` and `birdRest`.
   - Family by `l.Lang`, through `.Font(...)`. With Reduce Motion (`c.Preferences().ReduceMotion`), opacity only.
   - `birdNextFrame`: `c.AnimationFrame()` while something changes, otherwise `c.After` until the next change, and never a fixed tick.
   - The 火花 menu item: `.Disabled(a.settings.StageStyle == "aobird")`. The setting is kept.

   Commit: `add 青鸟 to 律动效果, which leaves the stage quiet: a line forms whole out of a mist and stays still, its reading under it and its translation answering late, off to the right, as a second part would; a hairline holds the place through breaks, over the cover's colours turned to a pale, cool glass; it throws no sparks and shows no hot comments`.

**Tests:**

- "Choosing 青鸟 leaves 火花 as it was, and it can't be turned on there."
  - Choose 青鸟 from 律动效果; then `tt.ChooseMenuItem("火花")` either fails, or leaves `a.settings.StageSparks` unchanged.
  - Choosing 流光 again keeps it as it was.
- Extend the WP3 "moves none of the words" test to `aobird`, and add: "with the translation off, the line stays where it is".
- `perf_test.go`:
  - `TestNowPlayingFrameBudget` already loops over `stageStyles` (at most 12).
  - Add: "青鸟 asks for at most 1 frame a second once a line has formed, and in breaks."
  - Hold `pos` 3 s into the line, then 3 s into a long break.
- Frames, skipped without `NM_FRAMES`, written as `bird-*.png`:
  - mist, half-formed, formed;
  - the answer arriving, and the line leaving;
  - the hairline;
  - red, green and grey covers;
  - translation on and off, reading on;
  - a 1.2 s line, a long wrapped line, an English line;
  - a frame with the serif families replaced by one that doesn't exist, to check the sans fallback.

  `TestStageFrames` also produces `stage-aobird.png`. Review these with the owner before merging.

### WP7. Small fixes (each its own `fix/…` branch and commit)

- **One lyric request a song** (P2). Use the eapi answer for all six texts, and call the web endpoint only when eapi fails or sends no `lrc`. Check signed in, with `LIVE`, that eapi's `lrc` and `tlyric` match the web's for the live-test ids.
  - Commit: `ask NetEase for a song's lyrics once, through the desktop client's API, which sends all of them: the web's answer was asked for as well, for the same lines`.
- **Now Playing during a stall** (P3, verify first). Starve a stream (throttle the network in the app), watch Control Center's progress against the app's own, and publish rate 0 while starved if they part.
- **The notch's display** (P3). Prefer the screen whose `safeAreaInsets.top > 0` in `menuBarScreen`. Verify with an external display set as main.

## 5. Before merging any work package

- [ ] `go test ./internal/...` passes, new UI tests pass with `-count=5`, and `go tool mygo vet .` is clean.
- [ ] Each commit builds and passes by itself, and says one behaviour in the house style.
- [ ] No new control types, and no settings without a full-sentence comment.
- [ ] Nothing moves when a toggle changes (look, don't assume).
- [ ] The frame budgets hold. A new animated surface has a budget in `perf_test.go`.
- [ ] For visuals: `NM_FRAMES` frames looked at, in dark Now Playing, with a red, a green and a grey cover.
- [ ] macOS-only code has its `*_other.go` stub. Windows and Linux still build (`GOOS=windows go build ./...`, `GOOS=linux go build ./...`).
- [ ] No PNGs, `build/` or `.mygo/` in the commit.

## 6. Open decisions for the owner

1. **The repository's name** (WP0 step 1), and when to push `main`.
2. **注音 or 音译** for the menu item. Check NetEase's current client and match it.
3. **The notch's lyrics default.** Off (recommended: it covers other apps' windows) or one line (the previous draft).
4. **Whether 青鸟's serif type stays,** once the frames are seen on a machine without the families.
5. **Atoll's extension RPC** (`next.md` §6.5): not now. Revisit only if listeners ask to see our own activity inside Atoll.
