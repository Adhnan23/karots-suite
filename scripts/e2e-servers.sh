#!/usr/bin/env bash
# Playwright's servers: the binary built once (every plugin), then for each of N
# workers a fresh database seeded like `make e2e-seed` and its own server
# (port 3000+i, Pocket Cashier 8443+i). Server 0 starts last, so Playwright's
# ready check (port 3000) means every one is up. Ctrl-C / Playwright's stop ends them all.
set -euo pipefail
cd "$(dirname "$0")/.."
n=${E2E_WORKERS:-3}
# E2E_PREBUILT=1: the caller already built it (scripts/full-run.sh, so the plugin
# lists never change under Go tests compiling beside it).
[ -n "${E2E_PREBUILT:-}" ] || { make plugins P=all >/dev/null && make build >/dev/null && make core-only >/dev/null; }
set -a; . ./.env; set +a
base=${DATABASE_URL%/*}
q=""; [[ "$DATABASE_URL" == *\?* ]] && q="?${DATABASE_URL#*\?}"
psql() { docker compose exec -T postgres psql -U suite_user -v ON_ERROR_STOP=1 -q "$@"; }
pids=()
trap 'kill "${pids[@]}" 2>/dev/null; wait' EXIT INT TERM
for ((i = n - 1; i >= 0; i--)); do
  db=e2e_$i
  psql -d postgres -c "DROP DATABASE IF EXISTS $db WITH (FORCE)" -c "CREATE DATABASE $db"
  url="$base/$db$q"
  DATABASE_URL=$url ./bin/karots-suite -reset >/dev/null
  psql -d $db -f - < scripts/e2e-users.sql
  psql -d $db -f - < scripts/e2e-till.sql
  DATABASE_URL=$url SERVER_PORT=$((3000 + i)) KAROTS_POCKET_PORT=$((8443 + i)) KAROTS_NO_SPOOL=1 ./bin/karots-suite \
    > ~/.cache/karots-logs/e2e-server-$i.log 2>&1 &
  pids+=($!)
  if ((i > 0)); then
    until curl -sf "http://localhost:$((3000 + i))/api/v1/auth/login-info" >/dev/null; do sleep 0.3; done
  fi
done
wait
