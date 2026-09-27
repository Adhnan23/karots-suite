# Karots Business Suite — Design Spec

- **Date:** 2026-09-27
- **Status:** Draft, awaiting review
- **Repo:** `~/Projects/go/karots-suite` (Go module `karots-suite`, binary `karots-suite`)
- **Product name:** Karots Business Suite

## 1. Why

karots-pos works and ships, but its web layer (Templ + HTMX + Alpine) blocks where we need to go:

- **Every client looks the same.** We can only swap colour, radius, density and receipt style. New clients want a system that doesn't look like any other shop's. That needs real components and layouts, and server-rendered partials can't give us that.
- **The admin can't be used on a phone.** karots-pos is intentionally desktop-only. Owners want to check reports, stock and activity from anywhere.
- **It isn't component-based.** Many screens are near-copies of each other with small drifts, so a fix in one isn't a fix in all of them.
- **Too much is server-side.** Filtering, sorting, previews and totals round-trip to the server when they could be instant on the client, with animation.
- **Charts are hand-rolled SVG.** They're hard to customise, and there's no hover, zoom or drill-down.

karots-suite is a **separate product**. It has every feature and every plugin karots-pos has, on a JSON-only Go backend with a component-based React frontend. After this, karots-pos is frozen and gets bug fixes only.

## 2. Goals and non-goals

**Goals**

1. **Full feature parity with karots-pos:**
   - core till and admin
   - every report
   - all 10 plugins (recharge, pharmacy, repairs, documents, alternatives, clearance, aicatalog, productplus, combos, requests)
   - bootstrap and the Demo Room
2. **A JSON-only backend.** No HTML is rendered anywhere in Go, except thermal and print documents (see §8).
3. **A component-based React frontend** with a design system. Screens are composed from shared components, never copy-pasted.
4. **A fully mobile-responsive admin.** Every admin page, report and chart works at 360px width.
5. **The till stays desktop and kiosk-first.** It's keyboard-driven and barcode-scanner-driven, and it auto-fits low resolutions.
6. **Still a single self-contained binary.** Frontend build, migrations and assets are all embedded, and it stays static and portable (`CGO_ENABLED=0 GOAMD64=v1`).
7. **One new visual theme** that does not look like karots-pos.

**Non-goals (for now)**

- **Multiple theme packs and per-client component overrides.** The component and token structure must allow them later (§6.4), but we build none now.
- **A separate mobile app.** The responsive web admin replaces that need.
- **Offline or PWA till.** The server runs on the counter, as it does today.
- **Changes to domain behaviour.** Money, stock, credit and lot logic is forked as-is. Bugs found during the port get fixed in both repos.

## 3. Starting point: fork, not rewrite, the domain

karots-pos is about 67k lines of Go, 24k lines of templ and 4.7k lines of JS. It has 75 migrations and about 560 HTML routes (339 core + 223 plugin). The business logic (`internal/features/*`) is battle-tested on the money path and is UI-agnostic.

**Forked unchanged:**
- `internal/features/*`
- `internal/db`, `internal/config`, `internal/datetime`, `internal/backup`, `internal/escpos`, `internal/receiptimg`, `internal/demogen`, `internal/demodb`
- `migrations/`
- the Go side of every plugin (services, repositories, migrations)
- `cmd/server`, `cmd/bootstrap`, `cmd/demo`, `cmd/demogen`
- all `_test.go`

**Removed:**
- `internal/web/*`
- `templates/`
- `static/js`, `static/css`
- every plugin's templ and HTML handlers
- the Tailwind CLI step

**Rewritten:**
- the HTTP layer, as JSON handlers (`internal/api/*`)
- the UI hooks in `internal/plugin` (§7)
- the whole frontend (`web/`)

**The schema stays identical to karots-pos**, and so do the goose version tables per plugin. A karots-pos shop moves to karots-suite with a karots-pos backup restored into karots-suite. The backup format is shared because `internal/backup` is forked with dynamic table and column discovery.

## 4. Architecture

```
karots-suite/
  cmd/server/            boot, migrations, wiring, flags (-migrate -seed -demo -reset -init)
  cmd/bootstrap/         per-shop build: bakes Go plugin set + frontend plugin set + support seed
  cmd/demo, cmd/demogen  Demo Room (embedded Postgres, profiles, embedded printer)
  internal/features/*    forked domain services
  internal/api/          NEW JSON HTTP layer, one file per area (§5)
  internal/plugin/       forked registry; data/money hooks unchanged, UI hooks → slot manifests (§7)
  internal/middleware/   forked guards (JWTAuth, RequireRole, RequireUnlocked, RequirePinChosen,
                         RequireSystemUser, RequireSupplierAccess, RequireKioskExit)
  internal/escpos, receiptimg, backup, db, config, datetime   forked
  migrations/            forked, same schema
  plugins/<name>/        Go side forked; + web/ (React half, §7)
  web/                   Vite + React + TypeScript app (§6)
    dist/                build output, go:embed'd (gitignored; built by make)
  docs/specs, docs/plans
```

