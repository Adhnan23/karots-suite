# M0 Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a karots-suite binary that boots on a forked karots-pos domain with a single squashed baseline migration. It serves a JSON-only `/api/v1` plus an embedded React SPA. Users can log in (with the install-ID Konami), lock and unlock, change their PIN, and reach an empty responsive admin shell or till shell. The system user can set Appearance, and admins and support can Konami-exit the kiosk.

**Architecture:**
- **Backend:** the Go domain (`internal/features/*`, infra packages, `cmd/server`) is copied from karots-pos at `HEAD` and the module is renamed. The HTML layer is deleted. A new `internal/api` package exposes `/api/v1` with httpOnly-cookie JWT auth.
- **Frontend:** a Vite + React + TypeScript app lives in `web/`. `web/go.mod` fences it off from Go's `./...`, because Go walks into `node_modules`. The app builds into `internal/spa/dist`, which is `go:embed`ed. Any non-`/api` GET falls back to `index.html`.
- **Plugins:** none in M0. Plugins arrive in M7.

**Tech Stack:**
- Go 1.26, Echo v4, sqlx, goose, Postgres 17
- tygo (Go → TS types)
- bun, Vite, React, TypeScript strict
- TanStack Router (file-based), TanStack Query
- Tailwind v4, shadcn/ui (Radix), sonner, lucide-react
- Vitest + Testing Library, Playwright

**Spec:** `docs/specs/2026-09-27-karots-suite-design.md` (read §3–§6, §10, §11 M0).

## Global Constraints

- **Module and binary.** Go module `karots-suite`, binary `karots-suite`, repo `~/Projects/go/karots-suite`, branch `main`. Commit after every task.
- **Source of the fork.** Copy from `~/Projects/go/karots-pos` at `HEAD`, read-only. **Never modify karots-pos.**
- **Core is plugin-inert.** Core Go and `web/src` (outside `web/src/plugins`) never name a plugin.
- **Static and portable.** Release build uses `CGO_ENABLED=0 GOAMD64=v1 go build -ldflags="-s -w"`. The binary must be statically linked. bun and Node are needed at build time only.
- **API shape:**
  - all JSON lives under `/api/v1`
  - success is plain JSON (no `success`/`data` envelope)
  - errors are `{"error":{"code":"…","message":"…"}}`
  - JSON field names are camelCase
  - money is sent as decimal strings, never floats
- **Session cookie.** Named `pos_token` (the forked `middleware.CookieName`), `HttpOnly`, `SameSite=Strict`, with the Secure flag decided per request as in karots-pos. No token is ever readable from JS.
- **Hidden system user.** Phone `0000000001`, rotating hourly PIN, `POS_SYSTEM_PIN` override, `2273` dev fallback. All of this is forked unchanged.
- **Dev ports, separate from karots-pos.**
  - Postgres runs as container `suite_db` on host port **5433** (DB `suite_db`, user `suite_user`, password `suite_password`).
  - The Go server runs on **3100**, and Vite dev on **5173**, proxying `/api` to 3100.
- **File size.** Every source file stays at 500 lines or fewer, Go and TS alike.
- **Visual design.** The UI must not resemble karots-pos: no indigo/slate karots palette, no karots-pos layout. The design is decided in Task 10 with user approval.
- **TanStack skills.** Before the first frontend task, and after `bun install`, run `intent list` from `web/` and `intent load` the matching TanStack skills (router, query), per the global instructions.

## Review Focus

These are the conditions most likely to break for a real user. Each one is pinned by a test in the named task.

1. **An expired or invalid cookie on page load** should land on `/login`, with no blank screen and no redirect loop. Pinned by Task 9 (`redirectFor`) and Task 11 (Playwright).
2. **A locked session that reloads any page, or calls any protected API,** should end up on `/lock`, and unlock should resume the same user. Pinned by Task 4 (`SESSION_LOCKED`), Task 5 (`/me` works while locked) and Task 11 (Playwright).
3. **Unknown `/api/v1/...` paths** should return a JSON 404, never `index.html`. **A deep link like `/admin/whatever` on reload** should serve `index.html`. Pinned by Task 7.
4. **The Konami code** should work with Caps Lock on (`B`/`A`) and recover after a wrong key. **A cashier never has kiosk-exit armed**, and the server returns 404 for them. Pinned by Task 6 and Task 9.
5. **Every fresh DB built from the squashed baseline gets its own random 8-hex `install_id`.** If the dev DB's ID were frozen into the baseline, every shop would share one support PIN. Pinned by Task 3.

---

## File Structure (created in M0)

```
karots-suite/
  go.mod                       module karots-suite
  Makefile                     db-up, migrate, test, types, web, build, verify-portable, dev
  docker-compose.yml           suite_db on 5433
  .env.example  .gitignore  .air.toml  tygo.yaml  AGENTS.md
  cmd/server/                  forked (main.go, seed.go, demo.go, init.go, reset.go,
                               system_admin.go, support_pin.go, enabled_plugins.go→empty)
  internal/app/app.go          BuildServer: services + api.Register + spa (no web/static)
  internal/app/services.go     service construction + core seam wiring (moved, unmounted)
  internal/api/                NEW
    dto.go                     every request/response type (tygo source)
    api.go                     Deps, handlers struct, Register (route table)
    cookie.go                  setCookie / clearCookie / secureFlag
    auth.go                    login-info, login, logout, lock, unlock, change PIN
    me.go                      GET /me + appearanceOf
    kiosk.go                   POST /kiosk/exit (moved from karots-pos web/kiosk.go)
    system.go                  GET/PUT /system/appearance
    *_test.go
  internal/spa/                NEW: embed.go (//go:embed all:dist), spa.go (handler), spa_test.go
  internal/middleware/         forked; error.go rewritten JSON-only; auth.go guards → codes
  internal/plugin/             forked minus the 5 templ-typed hooks
  migrations/00001_baseline.sql  squashed (Task 3) + baseline_test.go
  scripts/squash-migrations.sh   one-shot squash + identity check
  scripts/parity.sh              karots-pos route list → docs/parity.md
  docs/parity.md
  web/                         Vite app (go.mod fence, package.json, vite.config.ts, src/…)
```

---

### Task 1: Fork the Go core and make it compile without the HTML layer

**Files:**
- Create (by copy): `go.mod`, `go.sum`, `cmd/server/*`, `internal/**`, `migrations/*`, `.gitattributes`
- Delete after copy:
  - `internal/web/`, `internal/demogen/`, `internal/demodb/`
  - `internal/response/templ.go`, `internal/response/htmx.go`, `internal/response/htmx_test.go`
- Modify:
  - `internal/app/app.go`
  - `internal/middleware/error.go` (rewrite)
  - `internal/plugin/hooks.go`, `internal/plugin/hooks_product.go`
  - `cmd/server/enabled_plugins.go`
- Create: `internal/app/services.go`, `internal/middleware/error_test.go`

**Interfaces:**
- Produces:
  - `middleware.ErrorHandler(isProd bool) echo.HTTPErrorHandler`. It always writes `{"error":{"code","message"}}`.
  - `app.Services{Auth *auth.Service; Settings *settings.Service; Sales *sales.Service; CashRegister *cashregister.Service; Cashflow *cashflow.Service}`
  - `app.NewServices(db *sqlx.DB, cfg *config.Config, systemPIN func(string, time.Time) bool) *Services`

- [ ] **Step 1: Copy the core out of karots-pos at HEAD**

```bash
cd ~/Projects/go/karots-pos && git archive HEAD go.mod go.sum cmd/server internal migrations .gitattributes | tar -x -C ~/Projects/go/karots-suite
cd ~/Projects/go/karots-suite
rm -rf internal/web internal/demogen internal/demodb
rm -f internal/response/templ.go internal/response/htmx.go internal/response/htmx_test.go
go mod edit -module karots-suite
grep -rl '"karots-pos/' --include='*.go' . | xargs sed -i 's#"karots-pos/#"karots-suite/#g'
```

- [ ] **Step 2: Empty the plugin import list.** Replace the whole of `cmd/server/enabled_plugins.go` with:

```go
package main

// Compile-time plugin set. Rewritten per shop by `make bootstrap` (M8); plugins
// are ported in M7. Empty on purpose: M0 ships core only.
```

- [ ] **Step 3: Remove the templ-typed plugin hooks.**
  - In `internal/plugin/hooks.go`, delete the types `SettingsSection`, `DashboardCard` and `PosAction`, together with their registries, their `Register…` and accessor functions, and any `Plugin`-interface or `Core` reference to them.
  - In `internal/plugin/hooks_product.go`, delete `ProductFormSection` and `IntakeFormSection` the same way.
  - Remove the `github.com/a-h/templ` import.
  - Their only consumers lived in the deleted `internal/web`. M7 re-adds them as JSON slot manifests.

Run: `grep -rn 'templ' internal/plugin` → Expected: no output.

- [ ] **Step 4: Write the failing error-handler test** in `internal/middleware/error_test.go`:

```go
package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"karots-suite/internal/apperr"

	"github.com/labstack/echo/v4"
)

func runErr(t *testing.T, err error, isProd bool) (int, map[string]map[string]string) {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/admin/x", nil), rec)
	ErrorHandler(isProd)(err, c)
	var body map[string]map[string]string
	if jerr := json.Unmarshal(rec.Body.Bytes(), &body); jerr != nil {
		t.Fatalf("body is not JSON: %q", rec.Body.String())
	}
	return rec.Code, body
}

func TestErrorHandlerAlwaysJSON(t *testing.T) {
	code, body := runErr(t, apperr.Unauthorized("nope"), true)
	if code != http.StatusUnauthorized || body["error"]["code"] == "" || body["error"]["message"] != "nope" {
		t.Fatalf("got %d %v", code, body)
	}
	if _, has := body["success"]; has {
		t.Fatal("legacy envelope key leaked")
	}
}

func TestErrorHandlerHidesInternalInProd(t *testing.T) {
	code, body := runErr(t, errors.New("pq: secret detail"), true)
	if code != 500 || body["error"]["message"] != "Something went wrong" {
		t.Fatalf("got %d %v", code, body)
	}
}

func TestErrorHandlerEchoHTTPError(t *testing.T) {
	code, body := runErr(t, echo.NewHTTPError(http.StatusNotFound, "Not Found"), false)
	if code != 404 || body["error"]["code"] != "HTTP_ERROR" {
		t.Fatalf("got %d %v", code, body)
	}
}
```

- [ ] **Step 5: Rewrite `internal/middleware/error.go` as JSON-only.** Keep `classify` exactly as forked, and delete `toastTrigger`, `wantsJSON` and the `shared` templates import.

```go
// Package-level: the single HTTP error handler. karots-suite has no server-rendered
// pages, so every error — API or not — is the JSON shape the SPA understands:
// {"error":{"code","message"}}. The SPA decides where to navigate (login/lock/pin)
// from the code; the server never redirects.
func ErrorHandler(isProd bool) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		status, code, msg := classify(err, isProd)
		_ = c.JSON(status, map[string]any{
			"error": map[string]string{"code": code, "message": msg},
		})
	}
}
```

- [ ] **Step 6: Move service construction out of `BuildServer`.** Create `internal/app/services.go` by moving the service and seam wiring from `BuildServer`: the drawer kicker, `cashflow`, `pluginTenderLeg`, the sales seams, the cash-register seams and `purchases.WithSettleCredit`. Build services with `NewService` instead of `RegisterAPI`, so no legacy route gets mounted.

