# Accuracy diagnostic set

`diagnostic_set.json` defines 15 short FinanceBench documents (4–72 pages, 29 questions) split into
`train` (tune here) and `heldout` (report here, unseen during tuning). It stores only doc names +
the split; the questions are CC-BY-NC-4.0 FinanceBench content and are fetched at eval time.

## Reproduce a run

```sh
# 1. Fetch the question set + the diagnostic PDFs
curl -sSL -o fb.jsonl https://raw.githubusercontent.com/patronus-ai/financebench/main/data/financebench_open_source.jsonl
jq -r '.train[],.heldout[]' eval/financebench/testdata/diagnostic_set.json | while read d; do
  curl -sSL -o "pdfs/$d.pdf" "https://raw.githubusercontent.com/patronus-ai/financebench/main/pdfs/$d.pdf"
done

# 2. Filter the questions to one split (e.g. heldout) and index its docs
jq -c 'select(.doc_name as $d | $SPLIT | index($d))' \
   --argjson SPLIT "$(jq '.heldout' eval/financebench/testdata/diagnostic_set.json)" fb.jsonl > heldout.jsonl
pindex index pdfs/ --workspace ws --model gpt-4o --env-file .env

# 3. Score with the stage funnel (extraction / retrieval / answer / hallucination)
pindex eval --questions heldout.jsonl --workspace ws --model gpt-4o --judge-model gpt-4o --env-file .env
```

The funnel localizes where accuracy is lost; tune prompts against `train`, then report `heldout`.
PDFs and `*.jsonl` are gitignored — never committed.