Rules carried over from karots-pos, unchanged:

- **Core is plugin-inert.** Core Go never imports a plugin, and neither does the core frontend (`web/src` outside `web/src/plugins`). Grepping core for a plugin name gives zero hits.
- **Plugins attach only through hooks and slots** registered at the composition root, and are enabled at compile time.
- **Money moves only through `cashflow.Move`/`MoveTx`.**
- **Sales are the source of truth for revenue.**
- **Migrations are idempotent. Postgres-only SQL is deliberate.**
- **Files are organised by concern** and each stays at 500 lines or fewer. This applies to both Go and TS.

## 5. Backend: JSON API

### 5.1 Shape

- **Mounting and naming.** Echo, all under `/api/v1`. Handlers live in `internal/api/<area>.go` (`sales.go`, `products.go`, `reports_sales.go`, …). Routes are registered per area in `register_<area>.go`.
- **Responses:**
  - success: plain JSON
  - lists: `{items, total, page, pageSize}`
  - errors: `{error: {code, message, fields?}}`, mapped from the existing `apperr.Validation/Conflict/ConflictCode/Internal`
- **Money.** Sent as strings in the existing `money.Format` decimal form, never as floats. The frontend formats with a single `money` helper.
- **Dates.** Sent as ISO 8601, using the shop timezone via `internal/datetime`.
- **Types.** Request and response structs live in the handler files. **tygo** generates `web/src/api/types.gen.ts` from them. `make types` regenerates the file, and CI fails if it's stale. There's no OpenAPI or generated client: a single typed `api<T>()` fetch wrapper is enough.
- **Exports.** CSV, XLSX and ODS exports stay server-side (`internal/api/csv.go`, forked) as download endpoints. PDF and A4 prints are covered in §8.

### 5.2 Auth and session

- **Login.** It returns an **httpOnly, SameSite=Strict JWT cookie**, keeping today's claims (role, locked, pin-chosen, kiosk). The cookie plus SameSite=Strict and a same-origin-only API is the CSRF defence. No tokens are kept in JS.
- **Current user.** `GET /api/v1/me` returns the user, role, permissions (can_see_cost, can_manage_credit, supplier access, cashier_access, …), enabled plugins, the slot manifest (§7) and settings.
- **Hidden system user.** Phone `0000000001`, with the rotating hourly PIN (install ID + HMAC(seed, hour)), `POS_SYSTEM_PIN` override and 2273 dev fallback. It stays invisible to the shop and excluded from Activity and audit.
- **Screen lock and PIN.** Screen lock, PIN-chosen and kiosk-exit flows are the same middleware, returning JSON 401/403 with a `code` the frontend routes on.

### 5.3 Static serving

- `web/dist` is embedded. Any non-`/api` GET falls back to `index.html` so SPA routing works.
- Hashed assets get long-lived cache headers. `index.html` gets no-cache.

## 6. Frontend

### 6.1 Stack

| Need | Choice |
|---|---|
| Build, package manager | Vite, TypeScript strict, bun |
| Routing | TanStack Router, file-based, typed routes and typed search params. Report filters and date presets live in the URL, so they're shareable and survive a reload. |
| Server state | TanStack Query: cache, invalidation, optimistic till updates |
| Tables | TanStack Table: sort, filter, paginate, column visibility; mobile renders as cards |
| Forms | TanStack Form + Zod |
| UI primitives | Tailwind v4 + shadcn/ui (Radix), lucide icons, sonner toasts, cmdk command palette |
| Charts | ECharts behind one `<Chart>` wrapper |
| Animation | Motion |
| Tests | Vitest (logic, money maths, cart), Playwright (flows) |

Per the global instructions, run `intent list` and load the matching TanStack skills before implementing.

### 6.2 Layout of `web/src`

```
api/          client (api<T>(), error mapping), types.gen.ts, query keys
ui/           design-system primitives (shadcn-owned): Button, Card, Dialog, Sheet, DataTable,
              MoneyInput, DatePresetPicker, EntityPicker, OptionPicker, CategoryPicker, LotPicker,
              StatTile, Chart, EmptyState, ConfirmDialog, PrintPrompt, MoneyDirection (↓ green in / ↑ red out)
theme/        tokens (CSS variables) + the single theme's shell components
lib/          money, dates, keyboard/hotkeys, konami, barcode, kiosk autofit
features/
  auth/       login (+ install-ID Konami), PIN, lock screen
  till/       cashier (§6.5)
  admin/      one folder per area (products, stock, purchases, suppliers, customers, money,
              reports, activity, users, settings, data, …)
  system/     system-user-only menu (§6.4)
plugins/      enabled-plugins.ts (generated by bootstrap) + slot registry
routes/       TanStack Router file routes
```

