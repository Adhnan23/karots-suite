# Returns (area 1)

Code: `internal/features/sales/service_returns.go` (PartialReturnTx, ReturnTx, lineRefund), `store_returns.go`
(`PaidFactorSQL`), `internal/api/counter_returns.go` (the cash leaves the drawer in the same transaction), the return
screen `web/src/features/counter/returns/refund.ts`. The grid is in [sales.md](sales.md) (column **Ret**).

## How a return is valued today

- A line's **paid value** = its share of what the goods were paid (`sales.PaidShares`, to the cent; the lines add
  up to the sale exactly).
- Returning **part** of a line refunds paid value × n ÷ qty, rounded; returning **the last** of a line refunds what's
  left of its paid value (paid − refunded so far).
- A whole-sale return = a part return of every line still out.
- The refund first clears what the customer still owes **on this sale** (credit sales), the rest is cash out of the
  returning cashier's drawer.

## Rules

- **RET-1** Nothing comes back that wasn't sold: returned qty ≤ sold qty per line, at all times, including two
  returns of the same sale at once. *ledgercheck, machine.*
- **RET-2** A sale is `returned` exactly when every returnable line is fully back; `partially_returned` when some is.
- **RET-3** The refund for n units is the same on the return screen, the refund slip, the drawer and the database.
  *Today the screen rounds in TypeScript numbers and the server in decimals — TILL-2.*
- **RET-4** A line's refunds never add up to more than what was paid for that line, however it's split (1 + 1 + 1 of
  3; ⅓ kg + the rest); when the line is fully back they equal its paid value exactly. *Found in: d17d7b7 (refunded
  the sticker price over a bill discount).* *ledgercheck, machine, lean.*
- **RET-5** A whole sale's refunds never add up to more than the sale's goods were paid (total − payouts); fully
  returned, they equal it exactly — even when the bill discount doesn't split evenly. *Today: 3 × Rs 100 with Rs 10
  off (paid 290) refunds 96.67 × 3 = 290.01 — each line's share rounds alone.* *ledgercheck, machine, lean.*
- **RET-6** A pack line refunds by what the pack cost, not by unit prices.
- **RET-7** A loose line can come back in fractions; the last fraction takes what's left (RET-4).
- **RET-8** A line sold from a picked lot goes back to that lot; restocked goods keep the price they sold at.
- **RET-9** A frozen product's return doesn't go into stock unless damaged (damaged → in, then written off).
- **RET-10** A returned serial-tracked unit can be sold again, and its warranty ends with the return. *Today the
  serial stays "on record" with the old sale, so selling that unit again is refused ("serial number … is already on
  record").*
- **RET-11** A returned service line puts nothing on a shelf; what it used is booked as a loss at its cost.
- **RET-12** A payout line is never returned.
- **RET-13** A line group (combo / job + add-ons) comes back only whole: every line of the group, in whole sets (2 of 3
  sets = each line's qty × 2⁄3). One line of a combo on its own is refused. *User, 2026-11-18 (Q5). Today any line
  can come back alone.* Whole sets rather than "all sets at once" is the builder's call: it is still the whole combo.
- **RET-14** Tax comes back with the goods in the same share it was paid. *Known gap (tax-registered shops with
  mixed rates only): a line's paid share is by its net, so a taxed line refunds a little less and an untaxed one a
  little more (the sale's total is exact). Closing it needs each line's tax stored at sale time (a migration).*
- **RET-15** The cash part of a refund leaves the drawer of the person doing the return, in the same transaction as
  the return; the drawer never goes below what it held (DRAW-3).
- **RET-16** A card / online sale is refunded in **cash from the drawer**; the card / bank account keeps the money.
  *User, 2026-11-18 (Q1): refunding to the account is messy.* (As today.)
- **RET-17** A cheque sale whose cheque hasn't cleared asks the person doing the return which way: **give the cheque
  back** (the cheque is cancelled, no cash), or **refund in cash and keep the cheque** — if it later bounces, its amount
  goes onto the customer's account as debt (TEND-8). *User, 2026-11-18 (Q2). Today: cash at once, no question.*
  A cleared cheque refunds in cash like any other sale.
- **RET-18** A credit sale's return clears what is still owed **on that sale** first, never more than that; the rest
  is cash. Two returns never clear the same debt twice. *Found in: d17d7b7.*
- **RET-19** A return clears debt only while the customer **still owes it**: never more than their balance at the
  time of the return. If they have paid the credit sale off, the refund is **cash**. *User, 2026-11-18 (Q4). Today it
  lowers their balance instead (an advance).*
- **RET-20** Damaged on return: refunded in full, goods come in and go straight out as damage at cost; sellable stock
  doesn't change.
- **RET-21** Every return has its return record, one row per line, its refund and credit totals = the sum of its
  lines. *ledgercheck.*

## Drawer rules used here (area 2 owns them)

- **DRAW-1** A drawer's expected cash = opening + cash takings − change − cash refunds − cash out, to the cent.
- **DRAW-2** A payout leaves the drawer as change.
- **DRAW-3** Cash out (refunds, payouts) can't take the expected cash below zero **?** (area 2 decides).
- **DRAW-4** Card / online / cheque / on account never change the drawer's expected cash.
