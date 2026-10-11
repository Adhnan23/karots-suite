# M7.2 Product Plus — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: use superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> Same shape as M7.1. Opus executes and reviews, one commit per step, and UI gates are self-reviewed (4 combos, 1366 + 360).

**Goal:** Custom product fields, reinterpreted. Fields can be scoped to categories, filled in bulk, and
"required" means something (missing products are listed). Edits that would surprise are guarded, and
sheets carry custom fields behind a toggle. Core gains a Skip disabled toggle on product export.

**Spec:** `docs/specs/2026-10-11-m7-2-productplus-design.md`. Fork source: karots-pos `plugins/productplus/` (read-only).

## Global constraints

- **Core never references a plugin and never changes for one plugin.** The core changes are the generic
  seams in spec §3 (Tasks 1–3) and the core feature F (Task 3). `make check-core` must pass after every
  task. Core tests use a fake hook named `fake`.
- `cmd/server/enabled_plugins.go` and `web/src/plugins/enabled-plugins.ts` are committed **core-only**.
  Use `make plugins P=productplus` (or `P=all`) to work, and `make core-only` before committing.
- `make types` after any `dto*.go` change.
- The plugin's JSON types live in `plugins/productplus/api.go`, mirrored by hand in
  `web/src/plugins/productplus/types.ts`.
- Errors go through `apperr`, JSON is camelCase, and files are ≤ 500 lines.
- Plugin pages use the admin kit and theme tokens only, and must work at 360px in Studio / Night ×
  light / dark.
- No Claude / AI attribution in commits.

## Review focus

1. **A karots-pos backup with `pp_fields` / `pp_values`.** It must restore unchanged, every old field
   must apply to every product, and nothing old may read as missing (Task 9).
2. **Extras.** A refused `Validate` must write nothing. An unknown extras key → 400. A core-only build
   must ignore an empty `extra` (Task 1).
3. **Scope.** Subcategories inherit. A product moved out of scope keeps its values but hides them on
   every surface: search, ⓘ, label, meta, form, grid, export (Task 4).
4. **Default change and type change.** "Only new products" must leave every existing in-scope product
   reading exactly what it read before. A value someone chose that equals the default is kept. A lossy
   type change without `confirmLoss` must change nothing. A yes/no unticked against a Yes default stores
   "" (Task 4).
5. **Sheets.** With both toggles at their defaults, the export is byte-identical to today's (Task 3).
6. **A cashier on plugin routes** gets 403 (Task 5).

---

### Task 1: Core seam — product extras (form + intake)

**Files:** `internal/plugin/hooks_product.go`, `internal/api/{dto_catalog.go,admin_products.go,admin_intake.go,admin_products_test.go}`, `internal/api/product_extras.go` (new), `web/src/plugins/slots.ts`, `web/src/features/admin/products/ProductForm.tsx`, `web/src/features/admin/intake/IntakePage.tsx`.

- [ ] **Step 1:** in `hooks_product.go`, replace the `url.Values` hooks `ProductFormValidate` / `ProductSaved` (unused in the suite, templ-era) with:
  - `ExtraCtx{ProductID, CategoryID int64; IsNew bool}`;
  - `ProductExtras{Key string; Validate, Save func(ctx, ExtraCtx, json.RawMessage) error}`;
  - `AddProductExtras` / `ProductExtrasHooks()`.
  Update the doc comments.
- [ ] **Step 2:** `ProductRequest.Extra map[string]json.RawMessage \`json:"extra,omitempty"\`` (tygo → `Record<string, unknown>`, set via a tygo type override if needed), then `make types`.
- [ ] **Step 3: failing test** (`admin_products_test.go`). Register a `fake` extras hook in the test (registry helper, cleaned up after):
  - Create with `extra.fake` that Validate refuses → 400 and no product row.
  - Create with an accepted extra → Save is called with the new id and `IsNew` true.
  - Update → `IsNew` false.
  - Unknown key `nope` → 400.
  - Intake create passes `extra` through too.
