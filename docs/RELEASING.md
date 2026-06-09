# Releasing pindex

pindex uses **Conventional Commits + semantic-release-style automation**. You
never hand-edit a version or changelog; you merge a PR.

```
 PR ──CI (required)──▶ develop ──merge──▶ master ──┐
        (no-squash; every commit Conventional)     │
                                                    ▼
                          release-please maintains a "chore: release X.Y.Z" PR
                                                    │  (you review + merge it)
                                                    ▼
                              tag vX.Y.Z + GitHub Release  (pushed with the PAT)
                                                    │  triggers ↓
                                                    ▼
   release.yml: build cgo matrix → goreleaser builds lite → gh uploads ALL assets
              → scripts/publish-formulae.sh → jjfantini/homebrew-humbl
```

`brew install jjfantini/humbl/pindex` (cgo/MuPDF, AGPL-3.0) ·
`brew install jjfantini/humbl/pindex-lite` (pure-Go, Apache-2.0)

## Branch model

- **develop** — integration branch. CI runs on every push/PR.
- **master** — release branch. release-please watches it.
- Merge **develop → master with a real merge commit (no squash)** so the
  individual Conventional Commits survive for release-please to read. Do **not**
  enable "Require linear history" on master — it would block those merge commits.

## How to cut a release

1. Land work on `develop` via PRs (CI must be green; the `commit-guard` job
   enforces Conventional Commits).
2. Merge `develop → master`.
3. release-please opens/updates a **Release PR** ("chore: release X.Y.Z") with
   the computed version + changelog. Review it.
4. **Merge the Release PR.** That tags `vX.Y.Z`, creates the GitHub Release, and
   triggers `release.yml`, which builds binaries and updates the Homebrew tap.

That's it. Versioning is pre-1.0 (`bump-minor-pre-major`): breaking changes bump
the **minor** (0.x.0), features/fixes bump patch.

## First release (bootstrap to v0.1.0)

The manifest starts at `0.0.0`, and pre-1.0 a `feat:` commit bumps the **minor**
(`bump-minor-pre-major: true`, `bump-patch-for-minor-pre-major: false`). The
pipeline itself lands a `feat(license):` commit, so release-please computes
`0.0.0 → 0.1.0` for the first Release PR automatically — no extra step needed.

To force a specific first version regardless of commit types, add a `Release-As`
footer to a commit that reaches master, e.g.
`git commit --allow-empty -m "chore: bootstrap releases" -m "Release-As: 0.1.0"`.

## Secrets & GitHub Environment

The `release-please` job and the `publish` job run in the **`release`**
environment. Restrict that environment's deployment branches/tags to `master`
and `v*`. Two fine-grained PATs live there as **environment secrets**:

| Secret | Scope | Used by | Why |
| --- | --- | --- | --- |
| `RELEASE_PLEASE_TOKEN` | `pindex`: Contents R/W + Pull requests R/W | `release-please.yml` | Push the tag/Release so it **triggers** `release.yml` (the default `GITHUB_TOKEN` would not). |
| `HOMEBREW_TAP_TOKEN` | `homebrew-humbl`: Contents R/W | `scripts/publish-formulae.sh` | Push the updated formulae to the tap. |

`HOMEBREW_TAP_TOKEN` may be a shared "writes to my tap" token reused across
projects; `RELEASE_PLEASE_TOKEN` is pindex-scoped.

## Why the build pipeline looks the way it does

The full-fidelity `pindex` binary links **MuPDF via cgo** (go-fitz, which bundles
a static libmupdf per platform). Consequences:

- **cgo can't cross-compile**, so `pindex` is built **natively per platform** in
  the `build-cgo` matrix: `macos-latest` does darwin/arm64 (native) + darwin/amd64
  (`clang -arch x86_64`, Apple's universal toolchain); `ubuntu-latest` does
  linux/amd64; `ubuntu-24.04-arm` does linux/arm64.
- **OSS goreleaser can't package pre-built binaries** (the `prebuilt` builder and
  `--split`/merge are GoReleaser **Pro**). So goreleaser only builds + archives
  the pure-Go `pindex-lite` (which cross-compiles trivially) with
  `--skip=publish`; the cgo archives are tarred separately, and a **single `gh`
  step uploads everything** to the release release-please created (one upload
  mechanism, so there's no shared-release ambiguity between the two tools).
- **goreleaser deprecated `brews`** in favour of macOS-only `homebrew_casks`
  (which would drop Linux). So both formulae are templated by
  `scripts/publish-formulae.sh`, preserving Linux + macOS support.

`pindex-lite` excludes go-fitz/MuPDF (build-tagged out under `CGO_ENABLED=0`), so
it links no AGPL code and ships under Apache-2.0. It uses the pure-Go `purego`
extractor — see [`LICENSING.md`](../LICENSING.md).

## Verify locally

```sh
goreleaser check                                   # validate release config
make snapshot                                      # build+archive the 4 lite targets (no publish)
CGO_ENABLED=0 go list -deps ./cmd/pindex | grep go-fitz   # expect: no output (lite is AGPL-free)
VERSION=0.1.0 DIST_DIR=dist DRY_RUN=1 OUT_DIR=/tmp/f bash scripts/publish-formulae.sh   # render formulae
```

## Troubleshooting

- **Release PR merged but no binaries built** → the tag didn't trigger
  `release.yml`. Confirm `RELEASE_PLEASE_TOKEN` (a PAT, not `GITHUB_TOKEN`) is set
  in the `release` environment.
- **`publish` job can't push to the tap** → check `HOMEBREW_TAP_TOKEN` scope
  (Contents R/W on `homebrew-humbl`) and that `release` env policy allows the tag.
- **darwin cgo link failure** → ensure `go-fitz`'s bundled `libmupdf_darwin_*.a`
  is present (it is, in the module) and that the macOS runner used Apple clang.
- **A formula points at a missing asset** → the cgo `gh release upload` step must
  run before `publish-formulae.sh`; both run in the `publish` job in that order.
