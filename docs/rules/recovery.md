# Recovery and losses: warranty replacements, what a supplier makes good (area 6)

Code: `internal/features/recovery` (record a recovery, the supplier's claims list, the losses list),
`internal/features/warranty` (serial and by-receipt replacements), `internal/api/admin_reports_ops.go` (`POST
/recovery`, Losses & Recovery), `counter.go` (the counter's `/recovery`), `supplier_claims.go` (claims, give back,
told, recall), `counter_warranty.go`. The losses themselves are booked by area 4 (write-off STK-8, give back STK-9,
warranty shipped STK-19); a credit's effect on bills and the balance is checked by area 5 (SUP-1, PUR-8). Report sums
(the P&L's Losses and Recoveries lines, the Losses & Recovery page's totals) are area 7.

## The words the rules use

- **Loss** — stock that left the shelf with nothing coming in for it: a **damage write-off** (`stock_movements.type =
  'damage'`, a give back included — it remembers the supplier in `given_back_to`) or a **warranty replacement** (a
  unit handed out free, `warranty_replacement`). Its **worth** is that movement's cost.
- **Warranty unit** (`warranty_units`) — a serial sold under warranty, or a by-receipt item claimed (no serial).
  **Active** until it is **replaced** or **returned**; **under warranty** while active and today is on or before its
  last day (`warranty_until`, a date).
- **Claim** (`warranty_claims`) — one replacement: the faulty unit, the new unit (serial claims), who and why.
- **Recovery** (`loss_recoveries`) — what the supplier did about one loss: a **replacement** (units back on the
  shelf, maybe with an **extra charge** the supplier asked on top), **paid** (a **credit**: the shop owes them less; no
  money moves) or **written off** (nothing; the loss stands). **Recovered** = what came back in worth.
- **Claims list** — a supplier's page of things to settle when they call: recalled lots, expired and close-to-expiry
  lots still holding stock, and write-offs of their stock still waiting on them.

## The grid

Columns: **Do** · **Stock** (lots and movements, area 4) · **Supplier** (balance, bills, credit — area 5's SUP-1,
PUR-8) · **Loss book** (the loss's recovery, the claims list, the warranty register) · **Undo** · **Faults** (two at
once, power cut, sent twice).

| Feature | Do | Stock | Supplier | Loss book | Undo | Faults |
|---|---|---|---|---|---|---|
| Serial unit replaced | WAR-1,2 | WAR-3 | — a | WAR-5 | — d | WAR-6 |
| By-receipt item replaced | WAR-1,4 | WAR-3 | — a | WAR-5 | — d | WAR-6 |
| Unit of a returned sale (serial or by receipt) | WAR-1,4 | — b | — a | — b | — d | — e |
| Replacement of a frozen product | WAR-3 | WAR-3 | — a | WAR-5 | — d | WAR-6 |
| Write-off of a supplier's stock (damage, give back) | STK-8,9 | STK-8,9 | — a | REC-6 | — d | STK-23 |
| Recalled / expired / close-to-expiry lot | STK-10 | — c | — a | REC-6 | STK-10 | — e |
| Recovery: replacement, into a new lot | REC-2 | REC-2 | — a | REC-1,5 | REC-8 | REC-7 |
| Recovery: replacement, into a lot already there | REC-2 | REC-2 | — a | REC-1,5 | REC-8 | REC-7 |
| Recovery: replacement with an extra charge | REC-2 | REC-2 | REC-3 | REC-1,5 | REC-8 | REC-7 |
| Recovery: paid (a credit) | REC-3 | — a | REC-3, PUR-8 | REC-1,5 | REC-8 | REC-7 |
| Recovery: written off | REC-4 | — a | — a | REC-1,5 | REC-8 | REC-7 |
| Who and how much (any supplier, any amount, a disabled one) | REC-3 | — c | REC-3 | — c | — d | — e |

- `— a` Nothing of this kind moves.
- `— b` Refused, so nothing moves (WAR-1, WAR-4).
- `— c` Doesn't touch it.
- `— d` No undo: a wrong replacement is put right by the opposite action (a write-off of the unit handed out).
- `— e` One read-only list or one row nothing else writes at the same moment.

## Rules

### Warranty replacements

- **WAR-1** Only a unit under warranty is replaced: active (never one replaced already or one whose sale line was
  returned) and today on or before its last day, by the shop's date. The server refuses anything else — not only the
  screen; a goodwill swap after the cover ends is a write-off and a sale at Rs 0. *User, 2026-11-20 (Q5): today a
  serial swap of a returned or out-of-date unit goes through if sent straight to the server.* *example.*
- **WAR-2** A serial replacement issues a new unit with a serial no other unit has (any product), for the same
  customer and product, whose warranty continues: the same last day, never a fresh term. The faulty unit becomes
  replaced and names the new one; one claim records both. *example.*
- **WAR-3** A replacement hands out one unit from stock exactly as STK-19: a `warranty_replacement` movement of −1
  (good lots first, STK-21) worth that unit's lot cost, refused when there is none; a frozen product's unit comes off
  the shelf (a movement of 0, the product's cost still booked). Every claim has exactly one such movement, and its
  cost is the loss's worth. *ledgercheck, example.*
- **WAR-4** By receipt, a sale line's replacements never pass the units sold less the units returned (summed over the
  product's lines on that sale), and a void sale has none. Each replacement is one unit with no serial, replaced at
  once, its warranty continuing from the sale. *Today the units returned are not taken off.* *ledgercheck, example.*
- **WAR-5** A replacement is never a sale and never money: no sale, no drawer, no customer balance moves. It is a
  loss (worth WAR-3) that waits on Losses & Recovery for its supplier (REC-1). *example.*
- **WAR-6** Two replacements of the same unit, or of a receipt line's last claimable unit, at the same moment: one
  goes through, the other is refused — never two units out for one fault. A power cut leaves the claim, the units
  and the movement all or none. *Today neither path holds the unit or the line.* *fault.*

### Recoveries

- **REC-1** A loss is settled at most once: one recovery per damage write-off or replaced warranty unit. Its loss
  worth is the source's worth (the damage movement's cost; the warranty claim's movement cost) and its quantity the
  units lost (a warranty unit is 1). Two recoveries of one loss at the same moment: one goes through, the other is
  refused ("already recovered"). A warranty unit that wasn't replaced has nothing to recover. *ledgercheck, example,
  fault.*
- **REC-2** A replacement puts the units lost back on the books in one transaction: a `recovery` movement of the
  loss's quantity that names its recovery, into the lot picked (that product's own) or a new lot with the expiry and
  price given. Each unit is worth the loss ÷ its quantity (4 decimals) plus the extra per unit, so the stock that came
  back is worth the loss + the extra charge to within a cent. Topping up a lot blends its cost: (what it held × its
  cost + that worth) over all its units, 4 decimals. Recovered = the loss's worth. A new selling price moves the
  product's price as at intake. A whole-unit product takes whole units (STK-18). *User, 2026-11-20 (Q3): today the
  movement doesn't name its recovery, and a top-up takes the lot's cost, so the loss and the extra never reach the
  stock's worth.* *ledgercheck, example.*
- **REC-3** Money with the supplier, with no money moving: a **credit** (paid) lowers the supplier's balance by the
  amount (above 0), goes on their open bills oldest first (credited, PUR-11), then the old debt (SUP-2), and only
  what's beyond everything they're owed waits as credit for the next bills (`credit_unapplied`, PUR-8); what it put
  on each bill and on the old debt is kept, and with what waits never passes the amount (the rest went on what's owed
  outside bills, an extra charge). An **extra
  charge** (extra per unit × the units, to the cent; never below 0) raises their balance (SUP-1). Either needs a
  supplier: the form fills in the lot's supplier (changeable; a lot may have none). A credit may be any amount, even
  above the loss's worth (the form shows the worth beside it). A credit for a disabled supplier goes through (like a
  refund, SUP-9); an extra charge for one is refused ("bring them back first"). *User, 2026-11-20 (Q1, Q2).*
  *ledgercheck, example, machine.*
