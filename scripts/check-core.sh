#!/usr/bin/env bash
# Core never names a plugin (AGENTS.md hard rule): no import, no literal, not even
# an "e.g." in a comment. Core = everything outside plugins/ and web/src/plugins/.
# Migrations count as core too. Also fails when a generated plugin list was committed with a dev selection in it.
set -euo pipefail
cd "$(dirname "$0")/.."

# Distinctive plugin names and tender values: never in core at all.
names='clearance|productplus|aicatalog|pharmacy|recharge|wallet'
# Keys that are also plain English: only an import path or a quoted literal counts,
# so "HTTP requests" or "repairs" in a sentence is fine.
keys='requests|documents|alternatives|repairs|combos'
for f in plugins/*/plugin.json; do [ -f "$f" ] && keys="$keys|$(basename "$(dirname "$f")")"; done

src() {
  grep -rnE "$@" internal cmd web/src migrations --include='*.go' --include='*.ts' --include='*.tsx' --include='*.sql' \
    | grep -v '^web/src/plugins/' | grep -v '^cmd/bootstrap/' | grep -v 'types.gen.ts' || true
}
# \bWallet\b with a capital W is the lucide-react icon, not the tender.
hits=$( { src -iw "$names" | grep -vE '\bWallet\b' ; src "plugins/($keys)\b|[\"'\`]($keys)[\"'\`]"; } || true)
if [ -n "$hits" ]; then
  echo "core names a plugin:"; echo "$hits"; exit 1
fi
for f in cmd/server/enabled_plugins.go web/src/plugins/enabled-plugins.ts; do
  if [ -f "$f" ] && grep -qE "plugins/|from '\./" "$f"; then
    echo "$f is not core-only — run: make core-only"; exit 1
  fi
done
echo "OK: core is plugin-inert"
