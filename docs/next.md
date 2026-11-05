# After 0.1.26: review and plan

Reviewed 2026-11-05, on `main` at `3982879` (0.1.26) plus the uncommitted work in the tree (listed in §2). This replaces the previous draft of this file. How to build each part, in which order, on which branch and in which commits, is in [`implementation.md`](implementation.md); this file says what to build and why.

What was checked, and how:

- Read every file this touches: `internal/lyrics`, `internal/netease/netease.go` and `eapi.go`, `internal/player/player.go`, and in `internal/app` the stage (`ui_stage.go`, `ui_stage_styles.go`), Now Playing, the backdrop, the notch (`ui_notch.go`, `notch_darwin.go`), the settings, and their tests.
- Ran `go test ./internal/...` (all pass), the two new UI tests with `-count=5` (stable), and `go tool mygo vet .` (clean).
- Asked NetEase's lyric endpoints directly, without signing in, for Japanese, Korean, Cantonese, Mandarin, Russian and English songs (§3.1). The romanization rules below come from those answers, not from memory.
- Read the source of Folia (the stage's origin), Atoll and boring.notch on GitHub. Read interviews and reviews of 『リズと青い鳥』. Every claim about the film below cites one of them (§9).

Priorities: **P0** wrong today, or blocks a release. **P1** has to land with the feature it belongs to. **P2** worth doing next. **P3** later, or only once verified.

## 1. At a glance

| # | Finding | Pri | Work package |
|---|---|---|---|
| 1 | Translations go missing when a word-timed (YRC) line merges two timed lines: the second half's translation reaches no line. This is on `main` today, and the uncommitted work drops readings the same way. | P0 | WP1 |
| 2 | NetEase already sends `ytlrc` and `yromalrc`, timed to the YRC lines, which settle #1 for most lines. The app doesn't decode them. | P1 | WP1 |
| 3 | NetEase's "reading" is three different things: syllable-split romaji (Japanese), NetEase's own Korean spelling, and toneless Jyutping (Cantonese). Mandarin, Russian and English get none. The work in the tree treats them as one, and its comments say Korean "often has none", which is wrong. | P1 | WP1, WP2 |
| 4 | Turning translation or reading on or off in 歌词 moves the lit line away from a third of the way down. | P1 | WP2 |
| 5 | The stage's caption, now inside each style's full-screen Column, moves the big words whenever it comes or goes. 流光 centres it in a 72% box pinned to the left edge, so it sits off-centre. It is cut to one line. This departs from Folia, which uses one shared caption at the foot for all three of these styles. | P1 | WP3 |
| 6 | Japanese lyrics are drawn in the app's language, not as Japanese, because no line sets `Lang("ja")`. On a Chinese system that means Chinese glyph shapes. | P2 | WP2 |
| 7 | The notch window has the same level (`NSMainMenuWindowLevel + 3`) and the same collection behaviour as Atoll's and boring.notch's. With both running, two islands open on the same spot under the same pointer. | P1 | WP4 |
| 8 | The 青鸟 draft makes claims about the film that its sources don't support. Its demo is near-black and fixed-colour, where the app's stage sits on a cover-derived backdrop. It has no rules for short, long, Latin or untranslated lines, and no frame budget. | P1 | WP6 |
| 9 | Lyrics under the notch (previous draft) cover the top of whatever window is under them. The draft never said what the pointer does there, how long lines fit, or what the default should be. | P1 | WP5 |
| 10 | `mygo.json` points updates at `ice345/mygo-cloudmusic`, which doesn't exist. `origin` is `ice345/cloudmusic`, a private repo that is still empty, with `master` as its default branch. `RELEASING.md` still uses upstream's URLs. | P0 for a release | WP0 |
| 11 | Each song asks for lyrics twice (web, then eapi), but the eapi call alone returns all six texts. | P2 | WP7 |
| 12 | `TestStageCaptionFollowsTheLine` mostly checks that texts appear, which `AGENTS.md` asks us not to write. It would pass even while the words jump. | P1 | WP3 |
| 13 | `docs/aobird-frames/` and `docs/stage-caption/` hold 2.7 MB of PNGs that `NM_FRAMES` can regenerate. Don't commit them. | P1 | — |

## 2. The uncommitted work

The tree adds a reading next to the translation, from NetEase's `romalrc`, plus a 歌词显示 button and a reading/translation caption in every stage style. The parts:

| Part | Keep? | Notes |
|---|---|---|
| `netease.Lyrics` asks for `romalrc` (`rv: -1`) | Keep, reshape | The precedence it uses (`res.Romalrc`, else `words.Romalrc`, else `res.Romalrc`) is the same thing three ways. It skips `ytlrc` and `yromalrc`. `rv: 0` returns `romalrc` too (measured), so the `-1` changes nothing. Return a struct (WP1): seven strings is too many to pass in a row. |
| `lyrics.Line.Reading`, `Build(..., reading, ...)`, `align`, `sameLine` | Keep the field. Replace the alignment. | `align` gives each extra line to the sung line whose **start** is nearest, within 1.5 s, and the first one wins (#1). `sameLine` compares only case and surrounding spaces, so an echo that differs in punctuation stays. |
| `settings.LyricTranslation` (default on), `LyricRoman` (default off) | Keep | Safe for people upgrading: `loadSettings` decodes onto the defaults, so a missing key stays on. |
| 歌词显示 pill beside the tabs, on 歌词 and 律动 | Keep, with fixes | It follows the existing pill, and the place is right. Two problems. The lit line moves when it toggles (#4). And **注音** reads as bopomofo (注音符号) to many Chinese readers; NetEase's own clients call this **音译**. Check against the current client before deciding. |
| `stageCaption` inside `stageLine`, `partitaLine` and `tiltLine` | Revert | See §4. |
| `tiltLine` becomes a method | Revert | It was only done to reach the caption. |
| `TestLyricScript` | Keep, update labels | Input leads to state, as `AGENTS.md` asks. |
| `TestStageCaptionFollowsTheLine` | Rewrite | Make it check that the words don't move (§4.4). |
| `TestStageCaptionFrames` | Keep | It saves frames for looking at, and skips without `NM_FRAMES`. |
| `docs/*.png` | Don't commit | Regenerate them with `NM_FRAMES`. |

## 3. Translation and romanization

### 3.1 What NetEase sends (measured 2026-11-05, signed out)

Two endpoints were asked: the web one, `/api/song/lyric?lv=-1&tv=-1&rv=-1&kv=-1`, and the desktop client's eapi one, `/api/song/lyric/v1`, which the app already calls for YRC.

| Song (id) | Language | `tlyric` | `romalrc` (reading) | YRC and `y*` |
|---|---|---|---|---|
| Lemon (536622304), 夜に駆ける (1409311773), 春日影 (2097486090) | Japanese | Chinese | Romaji split into syllables, NetEase spelling: `ki tto mo u ko re i jyo u`, `su ku i wo mo to me` | none signed out |
| 残響散歌 (3427209483) | Japanese | Chinese | same | `yrc` 32 lines against `lrc` 37. `ytlrc` 20 lines. `yromalrc` 32 lines, timed exactly to the YRC lines. |
| Love wins all (2120465975), 봄날 live (526464627) | Korean | Chinese | NetEase's own spelling, not Revised Romanization: `ggok ga ti ga zul rae`, `geu go xi eo di deun`. English inside a line is kept: `zeo gi meol ri from Earth to Mars`. | — |
| 富士山下 (65766) | Cantonese | none | Jyutping without tones: `cin can ngaang faa zoeng sek tau…` | `yrc`, plus a `yromalrc` in a different, pinyin-like spelling: `cin can ang fa zoeng sei tao…` |
| Mandarin songs (186016, 185709, 28854182, 1901371647) | Mandarin | none, or empty lines | **none** | — |
| Хочешь? (3870797) | Russian | Chinese | **none** | — |

What follows from this:

1. A reading is not "romaji". It is whatever NetEase wrote for the song's language. Name it by language (§3.2).
2. YRC lines are not LRC lines. 残響散歌's YRC line at 37.45 s, 「光も痛みも怒りも全部抱きしめて」, is two LRC lines (37.64 s and 39.84 s). Today the second translation, 「全都 拥入怀中」, is 2.26 s from any YRC start, so it is dropped. The same happens at 51.49 s. And the YRC splits 71.49 s 「歌と散れ 残響」 in two, so 「残響」 gets nothing. Checked by running the current `lyrics.Build` on the real response.
3. `yromalrc` lines up with YRC exactly. `ytlrc` does too where it exists, but it skips merged lines. So: prefer the `y*` text, and fill the rest from the LRC-timed text, joining when several fall into one line.
4. For Cantonese, the eapi `yromalrc` uses a different spelling from the web `romalrc`. Prefer the standard Jyutping (`romalrc`) on Han-only lines that already have it, and fall back to `yromalrc` only where `romalrc` leaves a line empty. Check this on more Cantonese songs (P3): two samples are not enough to say it always holds.
5. These answers were all taken signed out. Signed in, the eapi call carries `MUSIC_U` and may return `yrc` for more songs (not measured). The rules above hold either way.

### 3.2 One policy per language

The language comes from the song's lines (WP1 `lyrics.Lang`), not from metadata.

| Song | What 音译 shows | Name in the menu | `Lang` of the line | Notes |
|---|---|---|---|---|
| Japanese (kana in at least 2 lines, or a fifth of the sung lines) | NetEase's romaji, as sent | 罗马音 | `ja` for every Han or kana line | Don't re-space or "fix" the syllables: there's no dictionary to do it with, and a wrong word split is worse than syllables. Lines that are kana only get a reading too; that's fine. |
| Korean (Hangul) | NetEase's spelling, as sent | 罗马音 | `ko` | It's NetEase's own scheme (`x`, `zeo`), aimed at Chinese speakers. Don't relabel it 韩语罗马字 (RR). |
| Cantonese (Han only, and the reading looks like Jyutping) | Jyutping, toneless | 粤拼 | none (app locale) | How to tell (WP1): Jyutping syllables end in p/t/k, contain `aa`, `eo`, `oe`, or start with `ng`; pinyin never does. No translation, usually. This is the one most useful to Mandarin listeners. |
| Mandarin (Han only, no reading) | nothing | — | none | Don't generate pinyin locally: the listeners read Chinese. If NetEase ever sends pinyin, call it 拼音. |
| English and other Latin | nothing | — | none | Drop a reading that only repeats the line, ignoring case, spaces and punctuation. Drop any reading of a line with no letters outside Latin. |
| Russian, Greek | nothing | — | — | A local transliteration would be deterministic and small, but nobody has asked for it. P3, only on request. |
| Thai, Vietnamese, others | nothing | — | — | Thai romanization needs syllable splitting; leave it. |

The translation (`tlyric`/`ytlrc`) is always Chinese. It needs no language policy beyond being dropped when it repeats the line.

### 3.3 Data rules (WP1)

- **Attaching an extra line to a sung line.** Of the sung lines that start no later than 500 ms after the extra's time, take the nearest-starting one, as long as it is still being sung then (its `End` plus 500 ms). A second extra reaching the same line is joined to the first with a space, in order. An extra that reaches no line is dropped. This covers lines that are equal (LRC only), merged (YRC) and slightly early or late (in the samples, YRC starts from 480 ms before its LRC line to 270 ms after it).
- **Order of sources.** On YRC lines, use `ytlrc` and `yromalrc` first, then fill the lines still empty from `tlyric` and `romalrc`. On LRC lines, use `tlyric` and `romalrc`. For Cantonese, see §3.1 point 4.
- **Placeholders.** Drop extra lines with no letters or digits (`//`, `●●●`, `-`), as Folia's shared caption does.
- **Song-level facts** come from pure functions over `[]Line`, tested without a view: `Lang` per line, and `ReadingName(lines)` (罗马音, 粤拼, 拼音 or nothing).

### 3.4 Where it shows, and how

The two switches, 翻译 and 音译, are shared by every surface: the lyrics list, the stage and the notch's 2- and 3-line modes. They're chosen in Now Playing with the pill (the previous draft's reasoning holds: listeners pick this while listening, as they pick a stage style).

- **Pill and menu.** The label shows the state, as now: 原文, 译, 音, 译·音. The menu items are 翻译 and 音译, and the second carries the song's name for its reading, as in 音译（罗马音） or 音译（粤拼）. For a song with none, the item stays enabled so the choice holds for the next song, and its name is just 音译. The pill uses the 14% selected background while either is on.
- **Order and weight.** Reading above translation, as Folia stacks them: the reading is what you sing along to, the translation its gloss. On the stage band (§4) the first row is the louder one and the second a step quieter: ×0.88 size, ×0.82 opacity, as Folia's `SECONDARY_TRACK_*`. In the lyrics list both rows stay 17 pt; the reading at weight 500 and 55% white, the translation at 600 and 70% (as the tree has it, except the weight).
- **Fonts and language.** Lines carry `Lang` (§3.2), so Japanese draws with Japanese glyphs (直, 骨, 角 differ). Readings are Latin. Don't use `Lang` for those; let the system font draw them.
- **Lyrics list.** Turning a row on or off keeps the lit line a third of the way down (#4). The re-aim must use the layout after the change: `LayoutBounds` returns the previous frame's (implementation guide, WP2).
- **Stage.** §4.
- **Notch.** The 1-line mode never shows them. In the 2- and 3-line modes they take the next line's place (§6.4).

### 3.5 Not doing

- **Furigana over kanji.** NetEase doesn't send it. It would need a Japanese dictionary (tens of MB) and ruby layout, which MyGo doesn't have.
- **Lighting the reading word by word.** `yromalrc` is timed by line, not by word. Don't fake word timing for it: it stays still, like the translation.
- **Other lyric sites** (LRCLIB and the like). Everything here comes from the call the app already makes.

## 4. Where the stage puts translation and romanization

### 4.1 The conventions this has to keep

From the code and its history:

1. **The stage is a function of where playback is.** "Everything follows from where playback is, so seeking shows the same frame playing would" (`ui_stage.go`). No state, no timers.
2. **A line is one full-stage layer, keyed by `l.At`,** so the outgoing and incoming lines can overlap while one fades.
3. **Nothing on Now Playing moves because something else changed.** The history is full of it: "keep Now Playing's tabs in one place for every pane… so they jumped going from 歌词 to 律动", "keep Now Playing still as the quality's badge goes", "keep their height from the last time, rather than showing lower for a frame and rising". The previous draft of this file asked the same of the stage: 「大字的位置不要因为翻译消失而跳」.
4. **The styles are Folia's.** In Folia, Classic, Partita and Tilt (流光, 云阶, 倾诉) all leave translation and romanization to one shared `VisualizerSubtitleOverlay` at the foot. Its README: 「两组字幕字号…只有一处来源…模式间切换时它们不能跳字号」 (the caption sizes come from one place, so changing style never changes them). Only styles designed around a second lyric track (Monet, Pendolo and others) set translation inside their own animation.
5. Before the tree's change, this app followed the same contract: "The translation, and the line coming next, along the foot."

### 4.2 What the tree does

Each style's line layer is now a Column holding the words and then the caption:

- **The words move.** The Column centres words and caption together. A caption appearing lifts the words by half its height; a line without one (an English line in a Japanese song, say) sits lower; toggling 翻译 moves the line. That breaks conventions 3 and 4.
- **流光's caption is off-centre.** The Column stretches its children; the caption has `MaxWidthPercent(72)` and no alignment of its own, so its box starts at the left padding. A centred line therefore gets its caption centred on x ≈ 0.1w + (0.72 × 0.8w)/2 ≈ 0.39w, where the stage's centre is 0.5w (465 px of 1200 in the frame). You can see it in `docs/stage-caption/caption.png`.
- **The space below the words is unreliable.** 流光 words grow to `scale × 1.4`, up to 1.82× their layout box, and move ±18 DIP. A caption placed `f×0.55` below the layout box can touch a sung word. 云阶's last guide is drawn 8–16 DIP under its row, and the caption sits `f×0.15` below that.
- **Long translations are cut.** `SingleLine()` with 72% width. Folia wraps them.
- **No fade of its own.** It appears at full strength with the line layer, before the words arrive, and the old line's caption fades in a different place for 0.4 s.
- **云阶 has less room.** Its row count is worked out from the stage height, and the caption is added on top of that. On a short window it runs into the next-line text at `Bottom(24)`.

### 4.3 Verdict

Putting the caption inside the line layer would be acceptable only if the result looked enough better to justify it. It doesn't. In 倾诉 it reads like a subtitle tucked closer, but no better than the foot. In 流光 it is off-centre, and in 云阶 it is cramped. The one real gain is that the caption crossfades with its own line. A keyed caption at the foot gets that too.

**Decision:** go back to the shared foot band for 流光, 云阶 and 倾诉, now carrying reading and translation. Make 青鸟 the one deliberate exception (§5.4), as Folia allows for styles designed around a second track.

### 4.4 The design (WP3)

- One place, `stageCaption`, for every Folia style: `Absolute().Left(0).Right(0).Bottom(24)`, centred, `PassThrough`, inside the `sung` box so the hot comment's `quietUnder` dims it with the rest.
- Rows from top to bottom: the reading, the translation, then the next line (14 pt, 32%, one line, as now). The first caption row is 18 pt, weight 600, 72% (today's translation exactly, so a listener who never turns on 音译 sees no change). The second row is ×0.88 size and ×0.82 opacity. Up to 72% of the stage wide, and at most two lines each (`MaxLines(2)`), not one.
- One line's caption at a time, keyed by its `At`, all worked out from `pos`. The old caption fades out over the first 120 ms after the new line's `At`. The new one fades in from 120 ms over 250 ms, rising 6 DIP. Nothing stacks, and seeking lands on the same frame playing would.
- In a long break (the stage's own test: `pos > End + 1s` and the next line more than 4 s away), the caption fades out over 400 ms with its line. The band shows only what the stage shows.
- The words return to their own centring, as before the tree's change: 流光's absolute `Row`, the original `tiltLine` function.
- Regression test: in each style, with a line sung, `tt.Find` the first word's box; turn 翻译 off and on; the box doesn't move. This replaces `TestStageCaptionFollowsTheLine`.

## 5. 青鸟

### 5.1 Is the current draft, with its demo, enough?

No. It gets the direction right: give space rather than make an entrance; condense the whole line, not each word; no sparks; a hairline in breaks; the translation as an answering voice; hot comments off in this style; copy nothing from the film. But:

1. **Some claims about the film have no source.** 「色彩设计要的是隔着一块厚玻璃看，玻璃偏一点绿」 (the colour design as seen through thick, greenish glass), 「画出少女们的叹息」 (drawing the girls' sighs) and 「剧集像在上台阶…电影像沿着台阶往心里走」 (the series climbs stairs, the film walks down them inward) appear in none of the interviews and reviews found (§9). "Telephoto only" is also wrong: the film uses wide-angle close-ups of eyes at key moments too. Either cite these or call them our own reading.
2. **The palette goes the wrong way.** The demo is near-black (`#0b0b0d`) with a teal vignette. Viewers describe the film as pale, soft and washed with light blue. But Now Playing's chrome is white on a deep backdrop and must stay readable. So the plan has to lift and cool the backdrop without making it pale (§5.4), not paint black.
3. **The demo isn't the app.** It uses a fixed wash where the app uses a cover-derived backdrop painted once a song. It uses CSS blur and Mincho, which other platforms may lack. Its 「流光」 for comparison is a re-implementation, not the real one, so it compares against something the app doesn't draw.
4. **The hard cases are missing.** Lines shorter than its 1.8 s condensation (common in fast songs), lines too long for one row, Latin lyrics, songs without word timing, intro and outro, seeking, Reduce Motion, reading and translation turned on or off.
5. **No frame budget.** This style should cost less than the others, and say by how much (§5.4).

### 5.2 What the film's makers and critics said

These are the only points this style relies on. Everything else in §5.4 is our design choice, and is named as such.

| Source | What it says |
|---|---|
| Yamada, interview (animeanime 37619_2) | 「身を潜めてのぞき見るような感覚で、女の子の秘密のお話を撮り逃さないように」: filming as if hiding and peeking, so as not to miss the girls' secrets. Also three gazes: Mizore's, Nozomi's, and those around them. |
| Yamada, same | She says she manages to express things only through 色や音、空気、時間: colour, sound, air, time. |
| Fujitsu (animeanime 37731) | The camera "peeks" through a telephoto lens, a technique from her earlier films. The last telephoto shot compresses the two so that 「2人の距離」 (the distance between them) doesn't show. |
| Fujitsu, same | The distance between them is first shown by splitting the screen, and by pulling focus to a lab stand in front of them, whose rod divides them. |
| Fujitsu, same | Wide-angle close-ups of the eyes at key moments carry inner "ripples" as if they were "big waves". |
| Fujitsu, same | Blue flowers feature in the picture-book world, yellow flowers in the real one. |
| Synopsis (animeanime 37619) | The free piece has an oboe and flute solo that call and answer, and the two players' parts somehow don't mesh: 「上手くかみ合わない歯車」, gears that don't mesh. |
| Matsue (Real Sound) | The film keeps its distance and quietly watches; its direction is live-action in many places. |
| Viewer impressions (not production statements) | 「水色がかった映像」, light-blue-tinted images, and a "soft visual palette" in English-language reviews. Both come from search-result summaries, not read in full. Treat these as a direction, not a spec. |

### 5.3 From reference to mechanics

| Reference | On the stage |
|---|---|
| Peeking from hiding, not intruding | The text never comes at the viewer: no scale, no per-word pop, no glow, no movement beyond 3 DIP. It's smaller than 流光 and stays where it is. |
| Distance shown by division, not by closeness | The line and its translation never share an axis. The translation sits offset to the right and arrives late. Breaks show one hairline, like the rod that divides them. |
| Two parts that call and answer, not quite in time | The translation is the second voice: it enters about 400 ms after the line has formed, lighter, from the right. The reading is the same voice's breath: tight under the line, faint, on time. |
| Blue world, yellow world | The line's white has the faintest blue cast; the answer's white the faintest warm one. Two voices, two worlds, at a contrast you barely notice. |
| Colour, air, time | The means are a backdrop tone, empty space, opacity and delays. No shapes, no particles, no new pictures. |
| (Not used) Close-ups as "big waves" | Deliberately left out. A climax rule would work against the rest. |

Don't copy characters, uniforms, the title graphic, flowers, feathers, birds or any frame. Nothing pictorial goes on the stage.

### 5.4 Spec (WP6)

**Name and place.** Id `aobird`, label 青鸟, the fourth entry of 律动效果. It's the app's own, not Folia's: fix the comment above `stageStyles`, which says they're all Folia's.

**Backdrop tone** (design choice): the existing backdrop with a different tone, so it still paints once a song and crossfades over `backdropFade` as the style or the tab changes.

- Hue: the cover's hue moved 70% of the way to 192° (aqua).
- Saturation 0.20 at the top and 0.24 at the foot, not `backdropColors`' 0.45–0.66.
- Value 0.40 at the top and 0.25 at the foot, against 0.35 and 0.24 today: lifted, airy, still dark enough for white (about 6:1).
- The cover's blurred wash at 0.06, not 0.1: the cover is seen from farther away.
- A "window light": one more linear gradient, at 135°, from white at 8% to nothing by 55%. Painted once, into the same canvas.
- With Fog on, the fog takes the same tone.

**Colours** (tokens; check them on NM_FRAMES frames with a red cover, a green one and a grey one):

| Token | Value | Use |
|---|---|---|
| `birdInk` | `#EEF4F6` | the line |
| `birdBreath` | white, 46% | the reading |
| `birdAnswer` | `#F3EEDF`, 62% | the translation |
| `birdRest` | `#B0CECC`, 50%, mixed 15% toward the accent | the hairline |

No glow and no shadow; the cover's accent goes nowhere else.

**Type:**

- **The line.** Size `min(56, max(32, w×0.04))`, below 流光's 36–64. Weight 400. Letter-spacing 0.10 em for CJK, 0.01 em for Latin. Line height 1.3. At most two lines; past that, shrink to fit, down to half size, as `tiltLine` does.
- **Family, by the line's `Lang`.** Japanese: `"Hiragino Mincho ProN, Yu Mincho, YuMincho, Noto Serif CJK JP"`. Otherwise: `"Songti SC, STSong, SimSun, Noto Serif CJK SC"`. A machine with none falls back to the system sans, and the style must still look right in it: check a frame without them.
- **The reading.** `max(13, 0.32×size)`, weight 400, the system font (not serif), 0.35×size under the line.
- **The translation.** `max(14, 0.38×size)`, weight 400 in the serif, letter-spacing 0.14 em. Its left edge sits 14% of the line block's width in from the block's left. It begins 0.6×size under the line, or under the reading when that shows.

**Motion.** Everything is a function of `pos`. `s` is the first word's start (or `At`), `e` is `End`.

- **Condensing.** `D = clamp(0.45 × (e − s), 0.6 s, 1.8 s)`, starting `L = min(0.3 s, D/3)` before `s`. With `k = smoothstep((pos − (s − L)) / D)`: opacity `k`, blur `9×(1 − k)` (scaled by size/48), y `3×(1 − k)` DIP. No scale and no x. The whole line is one piece; no per-word light.
- **The reading** follows the line's `k`.
- **The translation.** `k₂ = easeOut((pos − (s − L) − 0.4 s) / 0.9 s)`: opacity `k₂`, x `36×(1 − k₂)` DIP, blur `1.5×(1 − k₂)`.
- **Leaving.** The old line fades over 0.45 s, starting 0.15 s before the next line starts condensing, blurring back to 4. There's no movement, and the two overlap only while the new one is still mist.
- **Before a long break** (same test as the stage), the line fades at `End + 0.8 s` over 0.7 s.
- **Breaks and the intro.** 0.4 s after the line has left, a 1 DIP hairline grows from nothing to 32% of the stage width over 0.8 s (ease-out), centred where the line was. It doesn't breathe, so it costs no frames while still. It fades over 0.4 s, ending as the next line starts condensing.
- **Outro.** The hairline, then nothing.
- **Reduce Motion.** Opacity only, over 0.3 s. No blur and no x.

**What it takes from the rest of the stage:**

- No 火花: the menu item is disabled while 青鸟 is chosen, and the setting is kept.
- No hot comments: the style never calls `quietShown`. Say so in its comment and in the release notes.
- No foot band and no next-line preview: the style owns its captions, and leaving the space empty is the point.

**Frames.** Ask for a frame only while something is changing: condensing, leaving, the answer arriving, the hairline growing or fading. Otherwise ask with `c.After` for the next of those moments. Budget for `perf_test.go`: at most 1 frame a second between lines once the line has formed, and in breaks. That is stricter than the 12 the other styles get.

### 5.5 Edge cases

- **Fast lines (under 1.3 s).** `D` drops to 0.6 s and the lead to 0.2 s, so the line is readable as it is sung. Lines closer together than 0.5 s cross-fade without a gap.
- **No word timing** (LRC only). `s = At`, so nothing changes.
- **Latin lyrics.** The serif's Latin, the small tracking, the shrink-to-fit rule.
- **Mixed songs.** `Lang` is per line, so a Japanese song's English line takes the zh family's Latin (both are serif).
- **Pure music, or no lyrics.** 「纯音乐，请欣赏」 as the stage draws it, in `birdAnswer`, at the answer's size, without movement.
- **Seeking.** The frame at `pos`. Mid-condensation looks as it would playing.
- **Paused.** Nothing animates, nothing is asked for.

### 5.6 What to review it with

The HTML demo stays a mood reference; don't extend it into a second implementation. Review the app's own frames:

- `TestStageFrames` already writes one PNG per entry in `stageStyles`, so 青鸟 gets one for free.
- Add a 青鸟 frame test, skipped without `NM_FRAMES`, that saves:
  - mist, half-formed, formed, answer arriving and leaving, each at fixed `pos`;
  - a hairline frame;
  - the three cover tints (red, green, grey);
  - translation on and off, reading on;
  - a 1.2 s line, a long wrapped line and an English line;
  - one frame with the serif families missing (`Font` pointed at a family that doesn't exist).
- If an HTML demo is still wanted for quick mood checks, it needs the three cover tints, the toggles, a fast line, a long line and an English line. Drop its fake 流光.

## 6. The notch

### 6.1 Today

Turned on, the shell sits on the hardware notch. With 实时活动 it grows wings: the cover on the left, four level bars on the right. Resting the pointer there for 300 ms opens the player card. The window is a non-activating panel at `NSMainMenuWindowLevel + 3`, on every Space and over full-screen apps, and lets clicks through wherever it draws nothing (`notch_darwin.go`). The app also publishes what plays to the system (`mygo.NowPlaying`, which is `MPNowPlayingInfoCenter` on macOS): title, artist, album, artwork, duration, position, and play, pause, next, previous and seek.

### 6.2 Atoll and the others: is it a problem?

Yes.

- **Same spot, same level.** Atoll's `DynamicIslandWindow` (from boring.notch) is an `NSPanel` at `.mainMenu + 3` with `[.fullScreenAuxiliary, .canJoinAllSpaces, .ignoresCycle, .stationary]`, exactly ours. Whichever window was ordered front last is drawn on top. Both draw a black shell at the notch.
- **Both open under the same pointer.** Ours decides from the cursor's position, whatever window is on top. So resting on the notch opens our card and Atoll's, one over the other, and clicks go to whichever is on top.
- **Our greeting grows over Atoll's island** the first time a song plays.
- **What we lose by stepping aside is small.** Atoll reads what plays from the system, and we publish it, so it shows our song, cover, progress and controls. It even fetches lyrics for it, from NetEase when LRCLIB has none: `NetEaseLyrics.swift` searches NetEase by title and artist, and accepts a result within 2 s of the duration we publish.

boring.notch (`theboringteam.boringnotch`), NotchDrop (`wiki.qaq.NotchDrop`) and Droppy (`iordv.Droppy`) draw at the notch the same way.

### 6.3 Making way (WP4)

- **Detect.** On macOS, at start and whenever an app launches or quits (`NSWorkspace` notifications), look for a running app whose bundle id starts with one of:
  - `com.Ebullioscopic.Atoll` (also matches `.dev`)
  - `theboringteam.boringnotch`
  - `wiki.qaq.NotchDrop`
  - `iordv.Droppy`

  Closed-source apps (NotchNook, Alcove and others) go on the list only after their ids have been checked on a machine that has them. Don't scan other apps' windows: menu-bar managers draw full-width windows there, and the list is easier to reason about and to test.
- **Make way.**
  - The shell shrinks to nothing.
  - The pointer no longer opens it.
  - The pointer watch stops, so it costs nothing.
  - The greeting doesn't run, and isn't marked as done: it waits for a launch without the other app.
  - The 显示刘海播放器 setting is left as the listener set it.
  - When the other app quits, ours comes back.
- **Say so.** While making way, the hint under 显示刘海播放器 says 「Atoll 正在使用刘海，刘海播放器暂不显示；正在播放的歌曲会显示在 Atoll 中。退出 Atoll 后自动恢复。」, with the app's own name. Don't add a switch to show both: they can't share the spot.
- **Keep what Atoll shows correct.** Publish the position again on every seek, pause and play (done today). Check what happens during a stall mid-stream (§7): if the system's clock runs on while the audio waits, Atoll's progress and lyrics run ahead.

### 6.4 Lyrics under the notch (WP5; refines the previous draft)

What the previous draft got right, and stays: Atoll's idea of a line pinned below the closed shell, not Droppy's open-to-read page. Lyrics grow down from the shell, never sideways. Word fill like the lyrics list (0.38 to 1), no springs, no blur. The strip goes into the shell while the card is open. 翻译 and 音译 are read from the shared switches.

Changed or added:

- **Off by default:** 关 / 1 行 / 2 行 / 3 行, with the app's `segmented` control in 刘海播放器, unavailable while the notch is off. The strip covers the top of whatever window is under it, such as a browser's tabs or a toolbar's middle. The app's own rule for things like that is 「the listener turns it on」 (`Notch`, `Smoke`, `Fog`, `HotComments`).
- **The pointer passes through it.** Only the shell itself (housing and wings) counts for opening. Resting on the strip opens nothing, and clicks reach the window under it. While the pointer is over it, the strip fades to 20% so what's under it shows.
- **Width.** The strip is as wide as the shell's head. Widening the shape would widen it inside the menu bar too, covering icons. A long line shrinks from 13.5 to 11 pt to fit, then ends in an ellipsis. A wider foot with concave shoulders under a narrow head is a later design, to review on frames.
- **Rows.**
  - 1 行: the line being sung.
  - 2 行: that line, plus the reading or translation if one is on, else the next line.
  - 3 行: the previous line, the current one and the next one (the current one in the middle), with reading or translation taking the next line's place when on.
  - The 1-row mode never shows a reading or translation.
- **Breaks.** Gaps under 5 s change nothing. In longer ones, from 1 s after the last word, the strip shows the coming line, unfilled, at 38%: no 「纯音乐」 placeholder (which means "instrumental") and no dots. For the outro and songs with no lyrics, the strip goes into the shell.
- **Making way** (§6.3) removes the strip too.
- **Frames.** 25 a second while a word fills (as the lyrics list does), otherwise none until the next word or line. Add a budget to `perf_test.go`.
- **Podcasts.** Where an episode has captions (`ui_captions.go`), the strip could show them the same way. Optional, after the songs.

### 6.5 Later, not now

- **Atoll's extension RPC.** A WebSocket on `localhost:9020` with `atoll.presentLiveActivity` and similar, which needs approval inside Atoll. It would let us push our own live activity into Atoll's island. It's young, Atoll-only and permissioned. Revisit only if listeners ask.
- **Which display.** `menuBarScreen()` takes `NSScreen.screens[0]`. With an external display set as main, the shell goes there, not to the MacBook's notch. Prefer the display whose `safeAreaInsets.top > 0`. P3; verify with two displays.
- **Full-screen spaces.** The strip hangs over full-screen video. There's no public API to know another app is full screen; leave it unless people complain.

## 7. Everything else worth fixing

| Pri | What | Detail |
|---|---|---|
| P0 | Update feed points nowhere | `mygo.json` has `updates.github: ice345/mygo-cloudmusic`, which is a 404. `origin` is `ice345/cloudmusic`. Rename one or the other before the next release, or the app's updater finds nothing. |
| P0 | Remote is empty, default branch `master` | `ice345/cloudmusic` has no commits yet and its default branch setting is `master`, while everything here is `main`. Push `main` first, then set it as the default branch on GitHub. |
| P1 | `RELEASING.md` is upstream's | Its feed check uses `github.com/egoist/mygo-cloudmusic/...` and its website URL is `mygo-cloudmusic.egoist.workers.dev`. Update them for this repository, or say they're upstream's. |
| P1 | macOS builds are now unsigned | The signing and notarization steps were removed (`c1fb036`), so macOS quarantines downloaded builds. Say how to open them in the release notes. The podcast libraries come from upstream's `sherpa-onnx-2.0.1` release, signed with upstream's Developer ID. They load in ad-hoc-signed builds, because MyGo adds the hardened runtime only for a real signing identity. If this repository ever signs with its own Developer ID, library validation will refuse them: re-sign and host them as `RELEASING.md` describes. |
| P2 | Two lyric requests a song | The eapi `/api/song/lyric/v1` returns `lrc`, `tlyric`, `romalrc`, `yrc`, `ytlrc` and `yromalrc` in one answer. Keep the web call only as a fallback when eapi fails or sends no `lrc`. Check signed in with a `LIVE` test first. |
| P2 | Japanese drawn as Chinese | §3.4: `Lang("ja")`/`("ko")` on lyric lines (list, stage, notch). |
| P3 | Now Playing during a stall | Readers wait for bytes (`stream.go`). If the output keeps its clock running while starved, the system extrapolates past it. Check, and if so, publish rate 0 while starved and the position again after. |
| P3 | Placeholders in extras | Drop `//` and `●●●` lines (§3.3). |
| P3 | `stageCaption`'s test style | When rewriting stage tests, check behaviour (positions, toggles), not that texts exist, per `AGENTS.md`. |

## 8. Out of scope: other audio sources

Kept from the previous draft, unchanged in substance. Source collections (洛雪), 星海, UnblockNeteaseMusic and its Go rewrite all fetch a song from other platforms when NetEase won't serve it, sometimes decrypting it. This client doesn't do that, and this document doesn't say how. VIP and paid songs stay greyed and skipped, and previews are not played as songs. Free accounts can be helped with what we already have: say why a row is grey, offer to play the preview, open the song on NetEase. A file the listener already owns may one day play from their own disk.

## 9. Sources

- NetEase lyric answers: measured 2026-11-05 against `music.163.com/api/song/lyric` and `interfacepc.music.163.com/eapi/song/lyric/v1`, for the ids in §3.1.
- Folia: [`VisualizerSubtitleOverlay.tsx`](https://github.com/chthollyphile/folia-major/blob/main/src/components/visualizer/VisualizerSubtitleOverlay.tsx), [`subtitleTrackStyle.ts`](https://github.com/chthollyphile/folia-major/blob/main/src/utils/lyrics/subtitleTrackStyle.ts), [`alternateText.ts`](https://github.com/chthollyphile/folia-major/blob/main/src/utils/lyrics/alternateText.ts), [visualizer README](https://github.com/chthollyphile/folia-major/blob/main/src/components/visualizer/README.md). Classic, Partita and Tilt each mount the shared overlay.
- Atoll: [`DynamicIslandWindow.swift`](https://github.com/Ebullioscopic/Atoll/blob/dev/DynamicIsland/components/Notch/DynamicIslandWindow.swift), [`NetEaseLyrics.swift`](https://github.com/Ebullioscopic/Atoll/blob/dev/DynamicIsland/managers/NetEaseLyrics.swift), [`ExtensionRPCServer.swift`](https://github.com/Ebullioscopic/Atoll/blob/dev/DynamicIsland/services/Extensions/ExtensionRPCServer.swift). Bundle ids are from the Xcode projects of Atoll, [boring.notch](https://github.com/TheBoredTeam/boring.notch), [NotchDrop](https://github.com/Lakr233/NotchDrop) and [Droppy](https://github.com/1of1Adam/Droppy).
- 『リズと青い鳥』:
  - Yamada and Takeda interview: [p.1](https://animeanime.jp/article/2018/05/02/37619.html), [p.2](https://animeanime.jp/article/2018/05/02/37619_2.html).
  - Fujitsu Ryota, [アニメの門V #34](https://animeanime.jp/article/2018/05/11/37731.html).
  - Matsue Tetsuaki, [Real Sound](https://realsound.jp/movie/2018/05/post-193770_2.html).
  - Viewer impressions: [jorudan reviews](https://movie.jorudan.co.jp/film/89407/review/) (seen in search results only).