```go
package app

// Services are the domain services the JSON API calls, with every core seam wired
// exactly as karots-pos wired them. Built once in BuildServer. Areas add a field
// here as they are ported (M1+); the legacy per-feature RegisterAPI routes are
// NOT mounted — each area's port deletes its legacy api.go.
type Services struct {
	Auth         *auth.Service
	Settings     *settings.Service
	Sales        *sales.Service
	CashRegister *cashregister.Service
	Cashflow     *cashflow.Service
}

func NewServices(db *sqlx.DB, cfg *config.Config, systemPIN func(pin string, now time.Time) bool) *Services {
	st := settings.NewService(db)
	kick := newDrawerKicker(st)
	cf := cashflow.NewService(db, sales.NewService(db))
	pluginTenderLeg := func(ctx context.Context, tx *sqlx.Tx, ev sales.PluginTender) error {
		for _, t := range plugin.TenderMethods() {
			if t.Value == ev.Method && t.SettleTx != nil {
				return t.SettleTx(ctx, tx, plugin.TenderSettle{
					SaleID: ev.SaleID, Amount: ev.Amount, Meta: ev.Meta, ActorID: ev.ActorID,
				})
			}
		}
		return nil
	}
	salesSvc := sales.NewService(db).WithDrawerKick(kick).WithNonCashLeg(cf.SaleNonCashLeg).WithPluginTenderLeg(pluginTenderLeg)
	cr := cashregister.NewService(db, sales.NewService(db)).WithAudit(audit.NewService(db))
	cr.WithLockerLeg(cf.TillLockerLeg)
	cr.WithDrawerKick(kick)
	purchases.WithSettleCredit(supplierpay.NewService(db).ApplySupplierCreditTx)

	a := auth.NewService(db, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	a.WithSystemPINValidator(systemPIN)
	return &Services{Auth: a, Settings: st, Sales: salesSvc, CashRegister: cr, Cashflow: cf}
}
```

Move `newDrawerKicker` into this file unchanged. `sales.PluginTender`, `plugin.TenderMethods` and `plugin.TenderSettle` are the forked names; confirm them with `grep` if the compiler complains.

- [ ] **Step 7: Trim `BuildServer`.** In `internal/app/app.go`:
  - Delete the `web` and `static` imports, the `/static/*` route, every `xxx.RegisterAPI(...)` call and the `web.RegisterUI` line.
  - Replace them with `svc := NewServices(sqlxDB, cfg, systemPIN)`, followed by `_ = svc` for now. Task 5 replaces `_ = svc` with `api.Register`.
  - Keep `/health`, `Migrate`, the validator, the error handler, Recover, Logger and CORS.

- [ ] **Step 8: Build, vet and test**

```bash
go mod tidy
go build ./... && go vet ./... && go test ./...
```
Expected: builds; `go.mod` no longer lists `github.com/a-h/templ`; all tests PASS or SKIP (DB tests skip without `DATABASE_URL`). If `cmd/server/demo.go` or `seed.go` fails on a removed import, delete only the offending reference, never domain code.

- [ ] **Step 9: Commit**

```bash
git add -A && git commit -m "feat: fork karots-pos Go core, drop HTML layer, JSON-only errors"
```

---

### Task 2: Dev environment with its own database, and forked DB tests passing

**Files:**
- Create: `docker-compose.yml`, `.env.example`, `.gitignore`, `Makefile`

- [ ] **Step 1: Write `docker-compose.yml`**

```yaml
services:
  postgres:
    image: postgres:17-alpine
    container_name: suite_db
    restart: unless-stopped
    environment:
      POSTGRES_DB: suite_db
      POSTGRES_USER: suite_user
      POSTGRES_PASSWORD: suite_password
    ports: ["5433:5432"]
    volumes: [suite_pg:/var/lib/postgresql/data]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U suite_user -d suite_db"]
      interval: 5s
      retries: 10
volumes:
  suite_pg:
```

- [ ] **Step 2: Write `.env.example`, then `cp .env.example .env`.** Also copy any extra keys that `internal/config/config.go` reads with `getEnv(`, using karots-pos's `.env.example` values.

```
APP_ENV=development
DATABASE_URL=postgres://suite_user:suite_password@localhost:5433/suite_db?sslmode=disable
JWT_SECRET=dev-only-change-me-32-bytes-minimum!!
SERVER_PORT=3100
CORS_ORIGINS=http://localhost:5173
POS_SYSTEM_PIN=2273
```

- [ ] **Step 3: Write `.gitignore`**

```
.env
bin/
dist/
tmp/
web/node_modules/
web/test-results/
web/playwright-report/
internal/spa/dist/*
!internal/spa/dist/.gitkeep
```

- [ ] **Step 4: Write the first `Makefile`.** Later tasks extend it. Recipes use tabs.

```make
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
```

- [ ] **Step 5: Run the whole forked suite against the new DB**

```bash
make migrate && make test
```
Expected: all PASS. If a test fails, run the same package in karots-pos:

```bash
cd ~/Projects/go/karots-pos && set -a && . ./.env && set +a && go test ./internal/features/<pkg>/
```

