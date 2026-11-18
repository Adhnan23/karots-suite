#!/usr/bin/env bash
# The full run before a Release, with a coverage map: Go (every package, coverage
# of the shipped code), Vitest (coverage), Playwright, check-core. Run detached;
# everything lands in ~/.cache/karots-logs/full-<stamp>/. Ports 3000 / 5173 free,
# nothing else on suite_db.
set -u
cd "$(dirname "$0")/.."
out=~/.cache/karots-logs/full-$(date +%Y%m%d-%H%M); mkdir -p "$out"
set -a; . ./.env; set +a
pkgs=$(go list ./... | grep -v -e /internal/demo -e /cmd/demo -e /cmd/bootstrap -e /scripts | paste -sd, -)
start=$(date +%s)
go test -p 1 -count=1 -coverpkg="$pkgs" -coverprofile="$out/go.cover" ./... > "$out/go.log" 2>&1; echo "go exit $? in $(( $(date +%s) - start ))s" >> "$out/summary.txt"
go tool cover -func="$out/go.cover" > "$out/go-func.txt" 2>/dev/null
start=$(date +%s)
(cd web && bunx vitest run --coverage --coverage.reporter=json-summary --coverage.reporter=text-summary --coverage.reportsDirectory="$out/web-cover" > "$out/web.log" 2>&1); echo "web exit $? in $(( $(date +%s) - start ))s" >> "$out/summary.txt"
start=$(date +%s)
(cd web && bunx playwright test > "$out/e2e.log" 2>&1); echo "e2e exit $? in $(( $(date +%s) - start ))s" >> "$out/summary.txt"
make check-core > "$out/core.log" 2>&1; echo "check-core exit $?" >> "$out/summary.txt"
echo DONE >> "$out/summary.txt"
