#!/usr/bin/env bash
# Lists every route karots-pos serves (core web + plugins) as a checklist.
# Re-run any time; hand-edited ticks must be preserved, so this only writes a
# fresh file when docs/parity.md does not exist yet.
set -euo pipefail
SRC=${1:-$HOME/Projects/go/karots-pos}
OUT=docs/parity.md
[ -e "$OUT" ] && { echo "$OUT exists; delete it to regenerate"; exit 1; }
{
  echo "# karots-pos → karots-suite parity"
  echo
  echo "Generated $(date +%F) from karots-pos $(git -C "$SRC" rev-parse --short HEAD)."
  echo "Tick a row only when the route's behaviour is ported to /api/v1 + the SPA AND has a flow check."
  echo "Format: file · METHOD path (paths are relative to that file's route group)."
  for dir in internal/web $(cd "$SRC" && ls -d plugins/*); do
    echo; echo "## $dir"; echo
    (cd "$SRC" && grep -rnoE '\.(GET|POST|PUT|PATCH|DELETE)\("[^"]*"' --include='*.go' "$dir" | grep -v _test.go) \
      | sed -E 's#^([^:]+):[0-9]+:\.(GET|POST|PUT|PATCH|DELETE)\("([^"]*)"#- [ ] `\1` · \2 `\3`#' | sort -u
  done
} > "$OUT"
grep -c '^- \[ \]' "$OUT" | xargs echo "rows:"
