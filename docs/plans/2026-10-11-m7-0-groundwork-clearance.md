# M7.0 Plugin groundwork + Clearance — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
> Same shape as M2–M6: Opus executes and reviews, one commit per step, UI gates self-reviewed (4 combos, 1366 + 360).

**Goal:** the path every plugin uses (build, Go seams, React slots) — proven end to end by Clearance, reinterpreted.

**Architecture:** a plugin is `plugins/<key>/` (Go, `init()` → `plugin.Register`) + `web/src/plugins/<key>/index.tsx` (`register(slots)`). `cmd/bootstrap` writes the two generated import lists and restores them to core-only. Core reaches plugins only through `internal/plugin` hooks and `web/src/plugins/slots.ts`; its JSON routes mount at `/api/v1/x/…`, its admin pages at `/admin/x/$key/*`.

**Tech stack:** Go / Echo / sqlx / goose (per-plugin version table), React + TanStack Router / Query, admin kit, Playwright.

**Spec:** `docs/specs/2026-10-11-m7-0-groundwork-clearance-design.md`.

## Global constraints

- **Core never references a plugin, and never changes for one plugin — only generic hooks** (user, 2026-10-11). Core = everything outside `plugins/` and `web/src/plugins/`. `make check-core` must pass after every task.
- `cmd/server/enabled_plugins.go` and `web/src/plugins/enabled-plugins.ts` are committed **core-only**.
- Money is decimal strings in JSON; errors via `apperr`; JSON camelCase; files ≤ 500 lines.
- Plugin pages: admin kit + theme tokens only; work at 360px and in Studio / Night × light / dark.
- No Claude / AI attribution in commits.

## Spec deviations found while planning (spec updated)

1. **No `0004` migration.** `clearance_markdowns.updated_at` is already the approval / dismiss time (only Approve and Dismiss write the row), so "ended" and "snoozed" are computed from it: a karots-pos backup restores with **zero mapping**.
2. **Plugins are not started at all today.** `plugin.SetupAll` is never called and the `Mux` is never mounted; the product func-vars (`products.BadgeProvider` …) are never fed. Task 2 wires all of it.
3. **No SPA visibility filter.** `plugin.SetVisibilityFilter` has no caller in the suite (no demo profiles), so `/me.plugins` just lists keys; filtering comes back if Demo Room needs it (M8).

## Review focus

1. **A core-only build after plugin work** — must compile, and the Apps tile shows its empty state. (Task 3 `make verify` core-only build; Task 4 shell test.)
2. **A plugin route hit by a cashier** — `/api/v1/x/clearance/*` must 403 for a cashier, 401 signed-out. (Task 2 test with a fixture plugin.)
3. **A product approved, sold out, then restocked** — the badge must be gone on the fresh stock and the product a candidate again only if still slow. (Task 5 DB test + Task 8 e2e.)
4. **A sale tendered with a method no plugin settles** — refused as a validation error, no sale / payment row. (Task 1 test.)
5. **Bootstrap fails mid-build** — both generated files still restored to core-only. (Task 3 test.)

---

### Task 1: Core says no plugin names; unknown tender refused

**Files:**
- Create: `scripts/check-core.sh`
- Modify: `Makefile` (target `check-core`), `internal/features/sales/service_types.go:62,70`, `internal/features/sales/service.go` (`WithPluginTenderLeg`, Create pre-check, comments at 25/61-73/96/396), `internal/app/services.go:46-60`, comments in `internal/features/sales/{service_lines,sales}.go`, `internal/features/products/{products,service,service_write}.go`, `internal/api/dto_reports_ops.go:86`
- Test: `internal/features/sales/plugin_tender_test.go`

**Interfaces:**
- Produces: `func (s *Service) WithPluginTenders(known func(method string) bool, leg PluginTenderLeg) *Service` (replaces `WithPluginTenderLeg`); `make check-core`.

- [ ] **Step 1: failing test** — unknown method refused before anything is written:

```go
func TestCreateRefusesUnknownTender(t *testing.T) {
	db := testDB(t) // same DATABASE_URL skip pattern as cashregister/withdraw_policy_test.go
	svc := sales.NewService(db).WithPluginTenders(func(string) bool { return false }, nil)
	before := count(t, db, `SELECT count(*) FROM sales`)
	_, err := svc.Create(ctx, seededCashierID(t, db), sales.CreateInput{
		Items:    []sales.ItemInput{oneSeededItem(t, db)},
		Payments: []sales.PaymentInput{{Method: "mystery", Amount: "10.00"}},
	})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeValidation {
		t.Fatalf("want validation error, got %v", err)
	}
	if count(t, db, `SELECT count(*) FROM sales`) != before {
		t.Fatal("a sale row was written")
	}
}
```
(helpers `testDB`, `count`, `seededCashierID`, `oneSeededItem` in the same file, ~25 lines, reading the seed rows the existing sales DB tests use.)

