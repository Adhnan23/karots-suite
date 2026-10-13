# M7.6d Loose versions and honest conversions — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: use superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> Same shape as M7.6b. Opus executes and reviews, one commit per step, and UI gates are self-reviewed (4 combos, 1366 + 360).
> **Start in a fresh session:** read `AGENTS.md`, `docs/status.md`, the spec below, then this plan.

**Goal:** a packed product's loose version is made in one sheet (product + conversion + optional first run), a
conversion carries each source batch's expiry and cost into its own loose batch and spreads cost over the real
yield, and an alternatives swap across units asks the quantity.

**Architecture:** a new stock helper `DepleteFEFOLots` returns the consumed batches; `conversions.Run` opens one
destination batch per consumed batch and takes an optional actual yield. A core endpoint
`POST /api/v1/admin/products/:id/loose` does create product + create conversion + optional run in one transaction.
The product form's sheet calls it. Alternatives gets `unit` / `sameUnit` on till alts and a quantity prompt.

**Spec:** `docs/specs/2026-10-13-m7-6d-loose-versions-design.md`. Code to read first: `internal/features/conversions/conversions.go`
(`Run`), `internal/features/stock/batches.go` (`DepleteFEFO`, `InsertBatch`, `NewBatch`), `internal/api/admin_conversions*.go`,
`web/src/features/admin/conversions/ConversionsPage.tsx`, the counter Convert tab (`web/src/features/counter/`),
`web/src/features/admin/products/ProductForm.tsx` (footer: Duplicate / Merge into…), `plugins/alternatives/api.go` (`AltDTO`, `till`),
`web/src/plugins/alternatives/SwapChip.tsx`, `web/src/features/till/cart/lines.ts` (`swapProduct`).

## Global constraints

- Core never names a plugin; `make check-core` after every task. Plugin lists committed core-only.
- DTOs in the matching `internal/api/dto_*.go` → `make types`; never hand-edit `types.gen.ts`.
- `stock.quantity` = SUM(`stock_batches.qty_remaining`) after every write; a conversion moves value, never creates it.
- No money moves in a conversion (no "extra cost"). A question to the cashier is a prompt/dialog, never a toast.
- Theme tokens only, admin works at 360px, 4 combos. Files ≤ 500 lines. No AI attribution in commits.
- The :3100 demo may be running for the user — run test servers on :3101 (temporary Playwright config with
  `baseURL` 3101 and no `webServer`, deleted after), or ask before stopping the demo.

## Review focus

1. A run over two source batches: Σ loose qty = the yield exactly, Σ loose value = consumed value ± 0.01 per batch — Task 2.
2. A loose-version request that fails half-way (bad ratio, same unit, run short of stock) leaves no product and no conversion — Task 3.
3. A blank "Got" behaves exactly as today (yield = fromQty × ratio) — Task 2.
4. A different-unit swap never keeps the old quantity silently; cancelling the prompt leaves the line as it was — Task 6.
5. Existing conversions, sales FEFO and returns still work: `DepleteFEFO` callers see no change — Task 1.

---

### Task 1: `DepleteFEFOLots` (stock)
- [ ] `internal/features/stock/batches.go`: `ConsumedLot{BatchID int64; Qty, Cost decimal.Decimal; Expiry *time.Time; BatchNo *string}`;
  `(r *Repository) DepleteFEFOLots(ctx, productID, qty) ([]ConsumedLot, decimal.Decimal, error)` — the current `DepleteFEFO`
  body, selecting `expiry_date, batch_no` too; `DepleteFEFO` = call it, return the cost. Keep the zero-cost fallback rule.
- [ ] DB test: two batches (different expiry / cost / batch no) → lots in FEFO order with exact takes; `DepleteFEFO` cost unchanged.
  Commit `feat(stock): DepleteFEFOLots — which batches a depletion took, with their cost and expiry`

### Task 2: conversion runs keep batches and take the real yield
- [ ] `conversions.Run(ctx, id, fromQty, toQty decimal.Decimal, userID)` — `toQty` zero = `fromQty × ratio`; negative → validation.
  Use `DepleteFEFOLots`; per consumed lot open a destination batch: qty share `take/fromQty × toQty` (3 dp, last takes the
  remainder), cost = lot cost × take ÷ share (2 dp), `ExpiryDate` / `BatchNo` copied, source `conversion`. One destination
  `Increment` by `toQty`, movements as today, `LogRun` with the real `toQty`.
