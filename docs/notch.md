# The notch player

A quick player around the camera housing at the top of a Mac's screen: black like the housing, it grows out of it as music plays and opens into a small player under a resting pointer. It is macOS only. On other systems `notch_other.go` stubs the native calls and the app never makes one (`app.go`).

Code: `internal/app/ui_notch.go` (state, pointer, view, lyrics), `notch_darwin.go` (the window's level, the pointer, the screen, other apps), `notch_other.go` (stubs), `notch_test.go`.

## Settings

All under 刘海播放器 in settings.

| Setting | Default | What it does |
|---|---|---|
| `Notch` 显示刘海播放器 | off | The notch player at all |
| `NotchActivity` 实时活动 | on | While music plays, wings beside the housing: the cover on the left, four level bars on the right |
| `NotchVirtual` | off | On a display without a notch, keep one showing; otherwise it shows only while the pointer presses the top edge, about its middle |
| `NotchLyrics` 刘海歌词 | 关 | 关, 1 行, 2 行 or 3 行 of lyrics hanging under the housing. Off by default: they cover the top of the window under them |
| `NotchGreeted` | — | The greeting has run |

## What it shows

- **The shell.** The housing's own size, at rest.
- **Wings**, with 实时活动 on and a song loaded.
- **The card**, after the pointer rests on the shell for 300 ms: 468 wide and 178 under the housing; 330 by 72 with nothing loaded. It closes once the pointer has been off it for 300 ms.
- **The greeting**, once: the first time a song plays with the notch off, it shows itself for 7 s with a button to keep it (开启). The pointer resting on it leaves it as it is, so the button can be clicked.
- **Lyrics under the housing**, with `NotchLyrics` set, as wide as the housing. 1 行: the line sung, lit word by word from 38% to full. 2 行: that line, then the reading or translation if one is on, else the next line. 3 行: the line before, the line sung, then the reading, translation or next line. In a break of 5 s or more, from 1 s after the last word, the next line shows unlit at 38%. They go into the shell while the card is open, with no line to show, in the intro and in the outro. A line too wide shrinks from 13.5 to 11 pt, then ends in an ellipsis.

## The pointer

The window is a non-activating panel at `NSMainMenuWindowLevel + 3`, on every Space and over full-screen apps. It lets clicks through wherever it draws nothing, so it can't rely on its own mouse events; `poll` follows the pointer instead.

- **What counts as the notch** is the shape it has now, with 6 pt of slack: the shell (and wings), the card while open, the greeting while it greets. While lyrics hang under the shell, only the shell counts, with no slack. `TestNotchCardKeepsThePointer` holds this.
- **The lyrics pass the pointer.** Resting there opens nothing, clicks reach the window under them, and they fade to 20% while the pointer is over them.
- **Polling costs nothing at rest.** The pointer's moves wake a poll only while the notch shows and no other app has the spot (`watch`). While the pointer is over it or it is open, it polls every 33 ms; while the app is active, or moves go unseen, every 100 ms.

## Making way

Atoll, boring.notch, NotchDrop and Droppy draw at the notch the same way, at the same level. With one running, both used to open on the same spot under the same pointer. So at launch, when the app becomes active, and whenever an app launches or quits (`NSWorkspace` notifications), it looks for a running app whose bundle id starts with one of `notchApps` (`notch_darwin.go`):

- `com.Ebullioscopic.Atoll` (and its `.dev` builds)
- `theboringteam.boringnotch`
- `wiki.qaq.NotchDrop`
- `iordv.Droppy`

While one runs, the shell shrinks to nothing, the pointer opens nothing, the pointer watch stops, the greeting waits (it isn't marked as done), and the hint under 显示刘海播放器 names the other app. The setting stays as the listener left it. When that app quits, the notch comes back. Add an app to the list only after checking its bundle id on a machine that has it.

## Frames

The shape springs between sizes. The lyrics ask for a frame every 40 ms while a word fills and otherwise wait for the next word or line (`stripFrames`); `TestNotchLyricFrames` holds them to 2 a second between words. The level bars repaint at the display's rate while music plays, which the roadmap lowers to 30.

## Tests and seams

`cursorPoint` (where the pointer is) and `notchNeighbor` (another app at the notch) are package variables that tests swap and restore with `defer`. `notchGreets` says the greeting runs, which only macOS does; `greetAnywhere` in the tests turns it on and keeps other apps out, so the greeting's tests pass on Linux and on a Mac running Atoll alike. Tests build a `notch` with no window, so `poll` records `passing` instead of calling the native window.

## On a Mac, before a release

1. Turn the notch on. Rest on it: the card opens. Move down onto the card and press pause, next and the scrubber: each works, and the card stays open.
2. Reset `notchGreeted` and `notch` in settings.json, play a song: the greeting shows. Click 开启.
3. Set 刘海歌词 to 2 行 and 3 行, with 翻译 and 音译 on and off. Rest on the lyrics: nothing opens, they fade, and a click reaches the window under them. Rest on the housing: the card opens.
4. A song with a long intro: the lyrics come in with the first line, and go after the last.
5. Launch Atoll with the notch on: ours goes, and the hint names Atoll. Quit Atoll: ours comes back.
6. An external display set as main: note where the notch goes (it takes `NSScreen.screens[0]` today).
