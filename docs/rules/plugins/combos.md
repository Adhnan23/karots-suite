# Combos (CMB)

Code: `plugins/combos` (store.go: combos, members, sold rows, badges; api.go; report.go; merge.go),
`web/src/plugins/combos` (pricing.ts: a set's lines and discounts; till.ts: ring, sold rows; match.ts: spot a set).
Tables: `combos`, `combo_items` (members × qty per set), `combo_sales` (one row per combo per sale: sets, net,
discount; name kept), `combos_settings` (one row: auto apply).

## The words the rules use

- **Set** — one of each member at its qty. A combo is rung as `sets` sets: real product lines kept together (a core
  line group, `sale_items.line_group*`), each line carrying its share of the set's discount.
- **Deal** — percent off a set, Rs off a set (`fixed`), or a set price (`price`).
- **On** — active and today within its dates (start / end inclusive). Only an on combo reaches the till (menu, 🎁
  badge, hints, auto apply).
- **Sold row** — `combo_sales`: what a sale's combo lines came to (sets, net after discount, discount).

## The grid

Columns: **Do** · **Till** (menu, badge, hints, auto apply) · **Sale** (core money / stock) · **Return** · **Report**
· **Merge** · **Faults**.

| Feature | Do | Till | Sale | Return | Report | Merge | Faults |
|---|---|---|---|---|---|---|---|
| Create / edit a combo | CMB-1 | CMB-2 | — a | — a | — a | CMB-9 | PLG-3,4,5 |
| On / off, dates | CMB-2 | CMB-2 | — a | — a | — a | — a | PLG-5 |
| Delete | CMB-8 | CMB-2 | — a | — a | CMB-8 | — a | PLG-5 |
| Ring a combo (n sets) | CMB-3 | CMB-3 | CMB-4 | CMB-6 | CMB-5 | — a | CMB-5 |
| Auto apply (setting) | CMB-7 | CMB-7 | CMB-4 | CMB-6 | CMB-5 | — a | PLG-5 |
| A member disabled / a service / no stock | CMB-2 | CMB-2 | CMB-4 | — a | — a | CMB-9 | — b |

a — nothing to move. b — read-only.

## Rules

```
CMB-1  A combo has a name, a deal (percent 0–100, Rs off ≥ 0, or a set price > 0), dates YYYY-MM-DD with the
       end not before the start, and members that exist, are active, aren't services, each once with a qty
       above 0 — at least 2 units in a set. Anything else is refused and changes nothing.
       Checked by: refusals, ledgercheck (stored combos keep their shape).

CMB-2  The till sees exactly the on combos: menu, 🎁 badge and hints name the same ones, and a combo goes off
       by itself after its end date or while switched off. A combo with a disabled or service member shows
       why it can't be sold and isn't offered; a member short of stock is a warning only (never stop a sale).
       Checked by: example.

CMB-3  One set's discount never passes its full price, and its lines' discounts add up to exactly the set's
       discount in whole cents (percent rounded to the cent; a wholesale sale prices members at wholesale
       where they have one). n sets = n × one set.
       Checked by: example (table + random sets: Vitest and the Lean / cases file if the sums are proved).

CMB-4  A combo sale is an ordinary sale of its lines: core's sale, discount, stock and payment rules hold
       (SALE, DISC, TEND, stock out per line), and the auditor passes.
       Checked by: example, ledgercheck (core's checks after every battle test).

CMB-5  A sale's sold rows match its combo lines: per combo, sets = the groups' sets, net = the lines' net,
       discount = the lines' discount — written with the sale (its transaction), once, never for a sale that
       doesn't exist, never doubled by a resend. A void sale's rows leave the counts and the report. The
       till's discounts are trusted like any line discount; the row stores what the sale actually gave.
       (User, 2026-11-21: written by the sale-line hook from the sale's own lines, not an after-sale call.)
       Checked by: example, ledgercheck (sold rows against sale_items).

CMB-6  A combo comes back only whole, in whole sets (RET-13); the refund is what was paid for those sets
       (RET-4, RET-5). The report and the sold counts take returned sets, their net and discount off.
       (User, 2026-11-21.)
       Checked by: example.

CMB-7  Auto apply rings a full set in the cart as the combo exactly as tapping it would (same lines, same
       discounts); off = the cashier is offered it and may skip.
       Checked by: example (Vitest: match.ts / till).

CMB-8  Deleting a combo keeps its sales history: past sold rows keep their name and stay in the report.
       Checked by: example.

CMB-9  A product merge moves the duplicate's member lines to the kept product; a combo holding both becomes
       one line with the two quantities added (PLG-2).
       Checked by: example, ledgercheck (no member names a merged-away product).

CMB-10 The data holds its own shape: deal type and value in range, dates in order, members once per combo with
       qty above 0, one settings row; every sold row names a sale that exists, sets above 0, net and
       discount not below 0.
       Checked by: ledgercheck.
```