- [ ] `RunInput` / API DTO gets optional `toQty`; admin Conversions run sheet and the counter Convert tab get **Got** (prefilled
  expected; shows "0.4 kg less — cost per kg Rs 2.03 higher" when it differs) → `make types`.
- [ ] DB tests (focus 1, 3): blank yield unchanged; yield 24.6 → unit cost up, value conserved; two source batches → two loose
  batches with each expiry + batch no; API test for `toQty`. Check karots-pos's conversion run for the no-expiry flaw — if
  there, fix it there too (bug-fix-only commit) and note it in status. Commit `fix(conversions): loose stock keeps each bag's
  batch, expiry and cost, and cost spreads over what was really weighed out`

### Task 3: `POST /admin/products/:id/loose` (one transaction)
- [ ] DTO `LooseVersionRequest{name, unitId, ratio, sellingPrice, barcode, convertNow, extra}` → `LooseVersionResult{productId, conversionId}`;
  refuse: source is a service / inactive, same unit as the source, ratio ≤ 0, blank name. In one tx: products create (copy
  category, supplier, tax, reorder, warranty, local name from the source; `ProductExtras` Validate/Save with `x.IsNew` and the
  extras sent), conversions create, run if `convertNow > 0` (Task 2 path). Services / repositories take the tx (follow the
  merge code's pattern for running services inside one tx).
- [ ] API tests (focus 2): happy path with convert-now (stock + batch expiry); each refusal leaves nothing behind; run short of
  stock rolls back the product too. Commit `feat(products): make a loose version — product, conversion and first run in one go`

### Task 4: Make a loose version… sheet (UI gate)
- [ ] `web/src/features/admin/products/LooseVersion.tsx`: FormSheet — name, unit (not the source's), "1 bag = N kg", selling,
  barcode + Generate, cost-per-unit hint, Convert now, plugin sections via `ProductExtras` with `sourceId` = the source
  (so Product Plus copies), "Already sold loose as …" note when an active conversion from this product exists.
- [ ] Footer button in `ProductForm.tsx` (stocked, saved, active, not a service). Shots: 4 combos, 1366 + 360.
  Commit `feat(products): Make a loose version… sheet on the product form`

### Task 5: plugins on a loose version
- [ ] Product Plus: confirm its section copies the source's values for this sheet (`sourceId`); fix if it needs `productId === 0`.
- [ ] Alternatives: its section starts **unticked** here — a checkbox "Also offer as an alternative to <source>" (sends the
  source's tier only when ticked). Needs the section to know it's a loose version: add `kind?: 'copy' | 'loose'` to the
  `ProductFormSection` props (generic, core passes it) and default to today's behaviour for `copy`.
  Commit `feat(plugins): a loose version copies Product Plus values; alternatives only when ticked`

### Task 6: alternatives swap across units asks the quantity
- [ ] `plugins/alternatives`: `AltDTO.unit` (abbreviation) + `sameUnit` (vs the short line's product) in the till alts query.
- [ ] `SwapChip` (and the found-at-till hint): different unit → chip shows "· kg"; tap → prompt "How many kg of <name>?" (decimal
  > 0, required; Cancel leaves the line). `TillApi.swapLine(key, productId, qty?)` + reducer `swap` with optional qty
  (`swapProduct` sets that qty) — core change, generic. Group page: unit badge on members whose unit differs from the first.
- [ ] Vitest: `swapProduct` with qty; Go test: `sameUnit`. Commit `feat(alternatives): a swap to another unit asks how many`

### Task 7: e2e, docs
- [ ] `web/e2e/loose-versions.spec.ts` (+ an alternatives case): spec §6 flows. Full Playwright, `go test ./...` core-only and
  with every plugin (`-p 1`), `make test-web`, `make verify`, `make verify-portable`.
- [ ] `docs/differences.md` rows; `docs/status.md` + memory. Commit `docs: M7.6d done — loose versions`
