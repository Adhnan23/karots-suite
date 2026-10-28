# Developer guide

*For developers.* Read [`AGENTS.md`](../../AGENTS.md) and [Architecture](architecture.md) first.

## Setup

Needs Go 1.26, bun, Docker (for the dev PostgreSQL) and `make`. On Windows, `build.ps1` stands in
for the Makefile.

```bash
cp .env.example .env
make db-up          # PostgreSQL suite_db on :5433
make migrate
make dev            # Go (air) on :3100 + Vite on :5173, proxying /api
```

Seed a starter shop with `go run ./cmd/server -seed`. The dev database is disposable, so drop and
re-migrate freely. Karots POS can run alongside on 5432 / 3000.

## Everyday commands

| Command | Does |
|---|---|
| `make test` | Go tests (DB tests run when `DATABASE_URL` is set) |
| `make test-web` | Vitest |
| `make e2e-seed && cd web && bunx playwright test` | End-to-end flows, built with every plugin |
| `make types` | Regenerate `web/src/api/types.gen.ts` after any `dto*.go` change |
| `make plugins P=all` / `P=clearance,…` | Compile plugins in for dev |
| `make core-only` | Put the plugin lists back. **Never commit a plugin selection** |
| `make verify` | `check-core`, `go vet`, build, `tsc` |
| `make build && make verify-portable` | The static binary (`CGO_ENABLED=0 GOAMD64=v1`) |
| `make bootstrap` | The Shop Builder |
| `make demo` / `make demo-dev` | The Demo Room |
| `make docs-shots` | Retake `docs/images` from a running Demo Room |
| `go run ./scripts/notices > THIRD_PARTY_NOTICES.md` | Regenerate the third-party notices |

Always build the web app through `make`, because `vite build` empties `internal/spa/dist`.

## Adding to the API

1. Add the types to `internal/api/dto*.go`, then run `make types`.
2. Write the handler in `internal/api/<area>.go` on the right group (`h.protected`, admin, till).
3. Errors come from `apperr`. Money stays a decimal string.

## Migrations

Core migrations live in `migrations/` (goose, numbered; next is `00014_`). A plugin keeps its own
in `plugins/<key>/` with its own version table. Never hard-code an install ID in a migration.

## Writing a plugin

1. `plugins/<key>/plugin.json` (name, description, `requires`) and `init()` → `plugin.Register`.
2. Routes on `reg.Admin()` / `reg.Cashier()`, hooks with `reg.Add…`, and migrations.
3. `web/src/plugins/<key>/index.tsx`: `export default register(slots)`, which pushes into the
   slots it needs.
4. If core seems to need to know your plugin, add a generic hook or slot instead.
5. Add a Playwright spec (`web/e2e/plugin-<key>.spec.ts`).

## Tests

- Test first, one task at a time, one commit per step.
- Playwright runs one worker against one server. Import `test` from `e2e/fixtures.ts`, which waits
  out the login limiter.
- Don't run `go test` while Playwright is running, because they share `suite_db`.
- Kill anything on 3100 / 5173 before Playwright, which reuses whatever is listening there.

## Screenshots for the docs

```bash
make demo                                    # once, or after UI changes
dist/demo/karots-suite-demo-room &           # leave it on the picker
make docs-shots                              # all shots; DOCS_ONLY=till/ for some
```

The spec (`web/docs-shots/shots.spec.ts`) unlocks the demo with the hourly code, opens Karots
Super Store, hides the demo's chrome and fills empty screens before each shot. Output names are
stable, because the guides link to them.

## Commits

Small commits on `main`, one per step, with no AI attribution lines.