If it fails there too, it's pre-existing: list it in the commit message and do not fix it here. If it only fails in karots-suite, fix the fork, because something was lost in the copy.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "chore: dev env (suite_db on 5433), Makefile, forked tests green"
```

---

### Task 3: Squash the 75 core migrations into one baseline

karots-suite is a fresh product. Shops move over by **backup/restore**, and `internal/backup` skips every `goose_db_version*` table. So only the *final* schema and seed rows must match karots-pos, not the migration history.

**Files:**
- Create: `scripts/squash-migrations.sh`, `migrations/00001_baseline.sql`, `migrations/baseline_test.go`
- Delete: every other `migrations/*.sql`

**Interfaces:**
- Produces: `migrations.FS` (unchanged embed var name) containing only `00001_baseline.sql`. Later migrations continue from `00002_…`.

- [ ] **Step 1: Write the failing install-ID test** in `migrations/baseline_test.go`:

```go
package migrations_test

import (
	"os"
	"regexp"
	"testing"

	appdb "karots-suite/internal/db"
	"karots-suite/migrations"
)

// Each migrated database must get its OWN random install id. If the squash ever
// froze the dev DB's id into the baseline, every shop would share one support PIN.
func TestBaselineGeneratesFreshInstallID(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := appdb.RunMigrations(conn.DB, migrations.FS); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := conn.Get(&id, `SELECT install_id FROM settings LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9A-F]{8}$`).MatchString(id) {
		t.Fatalf("install_id %q is not 8 upper-hex", id)
	}
	frozen, _ := os.ReadFile("00001_baseline.sql")
	if regexp.MustCompile(`'`+id+`'`).Match(frozen) {
		t.Fatalf("install_id %s is hard-coded in the baseline", id)
	}
}
```

Run: `set -a && . ./.env && set +a && go test ./migrations/ -run Baseline -v` → Expected: FAIL with `open 00001_baseline.sql: no such file`, or PASS on the old migrations but for the wrong reason. Either way, continue.

- [ ] **Step 2: Write `scripts/squash-migrations.sh`**

```bash
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
migrate() { DATABASE_URL="$(url "$1")" JWT_SECRET=x go run ./cmd/server -migrate >/dev/null; }
normalize() {  # mask values that legitimately differ between two fresh DBs
  sed -E "s/[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9:.]+(\+[0-9:]+)?/TS/g"
}

if [ "${1:-}" != "--verify-only" ]; then
  fresh squash_orig && migrate squash_orig
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
diff <(DUMP squash_orig --schema-only) <(DUMP squash_base --schema-only) && echo "schema: IDENTICAL"
diff <(DUMP squash_orig --data-only --inserts | normalize) \
     <(DUMP squash_base --data-only --inserts | normalize) && echo "data: IDENTICAL"
```

- [ ] **Step 3: Run the squash**

```bash
chmod +x scripts/squash-migrations.sh && make db-up && scripts/squash-migrations.sh
```
Expected: the last two lines are `schema: IDENTICAL` and `data: IDENTICAL`, and `ls migrations/*.sql` shows only `00001_baseline.sql`. If the diff shows lines, fix the normalize or strip rules. Never hand-edit the schema. To re-run only the check, use `scripts/squash-migrations.sh --verify-only`.

- [ ] **Step 4: Reset the dev DB and run all tests**

```bash
docker compose exec -T postgres psql -U suite_user -d postgres -c "DROP DATABASE suite_db WITH (FORCE)" -c "CREATE DATABASE suite_db"
make migrate && make test
```
Expected: `TestBaselineGeneratesFreshInstallID` PASSES, and the whole suite PASSES.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(migrations): squash 75 core migrations into 00001_baseline (identity-checked)"
```

---

### Task 4: Guards return JSON codes instead of redirects

**Files:**
- Modify: `internal/middleware/auth.go` (`RequirePinChosen`, `RequireUnlocked`)
- Test: `internal/middleware/guards_test.go`

**Interfaces:**
- Produces two error codes the SPA routes on:
  - `SESSION_LOCKED` (403)
  - `PIN_CHANGE_REQUIRED` (403)
- `UNAUTHORIZED` (401) comes from forked `apperr.Unauthorized`. Check its `Code` in `internal/apperr`, and use that exact string in Task 9.

- [ ] **Step 1: Write the failing test**

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"karots-suite/internal/apperr"

	"github.com/labstack/echo/v4"
)

func guardErr(t *testing.T, mw echo.MiddlewareFunc, set func(echo.Context)) *apperr.AppError {
	t.Helper()
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/api/v1/anything", nil), httptest.NewRecorder())
	set(c)
	err := mw(func(echo.Context) error { return nil })(c)
	if err == nil {
		return nil
	}
	ae, ok := apperr.As(err)
	if !ok {
		t.Fatalf("not an AppError: %v", err)
	}
	return ae
}

func TestRequireUnlockedCode(t *testing.T) {
	ae := guardErr(t, RequireUnlocked(), func(c echo.Context) { c.Set(ctxLocked, true) })
	if ae == nil || ae.Status != 403 || ae.Code != "SESSION_LOCKED" {
		t.Fatalf("got %+v", ae)
	}
	if guardErr(t, RequireUnlocked(), func(echo.Context) {}) != nil {
		t.Fatal("unlocked session was blocked")
	}
}

func TestRequirePinChosenCode(t *testing.T) {
	ae := guardErr(t, RequirePinChosen(), func(c echo.Context) { c.Set(ctxMustChangePin, true) })
	if ae == nil || ae.Status != 403 || ae.Code != "PIN_CHANGE_REQUIRED" {
		t.Fatalf("got %+v", ae)
	}
}
```

`ctxLocked` and `ctxMustChangePin` are the forked context keys used by `IsLocked` and `MustChangePin`. Use the exact identifiers from `auth.go`.

- [ ] **Step 2: Run it and watch it fail**

Run: `go test ./internal/middleware/ -run 'RequireUnlocked|RequirePinChosen' -v` → Expected: FAIL. The current code redirects (303) and returns a nil error for GET.

- [ ] **Step 3: Replace both guards.** Route scoping in Task 5 keeps `/me` and `/auth/*` unguarded, so no path whitelist is needed.

```go
// RequirePinChosen blocks a user still carrying a server-assigned PIN. The SPA
// routes PIN_CHANGE_REQUIRED to its change-PIN screen. Must run after JWTAuth.
func RequirePinChosen() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if MustChangePin(c) {
				return &apperr.AppError{Status: http.StatusForbidden, Code: "PIN_CHANGE_REQUIRED", Message: "Choose a new PIN to continue"}
			}
			return next(c)
		}
	}
}

// RequireUnlocked blocks a screen-locked session (identity + open till intact).
// The SPA routes SESSION_LOCKED to its lock screen. Must run after JWTAuth.
func RequireUnlocked() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if IsLocked(c) {
				return &apperr.AppError{Status: http.StatusForbidden, Code: "SESSION_LOCKED", Message: "Session locked"}
			}
			return next(c)
		}
	}
}
```

- [ ] **Step 4: Run the middleware tests**

Run: `go test ./internal/middleware/ -v` → Expected: all PASS. Update any forked test that asserted the old 303 redirect so it asserts the new code.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(middleware): lock/PIN guards return JSON codes, no redirects"
```

---

### Task 5: Auth and session API (`/api/v1/auth/*`, `/api/v1/me`)

**Files:**
- Create: `internal/api/dto.go`, `api.go`, `cookie.go`, `auth.go`, `me.go`, `testutil_test.go`, `auth_test.go`
- Modify:
  - `internal/app/app.go` (call `api.Register`)
  - `internal/features/auth/routes.go`: delete `RegisterAPI`, keep `NewLoginLimiter`
  - delete `internal/features/auth/api.go`

**Interfaces:**
- Consumes: `app.Services` (Task 1) and the guard codes (Task 4).
- Produces:
  - `api.Deps{DB *sqlx.DB; Cfg *config.Config; Auth *auth.Service; Settings *settings.Service; CashRegister *cashregister.Service}`
  - `api.Register(e *echo.Echo, d Deps)`
  - `handlers.protected []echo.MiddlewareFunc` (jwt, lock, pin), which every later area group uses
  - DTOs `SessionUser`, `LoginRequest`, `LoginResponse`, `LoginInfo`, `ChangePinRequest`, `Flags`, `Appearance`, `Me`
- Routes:

| Method | Path | Guard | Result |
|---|---|---|---|
| GET | `/api/v1/auth/login-info` | public | `LoginInfo` |
| POST | `/api/v1/auth/login` | limiter | sets cookie, `LoginResponse` |
| POST | `/api/v1/auth/logout` | none | 204 + clears cookie; `409 TILL_OPEN` while own till is open |
| POST | `/api/v1/auth/lock` | jwt | 204, re-issued locked cookie |
| POST | `/api/v1/auth/unlock` | jwt + limiter | 204 (self or any admin) |
| POST | `/api/v1/auth/pin` | jwt + lock | 204, fresh cookie |
| GET | `/api/v1/me` | jwt only (works while locked or forced PIN) | `Me` |
| ANY | `/api/*` | — | JSON 404 `NOT_FOUND` |

- [ ] **Step 1: Write the DTOs** in `internal/api/dto.go`. This file is the tygo source, so keep only JSON types here.

```go
package api

// SessionUser is who is signed in.
type SessionUser struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"` // "admin" | "manager" | "cashier"
}

type LoginRequest struct {
	Phone string `json:"phone"`
	Pin   string `json:"pin"`
}

type LoginResponse struct {
	User SessionUser `json:"user"`
}

// LoginInfo feeds the login page's hidden Konami reveal + console line.
type LoginInfo struct {
	InstallID  string `json:"installId"`
	ServerTime string `json:"serverTime"` // RFC3339 UTC
}

type ChangePinRequest struct {
	CurrentPin string `json:"currentPin"`
	NewPin     string `json:"newPin"`
	ConfirmPin string `json:"confirmPin"`
}

type Flags struct {
	CanHandleSuppliers bool `json:"canHandleSuppliers"`
	CanManageCredit    bool `json:"canManageCredit"`
	CanSeeCost         bool `json:"canSeeCost"`
	CanExitKiosk       bool `json:"canExitKiosk"`
	IsSystem           bool `json:"isSystem"`
}

// Appearance is the system-user-locked look. BrandColor "" = theme default.
type Appearance struct {
	BrandColor   string `json:"brandColor"`   // "#rrggbb" or ""
	Radius       string `json:"radius"`       // "sharp" | "rounded" | "round"
	Density      string `json:"density"`      // "comfortable" | "compact"
	ReceiptStyle string `json:"receiptStyle"` // "classic" | "modern" | "boxed" | "compact"
}

type Me struct {
	User               SessionUser `json:"user"`
	Flags              Flags       `json:"flags"`
	Locked             bool        `json:"locked"`
	MustChangePin      bool        `json:"mustChangePin"`
	ShopName           string      `json:"shopName"`
	LockTimeoutMinutes int         `json:"lockTimeoutMinutes"`
	Appearance         Appearance  `json:"appearance"`
	Plugins            []string    `json:"plugins"`
}
```

- [ ] **Step 2: Write the DB-gated test helper and the failing tests.** `internal/api/testutil_test.go`:

```go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"karots-suite/internal/config"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"
	"karots-suite/internal/validator"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

type testEnv struct {
	e  *echo.Echo
	db *sqlx.DB
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	cfg := &config.Config{JWTSecret: "test-secret-test-secret-test-secret", JWTAccessTTL: time.Hour, JWTRefreshTTL: time.Hour}
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = middleware.ErrorHandler(false)
	a := auth.NewService(db, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	Register(e, Deps{DB: db, Cfg: cfg, Auth: a, Settings: settings.NewService(db),
		CashRegister: cashregister.NewService(db, sales.NewService(db))})
	return &testEnv{e: e, db: db}
}

// seedUser creates an active user with PIN 1234 and removes it after the test.
func (te *testEnv) seedUser(t *testing.T, phone, role string) int64 {
	t.Helper()
	ctx := context.Background()
	te.db.ExecContext(ctx, `DELETE FROM users WHERE phone=$1`, phone) //nolint:errcheck
	u, err := auth.NewService(te.db, "x", time.Hour, time.Hour).CreateUser(ctx,
		auth.CreateUserInput{Name: "T " + role, Phone: phone, Role: role, PIN: "1234"})
	if err != nil {
		t.Fatal(err)
	}
	te.db.ExecContext(ctx, `UPDATE users SET must_change_pin=false WHERE id=$1`, u.ID) //nolint:errcheck
	t.Cleanup(func() { te.db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, u.ID) }) //nolint:errcheck
	return u.ID
}

// do sends a JSON request, carrying cookie (may be nil); returns recorder.
func (te *testEnv) do(method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body) //nolint:errcheck
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	te.e.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == middleware.CookieName && c.Value != "" {
			return c
		}
	}
	t.Fatalf("no session cookie; status %d body %s", rec.Code, rec.Body.String())
	return nil
}

func errCode(rec *httptest.ResponseRecorder) string {
	var b struct{ Error struct{ Code string } }
	json.Unmarshal(rec.Body.Bytes(), &b) //nolint:errcheck
	return b.Error.Code
}
```

If `CreateUser` refuses `must_change_pin=false` via its input, the `UPDATE` above sets it.

`internal/api/auth_test.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"
)

func TestLoginSetsStrictHttpOnlyCookieAndMeWorks(t *testing.T) {
	te := newTestEnv(t)
	te.seedUser(t, "0770009001", "cashier")
	rec := te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: "0770009001", Pin: "1234"}, nil)
	if rec.Code != 200 {
		t.Fatalf("login %d %s", rec.Code, rec.Body)
	}
	ck := sessionCookie(t, rec)
	if !ck.HttpOnly || ck.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie not HttpOnly+Strict: %+v", ck)
	}
	rec = te.do(http.MethodGet, "/api/v1/me", nil, ck)
	var me Me
	json.Unmarshal(rec.Body.Bytes(), &me) //nolint:errcheck
	if rec.Code != 200 || me.User.Role != "cashier" || me.Flags.CanExitKiosk || me.Plugins == nil {
		t.Fatalf("me %d %+v", rec.Code, me)
	}
}

func TestLoginWrongPin(t *testing.T) {
	te := newTestEnv(t)
	te.seedUser(t, "0770009002", "cashier")
	rec := te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: "0770009002", Pin: "9999"}, nil)
	if rec.Code < 400 || rec.Code >= 500 || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("wrong pin gave %d cookies=%v", rec.Code, rec.Result().Cookies())
	}
}

func TestMeWithoutCookieIs401(t *testing.T) {
	te := newTestEnv(t)
	if rec := te.do(http.MethodGet, "/api/v1/me", nil, nil); rec.Code != 401 {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestLockThenMeStillWorksThenUnlock(t *testing.T) {
	te := newTestEnv(t)
	te.seedUser(t, "0770009003", "cashier")
	ck := sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: "0770009003", Pin: "1234"}, nil))
	locked := sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/lock", nil, ck))
	rec := te.do(http.MethodGet, "/api/v1/me", nil, locked)
	var me Me
	json.Unmarshal(rec.Body.Bytes(), &me) //nolint:errcheck
	if rec.Code != 200 || !me.Locked {
		t.Fatalf("locked me %d %+v", rec.Code, me)
	}
	if rec := te.do(http.MethodPost, "/api/v1/auth/pin", ChangePinRequest{"1234", "5678", "5678"}, locked); errCode(rec) != "SESSION_LOCKED" {
		t.Fatalf("locked pin change: %d %s", rec.Code, rec.Body)
	}
	un := sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/unlock", LoginRequest{Phone: "0770009003", Pin: "1234"}, locked))
	json.Unmarshal(te.do(http.MethodGet, "/api/v1/me", nil, un).Body.Bytes(), &me) //nolint:errcheck
	if me.Locked {
		t.Fatal("still locked after unlock")
	}
}

func TestLoginInfoPublic(t *testing.T) {
	te := newTestEnv(t)
	rec := te.do(http.MethodGet, "/api/v1/auth/login-info", nil, nil)
	var li LoginInfo
	json.Unmarshal(rec.Body.Bytes(), &li) //nolint:errcheck
	if rec.Code != 200 || !regexp.MustCompile(`^[0-9A-F]{8}$`).MatchString(li.InstallID) || li.ServerTime == "" {
		t.Fatalf("%d %+v", rec.Code, li)
	}
}

func TestUnknownAPIPathIsJSON404(t *testing.T) {
	te := newTestEnv(t)
	rec := te.do(http.MethodGet, "/api/v1/nope", nil, nil)
	if rec.Code != 404 || errCode(rec) == "" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	te := newTestEnv(t)
	te.seedUser(t, "0770009004", "admin")
	ck := sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: "0770009004", Pin: "1234"}, nil))
	rec := te.do(http.MethodPost, "/api/v1/auth/logout", nil, ck)
	if rec.Code != 204 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == ck.Name && c.MaxAge >= 0 {
			t.Fatal("cookie not cleared")
		}
	}
}
```

Run: `set -a && . ./.env && set +a && go test ./internal/api/ -v` → Expected: FAIL to compile (`Register` and `Deps` undefined).

- [ ] **Step 3: Write `internal/api/cookie.go`.** Copy `secureFlag`, `setCookie` and `clearCookie` from karots-pos `internal/web/auth.go`, onto a `*handlers` receiver that reads `h.d.Cfg.CookieSecure` and `h.d.Cfg.JWTAccessTTL`. **Change `SameSite` to `http.SameSiteStrictMode` in both.**

- [ ] **Step 4: Write `internal/api/api.go`**

```go
// Package api is karots-suite's JSON HTTP layer (/api/v1). Handlers are thin:
// bind → call a domain service → return a DTO from dto.go. One file per area;
// every protected area group uses h.protected.
package api

import (
	"karots-suite/internal/apperr"
	"karots-suite/internal/config"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

type Deps struct {
	DB           *sqlx.DB
	Cfg          *config.Config
	Auth         *auth.Service
	Settings     *settings.Service
	CashRegister *cashregister.Service
}

type handlers struct {
	d         Deps
	protected []echo.MiddlewareFunc // jwt → unlocked → PIN chosen
}

func Register(e *echo.Echo, d Deps) {
	jwt := middleware.JWTAuth(d.Cfg.JWTSecret)
	limiter := auth.NewLoginLimiter()
	h := &handlers{d: d, protected: []echo.MiddlewareFunc{jwt, middleware.RequireUnlocked(), middleware.RequirePinChosen()}}

	v1 := e.Group("/api/v1")
	v1.GET("/auth/login-info", h.LoginInfo)
	v1.POST("/auth/login", h.Login, limiter)
	v1.POST("/auth/logout", h.Logout)
	v1.POST("/auth/lock", h.Lock, jwt)
	v1.POST("/auth/unlock", h.Unlock, jwt, limiter)
	v1.POST("/auth/pin", h.ChangePin, jwt, middleware.RequireUnlocked())
	v1.GET("/me", h.Me, jwt)

	h.registerKiosk(v1, jwt)  // Task 6
	h.registerSystem(v1)      // Task 6

	e.Any("/api/*", func(echo.Context) error { return apperr.NotFound("endpoint") })
}
```

In this task, add empty `func (h *handlers) registerKiosk(*echo.Group, echo.MiddlewareFunc) {}` and `func (h *handlers) registerSystem(*echo.Group) {}` stubs at the bottom of `api.go`. Task 6 moves them into `kiosk.go` and `system.go`.

- [ ] **Step 5: Write `internal/api/auth.go`.** Port the logic from karots-pos `internal/web/auth.go` (`Login`, `Logout`, `Lock`, `Unlock`, `ChangePIN`, `pinChangeBlocked`) with JSON in and out:

```go
package api

import (
	"net/http"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

func (h *handlers) LoginInfo(c echo.Context) error {
	info := LoginInfo{ServerTime: time.Now().UTC().Format(time.RFC3339)}
	if cfg, err := h.d.Settings.Get(c.Request().Context()); err == nil && cfg != nil {
		info.InstallID = cfg.InstallID
	}
	return c.JSON(http.StatusOK, info)
}

func (h *handlers) bindLogin(c echo.Context) (auth.LoginInput, error) {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return auth.LoginInput{}, apperr.BadRequest("invalid request body")
	}
	in := auth.LoginInput{Phone: req.Phone, PIN: req.Pin}
	if err := c.Validate(&in); err != nil {
		return in, apperr.Validation("Enter a valid phone number and 4–6 digit PIN")
	}
	return in, nil
}

func (h *handlers) Login(c echo.Context) error {
	in, err := h.bindLogin(c)
	if err != nil {
		return err
	}
	pair, err := h.d.Auth.Login(c.Request().Context(), in)
	if err != nil {
		return err
	}
	h.setCookie(c, pair.AccessToken)
	return c.JSON(http.StatusOK, LoginResponse{User: SessionUser{ID: pair.User.ID, Name: pair.User.Name, Role: pair.User.Role}})
}

// Logout refuses while the caller has an open till (or a plugin guard blocks),
// so the cash trail stays honest. Unreadable token → clean logout, never trapped.
func (h *handlers) Logout(c echo.Context) error {
	claims, ok := middleware.ParseClaims(c, h.d.Cfg.JWTSecret)
	if ok && claims.UserID != 0 {
		ctx := c.Request().Context()
		for _, guard := range plugin.LogoutGuards() {
			if block, _, _ := guard(ctx, claims.UserID); block {
				return apperr.ConflictCode("LOGOUT_BLOCKED", "Finish your open work before signing out")
			}
		}
		if sess, err := h.d.CashRegister.Current(ctx, claims.UserID); err == nil && sess != nil {
			return apperr.ConflictCode("TILL_OPEN", "Close and count your till before signing out")
		}
	}
	h.clearCookie(c)
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) Lock(c echo.Context) error {
	claims, ok := middleware.ParseClaims(c, h.d.Cfg.JWTSecret)
	if !ok || claims.UserID == 0 {
		return apperr.Unauthorized("")
	}
	token, err := h.d.Auth.ReissueLocked(claims, true)
	if err != nil {
		return err
	}
	h.setCookie(c, token)
	return c.NoContent(http.StatusNoContent)
}

// Unlock: the locked-in user OR any admin may unlock; identity is unchanged.
func (h *handlers) Unlock(c echo.Context) error {
	claims, ok := middleware.ParseClaims(c, h.d.Cfg.JWTSecret)
	if !ok || claims.UserID == 0 {
		return apperr.Unauthorized("")
	}
	in, err := h.bindLogin(c)
	if err != nil {
		return err
	}
	u, err := h.d.Auth.VerifyCredentials(c.Request().Context(), in)
	if err != nil {
		return apperr.Validation("Wrong phone number or PIN")
	}
	if u.ID != claims.UserID && u.Role != auth.RoleAdmin {
		return apperr.Forbidden("Only " + claims.Name + " or an admin can unlock this terminal")
	}
	token, err := h.d.Auth.ReissueLocked(claims, false)
	if err != nil {
		return err
	}
	h.setCookie(c, token)
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) ChangePin(c echo.Context) error {
	ctx := c.Request().Context()
	if middleware.CurrentRole(c) == auth.RoleCashier && !middleware.MustChangePin(c) && !h.d.Auth.AllowCashierPinChange(ctx) {
		return apperr.Forbidden("PIN changes are turned off for cashiers")
	}
	var req ChangePinRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := auth.ChangeOwnPINInput{CurrentPIN: req.CurrentPin, NewPIN: req.NewPin, ConfirmPIN: req.ConfirmPin}
	if err := c.Validate(&in); err != nil {
		return apperr.Validation("PINs must be 4–6 digits")
	}
	u, err := h.d.Auth.ChangeOwnPIN(ctx, middleware.CurrentUserID(c), in)
	if err != nil {
		return err
	}
	token, err := h.d.Auth.AccessTokenFor(u)
	if err != nil {
		return err
	}
	h.setCookie(c, token)
	return c.NoContent(http.StatusNoContent)
}
```

Check what `plugin.LogoutGuard` really returns in `internal/plugin/hooks.go`. If it isn't `(bool, string, error)`, adjust the blank identifiers.

- [ ] **Step 6: Write `internal/api/me.go`**

```go
package api

import (
	"net/http"

	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

// appearanceOf maps the forked settings columns to the API shape. The new
// product has one theme, so the old "skin" only signals whether a brand colour
// is set ("custom").
func appearanceOf(s *settings.Settings) Appearance {
	a := Appearance{Radius: s.SkinRadius, Density: s.Density, ReceiptStyle: s.ReceiptStyle}
	if s.Skin == "custom" {
		a.BrandColor = s.SkinCustom
	}
	return a
}

// Me is the SPA's boot call. JWT only — it must answer while locked or forced to
// change PIN, so the SPA can route to /lock or /account/pin.
func (h *handlers) Me(c echo.Context) error {
	ctx := c.Request().Context()
	me := Me{
		User: SessionUser{ID: middleware.CurrentUserID(c), Name: middleware.CurrentUserName(c), Role: middleware.CurrentRole(c)},
		Flags: Flags{
			CanHandleSuppliers: middleware.CanHandleSuppliers(c),
			CanManageCredit:    middleware.CanManageCredit(c),
			CanSeeCost:         middleware.CanSeeCost(c),
			CanExitKiosk:       middleware.CanExitKioskCtx(ctx),
			IsSystem:           middleware.IsSystemUserCtx(ctx),
		},
		Locked:        middleware.IsLocked(c),
		MustChangePin: middleware.MustChangePin(c),
		Plugins:       []string{},
	}
	for _, p := range plugin.All() {
		me.Plugins = append(me.Plugins, p.Name())
	}
	st, err := h.d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	me.ShopName, me.LockTimeoutMinutes, me.Appearance = st.ShopName, st.LockTimeoutMinutes, appearanceOf(st)
	return c.JSON(http.StatusOK, me)
}
```

- [ ] **Step 7: Install the user validator and wire `Register`.**
  - Move the `middleware.SetUserValidator(...)` block from karots-pos `internal/web/web.go` (`RegisterUI`) into `internal/app/app.go` `BuildServer`, verbatim, before routes. Without it, the `CanExitKiosk` and `IsSystem` flags are never set.
  - In `internal/api/testutil_test.go` `newTestEnv`, call `app`-independent code instead: copy the same `SetUserValidator` call before `Register`, since `api` can't import `app`.
  - In `BuildServer`, replace `_ = svc` with:

```go
api.Register(e, api.Deps{DB: sqlxDB, Cfg: cfg, Auth: svc.Auth, Settings: svc.Settings, CashRegister: svc.CashRegister})
```

Delete `internal/features/auth/api.go` and the `RegisterAPI` function in `internal/features/auth/routes.go` (keep `NewLoginLimiter`).

- [ ] **Step 8: Run the tests**

Run: `make test` → Expected: every `internal/api` test PASSES, and nothing else regresses.

- [ ] **Step 9: Commit**

```bash
git add -A && git commit -m "feat(api): /api/v1 auth + me with Strict httpOnly cookie session"
```

---

### Task 6: Kiosk exit and system Appearance API

**Files:**
- Create: `internal/api/kiosk.go`, `internal/api/system.go`, `internal/api/system_test.go`
- Modify: `internal/api/api.go` (delete the two stubs)

**Interfaces:**
- Routes:
  - `POST /api/v1/kiosk/exit` (jwt + `RequireKioskExit`, so 404 for cashiers)
  - `GET /api/v1/system/appearance` → `Appearance`
  - `PUT /api/v1/system/appearance` with body `Appearance` → the saved `Appearance`. It runs through `h.protected` + `RequireSystemUser`, so it's 404 for everyone else.

- [ ] **Step 1: Write the failing tests** in `internal/api/system_test.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func login(t *testing.T, te *testEnv, phone string) *http.Cookie {
	t.Helper()
	return sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: phone, Pin: "1234"}, nil))
}

func TestKioskExitHiddenFromCashier(t *testing.T) {
	te := newTestEnv(t)
	te.seedUser(t, "0770009011", "cashier")
	if rec := te.do(http.MethodPost, "/api/v1/kiosk/exit", nil, login(t, te, "0770009011")); rec.Code != 404 {
		t.Fatalf("cashier kiosk exit: %d", rec.Code)
	}
}

func TestAppearanceSystemOnly(t *testing.T) {
	te := newTestEnv(t)
	te.seedUser(t, "0770009012", "admin")
	if rec := te.do(http.MethodGet, "/api/v1/system/appearance", nil, login(t, te, "0770009012")); rec.Code != 404 {
		t.Fatalf("admin saw system appearance: %d", rec.Code)
	}
}

func TestAppearanceRoundTripAndSanitises(t *testing.T) {
	te := newTestEnv(t)
	id := te.seedUser(t, "0770009013", "admin")
	ck := login(t, te, "0770009013") // log in BEFORE flagging: system users' PINs go through the rotating validator
	te.db.Exec(`UPDATE users SET is_system=true WHERE id=$1`, id) //nolint:errcheck
	var before Appearance
	json.Unmarshal(te.do(http.MethodGet, "/api/v1/system/appearance", nil, ck).Body.Bytes(), &before) //nolint:errcheck
	t.Cleanup(func() { te.do(http.MethodPut, "/api/v1/system/appearance", before, ck) })

	rec := te.do(http.MethodPut, "/api/v1/system/appearance",
		Appearance{BrandColor: "#0f766e", Radius: "round", Density: "compact", ReceiptStyle: "boxed"}, ck)
	var got Appearance
	json.Unmarshal(rec.Body.Bytes(), &got) //nolint:errcheck
	if rec.Code != 200 || got.BrandColor != "#0f766e" || got.Radius != "round" || got.Density != "compact" || got.ReceiptStyle != "boxed" {
		t.Fatalf("%d %+v", rec.Code, got)
	}
	rec = te.do(http.MethodPut, "/api/v1/system/appearance",
		Appearance{BrandColor: "red;}", Radius: "blob", Density: "x", ReceiptStyle: "y"}, ck)
	json.Unmarshal(rec.Body.Bytes(), &got) //nolint:errcheck
	if got.BrandColor != "" || got.Radius != "rounded" || got.Density != "comfortable" || got.ReceiptStyle != "classic" {
		t.Fatalf("junk not sanitised: %+v", got)
	}
}
```

The system flag comes from `is_system`, which the user validator reads on every request (Task 5 Step 7). That's why the test sets it after login.

Run: `go test ./internal/api/ -run 'Kiosk|Appearance' -v` → Expected: FAIL (404s from the stubs, or compile errors).

- [ ] **Step 2: Write `internal/api/kiosk.go`.** Copy `kioskSentinel` and `killKiosk` verbatim from karots-pos `internal/web/kiosk.go`, then add:

```go
func (h *handlers) registerKiosk(v1 *echo.Group, jwt echo.MiddlewareFunc) {
	v1.POST("/kiosk/exit", h.KioskExit, jwt, middleware.RequireKioskExit())
}

// KioskExit: write the relaunch loop's exit sentinel, then kill the --kiosk
// browser. Admin/support only (RequireKioskExit 404s everyone else).
func (h *handlers) KioskExit(c echo.Context) error {
	if err := os.WriteFile(kioskSentinel(), []byte("1"), 0o644); err != nil {
		return err
	}
	killKiosk()
	return c.NoContent(http.StatusNoContent)
}
```

- [ ] **Step 3: Write `internal/api/system.go`**

```go
package api

import (
	"net/http"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

func (h *handlers) registerSystem(v1 *echo.Group) {
	g := v1.Group("/system", append(h.protected, middleware.RequireSystemUser())...)
	g.GET("/appearance", h.GetAppearance)
	g.PUT("/appearance", h.PutAppearance)
}

func (h *handlers) GetAppearance(c echo.Context) error {
	st, err := h.d.Settings.Get(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, appearanceOf(st))
}

// PutAppearance stores the locked look. Unknown values fall back to defaults in
// the forked settings validators; a valid brand colour switches skin to "custom".
func (h *handlers) PutAppearance(c echo.Context) error {
	var in Appearance
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	skin := "default"
	if settings.ValidHexColor(in.BrandColor) != "" {
		skin = "custom"
	}
	st, err := h.d.Settings.SetAppearance(c.Request().Context(), skin, in.Density, in.ReceiptStyle, in.BrandColor, in.Radius)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, appearanceOf(st))
}
```

Confirm that `settings.ValidHexColor("red;}")` returns `""`. If it returns a default colour instead, compare the result against the input.

- [ ] **Step 4: Run the tests**

Run: `make test` → Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(api): kiosk exit + system-only appearance endpoints"
```

---

### Task 7: Embed the SPA with index.html fallback

**Files:**
- Create: `internal/spa/embed.go`, `internal/spa/spa.go`, `internal/spa/spa_test.go`, `internal/spa/dist/.gitkeep`
- Modify: `internal/app/app.go`

**Interfaces:**
- Produces: `spa.Handler(fsys fs.FS) echo.HandlerFunc`, mounted as `e.GET("/*", spa.Handler(spa.Dist()))` after `api.Register`.

- [ ] **Step 1: Write the failing test** in `internal/spa/spa_test.go`:

```go
package spa

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/labstack/echo/v4"
)

func serve(t *testing.T, fsys fstest.MapFS, path string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.GET("/*", Handler(fsys))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

var site = fstest.MapFS{
	"index.html":        {Data: []byte("<html>app</html>")},
	"assets/app-abc.js": {Data: []byte("js")},
}

func TestDeepLinkServesIndexNoCache(t *testing.T) {
	rec := serve(t, site, "/admin/reports/sales?from=2026-01-01")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "app") || rec.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("%d %q %q", rec.Code, rec.Body, rec.Header().Get("Cache-Control"))
	}
}

func TestHashedAssetImmutable(t *testing.T) {
	rec := serve(t, site, "/assets/app-abc.js")
	if rec.Code != 200 || rec.Body.String() != "js" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}

func TestMissingAssetIs404NotIndex(t *testing.T) {
	if rec := serve(t, site, "/assets/gone.js"); rec.Code != 404 {
		t.Fatalf("%d", rec.Code)
	}
}

func TestNotBuiltGives503(t *testing.T) {
	if rec := serve(t, fstest.MapFS{}, "/"); rec.Code != 503 {
		t.Fatalf("%d", rec.Code)
	}
}
```

Run: `go test ./internal/spa/` → Expected: FAIL (`Handler` undefined).

- [ ] **Step 2: Write `internal/spa/embed.go`** and create the empty `internal/spa/dist/.gitkeep`:

```go
// Package spa serves the built React app (web/ → internal/spa/dist) from the
// binary. `make web` fills dist; an unbuilt tree still compiles (.gitkeep).
package spa

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

func Dist() fs.FS {
	sub, _ := fs.Sub(dist, "dist")
	return sub
}
```

- [ ] **Step 3: Write `internal/spa/spa.go`**

```go
package spa

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
)

// Handler serves real files; any other path that is not an asset gets
// index.html so client-side routes survive a reload. /api/* never reaches here
// (Echo matches the more specific /api/* route first).
func Handler(fsys fs.FS) echo.HandlerFunc {
	files := http.FileServer(http.FS(fsys))
	return func(c echo.Context) error {
		p := strings.TrimPrefix(path.Clean(c.Request().URL.Path), "/")
		if p != "" && p != "." {
			if st, err := fs.Stat(fsys, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(c.Response(), c.Request())
				return nil
			}
			if strings.HasPrefix(p, "assets/") || path.Ext(p) != "" {
				return echo.NewHTTPError(http.StatusNotFound, "Not Found")
			}
		}
		index, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "frontend not built — run `make web`")
		}
		c.Response().Header().Set("Cache-Control", "no-cache")
		return c.HTMLBlob(http.StatusOK, index)
	}
}
```

- [ ] **Step 4: Mount it.** In `BuildServer`, after `api.Register(...)`, add `e.GET("/*", spa.Handler(spa.Dist()))`.

Run: `go build ./... && go test ./internal/spa/ ./internal/api/` → Expected: PASS. `TestUnknownAPIPathIsJSON404` must still pass.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(spa): embed built frontend with index.html fallback + cache headers"
```

---

### Task 8: Frontend scaffold, types pipeline and single-binary build

**Files:**
- Create:
  - `web/` (Vite React TS app), `web/go.mod`
  - `web/vite.config.ts`, `web/src/main.tsx`, `web/src/routes/__root.tsx`, `web/src/routes/index.tsx`
  - `web/src/app.css`, `web/src/lib/utils.ts` (from shadcn)
  - `tygo.yaml`, `.air.toml`
- Modify: `Makefile`

**Interfaces:**
- Produces:
  - `web/src/api/types.gen.ts`, generated from `internal/api/dto.go` and never hand-edited
  - `@/` path alias → `web/src/`
  - `make types`, `make web`, `make build`, `make verify-portable`, `make dev`

- [ ] **Step 1: Scaffold with Vite**

```bash
cd ~/Projects/go/karots-suite
bun create vite web --template react-ts
cd web && bun install
printf 'module karots-suite/web-fence\n\n// Empty module: stops `go ./...` from walking web/node_modules.\ngo 1.26\n' > go.mod
bun add @tanstack/react-router @tanstack/react-query sonner lucide-react
bun add -d @tanstack/router-plugin @tanstack/react-router-devtools @tanstack/react-query-devtools \
  tailwindcss @tailwindcss/vite vitest jsdom @testing-library/react @testing-library/jest-dom \
  @testing-library/user-event @playwright/test @types/node
intent list
```

`intent load` the listed skills for `@tanstack/react-router` and `@tanstack/react-query`, and follow them for Steps 2–4. Then:

```bash
cd .. && go vet ./...
```
Expected: no error from `web/node_modules` (the fence works).

- [ ] **Step 2: Write `web/vite.config.ts`**

```ts
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import path from 'node:path'

export default defineConfig({
  plugins: [tanstackRouter({ target: 'react', autoCodeSplitting: true }), react(), tailwindcss()],
  resolve: { alias: { '@': path.resolve(__dirname, 'src') } },
  build: { outDir: '../internal/spa/dist', emptyOutDir: true },
  server: { port: 5173, proxy: { '/api': 'http://localhost:3100' } },
  test: { environment: 'jsdom', setupFiles: ['./src/test-setup.ts'], include: ['src/**/*.test.{ts,tsx}'] },
})
```

Add `"paths": { "@/*": ["./src/*"] }` and `"baseUrl": "."` to `web/tsconfig.app.json` `compilerOptions`. Create `web/src/test-setup.ts` containing `import '@testing-library/jest-dom/vitest'`. Add these scripts to `web/package.json`: `"test": "vitest run"`, `"e2e": "playwright test"`.

**Caution:** `emptyOutDir` deletes `internal/spa/dist/.gitkeep`. Re-create it after each build with the Makefile's `web` target (Step 6).

- [ ] **Step 3: Initialise shadcn/ui**

```bash
cd web && bunx --bun shadcn@latest init
```
Choose: TypeScript yes, base colour **neutral**, CSS variables yes, CSS file `src/app.css`, aliases `@/components`, `@/lib/utils`. Task 10 replaces the colour tokens.

```bash
bunx --bun shadcn@latest add button input label card dialog sheet sonner dropdown-menu separator
```

- [ ] **Step 4: Write `web/src/main.tsx` and the root route**

```tsx
// web/src/main.tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createRouter } from '@tanstack/react-router'
import { routeTree } from './routeTree.gen'
import { queryClient } from '@/api/query-client'
import './app.css'

