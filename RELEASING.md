# Releasing

1. Clear the release-blocking items in `docs/roadmap.md` (§2 while it lists them): a release doesn't go out with one pending. Commit pending work, bring main up to date with origin, and run `go test ./internal/...`. Others push to main too, so a release cut from a stale main leaves out their work:

   ```bash
   git pull --rebase origin main
   ```

2. Bump `version` in `mygo.json`, and add a `## 0.1.8` section at the top of `CHANGELOG.md` saying, in Chinese, what changed for people using the app since the last release (`git log v0.1.7..HEAD`, after the pull). Commit both in a commit titled just the version, e.g. `0.1.8`. MyGo's updater shows that section in the app's update window, and the release page takes it as its notes; `mygo build` fails without one.

   macOS builds are unsigned (the signing and notarization steps were removed). While they stay unsigned, that section also tells people how to open a build the browser quarantined:

   > 从浏览器下载的 macOS 版本未经签名。若打开时提示已损坏或无法验证开发者，在「系统设置 → 隐私与安全性」中仍要打开，或在终端对应用执行 `xattr -dr com.apple.quarantine` 后再打开。
3. Push main first, and tag only once that push has gone through. Pushing both at once still pushes the tag when main is rejected, and the tag starts a release build of a commit that isn't on main:

   ```bash
   git push origin main
   git tag v0.1.8
   git push origin v0.1.8
   ```

   If main is rejected, pull again, add what came in to the changelog, amend the version commit, test, and push main again before tagging. If a tag went up on the wrong commit anyway, cancel its run, then delete the tag and the draft it made before tagging again:

   ```bash
   git push origin :refs/tags/v0.1.8
   gh release delete v0.1.8 --yes
   ```

4. `.github/workflows/release.yml` builds macOS, Windows and Linux and uploads them to a draft release. Wait for every job to pass:

   ```bash
   gh run watch "$(gh run list --workflow release.yml -L1 --json databaseId -q '.[0].databaseId')" --exit-status
   ```

   If a job fails, don't publish; fix it first.

5. Publish the draft and mark it latest:

   ```bash
   gh release edit v0.1.8 --draft=false --latest
   ```

6. Check the update feed shows the new version:

   ```bash
   curl -sL https://github.com/ice345/mygo-cloudmusic/releases/latest/download/update-darwin-arm64.json
   ```

7. The download site at https://mygo-cloudmusic.egoist.workers.dev is upstream's. This repository has not deployed its own. `website/src/site.ts` and `website/vite.config.ts` still read `egoist/mygo-cloudmusic`, so building `website/` from here would publish upstream's releases. Leave that check until the site is pointed at this repository and deployed. Skip it for a release of this repository.

## The speech recognition libraries

Podcast captions run sherpa-onnx, whose libraries the app downloads with the model from this repository's `sherpa-onnx-<version>` prerelease, not from the app's own releases. A prerelease never becomes the latest that updates look for. `internal/speech/natives.go` names each file with its digests; both come from `scripts/sherpa`, and they only change with sherpa-onnx's version.

A macOS build loaded them on Apple silicon on 2026-11-24, which 0.1.27 had shipped without; an Intel Mac has not (`docs/lyrics.md`, Podcast captions). The macOS libraries keep sherpa-onnx's own ad-hoc signatures, which this repository's builds load: they are signed ad hoc, and MyGo turns on the hardened runtime only for a real identity. If this repository later signs with a Developer ID, library validation will refuse them: pack them again with `-sign` and that identity, and upload the new ones over the old.

To make them, from the repository's root (the script reads the repository from `mygo.json`'s `updates.github`):

```bash
go run ./scripts/sherpa -out build/sherpa      # add -sign "Developer ID Application: …" once the app has one
gh release create sherpa-onnx-2.0.1 --prerelease --latest=false build/sherpa/*
LIVE=1 go test ./internal/speech -run TestNativesLive
```
