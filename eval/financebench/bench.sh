#!/usr/bin/env bash
# Run the incremental FinanceBench benchmark for ONE document and fold it into
# the accumulating results tree (results/<model>/<effort>/<DOC>/), then rebuild
# every derived summary via the aggregator. Run it again with another document
# whenever you want to extend coverage — existing results are never recomputed.
#
# Usage:
#   ./bench.sh DOC_NAME [--model M] [--judge J] [--efforts "low medium high ultra"]
#
# Example:
#   ./bench.sh BOEING_2022_10K
#   ./bench.sh PEPSICO_2022_10K --model gpt-4o-mini --efforts "low high"
set -euo pipefail
cd "$(dirname "$0")" # eval/financebench

log() { printf '[%s] bench: %s\n' "$(date +%H:%M:%S)" "$*" >&2; }

DOC="${1:?usage: bench.sh DOC_NAME [--model M] [--judge J] [--efforts \"low high\"]}"
shift
MODEL=claude-haiku-4-5-20251001
JUDGE=gpt-4o-2024-11-20
EFFORTS="low medium high ultra"
while [ $# -gt 0 ]; do
  case "$1" in
  --model) MODEL="$2"; shift 2 ;;
  --judge) JUDGE="$2"; shift 2 ;;
  --efforts) EFFORTS="$2"; shift 2 ;;
  *) echo "unknown arg: $1" >&2; exit 1 ;;
  esac
done
SAN=$(printf '%s' "$MODEL" | sed -E 's/[^A-Za-z0-9._-]+/_/g')
REPO_ROOT=$(cd ../.. && pwd)

log "document=$DOC  model=$MODEL  judge=$JUDGE  efforts=[$EFFORTS]"

log "step 1/4: fetching dataset"
./fetch.sh "$DOC"
jq -c --arg d "$DOC" 'select(.doc_name==$d)' testdata/fb.jsonl > "testdata/$DOC.jsonl"
[ -s "testdata/$DOC.jsonl" ] || { echo "no FinanceBench questions for doc_name=$DOC" >&2; exit 1; }
NQ=$(wc -l < "testdata/$DOC.jsonl" | tr -d ' ')
log "$DOC has $NQ questions"

log "step 2/4: building pindex"
BIN=$(mktemp -d)/pindex
go build -o "$BIN" "$REPO_ROOT/cmd/pindex"

# Indexing is resumable and skipped when the doc is already in the workspace.
log "step 3/4: indexing into testdata/ws (LLM-bound — a 100-page filing can take many minutes; already-indexed docs are skipped)"
"$BIN" index "testdata/pdfs/$DOC.pdf" --workspace testdata/ws \
  --model "$MODEL" --env-file "$REPO_ROOT/.env"
log "indexing done"

NEFF=$(set -- $EFFORTS; echo $#)
i=0
for EFF in $EFFORTS; do
  i=$((i + 1))
  log "step 4/4 ($i/$NEFF): eval --effort $EFF ($NQ questions; per-question funnel lines follow)"
  OUT=$(mktemp -d)
  "$BIN" eval --questions "testdata/$DOC.jsonl" --workspace testdata/ws \
    --model "$MODEL" --judge-model "$JUDGE" --effort "$EFF" --out "$OUT" \
    --env-file "$REPO_ROOT/.env"

  DST="results/$SAN/$EFF/$DOC"
  log "effort $EFF done — ingesting records into $DST"
  mkdir -p "$DST" "results/$SAN/trees"
  rm -rf "$DST/answers"
  cp -R "$OUT/$DOC/answers" "$DST/"
  # one text-stripped tree per doc, shared by all efforts
  [ -f "results/$SAN/trees/${DOC}_pindex.json" ] || cp "$OUT/$DOC/${DOC}_pindex.json" "results/$SAN/trees/"
  N=$(find "$DST/answers" -name '*.json' | wc -l | tr -d ' ')
  cat > "$DST/run.json" <<JSON
{
 "doc_name": "$DOC",
 "generated_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
 "model": "$MODEL",
 "judge_model": "$JUDGE",
 "effort": "$EFF",
 "questions": $N
}
JSON
done

log "re-aggregating the scoreboard"
go run ./aggregate results
log "done — $DOC folded into results/$SAN at every requested effort"
