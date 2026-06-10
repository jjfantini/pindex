---
title: Configuration
---

# Configuration

## Goal

Set up a pindex config YAML and the `.env` file that holds your API keys.

## The config file

There is **no automatic config discovery**. A config file is only read when you pass it explicitly:

```sh
pindex index report.pdf --config pindex.yaml
```

An empty path or a missing file silently falls back to the built-in defaults; a present file is YAML-parsed over the defaults, then validated.

### Keys

| Key | Type | Default | What it does |
|---|---|---|---|
| `model` | string | `gpt-4o-2024-11-20` | Default LLM for indexing-time reasoning |
| `retrieve_model` | string | `""` (falls back to `model`) | LLM for `ask` / `eval` |
| `extractor` | string | `mupdf` | PDF extractor: `mupdf`, `poppler`, `purego`, `vision` |
| `toc_check_page_num` | int | `10` | Leading pages scanned for a TOC; `0` disables TOC detection |
| `max_page_num_each_node` | int | `10` | Page-span gate for recursive node splitting |
| `max_token_num_each_node` | int | `20000` | Token gate for splitting and page-group size for structure generation |
| `if_add_node_id` | bool | `true` | Write node ids |
| `if_add_node_summary` | bool | `true` | Generate LLM node summaries |
| `if_add_doc_description` | bool | `false` | One-sentence document description |
| `if_add_node_text` | bool | `false` | Parsed and accepted, but currently unused (reserved) |

Validation: `model` must be non-empty, `extractor` must be one of the four values above, `toc_check_page_num >= 0`, both `max_*_each_node > 0`.

::: tip Model precedence
`--model` on the command line overrides the `model` key only. So if your YAML sets `retrieve_model`, it still wins over `--model` for `ask` and `eval`.
:::

## API keys: `.env`

`index`, `ask`, and `eval` load a `.env` file (default `./.env`, change with `--env-file`); `extract` does not, since it makes no LLM calls. Create one next to where you run pindex with the keys that matter:

```
OPENAI_API_KEY=...
ANTHROPIC_API_KEY=...
```

You need the one matching your model: names containing `claude` route to Anthropic, everything else to OpenAI.

**Precedence:** values in `.env` **override** the inherited process environment — an updated `.env` always wins. A missing file is a no-op. Blank lines and `#` comments are skipped; a leading `export ` and surrounding quotes are stripped.

## What you should see

- With no `--config`, commands run on the defaults above.
- With an invalid key value (e.g. `extractor: foo`), the command fails fast with a validation error instead of running.
