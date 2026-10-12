# M7.5 — Print & Copy (documents) (design)

The user approved the rethink on 2026-10-12: **A–I all as recommended** ("do whatever you recommended").
Plugin key stays `documents` (old backups carry `goose_db_version_documents` and `doc_*`); the display name is
**Print & Copy**.

## 1. Why it changes

karots-pos built Print & Copy first and core Services & Recipes later, so the plugin carries a second service
list (`doc_service` hub), a second bill of materials (`doc_consumable`, with a dedup guard against recipes), a
reversal that books a fake expense (core returns refused service lines), a labour worklist that duplicates
expenses tagged to a service, and its own receipts tab / activity feed / money report. In the suite the
plugin sits **on top of** core: it adds only what core can't know — the size × colour × side price grid with
quantity tiers, paper by size, double-sided sheet maths and the till job panel.

**Bulk discounts (user, 2026-10-12):** mass jobs get cheaper through the grid's quantity tiers ("from 100
copies Rs 6"); a one-off deal is the till's ordinary line discount (% or Rs), which a Print & Copy line keeps.
Combos are not involved (they are sets of different products).

## 2. What it does (user chose A–I)

- **A. One service list.** A print service **is** a core service (`products.is_service`), listed in core
  Services & Recipes like any other. `doc_service` stays only as the link: `product_id`, `kind`
  (metered | custom), `category` (copy | print | laminate | bind | other), `is_active`.
  Apps → **Print & Copy** lists the print services; **New print service** creates the core service and its
  link in one step. Name / price / category edit through core's `ProductForm`; toner, ink and other
  size-independent use through core's `RecipeEditor` (both reused, plugin → core imports).
- **B. Price grid.** One row per size (or one "Any size" row), four cells: B&W 1-side · B&W 2-side · Colour
  1-side · Colour 2-side; a blank cell = not offered. **Quantity tiers** are "from N copies" bands under a
  size, each with its own four cells. Same `doc_price` rows as karots-pos (size NULL = any; the greatest
  `min_qty ≤ qty` wins; a size-specific row beats the NULL-size fallback).
- **C. Paper by size.** Each size row names its paper (how much one sheet uses, default 1; any-size row =
  fallback when a size has none of its own). Paper is counted **per sheet**, whole units: double-sided =
  ⌈copies ÷ 2⌉ sheets. Everything used **per copy** (toner, ink) is the core recipe, fractional. One resolver
  (`resolve.go`); a product named both ways is taken once, as paper.
- **D. Till.**
  - A **🖨 Print & Copy** menu chip opens the service list → the job panel: size buttons, colour and side
    toggles (only those the grid offers), qty, the live price ("Rs 8.00 each · Rs 160.00", tier named when
    one applies), **Enter** adds, **Shop use** (H).
  - Picking a print service any other way (a group card, a scan, the Sell screen's search — which now finds services) opens
    the same panel instead of adding a bare line. **Core seam (generic):** `TillMenuRoot.claims?(p: TillProduct)` — the till asks each
    menu root before adding a product and opens the claiming root's panel with that product; the plugin
    claims by the **🖨** badge it gives its services (`ProductBadgeProvider`).
  - The line reads **"Photocopy A4 B&W 2-side ×20"**, qty = copies, unit = tier price, components = paper +
    recipe totals. Its qty is **fixed** (the tier price and paper depend on it): − / + are hidden, × removes.
    **Core seam (generic):** `ServiceLineInput.fixedQty` → `CartLine.fixedQty`; `setQty` leaves such lines
    alone. Line discount (F9 / the line's discount) works as on any line.
  - A short paper count raises core's existing "a material reads zero" found-at-till prompt.
- **E. Custom jobs** (CV, photo edit, poster): typed price + optional note → line "CV — Nimal", qty 1, no
  components. No worker, no wage at the till.
- **F. Labour: dropped.** Paying a designer is a core expense tagged to the service (counter Expenses tab),
  so Service profit shows the net. Old `doc_payout` rows stay in the database (their expenses are already
  in core).
- **G. Mistaken jobs go through core Returns.** **Core change (generic):** a service line can be returned —
  money back (same refund maths as any line), **nothing restocked**, and what the line consumed stays a cost:
  the return books a `damage` stock movement on the service product with quantity 0 and cost =
  `cost_price × returned qty`, note "used by a returned service line (S-…)", so P&L Losses carry the wasted
  paper while revenue and COGS drop like any return. Whole-sale return no longer refuses a sale with a
  service. Counter Returns: service lines are returnable (no "send to damage" choice). Applies to recipe
  services too (a spilt coffee refunded). A veto hook for recharge waits for M7.10.
- **H. Shop use.** The job panel's **Shop use** (in-app confirm) consumes that job's paper + recipe as
  `own_use` through core `Stock.Consume` and logs a `doc_job` of kind `own_use` (price 0).
- **I. Report.** Core **Service profit** stays the money report. Reports → More → **Print & Copy**: for a range,
  per service × size × colour × side — jobs, copies, sheets, revenue (returned copies taken off), and shop-use
  sheets + cost; CSV. Jobs from before the suite show as "earlier jobs" per service. Dropped: the plugin's
  receipts tab (core sales search matches line descriptions) and activity feed.

## 3. Data

- karots-pos `00001_documents.sql`, `00002_labour_per_job.sql`, `00004_consumables_to_core.sql`,
  `00005_job_kind_reversal.sql` and the karots-pos fix `00006_fractional_consumables.sql` **byte for byte** (no
  00003 in karots-pos either).
- **New `00007_suite.sql`:**
  - `doc_job` + `sale_item_id BIGINT REFERENCES sale_items ON DELETE SET NULL`, `size VARCHAR(20)`,
    `color BOOLEAN`, `double_side BOOLEAN`, `sheets DECIMAL(14,3)` — all NULL on old rows.
  - karots-pos 00004 copied every NULL-size consumable into recipes (`whole_units = true`) **and** left it in
    `doc_consumable`. 00007 splits them: a fractional one (toner, 0.0002 per copy) becomes recipe-only with
    `whole_units = false`; a whole one is paper and stays, and its recipe copy is deleted (it would count the
    sheets again, per copy).
  - **karots-pos bug fixed here:** rounded up to whole units, that fractional toner took a whole cartridge per
    job — the `.Ceil()` bug recipes were built to fix. Fixed in karots-pos too (`56e6fcd`: its resolver plus 00006, copied here so a backup at
    version 6 restores cleanly).
- `doc_job` rows are written **after** the sale (`tillAfterSale`, best-effort like combos):
  `POST /x/documents/jobs {saleId, jobs[]}`; the server links each job to its sale line (same sale, product
  and description, not yet linked) and refuses jobs that match no line.
- An old backup restores with no mapping beyond 00007 (`restore_db_test.go`).

## 4. API (`/api/v1/x/documents`)

| Who | Route | |
|---|---|---|
| cashier | `GET /till` | active services with their grid + sizes (one call for the panel) |
| cashier | `GET /quote?service&size&color&double&qty` | unit, total, tier, description, components, sheets |
| cashier | `POST /own-use` | H |
| cashier | `POST /jobs` | after-sale ledger |
| admin | `GET /services`, `POST /services`, `PUT /services/:id`, `POST /services/:id/active` | list (incl. disabled), create (core service + link), kind/category, on/off |
| admin | `PUT /services/:id/grid` | replace prices + paper in one tx |
| admin | `GET /report?from&to`, CSV | I |

## 5. Out of scope

Tap-a-line-to-edit (remove and add again), per-worker labour, a recharge return veto (M7.10).
