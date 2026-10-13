# M7.6b — Batches in stock take, intake and merge (core) (design)

Approved by the user on 2026-10-13 ("lets start"), with one change: the merge prompt shows the
defaults below but **lets the person change them**. Core work, no plugin. The UI says **batch**,
the code says lot (`stock_batches`).

## 1. Why

- Every intake restock opens a new `adjust` lot, so a product sold and restocked weekly grows a long list of
  identical batches.
- Stock-take is one row per product: a count on a product with 2+ batches comes off the oldest one (karots-pos's
  known limit — the count sheet had nowhere to write "10 old, 2 new").
- There is no way to add a batch by hand (with its cost / price / expiry) or to fold batches together.

## 2. Batches sheet (Stock → Batches of …)

Today: per batch Set count, Remove, Batch price. New:

- **+ Add batch** — qty, cost (default the product's), price (blank = follows the product), expiry (optional).
  Saves through `stock.Adjust` with `BatchID = 0` plus the new `CostPrice` / `ExpiryDate` fields (an `adjust`
  movement valued at that cost).
- **Merge batches** — tick 2+ batches → an in-place prompt. They merge into the **oldest** ticked batch
  (lowest `created_at`, then id). The prompt is pre-filled and every field can be changed:
  - **Quantity** = the sum (not editable — a merge never changes the count).
  - **Cost** = weighted average of the ticked batches, 2dp. When the person changes it the prompt shows how
    much the stock value moves ("value −Rs 12.40").
  - **Expiry** = the earliest of them (blank when none has one). When they differ the prompt warns
    "the later ones will be sold as if they expire 12 Mar".
  - **Price** = the oldest batch's own price (blank = follows the product); shown as a choice when they differ.
  - One transaction: the kept lot gets the summed `qty_remaining` (and `qty_received` += the others'
    remaining), the cost / expiry / price chosen; the others go to `qty_remaining = 0` and their rows stay
    (sale lines and returns still point at them). `stock.quantity` is unchanged; no stock movement (nothing
    moved). Audit: "merged 3 batches of Panadol into batch #41 (cost 12.40, exp 2027-03-12)".
  - Refused: fewer than 2 batches, a batch of another product, a batch already at 0.

## 3. Stock-take per batch

- A product with 2+ live batches gets a **▸ batches** expander under its row: per batch a count, cost and own selling price (user, 2026-10-13)
  (qty, expiry, cost, own price shown) and **+ batch** (qty, cost, price, expiry).
- While its batch counts are open the product-level count input is disabled (no double apply).
- Save sends `CountLine.batches: [{batchId, counted}]` and `newBatches: [{qty, cost, price, expiry}]`;
  each batch count goes through `Adjust{BatchID}` (the path Set count already uses).
- Single-batch products are unchanged.

## 4. Count sheet

- Export: a product with 2+ live batches gets **one row per batch** with a new `batch` column (the lot id) and
  an `expiry` column; single-batch products keep one row with `batch` blank.
- Import matches barcode + batch. A blank `batch` keeps today's product-level behaviour, so old sheets (no
  `batch` column) still import. A `batch` that isn't that product's live lot is a row error.
- Cost / price columns on a batch row set **that batch's** cost / own price; on a product row the product's,
  as today.

## 5. Stock intake asks which batch

- The restock form lists the product's live batches (qty, cost, price, expiry) as choices plus **New batch**.
- **Default:** a batch whose cost and effective price equal what's typed — and its expiry, when one is typed;
  else New batch. Changing cost / price / expiry re-picks the default until the person picks one by hand.
- Topping up a batch raises `qty_remaining` **and** `qty_received`; a new price stamps only a new batch.
- An optional **Expiry** field on restock and on a new product.
- API: `IntakeRestockRequest.batchId` (0 = new) and `expiry`; `IntakeCreateRequest.expiry`. The session row
  remembers the batch, so **Undo** takes the units off that batch.

## 6. Unchanged

Purchase receive always opens its own batch; returns keep `RestockLot`; the till, FEFO and the "which price?"
prompt are untouched. No migration (`stock_batches` already has `expiry_date`, `qty_received`).

## 7. API

| Method | Path | What |
|---|---|---|
| POST | `/admin/stock/lots/:id/add` | `{qty, cost, sellingPrice, expiry}` add a batch |
| POST | `/admin/stock/lots/:id/merge` | `{batchIds, cost, expiry, sellingPrice}` merge into the oldest |
| POST | `/admin/stock/take` | `CountLine` gains `batches`, `newBatches` |
| POST | `/admin/inventory/intake/restock` | gains `batchId`, `expiry` |

DTOs in `dto_stock.go`, `make types`.

## 8. Checks

- Go (DB): merge keeps the count, weighted cost, earliest expiry, edited values honoured, emptied rows stay
  at 0, refusals; add batch; per-batch stock-take save; sheet export one row per batch and import with and
  without the `batch` column; intake top-up vs new batch (qty_received too), matching default, undo off the
  same batch.
- Web unit: the intake default-batch matcher; the merge defaults (weighted cost, earliest expiry).
- Playwright: Batches sheet add + merge (edit the cost), stock-take expander save, intake picks the matching
  batch then a new one; 360 px for the Batches sheet, the merge prompt and the stock-take expander.

## 9. Added during the build (user, 2026-10-13)

- **Stock-take batch lines** take a cost and own selling price too; **wholesale stays per product** (read-only
  on batch lines — the till never asks which batch for a wholesale sale).
- **Write-off follow-up:** a damage write-off (Batches sheet, Batches page, Stock → Record write-off) asks in a
  prompt — never a toast — whether the supplier will replace it or pay for it; the write-off answers its loss id
  (`ConsumeResult.lossId`); a write-off naming a batch remembers it (`reference_type = 'batch'`).
- **Replacement like intake:** Recover → Replaced it asks which batch (matching default), expiry, selling price
  (a new one moves the product too) and **extra per unit** (0 = free); the batch cost is the lost unit cost +
  extra; the extra × qty is owed to the supplier (`loss_recoveries.extra_charge`, migration `00004`) or paid now
  (counter: cash from the till; admin: a cash locker) — a failed payment leaves it owed, with a warning.
- **Settle where the delivery is received:** admin receive / delivery page and the counter's Suppliers tab show
  the supplier's expired batches and unsettled write-offs (`GET …/suppliers/:id/claims`,
  `POST …/claims/expired/:lot`; the counter also gets `/till/recovery`, `/till/stock/lots/:id`).
- **Supplier statement** shows recovery credits and replacement charges (bug fixed in karots-pos too).