export const router = createRouter({ routeTree, context: { queryClient }, defaultPreload: 'intent' })
declare module '@tanstack/react-router' {
  interface Register { router: typeof router }
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
)
```

```tsx
// web/src/routes/__root.tsx
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router'
import type { QueryClient } from '@tanstack/react-query'
import { Toaster } from '@/components/ui/sonner'

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: () => (<><Outlet /><Toaster richColors position="top-center" /></>),
})
```

Task 9 creates `@/api/query-client`. For now, create `web/src/api/query-client.ts` with `export const queryClient = new QueryClient()` so the app builds. Task 9 replaces it.

`web/src/routes/index.tsx` is a temporary `<p>karots-suite</p>`, replaced in Task 12. Delete Vite's demo `App.tsx`, `App.css` and `assets/`.

- [ ] **Step 5: Wire tygo types.** Install tygo with `go install github.com/gzuidhof/tygo@latest`, then write `tygo.yaml`:

```yaml
packages:
  - path: "karots-suite/internal/api"
    include_files: ["dto.go"]
    output_path: "web/src/api/types.gen.ts"
    frontmatter: "// GENERATED by `make types` from internal/api/dto.go — do not edit."
```

- [ ] **Step 6: Extend the `Makefile`.** Add these to `.PHONY`:

```make
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
```

Copy `.air.toml` from karots-pos, then remove any `templ generate` pre-command and `.templ` from `include_ext`. Set `bin = "./tmp/main"` and `cmd = "go build -o ./tmp/main ./cmd/server"`.

- [ ] **Step 7: Verify the single-binary path end to end**

```bash
make types && test -s web/src/api/types.gen.ts && grep -q 'export interface Me' web/src/api/types.gen.ts
make build && make verify-portable
set -a && . ./.env && set +a && ./bin/karots-suite &
sleep 2; curl -s localhost:3100/ | grep -q '<div id="root">' && curl -s localhost:3100/api/v1/nope | grep -q '"error"' && echo SERVED
kill %1
```
Expected: `OK: … statically linked`, then `SERVED`.

- [ ] **Step 8: Commit**

```bash
git add -A && git commit -m "feat(web): Vite+React+TanStack+Tailwind+shadcn scaffold, tygo types, single-binary build"
```

---

### Task 9: API client, session redirects, Konami, and the `me` query

**Files:**
- Create:
  - `web/src/api/client.ts`, `web/src/api/client.test.ts`
  - `web/src/api/session.ts`, `web/src/api/session.test.ts`
  - `web/src/api/query-client.ts` (replace)
  - `web/src/lib/konami.ts`, `web/src/lib/konami.test.ts`

**Interfaces:**
- Produces:
  - `api<T>(path: string, opts?: { method?: string; json?: unknown; signal?: AbortSignal }): Promise<T>`, where `path` is relative to `/api/v1`
  - `class ApiError extends Error { status: number; code: string; path: string }`
  - `redirectFor(err: unknown): '/login' | '/lock' | '/account/pin' | null`
  - `meQuery` (a `queryOptions` for `['me']`)
  - `queryClient` (redirects globally via `onSessionError`)
  - `KONAMI: string[]`, `konamiMatcher(onMatch: () => void): (key: string) => void`, `useKonami(onMatch: () => void, enabled?: boolean): void`

- [ ] **Step 1: Write the failing tests**

```ts
// web/src/api/client.test.ts
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError } from './client'

