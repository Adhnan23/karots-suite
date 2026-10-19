.PHONY: db-up db-down migrate test run types web build verify-portable dev test-web test-fresh e2e-seed check-core bootstrap plugins core-only verify setup shop
ENV = set -a && . ./.env && set +a &&

db-up:
	docker compose up -d --wait postgres
db-down:
	docker compose down
migrate: db-up
	$(ENV) go run ./cmd/server -migrate
run: db-up
	$(ENV) go run ./cmd/server
test:
	$(ENV) go test ./...
types:
	tygo generate
web: types
	cd web && bun install --frozen-lockfile && bun run build
	touch internal/spa/dist/.gitkeep
build: web
	CGO_ENABLED=0 GOAMD64=v1 go build -ldflags="-s -w" -o bin/karots-suite ./cmd/server
verify-portable:
	@file bin/karots-suite | grep -q "statically linked" \
	  && echo "OK: bin/karots-suite is statically linked (portable)" \
	  || { echo "FAIL: bin/karots-suite is NOT static"; exit 1; }
dev: db-up types
	$(ENV) air & cd web && bun run dev
test-web:
	cd web && bun run test

# Wipe the dev DB, migrate + seed it, then run every Go test. Several forked DB
# tests assume seeded reference rows (a category, unit, user) exist.
test-fresh: db-up
	$(ENV) go run ./cmd/server -reset -seed && $(ENV) go test ./...

# Fresh DB + the fixed e2e logins (0770000001 admin, 0770000002 cashier, PIN 1234)
# + the till fixtures (products/lots/customer/card account). Resets the dev DB.
e2e-seed: db-up
	$(ENV) go run ./cmd/server -reset
	docker compose exec -T postgres psql -U suite_user -d suite_db -v ON_ERROR_STOP=1 -f - < scripts/e2e-users.sql
	docker compose exec -T postgres psql -U suite_user -d suite_db -v ON_ERROR_STOP=1 -f - < scripts/e2e-till.sql

# Core never names a plugin (AGENTS.md hard rule); the generated plugin lists are core-only.
check-core:
	bash scripts/check-core.sh

# Per-shop build: opens the bootstrap app in the browser (pick plugins, target,
# install id). Scripts pass flags instead: go run ./cmd/bootstrap -plugins all -os windows
bootstrap:
	go run ./cmd/bootstrap
# A shop from shops.json → dist/<key>/ (karots-setup, install.conf, the binary): make shop S=al-raj (or S=all)
shop:
	go run ./cmd/bootstrap -shop $(S)
# The installer for this PC (shop builds make their own per OS).
setup:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/karots-setup ./cmd/setup
# Dev: compile plugins into the working tree (P=all or P=clearance,requests).
# Never commit the result — make core-only puts the lists back (check-core enforces it).
plugins:
	go run ./cmd/bootstrap -write-only -plugins $(or $(P),all)
core-only:
	go run ./cmd/bootstrap -write-only -plugins none
# Core is plugin-inert and still builds with no plugin at all.
verify: check-core
	go vet ./... && go build ./... && cd web && bunx tsc -b