- [ ] **Step 4: implement `product_extras.go`:**
  - `checkExtras(ctx, req, ectx) error` runs before the write;
  - `saveExtras(ctx, req, ectx) error` runs after the write; a failure answers 500 naming the key, and the product is saved (as in karots-pos);
  - call both from create, update and intake create.
- [ ] **Step 5: React slot.** Add `productFormSections: [] as ProductFormSection[]` to `slots.ts`, where
  `ProductFormSection = { key; Section: ComponentType<{ productId: number; sourceId: number; categoryId: number; where: 'form' | 'intake'; value: unknown; onChange: (v: unknown) => void }> }`.
  - `ProductForm` keeps an `extra` state, renders each Section in a block after the barcode / supplier grid (`sourceId = editing.copyOf`), and sends `extra`.
  - Intake `NewItem` does the same with `where="intake"`.
  - With no sections, nothing renders and the body has no `extra` key.
- [ ] **Step 6:** `go test ./internal/api/...`, `bunx tsc -b`, `bun run test`, `make check-core`.
- [ ] **Step 7: commit** `feat(products): plugin extras on the product form and intake — checked before the save, stored after it (generic seam)`

### Task 2: Core seam — list line

**Files:** `internal/features/products/products.go` (func var `MetaProvider`), `internal/app/plugins.go`, `internal/api/{dto_catalog.go,admin_products.go,admin_products_test.go}`, `web/src/features/admin/products/ProductsPage.tsx`.

- [ ] **Step 1: failing test.** With `products.MetaProvider` set to a fake, `GET /admin/products` rows carry `meta`; without one, `meta` is "".
- [ ] **Step 2: implement.**
  - `products.MetaProvider func(ctx, ids) map[int64]string`, bridged in `bridgeProductHooks`. Parts from several providers are joined with " · ", and an error skips that provider.
  - `AdminProductRow.Meta` is filled for the page's ids in **one call**.
  - `make types`.
  - ProductsPage shows `meta` as a faint line under the name, truncated, in both table and cards.
- [ ] **Step 3:** tests, `tsc -b`, `check-core`. **Commit** `feat(products): a plugin can add a short line under each product in the list (generic seam)`

### Task 3: Core — sheet columns seam + Skip disabled (UI gate)

**Files:** `internal/plugin/hooks_product.go`, `internal/app/plugins.go`, `internal/features/products/{service.go,products.go,service_import.go}`, `internal/api/{admin_import.go,product_sheet_extras.go (new),admin_import_test.go,dto_*.go}`, `web/src/components/admin/ImportSheet.tsx`.

- [ ] **Step 1: failing tests.**
  - Defaults (no params): the export is the same as today (snapshot the header and one row).
  - `disabled=1`: a disabled product is included and the header ends with `active`; importing that sheet back keeps it disabled; `active=yes` on import re-enables.
  - With a `fake` sheet-column hook:
    - `GET /admin/products/sheet-extras` → `[{key:"fake",label}]`;
    - `extras=fake` adds its columns to the export and template;
    - the import calls `Import` with only those cells and the product id, and its note joins the line note;
    - without `extras`, those columns are ignored.
  - Core-only: `sheet-extras` → `[]`.
- [ ] **Step 2: implement.**
  - `ListAll(ctx, includeDisabled)`: update every caller.
  - `ImportRow.Active *bool`, applied only when it's given.
  - `ImportResult.ProductID`.
  - `plugin.ProductSheetColumns` per spec §3.3.
  - `product_sheet_extras.go` holds the route and a helper `sheetExtras(c)` that parses `extras=`; refuse unknown keys.
- [ ] **Step 3: UI.** In `SpreadsheetMenu` (products only), inside the dropdown:
  - **Skip disabled** (`DropdownMenuCheckboxItem`, default on);
  - one **Include <label>** per sheet extra, from `GET sheet-extras` (not fetched for customers / suppliers).
  - Both are remembered in `localStorage` (`ks-sheets-products`, wrapped in try / catch) and passed on the export links, the template link and the import upload.
  - The import sheet shows "Custom fields: included / not included" when an extra exists.
