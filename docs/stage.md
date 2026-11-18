# The stage

The stage is Now Playing's 律动 pane: the song's lyrics set large over a backdrop made from its cover, in one of four styles. This file says how it works and what any change to it must keep. When you change a rule here in the code, change it here too.

## Styles

`stageStyles` (`internal/app/ui_stage_styles.go`) lists them; the setting `StageStyle` holds the id.

| Id | Label | Code | From |
|---|---|---|---|
| `""` | 流光 | `stageLine`, `stageWord` (`ui_stage.go`) | Folia's classic: words drift, swell and glow as they are sung |
| `partita` | 云阶 | `partitaLine` (`ui_stage_styles.go`) | Folia: a line in staggered rows, each springing in as it is sung |
| `tilt` | 倾诉 | `tiltLine` (`ui_stage_styles.go`) | Folia: a few large lines, one leaning, characters swelling as sung |
| `aobird` | 青鸟 | `birdStage` (`ui_stage_bird.go`) | This app's own: a line forms whole out of a mist and stays still |

Folia is [chthollyphile/folia-major](https://github.com/chthollyphile/folia-major). An unknown id draws as 流光.

## What every change must keep

1. **The stage is a function of where playback is.** Every opacity, blur and offset is worked out from `pl.Position()` and the lines: no state, no timers. Seeking lands on the frame playing would show, and pausing freezes it.
2. **A line is one layer over the whole stage, keyed by its time** (`c.Key(l.At)`), so the line going and the line coming can overlap while one fades. Stateful widgets get a key before they are built. Never store a `ui.Element`.
3. **Nothing moves because something else changed.** Turning the translation or reading on or off, or a line without them, must not move the words. Anchor at the edge that shouldn't move: the foot band is anchored at its bottom, so it grows upward. `TestStageWordsStayWhenTheCaptionChanges` checks this for each style it lists; add new styles to it.
4. **Frames only while something changes.** `c.AnimationFrame()` while something visibly moves; otherwise `c.After(d)` until the next change. Budgets live in `internal/app/perf_test.go`: between lines, at most 12 frames a second for every style (`TestNowPlayingFrameBudget`), and at most 1 for 青鸟 once a line has formed and in breaks (`TestBirdFrameBudget`). A new animated surface gets a budget.
5. **Reduce Motion** (`c.Preferences().ReduceMotion`) turns movement into fades.

## The flow

`stage` (`ui_stage.go`) takes the lines and the position:

- **No lines.** A skeleton while loading, else 「纯音乐，请欣赏」.
- **The line before** fades over the first 0.4 s of the next, unless a long break came between them (`breakBefore`): then it went with the break and stays gone.
- **A long break**, when the next line is more than 4 s after this one ends and playback is more than 1 s past its end: the interlude takes the stage. 青鸟 has its own test and draws a hairline instead.
- **Hot comments.** With 热评 on, a hot comment takes the middle in a quiet part, the lyrics dimmed under it (`ui_quiet.go`). 青鸟 never shows one.
- **Sparks.** 火花 throws sparks off each word as it lands, for 流光 and 倾诉. The menu disables it while 青鸟 is chosen and keeps the setting.
- **Frames.** `stageNextFrame` asks at once while a word springs in, a few times a second while words only drift, and otherwise when the next word is due. 倾诉 starts a part 250 ms early (`tiltLead`).

## The foot band

流光, 云阶 and 倾诉 set the reading and translation in one shared band along the foot, as Folia's shared subtitle does, so they look the same in every style and never move the words (`stageFoot`, `stageCaption`):

- `Absolute().Left(0).Right(0).Bottom(24)`, centred, passing the pointer through, inside the dimmed `sung` box so a hot comment dims it too.
- Rows from top to bottom: the reading, the translation, then the next line (14 pt, 32%, one line).
- The first caption row present is 18 pt, weight 600, 72% white; the second ×0.88 the size, weight 500, ×0.82 the opacity. Each at most 72% of the stage wide and two lines.
- One line's captions at a time: the old ones fade over the first 120 ms after the new line's time, and the new ones come in from 120 ms over 250 ms, rising 6 DIP. In a long break they fade over 400 ms with their line.

## 青鸟

A line forms whole out of a mist and stays still. Its reading sits under it, and its translation answers late, off to the right. In breaks a hairline holds the place. No hot comments, no sparks, no foot band and no next line: the empty space is the point.

**Type.** The line is `min(56, max(32, 0.04 w))`, weight 400, line height 1.3, letter spacing 0.10 em for CJK and 0.01 em for Latin, at most two lines, shrinking to fit (`birdFit`). Families (`birdFonts`), each system asked only for what it installs: for `ja` lines, Hiragino Mincho ProN on macOS, Yu Mincho on Windows, Noto Serif CJK JP elsewhere; for everything else, Songti SC, SimSun, Noto Serif CJK SC. Without them the system sans draws it. Don't add a family macOS only offers as a download (YuMincho and the like): asking CoreText for one can wait on the system's font catalog, and stalled a Mac in CI for ten minutes. The reading is `max(13, 0.32 × size)` in the system font; the translation `max(14, 0.38 × size)` in `birdFontZH`, tracking 0.14 em, starting 14% in from the line block's left, at most two lines.

**Colours.** `birdInk` `#EEF4F6` for the line; `birdBreath` white at 46% for the reading; `birdAnswer` `#F3EEDF` at 62% for the translation; `birdRest` `#B0CECC` mixed 15% toward the cover's accent, at 50%, for the hairline.

**Motion**, with `s` the first word's start (or the line's time) and `e` its end:

