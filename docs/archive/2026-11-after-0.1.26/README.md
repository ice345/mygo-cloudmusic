# After 0.1.26: the plan, and what became of it

This folder keeps the plan written after 0.1.26 and the review of it, as they were. Nothing here is current: what comes next is in [`docs/roadmap.md`](../../roadmap.md), and how things work now is in the living docs beside it (`lyrics.md`, `stage.md`, `notch.md`, and `CONTRIBUTING.md` at the root).

| File | What it is |
|---|---|
| [`next.md`](next.md) | The plan: what to build after 0.1.26 and why, with the evidence. Written 2026-11-05 in `b7c043a`. |
| [`implementation.md`](implementation.md) | How to build it: order, branches, files, tests and commit messages, by work package. Written with `next.md`. |
| [`review-2026-11-17.md`](review-2026-11-17.md) | A review of the project on 2026-11-17, at `7c8f425`: defects, the plan's delivery, the lyric display, and a style after 『リズと青い鳥』 and 『響け！ユーフォニアム』. |

## Which commits delivered the plan

All of it landed on 2026-11-05, in `aa5d2e1..7c8f425`, in the guide's order.

| Work package | Commits | State |
|---|---|---|
| WP0 Fork and release | `3982879`, `280bfdc` (before the plan) | Partly. The rest moved to the roadmap: the signing secret, pushing `main`, a public repository. |
| WP1 Lyrics data | `aa5d2e1`, `da27a42`, `5af1284`, `b251518` | Done |
| WP2 Lyrics list and the pill | `c2aec04`, `83a419c`, `e74dba3` | Done. The lit line is re-aimed by polling for the moved box, the guide's fallback. |
| WP3 The stage's foot band | `dd9be95` | Done |
| WP4 The notch makes way | `43ace64` | Done, not checked on a Mac with Atoll |
| WP5 Lyrics under the notch | `c49756f` | Done. It broke the open card's hit area, fixed after the review. |
| WP6 青鸟 | `3243b7d` | Done, with the defects the review lists |
| WP7 Small fixes | `7c8f425` (one lyric request a song) | The stall and the notch's display were not started; they are on the roadmap. |

## Read these with care

- They describe the tree as it was on 2026-11-05: "the tree isn't clean", the `wip/reading-caption` branch, `ice345/cloudmusic`, a remote with `master` as its default. None of that holds now.
- They contradict each other in places: the window for attaching an extra line (500 ms in `next.md`, 200 ms in the guide and the code), the rule for a Japanese song, where `Lang` is set, and when 青鸟's answer arrives (§5.3 against §5.4). The code, and `docs/lyrics.md` and `docs/stage.md`, say what was built.
- `docs/aobird-preview.html`, the HTML mood demo they mention, was removed. It is in `b7c043a`: `git show b7c043a:docs/aobird-preview.html`.
- The frame folders they mention (`docs/aobird-frames/`, `docs/stage-caption/`) were never committed. `NM_FRAMES` writes them again.
