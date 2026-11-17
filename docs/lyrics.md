# Lyrics

How the app gets a song's lyrics, how it turns them into lines with a translation and a reading, and where those show. This describes the code as built; when you change the rules, change this file in the same commit.

## What NetEase sends

One request a song, to the desktop client's API, `/api/song/lyric/v1` (`netease.Lyrics`, `internal/netease/netease.go`). It answers with six texts, any of which may be empty:

| Field | Text | Timed by |
|---|---|---|
| `LRC` (`lrc`) | the timed lines | itself |
| `Translation` (`tlyric`) | the translation, always Chinese | `LRC`'s lines |
| `Reading` (`romalrc`) | how the lines are read aloud | `LRC`'s lines |
| `YRC` (`yrc`) | the word-timed lines | itself, word by word |
| `YTranslation` (`ytlrc`) | the translation | `YRC`'s lines; it skips some merged lines |
| `YReading` (`yromalrc`) | the reading | `YRC`'s lines exactly |

The web API (`/api/song/lyric`) is asked only when that call fails or sends no timed lines; a reading the web leaves empty then keeps the desktop client's.

Measured signed out on 2026-11-05 (the evidence is in [`archive/2026-11-after-0.1.26/next.md`](archive/2026-11-after-0.1.26/next.md) §3.1):

- The reading is whatever NetEase writes for the song's language: romaji split into syllables for Japanese (`ki tto mo u ko re i jyo u`), NetEase's own spelling for Korean (`ggok ga ti ga zul rae`, not Revised Romanization), and toneless Jyutping for Cantonese. Mandarin, Russian and English get none.
- Word-timed lines are not timed lines: one may sing two of them, or two may split one. 残響散歌's line at 37.45 s sings the timed lines at 37.64 and 39.84 s.
- For Cantonese, `yromalrc` uses a pinyin-like spelling, different from `romalrc`'s Jyutping.

## Building the lines

`lyrics.Build` (`internal/lyrics/lyrics.go`):

1. **The lines.** The word-timed lines if there are any; else the timed lines. A timed line ends at the next one's start, or 6 s after its own, whichever is sooner, and its words share 85% of that by their length (`SpreadWords`), a Chinese, Japanese or Korean character being a word of its own.
2. **Attaching a translation or reading line** (`belongs`). Of the two lines starting nearest it, no later than 500 ms after it, it goes to the nearer one that is still sung then: from 500 ms before that line's start until 200 ms after its end. Two reaching one line are joined by a space, in order. One that reaches no line is dropped, and so is one with no letters or digits (`//`, `●●●`).
3. **Which text wins.** On word-timed lines, the texts timed by them first; a line they leave empty takes the text of the timed lines it sings. A line of Han characters only keeps the timed lines' reading where it has one (the Jyutping, for Cantonese). This applies to Han-only lines in any language, not only Cantonese; check it on more songs before widening or narrowing it.
4. **Dropped.** A translation with the same letters and digits as its line, whatever their case, spacing or punctuation. A reading of a line with no letters outside Latin, or one that only repeats the line.
5. **Language** (`setLang`). A song is Japanese when at least max(2, a fifth) of its sung lines (credits aside) have kana; then each of its lines with kana or Han is `ja`. A line with Hangul is `ko`. Everything else, Cantonese and Mandarin included, is `""`, the app's language. `Lang` exists so that Japanese Han characters draw in their Japanese forms (直, 骨, 角).
6. **The reading's name** (`ReadingName`), for the menu: 罗马音 when any line with a reading is `ja` or `ko`; else, over the Han-only lines with a reading, 粤拼 when at least one syllable in 20 looks like Jyutping (ends in p, t or k; has aa, eo or oe; starts with ng and a vowel), 拼音 otherwise; and nothing when no line has a reading.

## By language

| Song | 音译 shows | Named | `Lang` |
|---|---|---|---|
| Japanese | NetEase's romaji, as sent; not re-spaced | 罗马音 | `ja` |
| Korean | NetEase's spelling, as sent | 罗马音 | `ko` |
| Cantonese | Jyutping, toneless | 粤拼 | `""` |
| Mandarin | nothing; no pinyin is made locally | — | `""` |
| English and other Latin | nothing | — | `""` |
| Russian, Thai, others | nothing | — | `""` |

## Switching them on

Two settings, shared by every surface: `LyricTranslation` (on by default) and `LyricRoman` (off). In Now Playing, the 歌词显示 pill beside the tabs, on 歌词 and 律动, shows the state as 原文, 译, 音 or 译·音, and its menu has 翻译 and 音译（name）, the name from `ReadingName`. For a song with no reading the item stays enabled, so the choice holds for the next song. Every surface reads them through `a.lyricTranslation` and `a.lyricReading` (`ui_nowplaying.go`).

## Where they show

| Surface | Code | Reading | Translation |
|---|---|---|---|
| 歌词 list | `timedLines`, `ui_nowplaying.go` | 17 pt, 500, 55% | 17 pt, 600, 70% |
| Foot band of 流光, 云阶 and 倾诉 | `stageFoot`, `stageCaption`, `ui_stage.go` | the first row present is 18 pt, 600, 72%; the second ×0.88 size and ×0.82 opacity | same rule |
| 青鸟 | `birdCaptions`, `ui_stage_bird.go` | 0.32 × the line's size, 400, 46%, the system sans | 0.38 ×, 400, serif, `#F3EEDF` at 62%, 14% in from the line's left |
| Under the notch | `withExtra`, `ui_notch.go` | one extra row, in 2- and 3-line modes | only when the reading is off |
| 选歌 wall, podcast captions | `ui_wall.go`, `ui_captions.go` | none | none |

Reading sits above translation everywhere. The weight between them does not: that is on the roadmap. The agreed direction, not yet built:

1. A row's style depends on its role, never on what else is showing.
2. The reading binds tight to its line; the translation is its own group, with more space above it.
3. The translation is never smaller or fainter than the reading, and never under 18 pt on the stage.
4. The reading is Latin: the system sans, no `Lang`, about a third of the line's size.
5. `Lang` goes on the line's own text; the translation is `zh-Hans`.
6. Both reach 4.5:1 contrast against the lightest backdrop under them.
7. Two balanced rows at most, never broken inside a word; Korean breaks only at spaces.

## Not doing

- **Furigana.** NetEase doesn't send it, and it would need a Japanese dictionary and ruby layout, which MyGo doesn't have.
- **Lighting the reading word by word.** `yromalrc` is timed by line, not by word.
- **Other lyric sites**, or pinyin made locally for Mandarin.

## Tests

Logic is tested in `internal/lyrics/lyrics_test.go` without views: merged and split lines, late lines, echoes, `Lang`, `ReadingName`. `TestLyricTextsLive` in `internal/netease/live_test.go` asks NetEase itself when `LIVE` is set. UI behaviour (the pill, the lit line staying put as rows turn on and off) is in `internal/app/lyric_script_test.go`.
