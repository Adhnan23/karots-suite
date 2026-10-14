# M7.7a — Packs: sell by the card or by the tablet (design)

Asked by the user on 2026-10-14 while rethinking pharmacy (M7.7): "conversion can bring many bloated products inside
medicines … a card can have 8 or 10 or any kind of tablets … sometimes they ask 2 or 3, sometimes 2 days' worth".
A loose twin per card ("Panadol (loose)" + a conversion + a cut before every sale) doubles the catalogue. The user
approved the recommended way ("do the recommended way … anyone can use it easily"). **Core feature, plugin-free**:
cigarettes (packet / stick), eggs (tray / egg) and sachet strips need it with no plugin; pharmacy (M7.7b) builds on it
through seams only. Conversions stay for real repacking (a rice bag → kg, where weight is lost).

## 1. The model

- A product keeps its stock, batches, costs, sale lines, returns and reports in its **smallest unit** (tablet) —
  nothing that reads `quantity` changes.
- It may carry **packs**: `name` (Card, Box, Packet, Tray, Strip, Bottle — free text, suggestions), `size` (> 1, how
  many units in one), `sellingPrice` (0 = follows the product: size × unit price), `wholesalePrice` (0 = the
  selling price), optional **barcodes** (the manufacturer's code on the card).
- New table `product_packs (id, product_id → products ON DELETE CASCADE, name varchar(30), size numeric(12,3) CHECK
  > 1, selling_price numeric(12,2) DEFAULT 0, wholesale_price numeric(12,2) DEFAULT 0, set_at_till bool DEFAULT false,
  created_at)`, `UNIQUE (product_id, lower(name))`.
- Pack barcodes reuse M7.6a: `product_barcodes.pack_id → product_packs ON DELETE CASCADE` (NULL = an extra code of the
  product itself). The one-code-one-product trigger already covers them; `barcodeIs` finds the product.
- Migration `00005_product_packs.sql`. A karots-pos backup has no packs and restores unchanged.

## 2. Price: full packs at the pack price, automatically (best for the customer)

One rule, the same in Go (`sales`) and TS (`cart/totals.ts lineGross`), table-tested on both sides:

```
packGross(qty, unit, packs, saleType):
  for each pack, largest size first:
    price = saleType wholesale && pack.wholesale > 0 ? pack.wholesale : pack.selling   (0 → size × unit)
    n = floor(rest / size); if price < size × unit: gross += n × price else gross += n × size × unit
    rest -= n × size
  gross += rest × unit           → round 2
```

- `unit` is what the line rings at today (selling / wholesale / a picked lot's own price). A pack only ever lowers
  the line; a pack priced above size × unit is ignored for price but still shown in the split.
- Typing 14 tablets of a 12-card → "= 1 Card + 2 · Rs 250 + 2 × 26". Typing 24 → "= 2 Cards".
- Discounts unchanged (fixed = per unit × qty, percent off the line, clamped). Tax on the net, unchanged.
- `sale_items` gets `pack_label text` (NULL when no pack fits): the split as sold ("1 Card + 2"), so receipts and
  history read it without recomputing. `unit_price` stays the unit price (receipt "each" shows it), `subtotal` is the
  exact packed gross − discount, so refunds (already pro-rated on paid value) stay exact.
- Returns, oversell, FEFO, lots, serials, groups: all in units, untouched.

## 3. Till

- `TillProduct.packs: [{id, name, size, sellingPrice, wholesalePrice}]`; the barcode lookup also returns
  `scannedPackId` when the code is a pack's.
- **Tap / search / Enter on a product with packs** → a small chooser in place of the instant add: big buttons
  **1 Tablet · Rs 26** · **1 Card ×12 · Rs 250** (· **1 Box ×120**), keys 1 / 2 / 3, Esc = back. (The price-pick
  prompt for lots still comes first when needed, then the chooser.)
- **Scanning a pack's barcode** adds that pack (size units) straight away — no chooser.
- **Cart line** of a product with packs: the qty stays units with − / + (one unit); under it a chip row:
  `= 1 Card + 2` and **+ Card** (adds one pack of units). Unit price line: "Rs 26 each · Card Rs 250".
- **No pack yet** (the cashier is asked for "2 cards" of something never set up): a dashed **Pack…** chip on lines of
  whole-unit, non-service products opens a prompt (never a toast): *"How many <unit> in one [Card]? [__] · Sell [1]
  card(s)"* → `POST /till/products/:id/packs {name, size}` creates the pack (price 0 = follows the unit price,
  `set_at_till = true`) and sets the line to count × size. Never blocks the sale.
- The ⓘ product info lists packs ("Card ×12 · Rs 250").

## 4. Admin

- **Product form → Packs** section (stocked products only): rows name / size / price / wholesale / barcode, add,
  remove; "set at the till" tag on a till-made pack. Each pack saves on its own (like Other barcodes,
  `/admin/products/:id/packs`). Whole-unit products only. The form shows "Stock 34 tablets = 2 Cards + 10".
- **Items to review** also lists products with a pack set at the till ("Card size set at the till — check it");
  **Looks fine** and saving the product form clear `set_at_till`.
- **Stock / product lists and the stock-take count rows** show the split hint next to on-hand.
- **Purchases (LineEditor: entry, receive, counter) and intake**: a product with packs gets a per-line **in** chip
  (`tablet · Card ×12 · Box ×120`). Picking a pack makes Qty and Cost per pack; the request carries `packSize`; the
  server stores units (qty × size), cost per unit (cost ÷ size, 2 dp) and keeps the line **subtotal = qty × pack
  cost exactly** (what the supplier invoices). Free qty and ordered qty follow the same factor.

## 5. Fold an existing twin (karots-pos card + "(loose)" product)

- On a product that is the **destination of an active conversion** (the loose one), the form footer gets
  **Fold <card> into packs…**: one sheet showing card name, size (= the conversion ratio), card price, barcodes.
- One transaction: convert all of the card's stock (`conversions.Run`, batches / expiry / cost carried, M7.6d); create
  the pack (size = ratio, prices from the card); move the card's main + extra barcodes onto the pack; turn the
  conversion off; turn the card product off. History stays on the card (its old sales were cards — still true).
- Plugin rows are not rewritten (no quantity-changing hook exists); the sheet says "set its alternatives group etc.
  on <loose> if needed".

## 6. Checks

- Go: `packGross` table test (12-card ×14, 24, no pack fits, pack dearer than units, wholesale, two pack sizes);
  sale with a pack line → exact subtotal + pack_label, stock in units, refund of part of it exact; pack barcode
  scan; till pack creation; purchase with `packSize` (units, unit cost, exact subtotal); fold (stock moved with
  expiry, barcode on the pack, card + conversion off, one tx).
- Web unit: `lineGross` with packs mirrors Go on the same table; split label.
- Playwright: tap → chooser → Card; type 14 → split + price; scan card barcode; Pack… at the till → review list;
  product form packs; purchase entry by the card; fold. 360px admin, 4 palette combos, till 1366.

## 7. Out of scope

Packs inside a pack's price tiers per customer, labels for pack barcodes, stock-take typed in packs (hint only),
packs on service products.
