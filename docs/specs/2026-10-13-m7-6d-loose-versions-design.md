# M7.6d — Loose versions and honest conversions (design)

Asked by the user on 2026-10-13 during the M7.6 review: "sometimes we create the same product twice if we sell it
loose" (a 25 kg rice bag, then "Rice (loose)" by the kg, then a conversion). Approved: "do these" — all of §1–5.
Core work except §5 (plugins). Builds on M7.6b batches.

## 1. One place: Make a loose version…

- A **Make a loose version…** button in the product form footer (beside Duplicate / Merge into…), for a stocked,
  non-service product that is saved. Opens one sheet:
  - **Name** (prefilled "<name> (loose)"), **Unit** (core units; must differ from the source's unit),
    **1 <source unit> = N <loose unit>** (the ratio, > 0), **Selling price** per loose unit (free — the shop sets it),
    **Barcode** (blank or Generate), **Convert now: [ ] <source unit>** (optional, 0 = don't).
  - Category, supplier, tax, reorder, warranty, local name are copied from the source (as Duplicate does).
  - The shown **cost per loose unit** = source cost ÷ ratio (read-only hint; the real cost comes from the batches
    on each conversion, §3).
- **One transaction:** create the product (core `products.Create` with the copied fields), create the conversion
  (`conversions.Create`), run the first conversion if asked (§3/§4). Any failure → nothing saved.
- A loose version that already exists (an active conversion source → a product of that unit) is not blocked —
  the sheet just says "Already sold loose as <name>" with a link; the shop may want two (kg and 500 g).
- Plugin data follows through the existing duplicate path (§5): the request carries `sourceId` so each plugin's
  `ProductExtras` / form section can copy from the source.

## 2. Loose price: free (already true)

The loose product has its own selling price. Only its cost is derived. No change.

## 3. Cost from the real bag, with the real yield (instead of "extra cost")

- Today `conversions.Run` spreads the consumed source cost over `fromQty × ratio`. A 25 kg bag often gives ~24.6 kg.
- **Run takes an optional actual yield** (`toQty`): blank = `fromQty × ratio` (as today); typed = what was really
  weighed out (> 0). The consumed cost is spread over the actual yield, so each loose unit costs a little more and
  the loss is in the cost, not hidden. `conversion_runs.to_qty` already records what came out.
- **No "extra cost" in money** (packing bags, labour): it would need a real money source (cashflow rule) or would
  double-count an expense. Left out on purpose; revisit only if the user asks.
- The counter's Convert tab and the admin Conversions page both get the optional "Got" field (prefilled with the
  expected amount; showing the difference "0.4 kg less than 25 — cost per kg Rs 2.03 higher").

## 4. Batches and expiry follow the bag (a bug today)

- Today a run opens **one** destination batch with **no expiry**, however many source batches it consumed, so loose
  stock from a bag expiring next week shows no expiry and FEFO / expiry prompts miss it.
- New: a stock helper `DepleteFEFOLots(ctx, productID, qty) ([]ConsumedLot, avgCost, error)` returning each consumed
  batch (id, qty taken, cost, expiry, batch no) — `DepleteFEFO` becomes a thin wrapper so every other caller is
  unchanged. A run opens **one destination batch per consumed source batch**: qty = its share of the yield
  (`taken / fromQty × toQty`, rounded to 3 dp, the last batch takes the remainder so the total is exact), cost =
  that batch's cost × taken ÷ its share (so value moves exactly), **expiry and batch no copied**, source
  `conversion`, own selling price 0 (follows the loose product's price).
- Stock value conservation: Σ destination value = Σ consumed value (± 0.01 per batch rounding). Tested.

## 5. Plugins: copy across, but alternatives only on purpose

- **Product Plus:** the loose version gets the source's field values (its form section already loads the source's
  values for a duplicate — pass `sourceId`).
- **Alternatives:** the loose version starts **outside** the source's group, with a tick **"Also offer as an
  alternative to <name>"** (off by default) — because a till swap keeps the quantity.
- **Alternatives swap across units (fix, user: "swap warns when unit is different"):** `AltDTO` carries `unit`
  (abbreviation) and `sameUnit`. A same-unit alternative swaps as today. A different-unit one shows its unit on the
  chip ("⇄ Rice (loose) · kg") and, on tap, **asks how many** in a prompt ("How many kg of Rice (loose)?", required
  > 0; never a toast) before swapping — the cart line becomes that product at that quantity. Same in the
  found-at-till prompt (`tillShortHints`). The group page shows a small "kg" badge on members whose unit differs
  from the group's first member.

## 6. Checks

- Go: `DepleteFEFOLots` (two batches with different expiry/cost → exact per-batch takes); Run with actual yield
  (cost per unit up, value conserved) and with two source batches (two destination batches, expiry + batch no
  copied, Σ qty = toQty, Σ value = consumed value ± 0.01·n); the loose-version endpoint (one tx — a bad ratio or a
  unit clash leaves no product behind; convert-now runs). Alternatives: `unit` / `sameUnit` in the till alts.
- Web unit: swap of a different-unit line sets the asked qty (`swapProduct` with qty).
- Playwright: make a loose version with Convert now 1 → the loose product has stock 25 with the bag's expiry and
  the bag lost 1; Product Plus values copied; not in the alternatives group unless ticked; a run with "Got 24.6"
  shows the higher cost; a till swap to a different-unit alternative asks the quantity. 360px for the sheet.
- `docs/differences.md` rows; no karots-pos routes (suite-only), but **check karots-pos's conversion run for the
  same no-expiry flaw** — if present it is a domain bug: fix in both (bug-fix commit only in karots-pos).