- [ ] **Step 4:** tests, `tsc -b`, `check-core`.
- [ ] **Step 5: UI gate.** Shots of the Sheets menu, core-only and with the plugin in, at 1366 + 360 in 4 combos.
- [ ] **Step 6: commit** `feat(products): Sheets ▾ — Skip disabled (off adds disabled products with an active column) and plugin sheet columns behind a toggle (generic seam)`

### Task 4: Product Plus Go — migrations + store

**Files:** `plugins/productplus/{plugin.json,productplus.go,store.go,scope.go,values.go,store_db_test.go}`, `plugins/productplus/migrations/{embed.go,0001_productplus.sql,0002_field_meta.sql,0003_drop_print_on_receipt.sql,0004_show_in_intake.sql,0005_field_scope.sql}`.

- [ ] **Step 1: migrations.**
  - Copy `0001`–`0004` and `embed.go` from karots-pos **byte for byte**.
  - Write `0005_field_scope.sql` per spec §4, with a Down migration.
  - `plugin.json`: key `productplus`, name "Product Plus", import `karots-suite/plugins/productplus`, version `2.0.0`, and a description that mentions scope, Fill in, missing and sheets.
- [ ] **Step 2: failing DB test** (`store_db_test.go`).
  - Setup: skip without DATABASE_URL, run the plugin migrations on `goose_db_version_productplus`, make its own categories (parent + child + other) and products ("PP Test <nanos>"), and clean up after.
  - Scope:
    - A field scoped to the parent applies to a product in the child but not to one in other.
    - Move a product to other → its value is hidden from `FormFields`, `TillRows`, `MetaFor` and `MatchProductIDs`, and the row is still in `pp_values`; move it back → it shows again.
  - Missing:
    - A required text field with a blank default → products with no value are missing, counted.
    - A required field with a default → 0 missing.
    - A yes/no field → never missing.
    - A disabled product is not counted.
  - Values:
    - `SetValues` with a value equal to the default **keeps** a row.
    - Default change, `new`: products with no row get the old default stored, so they read the same;
      `all`: they read the new one; both run in one tx.
    - A yes/no with a Yes default set to "" stores "".
    - A bad number / option / date → per-cell error while the others save.
  - Type change:
    - `Impact` counts `lossy` (text → number: values that don't parse; → yes/no: anything but "1" / "";
      → text / long text: 0).
    - `UpdateField` with `lossy > 0` and no `confirmLoss` → `ErrDataLoss`, nothing changed.
    - With `confirmLoss`, the fitting values stay and the rest are deleted, in one tx.
  - Search: a value substring; a product with no row is found by its **default**; a yes/no by label when Yes only; out of scope → not found. `MetaFor` also shows the default for a product with no row.
  - `Grid(fields, categoryID (+ subtree), missingOnly, q, page)` returns rows and total, with `null` for reads-default.
- [ ] **Step 3:** `make plugins P=productplus && go test ./plugins/productplus` → FAIL.
- [ ] **Step 4: implement.**
  - `store.go`: field CRUD (karots-pos `fieldColumns` + scope), move, active, `KeyExists`/slug, counts (`valueCount`, `missing`, `usingDefault`), with the default change (`new` / `all`) and the type change (`Impact`, `confirmLoss`) in `UpdateField`, all in one tx.
  - `scope.go`: `appliesSQL` (the recursive-CTE helper from spec §4, one place) and `SetScope` (replace rows, in a tx with the field write).
  - `values.go`: `FormFields(productID, categoryID, where)`, `SetValues([]Cell) (saved, []CellError)`, `coerce(field, raw)` shared by the form, grid and sheets (yes/no words → "1" / ""), and `MetaFor`, `TillRows`, `LabelRows`, `MatchProductIDs` (from karots-pos, plus scope), and `Grid`.
  - `productplus.go`: `init` → `plugin.Register`, `Key()` "productplus", `Migrations()`, data group (three tables). Hooks come in Task 5.
- [ ] **Step 5:** PASS, then `make core-only && make check-core`.
- [ ] **Step 6: commit** `feat(productplus): store — karots-pos tables kept byte for byte, fields scoped to categories (subcategories inherit), missing counts, Fill in grid, typed values with per-cell errors; a default change asks new or all, a lossy type change needs a confirm`