- **REC-4** Written off: nothing moves and nothing is recovered (0); the loss stands and leaves the claims list.
  *ledgercheck, example.*
- **REC-5** What a recovery holds agrees with its outcome: a replacement recovers the loss's worth and may carry an
  extra charge; paid recovers its amount, has a supplier and no extra charge, and what's left of it for the next
  bills is between 0 and the amount; written off recovers 0 and carries nothing. *ledgercheck.*
- **REC-6** A supplier's claims list shows only their own: lots they delivered that are recalled, past their expiry
  or inside their warning days and still holding stock, and their damage write-offs not yet settled — every one given
  back to them, and others of their lots for 180 days. A settled loss and another supplier's lot never appear. Giving
  back, telling and recalling act only on that supplier's live lots. *example.*
- **REC-7** A recovery beside anything else on the same product or supplier — a sale, a delivery, a payment, a
  refund, another recovery — lands whole: the stock, the lot, the balance and the bills move by exactly what each
  says (SUP-8). Lock order as areas 4–5: the loss's row, then stock rows, then lots, then the supplier last. A power
  cut leaves the recovery with all its stock and supplier effects, or none. *fault.*
- **REC-8** A loss's recovery can be undone while nothing came after it: a replacement while no stock of its
  product moved since; a credit (or an extra charge) while nothing since touched the supplier's books — no bill,
  payment, goods sent back, refund or other recovery — and what it left waiting for the next bills is still unspent
  (it is taken back off the bills and old debt it went on, which then owe that again; refused if the opening was
  changed since so the old debt no longer fits, SUP-2); written off always. One recorded before undo existed can't be. Undo takes back exactly what it did —
  a `recovery` movement of the units out at their worth (a top-up's lot back to its cost before), the balance, the
  extra charge — removes the recovery, and the loss waits on the claims list again. Anything else is refused, saying
  what came after. Two undos, or an undo beside a sale of those units, at the same moment: one wins.
  *User, 2026-11-20 (Q4).* *ledgercheck, example, fault.*

## Not checked here

- The P&L's Losses and Recoveries lines and the Losses & Recovery page's totals (sums: area 7).
- The cover by receipt uses the product's warranty months **now**, not at the sale (nothing stores them on the line);
  known, not a money rule.
