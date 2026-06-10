---
title: Ask Questions
---

# Ask Questions

## Goal

Ask a question against an indexed document and get a page-cited answer.

## Prerequisites

- A document indexed into the workspace ([Index a PDF](/guides/index-a-pdf))
- An API key in `.env` (`OPENAI_API_KEY` or `ANTHROPIC_API_KEY`)

## Steps

**1. Ask.**

```sh
pindex ask "What was total revenue in fiscal 2022?"
```

If the workspace holds exactly one document, it is used automatically. With several documents, pass `--doc` — either the stored doc id or the original file path:

```sh
pindex ask "What was total revenue in fiscal 2022?" --doc report.pdf
```

**2. Read the citations.**

The answer goes to stdout; the citation line goes to stderr:

```
cited pages: [12 13 14]  (doc: report.pdf)
```

**3. Choose a model.**

The provider is routed by model name: names containing `claude` go to Anthropic, everything else goes to OpenAI.

```sh
pindex ask "..." --model gpt-4o-mini    # OpenAI, cheaper, weaker
pindex ask "..." --model gpt-4o         # OpenAI, stronger
```

Any `claude*` model name routes to Anthropic and needs `ANTHROPIC_API_KEY`.

::: tip
A `retrieve_model` set in a config YAML wins over `--model` for `ask` — see [model precedence](/guides/configuration#keys).
:::

## When the answer lacks support

If the model honestly says it can't find the answer, raise the effort:

```sh
pindex ask "..." --effort medium
```

`medium` retries once with a different page selection when the first pass is a refusal (`low`, the default, is a single pass; `high`/`ultra` currently behave like `medium`). Switching to a stronger model also helps — accuracy is model-bound.

To keep a browsable record, add `--out <dir>`; each Q&A plus the doc tree is written under `<out>/<doc>/`.

## What you should see

- stdout: the answer text, nothing else.
- stderr: a `cited pages: [...]` line (printed only when pages were cited).