- [ ] **Step 2:** `go test ./internal/features/sales -run UnknownTender` → FAIL (method not defined / validator lets `wallet`-style methods through).
- [ ] **Step 3: implement.**
  - `PaymentInput.Method` validate → `validate:"required"`. Comment on `Meta`: "opaque per-line data for a plugin tender method; core never reads it".
  - `service.go`: field `knownPluginTender func(string) bool`; `WithPluginTenders(known, leg)`. At the top of `Create` (before the tx):
    ```go
    for _, p := range in.Payments {
    	if !coreTender(p.Method) && (s.knownPluginTender == nil || !s.knownPluginTender(p.Method)) {
    		return nil, apperr.Validation("unknown payment method: " + p.Method)
    	}
    }
    ```
  - `internal/app/services.go`: dispatcher refuses a method with no `SettleTx` (`return apperr.Validation(...)`) instead of `return nil`; wire with
    ```go
    known := func(m string) bool { for _, t := range plugin.TenderMethods() { if t.Value == m { return true } }; return false }
    salesSvc := sales.NewService(db).WithDrawerKick(kick).WithNonCashLeg(cf.SaleNonCashLeg).WithPluginTenders(known, pluginTenderLeg)
    ```
  - Rewrite every comment listed above generically ("a plugin tender", "a pass-through line such as a top-up", "a plugin's markdown") — no plugin name, no `wallet`.
- [ ] **Step 4: `scripts/check-core.sh`** — fails on any core hit:
    ```sh
    #!/usr/bin/env bash
    # Core never names a plugin (AGENTS.md hard rule). Keys come from every plugin.json
    # plus the tender values plugins add; ALLOW lists plain words that only look alike.
    set -euo pipefail
    cd "$(dirname "$0")/.."
    # Distinctive names: never in core at all (comments included).
    names="clearance|productplus|aicatalog|pharmacy|recharge|wallet"
    # Keys that are also plain English (requests, documents, alternatives, repairs, combos):
    # only an import path or a quoted literal counts — "HTTP requests" in a comment is fine.
    keys="requests|documents|alternatives|repairs|combos|$(ls plugins/*/plugin.json 2>/dev/null | xargs -r -n1 dirname | xargs -r -n1 basename | paste -sd'|' -)"
    keys=${keys%|}
    src() { grep -rnE "$@" internal cmd web/src --include='*.go' --include='*.ts' --include='*.tsx' \
      | grep -v '^web/src/plugins/' | grep -v '^cmd/bootstrap/' | grep -v 'types.gen.ts' || true; }
    hits=$( { src -iw "$names" | grep -vE 'lucide-react|\bWallet\b'; \
              src "plugins/($keys)\b|[\"'\`]($keys)[\"'\`]"; } || true)
    [ -z "$hits" ] || { echo "core names a plugin:"; echo "$hits"; exit 1; }
    for f in cmd/server/enabled_plugins.go web/src/plugins/enabled-plugins.ts; do
      [ ! -f "$f" ] || ! grep -q 'plugins/' "$f" || { echo "$f is not core-only — run make core-only"; exit 1; }
    done
    echo "OK: core is plugin-inert"
    ```
    `\bWallet\b` (capital, case-sensitive second grep) is the lucide icon; the first grep is `-i` so lower-case `wallet` still hits. Run it before the rewrite: the only hits must be the ones this task rewrites. If a new false positive appears, narrow the pattern — never add a blanket allow. `Makefile`: `check-core: ; bash scripts/check-core.sh`, add it to `.PHONY`.
- [ ] **Step 5:** `go test ./internal/features/sales/... ./internal/app/...` PASS; `make check-core` → OK; `go vet ./...`.
- [ ] **Step 6: commit** `fix(core): a sale with a payment method no plugin settles is refused before anything is written; core no longer names any plugin (validation list, comments) and make check-core keeps it that way`

---

### Task 2: Start plugins — registry, routes at /api/v1/x, product hooks, UI hooks pruned

**Files:**
- Modify: `internal/plugin/plugin.go` (add `Key() string` to `Plugin`), `internal/plugin/core.go` (+ `Products`, keep `Sales` = wired), `internal/plugin/hooks.go` (delete templ-era hooks), `internal/app/app.go` (`BuildServer`), `internal/api/api.go` (`Deps.MountPlugins`), `internal/api/me.go:43` (keys)
- Create: `internal/app/plugins.go` (registry + product-hook bridge), `internal/app/plugins_test.go`

**Interfaces:**
- Consumes: `Services` (Task 1 wiring).
- Produces: `plugin.Plugin.Key() string`; plugin routes under `/api/v1/x` (Public = no auth, Cashier = `h.protected`, Admin = protected + admin|manager); `api.Deps.MountPlugins func(public, cashier, admin *echo.Group)`; `/me.plugins` = keys.