**Components rule.** When a pattern appears on two screens, it becomes a `ui/` or feature-level component with props. We never copy it with tweaks. This is the direct fix for the karots-pos drift problem.

### 6.3 Responsiveness

- **Admin is mobile-first:**
  - bottom tab bar plus a "More" sheet under 768px, sidebar at 768px and up
  - tables collapse to cards
  - dialogs become bottom sheets
  - charts resize and use touch tooltips
  - every admin flow is usable on a phone, including approvals, reorder and add product
- **The till is desktop and kiosk.** It keeps today's low-resolution auto-fit for sub-1080p terminals. It isn't designed for phones.

### 6.4 Theme and appearance

- **One new theme.** It deliberately doesn't resemble karots-pos: new shell, typography, spacing and card language. The visual design is done as its own step in M0, using the frontend-design skill with mockups reviewed before building.
- **Everything visual comes from CSS-variable tokens:** colour, radius, density, font and shadow. Components never hard-code colours. Charts read their palette from tokens. Light and dark mode are both supported.
- **System menu → Appearance** is visible only to the system user. It covers brand colour, radius, density, light/dark, logo, shop name and receipt style (classic/modern/boxed/compact, same as today). It's stored in the existing settings table and applied at boot, with no rebuild.
- **Future packs (not built now).** Screens consume shell and variant components through `theme/`, never directly. A second theme later means adding `theme/<pack>/` and a selector in the system menu, with no screen changes.

### 6.5 Till (cashier) parity checklist

Everything from karots-pos, including:

- **Sale:** search and scan (two-field barcode/name), group cards menu, frequently-sold grid, ⓘ product info popup (cost gated by `can_see_cost`)
- **Stock prompts:**
  - FEFO lots, dual-price "which price?" prompt, expiry warning (sell anyway / write off / back)
  - found-at-till correction, including recipe and service lines
  - clearance suggestions
- **Payment:**
  - tenders and split payments, card and online routed to a picked account
  - credit as a payment line, credit over-limit override, advance customers and refunds
  - change-kept rounding, instant change display, single-job non-blocking print
- **Held sales, returns, warranty by receipt, damage, conversions, labels**
- **Drawer:**
  - open/close with side-by-side float, shared drawer "continue last"
  - cash in/out with the money-direction cue, default locker, untracked cash (out only), drawer kick
- **Supplier actions at the counter** (pay/receive/order/expenses, behind the per-user flag)
- **Credit collection** (name and phone search), cashier receipts
- **Hotkeys:**
  - F1 / "⟲ Menu" = reset, which clears the **search only, not the cart**. Only New Sale clears the cart.
  - F10 → amount → Enter
  - scroll-wheel guard on numeric inputs
- **Kiosk:**
  - lockdown
  - Alt+F4 relaunch (the kiosk.sh/.cmd scripts are forked)
  - admin/support **Konami code → kiosk exit**

### 6.6 Hidden and system behaviours (must port)

- **Login page Konami → reveals install ID.** This supports the rotating support PIN and the PIN Vault app.
- **Kiosk Konami → `/kiosk/exit`,** for admin and support only.
- **System user menu,** reachable only by the system user: appearance, support tools, data recovery, and anything that's system-locked in karots-pos today.
- **Activity and audit views never show the system account.**

## 7. Plugins in the new shape

Each plugin keeps its Go package, including services, repositories, migrations, its own `goose_db_version_<suffix>` table, `Setup` and data/money hooks. It gains a React half:

```
plugins/<name>/
  *.go                      forked Go side; HTML handlers rewritten as JSON under /api/v1/p/<name>/
  migrations/
  web/index.ts              registers into slots
  web/…                     its screens/components (may import web/src/ui, never core features)
```

**The slot registry (`web/src/plugins/slots.ts`)** replaces today's templ UI hooks. The slots are typed:

- `routes`, `adminNav`, `tillMenu`, `paletteEntries`, `dashboardWidgets`, `reportCards`
- `receiptTabs`, `drawerSections`, `productPanels` (the product form/detail extension, used by productplus and pharmacy)
- `productRowActions`, `tillProductBadges`, `tillActions`, `cashierSupplierActions`, `dataGroups`

