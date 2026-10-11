# M7.0 — Plugin groundwork + Clearance (design)

Date: 2026-10-11. Status: for review. Next: `docs/plans/2026-10-11-m7-0-groundwork-clearance.md`.

M7.0 builds the path every plugin uses (build, Go seams, React slots) and proves it with a real
plugin, **Clearance** (user, 2026-10-11: fold clearance into M7.0; no throwaway fixture plugin).
M7.1 is then requests, M7.2 productplus, and so on in `docs/status.md` order.

## 1. The rule (user, 2026-10-11)

**Core never references a plugin, and core never changes for one plugin — only generic hooks.**

- Core = everything outside `plugins/` and `web/src/plugins/`. It never imports, names, or
  documents a plugin — not even as "e.g." in a comment or a validation list.
- When a plugin needs something from core, core gets a **generic seam** (a hook in
  `internal/plugin`, a slot in `web/src/plugins/slots.ts`) that is named and justified without
  the plugin. Plugin → core (read-only, through `plugin.Core`) and plugin → plugin imports are fine.
- **Enforced:** `make check-core` greps core (Go + `web/src` outside `web/src/plugins`) for every
  plugin key listed in `plugins/*/plugin.json` plus their tender values (`wallet`, `deposit`), and
  fails on any hit. It runs in `make verify`. Plain English words that happen to match (a browser
  "reload", the lucide `Wallet` icon) are excluded by an explicit allow-list in the script, each with
  a reason.
- **Today's violations, removed in M7.0:** `sales.PaymentInput` validates `oneof=… wallet …`;
  comments naming recharge / wallet / clearance / photocopy / reload / "the recharge plugin" in
  `internal/app/services.go`, `internal/features/sales/{service,service_types,service_lines,sales}.go`,
  `internal/features/products/{products,service,service_write}.go`, `internal/api/dto_reports_ops.go`.
  Rewritten generically ("a plugin tender", "a pass-through line").

## 2. Layout

```
plugins/<key>/                 Go half (module karots-suite/plugins/<key>), as karots-pos
  plugin.json                  key, name, version, description
  <key>.go                     init() → plugin.Register; Setup wires hooks + routes
  migrations/                  own goose set, table goose_db_version_<key>
web/src/plugins/<key>/
  index.tsx                    export default function register(s: Slots): void
web/src/plugins/slots.ts       typed slots (exists) + adminPages, apps
web/src/plugins/enabled-plugins.ts   generated; committed core-only (imports nothing)
cmd/server/enabled_plugins.go        generated; committed core-only
```

Approach A of three considered (B: React half inside `plugins/<key>/web` via a Vite alias —
breaks the AGENTS.md layout and Tailwind / tsconfig paths; C: generated TanStack file routes per
build — codegen for little gain).

## 3. Build — `cmd/bootstrap`

Ported from karots-pos `cmd/bootstrap` (466 lines), cut to what the suite needs now:

