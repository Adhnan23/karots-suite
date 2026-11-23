# Stock: lots, packs, units, conversions, freeze, write-offs (area 4)

Code: `internal/features/stock` (on hand, movements, lots, FEFO, counts, write-offs, give back, recall, freeze, lot
merge), `internal/features/conversions` (runs, make loose, fold into a pack), the stock parts of
`internal/features/sales/service_lines.go` (area 1: sale, found at till) and `service_returns.go` (restock),
`internal/features/products` (opening stock, packs, product merge, valuation), `warranty/service.go` (a replacement
shipped), `internal/api/admin_stock.go`, `admin_stocktake*.go`, `admin_intake.go`, `admin_lots.go`,
`stock_freeze.go`, `counter_stock.go`.

## The words the rules use

- **On hand** (`stock.quantity`) — what the books say the shop holds, in the product's own unit (its **base unit**).
- **Movement** (`stock_movements`) — one row per change to on hand, signed (+ in, − out), with its type (sale,
  return, purchase, adjust, damage, own_use, staff, conversion, purchase_return, warranty_replacement, recovery,
  to_shelf), who, why and its **worth** (`cost`, always ≥ 0: what the goods moved were worth at cost).
- **Lot** (`stock_batches`) — one arrival of a product with its own cost, price (0 = follow the product), expiry and
  batch no; **live** while `qty_remaining > 0`. **FEFO** — first expiry, first out: earliest expiry first, lots with
  no expiry last, then oldest.
- **Count** — setting on hand (or one lot) to what was really there: Adjust, Stock-take, a count sheet, a lot's Set
  count. **Write-off** — stock leaving for a reason that isn't a sale: damage, own use, staff.
- **Frozen** (`products.stock_frozen`) — while a shop starts: what's **out for sale** isn't counted, only what's **in
  storage**. **Pack** (`product_packs`) — a way to sell N base units at once (a Card of 12).
- **Conversion** — opening a source product into another (a 25 kg bag → kg, a card → tablets) at a ratio.

## The grid

Columns: **Do** (what on hand and the movement do) · **Lots** (which lot moves; lots add up to on hand) · **Worth**
(the cost booked, value kept) · **Guard** (never below zero, refusals) · **Undo** (put right) · **Reports**
(movements list, P&L loss lines, stock value, expiry, low stock — sums are area 7) · **Faults** (two at once, power
cut, sent twice).

| Feature | Do | Lots | Worth | Guard | Undo | Reports | Faults |
|---|---|---|---|---|---|---|---|
| Opening stock (product made, sheet, quick-add) | STK-24 | STK-2 | STK-7 | STK-3 | STK-4 | STK-22 | — e |
| Stock added (intake, + Add batch, new lot at a count) | STK-6 | STK-2,6 | STK-7 | STK-18 | STK-4 | STK-22 | — e |
| Count (Adjust, stock-take, count sheet) | STK-4 | STK-2,6 | STK-7 | STK-3,18 | STK-4 | STK-22 | STK-4 |
| Lot counted (stock-take by lot, Set count) | STK-5 | STK-5 | STK-7 | STK-3 | STK-5 | STK-22 | STK-5 |
| Sale, plain / picked lot / pack | SALE-20,21 | SALE-22, STK-21 | SALE-32 | SALE-6, STK-18 | RET (area 1) | STK-22 | STK-23, SALE-50 |
| Found at till (oversold) | SALE-6 | STK-2 | STK-7 | SALE-23 | STK-4 | STK-22 | STK-23 |
| Return restocked | RET-8 | RET-8, STK-2 | RET-20 | — a | — d | STK-22 | — e |
| Write-off: damage / own use / staff | STK-8 | STK-6 | STK-7 | STK-3 | — d | STK-8 | STK-23 |
| Write-off of a frozen item out for sale | STK-8 | STK-2 | STK-8 | — a | — d | STK-8 | — e |
| Give back to the supplier (counted) | STK-9 | STK-9 | STK-9 | — b | — d | STK-8 | STK-23 |
| Recall / undo recall | STK-10 | STK-10,15 | — c | — c | STK-10 | STK-10 | — e |
| Conversion run | STK-11 | STK-11 | STK-11 | STK-3,18 | — d | STK-22 | STK-23 |
| Conversion into a frozen product | STK-13 | STK-13 | STK-11 | STK-3 | — d | STK-13 | STK-23 |
| Make loose version | STK-11 | STK-11 | STK-11 | STK-3 | — d | STK-22 | — e |
| Fold card + loose into a pack | STK-14 | STK-11 | STK-14 | STK-14 | — d | STK-22 | — e |
| Lots merged | STK-15 | STK-15 | STK-15 | — c | — d | STK-22 | — e |
| Lot price / expiry / cost edited | STK-20 | STK-20 | STK-20 | — c | STK-20 | STK-22 | — e |
| Freeze / freeze the shop | STK-12 | — c | — c | — c | STK-12 | STK-12 | — e |
| Put out for sale (frozen) | STK-12 | STK-6 | STK-12 | STK-3 | STK-12 | STK-12 | STK-23 |
| Unfreeze (with what's out) / unfreeze the shop | STK-12 | STK-2 | STK-12 | — a | — d | STK-12 | — e |
| Warranty replacement shipped | STK-19 | STK-6 | STK-19 | STK-3 | — d | STK-8 | STK-23 |
| Product merged into another | STK-16 | STK-16 | STK-16 | — a | — d | STK-22 | — e |
| Pack saved | STK-17 | — c | — c | STK-17 | STK-17 | — c | — e |
| Unit changed (a product's, or a unit's decimals) | STK-18 | — c | — c | STK-18 | STK-18 | — c | — e |
| Products sheet import (preview, then import) | IMP-2,4 | STK-2 | STK-7 | IMP-3 | — d | STK-22 | IMP-1,3 |

- `— a` Only adds stock; nothing to go below zero.
- `— b` Never refused for a difference: what was counted wins (STK-9).
- `— c` No stock moves.
- `— d` No undo: put right by the opposite action (a count, a sale, a new lot), so both stay on the movements list.
- `— e` One action in one transaction; a power cut leaves all of it or none (STK-23), and nothing else touches the
  same rows at once in a way the action reads.

## Rules

### On hand, lots and movements

- **STK-1** Every change to on hand writes one movement of exactly that signed quantity, in the same transaction; on
  hand = Σ the product's movements, always (the auditor's SALE-20 check covers every movement type). A change that
  isn't counted (a frozen sale, pieces out for sale) moves nothing or writes a movement of 0. *ledgercheck, machine.*