const reply = (status: number, body?: unknown) =>
  vi.stubGlobal('fetch', vi.fn(async () => new Response(body === undefined ? null : JSON.stringify(body), { status })))
afterEach(() => vi.unstubAllGlobals())

describe('api', () => {
  it('prefixes /api/v1, sends JSON, returns body', async () => {
    reply(200, { ok: 1 })
    expect(await api<{ ok: number }>('/me', { method: 'POST', json: { a: 1 } })).toEqual({ ok: 1 })
    const [url, init] = (fetch as unknown as ReturnType<typeof vi.fn>).mock.calls[0]
    expect(url).toBe('/api/v1/me')
    expect(init.headers['Content-Type']).toBe('application/json')
    expect(init.body).toBe('{"a":1}')
    expect(init.credentials).toBe('same-origin')
  })
  it('returns undefined on 204', async () => {
    reply(204)
    expect(await api('/auth/lock', { method: 'POST' })).toBeUndefined()
  })
  it('throws ApiError with server code', async () => {
    reply(403, { error: { code: 'SESSION_LOCKED', message: 'Session locked' } })
    await expect(api('/x')).rejects.toMatchObject({ status: 403, code: 'SESSION_LOCKED', message: 'Session locked', path: '/x' })
  })
  it('survives a non-JSON error body', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('<html>502</html>', { status: 502 })))
    const e = await api('/x').catch((x) => x)
    expect(e).toBeInstanceOf(ApiError)
    expect(e.code).toBe('HTTP_ERROR')
  })
})
```

```ts
// web/src/api/session.test.ts
import { describe, expect, it } from 'vitest'
import { ApiError } from './client'
import { redirectFor } from './session'

