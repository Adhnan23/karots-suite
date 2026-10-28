.PHONY: demo demo-fresh demo-web demo-dev demo-reorder docs-shots videos-clips videos agreement db-up db-down migrate test run types web build verify-portable dev test-web test-fresh e2e-seed check-core bootstrap plugins core-only verify setup shop shop-test support-pin
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
# The Demo Room's pages and chrome (cmd/demo only; never in a shop's SPA).
demo-web:
	cd web && bunx vite build --config demo/vite.config.ts
	touch internal/demo/ui/dist/.gitkeep
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

# Demo data for trying smart reorder levels: the demo shop plus a year of sales and
# deliveries (wipes the dev DB). Start the server after; it learns the levels a minute later.
demo-reorder: db-up
	$(ENV) go run ./cmd/server -reset -demo
	docker compose exec -T postgres psql -U suite_user -d suite_db -v ON_ERROR_STOP=1 -q -f - < scripts/demo-reorder.sql

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
# A local Test build of one shop (nothing uploaded): make shop-test S=al-raj
shop-test:
	go run ./cmd/bootstrap -shop $(S) -test
# A shop's current support PIN (hourly): make support-pin ID=BB876E88
support-pin:
	go run ./cmd/server -support-pin $(ID)
# The installer for this PC (shop builds make their own per OS).
setup:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/karots-setup ./cmd/setup
# Dev: compile plugins into the working tree (P=all or P=clearance,requests).
# Never commit the result — make core-only puts the lists back (check-core enforces it).
plugins:
	go run ./cmd/bootstrap -write-only -plugins $(or $(P),all)
core-only:
	go run ./cmd/bootstrap -write-only -plugins none
# The Demo Room: dist/demo/karots-suite-demo-room (Linux) + .exe (Windows), every
# built-in shop's year built once and carried inside. Needs POS_SUPPORT_SECRET for
# its rotating unlock code (internal/demo/INSTALL_ID is its install id).
demo:
	$(ENV) go run ./cmd/bootstrap -demo
# The same, building every shop's year again even when nothing that shapes them changed.
demo-fresh:
	$(ENV) go run ./cmd/bootstrap -demo -fresh
# The Demo Room for this machine (dev): every plugin in the SPA, gated per shop;
# the file-mode PostgreSQL from the user cache (go test ./internal/filedb fetches it).
demo-dev:
	go run ./cmd/bootstrap -write-only -demo-list
	$(MAKE) web; s=$$?; go run ./cmd/bootstrap -write-only -plugins none; exit $$s
	$(MAKE) demo-web
	mkdir -p internal/filedb/bundle && cp $$(ls $${XDG_CACHE_HOME:-$$HOME/.cache}/karots-suite-filedb/*linux-amd64*.txz | tail -1) internal/filedb/bundle/postgres.txz
	go build -tags filemode -o bin/karots-demo ./cmd/demo; s=$$?; rm -rf internal/filedb/bundle; exit $$s
	./bin/karots-demo
# Screenshots for README + docs/guide (docs/images) from a running Demo Room
# (dist/demo/karots-suite-demo-room); DOCS_ONLY=<substring> retakes some.
docs-shots:
	cd web && DOCS_CODE=$$(cd .. && $(ENV) go run ./cmd/server -support-pin $$(cat internal/demo/INSTALL_ID) | grep -o 'PIN [0-9]*' | cut -d' ' -f2) bunx playwright test -c playwright.docs.config.ts
# Screen clips for the promo videos (videos/public/clips) from a running Demo Room; DOCS_ONLY=<name> redoes one.
videos-clips:
	cd web && DOCS_CLIPS=1 DOCS_CODE=$$(cd .. && $(ENV) go run ./cmd/server -support-pin $$(cat internal/demo/INSTALL_ID) | grep -o 'PIN [0-9]*' | cut -d' ' -f2) bunx playwright test -c playwright.docs.config.ts
# The 27 promo videos (videos/README.md).
videos:
	cd videos && bun run render
# The printable agreement a shop signs (Order Form with blanks + the terms from LICENSE).
agreement:
	cd web && bun docs-shots/agreement.ts
# Core is plugin-inert and still builds with no plugin at all.
verify: check-core
	go vet ./... && go build ./... && cd web && bunx tsc -b
