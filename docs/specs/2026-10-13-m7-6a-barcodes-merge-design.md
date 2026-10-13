# M7.6a — Extra barcodes & Merge products (core) (design)

Approved by the user on 2026-10-13 ("yes perfect"). Core work, no plugin needed. M7.6 is split three ways:
**M7.6a** this (extra barcodes, merge products) → **M7.6b** lots in stock take / intake / merge lots →
**M7.6c** the AI Assistant (`docs/specs/2026-10-13-m7-6c-aicatalog-design.md`, whose duplicate finder opens
this merge).

## 1. Why

A product has exactly one barcode (`products.barcode`, unique) and sale lines point at the product (no name
copy). A shop that entered the same item twice — two barcodes, two names — can't fix it today: disabling one
makes its barcode scan "not found" a few weeks later, and its stock and sales stay split. The same item also
genuinely arrives with two barcodes (new packaging, a second supplier).

**Decided with the user:**
- Barcodes belong to the **product**, not to a lot. The till depletes earliest-expiry first, not by the code
  scanned, so "how many left under the old code" could never be trusted.
- Barcodes are **never dropped automatically**: a forgotten tin with the old code must still sell (never stop a
  sale). Each barcode shows when it was last scanned; the person removes one when they choose.
- The till never links barcodes; an unknown scan stays a quick-add (Items to review), cleaned up later.

## 2. Extra barcodes

- **Data:** core migration `00003_product_barcodes.sql`:
  - `product_barcodes (id, product_id → products ON DELETE CASCADE, barcode varchar(50) NOT NULL UNIQUE,
    source varchar(20) NOT NULL — intake | receive | form | merge | import, added_at, last_scanned_at NULL)`.
  - `products.barcode_scanned_at timestamptz NULL` (the main barcode's last scan).
  - A trigger on both tables refuses a code already used by the other (one code → one product, in the DB).
- `products.barcode` stays the **main** barcode: labels print it, everything that shows "the barcode" shows it.
- **One lookup:** `products.Repository.FindByBarcode` / `BarcodeExists`, `search.go`'s exact-barcode match and
  ranking, and the import matcher all check main **or** extra. A scan at the till (`TillProductByBarcode`) and
  at intake stamps `last_scanned_at` / `barcode_scanned_at` (one cheap UPDATE).
- Product rows (`AdminProductRow`, till product) carry `barcodes: string[]` (extras). The web helper
  `hasBarcode(p, code)` replaces every `p.barcode === term` (intake, ProductPicker, purchase LineEditor, till).
- **Where a barcode is added:**
  - **Stock intake:** an unknown scan offers **New product** (today) or **Same as an existing one → add this
    barcode** (ProductPicker), which adds it (source `intake`) and opens that product's restock.
  - **Purchase entry / receive (LineEditor):** an unknown scan offers **+ New product** (today) or **Same as an
    existing one** — adds the barcode (source `receive`) and the line for that product.
  - **Product form:** a Barcodes block — the main barcode, then each extra with "added · last scanned 4 months
    ago" (or "never scanned"), **Make main** (swaps them) and **Remove** (in-place confirm). **Add barcode**
    field (dup-checked: "used by Sprite 1L").
  - **Products sheet:** an **Extra barcodes** column (`;`-separated) in export, template and import. Import adds
    new ones, never removes.
- `GenerateBarcode` skips codes used as extras.

## 3. Merge products

- **Where:** Products → a product's row menu → **Merge into…** (ProductPicker for the product to keep), and
  **Items to review** → **Same as an existing one** (merges the quick-added item into the picked product).
  M7.6c's duplicate finder opens the same screen with both filled.
- **Refused (clear message, nothing changes):** same product; different units (pcs vs kg would corrupt
  quantities); a service with a stock item; the duplicate is in a **held bill** ("held by Nimal at 10:42 —
  resume or clear it first").
- **Confirm screen** (in-place, mobile-friendly) shows what moves: "Coca Cola 1 Ltr → Coke 1L: 1 barcode,
  8 in stock (2 lots), 41 sales, 3 purchases". When the two **selling prices differ** and the duplicate has
  lots that follow its price, one choice: **Sell those 8 at Rs 300 (stamp the lot)** / **Sell at Rs 320**
  (default: the kept product's price). Not undoable — the screen says so.
- **One transaction** (`products.Merge(ctx, fromID, intoID, opts, userID)`):
  - Barcodes: the duplicate's main + extras become extras of the kept product (source `merge`, scan dates kept).
  - Stock: `stock_batches.product_id` → kept; `stock.quantity` kept += duplicate's; duplicate's row → 0. Lots
    keep their cost, price and expiry (lot merging is M7.6b).
  - History → kept: `sale_items`, `sale_return_items`, `purchase_items`, `purchase_return_items`,
    `stock_movements`, `loss_recoveries`, `warranty_units`, `conversion_runs` (from / to),
    `expenses.service_product_id`.
  - Set-up rows → kept, doubles collapsed: `product_group_items` (one per group), `product_recipes` both sides (a
    clash keeps the kept product's line), `product_recipe_costs` (label clash keeps the kept product's),
    `product_conversions` (a pair between the two is deleted; a clash keeps the kept product's).
  - **Plugins** move their own rows through a new generic hook `ProductMerger{Key, Merge(ctx, tx, fromID,
    intoID) error}` — implemented now by alternatives, combos, productplus, documents, clearance and requests
    (each collapses its own doubles). Core never names them.
  - The duplicate stays as a row (`is_active = false`, `barcode = NULL`, new column `merged_into_id`) so
    anything still pointing at it (audit entries, old backups' references) resolves. It is hidden from
    Products, pickers and "Show disabled", and shows "Merged into Coke 1L" where it still appears.
  - Activity + audit: "Merged Coca Cola 1 Ltr into Coke 1L (8 in stock, 41 sales)".
- Reports, receipts and statements need no change: they read the product by id, so past sales now show the
  kept name — the same item.

## 4. API

| Method | Path | What |
|---|---|---|
| POST | `/admin/products/:id/barcodes` | `{barcode, source}` add an extra |
| DELETE | `/admin/products/:id/barcodes/:code` | remove an extra |
| POST | `/admin/products/:id/barcodes/:code/main` | make it the main barcode |
| GET | `/admin/products/:id/merge/:into` | the confirm preview (counts, price clash, refusal reason) |
| POST | `/admin/products/:id/merge/:into` | `{lotPrice: 'keep' | 'kept'}` merge |

DTOs in `dto_catalog.go` (`ProductBarcode`, `MergePreview`, `MergeRequest`), `make types`.

## 5. Data & restore

- karots-pos backups have no `product_barcodes` / `merged_into_id` / `barcode_scanned_at`: restore leaves
  them empty / NULL (core already tolerates missing tables and columns). No mapping.
- `restore_db_test.go`-style check in core: a baseline backup restores into 00003.

## 6. Checks

- Go (DB): extra barcode found by the till lookup, search, import; the trigger refuses a code used on the other
  table; make main swaps; merge moves every table above (one assertion per table), collapses doubles, stock sums,
  lot price stamp option, refuses held / unit / service cases, plugin hook called with every plugin compiled in.
- Web unit: `hasBarcode`.
- Playwright (`products-merge.spec.ts`, `barcodes.spec.ts`): add a barcode at intake via "Same as an existing
  one" and scan it at the till; Make main / Remove in the form; merge two products then scan the old barcode at
  the till → the kept product, Reports show one row; Items to review → Same as existing; purchase LineEditor
  unknown scan → existing; 360 px for the form's Barcodes block and the merge screen.