const e = (status: number, code: string, path = '/products') => new ApiError(status, code, 'm', path)

describe('redirectFor', () => {
  it('routes session codes', () => {
    expect(redirectFor(e(401, 'UNAUTHORIZED'))).toBe('/login')
    expect(redirectFor(e(403, 'SESSION_LOCKED'))).toBe('/lock')
    expect(redirectFor(e(403, 'PIN_CHANGE_REQUIRED'))).toBe('/account/pin')
  })
  it('never redirects for /auth/* calls (wrong PIN is handled by the form)', () => {
    expect(redirectFor(e(401, 'UNAUTHORIZED', '/auth/login'))).toBeNull()
    expect(redirectFor(e(403, 'SESSION_LOCKED', '/auth/pin'))).toBeNull()
  })
  it('ignores other errors', () => {
    expect(redirectFor(e(409, 'TILL_OPEN'))).toBeNull()
    expect(redirectFor(new Error('x'))).toBeNull()
  })
})
```

Replace `'UNAUTHORIZED'` with the exact `Code` that forked `apperr.Unauthorized` sets (see Task 4 Interfaces).

```ts
// web/src/lib/konami.test.ts
import { describe, expect, it, vi } from 'vitest'
import { KONAMI, konamiMatcher } from './konami'

describe('konami', () => {
  it('fires once on the full sequence', () => {
    const hit = vi.fn(); const feed = konamiMatcher(hit)
    KONAMI.forEach(feed)
    expect(hit).toHaveBeenCalledTimes(1)
  })
  it('accepts caps-lock B/A', () => {
    const hit = vi.fn(); const feed = konamiMatcher(hit)
    ;[...KONAMI.slice(0, 8), 'B', 'A'].forEach(feed)
    expect(hit).toHaveBeenCalledTimes(1)
  })
  it('recovers after a wrong key', () => {
    const hit = vi.fn(); const feed = konamiMatcher(hit)
    ;['ArrowUp', 'ArrowUp', 'x', ...KONAMI].forEach(feed)
    expect(hit).toHaveBeenCalledTimes(1)
  })
  it('tolerates an extra ArrowUp before the sequence (karots-pos matcher failed this)', () => {
    const hit = vi.fn(); const feed = konamiMatcher(hit)
    ;['ArrowUp', ...KONAMI].forEach(feed)
    expect(hit).toHaveBeenCalledTimes(1)
  })
})
```

Run: `cd web && bun run test` → Expected: FAIL (modules missing).

- [ ] **Step 2: Implement**

```ts
// web/src/api/client.ts
export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string, public path: string) {
    super(message)
  }
}

type Opts = { method?: string; json?: unknown; signal?: AbortSignal }

/** Typed JSON call to /api/v1. Cookie session rides along automatically. */
export async function api<T = void>(path: string, opts: Opts = {}): Promise<T> {
  const hasBody = opts.json !== undefined
  const res = await fetch(`/api/v1${path}`, {
    method: opts.method ?? (hasBody ? 'POST' : 'GET'),
    credentials: 'same-origin',
    headers: hasBody ? { 'Content-Type': 'application/json' } : {},
    body: hasBody ? JSON.stringify(opts.json) : undefined,
    signal: opts.signal,
  })
  if (res.status === 204) return undefined as T
  const body = await res.json().catch(() => null)
  if (!res.ok) throw new ApiError(res.status, body?.error?.code ?? 'HTTP_ERROR', body?.error?.message ?? res.statusText, path)
  return body as T
}
```

```ts
// web/src/api/session.ts
import { queryOptions } from '@tanstack/react-query'
import { api, ApiError } from './client'
import type { Me } from './types.gen'

const ROUTES: Record<string, '/login' | '/lock' | '/account/pin'> = {
  UNAUTHORIZED: '/login',
  SESSION_LOCKED: '/lock',
  PIN_CHANGE_REQUIRED: '/account/pin',
}

/** Where a failed call should send the user, or null to let the caller handle it. */
export function redirectFor(err: unknown) {
  if (!(err instanceof ApiError) || err.path.startsWith('/auth/')) return null
  return ROUTES[err.code] ?? null
}

export const meQuery = queryOptions({ queryKey: ['me'], queryFn: () => api<Me>('/me'), staleTime: 60_000, retry: false })
```

```ts
// web/src/api/query-client.ts
import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query'
import { redirectFor } from './session'

/** Any query/mutation hitting an expired/locked/forced-PIN session navigates away. */
function onSessionError(err: unknown) {
  const to = redirectFor(err)
  if (to && window.location.pathname !== to) window.location.assign(to)
}

export const queryClient = new QueryClient({
  queryCache: new QueryCache({ onError: onSessionError }),
  mutationCache: new MutationCache({ onError: onSessionError }),
  defaultOptions: { queries: { retry: (n, e) => !redirectFor(e) && n < 2 } },
})
```

```ts
// web/src/lib/konami.ts
import { useEffect, useRef } from 'react'

export const KONAMI = ['ArrowUp', 'ArrowUp', 'ArrowDown', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'ArrowLeft', 'ArrowRight', 'b', 'a']

/** Rolling window of the last 10 keys — unlike a position counter, an extra
 *  ArrowUp (↑↑↑↓↓…) or any stray key before the sequence can't break it. */
export function konamiMatcher(onMatch: () => void) {
  let recent: string[] = []
  const target = KONAMI.join()
  return (key: string) => {
    const k = key.length === 1 ? key.toLowerCase() : key
    recent = [...recent, k].slice(-KONAMI.length)
    if (recent.join() === target) { recent = []; onMatch() }
  }
}