**Go hooks.** Hooks that carry data or money stay as they are: tender methods, money-location and readiness providers, activity contributors, search contributors, `PLIncome`, the `plugin.Core.Stock/Cashflow/Lockers` seams, `sales.WithNonCashLeg`, `cashregister.WithLockerLeg` and `purchases.WithSettleCredit`. Hooks that used to return templ components now return **JSON data**, and the plugin's React slot component renders it.

**Bootstrap.** `make bootstrap` writes both `cmd/server/enabled_plugins.go` and `web/src/plugins/enabled-plugins.ts`. Disabled plugins aren't in the binary or the bundle. The Demo Room enables all of them, and the per-profile visibility filter (`plugin.SetVisibilityFilter`) is forwarded in `/me`.

## 8. Printing and documents

- **Thermal receipts stay in Go:** `internal/escpos` and `receiptimg`, the TCP printer, Windows winspool RAW and the drawer kick. They're triggered by API calls exactly as today. The frontend only shows the print prompt and a preview (a receipt-image PNG endpoint).
- **A4 and PDF documents** (PO/GRN, statements, low-stock, count sheet, labels) become **React print routes** with print CSS, which reuse the components. The one exception is label barcodes, which are generated client-side.

## 9. Testing

- **Go:** all forked tests keep passing (`make test`). Every new JSON handler area gets a handler test for status codes and JSON shape. DB tests stay gated on `DATABASE_URL`.
- **Frontend:**
  - Vitest for money formatting and parsing, cart and tender maths, and hotkey logic
  - Playwright for each milestone's golden flows: sale with cash, credit, split and return; drawer open/close; receive a PO; each report renders; mobile admin at 360px
- **Parity:** a script lists every karots-pos route into `docs/parity.md`. Each row gets ticked when it's ported and has a flow check. A milestone is done only when all its rows are ticked.

## 10. Build and release

- `make dev` runs the Go server (air) and Vite dev server together, with Vite proxying `/api`.
- `make build` runs `make types`, then `bun run build` in `web/`, then `go build` (static). `make verify-portable` is forked.
- `make demoroom-build` is forked. Its output includes the frontend.
- The only Node/bun dependency is at build time. The shipped binary has none.

## 11. Milestones

Each milestone gets its own implementation plan and goes on its own branch, with one commit per step.

| # | Milestone | Done when |
|---|---|---|
| M0 | **Foundation:** fork Go domain; strip web/templ; JSON skeleton + error model + tygo; Vite app embedded in the binary; login (+install-ID Konami), JWT cookie, roles, lock, PIN; system user + system menu; kiosk + Konami exit; theme design + design system `ui/`; parity list generated | binary boots, logs in, shows the empty shell on desktop and phone; `make test` green |
| M1 | **Till:** full §6.5 parity | Playwright till flows pass; parity rows ticked |
| M2 | **Catalog and stock:** products, categories (nested), groups, units, services & recipes, intake, stock-take (+toggle), stock movement, batches, labels, conversions, damage | rows ticked |
| M3 | **Purchasing and people:** suppliers (+statements, money flows, receiving discounts, returns), POs → receive/GRN, reorder, customers (+statements, credit, opening balance), users/permissions | rows ticked |
| M4 | **Money:** cashflow, lockers, expenses (+categories), bill-pay/get-money, settle card, receipts hub (S-/CR-/DP-/warranty), finance hub | rows ticked |
| M5 | **Reports and dashboard:** dashboard (carousel, vs-yesterday), every report (sales, product sales, trend, peak hours, profit by category, finance/P&L, tax, dues, losses & recovery, returns, inventory/valuation, count sheet, top-up & reorder, cashier expenses, recipes, …), Activity, audit; ECharts; CSV/XLSX/ODS | every report renders on desktop and 360px |
| M6 | **Data and settings:** backup/restore, auto-backups, exports, imports (products/customers/suppliers), settings, appearance, setup/init, catalog sync endpoint | karots-pos backup restores into karots-suite |
| M7 | **Plugins:** all 10 ported into slots, one at a time (one plan each or grouped) | each plugin's flows pass; core grep is plugin-free |
| M8 | **Ship:** bootstrap (Go + TS plugin sets), Demo Room, release builds, install scripts, README/AGENTS.md | `make build` + `verify-portable` pass; Demo Room runs zero-env |

## 12. Risks

- **Scope.** About 560 routes is a lot. Mitigations: the parity list, one milestone at a time, and the domain being forked rather than rewritten.
- **Two repos drift on domain bugs.** A domain bug found in either repo gets fixed in both while karots-pos is still supported.
- **Till regressions on the money path.** The till UI is new code. Mitigation: Playwright golden flows are written against karots-pos behaviour, and M1 isn't done until they pass.
- **Bundle size on old counter PCs.** Mitigations: route-level code splitting (TanStack Router), ECharts lazy-loaded on report routes only, and the till bundle kept lean.
