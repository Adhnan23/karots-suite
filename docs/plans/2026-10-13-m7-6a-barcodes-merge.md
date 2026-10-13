# M7.6a Extra barcodes & Merge products (core) — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: use superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> Same shape as M7.5. Opus executes and reviews, one commit per step, and UI gates are self-reviewed (4 combos, 1366 + 360).

**Goal:** a product carries extra barcodes (added at intake, purchase entry, the product form, sheets; each shows
its last scan), and two products can be merged into one — barcodes, stock, history and plugin rows move, so the
old code still scans.

**Spec:** `docs/specs/2026-10-13-m7-6a-barcodes-merge-design.md`. No karots-pos source (new in the suite; karots-pos
dropped multi-barcode in its lot-pricing design — see its `batch-dual-pricing-design` note for why lots don't own
barcodes).

## Global constraints

- Core never names a plugin. The only new seam is the generic `ProductMerger` hook. `make check-core` after every task.
- `enabled_plugins.go` / `enabled-plugins.ts` committed core-only.
- DTOs in `internal/api/dto_catalog.go` → `make types`; never hand-edit `types.gen.ts`.
- Money and stock: `stock.quantity` = SUM(`stock_batches.qty_remaining`) before and after every write. No new money path.
- Theme tokens only, 360px admin, 4 combos. Files ≤ 500 lines. No AI attribution in commits.

## Review focus

1. A code is on one product only — main or extra, across both tables, also under two concurrent adds (the DB trigger, not only the service check) — Task 1.
2. After a merge the duplicate's old main barcode **and** its extras scan at the till to the kept product — Task 6, e2e Task 9.
3. A merge changes no money: P&L for any period, stock value and per-product SUM(lots) = stock are equal before and after (only the product the figures sit under changes) — Task 6.
4. With every plugin compiled in, a merge hits no unique violation and leaves no plugin row on the duplicate — Task 7.
5. A merged row never comes back: product list, "Show disabled", pickers, till search, reorder, import's "match by name", Items to review — Task 1 / Task 6.

---

### Task 1: Core data + one barcode lookup
- [x] `migrations/00003_product_barcodes.sql`: `product_barcodes` (spec §2), `products.barcode_scanned_at`, `products.merged_into_id bigint NULL REFERENCES products(id)`; trigger function on `product_barcodes` (insert/update) and `products` (barcode update/insert) refusing a code used by the other table (`unique_violation`, message "barcode is already used").
- [x] `products` repository: `FindByBarcode` / `BarcodeExists` / `search.go` exact match + ranking / `service_import.go` matcher check main OR extra (one SQL fragment, `barcodeMatchSQL`); `GenerateBarcode` skips extras; every list / search / picker / reorder query excludes `merged_into_id IS NOT NULL`; product rows load `barcodes []string` (one `array_agg` subquery).
- [x] `TouchBarcode(ctx, code)` stamps `last_scanned_at` / `barcode_scanned_at`; called from `TillProductByBarcode` and the intake scan.
- [x] DB tests: extra found by till lookup, search, import; trigger refuses both directions; generated code never an extra; merged row hidden everywhere in focus 5; a baseline-only backup (karots-pos shape) restores into 00003 with empty barcodes. Commit `feat(products): extra barcodes — one lookup checks main and extras, each remembers its last scan`

### Task 2: Barcode API
- [x] `ProductBarcode`, `AdminProductRow.barcodes`, `TillProduct.barcodes` in `dto_catalog.go` / till DTOs → `make types`.
- [x] Routes (spec §4): add (dup → 409 naming the other product), remove, make main (swap in one tx; the old main keeps its scan date as an extra). Admin + manager. Audit.
- [x] API tests incl. cashier 403, 409 text, swap round trip. Commit `feat(api): add / remove / make-main barcodes`

### Task 3: Web — hasBarcode + product form Barcodes block (UI gate)
- [x] `web/src/lib/barcode.ts` `hasBarcode(p, code)` (+ Vitest); replace every `p.barcode === term` (IntakePage, ProductPicker, purchasing LineEditor, till scan path).
- [x] `ProductForm`: Barcodes block — main, extras with "added · last scanned 4 months ago / never scanned", Make main, Remove (in-place confirm), Add barcode (409 message inline). New product: main only (extras after first save).
- [x] Gate: 4 combos at 1366 + 360. Commit `feat(products): the product form lists every barcode — make main, remove, add`

