# Supplier by field Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Product Plus lets the shop pick one text / dropdown field and set a supplier per value; a product with no
preferred supplier of its own then counts as that supplier's (the **rule supplier**) everywhere except claims, lots
and sheet exports.

**Architecture:** A generic core hook (`plugin.RuleSupplier`) answers in product ids: "who supplies these products?"
and "which products does supplier X get by rule?". Core keeps the rule supplier in separate `Product.RuleSupplier*`
fields, never in `preferred_supplier_id`, and each read site opts in. Product Plus is the one plugin answering.

**Tech Stack:** Go (Echo, sqlx, goose plugin migrations), React + TS (admin kit, TanStack Query), Postgres 17.

**Spec:** `docs/specs/supplier-by-field.md`

## Global Constraints

- Core never names Product Plus; the hook is generic (AGENTS.md hard rule). `make check-core` must pass.
- `products.preferred_supplier_id` is written only by what writes it today (form, sheet import, order / delivery
  set-if-empty, Move products). Product Plus never writes it.
- Rule supplier is used for: products list (name + `?supplier=` filter), Low stock (name, id, filter, total),
  orders built from Low stock, stock-take page + count sheet filters, the product form hint, the supplier page's
  count, smart reorder levels' supplier rhythm. **Never** for: `recovery/losses.go`, `stock.LotSupplierSQL`,
  products sheet export (`admin_import.go`), import preview (`import_diff.go`), loose versions (`conversions/loose.go`).
- A rule pointing at a disabled supplier = no rule supplier.
- Only **text** and **dropdown** (`select`) fields can be the supplier field. Values match `lower(btrim(value))`.
- Files stay ≤ 500 lines; admin screens work at 360 px, theme tokens only, every palette light + dark.
- No Claude / AI attribution in commits. One commit per task step marked Commit. `make core-only` before committing.

## Review Focus

1. An order built from Low stock through a rule supplier sets that supplier as the product's own (existing
   set-if-empty) — afterwards the product shows its own supplier and no "by Brand" mark. (Task 4 test.)
2. `"Bosch "`, `"BOSCH"` and `"bosch"` on three products all follow the one `bosch` rule; the value list shows them as
   one row with 3 products. (Task 3 test.)
3. A Low stock / products page filtered by a supplier: `total` equals the rows across all pages when some rows come by
   rule (List and Count must both carry the OR). (Task 2 test.)
4. Disabling the rule's supplier drops those products from its filter and from their rule supplier; bringing it back
   restores them. (Task 3 test.)
5. Products sheet export of a product with only a rule supplier has an empty supplier column, and importing that sheet
   back leaves `preferred_supplier_id` NULL. (Task 4 test.)

---

### Task 1: Rule book

**Files:**
- Modify: `docs/rules/purchasing.md` (grid row + rules after SUP-10)
- Modify: `docs/rules/plugins/productplus.md` (grid row + PPF-10…13)