- [ ] **Step 1: failing test** (`internal/app/plugins_test.go`) — a fixture plugin registered only in the test binary:

```go
type fixture struct{}
func (fixture) Name() string { return "Fixture" }
func (fixture) Key() string  { return "fixture" }
func (fixture) Migrations() (fs.FS, string) { return nil, "" }
func (fixture) Setup(r *plugin.Registry) {
	r.Admin().GET("/fixture/ping", func(c echo.Context) error { return c.JSON(200, map[string]string{"ok": "admin"}) })
	r.AddProductBadgeProvider(plugin.ProductBadgeProvider{Batch: func(_ context.Context, ids []int64) (map[int64][]string, error) {
		out := map[int64][]string{}; for _, id := range ids { out[id] = []string{"Fixture"} }; return out, nil
	}})
}
func init() { plugin.Register(fixture{}) }

func TestPluginsStart(t *testing.T) {
	e := BuildServer(testDB(t), testCfg(t), func(string, time.Time) bool { return false })
	if rec := do(e, "GET", "/api/v1/x/fixture/ping", ""); rec.Code != 401 { t.Fatalf("signed out: %d", rec.Code) }
	if rec := do(e, "GET", "/api/v1/x/fixture/ping", cashierCookie(t, e)); rec.Code != 403 { t.Fatalf("cashier: %d", rec.Code) }
	if rec := do(e, "GET", "/api/v1/x/fixture/ping", adminCookie(t, e)); rec.Code != 200 { t.Fatalf("admin: %d", rec.Code) }
	rows := []products.Product{{ID: 1}}
	products.Decorate(context.Background(), rows)
	if len(rows[0].Badges) != 1 || rows[0].Badges[0] != "Fixture" { t.Fatalf("badge bridge: %v", rows[0].Badges) }
}
```
(`do`, `adminCookie`, `cashierCookie` log in the e2e users from `scripts/e2e-users.sql` — reuse the login helper the `internal/api` tests already have, copied as a 20-line helper.)

