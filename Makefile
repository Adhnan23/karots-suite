.PHONY: db-up db-down migrate test run
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