- Discovers `plugins/*/plugin.json`. `-plugins a,b|all`, `-os`, `-arch`, `-name`, `-out`, `-yes`,
  `-install-id`, `-X` (kept for M8's release tool).
- Writes **both** generated files (blank imports in Go; `import register from './<key>'` + a call
  in TS), runs `make build` (so the SPA is embedded), copies the binary to `-out`, then **restores
  both files to core-only** — also on failure (defer).
- `-write-only` writes the two files and stops — for dev. `make plugins P=clearance` (or `P=all`)
  wraps it; `make core-only` restores them. `make check-core` also fails if either committed file
  is not core-only, so a dev selection can't be committed by accident.
- **Playwright** builds with every ported plugin (`P=all`); core specs must still pass with
  plugins in (plugins are additive). `make verify` additionally builds core-only (`go build` +
  `bunx tsc -b`) so a core that only compiles with a plugin present is caught.

## 4. Go seams

`internal/plugin/hooks.go` keeps only **data / behaviour** hooks. The templ-era UI hooks are
deleted because their React slots replace them: `AdminNavEntry`, `CashierTab`, `PaletteEntry`,
`ReportCard` (Go), `ProductRowAction.Href`, and any other hook whose only output was HTML / a URL
for a templ page. Each deletion is checked against `docs/parity.md` so no plugin capability loses
its path; the plan lists the old hook → new slot for each.

- **Routes:** a plugin's JSON API is mounted under `/api/v1/x/<key>/…`, using the Mux's existing
  scoped groups with the same guards as core (`Admin` = admin + manager, `Cashier` = protected).
  Errors through `apperr` like core.
- **Wiring that exists in core but is not connected:** `products.BadgeProvider`,
  `products.SaleSuggestionProvider` and the product detail rows are declared but nothing feeds them
  from `plugin.ProductBadgeProviders()` etc. M7.0 connects them in `internal/app/services.go`
  (generic: merge every provider's result).
- **`/me.plugins`** returns the **keys** of the plugins whose contributions are visible (the demo
  visibility filter applies), not `Name()`.
- **Money correctness (2026-10-09 inputs):** an unknown tender method is a validation error before
  any payment row is written (the `oneof` list becomes core methods + `plugin.TenderMethods()`
  values, checked in the service); `pluginTenderLeg` refuses a method no plugin settles instead of
  returning nil; `plugin.Core.Sales` is the wired `svc.Sales`.
- **Deferred to the first plugin that uses them** (each lands with that plugin's tests, still as a
  generic seam): `TenderMethod.Options` / `NotAtTill`, `SaleLineHook`, credit repayments taking a
  plugin method with `paid_into`. Building them now would be dead code with nothing to prove them.

## 5. React slots

- **New `adminPages`:** `{ key, Page }` served at `/admin/x/$key/*` by one catch-all file route
  (the till's `/till/x/$key` pattern); the Page reads the splat for its own sub-paths.
- **New `apps`:** `{ key, label, icon, blurb, href }` → the dock's Apps tile (`nav.apps` exists,
  empty) and the Ctrl K list. The Apps tile shows only when it has an entry.
- Every registration carries its plugin key; core renders only slots whose key is in
  `/me.plugins`. The till's existing slots (`tillMenu`, `tillTenders`, …) get the same filter.
- Plugin pages are built from the admin kit (`PageHead`, `DataList`, `FormSheet`, …) and theme
  tokens only, so they work in both palettes × both modes and at 360px like core pages.

## 6. Clearance

### What karots-pos does (kept)
Flags **stale** products (in stock, no sale in `stale_days`, default 60) and **expiring** ones (a
live future lot within `expiry_days`, default 14; already-expired lots stay the Batches write-off's
job). Suggests a markdown % floored at cost × (1 + `min_margin_percent`), default 20% / 5%; the
admin approves (editable %) or dismisses. An approved markdown shows at the till as a card badge
("Clearance −20%"), an apply / skip prompt when added (core per-line discount — no new discount
machinery), and a row in the ⓘ product info. **Dead stock:** active, non-service, zero stock, no
movement / sale / creation for `dead_months` (default 6) → Disable (core soft-delete, reversible).
Nothing is ever disabled automatically. Thresholds are editable.

### What changes (user, 2026-10-11 — all four chosen)
1. **The markdown ends itself.** It is live only while the product has stock **and** no stock lot
   arrived after `approved_at`. When the old stock sells out or a fresh delivery comes in, the
   badge and prompt stop; if the product is still slow it reappears as a candidate. Computed live
   in the badge / suggestion query (no cron, no core change).
2. **Dismiss = snooze.** Dismiss sets `snoozed_until = now() + stale_days`; the product reappears
   after that if it still qualifies. Undo-able from a "Snoozed" filter.
3. **Disable all shown** on the dead-stock list, behind a confirm naming the count. Each product
   goes through `plugin.Core.Products` disable, the same as one-by-one.
4. **Lot cost floor.** When the reason is an expiring lot, the floor uses that lot's own cost
   (`stock_batches` cost); otherwise the product's cost as before.

### Data and old backups
- karots-pos migrations `0001_clearance`, `0002_expiry`, `0003_dead_stock` are copied
  **byte-for-byte** (same goose versions, table `goose_db_version_clearance`), so a karots-pos
  backup's clearance tables restore as-is.
- `0004_lifecycle.sql` adds `approved_at TIMESTAMPTZ` and `snoozed_until TIMESTAMPTZ`, backfilled:
  approved rows → `approved_at = updated_at`; dismissed rows → `snoozed_until = updated_at +
  stale_days`. A restored karots-pos backup lacks these columns; M6 restore loads common columns
  and the defaults apply — the plan adds a restore-time backfill step via `plugin.DataGroup` if the
  defaults alone aren't right (checked with a test on a backup holding clearance rows).
- `plugin.DataGroup{Key: "clearance", Tables: clearance_markdowns, clearance_settings}`.

### Screens
- **Apps → Clearance** (`/admin/x/clearance`), admin + manager. `PageHead` with two tabs:
  **To mark down** (candidates + live markdowns; reason chip "60 days no sale" / "expires in 9
  days"; cost, price, editable %, new price; Approve / Snooze; filter All · Live · Snoozed) and
  **Dead stock** (idle months, last activity; Disable; Disable all shown). **Rules** sheet holds the
  five thresholds. Table on desktop, cards at 360px (`DataList`).
- **Till:** unchanged code paths — badge on `ProductTile`, prompt via `Suggestion.tsx`, ⓘ row —
  fed by the newly connected hooks.

### API (`/api/v1/x/clearance`, admin group)
`GET /` (rows + dead + settings), `PUT /settings`, `POST /:pid/approve {percent}`,
`POST /:pid/snooze`, `POST /:pid/unsnooze`, `POST /disable {productIds}` (one or many). Maps the
five karots-pos parity rows (GET page, approve, dismiss→snooze, disable, settings).

## 7. Testing

- **Go:** `suggestPercent` / `newPrice` table test (from karots-pos); DB tests for each change —
  markdown ends on sell-out and on a later delivery, snooze hides then returns, bulk disable,
  expiry floor uses the lot cost; badge / suggestion wiring returns plugin results through core;
  unknown tender method refused before any row is written.
- **Build:** `make check-core` (zero plugin names; committed generated files core-only);
  core-only build compiles; bootstrap round-trip restores both files even when the build fails.
- **Playwright** (`plugin-clearance.spec.ts`, plugins built in): approve → till badge + prompt
  applies the discount → sell the stock out → badge gone; snooze; disable all shown; the page at
  1366 and 360 with no side scroll. All earlier specs still green with plugins in.
- **UI gate:** screenshots of the Clearance page and the till badge / prompt in Studio + Night ×
  light + dark, self-reviewed.
- **Restore:** a karots-pos backup with clearance rows restores and the page shows them.
- Parity: the 5 `plugins/clearance` rows ticked.

## 8. Out of scope

Lot-only discounts at the till (the till discounts per line, not per lot — would need a core
seam with no second user yet); a clearance-recovered report; the deferred money seams in §4; every
other plugin.

## 9. Changed while planning (2026-10-11)

- **No `0004` migration:** `clearance_markdowns.updated_at` already is the approval / dismiss time,
  so "ended" and "snoozed" are computed from it and a karots-pos backup restores with no mapping.
  §6 "Data and old backups" is superseded by this.
- **Plugins weren't started at all:** `SetupAll` is never called and the `Mux` never mounted —
  M7.0 wires both (plan Task 2).
- **No SPA visibility filter:** nothing in the suite sets `plugin.SetVisibilityFilter` (no demo
  profiles), so `/me.plugins` just lists keys; §5's filter returns only if Demo Room needs it.
