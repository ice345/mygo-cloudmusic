# The stage

The stage is Now Playing's 律动 pane: the song's lyrics set large over a backdrop made from its cover, in one of five styles. This file says how it works and what any change to it must keep. When you change a rule here in the code, change it here too.

## Styles

`stageStyles` (`internal/app/ui_stage_styles.go`) lists them, with what each does (see [Adding a style](#adding-a-style)); the setting `StageStyle` holds the id.

| Id | Label | Code | From |
|---|---|---|---|
| `""` | 流光 | `classicLine`, `stageWord` (`ui_stage.go`) | Folia's classic: words drift, swell and glow as they are sung |
| `partita` | 云阶 | `partitaLine` (`ui_stage_styles.go`) | Folia: a line in staggered rows, each springing in as it is sung |
| `tilt` | 倾诉 | `tiltLine` (`ui_stage_styles.go`) | Folia: a few large lines, one leaning, characters swelling as sung |
| `aobird` | 青鸟 | `birdStage` (`ui_stage_bird.go`) | This app's own: a line forms whole out of a mist and stays still |
| `chorus` | 和鸣 | `chorusLine` (`ui_stage_chorus.go`) | This app's own, 青鸟's sibling: a line in full light, lit from behind, pulled into focus, one gold word, its translation in unison |

Folia is [chthollyphile/folia-major](https://github.com/chthollyphile/folia-major). An unknown id, as a later version may save, draws as 流光 (`styleOf`).

## What every change must keep

1. **The stage is a function of where playback is.** Every opacity, blur and offset is worked out from `pl.Position()` and the lines: no state, no timers. Seeking lands on the frame playing would show, and pausing freezes it.
2. **A line is one layer over the whole stage, keyed by its time and its text** (`lineKey`), so the line going and the line coming can overlap while one fades, and two lines a duet sings at one time are two layers. Stateful widgets get a key before they are built. Never store a `ui.Element`.
3. **Nothing moves because something else changed.** Turning the translation or reading on or off, or a line without them, must not move the words. Anchor at the edge that shouldn't move: the foot band is anchored at its bottom, so it grows upward. `TestStageWordsStayWhenTheCaptionChanges` checks this for every style.
4. **Frames only while something changes.** `c.AnimationFrame()` while something visibly moves; otherwise `c.After(d)` until the next change. Budgets live in `internal/app/perf_test.go`: between lines, at most 12 frames a second for every style (`TestNowPlayingFrameBudget`), and at most 1 for 青鸟 once a line has formed and in breaks (`TestBirdFrameBudget`). A new animated surface gets a budget.
5. **Reduce Motion** (`c.Preferences().ReduceMotion`) turns movement into fades.

## The flow

`stage` (`ui_stage.go`) takes the lines and the position:

- **No lines.** A skeleton while loading, else 「纯音乐，请欣赏」.
- **The line before** fades over the first 0.4 s of the next, unless a long break came between them (`breakBefore`): then it went with the break and stays gone.
- **A long break**, when the next line is more than 4 s after this one ends and playback is more than 1 s past its end: the interlude takes the stage. 青鸟 has its own test and draws a hairline instead.
- **Hot comments.** With 热评 on, a hot comment takes the middle in a quiet part, the lyrics dimmed under it (`ui_quiet.go`). 青鸟 never shows one.
- **Sparks.** 火花 throws sparks off each word as it lands, in the styles whose `sparks` is set: 流光, 云阶 and 倾诉. The menu disables it for the others and keeps the setting.
- **Frames.** `stageNextFrame` asks at once while a word springs in, a few times a second while words only drift, and otherwise when the next word is due. 倾诉 starts a part 250 ms early (`tiltLead`).

## The foot band

流光, 云阶 and 倾诉 set the reading and translation in one shared band along the foot, as Folia's shared subtitle does, so they look the same in every style and never move the words (`stageFoot`, `stageCaption`):

- `Absolute().Left(0).Right(0).Bottom(24)`, centred, passing the pointer through, inside the dimmed `sung` box so a hot comment dims it too.
- Rows from top to bottom: the reading, the translation, then the next line (14 pt, 32%, one line).
- The reading is 16 pt, weight 500, white at 70% (`footReading`); the translation 18 pt, weight 600, white at 72% (`footTranslation`), `zh-Hans`, the same with the reading on or off. Each at most 72% of the stage wide, in at most two even rows broken between phrases (`breakRows`, below).
- One line's captions at a time: the old ones fade over the first 120 ms after the new line's time, and the new ones come in from 120 ms over 250 ms, rising 6 DIP. In a long break they fade over 400 ms with their line.

## 青鸟

A line forms whole out of a mist and stays still. Its reading sits close under it, and its translation answers late, from the right. In breaks a hairline holds the place. No hot comments, no sparks, no foot band and no next line: the empty space is the point.

**Type.** The line is `min(56, max(32, 0.04 w))`, weight 400, line height 1.3, letter spacing 0.10 em for CJK and 0.01 em for Latin, in at most two even rows broken between phrases, shrinking to fit (`birdFit`). Families (`birdFonts`), each system asked only for what it installs:

| Line | macOS | Windows | Elsewhere |
|---|---|---|---|
| `ja` | Hiragino Mincho ProN | Yu Mincho | Noto Serif CJK JP |
| `ko` | AppleMyungjo | Batang | Noto Serif CJK KR |
| anything else | Songti SC | SimSun | Noto Serif CJK SC |

Without them the system sans draws it. Don't add a family macOS only offers as a download (YuMincho and the like): asking CoreText for one can wait on the system's font catalog, and stalled a Mac in CI for ten minutes.

**Captions.** The reading is `max(15, 0.34 × size)`, weight 400, in the system sans with no `Lang`, centred, `0.1 × size` under the line. The translation is `max(18, 0.38 × size)` in `birdFontZH`, `zh-Hans`, tracking 0.14 em, `0.35 × size` under the line or the reading. Both take at most two even rows; neither takes its width from the line, so a one-character line keeps its translation on one row. The translation's right edge is the line's; one wider than the line sets out from the line's middle instead, to the right.

**Colours.** `birdInk` `#EEF4F6` for the line; `birdBreath` `#EEF4F6` at 82% for the reading; `birdAnswer` `#F3EEDF` at 88% for the translation; `birdRest` `#B0CECC` mixed 15% toward the cover's accent, at 50%, for the hairline. Over the lightest glass, under a white cover, the reading reads at 4.87:1 and the translation at 5.1:1; `TestCaptionsReadOverTheBackdrop` holds both above 4.5:1.

**Motion**, with `s` the first word's start (or the line's time) and `e` its end:

- **Condensing.** It takes `D = clamp(0.45 (e − s), 0.6 s, 1.8 s)` and starts `min(0.3 s, D/3)` before `s`. Opacity follows a smoothstep; blur goes from 9 (scaled by size / 48) to 0; the line rises 3 DIP. No scale, no per-word light.
- **The answer** comes in from `s + 0.4 s`, whatever the line's length, over 0.9 s, from 36 DIP to the right (`birdAnswerTimes`). It goes 0.4 s after the line starts to, over as long, blurring to 4. Waiting for the line to form would halve its reading time on a 3 s line.
- **Leaving.** Over 0.45 s, starting 0.15 s before the next line starts condensing, blurring to 4. Before a break of more than 4 s, it leaves at `e + 0.8 s` over 0.7 s.
- **The hairline.** 1 DIP, growing to 32% of the stage's width over 0.8 s, 0.4 s after the line has left. It fades over the 0.4 s before the next line starts condensing.
- **Reduce Motion.** Opacity only, over 0.3 s.

**Backdrop.** While 青鸟 shows, the backdrop takes its tone, `toneGlass` (`backdrop.go`): one glass for every cover. Hue 186° (`glassHue`), saturation 0.20 at the top and 0.24 at the foot, value 0.36 and 0.25 (`glassColors`). The cover shows through only as its light and dark, its grey copy (`artwork.grey`) at 4% (`glassWash`), and a light falls at 135° from 4% white (`glassLight`) to nothing by 55%. The fog takes the same hue. Lighten any of these and the contrast test fails.

## 和鸣

青鸟's sibling, after what the makers of a series about a school band said of its look (the review's §5, in `archive/`): high contrast, light from behind, a shallow depth of field. Where 青鸟's parts answer one another late and off to one side, here they sound together. Nothing of the series itself: no characters, places, instruments, shots, titles or music, in the style, its frames or its release notes.

It shares the stage, so hot comments, interludes, the skeleton and the next line along the foot are the shared ones; it sets its own reading and translation (`captions`) and asks for its own frames (`chorusFrames`).

**Type.** The line is `min(60, max(34, 0.046 w))`, weight 700, the system sans, line height 1.25, in at most two even rows broken between phrases, shrinking to 60% to fit. The reading is `max(15, 0.34 × size)`, 500, white at 84% (`chorusReading`), `0.1 × size` under the line; the translation `max(18, 0.38 × size)`, 600, `#FFF3E2` at 90% (`chorusAnswer`), `zh-Hans`, `0.35 × size` under the line or reading. All three are centred on one axis.

**Motion**, with `s` the first word's start:

- **Rack focus.** A line comes into focus over 0.45 s from `s − 0.15 s`: blur from 12 (scaled by size / 48) to 0, opacity in, from 4% larger. The line before goes out of focus as the stage fades it, over the 0.4 s after the next line's time, to 4% smaller. The captions go with their line.
- **The gold.** On a line timed word by word (`Line.Timed`), the word being sung is brass gold `#E8BC5A`, turning over 0.12 s and back over 0.3 s once it ends or the next starts (`chorusGoldOf`). A line timed only as a whole has none: its words' times are guesses.
- **Frames.** At once only while a line comes into focus or leaves it, or a word turns; a held word asks for none (`TestChorusFrameBudget`).
- **Reduce Motion.** Opacity only, the gold at once.

**Backdrop.** `toneBacklit` (`backdrop_backlit.go`): the cover's hue, saturation 0.5 to 0.75, value 0.24 at the top and 0.12 at the foot, deeper than the usual backdrop; the cover's wash at 7%; a warm light (`#FFE9C8`, 30% toward the cover's hue) behind the line at 20% (`backlitGlow`), 0.84 of the window wide, from a falloff bitmap, since MyGo has no radial gradient; seven soft discs of it out of focus away from the line, placed by the cover's tint; the sides and foot darker. Over the lightest of it the reading reads at 4.92:1, the translation at 4.93:1 and the gold word, large, at 3.34:1 (`TestCaptionsReadOverTheBackdrop`).

## Breaking lines

MyGo breaks text wherever a row runs out of room, which in Japanese and Chinese is between any two characters. So the stage breaks its own: `breakRows` (`ui_stage.go`) asks `internal/linebreak` for the places a row may end, and draws the rows `NoWrap`, on lines of their own.

- **Where a row may end:** at spaces, the only place Korean and the languages spaced into words break; and between the phrases of Japanese and Chinese, as [BudouX](https://github.com/google/budoux) finds them (its v0.10.4 models, in `internal/linebreak/models`). A run with kana is read as Japanese, other Chinese characters as Chinese. A row never starts with a closing mark or a small kana, and never ends with an opening mark.
- **Which of those:** one row if the whole fits; else as few as fit, as even as can be, a break after punctuation or a space counting as an eighth of the whole narrower. A phrase wider than the row on its own breaks between its characters.

`TestRowsNeverBreakInsideAWord` holds this. The 歌词 list still wraps as MyGo does: its width is known only once it is laid out.

## The backdrop

`backdrop.go` paints, once a song, a gradient from the cover's tint with a faint wash of the blurred cover and a shade toward the foot. A new song, a new tone or a new pane crossfades over `backdropFade`. Fog and smoke are opt-in layers over it.

## Adding a style

A style is one entry in `stageStyles` (`ui_stage_styles.go`), a `stageStyle`. The menu, `stage()` and Now Playing read what they need from it and never compare ids:

- `line` draws one line on the shared stage, which gives the style hot comments, the foot band, interludes, the skeleton and `stageNextFrame`. About a day of work for a style that only draws lines.
- `own`, in place of `line`, draws the whole stage, as 青鸟 does. It then redoes the empty states, the intro, breaks and outro, its own frames and its own captions, and no hot comment takes it. Three days or more.
- `lead` is how long before a word it starts to show it; `sparks` says 火花 applies (the menu disables it otherwise and keeps the setting); `tone` is the backdrop's (`toneDeep` or `toneGlass`).

Then:

1. Fonts by `l.Lang`, and `Lang` on the line's own text only.
2. The tests that range over `stageStyles` fail until they know what the new style shows: `TestLineBeforeStaysGoneAfterABreak` and `TestStageLinesAtOneTime`. `TestStageWordsStayWhenTheCaptionChanges`, `TestNowPlayingFrameBudget`, `TestStageFrames` and `TestStageCaptionFrames` pick it up by themselves.
3. A stricter budget in `perf_test.go` if it is a quiet style.

## Looking at it

Don't assert on how it looks; look at it. `NM_FRAMES=$TMPDIR/frames go test ./internal/app -run 'Frames'` saves `stage-*.png`, `caption-*.png` and `bird-*.png`; with `LIVE` set too, `chorus-covers.png` sets 和鸣 over six real covers (mist, half, formed, answer, leaving, hairline, red, green and grey covers, a fast line, a long line, an English line, no serif). Never commit them.
