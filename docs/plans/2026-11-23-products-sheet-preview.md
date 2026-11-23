# Products Sheet Preview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans (the user chose native). Steps use checkbox (`- [ ]`) syntax.

**Goal:** Products → Sheets ▾ → Import previews every change (old → new) before applying, and applies all or nothing.

**Architecture:** `AdminProductsImport` runs the whole import inside one transaction carried in the context
(`db.InTx`); each row runs in its own savepoint (`db.WithTx` joins); category / supplier creation moves onto the
transaction; a field snapshot of each touched product before and after its row gives the diff; `?dry=1` rolls back.
The import FormSheet shows the preview, then imports the copied bytes.

**Tech Stack:** Go (Echo, sqlx), React + TS, battle tests, Playwright.

**Spec:** `docs/specs/2026-11-23-products-sheet-preview.md`

## Global Constraints

- The preview changes nothing in any table (plugins' too). Apply = preview (counts, changes, skipped lines).
- Only the columns a sheet fills change (today's partial update); the diff lists exactly the changed fields.
- Core names no plugin: plugin columns are read through `plugin.ProductSheetColumns.Export` inside the transaction.
- Customers / suppliers sheets unchanged. Files ≤ 500 lines. Theme tokens only; 360 px. Rule first. No AI attribution.

## Review Focus

- A row that fails after it created a category / supplier → nothing of that row stays (savepoint). Task 2 test.
- A sheet without a `ref` column (barcode / name matching) → the diff still finds the product before its row. Task 3 test.
- A plugin column cell that the plugin refuses (a note, not an error) → shown as a note, value unchanged in the diff. Task 3 test.
- A 1,700-row sheet → the preview answers within the request (one transaction, no per-row commit). Task 5 rehearsal timing.
- The same product named on two rows → two changes listed, Apply ends with the second row's values (as today). Task 3 test.

---

### Task 1: Rule book — IMP

**Files:** Modify `docs/rules/stock.md` (an IMP section + a row "Products sheet import" in the grid), `docs/rules/README.md` (ids line).

- [ ] Write IMP-1 … IMP-5 from the spec's Rule book section in the README rule format; commit `docs(rules): IMP — preview a products sheet before it imports`.

### Task 2: One transaction, a savepoint per row, a dry run

**Files:** Modify `internal/api/admin_import.go` (`AdminProductsImport`), `internal/features/suppliers/suppliers.go` (`FindOrCreateByName` works on `appdb.On(ctx, s.db)`), `internal/features/categories/categories.go` (`FindOrCreateByPath` works on the ctx transaction: build the repo from `db.TxFrom(ctx)` when set). Test: `internal/battle/cases_import_test.go`.

**Interfaces:** Produces `?dry=1` on `POST /api/v1/admin/products/import`; `EntityImportResult.Applied bool` (false on a preview).

- [ ] **Tests (fail first):** `TestIMP1PreviewChangesNothing` — a sheet that updates a product by ref, adds one with a new category path, a new supplier, extra barcodes, opening_qty and a Product Plus cell (battle shops have every plugin) → `?dry=1` → `ltest.Changed(before, after)` is empty. `TestIMP3FailedLineLeavesNothing` — a line with a new category path and a negative selling price → skipped, and no category of that name exists after Apply.
- [ ] **Implement:** wrap the `importRows` loop in `db.WithTx(ctx, h.d.DB, …)` with `ctx = db.InTx(ctx, tx)`; each row's work inside `db.WithTx(rowCtx, …)` (savepoint); a preview returns a sentinel error after `finishImport`'s result is built (like the menu import); audit only on Apply.
- [ ] Run `go test -exec "$PWD/bin/testdb exec" ./internal/battle/ -run TestIMP -count=1` → PASS; the existing import tests (`./internal/api/ -run Import`, `./internal/features/products/ -run Import`) still pass. Commit `feat(import): the products sheet imports in one transaction and can be previewed (IMP-1, 3)`.

### Task 3: The diff

**Files:** Create `internal/api/import_diff.go` (`productFields(ctx, q db.Queryer, id int64, xs []sheetExtra) (map[string]string, error)` and `diffFields(before, after map[string]string) []FieldChange`); modify `internal/api/dto_data.go` (`EntityImportResult.Changes []ImportChange`), `make types`. Test: battle `TestIMP4PreviewListsTheChanges`, `TestIMP2ApplyIsThePreview`.

**Interfaces:** `type FieldChange struct { Field, From, To string }`; `type ImportChange struct { Line int; Product string; New bool; Fields []FieldChange }` (camelCase json). Field names as the sheet's headers (`name`, `category`, `unit`, `selling_price`, …, `extra_barcodes`, then each plugin column's header).

- [ ] **Tests (fail first):** IMP-4 — a ref sheet that renames one product, moves another to `A > B`, sets one's Generic Name, and leaves a fourth untouched (only its ref + same name) → the preview's changes are exactly those three, each with only its changed fields and the right old / new values; a sheet without `ref` (barcode match) gives the same; a twice-named product gives two changes. IMP-2 — Apply's result equals the preview's (Applied aside).
- [ ] **Implement:** before each row, snapshot the matched product (ref id, else the import's own barcode / name match — read the id ImportOne returns, so snapshot "before" by re-running the same lookup the import uses, or snapshot every candidate cheaply: decide by the simplest that the tests pin and ledger it); after the row, snapshot again; diff; a new product lists `New` and its name.
- [ ] Run, pass, commit `feat(import): the preview lists each product's changes, old → new (IMP-2, 4)`.

### Task 4: The import sheet

**Files:** Modify `web/src/components/admin/ImportSheet.tsx` (split out `ImportPreview.tsx` if it passes 500 lines). Tests: `web/src/components/admin/ImportSheet.test.tsx` (Vitest), the products import e2e in `web/e2e/admin-catalog.spec.ts` (the existing one now confirms the preview).

- [ ] **Tests (fail first):** Vitest — choosing a file calls `…/products/import?dry=1` with a copied File; the sheet shows "N to update · M new · K skipped", a change row "old → new"; the search filters; Import sends the copy without `dry`. e2e — the existing products-import test passes through the preview (press Import after it) and checks one listed change.
- [ ] **Implement:** the FormSheet: file → preview (busy state) → counts, changes list (scrolls, wraps at 360 px), new products, skipped lines, a search input; submit = **Import** (re-uploads the copy), then today's report; a different file re-previews. Customers / suppliers: unchanged path (no dry run).
- [ ] UI gate (two palettes × light / dark × 360 / 1280), commit `feat(import): Products → Sheets → Import shows what will change before it imports`.

### Task 5: Rehearse the Iman fix, docs, done

- [ ] A second dev shop **as Iman is today** (`iman_asis` on :3011, Pocket port 8454): Iman's own names, categories, units, prices, suppliers, barcodes and Product Plus values from the Onboard backup (`iman-preview/load_asis.py`). Export Products with Include ref + custom fields; build the fix sheet by ref from `shops/iman-categories.csv` (catalogue name, category path, Generic Name; unit only where no packs); import it through the preview; check and time it; leave it running for the user.
- [ ] Guide (`docs/guide/` products import section) + status; `make test-all` green; review; delete spec and plan; commit.
