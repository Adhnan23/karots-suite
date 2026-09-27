.PHONY: db-up db-down migrate test run types web build verify-portable dev test-web test-fresh
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