- [ ] **Step 1: Write the core rules.** Grid row "A product's supplier (own, by rule)" → SUP-11 / SUP-12 in its
  squares (`— a` where nothing applies). Rules:
  - **SUP-11** A product's own (preferred) supplier is set by hand, by a products sheet, by Move products, or by the
    first order or delivery that has it while it has none — never by anything else (a plugin's rule included).
  - **SUP-12** A product's supplier is its own, else the supplier a plugin's rule gives (a disabled supplier counts as
    none), else none. The rule supplier counts in the products list, Low stock and the orders made from it, stock-take
    and the count sheet, the supplier's product count and smart reorder levels; never in loss claims, a lot's
    supplier, a products sheet export or an import preview.
- [ ] **Step 2: Write the Product Plus rules** (grid row "Supplier by field" → PPF-10…13):
  - **PPF-10** One field at most is the supplier field, and only an active text or dropdown field can be chosen.
    Choosing a different one while rules exist needs confirming and clears them.
  - **PPF-11** A product with no own supplier gets the rule supplier of its value of that field (trimmed, any case),
    only while the field is active, of a usable type and the product is in its scope; otherwise none. Rules are kept
    while the field is unusable.
  - **PPF-12** The value list is every distinct value (trimmed, any case; shown in its most used spelling) plus the
    dropdown's unused options, each with its product count: values with no supplier first, then most products first.
  - **PPF-13** Moving a supplier's products to another moves the rules that point at it, in the same transaction.
  Each with `Checked by:` (example; ledgercheck for PPF-10 and the map's suppliers).
- [ ] **Step 3: Commit** `docs(rules): SUP-11, SUP-12 a product's supplier; PPF-10…13 supplier by field`

### Task 2: Core seam and products list / count / count sheet

**Files:**
- Modify: `internal/plugin/hooks_product.go` (type, registry var, `Add…`, getter)
- Modify: `internal/features/products/hooks.go`, `products.go` (Product fields), `service.go` (fill in List, Get),
  `products.go` `List` + `repository_write.go` `Count` + `countsheet.go` (filter)
- Modify: `internal/app/plugins.go` (wiring + disabled-supplier filter)
- Test: `internal/features/products/rulesupplier_db_test.go`

**Interfaces:**
- Produces (plugin):
  ```go
  type RuleSupplierOf struct { SupplierID int64; Why string } // Why: "Brand: Bosch"
  type RuleSupplier struct {
      For      func(ctx context.Context, productIDs []int64) (map[int64]RuleSupplierOf, error)
      Products func(ctx context.Context, supplierID int64) ([]int64, error)
  }
  func (r *Registry) AddRuleSupplier(s RuleSupplier)
  func RuleSuppliers() []RuleSupplier
  ```
- Produces (products, set by `internal/app/plugins.go`; nil = core-only):
  ```go
  type RuleSupplierOf struct { SupplierID int64; Name, Why string }
  var RuleSupplierFor func(ctx context.Context, productIDs []int64) (map[int64]RuleSupplierOf, error)
  var RuleSupplierProducts func(ctx context.Context, supplierID int64) ([]int64, error)
  // on Product (not db columns):
  RuleSupplierID *int64 `db:"-" json:"-"`; RuleSupplierName, RuleSupplierWhy string `db:"-" json:"-"`
  func (s *Service) FillRuleSuppliers(ctx context.Context, rows []Product) error // only rows with no own supplier
  ```
  The wiring asks every registered hook (first answer per product wins), then drops ids whose supplier is missing or
  `NOT is_active` and adds `Name` in one query. Hook errors are returned, never swallowed.

- [ ] **Step 1: Write the failing tests** (set the two vars to fakes, restore in `t.Cleanup`; no `t.Parallel`):
  - `TestSUP12_ListFillsRuleSupplier`: products A (own supplier S1), B (none, fake gives S2), C (none, no answer) →
    after `Service.List`: A `RuleSupplierID == nil`; B `*RuleSupplierID == S2`, `RuleSupplierName == "S2 name"`,
    `RuleSupplierWhy == "Brand: Bosch"`; C nil. Fake `For` must be called only with B and C's ids.
  - `TestSUP12_FilterBySupplierIncludesRule`: `ListQuery{PreferredSupplierID: &S2}` returns B, not A even if the fake
    lists A for S2 (own wins); `Count` with the same query == len(rows). `CountSheet(nil, &S2)` has B.
  - `TestSUP12_GetFillsRuleSupplier`: `Service.Get(B)` has the rule supplier.
- [ ] **Step 2: Run** `go test -exec bin/testdb ./internal/features/products -run SUP12` → FAIL (undefined).
- [ ] **Step 3: Implement** the types and vars above; `List`/`Count`/`CountSheet` supplier filter becomes
  `(p.preferred_supplier_id = $n OR (p.preferred_supplier_id IS NULL AND p.id = ANY($m::bigint[])))` with ids from
  `RuleSupplierProducts` (empty when nil); `Service.List` and `Get` call `FillRuleSuppliers`; hook types + wiring in
  `internal/app/plugins.go` (in the existing hooks block).
- [ ] **Step 4: Run** the same command → PASS; `go test -exec bin/testdb ./internal/features/products ./internal/app`
  → PASS; `make check-core` → OK.
- [ ] **Step 5: Commit** `feat(products): a plugin's rule supplier for products with none (SUP-12)`

### Task 3: Product Plus supplier rules (store, API, hook)

**Files:**
- Create: `plugins/productplus/migrations/0006_supplier_rules.sql`, `plugins/productplus/suppliers.go` (store +
  hook), `plugins/productplus/api_suppliers.go` (handlers)
- Modify: `plugins/productplus/productplus.go` (routes, `AddRuleSupplier`, data group tables)
- Test: `plugins/productplus/battle/suppliers_db_test.go`

**Interfaces:**
- Tables: `pp_supplier_field (id smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1), field_id bigint NOT NULL REFERENCES
  pp_fields ON DELETE CASCADE)`; `pp_supplier_map (value_key text PRIMARY KEY, supplier_id bigint NOT NULL REFERENCES
  suppliers)`; `value_key` = `lower(btrim(value))`. Both added to the `productplus` DataGroup.
- Routes (admin):
  - `GET  /api/v1/x/productplus/supplier-rules` → `SupplierRulesDTO{FieldID int64; Fields []FieldOption{ID, Label};
    Values []SupplierValueDTO{Value string; Products int; SupplierID int64; SupplierName string}}` (PPF-12 order).
  - `PUT  /api/v1/x/productplus/supplier-rules/field` body `{fieldId int64, confirm bool}` — 0 clears; a different
    field while rules exist and `!confirm` → `apperr.Validation` with code the SPA shows as a confirm. Not text /
    select / active → validation error.
  - `PUT  /api/v1/x/productplus/supplier-rules/value` body `{value string, supplierId int64}` — 0 removes the rule.
  Each change goes through `h.audit`.
- Hook: `For` joins `pp_values` of the chosen field (active, type text/select, product in scope via `covCTE`) to
  `pp_supplier_map` on `lower(btrim(v.value))`; `Why` = `"<field label>: <value>"`. `Products` is the same join
  filtered by supplier.

- [ ] **Step 1: Write the failing battle tests** (helpers `field`, `fill`, `s.Product` from `battle_db_test.go`;
  suppliers via `POST /api/v1/admin/suppliers`):
  - `TestPPF10_OneUsableField`: choosing a number field → 422; a text field → 200; another text field while a rule
    exists without confirm → 422, with `confirm: true` → 200 and the map is empty.
  - `TestPPF11_RuleSupplierInLists`: Brand values `"Bosch "`, `"BOSCH"`, `"bosch"` on P1–P3, P4 `"NGK"`; rule
    `bosch → S1`. `GET /api/v1/admin/products?supplier=S1` has P1–P3 (`total` 3), not P4. P2 given own supplier S9 →
    drops out of S1. Field disabled → S1's filter empty; enabled → back. P3 moved out of the field's scope → gone.
  - `TestPPF11_DisabledSupplier`: `DELETE /api/v1/admin/suppliers/S1` → S1 filter empty and the product detail has no
    rule supplier; `POST …/S1/activate` → back.
  - `TestPPF12_ValueList`: the three Bosch spellings are one row, `Products == 3`, shown spelling = the most used;
    an unused dropdown option shows with 0; rows without supplier come first.
- [ ] **Step 2: Run** `go test -exec bin/testdb ./plugins/productplus/battle -run 'PPF1[0-2]'` → FAIL (404).
- [ ] **Step 3: Implement** migration, store, handlers, routes and `reg.AddRuleSupplier`.
- [ ] **Step 4: Run** the same → PASS; then the whole `./plugins/productplus/...` → PASS.
- [ ] **Step 5: Commit** `feat(productplus): supplier by field — one field, a supplier per value (PPF-10…12)`

### Task 4: Core read sites (Low stock, orders, product, supplier, reorder rhythm)

**Files:**
- Modify: `internal/api/reorder_rows.go` (rows: `SupplierID`/`SupplierName` from the rule when no own one; new
  `SupplierWhy string json:"supplierWhy"` on `ReorderRow` — fill after rows are loaded with `FillRuleSuppliers`)
- Modify: `internal/api/dto_catalog.go` + `admin_products.go` (`AdminProductRow.SupplierWhy`, rule name in
  `SupplierName` when no own one; `AdminProduct.RuleSupplierName`, `RuleSupplierWhy` for the form hint — `SupplierID`
  stays the own one)
- Modify: `internal/api/dto_purchasing.go` + `admin_suppliers.go` (`AdminSupplierDetail.ProductsByRule int` — rule
  products with no own supplier; `Products` unchanged)
- Modify: `internal/features/reorder/run.go` (supplier = own, else `products.RuleSupplierFor`)
- `make types` after the DTO changes.
- Test: `plugins/productplus/battle/suppliers_sites_db_test.go`

- [ ] **Step 1: Write the failing battle tests** (one shop, Brand rule `bosch → S1`, P1 Bosch with no own supplier
  and stock below its level):
  - `TestSUP12_LowStockAndOrder`: `GET /api/v1/admin/reorder?supplier=S1` has P1 with `supplierId == S1`,
    `supplierWhy == "Brand: bosch"`-style text; `POST /api/v1/admin/reorder/build` with that line → a draft order for
    S1, and P1's `preferred_supplier_id` is now S1 (SUP-11); P1's row then has `supplierWhy == ""`.
  - `TestSUP12_ProductAndSupplierPage`: product detail `supplierId == 0`, `ruleSupplierName == "S1"`; supplier S1
    detail `products == 0`, `productsByRule == 1`.
  - `TestSUP12_NeverClaimsLotsExport`: a damage write-off of P1 → its loss row's `supplier_id` is NULL; P1's lot
    supplier is empty; products sheet export row for P1 has an empty supplier column; importing that sheet back keeps
    `preferred_supplier_id` NULL.
  - `TestSUP12_ReorderRhythm`: three S1 deliveries of another product, run the reorder calc for P1 → `RhythmFrom ==
    "supplier"`.
  (Check the exact reorder / export / losses routes in `internal/api` before writing the calls.)
- [ ] **Step 2: Run** `go test -exec bin/testdb ./plugins/productplus/battle -run SUP12` → FAIL.
- [ ] **Step 3: Implement** the read sites above; leave `losses.go`, `LotSupplierSQL`, `admin_import.go`,
  `import_diff.go`, `loose.go` untouched.
- [ ] **Step 4: Run** the same → PASS; `make test` → PASS (core unchanged without the plugin).
- [ ] **Step 5: Commit** `feat(suppliers): the rule supplier in Low stock, orders, product, supplier page and reorder levels (SUP-12)`

### Task 5: Moving a supplier's products moves the rules

**Files:**
- Modify: `internal/plugin/hooks_product.go` (`SupplierMover{Move func(ctx context.Context, tx *sqlx.Tx, fromID, toID
  int64) error}`, `AddSupplierMover`, `SupplierMovers()`)
- Modify: `internal/features/products/hooks.go` (`var SupplierMoved func(ctx, tx *sqlx.Tx, from, to int64) error`),
  `repository_write.go` `ReassignPreferredSupplier` runs in one transaction and calls it; `internal/app/plugins.go`
- Modify: `plugins/productplus/suppliers.go` (`UPDATE pp_supplier_map SET supplier_id = $2 WHERE supplier_id = $1`)
- Test: add `TestPPF13_MoveProductsMovesRules` to `plugins/productplus/battle/suppliers_db_test.go`

- [ ] **Step 1: Write the failing test:** rule `bosch → S1`; `POST /api/v1/admin/suppliers/S1/reassign {"toId": S2}`
  → the rule points at S2 and S2's filter has the Bosch products. A mover that errors (core unit test with a fake in
  `internal/features/products`) → no product moved.
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement.** **Step 4: Run** → PASS; `make check-core` → OK.
- [ ] **Step 5: Commit** `feat(suppliers): moving a supplier's products moves plugin rules too (PPF-13)`

### Task 6: Auditor checks

**Files:** Modify `plugins/productplus/ledger.go`; Test: extend `TestLedgerChecksFire` in `battle_db_test.go`.

- [ ] **Step 1: Add cases to `TestLedgerChecksFire`:** `PPF-10` fires on `UPDATE pp_fields SET type = 'number'` for the
  chosen field; `PPF-11` fires on a `pp_supplier_map` row whose `value_key <> lower(btrim(value_key))`.
- [ ] **Step 2: Run** → FAIL. **Step 3: Add** `ledgercheck.Add("PPF-10", …)` (chosen field is text / select) and
  `("PPF-11", …)` (keys are normalised). **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `test(productplus): auditor checks for supplier by field`

### Task 7: Screens

**Files:**
- Create: `web/src/plugins/productplus/SupplierRules.tsx` (section on Product Plus's page), queries in `api.ts`,
  types in `types.ts`
- Modify: `web/src/plugins/productplus/FieldsPage.tsx` (mount the section)
- Modify: `web/src/features/admin/products/ProductForm.tsx` (hint under Preferred supplier: "By Brand: Lanka Auto
  Parts", muted, only when `supplierId == 0` and `ruleSupplierName`), `ProductsPage.tsx` and
  `web/src/features/counter/lowstock/LowStockTab.tsx` (supplier name with a small muted "by Brand" when
  `supplierWhy`), `web/src/features/purchasing/suppliers/SupplierPage.tsx` ("12 own + 30 by Brand"; the Move products
  sheet counts only own)

- [ ] **Step 1:** Section: field picker (eligible fields; changing with rules asks through a confirm dialog, never a
  toast), search box, one row per value — value, product count, `SupplierPick`; admin kit (`PageHead`, `DataList`
  cards at 360 px). Vitest for any pure sorting / filtering helper.
- [ ] **Step 2:** `cd web && bunx tsc -b` and `make test-web` → PASS.
- [ ] **Step 3: UI gate:** `make plugins P=all && make dev`; screenshots of the section, product form hint, Low stock
  and supplier page at 360 px and desktop in Studio light, Night dark and one more palette; check every card's
  contents stay inside at 360 px. Fix what's off.
- [ ] **Step 4: Playwright:** a spec in `web/e2e/` (import `test` from `e2e/fixtures.ts`): pick Brand, set a value's
  supplier, see it on the products list filtered by that supplier. Run it → PASS.
- [ ] **Step 5:** `make core-only`; **Commit** `feat(productplus): Supplier by field screen; rule supplier shown on products, Low stock and supplier page`

### Task 8: Docs, finish

- [ ] **Step 1:** `docs/guide/plugins/productplus.md` (Supplier by field: why — onboarding — and how it fades as
  deliveries come), the purchasing / products guide line about the "by Brand" mark; add the section's shot to
  `web/docs-shots/shots.spec.ts` (fill a field + rules in the Demo Room data if empty) → `make docs-shots`.
- [ ] **Step 2:** `make test-all` → all green (log in `~/.cache/karots-logs/full-*`).
- [ ] **Step 3:** Review the whole change against the spec and AGENTS hard rules; fix findings test-first.
- [ ] **Step 4:** Delete `docs/specs/supplier-by-field.md` and this plan; update `docs/status.md` (Done + Now).
- [ ] **Step 5: Commit** `docs: supplier by field — guide, screenshot, status`
