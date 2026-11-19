# Questions for the user

Squares in the grid marked **?** — only the shop owner's way of working can answer them. Each says what the suite does
**today**. Answered ones move into the rule (with the date) and leave this list.

## Area 1 — sales and returns

All five answered 2026-11-18 (RET-13, RET-16, RET-17, RET-19, SALE-35).

## Area 2 — drawer, cash flow, lockers, expenses

All three answered 2026-11-18 (DRAW-3, DRAW-6, CF-2).

## Area 3 — customers

All three answered 2026-11-19 (CUST-15, CUST-16, CUST-17).

## Area 4 — stock

- **Q1 (STK-21) — a sale that names no lot, when a lot is expired or recalled.** The till asks which lot when one is
  expired or recalled, but a sale that names none (an offline phone sale replayed, a plugin's line, a Review sale)
  takes FEFO — and an expired lot sorts first, so the books say the expired box went while it's still on the shelf;
  a recalled "stop selling" lot is taken in its date order too. *Suggest:* with no lot named, take the good lots
  first (FEFO among them) and fall back to expired / recalled ones only when nothing else is left.
- **Q2 (STK-15) — merging a recalled lot.** Merging keeps the oldest lot's marks: a recalled lot merged into a good
  one loses its recall, and a good lot merged into a recalled one becomes recalled. *Suggest:* refuse merging a
  recalled lot ("give it back or undo the recall first").
- **Q3 (STK-18) — changing a unit while stock holds fractions.** A product can be moved from kg to pieces while it
  holds 2.5, and a unit's "decimals" can be unticked while its products hold fractions (or ticked while they have
  packs, which need whole units). Today all of these are allowed. *Suggest:* refuse while any of its products holds a
  fraction (count it whole first), and refuse ticking decimals on a unit whose products have packs.
- **Q4 (STK-4) — a count typed in after selling.** Counting on paper at 9:00 and typing it in at 11:00 sets stock to
  the 9:00 count; whatever sold in between is counted twice (stock too high by it). Today the count simply replaces
  stock when saved. *Suggest:* keep it so (the shop counts when quiet, or freezes while starting), and the count sheet
  says "type it in before selling again" — or, if you'd rather, the sheet remembers what the books said when it was
  printed and only the difference is applied.