- [ ] **Step 2:** `go test ./internal/app -run PluginsStart` → FAIL (404: nothing mounted).
- [ ] **Step 3: implement.**
  - `Plugin` gets `Key() string` ("stable id, matches plugin.json key and the web/src/plugins folder").
  - `internal/app/plugins.go`:
    ```go
    // startPlugins builds the registry over the same wired services core uses,
    // runs every plugin's Setup, and feeds the product func-vars from the hooks.
    // Returns the route mounter for api.Register.
    func startPlugins(e *echo.Echo, db *sqlx.DB, cfg *config.Config, svc *Services) func(pub, cashier, admin *echo.Group) {
    	mux := plugin.NewMux()
    	reg := plugin.NewRegistry(plugin.Core{DB: db, Cfg: cfg, Audit: audit.NewService(db), Settings: svc.Settings,
    		CashRegister: svc.CashRegister, Sales: svc.Sales, Cashflow: svc.Cashflow, Lockers: lockers.NewService(db),
    		Expenses: expenses.NewService(db), Products: products.NewService(db), Stock: stock.NewService(db)}, mux, e)
    	plugin.SetupAll(reg)
    	bridgeProductHooks()
    	return mux.Mount
    }
    ```
    (constructor names: match each package's existing `NewService` signature — check before writing.) `bridgeProductHooks` sets `products.BadgeProvider`, `SaleSuggestionProvider`, `DetailContributor`, `LabelFieldProvider`, `SearchContributor` **only when at least one provider is registered**, each merging every provider's result and skipping a provider that errors (a plugin issue never breaks the till).
  - `BuildServer`: `mount := startPlugins(e, sqlxDB, cfg, svc)` before `api.Register`, pass `MountPlugins: mount`.
  - `api.Register`: after `registerAdmin`:
    ```go
    if d.MountPlugins != nil {
    	x := v1.Group("/x")
    	d.MountPlugins(x, v1.Group("/x", h.protected...),
    		v1.Group("/x", slices.Concat(h.protected, []echo.MiddlewareFunc{middleware.RequireRole(auth.RoleAdmin, auth.RoleManager)})...))
    }
    ```
  - `hooks.go`: delete `AdminNavEntry`, `CashierTab`, `PaletteEntry`, `ReportCard`, `CashierMenuRoot`, `DrawerSection`, `CashierSupplierAction`, `ReceiptTab`, their vars / `Add*` / getters / `*JSON` funcs, and `ReadinessRow.Edit*`; `hooks_product.go`: delete `ProductRowAction`. Header comment rewritten: "Data and behaviour hooks only. A plugin's UI is React, registered into web/src/plugins/slots.ts." Each deleted hook's replacement goes in the commit body: AdminNav → `slots.apps` + `slots.adminPages` (Task 4); CashierTab → `slots.tillTabs`; CashierMenuRoot → `slots.tillMenu`; DrawerSection → `slots.drawerSections`; ReceiptTab → `slots.tillReceiptKinds`; ReportCard → `slots.reportCards`; PaletteEntry → apps feed Ctrl K (Task 4); CashierSupplierAction / ReadinessRow edit / ProductRowAction → a slot added by the first plugin needing it (recharge / productplus), noted in status.md.
  - `me.go`: `me.Plugins = append(me.Plugins, p.Key())`.
- [ ] **Step 4:** `go test ./...` PASS, `make check-core` OK, `go vet ./...`.
- [ ] **Step 5: commit** `feat(plugin): plugins start — Setup runs over the wired core services, plugin JSON routes mount at /api/v1/x with the same guards as core, product badges / suggestions / info rows / label fields / search feed from plugin hooks; templ-era UI hooks removed (React slots replace them)`

---

### Task 3: `cmd/bootstrap` + make targets

**Files:**
- Create: `cmd/bootstrap/main.go` (port of karots-pos `cmd/bootstrap/main.go`, trimmed), `cmd/bootstrap/render.go`, `cmd/bootstrap/render_test.go`, `web/src/plugins/enabled-plugins.ts`
- Modify: `Makefile` (`plugins`, `core-only`, `verify`), `web/playwright.config.ts` (webServer builds with every plugin), `cmd/server/enabled_plugins.go` comment

**Interfaces:**
- Produces: `manifest{Key, Name, Import, Version, Description}` from `plugins/*/plugin.json`; `renderGo(sel []manifest) []byte`, `renderTS(sel []manifest) []byte`; flags `-plugins a,b|all`, `-os`, `-arch`, `-name` (default `karots-suite`), `-out` (default `dist`), `-yes`, `-install-id`, `-X`, `-write-only`.

- [ ] **Step 1: failing test** (`render_test.go`):

```go
func TestRender(t *testing.T) {
	sel := []manifest{{Key: "clearance", Import: "karots-suite/plugins/clearance"}}
	if g := string(renderGo(sel)); !strings.Contains(g, `_ "karots-suite/plugins/clearance"`) { t.Fatal(g) }
	ts := string(renderTS(sel))
	if !strings.Contains(ts, `import clearance from './clearance'`) || !strings.Contains(ts, `clearance(slots)`) { t.Fatal(ts) }
	if g := string(renderGo(nil)); strings.Contains(g, "plugins/") { t.Fatal("core-only Go names a plugin") }
	if ts := string(renderTS(nil)); strings.Contains(ts, "import ") && strings.Contains(ts, "./") { t.Fatal("core-only TS imports a plugin") }
}

func TestRestoreOnFailure(t *testing.T) {
	dir := t.TempDir()
	goF, tsF := filepath.Join(dir, "a.go"), filepath.Join(dir, "b.ts")
	os.WriteFile(goF, renderGo(nil), 0o644); os.WriteFile(tsF, renderTS(nil), 0o644)
	err := withSelection(goF, tsF, []manifest{{Key: "x", Import: "karots-suite/plugins/x"}}, func() error { return errors.New("build failed") })
	if err == nil { t.Fatal("want the build error") }
	if g, _ := os.ReadFile(goF); !bytes.Equal(g, renderGo(nil)) { t.Fatal("Go file not restored") }
	if s, _ := os.ReadFile(tsF); !bytes.Equal(s, renderTS(nil)) { t.Fatal("TS file not restored") }
}
```
- [ ] **Step 2:** `go test ./cmd/bootstrap` → FAIL (undefined).
- [ ] **Step 3: implement.** `render.go`:
  ```go
  func renderGo(sel []manifest) []byte // package main + "// Generated by cmd/bootstrap" + import ( _ "…" ) block (empty: comment only)
  func renderTS(sel []manifest) []byte // header comment, `import { slots } from './slots'`, one `import <key> from './<key>'` + `<key>(slots)` per plugin
  // withSelection writes both files for sel, runs fn, and always writes them back core-only (defer).
  func withSelection(goPath, tsPath string, sel []manifest, fn func() error) error
  ```
  `main.go`: `discover` (verbatim from karots-pos), `selectPlugins`, `chooseTarget`, then `withSelection(..., func() error { return sh("make", "build") })` with `GOOS/GOARCH` env and `-ldflags` extra `-X` pairs, copy `bin/karots-suite` → `<out>/<name>[.exe]`. `-write-only`: write the selection and exit without restoring. Drop karots-pos's templ / stylesheet generation and `mergeEnv` (no plugin env yet — add when one ships an `env.sample`).
  `web/src/plugins/enabled-plugins.ts` = `renderTS(nil)`; imported once from `web/src/main.tsx` (`import '@/plugins/enabled-plugins'`) before the router is created.
  `Makefile`:
  ```make
  plugins:     ; go run ./cmd/bootstrap -write-only -yes -plugins $(or $(P),all)
  core-only:   ; go run ./cmd/bootstrap -write-only -yes -plugins none
  verify: check-core
  	go vet ./... && go build ./... && cd web && bunx tsc -b
  ```
  (`-plugins none` = empty selection.) Playwright `webServer.command`: `cd .. && make plugins P=all && make build && make core-only && set -a && . ./.env && set +a && ./bin/karots-suite` — the binary keeps every plugin, the tree goes back to core-only.
- [ ] **Step 4:** `go test ./cmd/bootstrap` PASS; `make plugins P=all && git status --short` shows both files changed; `make core-only && git status --short` clean; `make check-core && make verify` OK.
- [ ] **Step 5: commit** `feat(bootstrap): per-shop build with chosen plugins — writes the Go and React plugin lists, builds the one binary, always restores both to core-only; make plugins / core-only / verify; Playwright runs with every plugin built in`

---

### Task 4: React slots — Apps tile entries and admin plugin pages

**Files:**
- Modify: `web/src/plugins/slots.ts` (+ `apps`, `adminPages`), `web/src/theme/nav.ts:12,93-94` (`AppEntry` + `to`; `apps` from slots), `web/src/theme/Shelf.tsx:44-53` (Link), `web/src/theme/CommandPalette.tsx:19` (apps), `web/src/theme/PhoneTabs.tsx` (More lists apps as links)
- Create: `web/src/routes/_authed/admin/x.$key.$.tsx`, `web/src/theme/nav.test.ts` case

**Interfaces:**
- Produces:
  ```ts
  export type AppEntry = { key: string; label: string; description: string; icon: LucideIcon; color: string; to: string }
  export type AdminPage = { key: string; Page: ComponentType<{ path: string }> } // path = the splat after /admin/x/<key>/
  slots.apps: AppEntry[]; slots.adminPages: AdminPage[]
  ```
  `color` is a theme token expression (e.g. `'var(--brand)'`), never a raw colour.

- [ ] **Step 1: failing unit test** (`nav.test.ts`): pushing an `AppEntry` into `slots.apps` makes `apps` (nav) contain it and `paletteItems(areas)` (export the list builder from `CommandPalette.tsx` into `nav.ts` as `paletteItems`) include `{ label, to }`.
- [ ] **Step 2:** `cd web && bun run test nav` → FAIL.
- [ ] **Step 3: implement.** `nav.ts`: `export const apps = slots.apps` (move `AppEntry` type to `slots.ts`, re-export). Shelf tile → `<Link to={a.to} onClick={props.onClose}>` keeping the same classes. Palette: `...apps.map((a) => ({ key: 'app:' + a.key, label: a.label, to: a.to, icon: a.icon }))`. Route file:
  ```tsx
  /** A plugin's admin page (slots.adminPages), with its own sub-paths in the splat. */
  export const Route = createFileRoute('/_authed/admin/x/$key/$')({
    loader: ({ params }) => { if (!slots.adminPages.some((p) => p.key === params.key)) throw notFound() },
    component: function PluginAdminPage() {
      const { key, _splat } = Route.useParams()
      const Page = slots.adminPages.find((p) => p.key === key)!.Page
      return <Page path={_splat ?? ''} />
    },
  })
  ```
  (`/admin/x/clearance` with no trailing segment must also match — add `x.$key.index.tsx` rendering the same with `path=''` if the splat route doesn't catch it; check with the router's generated tree.)
- [ ] **Step 4:** `bun run test`, `bunx tsc -b`, `make check-core` OK; core-only build: Apps shelf still says "No apps installed on this shop yet."
- [ ] **Step 5: commit** `feat(web): plugin slots for the dock's Apps tile (tiles link, Ctrl K and the phone More list them) and plugin admin pages at /admin/x/<key>`

---

### Task 5: Clearance Go — plugin, migrations, store with the four changes

**Files:**
- Create: `plugins/clearance/{plugin.json,clearance.go,store.go,store_test.go,store_db_test.go}`, `plugins/clearance/migrations/{embed.go,0001_clearance.sql,0002_expiry.sql,0003_dead_stock.sql}` (byte-for-byte from karots-pos)

**Interfaces:**
- Produces (`package clearance`):
  ```go
  type Settings struct { StaleDays int; DefaultPercent, MinMarginPercent decimal.Decimal; ExpiryDays, DeadMonths int }
  func (s *Store) GetSettings(ctx) (Settings, error); func (s *Store) SaveSettings(ctx, Settings) error
  type Candidate struct { ProductID int64; Name, Unit string; OnHand, Cost, Price decimal.Decimal
  	DaysSinceSale *int; SoonestExpiry *time.Time; LotCost *decimal.Decimal // cost of the soonest-expiring live lot
  	Status string // "" candidate | "live" | "snoozed"
  	Percent *decimal.Decimal; SnoozedUntil *time.Time }
  func (s *Store) Candidates(ctx) ([]Candidate, error)   // candidates + live + snoozed
  func (s *Store) Dead(ctx) ([]DeadItem, error)          // as karots-pos DeadItems
  func (s *Store) Approve(ctx, productID int64, pct decimal.Decimal, userID int64) error
  func (s *Store) Snooze(ctx, productID, userID int64) error   // status 'dismissed', updated_at = now()
  func (s *Store) Unsnooze(ctx, productID int64) error          // DELETE the row
  func (s *Store) live(ctx, ids []int64) (map[int64]decimal.Decimal, error) // live markdowns only
  func suggestPercent(sell, cost, defaultPct, minMarginPct decimal.Decimal) decimal.Decimal // verbatim
  func newPrice(sell, pct decimal.Decimal) decimal.Decimal                                   // verbatim
  ```

**The rules (all in SQL, from `updated_at`):**
- **Live markdown** = `status='approved' AND discount_value > 0 AND stock.quantity > 0 AND NOT EXISTS (SELECT 1 FROM stock_batches b WHERE b.product_id = m.product_id AND b.created_at > m.updated_at)` (any lot — delivery, intake, count — means fresh stock). Not live → the row is ignored (it shows as an ordinary candidate if it still qualifies; Approve re-stamps `updated_at`).
- **Snoozed** = `status='dismissed' AND m.updated_at + (stale_days || ' days')::interval > now()`; past that the product is a candidate again (a restored karots-pos dismissal snoozes from its own `updated_at` — no mapping).
- **Lot floor:** `LotCost` = `cost_price` of the live lot with the soonest future expiry (`DISTINCT ON (product_id) … ORDER BY product_id, expiry_date, id`); the API uses it as the floor cost when the row is expiring.

- [ ] **Step 1: unit test** (`store_test.go`) — port karots-pos `store_test.go` (`suggestPercent` / `newPrice` table).
- [ ] **Step 2: failing DB test** (`store_db_test.go`, DATABASE_URL skip, runs `db.RunMigrationsTable(conn.DB, migrations.FS, "goose_db_version_clearance")`, creates its own product "Clr Test <nanos>" with a lot via SQL, cleans up in `t.Cleanup`):
  ```go
  func TestMarkdownLifecycle(t *testing.T) {
  	// approved on a product with stock → live
  	must(st.Approve(ctx, pid, d("20"), uid)); assertLive(t, st, pid, true)
  	// sold out → not live
  	exec(t, `UPDATE stock SET quantity = 0 WHERE product_id = $1`, pid); assertLive(t, st, pid, false)
  	// back in stock from a delivery after approval → still not live
  	exec(t, `UPDATE stock SET quantity = 5 WHERE product_id = $1`, pid)
  	exec(t, `INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, created_at) VALUES ($1,5,5,90, now())`, pid)
  	assertLive(t, st, pid, false)
  	// re-approve → live again
  	must(st.Approve(ctx, pid, d("10"), uid)); assertLive(t, st, pid, true)
  }
  func TestSnooze(t *testing.T) {
  	must(st.Snooze(ctx, pid, uid)); assertStatus(t, st, pid, "snoozed")
  	exec(t, `UPDATE clearance_markdowns SET updated_at = now() - interval '61 days' WHERE product_id = $1`, pid)
  	assertStatus(t, st, pid, "") // candidate again (stale_days 60, product unsold)
  }
  func TestLotCostFloor(t *testing.T) { /* two live lots: expiring in 5 days cost 80, fresh cost 95 → LotCost = 80 */ }
  ```
