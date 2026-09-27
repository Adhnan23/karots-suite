# Karots Suite

Business system for small Sri Lankan retail — till, stock, purchasing, money, reports.
JSON-only Go backend + a React SPA, shipped as one static binary. See `AGENTS.md` for
the full layout, conventions and hidden behaviours.

## Quickstart

```bash
cp .env.example .env
make db-up          # Postgres on :5433
make migrate
make dev            # Go (air) on :3100 + Vite on :5173, proxying /api
```

Open http://localhost:5173. Seed a starter dataset with `go run ./cmd/server -seed`
(Admin `0771234567` / `1234`).

## Testing

```bash
make test           # Go
make test-web       # Vitest
make e2e-seed && (cd web && bunx playwright test)
```

## Shipping

```bash
make build && make verify-portable
```
