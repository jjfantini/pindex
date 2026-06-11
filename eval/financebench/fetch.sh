#!/usr/bin/env bash
# Fetch the FinanceBench dataset into testdata/ (CC-BY-NC-4.0 — fetched at eval
# time, never committed; testdata/.gitignore enforces this).
#
# Usage:
#   ./fetch.sh                  # questions JSONL only
#   ./fetch.sh DOC [DOC ...]    # questions + those documents' PDFs
#   ./fetch.sh --all            # questions + all PDFs (~84 docs, hundreds of MB)
set -euo pipefail
cd "$(dirname "$0")/testdata"

QURL=https://raw.githubusercontent.com/patronus-ai/financebench/main/data/financebench_open_source.jsonl
PURL=https://raw.githubusercontent.com/patronus-ai/financebench/main/pdfs

[ -f fb.jsonl ] || curl -fsSL -o fb.jsonl "$QURL"
mkdir -p pdfs

docs=()
if [ "${1:-}" = "--all" ]; then
  while IFS= read -r d; do docs+=("$d"); done < <(jq -r '.doc_name' fb.jsonl | sort -u)
elif [ $# -gt 0 ]; then
  docs=("$@")
fi

for d in ${docs[@]+"${docs[@]}"}; do
  if [ ! -f "pdfs/$d.pdf" ]; then
    echo "fetching $d.pdf"
    curl -fsSL -o "pdfs/$d.pdf" "$PURL/$d.pdf"
  fi
done

echo "questions: $(wc -l < fb.jsonl | tr -d ' ')   pdfs on disk: $(find pdfs -name '*.pdf' | wc -l | tr -d ' ')"
