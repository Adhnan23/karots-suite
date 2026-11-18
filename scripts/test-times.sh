#!/usr/bin/env bash
# Where the full test run spends its time: Go seconds per package and per test,
# Playwright seconds per spec. Go first, then Playwright (they share suite_db).
# Run detached; output in ~/.cache/karots-logs/test-times-<stamp>.txt.
# Needs ports 3000 / 5173 free and nothing else on suite_db.
set -u
cd "$(dirname "$0")/.."
logs=~/.cache/karots-logs; mkdir -p "$logs"
stamp=$(date +%Y%m%d-%H%M); out="$logs/test-times-$stamp.txt"
gojson="$logs/test-times-$stamp-go.json"; pwjson="$logs/test-times-$stamp-pw.json"

start=$(date +%s)
(set -a && . ./.env && set +a && go test -p 1 -json ./... > "$gojson" 2>&1)
gosecs=$(( $(date +%s) - start ))

start=$(date +%s)
(cd web && bunx playwright test --reporter=json > "$pwjson" 2>/dev/null)
pwsecs=$(( $(date +%s) - start ))

{
  echo "Go wall: ${gosecs}s   Playwright wall: ${pwsecs}s"
  echo; echo "== Go packages (s)"
  jq -r 'select(.Action=="pass" or .Action=="fail") | select(.Test==null) | "\(.Elapsed)\t\(.Action)\t\(.Package)"' "$gojson" | sort -rn | head -30
  echo; echo "== Go tests (s)"
  jq -r 'select((.Action=="pass" or .Action=="fail") and .Test!=null) | "\(.Elapsed)\t\(.Package | split("/") | last).\(.Test)"' "$gojson" | sort -rn | head -30
  echo; echo "== Go failures"
  jq -r 'select(.Action=="fail" and .Test!=null) | "\(.Package).\(.Test)"' "$gojson"
  echo; echo "== Playwright specs (s)"
  jq -r '[.suites[] | {f: .file, s: ([.. | objects | select(has("duration")) | .duration] | add / 1000)}] | sort_by(-.s) | .[:30][] | "\(.s | floor)\t\(.f)"' "$pwjson"
  echo; echo "== Playwright: $(jq -r '.stats | "expected \(.expected) unexpected \(.unexpected) flaky \(.flaky) skipped \(.skipped)"' "$pwjson")"
} > "$out"
echo "$out"