- **STK-2** A product's live lots add up to its on hand, always: every way stock comes in opens or tops up a lot, and
  every way it goes out takes from one. Lots carry the cost, price and expiry, so stock outside them is stock nobody
  can price, date or give back. *ledgercheck, machine.*
- **STK-3** On hand and every lot are never below zero. A removal beyond what there is — a write-off, put out for
  sale, a conversion, a warranty replacement — is refused and changes nothing; only a sale goes on, by correcting the
  count up first (SALE-6, found at till). *ledgercheck, example.*
- **STK-6** A removal names its lot or comes off FEFO (STK-21). A named lot that holds too little is refused, never
  quietly made up from another lot. An addition tops up the named lot (its cost, price and expiry stay) or opens a new
  lot at the cost given (blank = the product's cost) and the price given (blank = follows the product). *example.*
- **STK-18** A product sold in whole units (pieces, packets) never holds or moves a fraction: a count, a write-off, an
  addition, a conversion into it and a sale of it are whole numbers or refused. Only units marked decimal (kg, l) take
  fractions — and a service's recipe may use part of one (a packet of tea opened for cups) unless it says whole units
  (user, 2026-11-19: found by the auditor in a Demo Room year). A product can't move to a whole unit, nor a unit lose its decimals, while one of its products holds a
  fraction (count it whole first); a unit whose products have packs can't be given decimals. *User, 2026-11-19 (Q3).
  Today only the sale checks, and unit changes are all allowed.* *example.*
- **STK-21** A sale, write-off or removal that names no lot takes the **good** lots first — FEFO among them: earliest
  expiry first, no expiry last, then the oldest — and an expired or recalled (stop selling) lot only when nothing else
  is left. *User, 2026-11-19 (Q1). Today an expired lot sorts first, so the books say it went while it's still on the
  shelf.* *example, machine.*

### Counts

- **STK-4** A count sets on hand to exactly what was counted: its movement = counted − what the books held at that
  moment, worth that many units at their lot cost (a count's shortfall is a loss the P&L sees). A sale landing at the
  same moment is never lost: on hand still = Σ movements (STK-1) afterwards. Put right by counting again.
  A count replaces stock as of when it's saved, so the stock-take screen and the count sheet say "type it in before
  selling again". *User, 2026-11-19 (Q4).* *example, fault.*
- **STK-5** A lot counted ends holding its count; the product's on hand moves by that lot's difference and no other
  lot changes — even when a sale takes from another lot at the same moment. *example, fault.*
- **STK-24** Opening stock (a product made with stock, a sheet's stock column, a till quick-add) is one lot (source
  opening) and one adjust movement of the same quantity. *example.*

### Worth

- **STK-7** Every movement other than a sale or return records its worth = the units × the cost of the lots they came
  from or went into (a lot with a blank cost at the product's cost): counts, write-offs, found at till, additions.
  Stock value only changes by what the movements say, a lot's cost edit (STK-20) or a merge's rounding (STK-15).
  The worth is kept to the cent, so units worth under half a cent at their lot's cost book Rs 0. *User, 2026-11-20
  (a pack bought at a big discount: 2 pieces at Rs 0.0023).* *ledgercheck, example.*

### Write-offs, giving back, recall

- **STK-8** A write-off is damage, own use or staff — each its own movement type and P&L line, never an adjustment;
  any other reason is refused. Its worth comes off profit. A frozen product's item that was out for sale is first
  added (to_shelf) and then written off: on hand unchanged, the loss still booked (`99fadca`). *example.*
- **STK-9** Giving a lot back: what was counted on the shelf wins over the books. The lot ends at 0; the hand-over is
  a damage write-off that remembers the supplier (and waits on them, area 6). A difference first moves between the
  product's lots (next to sell first) at a blended cost, so no value appears or vanishes; only what the other lots
  can't cover changes on hand, as an adjustment. It never stops for a difference. *example, fault.*
- **STK-10** A recall marks a lot (today's date; stop selling or not); stock and money don't move. A stop-selling lot
  is offered at the till after expired ones (`85c211d`) and stays in stock until given back. Undo clears both marks.
  *example.*

### Conversions and packs

- **STK-11** A conversion run takes `from` units of the source and adds what was really weighed out (blank = from ×
  ratio, 3 decimals) to the destination, in one transaction. Each source lot opened becomes a destination lot with its
  expiry, batch no and its value spread over its share, so value out = value in (± 0.01 a lot) and the loss of a
  short weigh-out shows in the loose cost (`1471981`). Not enough source is refused. *example, machine.*
- **STK-13** Into a frozen destination: pieces put out for sale aren't added (a movement of 0; the run says "out for
  sale"), pieces into storage are; when the source is frozen too nothing is asked — out for sale. *example.*
- **STK-14** Folding a card + loose pair opens every card on hand into the loose product (as a run), adds a pack of the
  ratio's size at the card's prices, moves the card's barcodes to that pack and turns the card and the conversion off.
  Value unchanged; refused when the card shows less than nothing, the ratio isn't a whole number above 1 or the loose
  unit is decimal. *example.*
- **STK-17** A pack holds a whole number (above 1) of a whole unit; a pack line moves size × packs base units
  (SALE-21). *Today a size of 2.5 is accepted.* *example.*

### Lots

- **STK-15** Merging lots keeps the oldest; it ends with all their units at the value-weighted cost (4 decimals) and
  the earliest expiry unless the admin set others; the rest go to 0 and stay as rows (sales and returns point at
  them). On hand doesn't change and no movement is written. A recalled lot can't be merged ("give it back or undo the recall first").
  *User, 2026-11-19 (Q2). Today the oldest lot's marks win.* *example.*
- **STK-20** Editing a lot's price (0 = follows the product), expiry or cost moves no stock and writes no movement; a
  cost edit changes stock value and the cost of what is sold from it next. *example.*

### Freeze

- **STK-12** A frozen product's sales, returns (unless damaged, RET-9) and warranty replacements don't move stock;
  deliveries, counts, write-offs from storage and conversions do. Put out for sale takes units out of stock (to_shelf,
  refused beyond on hand); unfreezing adds what's out for sale now as a to_shelf lot at the product's cost. Freezing
  and unfreezing the shop change no stock. to_shelf is neither a loss nor an adjustment: no report sums it. A frozen
  product never shows as low. *example, machine.*

### Elsewhere

- **STK-16** Merging a product into another: the kept product's on hand becomes the sum of both, and the duplicate's
  lots and movements move with it, so STK-1 and STK-2 hold for the kept product. *example.*
- **STK-19** A warranty replacement takes one unit at its FEFO cost as a warranty movement (worth booked), refused
  when there is none; a frozen product's unit comes off the shelf (a movement of 0, its cost still booked). *example.*
- **STK-22** The stock value (Stock page, Net position) = Σ live lots × their cost, plus any on hand outside lots at
  the product's cost; frozen products count what's in storage. The movements list shows every movement with who and
  why. *Report sums are area 7.* *example.*
- **STK-23** Two removals of the last units at the same moment never both pass (the guarded decrement); a power cut in
  any stock action leaves all of it or none of it; every rule above still holds after. *fault.*

### The products sheet (IMP)

Products → Sheets ▾ → Import (user, 2026-11-23: fix a shop's whole list from a sheet, safely, at the shop). Code:
`internal/api/admin_import.go`, `import_diff.go`, `web/src/components/admin/ImportSheet.tsx`.

- **IMP-1** Choosing a sheet previews it: the real import, run and thrown away. The preview changes nothing in any
  table, plugins' included — no product, category, supplier, barcode, lot, stock or custom-field value. *example.*
- **IMP-2** Import does exactly what the preview showed: the same products updated and added, the same changes, the
  same skipped lines; it sends the very file that was previewed (a copy taken then), and if it still skips a line the
  preview didn't, the result says so. *example.*
- **IMP-3** An import is all or nothing: one transaction for the sheet, a savepoint per line. A line that fails is
  skipped whole — nothing it made (a category, a supplier, a barcode, opening stock) stays; a database failure or a
  power cut leaves the shop as it was. Tricky: a line that fails inside the product write itself, after it made a
  category (nested savepoints: each is released after its rollback); a plugin that swallows a failed statement (its
  savepoint is undone, the import goes on); a product a till holds at that moment is skipped after half a second
  with its reason, never waited on (never stop a sale). *example, fault.*
- **IMP-4** Only the columns a sheet fills change (a sheet of names and categories leaves barcodes, packs, prices,
  suppliers and stock alone), and the preview lists, for each product, exactly the fields that change, old → new —
  plugin columns included when the plugin is in the shop. *example.*
- **IMP-5** Customers' and suppliers' sheets import as before (no preview). *example.*