- **Condensing.** It takes `D = clamp(0.45 (e − s), 0.6 s, 1.8 s)` and starts `min(0.3 s, D/3)` before `s`. Opacity follows a smoothstep; blur goes from 9 (scaled by size / 48) to 0; the line rises 3 DIP. No scale, no per-word light.
- **The answer** comes in from 0.4 s after condensing starts, over 0.9 s, from 36 DIP to the right.
- **Leaving.** Over 0.45 s, starting 0.15 s before the next line starts condensing, blurring to 4. Before a break of more than 4 s, it leaves at `e + 0.8 s` over 0.7 s.
- **The hairline.** 1 DIP, growing to 32% of the stage's width over 0.8 s, 0.4 s after the line has left. It fades over the 0.4 s before the next line starts condensing.
- **Reduce Motion.** Opacity only, over 0.3 s.

**Backdrop.** While 青鸟 shows, the backdrop takes its "glass" tone (`glassColors`, `backdrop.go`). The cover's hue moves 70% of the way to 192°, at saturation 0.20 at the top and 0.24 at the foot, and value 0.40 and 0.25. The cover's wash drops to 0.06, and a light falls at 135° from 8% white to nothing by 55%. A grey cover stays grey. The fog takes the same tone.

Known defects, on the roadmap: the answer arrives before the line has formed and sits left of centre; the captions are below 4.5:1; the line breaks inside words; red covers turn mauve and the hue jumps at 12°.

## The backdrop

`backdrop.go` paints, once a song, a gradient from the cover's tint with a faint wash of the blurred cover and a shade toward the foot. A new song, a new tone or a new pane crossfades over `backdropFade`. Fog and smoke are opt-in layers over it.

## Adding a style

Today a style touches several places, because the registry holds only ids and labels:

1. An entry in `stageStyles`.
2. Either a case in `stageLine`, which gives it hot comments, the foot band, interludes, the skeleton and `stageNextFrame`, or an early branch in `stage()` like 青鸟's, which must redo the empty states, the intro, breaks and outro, its own frames and its own captions.
3. Opting out of sparks (`ui_stage_styles.go`, the 火花 item) and of hot comments: by comparing ids.
4. A backdrop tone: `glass` is a bool from `StageStyle == "aobird"` (`ui_nowplaying.go`), threaded through `backdrop`, its cache key, `drawBackdrop`, `drawFog` and `fogColors`.
5. Fonts by `l.Lang`, and `Lang` on the line's text.
6. Its id in `TestStageWordsStayWhenTheCaptionChanges`; a stricter budget in `perf_test.go` if it is a quiet style; `TestStageFrames` and `TestStageCaptionFrames` pick it up by themselves.

The roadmap replaces this with a registry of `stageStyle` values (sparks, hot comments, tone, lead, and the function that draws), read by the menu, `stage()` and Now Playing.

## Looking at it

Don't assert on how it looks; look at it. `NM_FRAMES=$TMPDIR/frames go test ./internal/app -run 'Frames'` saves `stage-*.png`, `caption-*.png` and `bird-*.png` (mist, half, formed, answer, leaving, hairline, red, green and grey covers, a fast line, a long line, an English line, no serif). Never commit them.
