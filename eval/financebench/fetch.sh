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

log() { printf '[%s] fetch: %s\n' "$(date +%H:%M:%S)" "$*" >&2; }

QURL=https://raw.githubusercontent.com/patronus-ai/financebench/main/data/financebench_open_source.jsonl
PURL=https://raw.githubusercontent.com/patronus-ai/financebench/main/pdfs

if [ -f fb.jsonl ]; then
  log "questions already cached (fb.jsonl, $(wc -l < fb.jsonl | tr -d ' ') questions)"
else
  log "downloading question set -> testdata/fb.jsonl"
  curl -fsSL -o fb.jsonl "$QURL"
  log "downloaded $(wc -l < fb.jsonl | tr -d ' ') questions"
fi
mkdir -p pdfs

docs=()
if [ "${1:-}" = "--all" ]; then
  while IFS= read -r d; do docs+=("$d"); done < <(jq -r '.doc_name' fb.jsonl | sort -u)
  log "fetching the full set: ${#docs[@]} documents"
elif [ $# -gt 0 ]; then
  docs=("$@")
fi

i=0
for d in ${docs[@]+"${docs[@]}"}; do
  i=$((i + 1))
  if [ -f "pdfs/$d.pdf" ]; then
    log "($i/${#docs[@]}) $d.pdf already cached"
  else
    log "($i/${#docs[@]}) downloading $d.pdf"
    curl -fsSL -o "pdfs/$d.pdf" "$PURL/$d.pdf"
  fi
done

log "done — questions: $(wc -l < fb.jsonl | tr -d ' '), pdfs on disk: $(find pdfs -name '*.pdf' | wc -l | tr -d ' ')"
