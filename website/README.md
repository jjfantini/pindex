# pindex documentation site

VitePress site for [pindex](https://github.com/jjfantini/pindex), deployed to
GitHub Pages by `.github/workflows/pages.yml`.

## Run locally

```sh
cd website
npm ci          # or: npm install
npm run docs:dev
```

`npm run docs:build` builds the static site into `.vitepress/dist/`, and
`npm run docs:preview` serves that build locally.

## CLI reference (`reference/`)

The pages under `reference/` are **auto-generated** — never hand-edit them.
They come from a hidden cobra subcommand. Regenerate from the repo root:

```sh
go run ./cmd/pindex docs --dir website/reference
```

The generated files are committed, and CI regenerates them on every deploy so
the published reference always matches the command tree on the deployed
branch.

## Future automation

Planned (not implemented): a Claude skill/loop that makes directed edits to
these docs on each version bump — i.e. as part of every release-please PR —
so the prose pages stay in sync with shipped behavior.