### Task 4: Web — "Same as an existing one" at intake and purchase entry (UI gate)
- [x] IntakePage: an unknown scan's new panel gets **Same as an existing one** → ProductPicker → add barcode (`source: intake`) → that product's restock panel.
- [x] LineEditor: the unknown-scan row gets the same choice (`source: receive`) → adds the line for the picked product.
- [x] Gate shots both screens, 1366 + 360. Commit `feat(stock): an unknown scan at intake or purchase entry can join an existing product`

### Task 5: Products sheet — Extra barcodes column
- [x] Export / template / import: `Extra barcodes` (`;`-separated). Import adds missing ones (`source: import`), never removes; a code on another product is a row note, not a failed row. Round-trip test (CSV + XLSX). Commit `feat(sheets): extra barcodes column`

### Task 6: Merge service + ProductMerger hook (core)
- [x] `internal/plugin/hooks_product.go`: `ProductMerger{Key string; Merge func(ctx, tx *sqlx.Tx, fromID, intoID int64) error}`, `AddProductMerger`, `ProductMergers()`.
- [x] `internal/features/products/merge.go`: `Preview(ctx, from, into) (MergePreview, error)` (counts, price clash, refusal reason) and `Merge(ctx, from, into, lotPrice string, userID)` — one tx, spec §3 order: refusals (same, unit, service vs stock, held bill via `held_sales.cart`) → barcodes → stock + lots (+ stamp option) → history tables → set-up rows with doubles collapsed → plugin mergers → duplicate disabled, barcode NULL, `merged_into_id` → activity + audit. Split into `merge.go` + `merge_tables.go` to stay under 500 lines.
- [x] DB tests: one assertion per moved table; doubles collapsed (group item, recipe both sides, recipe cost label, conversion pair deleted); each refusal; money invariant (P&L, stock value, SUM(lots) = stock before vs after); old main + extra scan → kept. Commit `feat(products): merge two products — barcodes, stock, history and set-up move, the duplicate is retired`

### Task 7: Plugins implement ProductMerger
- [x] alternatives (`alt_members`: drop the duplicate's membership when the kept one is in a group), combos (`combo_items` sum qty on clash), productplus (`pp_values`: kept product's value wins, else move), documents (`doc_service` / `doc_job` / `doc_consumable`), clearance (`clearance_markdowns`), requests (`requests.product_id`). Each in its own file, each with a DB test.
- [x] `go test ./... -p 1` with every plugin. Commit `feat(plugins): every plugin moves its rows when products merge`

### Task 8: Merge API + UI (UI gate)
- [x] `MergePreview`, `MergeRequest` DTOs → `make types`; `GET/POST /admin/products/:id/merge/:into`. API tests.
- [x] Products row menu **Merge into…** → ProductPicker (excludes itself, services vs stock filtered) → confirm sheet: counts, price-clash choice, refusal shown in place of the button, "can't be undone". Items to review: **Same as an existing one** → same sheet.
- [x] Gate: 4 combos 1366 + 360 (confirm with and without a price clash, a refusal). Commit `feat(products): Merge into… from Products and Items to review`

### Task 9: E2E
- [x] `barcodes.spec.ts`: intake "Same as an existing one" → scan the new code at the till sells the product; form Make main / Remove; purchase entry unknown scan → existing.
- [x] `products-merge.spec.ts`: merge two with stock + a sale → old main and extra scan at the till to the kept product, Reports show one row, stock summed; price clash keep-old → till "Which price?"; held bill refusal; Items to review → merge; 360px. Commit.

### Task 10: Docs + full checks
- [x] `docs/differences.md` (extra barcodes and merge are suite-only), `docs/status.md` (M7.6a done, M7.6b next), AGENTS.md layout line for `product_barcodes` / merge if needed.
- [x] `make verify`, `go test ./...` core-only and all plugins (`-p 1`), web unit, Playwright, `make verify-portable`. Commit `docs: M7.6a done — extra barcodes and merge products`