### Task 5: Product Plus API + hooks

**Files:** `plugins/productplus/{api.go,api_admin.go,hooks.go,api_test.go}`, `productplus.go`.

- [ ] **Step 1: failing test** (httptest, the requests `api_test.go` pattern):
  - A field with an empty label → 400; a dropdown with no options → 400; a lossy type change without `confirmLoss` → 409 with `{lossy}`; `GET /fields/:id/impact` counts match.
  - `PUT /values` with one bad cell → 200 with `errors` of length 1, and the rest saved.
  - The extras `Validate`:
    - a blank required field present in the payload → 400 naming the label;
    - a required field absent from the payload (not shown, e.g. on intake) → ok.
  - `Save` with `IsNew` and a copied value → stored.
  - Sheet `Import` with a bad date → note "Expiry: not a date", other cells saved.
  - Cashier → admin route → 403.
- [ ] **Step 2:** FAIL. **Step 3: implement** spec §5.
  - Extras payload: `{values: {fieldKey: string}}`. Only the keys sent are validated and saved, and a field out of the product's scope is ignored.
  - Register `ProductExtras`, `ProductSheetColumns` (columns = active field labels; export / import through `coerce`, scope-aware), `ProductSearchContributor`, `ProductMetaProvider`, `ProductDetailContributor` and `ProductLabelFieldProvider`.
  - Every admin write is audited (`product_field`; Fill in records "set N values").
- [ ] **Step 4:** PASS with the plugin in, then core-only `check-core`.
- [ ] **Step 5: commit** `feat(productplus): JSON API at /api/v1/x/productplus and its hooks — form extras, sheet columns, search, list line, till ⓘ, label lines; audited`

### Task 6: Admin — Apps → Product Plus: fields (UI gate)

**Files:** `web/src/plugins/productplus/{index.tsx,types.ts,api.ts,FieldsPage.tsx,FieldSheet.tsx,coerce.ts,coerce.test.ts}`.

- [ ] **Step 1:** re-read karots-pos `pages.templ` and list every field and action; the screen must carry all of them.
- [ ] **Step 2: unit test** `coerce.test.ts`:
  - yes / no / true / false / 1 / 0 / blank;
  - number with a comma → error;
  - date `2026-13-01` → error;
  - dropdown not in options → error.
  FAIL, then implement (the TS twin of Go `coerce`, used by the section and the grid).
