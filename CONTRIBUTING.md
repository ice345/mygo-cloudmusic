# Contributing

This is a fork of [egoist/mygo-cloudmusic](https://github.com/egoist/mygo-cloudmusic), which is archived: nothing comes from upstream any more, and changes land here.

## Read first

- [`AGENTS.md`](AGENTS.md): which tests to write, and how.
- [`.agents/skills/mygo-maintenance/SKILL.md`](.agents/skills/mygo-maintenance/SKILL.md): MyGo's element lifetime, keys, threads and input.
- MyGo's `ui/doc.go` at the pinned version: `go list -m -f '{{.Dir}}' github.com/egoist/mygo`.
- [`docs/roadmap.md`](docs/roadmap.md) for what's next, and [`docs/lyrics.md`](docs/lyrics.md), [`docs/stage.md`](docs/stage.md) and [`docs/notch.md`](docs/notch.md) for how those parts work.

## Commands

```bash
go tool mygo dev                                                # the app
go test ./internal/...                                          # before every commit
go test ./internal/app -run 'TestName' -count=5                 # every new UI test
go vet ./... && go tool mygo vet .                              # stored elements, goroutine captures
NM_FRAMES=$TMPDIR/frames go test ./internal/app -run 'Frames'   # frames to look at
LIVE=1 go test ./internal/netease -run 'Live'                   # NetEase itself
```

Go 1.27.3 or later. The tests need no display and no sound device.

## Conventions

1. **Threads.** Network on goroutines; results come back through `p.post` / `Window.Update`. Never block the UI thread. A package variable a test swaps (a seam) is read before the goroutine starts, not inside it.
2. **Identity.** Never store a `ui.Element`; keep data, `ui.Handle`s or scroll states. Stateful widgets get `c.Key(...)` before they are built. Geometry in a later callback comes from a handle; `LayoutBounds` is the previous frame's.
3. **Nothing moves because something else changed.** A toggle, a badge or a missing row must not move what's around it. Anchor at the edge that shouldn't move.
4. **Frames.** Ask with `c.AnimationFrame()` only while something visibly changes; otherwise `c.After(d)`. Budgets live in `internal/app/perf_test.go`.
5. **Controls.** Pills are `ui.ButtonBase(...).Padding(6, 14).Radius(999)` with 13 pt / 600 text: 55% white at rest, 6% background on hover, 14% when on. Menus use `b.Menu`. Settings rows use `row(label, desc, control)` with `ui.Switch` or `segmented`. Don't make a new kind of control.
6. **Settings.** A field with a full-sentence comment; anything that covers other apps or costs power is opt-in. `omitempty` for zero defaults; defaults in `defaultSettings`; `a.settings.save()` right after a change.
7. **Platform code.** macOS in `*_darwin.go` through `purego/objc`, with stubs in `*_other.go`. Windows and Linux must still build.
8. **Comments.** Full sentences about behaviour, in the listener's terms. A function's comment starts with its name and names its parameters in prose. Say why where a reader would ask.

## Branches and commits

- `main` is the trunk, and history is linear. Work on a topic branch cut from `main` (`fix/<topic>`, `lyrics/<topic>`, `stage/<topic>`, `notch/<topic>`, `docs/<topic>`, `ci/<topic>`), rebase it on `main`, merge it fast-forward, and delete it.
- **One commit, one behaviour,** with its tests, passing `go test ./internal/...` by itself. A regression found along the way is its own commit, before the change that needs it. No formatting changes mixed into behaviour.
- **The subject** is one line in English, lower case except names, starting with a verb (add, let, keep, show, set, make, move, ask, give, write). UI names are written as they appear (`add 歌单 to search`). After a colon, say what went wrong before, or why. Length is fine.

  Good: `keep the open notch card under the pointer: it let clicks through to the window under it and closed as the pointer moved onto its controls`. Bad: `fix notch`, `feat(notch): hit area`.
- An optional body of `- ` lines lists what else changed.
- Commits written with an AI assistant end with a `Co-Authored-By:` trailer naming it.
- `CHANGELOG.md` is written at release, in Chinese, for listeners, not in feature commits.
- **Never commit** frames (`*.png` from `NM_FRAMES`), `build/`, `.mygo/`, keys, or `.DS_Store`.

## Before merging

- [ ] `go test ./internal/...` passes; new UI tests pass with `-count=5`; `go tool mygo vet .` is clean.
- [ ] Each commit builds and passes by itself, and says one behaviour.
- [ ] Nothing moves when a toggle changes (look, don't assume).
- [ ] The frame budgets hold; a new animated surface has one in `perf_test.go`.
- [ ] Visual changes: `NM_FRAMES` frames looked at, in dark Now Playing, with a red, a green and a grey cover.
- [ ] macOS-only code has its `*_other.go` stub.
- [ ] Docs in `docs/` changed with the behaviour they describe.

## Releases

[`RELEASING.md`](RELEASING.md) has the steps. Two things they rely on:

**The update signing key.** Installed apps accept only updates signed with the Ed25519 key whose public half is `updates.publicKey` in `mygo.json`. CI signs with the repository's Actions secret `MYGO_UPDATER_PRIVATE_KEY` (Settings → Secrets and variables → Actions). The owner keeps a copy offline; MyGo's `keygen` writes it as `mygo-update.key`. Never commit it, print it or paste it into an issue.

To rotate it, only if it leaks or is lost:

1. `go tool mygo keygen -o <a new directory>`.
2. Put the new `mygo-update.pub` in `updates.publicKey`, and commit that.
3. `gh secret set MYGO_UPDATER_PRIVATE_KEY -R ice345/mygo-cloudmusic < <the new directory>/mygo-update.key`, then delete the old copy.

MyGo refuses to build with a key that doesn't match `updates.publicKey`, and each app holds the public key it was built with. So every app installed before the rotation stops updating: its users have to download the next release by hand. Say so in that release's notes.

**The speech libraries.** Podcast captions download sherpa-onnx's libraries from this repository's `sherpa-onnx-<version>` prerelease, named with their digests in `internal/speech/natives.go`. `go run ./scripts/sherpa` makes both; see `RELEASING.md`.
