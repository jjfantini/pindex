# pindex examples

A complete, real end-to-end run of pindex on a small public document, so you
can see what the tool produces without spending any LLM tokens yourself.

## Files

| File | What it is |
|---|---|
| `bitcoin.pdf` | The input: Satoshi Nakamoto, *"Bitcoin: A Peer-to-Peer Electronic Cash System"* (9 pages, ~180 KB). Downloaded from <https://bitcoin.org/bitcoin.pdf>. The Bitcoin whitepaper is distributed under the MIT License. |
| `output/structure.json` | The tree index pindex built for the PDF — the browsable per-doc export (`<workspace>/pindex/bitcoin/bitcoin_pindex.json`), copied verbatim from the workspace. Sections, node ids, page ranges, and LLM summaries; no embeddings, no chunks. |
| `output/questions-and-answers.md` | An unedited transcript of three real `pindex ask` runs against the index, with the exact commands and the page citations each answer produced. |

## Reproduce in 3 commands

From a directory containing `bitcoin.pdf` and a `.env` with `OPENAI_API_KEY=...`:

```sh
pindex index bitcoin.pdf --model gpt-4o-mini
pindex ask "How does the network prevent double-spending?" --model gpt-4o-mini
pindex ask "What incentive do nodes have to support the network honestly?" --model gpt-4o-mini
```

The tree prints to stdout and is saved under `.pindex/workspace/`; each answer
prints to stdout with a `cited pages: [...]` line on stderr. Re-running `index`
is nearly free — every LLM response is cached by prompt hash in `.pindex/cache`.

See the guides: [Index a PDF](../website/guides/index-a-pdf.md) and
[Ask questions](../website/guides/ask-questions.md).