- [ ] **Step 3:** `go test ./plugins/clearance` → FAIL.
- [ ] **Step 4: implement** `store.go` from karots-pos `store.go` with the three rule changes above (the `StaleItems` query gains the snooze / live columns and the lot-cost subquery; `approvedMarkdowns` becomes `live`). `clearance.go`: `init() { plugin.Register(&Plugin{}) }`, `Name() "Clearance"`, `Key() "clearance"`, `Migrations() (migrations.FS, "clearance")`, `Setup` (routes + hooks in Task 6). `plugin.json`: `{"key":"clearance","name":"Clearance","import":"karots-suite/plugins/clearance","version":"2.0.0","description":"…"}` (description from karots-pos, plus the four changes).
- [ ] **Step 5:** `make plugins P=clearance && go test ./plugins/clearance ./internal/... && make core-only && make check-core`.
- [ ] **Step 6: commit** `feat(clearance): store — karots-pos migrations byte-for-byte; a markdown ends itself when its stock sells out or a fresh delivery comes, dismiss snoozes for one stale period, expiring lots floor at their own cost`

---

### Task 6: Clearance API + hooks

**Files:**
- Create: `plugins/clearance/api.go`, `plugins/clearance/api_test.go`
- Modify: `plugins/clearance/clearance.go` (`Setup`)