/** Listens on window while enabled. Used by login (install id) and kiosk exit. */
export function useKonami(onMatch: () => void, enabled = true) {
  const cb = useRef(onMatch)
  cb.current = onMatch
  useEffect(() => {
    if (!enabled) return
    const feed = konamiMatcher(() => cb.current())
    const onKey = (e: KeyboardEvent) => feed(e.key)
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [enabled])
}
```

- [ ] **Step 3: Run the tests**

Run: `cd web && bun run test && bunx tsc -b` → Expected: all PASS, no type errors.

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "feat(web): typed api client, session redirects, konami matcher"
```

---

### Task 10: Theme design, then tokens and appearance application

**This task has a user gate.** No shell or screen styling is final until the user approves the design.

**Files:**
- Create: `web/src/theme/tokens.css` (imported by `app.css`), `web/src/theme/appearance.ts`, `web/src/theme/appearance.test.ts`, `docs/design/theme.md`
- Modify: `web/src/app.css`

**Interfaces:**
- Produces:
  - `applyAppearance(a: Appearance, root?: HTMLElement): void`
  - CSS tokens used by every component: `--brand`, `--brand-foreground`, `--radius`, and shadcn's `--background`/`--foreground`/`--card`/`--muted`/`--border`, …
  - `[data-density="compact"]` spacing overrides
  - `[data-theme="dark"]` plus `prefers-color-scheme`

- [ ] **Step 1: Design with the user.** Invoke `frontend-design:frontend-design`. Produce `docs/design/theme.md` with:
  1. palette (light and dark), with contrast ≥ 4.5:1 for text
  2. type scale and font (a Google Font or a system stack; it must work offline, so self-host it through `@fontsource/*` or use a system stack)
  3. radius scale for sharp, rounded and round
  4. spacing for comfortable and compact
  5. the admin shell layout at 360px and 1366px
  6. the till shell layout at 1366×768 and 1024×768

Render static mockups (an HTML artifact is fine) of the login page, admin home at both widths, and the till shell. It **must not** use the karots-pos indigo/slate look or its layouts. **STOP and show the user. Continue only after explicit approval.** Apply requested changes, then re-show.

- [ ] **Step 2: Write the failing test**

```ts
// web/src/theme/appearance.test.ts
import { describe, expect, it } from 'vitest'
import { applyAppearance } from './appearance'

describe('applyAppearance', () => {
  it('sets brand, radius and density', () => {
    const el = document.createElement('div')
    applyAppearance({ brandColor: '#0f766e', radius: 'round', density: 'compact', receiptStyle: 'classic' }, el)
    expect(el.style.getPropertyValue('--brand')).toBe('#0f766e')
    expect(el.dataset.radius).toBe('round')
    expect(el.dataset.density).toBe('compact')
  })
  it('clears a previous brand when reset to theme default', () => {
    const el = document.createElement('div')
    applyAppearance({ brandColor: '#0f766e', radius: 'rounded', density: 'comfortable', receiptStyle: 'classic' }, el)
    applyAppearance({ brandColor: '', radius: 'rounded', density: 'comfortable', receiptStyle: 'classic' }, el)
    expect(el.style.getPropertyValue('--brand')).toBe('')
  })
})
```

- [ ] **Step 3: Implement**

```ts
// web/src/theme/appearance.ts
import type { Appearance } from '@/api/types.gen'

/** Applies the system-locked look to <html>. Tokens in tokens.css read these. */
export function applyAppearance(a: Appearance, root: HTMLElement = document.documentElement) {
  if (a.brandColor) root.style.setProperty('--brand', a.brandColor)
  else root.style.removeProperty('--brand')
  root.dataset.radius = a.radius
  root.dataset.density = a.density
}
```

In `tokens.css`, put the approved palette as `:root` and `[data-theme="dark"]` variables, plus `@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) { … } }`. Map `[data-radius="sharp"|"rounded"|"round"]` to `--radius` values. Derive `--brand-foreground` with `color-mix()` or a fixed pair, and point shadcn's `--primary` at `var(--brand)`. Components must use tokens only, never raw colour classes like `bg-indigo-600`.

- [ ] **Step 4: Run the tests**

Run: `cd web && bun run test` → Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(web): approved theme tokens + appearance application"
```

---

### Task 11: Login, lock, change-PIN and auto-lock

**Files:**
- Create:
  - `web/src/routes/login.tsx`, `web/src/routes/lock.tsx`, `web/src/routes/account.pin.tsx`
  - `web/src/features/auth/PinForm.tsx` (the shared phone + PIN form)
  - `web/src/features/auth/AutoLock.tsx`
  - `web/e2e/auth.spec.ts`, `web/playwright.config.ts`

**Interfaces:**
- Consumes: `api`, `ApiError`, `meQuery`, `useKonami`, and the `LoginInfo`/`LoginResponse`/`ChangePinRequest` types.
- Produces:
  - `<PinForm onSubmit={(phone, pin) => Promise<void>} submitLabel error />`
  - `<AutoLock minutes={number} />`, which does `POST /auth/lock` and then navigates to `/lock` after N idle minutes (0 = off)

- [ ] **Step 1: Write the Playwright config and the failing e2e test**

```ts
// web/playwright.config.ts
import { defineConfig } from '@playwright/test'
export default defineConfig({
  testDir: 'e2e',
  use: { baseURL: 'http://localhost:3100' },
  webServer: {
    command: 'cd .. && make build && set -a && . ./.env && set +a && ./bin/karots-suite',
    url: 'http://localhost:3100/api/v1/auth/login-info',
    reuseExistingServer: true,
    timeout: 180_000,
  },
})
```

```ts
// web/e2e/auth.spec.ts
import { expect, test } from '@playwright/test'

// Seeded by `make e2e-seed` (Step 2): admin 0770000001 / cashier 0770000002, PIN 1234.
const login = async (page, phone: string) => {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
}

test('bad cookie on load lands on login, not a blank page or loop', async ({ page, context }) => {
  await context.addCookies([{ name: 'pos_token', value: 'garbage', url: 'http://localhost:3100' }])
  await page.goto('/admin')
  await expect(page).toHaveURL(/\/login$/)
  await expect(page.getByRole('button', { name: /sign in/i })).toBeVisible()
})

test('wrong PIN shows an inline error and stays on login', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('9999')
  await page.getByRole('button', { name: /sign in/i }).click()
  await expect(page.getByRole('alert')).toBeVisible()
  await expect(page).toHaveURL(/\/login$/)
})

test('konami on login reveals the install id', async ({ page }) => {
  await page.goto('/login')
  await expect(page.getByText(/install id/i)).toBeHidden()
  for (const k of ['ArrowUp', 'ArrowUp', 'ArrowDown', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'ArrowLeft', 'ArrowRight', 'b', 'a'])
    await page.keyboard.press(k)
  await expect(page.getByText(/^[0-9A-F]{8}$/)).toBeVisible()
})

test('admin lands on /admin; lock + reload stays locked; unlock resumes', async ({ page }) => {
  await login(page, '0770000001')
  await expect(page).toHaveURL(/\/admin/)
  await page.getByRole('button', { name: /lock/i }).click()
  await expect(page).toHaveURL(/\/lock$/)
  await page.goto('/admin')
  await expect(page).toHaveURL(/\/lock$/)
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /unlock/i }).click()
  await expect(page).toHaveURL(/\/admin/)
})

test('cashier lands on /till', async ({ page }) => {
  await login(page, '0770000002')
  await expect(page).toHaveURL(/\/till/)
})
```

The admin-shell and till-shell assertions depend on Task 12. This spec is completed there, so run only the first three tests now.

- [ ] **Step 2: Add a seed target for e2e users.** Add to the `Makefile`:

```make
e2e-seed: db-up
	docker compose exec -T postgres psql -U suite_user -d suite_db -c "DELETE FROM users WHERE phone IN ('0770000001','0770000002')"
	$(ENV) go run ./cmd/server -seed
```

Check what karots-pos `cmd/server/seed.go` creates. The Demo Room logins are owner `0770000001`/`1234`. If `-seed` doesn't create both users with PIN 1234 and `must_change_pin=false`, add a small `scripts/e2e-users.sql` that inserts them using a bcrypt hash generated once with `htpasswd -bnBC 10 "" 1234 | tr -d ':\n'`, and run it in `e2e-seed` instead.

- [ ] **Step 3: Implement the screens.** Use the approved theme and shadcn components.
  - **`login.tsx`**
    - `useQuery(LoginInfo)` from `/auth/login-info`.
    - On mount, `console.info('POS install ' + installId + ' · server time ' + serverTime)`.
    - `useKonami(() => setReveal(true))` shows the Install ID block (label "Install ID", the id in mono, and the server time).
    - Submit does `api<LoginResponse>('/auth/login', { json: { phone, pin } })`, then `queryClient.removeQueries({queryKey:['me']})`, then navigates to `/`.
    - On `ApiError`, render `<p role="alert">{err.message}</p>`.
  - **`lock.tsx`**
    - `beforeLoad` fetches `meQuery`; not locked → redirect `/`; 401 → `/login`.
    - Shows "Locked — {me.user.name}" and a `PinForm` that submits `/auth/unlock`, then does `invalidateQueries(['me'])` and navigates to `/`.
    - A sign-out link calls `/auth/logout`. On `TILL_OPEN`, toast the message.
  - **`account.pin.tsx`**
    - Three PIN fields submit `/auth/pin`, then `invalidateQueries(['me'])` and navigate to `/`.
    - When `me.mustChangePin`, show "Choose a new PIN to continue" and hide the cancel link.
  - **`PinForm.tsx`**
    - Labelled inputs "Phone" (`inputMode="tel"`, `autoComplete="username"`) and "PIN" (`type="password"`, `inputMode="numeric"`, `autoComplete="current-password"`, 4–6 digits).
    - The submit button is disabled while pending.
  - **`AutoLock.tsx`**
    - Resets an idle timer on `mousemove mousedown keydown touchstart scroll click`.
    - On timeout it does `api('/auth/lock',{method:'POST'})`, then `window.location.assign('/lock')`.
    - It renders nothing when `minutes <= 0`.

- [ ] **Step 4: Run the tests**

```bash
make e2e-seed && cd web && bun run test && bunx playwright install chromium && bunx playwright test -g "bad cookie|wrong PIN|konami"
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(web): login (+install-id konami), lock, change PIN, auto-lock"
```

---

### Task 12: Guarded app shells (admin responsive, till kiosk) and kiosk-exit Konami

**Files:**
- Create:
  - `web/src/routes/_authed.tsx`
  - `web/src/routes/_authed/admin.tsx`, `web/src/routes/_authed/admin/index.tsx`
  - `web/src/routes/_authed/till.tsx`, `web/src/routes/_authed/till/index.tsx`
  - `web/src/theme/AdminShell.tsx`, `web/src/theme/TillShell.tsx`, `web/src/theme/nav.ts`
  - `web/src/features/kiosk/KioskExit.tsx`, `web/src/lib/autofit.ts`, `web/src/lib/autofit.test.ts`
  - `web/e2e/shell.spec.ts`
- Modify: `web/src/routes/index.tsx`

**Interfaces:**
- Consumes: `meQuery`, `redirectFor`, `applyAppearance`, `useKonami`, `AutoLock`.
- Produces:
  - Route context `{ me: Me }` for everything under `_authed`.
  - `adminNav: { to: string; label: string; icon: LucideIcon; mobile?: boolean }[]` in `theme/nav.ts`. In M0 it holds only Home plus System (the system user only). Later milestones append entries.
  - `<AdminShell>`: a sidebar at 768px and up; below 768px, a top bar, a bottom tab bar (`mobile: true` items) and a "More" `Sheet`.
  - `<TillShell>`: a full-height desktop frame with `autofit`.
  - `autofitZoom(needed: number, avail: number, pad?: number): number`

- [ ] **Step 1: Write the failing autofit test.** This ports karots-pos `kioskAutoFit` exactly: it only shrinks, and leaves 16px of slack.

```ts
// web/src/lib/autofit.test.ts
import { describe, expect, it } from 'vitest'
import { autofitZoom } from './autofit'

describe('autofitZoom', () => {
  it('never enlarges', () => expect(autofitZoom(1000, 1920)).toBe(1))
  it('shrinks to fit with 16px slack', () => expect(autofitZoom(1384, 1366)).toBeCloseTo(1366 / 1400, 4))
})
```

- [ ] **Step 2: Implement `autofit.ts`**

```ts
/** Zoom that makes `needed` px fit `avail` px (never above 1). Ported from karots-pos kioskAutoFit. */
export function autofitZoom(needed: number, avail: number, pad = 16) {
  return needed + pad > avail ? Number((avail / (needed + pad)).toFixed(4)) : 1
}

