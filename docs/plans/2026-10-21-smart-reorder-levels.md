# Smart reorder levels + Low stock at the till — Implementation Plan

**Spec:** `docs/specs/2026-10-21-smart-reorder-levels-design.md` (mockup linked there).

**Goal:** Auto reorder levels learned nightly from sales speed, supplier rhythm and last year; the Reorder page says
why; a cashier with the new "Low stock" permission sees the list at the till and orders through the Suppliers tab.

**Architecture:** `reorder_level` stays the effective level every reader uses; a new core package
`internal/features/reorder` computes `reorder_calc` (+ the level for Auto products) in a daily loop. One shared row
builder serves `/admin/reorder` and `/till/low-stock`.

**Constraints:** core only, plugin-inert; ≤ 500-line files; theme tokens only; 360 px + Studio/Night × light/dark;
no Claude attribution; one commit per task; `make types` after `dto*.go`; never `go test` during Playwright.

## Review Focus

- A product sold only in a week it was out for half → speed from in-stock days only (Task 2 test).
- A brand-new product (created today, no sales) → level = start, no divide-by-zero, `historyDays` 0 (Task 2 test).
- A loose version (derived) never shows on Reorder / Low stock; its sales ÷ ratio count for the bag (Tasks 2, 3, 4).
- Restored old backup (no new columns) → products Fixed, `reorder_calc` NULL, Reorder still lists them (Task 1 test).
- Cashier without the permission typing `/till/low-stock` → redirect to Sell; API 403 (Tasks 6, 8).

---

- [x] **1. Migration + fields** — `migrations/00008_reorder_auto.sql`: `products.reorder_auto` (ADD … DEFAULT false,
  then `ALTER … SET DEFAULT true`), `reorder_start INT NOT NULL DEFAULT 0`, `reorder_calc JSONB`;
  `settings.reorder_default_days INT NOT NULL DEFAULT 7`, `reorder_safety_days INT NOT NULL DEFAULT 3`,
  `reorder_ran_at TIMESTAMPTZ`; `users.can_see_low_stock BOOLEAN NOT NULL DEFAULT false`. Down drops them.
  `products.Product` gains `ReorderAuto bool`, `ReorderStart int`, `ReorderCalc *reorder.Calc` (scan JSONB);
  `CreateInput`/`UpdateInput` gain `ReorderAuto *bool` (nil = column default on create, keep on update) — create writes
  `reorder_start = reorder_level`. Loose version copies the source's `ReorderAuto`. Import: on an Auto product a
  reorder number updates `reorder_start` only. Backup dump/restore: check the new columns travel and an old dump
  restores (Fixed). DB test: insert without `reorder_auto` → true; existing-row default false after migration.
- [x] **2. Calculation** — `internal/features/reorder/calc.go` (pure, no DB):
  `type Calc struct{ Daily, RecentDaily, LastYearDaily float64; RhythmDays int; RhythmFrom string; Regular bool;
  SafetyDays, CalcLevel, LearnedPct, HistoryDays int; NotSelling bool; CappedBills int }`,
  `func Compute(in Input, now time.Time) Calc` and `func Blend(start, calcLevel, historyDays int) int` with
  `Input{ Sales []Sale{At time.Time; Qty float64}; ConvOut []Move; Stock []Move (all movements, for the in-stock
  rebuild); OnHand float64; OwnDeliveries, SupplierDeliveries []time.Time; DefaultDays, SafetyDays int; Created time.Time }`.
  Constants in one block: 30/90/180 days, 10 in-stock days, 56-day learning, 3× / p75 / ≥ 5 sales, 1.5× regular,
  last 4 gaps, ≥ 3 deliveries; `LooksOff(level int, c Calc) bool` (100 % learned, ≥ 2× or ≤ ½, ≥ 3 units).
  `calc_test.go`: one-off bill capped; regular big buyer not capped; out-of-stock days excluded; 30 → 90 fallback;
  regular → median, irregular → longest of last 4; own → supplier → default; last-year window wins; not selling →
  0; blend day 0 / 28 / 56+; start 0 → pure calc; new product with nothing → start; LooksOff cases.
- [x] **3. Loader + nightly run** — `internal/features/reorder/run.go`: `func Run(ctx, db) (n int, err error)`
  loads inputs for all active non-service products in batches of 500 (one query per input kind per batch, base
  units: sale_items qty − returned_qty; derived products' sales ÷ `product_conversions.ratio` (active, this product =
  `from_product_id`) as `Input.Derived`; type `purchase` days; supplier
  days via `purchases.supplier_id`), writes `reorder_calc` for all and `reorder_level = Blend(...)` for Auto
  products, then `settings.reorder_ran_at = now()`. `func RunOne(ctx, db, productID) error` for a switch to Auto.
  `func Loop(ctx, db)`: after 1 min run if `reorder_ran_at` is older than 20 h, then daily at 03:00 local; started in
  `internal/app/services.go` beside the backup scheduler. DB test: an Auto product's level moves, a Fixed one's
  doesn't; a loose version's sales count for its bag.
