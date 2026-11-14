# Stock freeze Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A product (or the whole shop) can be frozen: its sales never reduce stock or ask about stock, while counts,
GRNs, conversions and "moved to the shelf" keep working; unfreezing asks how many are on the shelf and adds them.

**Architecture:** One column `products.stock_frozen`, checked inside the existing stock paths (sale lines, service
components, returns, warranty, conversions, write-offs) so every caller, core and plugin, follows it. A new movement
type `to_shelf` books "Opened / moved to the shelf". A core hook `plugin.FreezeGuard` lets a plugin refuse freezing
(pharmacy: controlled drugs). The till treats a frozen product like a service for stock (never asks).

**Tech Stack:** Go (Echo, sqlx, goose, Postgres 17), React + TS (TanStack Query), Vitest, Playwright.

**Spec:** `docs/specs/2026-11-14-stock-freeze-design.md` (approved 2026-11-14). Read it before Task 1.

**Branch:** `stock-freeze` (user: build on a branch, hand-test on a dev server, merge when proven, then Release to Iman).

## Global Constraints

- **Quantities are the whole point (user).** Every task's tests assert exact stock numbers before and after, and the
  movements written. Use the spec's pencil numbers where they fit: 200 counted, opened 10 → 190, unfreeze with 13 on
  the shelf → 203.
- **Never stop a sale:** a frozen product never gets "insufficient stock", a found-at-till prompt or an `oversold` flag.
- **Losses are never adjustments.** Damage goes to the damaged list (`damage` movement); moving to the shelf is
  `to_shelf`; only "Count changed" is `adjust`.
- Core never names a plugin (`make check-core`). Plugins refuse freezing only through `plugin.FreezeGuard`.
- Pocket Cashier changes go only in `plugins/pocketcashier` / `web/src/plugins/pocketcashier` (hard rule).
- Migration `00035_stock_freeze.sql` (next free number). Update AGENTS.md's migration list.
- `make types` after any `dto*.go` change. Files ≤ 500 lines. Admin screens work at 360px; theme tokens only.
- One commit per task, no AI attribution. Go DB tests gated on `DATABASE_URL`; never run `go test` during Playwright.

## Review Focus

1. **A cart mixing frozen and unfrozen products:** only the unfrozen lines reduce stock (Task 2 test
   `TestFrozenMixedCart`).
2. **A product frozen after it was sold, then returned:** the return follows the product's state *now* (Task 3 test
   `TestReturnFollowsFrozenNow`).
3. **Unfreezing twice / a shelf count of 0 / a negative or non-number count:** the second unfreeze is refused; 0 is
   fine; negative refused (Task 5 tests).
4. **Moving to the shelf more than the counted stock:** refused, nothing changes (Task 5 test
   `TestMoveToShelfMoreThanStock`).
5. **Offline phone sale of a frozen product replayed after it was unfrozen:** the replay follows the state at replay
   time (server decides), and the sale still goes through (Task 2 test `TestReplayAfterUnfreeze`).

---

### Task 1: Schema and model

**Files:**
- Create: `migrations/00035_stock_freeze.sql`
- Modify: `internal/features/products/products.go` (field), `internal/features/products/repository*.go` (select
  lists that build `Product`), `internal/features/stock/stock.go` (const), `internal/features/settings/settings.go`
  (field + update), `AGENTS.md` (migration list)
- Test: `internal/features/products/frozen_db_test.go`

**Interfaces:**
- Produces: `products.stock_frozen BOOLEAN NOT NULL DEFAULT false`; `settings.freeze_new_products BOOLEAN NOT NULL
  DEFAULT false`; enum value `stock_movement_type 'to_shelf'`; `Product.StockFrozen bool` (`db:"stock_frozen"
  json:"stock_frozen"`); `stock.MoveToShelf = "to_shelf"`; `Settings.FreezeNewProducts bool`.

- [ ] **Step 1:** Test `TestProductReadsStockFrozen`: insert a product with `stock_frozen=true`, read it through the
  products service; assert `StockFrozen == true`. Test `TestNewProductStartsFrozenWhenShopFrozen`: set
  `freeze_new_products=true`, create a product through the normal create path and through `CreateForIntake`; assert
  both `StockFrozen == true`; with the setting false, `false`.
