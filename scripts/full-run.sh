#!/usr/bin/env bash
# The full run before a Release: Go, Vitest and Playwright side by side (each Go
# package and each Playwright worker on its own database, so nothing collides),
# then check-core. COVER=1 also writes the coverage map's numbers (slower Go).
# Run detached; everything lands in ~/.cache/karots-logs/full-<stamp>/.
set -u
cd "$(dirname "$0")/.."
out=~/.cache/karots-logs/full-$(date +%Y%m%d-%H%M); mkdir -p "$out"
set -a; . ./.env; set +a
t0=$(date +%s)
go build -o bin/testdb ./scripts/testdb && ./bin/testdb template > "$out/template.log" 2>&1
cover=()
if [ "${COVER:-}" = 1 ]; then
  pkgs=$(go list ./... | grep -v -e /internal/demo -e /cmd/demo -e /cmd/bootstrap -e /scripts | paste -sd, -)
  cover=(-coverpkg="$pkgs" -coverprofile="$out/go.cover")
fi
( s=$(date +%s); go test -count=1 -exec "$PWD/bin/testdb exec" "${cover[@]}" ./... > "$out/go.log" 2>&1
  echo "go exit $? in $(( $(date +%s) - s ))s" >> "$out/summary.txt" ) &
( s=$(date +%s); cd web && bunx vitest run ${COVER:+--coverage --coverage.reporter=json-summary --coverage.reportsDirectory="$out/web-cover"} > "$out/web.log" 2>&1
  echo "web exit $? in $(( $(date +%s) - s ))s" >> "$out/summary.txt" ) &
( s=$(date +%s); cd web && bunx playwright test > "$out/e2e.log" 2>&1
  echo "e2e exit $? in $(( $(date +%s) - s ))s" >> "$out/summary.txt" ) &
wait
[ -f "$out/go.cover" ] && go tool cover -func="$out/go.cover" > "$out/go-func.txt" 2>/dev/null
make check-core > "$out/core.log" 2>&1; echo "check-core exit $?" >> "$out/summary.txt"
echo "total $(( $(date +%s) - t0 ))s" >> "$out/summary.txt"
echo DONE >> "$out/summary.txt"
