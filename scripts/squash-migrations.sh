#!/usr/bin/env bash
# One-shot: collapse migrations/*.sql into migrations/00001_baseline.sql and prove
# the result is identical (schema + seed rows) to running the originals.
set -euo pipefail
cd "$(dirname "$0")/.."
PSQL() { docker compose exec -T postgres psql -U suite_user -d "$1" -v ON_ERROR_STOP=1 -qAt -c "$2"; }
DUMP() { docker compose exec -T postgres pg_dump -U suite_user -d "$1" --no-owner --no-privileges \
           --exclude-table='goose_db_version*' "${@:2}" | grep -v '^\\' ; }  # drop \restrict meta-cmds
url() { echo "postgres://suite_user:suite_password@localhost:5433/$1?sslmode=disable"; }
fresh() { PSQL suite_db "DROP DATABASE IF EXISTS $1" && PSQL suite_db "CREATE DATABASE $1"; }
# The originals always come from karots-pos's migration files (frozen repo), so the
# check is reproducible any time — no leftover database required.
ORIG=${ORIG_MIGRATIONS:-$HOME/Projects/go/karots-pos/migrations}
migrate_orig() { go run ./scripts/migratedir "$(url "$1")" "$ORIG" >/dev/null; }
migrate() { DATABASE_URL="$(url "$1")" JWT_SECRET=squash-script-throwaway-secret-32b go run ./cmd/server -migrate >/dev/null; }
normalize() {  # mask values that legitimately differ between two fresh DBs
  sed -E "s/[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9:.]+(\+[0-9:]+)?/TS/g"
}
normalize_schema() {  # pg_dump renders `x = ANY (ARRAY[...])` casts differently depending on
  # whether the CHECK came from an inline CREATE TABLE constraint or a later ALTER TABLE
  # (whole-array cast vs per-element cast) — same constraint, different text. Collapse both
  # to one form so only real schema differences show up.
  sed -E -e "s/::text//g" -e "s/\)\[\]/\)/g" -e "s/\(('[^']*'::character varying)\)/\1/g" |
    awk '{ while ($0 ~ /\(\(/ || $0 ~ /\)\)/) { gsub(/\(\(/, "("); gsub(/\)\)/, ")") } print }'
}

fresh squash_orig && migrate_orig squash_orig

if [ "${1:-}" != "--verify-only" ]; then
  {
    echo "-- Baseline squashed from karots-pos migrations 0001–$(ls migrations/*.sql | tail -1 | xargs basename)."
    echo "-- +goose Up"
    echo "-- +goose StatementBegin"
    DUMP squash_orig --inserts | grep -v "set_config('search_path'"
    echo "-- install_id must be random PER DATABASE (support PIN is derived from it)."
    echo "UPDATE settings SET install_id = upper(substr(md5(random()::text || clock_timestamp()::text), 1, 8));"
    echo "-- +goose StatementEnd"
  } > /tmp/00001_baseline.sql
  mkdir -p /tmp/old-migrations && mv migrations/*.sql /tmp/old-migrations/
  mv /tmp/00001_baseline.sql migrations/00001_baseline.sql
fi

# Identity check: originals vs baseline, schema exact, data normalized.
fresh squash_base && migrate squash_base
for db in squash_orig squash_base; do PSQL "$db" "UPDATE settings SET install_id='X'"; done
diff <(DUMP squash_orig --schema-only | normalize_schema) <(DUMP squash_base --schema-only | normalize_schema) && echo "schema: IDENTICAL"
diff <(DUMP squash_orig --data-only --inserts | normalize) \
     <(DUMP squash_base --data-only --inserts | normalize) && echo "data: IDENTICAL"