- [ ] **Step 2:** Run, see them fail.
- [ ] **Step 3:** Migration (up adds the columns + `ALTER TYPE stock_movement_type ADD VALUE 'to_shelf'`; down drops
  the columns, leaves the enum value — Postgres can't drop one). Every product insert (`repository_write.go:105`,
  `service_import.go`, product merge keeps the kept product's flag) sets `stock_frozen` from
  `settings.freeze_new_products`.
- [ ] **Step 4:** Run the tests, `make migrate`, `go test ./migrations/...`: pass.
- [ ] **Step 5:** Commit `feat(stock): stock_frozen on products, freeze_new_products setting, to_shelf movement`.

### Task 2: Sales never touch a frozen product's stock

**Files:**
- Create: `internal/features/stock/frozen.go`
- Modify: `internal/features/sales/service_lines.go` (product path ~324–380, service-component path ~266–320)
- Test: `internal/features/sales/frozen_db_test.go`

**Interfaces:**
- Produces: `stock.(*Repository).Frozen(ctx, productID int64) (bool, error)` and
  `FrozenSet(ctx, ids []int64) (map[int64]bool, error)` (one query for a cart).

- [ ] **Step 1:** Tests (all assert `stock.quantity`, lot `qty_remaining`, movements, flags):
  - `TestFrozenSaleKeepsStock`: 200 in stock, frozen, sell 4 → stock 200, lots unchanged, no `sale` movement for it,
    no `found` lot, no `oversold` flag, line `cost_price` = product cost price.
  - `TestFrozenSaleAtZeroNeverAsks`: stock 0, frozen, sell 3 with `AllowOversell=false`, `Review=false` → succeeds,
    stock 0.
  - `TestFrozenMixedCart`: frozen A (200) ×4 + unfrozen B (50) ×5 → A 200, B 45.
  - `TestFrozenServiceComponent`: a service whose recipe uses frozen paper (100) ×2 → paper 100; unfrozen paper → 98.
  - `TestFrozenPickedLot`: frozen product with two priced lots, sale picks a lot → price from the lot, lot qty
    unchanged.
  - `TestReplayAfterUnfreeze`: replay (client_ref) of a sale made while frozen, product unfrozen with stock 10 by
    replay time → replays, stock 10 − qty (server state wins).
- [ ] **Step 2:** Run, fail.
- [ ] **Step 3:** In `buildSaleLines`, load `FrozenSet` for the cart's products + components once; for a frozen
  product skip `DecrementGuarded` / `FoundAtTill` / `DepleteBatch` / `DepleteFEFO` and the oversold flag, cost =
  `p.CostPrice`; for a frozen component skip its decrement/deplete, its cost = its product cost price × qty.
- [ ] **Step 4:** Run the new tests + `go test ./internal/features/sales/`: pass.
- [ ] **Step 5:** Commit `feat(sales): a frozen product sells without touching stock`.

### Task 3: Returns, warranty, conversions, write-offs

**Files:**
- Modify: `internal/features/sales/service_returns.go` (restock ~224–230, damage ~256–275, full return path),
  `internal/features/warranty/service.go:152-160`, `internal/features/conversions/run.go:79`,
  `internal/features/stock/service.go` (`ConsumeInput`, `consumeTx`), `internal/api/till_customers.go`
  (`WriteOffRequest` → `FromShelf`), `plugins/documents/api_till.go:193`
- Test: `internal/features/sales/frozen_returns_db_test.go`, `internal/features/stock/frozen_consume_db_test.go`,
  `internal/features/conversions/frozen_db_test.go`

**Interfaces:**
- Consumes: `Repository.Frozen` (Task 2).
- Produces: `ConsumeInput.FromShelf bool` (`json:"from_shelf"`): for a frozen product, `true` = found-at-till the
  quantity first (the existing `FoundAtTill`, so the "extra" is a `found` lot), then consume as normal → net stock
  unchanged, the damage / own-use movement written with its cost. `false` = consume as normal. Ignored when not frozen.

- [ ] **Step 1:** Tests:
  - `TestFrozenReturnNotRestocked`: frozen, 200, return 2 (disposition restock) → 200.
  - `TestFrozenReturnDamaged`: frozen, 200, return 2 as damage → 200, one `damage` movement qty 2 with cost.
  - `TestReturnFollowsFrozenNow`: sold while unfrozen (50 → 48), then frozen, return 1 → 48; sold frozen, unfrozen,
    return 1 → +1.
  - `TestFrozenWriteOffShelf` / `...Back`: frozen 200, write off 1 `FromShelf=true` → 200 + damage movement 1;
    `FromShelf=false` → 199 + damage movement 1.
  - `TestFrozenWarrantyReplacement`: frozen 200 → 200.
  - `TestConversionIntoFrozen`: Pencil Box (20, unfrozen) → Pencil (frozen, 200), run 1 box = 10 → Box 19, Pencil 200,
    a `conversion` movement recorded for both sides with the qty. Pencil unfrozen → Pencil 210.
  - Purchase return of a frozen product: 200, return 5 to supplier → 195 (unchanged behaviour, pinned).
- [ ] **Step 2:** Run, fail.
- [ ] **Step 3:** Restock only when not frozen (or damaged → `FromShelf` path). Warranty: skip decrement when frozen.
  Conversion: skip the target `Increment` + lots when the target is frozen; still write its movement. `consumeTx`:
  `FromShelf` on frozen → `FoundAtTill` then the normal consume. Documents copy job: pass `FromShelf: true`
  (the open ream is the shelf; a non-frozen ream ignores it). Admin expired-lot write-off (`admin_batches.go:114`)
  stays `FromShelf: false`.
- [ ] **Step 4:** Run all three packages + `go test ./plugins/documents/`: pass.
- [ ] **Step 5:** Commit `feat(stock): returns, warranty, conversions and write-offs follow the freeze`.

### Task 4: Freeze guard hook + pharmacy

**Files:**
- Modify: `internal/plugin/hooks_product.go` (type + `AddFreezeGuard` + `FreezeRefusals`), `plugins/pharmacy/pharmacy.go`
  (register), `plugins/pharmacy/store_controlled.go` (query)
- Test: `internal/plugin/freeze_guard_test.go`, `plugins/pharmacy/freeze_guard_db_test.go`

**Interfaces:**
- Produces: `type FreezeGuard struct { Refuse func(ctx context.Context, productIDs []int64) (map[int64]string, error) }`;
  `func (r *Registry) AddFreezeGuard(g FreezeGuard)`; `func FreezeRefusals(ctx, ids []int64) (map[int64]string, error)`
  (merged from every guard; value = reason shown to the user). Pharmacy reason: `"a controlled drug: count it in full"`.

- [ ] **Step 1:** Tests: registry merges two guards' refusals; pharmacy refuses a controlled product, allows a plain one.
- [ ] **Step 2–4:** Fail, implement, pass (`make check-core` clean).
- [ ] **Step 5:** Commit `feat(plugin): FreezeGuard; pharmacy keeps controlled drugs counted`.

### Task 5: Freeze, unfreeze, moved to the shelf (service)

**Files:**
- Modify: `internal/features/stock/frozen.go`
- Test: `internal/features/stock/freeze_db_test.go`

**Interfaces:**
- Consumes: `plugin.FreezeRefusals` (Task 4; injected as a func so stock doesn't import plugin wiring — follow how
  `internal/app/services.go` wires other seams).
- Produces (on `stock.Service`):
  - `Freeze(ctx, productIDs []int64, userID int64) (FreezeResult, error)` — `FreezeResult{Frozen int; Refused
    map[int64]string}`; services skipped.
  - `FreezeShop(ctx, userID) (FreezeResult, error)` — every non-service product + `freeze_new_products=true`.
  - `Unfreeze(ctx, productID int64, shelfQty string, userID int64) error` — refuses when not frozen or qty < 0 / not a
    number; stock += shelfQty with an `adjust` movement note `"unfrozen: N on the shelf"`; flag off.
  - `UnfreezeShop(ctx, userID) (int, error)` — all flags off (no stock change), `freeze_new_products=false`.
  - `MoveToShelf(ctx, productID int64, qty string, userID int64) error` — frozen only; refuses qty > stock; stock −= qty,
    lots FEFO, movement `to_shelf`.
  - `FrozenProducts(ctx) ([]FrozenRow, error)` — id, name, stock, unit, frozen since.
- [ ] **Step 1:** Tests: the pencil day — 200, freeze, sell 4 (via sales service) → 200, MoveToShelf 10 → 190 with a
  `to_shelf` movement, Unfreeze "13" → 203, sell 1 → 202. Plus: unfreeze twice refused; "0" ok; "-1" / "abc"
  refused; `TestMoveToShelfMoreThanStock` (190, move 191 → error, still 190); MoveToShelf on unfrozen refused;
  FreezeShop skips a refused product and services and sets the setting; a product created afterwards is frozen;
  UnfreezeShop clears all and the setting.
- [ ] **Step 2–4:** Fail, implement, pass.
- [ ] **Step 5:** Commit `feat(stock): freeze, unfreeze with a shelf count, moved to the shelf`.

### Task 6: API

**Files:**
- Modify: `internal/api/dto_stock.go`, `internal/api/dto_till.go` (`TillProduct.StockFrozen bool json:"stockFrozen"`),
  `internal/api/dto_catalog.go` (product DTOs carry `stockFrozen`), `internal/api/admin_stock.go`,
  `internal/api/counter_stock.go`, `internal/api/counter.go` (routes), `internal/api/till_customers.go`
  (`WriteOffRequest.FromShelf`), `web/src/api/types.gen.ts` via `make types`
- Test: `internal/api/stock_freeze_test.go`

**Interfaces:**
- Produces routes: admin `POST /api/v1/admin/stock/freeze {productIds: number[]}`, `POST /admin/stock/freeze-shop`,
  `POST /admin/stock/unfreeze-shop`, `GET /admin/stock/frozen`; admin + counter
  `POST …/stock/:id/unfreeze {shelfQty}`, `POST …/stock/:id/to-shelf {quantity}`, counter `GET /counter/frozen`.
  Unfreeze / to-shelf on the counter: every till user incl. sales person (user); freeze / shop switches: admin.
  `FreezeResultDTO{frozen, refused: [{productId, name, reason}]}`.
- [ ] **Step 1:** API tests: roles (cashier can unfreeze, cannot freeze-shop), refused list returned, the till catalog
  shows `stockFrozen`, write-off with `fromShelf`.
- [ ] **Step 2–4:** Fail, implement, `make types`, pass.
- [ ] **Step 5:** Commit `feat(api): stock freeze endpoints`.

### Task 7: Till never asks for a frozen product

**Files:**
- Modify: `web/src/features/till/cart/lines.ts:50` (stock = `MAX_STOCK` when `p.stockFrozen`), the product card
  badge component, `web/src/plugins/pocketcashier/offline/routes/core.ts` only if it reads stock (check; the phone
  uses the same `TillProduct`)
- Test: `web/src/features/till/cart/lines.test.ts`, Pocket Cashier offline test if touched

- [ ] **Step 1:** Vitest: a frozen product line has unlimited stock, adding past `stockQty` never sets
  `allowOversell`; an unfrozen one still does.
- [ ] **Step 2–4:** Fail, implement (badge "Frozen", no stock number), `make test-web` pass.
- [ ] **Step 5:** Commit `feat(till): frozen products sell without the stock question`.

### Task 8: Screens

**Files:**
- Modify: `web/src/features/admin/stock/AdjustForm.tsx` ("What happened?" for frozen: Moved to the shelf / Damaged /
  Count changed), `web/src/features/counter/stock/WriteOffTab.tsx` ("From the shelf or the back?" when frozen),
  `web/src/features/admin/stock/StockPage.tsx` (Frozen filter, Freeze shop / Unfreeze shop with the still-frozen
  list as the warning), product form (Freeze toggle; shows the refusal reason)
- Create: `web/src/features/counter/frozen/FrozenTab.tsx` (+ its route under `web/src/routes/_authed/till/`):
  list, Unfreeze ("How many are on the shelf now?"), Moved to the shelf
- Test: `web/e2e/stock-freeze.spec.ts`

**Moved to the shelf** follows the product's setup (spec table): if the product is the *from* side of a conversion,
offer that conversion (existing run endpoint); else quantity, with a Pack picker when the product has packs.

- [ ] **Step 1:** Playwright (import `test` from `e2e/fixtures.ts`): the pencil day through the UI — admin freezes
  Pencil (200), cashier sells 4 with no question (200), cashier moves 10 to the shelf (190), write-off 1 from the
  shelf (190, on the damaged list), cashier unfreezes with 13 (203), sells 1 (202). Then Freeze shop → a new product
  is frozen → Unfreeze shop shows the warning list.
- [ ] **Step 2–4:** Fail, build the screens (360px, tokens), pass.
- [ ] **Step 5:** Self-review screenshots (light + dark, 360px for admin) per `docs/status.md`'s gate; commit
  `feat(stock): freeze screens (adjust, write-off, counter Frozen tab, shop switch)`.

### Task 9: Alerts, reports, plugin views

**Files:**
- Modify: `internal/api/counter_lowstock.go` and the low-stock / reorder queries (exclude frozen),
  `internal/features/products/` valuation (label count), `plugins/combos/store.go:70` (skip frozen parts' warning),
  plugins that show stock (`alternatives`, `clearance`, `pharmacy` lists) show "Frozen" instead of a number
- Test: `internal/api/counter_lowstock_test.go` (frozen excluded), combos test

- [ ] **Step 1–4:** Tests fail → implement → pass. Stock value report shows "N frozen products: counted stock only".
- [ ] **Step 5:** Commit `feat(stock): frozen products out of low stock; reports and plugins say Frozen`.

### Task 10: Docs, shots, hand test, merge, Release

- [ ] Guide: `docs/guide/stock.md` (Freezing stock while a shop starts: the pencil example), `docs/guide/till.md`
  (Frozen tab, write-off question), `docs/guide/plugins/pharmacy.md` (controlled drugs can't be frozen).
- [ ] Shots: add the Frozen tab + Adjust question to `web/docs-shots/shots.spec.ts`, `make docs-shots`.
- [ ] `make verify`, `make test`, `make test-web`, full Playwright (one run, nothing else on 3000/5173).
- [ ] **User hand test on a dev server** (`make dev`, clean starter data): the pencil day, a box with a conversion, a
  pack product, a damaged return. Fix what they find.
- [ ] After the user says it's proven: merge `stock-freeze` → `main`, delete this plan and the spec, update
  `docs/status.md`, Release to Iman (Shop Builder; release notes = the `feat` titles).
- [ ] Onboard (separate repo, after): push products to a shop already frozen.