/** Keeps the till frame fitted on sub-1080p counter screens. Returns a cleanup fn. */
export function startAutofit(el: HTMLElement = document.documentElement) {
  const fit = () => {
    el.style.setProperty('zoom', '1')
    el.style.setProperty('zoom', String(autofitZoom(Math.max(el.scrollWidth, document.body.scrollWidth), el.clientWidth)))
  }
  const ro = new ResizeObserver(fit)
  ro.observe(document.body)
  window.addEventListener('resize', fit)
  document.fonts?.ready.then(fit)
  fit()
  return () => { ro.disconnect(); window.removeEventListener('resize', fit); el.style.removeProperty('zoom') }
}
```

- [ ] **Step 3: Write the `_authed` guard**

```tsx
// web/src/routes/_authed.tsx
import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
import { useEffect } from 'react'
import { meQuery, redirectFor } from '@/api/session'
import { applyAppearance } from '@/theme/appearance'
import { AutoLock } from '@/features/auth/AutoLock'
import { KioskExit } from '@/features/kiosk/KioskExit'

export const Route = createFileRoute('/_authed')({
  beforeLoad: async ({ context }) => {
    let me
    try {
      me = await context.queryClient.ensureQueryData(meQuery)
    } catch (err) {
      throw redirect({ to: redirectFor(err) ?? '/login' })
    }
    if (me.locked) throw redirect({ to: '/lock' })
    if (me.mustChangePin) throw redirect({ to: '/account/pin' })
    return { me }
  },
  component: Authed,
})

function Authed() {
  const { me } = Route.useRouteContext()
  useEffect(() => applyAppearance(me.appearance), [me.appearance])
  return (
    <>
      <Outlet />
      <AutoLock minutes={me.lockTimeoutMinutes} />
      {me.flags.canExitKiosk && <KioskExit />}
    </>
  )
}
```

- [ ] **Step 4: Implement the shells and routes.**
  - **`KioskExit.tsx`:** `useKonami(() => api('/kiosk/exit', { method: 'POST' }).catch(() => toast.error('Kiosk exit failed')))`, and it renders `null`. It only mounts when `canExitKiosk`, so a cashier never has it armed.
  - **`admin.tsx`:**
    - `beforeLoad`: if `me.user.role === 'cashier'`, redirect to `/till`.
    - Renders `<AdminShell><Outlet/></AdminShell>`.
    - The shell shows the shop name, the user menu (Lock, Change PIN, Sign out with `TILL_OPEN` shown as a toast; Lock and Sign out both end with `window.location.assign(...)`, so no stale cached `me` survives), `adminNav` filtered by `me` (the System entry only when `me.flags.isSystem`), and a "Till" link.
  - **`admin/index.tsx`:** a welcome `Card` ("Welcome, {name}") with an empty state saying the modules arrive in later milestones.
  - **`till.tsx`:**
    - Renders `<TillShell>`, which calls `useEffect(() => startAutofit(), [])`.
    - The top strip holds the shop name, cashier name, Lock button and Sign out.
    - The body is a placeholder "Till — M1".
    - The till is desktop only, with no mobile layout.
  - **`routes/index.tsx`:** in `beforeLoad`, use `ensureQueryData(meQuery)`. A cashier redirects to `/till`, anyone else to `/admin`, and an error redirects to `redirectFor(err) ?? '/login'`.
  - All UI uses theme tokens only. Before writing `AdminShell`, check it against the approved mockups in `docs/design/theme.md`.

- [ ] **Step 5: Write the shell e2e test and finish `auth.spec.ts`**

```ts
// web/e2e/shell.spec.ts
import { expect, test } from '@playwright/test'

async function loginAdmin(page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await expect(page).toHaveURL(/\/admin/)
}

test('admin shell is usable at 360px (no horizontal scroll, bottom tabs)', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 780 })
  await loginAdmin(page)
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(360)
  await expect(page.getByRole('navigation', { name: /primary/i })).toBeVisible()
})

test('admin shell shows sidebar at 1366px', async ({ page }) => {
  await page.setViewportSize({ width: 1366, height: 768 })
  await loginAdmin(page)
  await expect(page.getByRole('navigation', { name: /sidebar/i })).toBeVisible()
})

test('deep link reload survives', async ({ page }) => {
  await loginAdmin(page)
  await page.goto('/admin?x=1')
  await expect(page).toHaveURL(/\/admin/)
  await page.reload()
  await expect(page.getByText(/welcome/i)).toBeVisible()
})
```

Give the bottom tab bar `aria-label="Primary"` and the sidebar `aria-label="Sidebar"`.

- [ ] **Step 6: Run the tests**

```bash
make e2e-seed && cd web && bun run test && bunx tsc -b && bunx playwright test
```
Expected: all PASS, including every test in `auth.spec.ts`.

- [ ] **Step 7: Commit**

```bash
git add -A && git commit -m "feat(web): guarded admin (responsive) + till (autofit) shells, kiosk-exit konami"
```

---

### Task 13: System menu → Appearance page

**Files:**
- Create: `web/src/routes/_authed/admin/system.appearance.tsx`, `web/e2e/system.spec.ts`
- Modify: `web/src/theme/nav.ts` (add "System → Appearance", visible when `me.flags.isSystem`)

**Interfaces:**
- Consumes: `GET/PUT /system/appearance`, `applyAppearance`, `meQuery`.

- [ ] **Step 1: Write the failing e2e test**

```ts
// web/e2e/system.spec.ts
import { expect, test } from '@playwright/test'

test('shop admin cannot reach system appearance', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.goto('/admin/system/appearance')
  await expect(page.getByText(/not found/i)).toBeVisible()
})

test('system user changes brand colour and it applies live', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0000000001')
  await page.getByLabel('PIN').fill(process.env.POS_SYSTEM_PIN ?? '2273')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.goto('/admin/system/appearance')
  await page.getByLabel(/brand colou?r/i).fill('#0f766e')
  await page.getByRole('button', { name: /save/i }).click()
  await expect(page.getByText(/saved/i)).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.style.getPropertyValue('--brand'))).toBe('#0f766e')
  await page.getByLabel(/brand colou?r/i).fill('')
  await page.getByRole('button', { name: /save/i }).click()
})
```

The hidden system user is re-ensured at every server boot (`ensureSystemAdmin`). In dev, `POS_SYSTEM_PIN=2273` from `.env` is its PIN.

- [ ] **Step 2: Implement the page.**
  - `beforeLoad`: if `!me.flags.isSystem`, `throw notFound()`. Add a `notFoundComponent` on the root route that shows "Not found".
  - Load the current values with `useQuery` on `['system','appearance']`.
  - Controls:
    - "Brand colour": a native `<input type="color">` plus a hex text input labelled "Brand colour", and a "Use theme default" button that sends `""`.
    - Radius: a segmented control for sharp, rounded and round.
    - Density: comfortable or compact.
    - Receipt style: classic, modern, boxed or compact.
  - Changes preview live through `applyAppearance`.
  - Save does `PUT`, then `toast.success('Saved')`, then `invalidateQueries(['me'])`.
  - Mobile-first layout: a single column below 768px.

- [ ] **Step 3: Run the tests**

```bash
cd web && bunx tsc -b && bunx playwright test system.spec.ts
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "feat(web): system-only Appearance page with live preview"
```

---

### Task 14: Parity list from karots-pos routes

**Files:**
- Create: `scripts/parity.sh`, `docs/parity.md`

- [ ] **Step 1: Write `scripts/parity.sh`**

```bash
#!/usr/bin/env bash
# Lists every route karots-pos serves (core web + plugins) as a checklist.
# Re-run any time; hand-edited ticks must be preserved, so this only writes a
# fresh file when docs/parity.md does not exist yet.
set -euo pipefail
SRC=${1:-$HOME/Projects/go/karots-pos}
OUT=docs/parity.md
[ -e "$OUT" ] && { echo "$OUT exists; delete it to regenerate"; exit 1; }
{
  echo "# karots-pos → karots-suite parity"
  echo
  echo "Generated $(date +%F) from karots-pos $(git -C "$SRC" rev-parse --short HEAD)."
  echo "Tick a row only when the route's behaviour is ported to /api/v1 + the SPA AND has a flow check."
  echo "Format: file · METHOD path (paths are relative to that file's route group)."
  for dir in internal/web $(cd "$SRC" && ls -d plugins/*); do
    echo; echo "## $dir"; echo
    (cd "$SRC" && grep -rnoE '\.(GET|POST|PUT|PATCH|DELETE)\("[^"]*"' --include='*.go' "$dir" | grep -v _test.go) \
      | sed -E 's#^([^:]+):[0-9]+:\.(GET|POST|PUT|PATCH|DELETE)\("([^"]*)"#- [ ] `\1` · \2 `\3`#' | sort -u
  done
} > "$OUT"
grep -c '^- \[ \]' "$OUT" | xargs echo "rows:"
```

- [ ] **Step 2: Run it**

```bash
chmod +x scripts/parity.sh && scripts/parity.sh
```
Expected: `rows:` followed by roughly 560, and `docs/parity.md` has one section per area and plugin.

- [ ] **Step 3: Tick the M0 rows.** Tick the rows that M0 covered: login, logout, lock, unlock, lock config (now in `/me`), account/pin, kiosk/exit, system/appearance, and the root redirect. Leave `receipt-preview` unticked; it moves to M6 with receipts.

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "docs: karots-pos route parity checklist (M0 rows ticked)"
```

---

### Task 15: Ship check and AGENTS.md

**Files:**
- Create: `AGENTS.md`, `README.md`

- [ ] **Step 1: Write `AGENTS.md`.** Use the structure of karots-pos's `AGENTS.md`, adapted to karots-suite. Cover:
  - the plugin-inert rule for Go *and* `web/src`
  - the layout from the File Structure above
  - the dev workflow: `make db-up`, `make dev`, `make test`, `make test-web`, `make types` after any `dto.go` change, `make build`, `make verify-portable`, `make e2e-seed` + `bunx playwright test`
  - API conventions: `/api/v1`, the error shape, camelCase, money as strings, `h.protected`, and one file per area
  - the rule that each area's port deletes that feature's legacy `api.go`/`RegisterAPI`
  - the migrations baseline (`00001_baseline.sql`, next is `00002_`)
  - frontend conventions: tokens only, shared components in `components/ui` and `theme/`, `intent load` before TanStack work, file routes
  - hidden behaviours: the system user, both Konami codes, auto-lock
  - Gotchas:
    - `web/go.mod` is the node_modules fence, so don't delete it
    - `vite build` wipes `internal/spa/dist/.gitkeep`, and `make web` restores it
    - dev ports are 5433, 3100 and 5173

  Keep it under 150 lines. `README.md` is a 20-line quickstart.

- [ ] **Step 2: Run the full verification**

```bash
make test && make test-web && make build && make verify-portable
make e2e-seed && (cd web && bunx playwright test)
grep -rniE 'recharge|pharmacy|repairs|documents|alternatives|clearance|aicatalog|productplus|combos|requests' internal cmd web/src --include='*.go' --include='*.ts' --include='*.tsx' | grep -v '_test.go' || echo "core is plugin-free"
```
Expected:
- every command is green
- `OK: … statically linked`
- all Playwright specs pass
- the last line prints `core is plugin-free`

If the grep hits generic words (for example "requests" in a comment), confirm each hit is not a plugin reference.

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "docs: AGENTS.md + README for karots-suite; M0 complete"
```