**Interfaces:**
- Consumes: Task 2 (`r.Admin()` mounts at `/api/v1/x`, admin + manager), Task 5 store.
- Produces (JSON, camelCase, money as strings):
  ```
  GET  /api/v1/x/clearance            → { settings, rows: Row[], dead: DeadRow[] }
       Row { productId, name, unit, onHand, cost, price, reason ("stale"|"expiring"), reasonLabel,
             status (""|"live"|"snoozed"), percent (suggested or approved), newPrice, floorCost, snoozedUntil? }
       DeadRow { productId, name, unit, monthsIdle, lastActivity (ISO date) }
  PUT  /api/v1/x/clearance/settings   { staleDays, defaultPercent, minMarginPercent, expiryDays, deadMonths } → settings
  POST /api/v1/x/clearance/:pid/approve  { percent }  → 204   (0 ≤ percent < 100, else 400)
  POST /api/v1/x/clearance/:pid/snooze   → 204
  POST /api/v1/x/clearance/:pid/unsnooze → 204
  POST /api/v1/x/clearance/disable    { productIds: number[] } → { disabled: n }  (1..500 ids)
  ```
  Types live in `api.go` (a plugin's DTOs are its own; `types.gen.ts` is core-only), mirrored by hand in `web/src/plugins/clearance/types.ts`.

- [ ] **Step 1: failing test** (`api_test.go`, httptest over a bare echo with the plugin's handlers): approve 120 → 400; approve 15 then GET shows `status:"live"`, `percent:"15"`; snooze then GET shows `status:"snoozed"`; `disable` with two dead fixture ids → `{disabled:2}` and both `products.is_active = false`; settings PUT with `staleDays: 0` → 400 (karots-pos silently reset to 60; the suite says what's wrong).
- [ ] **Step 2:** FAIL. **Step 3: implement** handlers (reason label logic and ceil-of-days from karots-pos `admin.go`; floor cost = `LotCost` when expiring else `Cost`; `disable` loops `core.Products.Delete` and stops at the first error with `apperr`), and `Setup`:
  ```go
  reg.AddDataGroup(plugin.DataGroup{Key: "clearance", Name: "Clearance / markdowns", Tables: []string{"clearance_markdowns", "clearance_settings"}})
  reg.AddProductBadgeProvider(plugin.ProductBadgeProvider{Batch: p.store.BadgesFor})               // live only
  reg.AddProductSaleSuggestionProvider(plugin.ProductSaleSuggestionProvider{Batch: p.store.SuggestionsFor})
  reg.AddProductDetailContributor(plugin.ProductDetailContributor{Rows: p.detailRows})
  a := reg.Admin()
  a.GET("/clearance", h.page); a.PUT("/clearance/settings", h.saveSettings)
  a.POST("/clearance/:pid/approve", h.approve); a.POST("/clearance/:pid/snooze", h.snooze)
  a.POST("/clearance/:pid/unsnooze", h.unsnooze); a.POST("/clearance/disable", h.disable)
  ```
  Approve / snooze / disable / settings write an audit entry through `core.Audit` (same action kinds core uses for settings / product changes).
- [ ] **Step 4:** PASS with `make plugins P=clearance`; `make core-only && make check-core`.
- [ ] **Step 5: commit** `feat(clearance): JSON API at /api/v1/x/clearance (admin + manager) and the till hooks — badge, apply / skip prompt and info row show only live markdowns; dead stock disables one or many`

---

### Task 7: Clearance screen (UI gate)

**Files:**
- Create: `web/src/plugins/clearance/{index.tsx,ClearancePage.tsx,MarkdownList.tsx,DeadList.tsx,RulesSheet.tsx,api.ts,types.ts}`

**Interfaces:**
- Consumes: `slots.apps`, `slots.adminPages` (Task 4); API (Task 6); admin kit (`PageHead`, `DataList`, `FormSheet`, `ConfirmButton`, `EmptyState`, `SURFACE`), `api<T>` from `@/api/client`.
- Produces: `export default function register(s: typeof slots)` pushing
  `{ key: 'clearance', label: 'Clearance', description: 'Mark down slow and expiring stock', icon: Tags, color: 'var(--brand)', to: '/admin/x/clearance' }` and `{ key: 'clearance', Page: ClearancePage }`.

- [ ] **Step 1:** before building, read karots-pos `plugins/clearance/pages.templ` once more and list every field / action; the screen must carry all of them (stale days, reason chip, on hand + unit, cost, price, new price live as the % is typed, approve, dismiss→snooze, dead list with idle + last activity, disable, the five rules).
- [ ] **Step 2: build.** `PageHead` "Clearance" (sub: counts "12 to mark down · 3 live · 5 dead"), actions: **Rules** (opens `RulesSheet`, a `FormSheet` with the five numbers and one line under each saying what it does). Tabs (`?tab=markdown|dead`, URL search like the reports kit): 
  - **To mark down** — filter chips All · Live · Snoozed; `DataList` columns: product (+ unit), reason chip (amber `--warn` token for expiring, muted for stale), on hand, cost (floor note "floor Rs 84 · lot cost" when expiring), price → new price (computed client-side with the same 2dp rounding as `newPrice`), % input with Approve; row menu Snooze / Unsnooze. Live rows show a "Live −15%" chip. Cards at 360px.
  - **Dead stock** — `DataList`: product, idle months, last activity; Disable per row (`ConfirmButton`) and **Disable all shown (n)** in the list footer (`ConfirmButton`, body "n products will be hidden from the till and lists. You can bring each back from Products → Show disabled.").
  - Empty states say what would put something here ("Nothing has gone 60 days without a sale.").
  - Mutations invalidate `['x','clearance']`; toasts via sonner; errors show `ApiError.message`.
- [ ] **Step 3:** `bunx tsc -b`, `bun run test`, `make check-core`.
- [ ] **Step 4: UI gate.** `make plugins P=clearance && make build && make core-only`, `make e2e-seed`, run the binary on :3100, approve a markdown on a seeded product. Screenshots: the page (both tabs, Rules sheet) at 1366 and 360, the till card badge and the apply prompt, in Studio + Night × light + dark (system user → Appearance; restore Studio after). Review: no side scroll at 360, card contents inside their edges (figures `nowrap` don't overflow), tokens only, matches the admin look of the M3–M6 pages. Fix and re-shoot until right.
- [ ] **Step 5: commit** `feat(clearance): Apps → Clearance — to-mark-down list with live new price, approve / snooze, live and snoozed filters, dead stock with disable all, rules sheet; phone cards at 360px`

---

### Task 8: e2e, restore, parity, docs, review

**Files:**
- Create: `web/e2e/plugin-clearance.spec.ts`
- Modify: `docs/parity.md` (5 clearance rows), `docs/status.md`, `docs/differences.md` (clearance's four changes), `AGENTS.md` (Layout: `plugins/`, `cmd/bootstrap`, `make plugins / core-only / check-core`, "plugin routes `/api/v1/x`, pages `/admin/x/<key>`"), `internal/backup/restore_compat_db_test.go` (case)

- [ ] **Step 1: e2e** (import `test` from `./fixtures`): admin opens Apps → Clearance (from the dock tile); a seeded product with stock and no sales appears with reason "never sold"; type 15 → new price updates; Approve → "Live −15%"; cashier till: product card shows "Clearance −15%", adding it shows the prompt, Apply puts a 15% line discount; sell the remaining stock → admin reload: the row is no longer Live; Snooze another → only under the Snoozed filter; Dead stock: Disable all shown → confirm → list empty and the products gone from the till search; page at 360 has no horizontal scroll (`scrollWidth <= clientWidth`). The seed gets what it needs via SQL in `beforeAll` (a product with `created_at` 7 months back and zero stock; a product with stock and no sales) — never touches products other specs use.
- [ ] **Step 2:** full run: kill :3100 / :5173, `make e2e-seed && cd web && bunx playwright test` — every earlier spec still green with plugins built in.
- [ ] **Step 3: restore.** In `restore_compat_db_test.go`, a case (skips when the plugin isn't compiled in) restoring a karots-pos-shaped archive holding `clearance_markdowns` (one approved, one dismissed 10 days ago) + `clearance_settings`: rows load, the dismissed one reads as snoozed, the approved one is live only if no lot is newer than its `updated_at`. Also run the user's real backups (`TestRestoreKarotsPosBackup`, opt-in) with clearance built in.
- [ ] **Step 4: parity** — tick `GET /clearance`, `POST /clearance/:pid/approve`, `/dismiss` (→ snooze), `/disable`, `/settings` with "→ /api/v1/x/clearance…" notes.
- [ ] **Step 5: milestone review.** `make verify` (check-core, vet, core-only `go build` + `tsc -b`), `go test ./...` with plugins in and core-only, `bun run test`, Playwright, `make build && make verify-portable`, read the whole diff, every hand-written file ≤ 500 lines. Fixes in one `fix:` commit.
- [ ] **Step 6: docs** — `status.md`: M7.0 done, the slot table (old hook → slot), the deferred seams (`TenderMethod.Options` / `NotAtTill`, `SaleLineHook`, credit plugin methods + `paid_into`, supplier-action / readiness-edit / product-row slots) as M7.1+ inputs, NEXT = M7.1 requests (rethink first). `differences.md`: the four clearance changes. Commit `docs: M7.0 done — plugin groundwork + clearance`.