- [x] **4. Admin API** — DTOs (`dto_catalog.go`: product `reorderAuto`, `reorderStart`, `reorderCalc`
  (`ReorderCalcView` camelCase mirror of Calc); `dto_purchasing.go`: `ReorderRow` + `auto bool`, `calc
  *ReorderCalcView`, `looksOff bool`; settings `reorderDefaultDays`, `reorderSafetyDays`); `make types`.
  Move the row building out of `AdminReorder` into `internal/api/reorder_rows.go`
  `func (h *handlers) buildReorderList(c echo.Context, q products.ListQuery, all, hideCovered, withCost bool) (ReorderList, error)`;
  suggested qty = `ceil(level + daily × rhythm − onHand)` when calc has use, else today's rules; plugin annotator
  suggestion still wins when bigger. Derived products (`to_product_id` of an active conversion) never appear (`products.ListQuery.NotDerived`).
  `?looksOff=1` lists Fixed products with `LooksOff` (low or not). New:
  `POST /admin/products/reorder-level` `{ids, use: "suggested"|"auto"}` (Use N / Use all / Switch these to Auto,
  audited) and `POST /admin/products/reorder-auto` `{categoryId (0 = all)}` → `{switched}` (sub-categories
  included, start = current level, then `RunOne` each, audited). Product update switching to Auto calls `RunOne`.
  Handler tests: looksOff filter, Use, category switch counts, suggested qty from calc.
- [x] **5. Admin UI** — `ProductForm.tsx` Reorder level → Auto | Fixed segmented with the three states from the
  mockup (learned reason, learning bar + Starting guess, Fixed + "Sales suggest N · Use N"), reason text from one
  helper `web/src/features/purchasing/reorder/reason.ts` `reasonLine(calc, auto, level): string` (Vitest).
  `ReorderPage.tsx`: reason line (Auto/Fixed chip + `reasonLine`), Not selling tag, "1 big bill ignored", "Levels
  look off · N" chip → looksOff list with Use N / Use all / Switch these to Auto. Settings → Stock: the two day
  boxes + Switch to Auto (CategoryPicker incl. "All products", confirm with the count). Screenshots: 360 + desktop,
  Studio/Night × light/dark.
- [x] **6. Permission** — `can_see_low_stock` through `auth` (create/update/insert), `user_validator.go`,
  `middleware.UserFlags.CanSeeLowStock` + `MaySeeLowStock(role, flag)` (admin/manager true) +
  `RequireLowStockAccess()`, `/me`, admin users DTO, `TillConfig.canSeeLowStock`; Users page PERMS gains
  `{ k: 'canSeeLowStock', label: 'See low stock at the till', hint: 'A Low stock tab with what to order and from whom.
  Add Counter operations to let them place the orders too.' }`. `flags_test.go` + users handler test.
- [x] **7. Till API** — `GET /api/v1/till/low-stock` (`counter_lowstock.go`, behind `RequireLowStockAccess`) →
  `buildReorderList` with `withCost = MaySeeCost(...)` (cost "" otherwise), search / category / supplier / page,
  rows grouped client-side. Handler test: 403 without the flag; cost blank without See cost.
- [x] **8. Till UI** — tab `{ to: '/till/low-stock', label: 'Low stock', show: (c) => c.canSeeLowStock }` in `MORE`;
  route `web/src/routes/_authed/till/low-stock.tsx` (redirect like suppliers.tsx);
  `web/src/features/counter/lowstock/LowStockTab.tsx`: filters + rows grouped by supplier (reason line, Not selling,
  suggested qty), Print → `/print/low-stock`. With `canHandleSuppliers`: **Order these** per group → writes
  `sessionStorage['ks-order-prefill'] = {supplierId, lines: StockLine[]}` and navigates to
  `/till/suppliers?supplier=<id>&job=order`; `SuppliersTab` reads the search params to pick supplier + job;
  `StockForm` gains `initial?: StockLine[]` (read once, then the key removed). The no-supplier group: Order these
  asks for a supplier first (SupplierSelect). Screenshots at the till (desktop) in both palettes.
- [x] **9. e2e** — `web/e2e/reorder-levels.spec.ts`: new product shows Auto; Fixed → Auto shows a level + reason;
  Reorder reason line + Levels look off → Use; Settings switch a category. `web/e2e/counter-lowstock.spec.ts`:
  cashier without the tick has no tab (and URL redirects); admin ticks it → tab lists items; with Counter operations
  too, Order these → Suppliers Order prefilled → Place order saves.
- [x] **10. Docs** — status.md handoff, AGENTS.md (migration list → next 00009_; `features/counter` tab list gains
  low stock), memory.