- [ ] **Step 3: build.**
  - **`FieldsPage`:** `PageHead` "Product Plus" with tabs **Fields · Fill in** (`?tab=`) and Show disabled.
  - **Field rows / 360 cards:**
    - label, type chip, scope ("All products" or category names), Where-it-shows chips, required / searchable chips;
    - **"14 missing"** links to Fill in (`?tab=fill&field=&missing=1`);
    - ▲▼, Edit, Disable / Enable.
  - **`FieldSheet` (`FormSheet`):**
    - label;
    - type: changing it calls `impact`, and when values would be lost a warning block says "12 of 40
      products' values don't fit Number and will be deleted. This can't be undone". Save then needs a
      confirm (`ConfirmButton`, destructive). Text ↔ long text never warns;
    - options (one per line; a removed option still in use warns "3 products keep it");
    - default: when it changes on an existing field, a choice appears with **Only new products**
      preselected ("existing products keep <old>") or **Change existing products too** ("N products will
      become <new>");
    - help text, required, searchable;
    - Where it shows chips (Till ⓘ · Label · Stock intake);
    - scope: All products or categories (a multi-pick built on the core `CategoryPicker`).
  - **`index.tsx`:** Apps tile (`ListPlus`, "Product Plus", "Extra fields for your products") and `adminPages`.
- [ ] **Step 4:** `tsc -b`, `bun run test`, `check-core`.
- [ ] **Step 5: UI gate.** Shots of the list (with missing counts and a scoped field), the sheet (lossy type warning, default new / all choice, scope picked), at 1366 + 360 in 4 combos. Match Requests / Clearance.
- [ ] **Step 6: commit** `feat(productplus): Apps → Product Plus — fields with category scope, where-it-shows chips, missing counts, default change for new or all products, a warning before a type change loses data`

### Task 7: Product form + intake section (UI gate)

**Files:** `web/src/plugins/productplus/{FieldsSection.tsx,FieldInput.tsx}`, `index.tsx`.

- [ ] **Step 1: build.**
  - **`FieldInput`:** one control per type (text, textarea, number `inputMode="decimal"`, `Toggle` for yes/no, select with "X (removed)", date). Shared with the grid.
  - **`FieldsSection`:**
    - load `GET /form?productId=<productId || sourceId>&categoryId=&where=`, refetching when `categoryId` changes;
    - heading "More details";
    - show the hint under each field and a required star;
    - `onChange({ values })` sends every shown field (blank as "") so the server sees what was shown;
    - a client-side required check comes from the `required` attribute.
  - Register in `slots.productFormSections`.
- [ ] **Step 2:** `tsc -b`, `check-core`.
- [ ] **Step 3: UI gate.** Shots of the product form (new, edit, duplicate, category switched in and out of scope) and intake New item, at 1366 + 360 in 4 combos.
- [ ] **Step 4: commit** `feat(productplus): custom fields on the product form and intake — only the fields for the product's category, kept on a duplicate`

### Task 8: Fill in grid (UI gate)

**Files:** `web/src/plugins/productplus/{FillIn.tsx,FillRow.tsx}`.

- [ ] **Step 1: build.**
  - **Bar:** field chips (multi at ≥ md, single at 360), `CategoryPicker` filter, **Missing only**, search, `Pager`.
  - **Rows:** product name + category, then one `FieldInput` per chosen field. A `null` value shows the default as a placeholder. Edited cells get a `--selected` ring.
  - **Sticky footer:** "N changes · Save". A save sends `PUT /values`, and errors mark their cell with the message.
  - Leaving with unsaved changes asks first.
  - At 360px: cards, one field.
- [ ] **Step 2:** `tsc -b`, `check-core`.
- [ ] **Step 3: UI gate.** Shots of Missing only with dirty and errored cells, at 1366 + 360 in 4 combos.
- [ ] **Step 4: commit** `feat(productplus): Fill in — set a field across many products, missing only, per-cell errors`

### Task 9: e2e, restore, parity, docs, review

- [ ] **Step 1: e2e** (`web/e2e/plugin-productplus.spec.ts`, importing `test` from `./fixtures`, serial). Steps from spec §6; the seed is SQL in `beforeAll` and touches only its own rows.
- [ ] **Step 2:** core e2e for Skip disabled in `admin-data.spec.ts` (or the products spec), so it runs core-only too.
- [ ] **Step 3:** a full run. Kill :3100 / :5173, then `make e2e-seed && cd web && bunx playwright test`; every spec stays green with all plugins in.
- [ ] **Step 4: restore** (`plugins/productplus/restore_db_test.go`, the requests pattern). A karots-pos-shaped archive with `pp_fields` (a required text with a default, a yes/no, a dropdown) and `pp_values`. Expect:
  - the rows load;
  - `pp_field_categories` is empty and every field applies to every product;
  - the missing count is 0;
  - search finds a value.
  Also run `TestRestoreKarotsPosBackup` (opt-in) with the plugin in.
- [ ] **Step 5: parity.** Tick the 8 `plugins/productplus` rows with "→ /api/v1/x/productplus…" notes (`form` / `table` → client-side sheet / list).
- [ ] **Step 6: milestone review.**
  - Run `make verify`, `go test ./...` with plugins (`-p 1`) and core-only, `bun run test`, Playwright, and `make build && make verify-portable`.
  - Read the whole diff and check every file is ≤ 500 lines.
  - Put fixes in one `fix:` commit.
- [ ] **Step 7: docs.**
  - `status.md`: M7.2 done, NEXT M7.3 combos (rethink first).
  - `differences.md`: rows for A–F.
  - `AGENTS.md` layout: the new slot `productFormSections`.
  - Commit `docs: M7.2 done — Product Plus`.
