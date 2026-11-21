# Reload & Bills (RCH)

Code: `plugins/recharge` (store.go: carriers, devices, rates, own products; packages.go; float.go: float rows,
balances, counts; hooks.go: the sale, return and wallet-tender hooks; api.go: the till's menu, drawer counts,
buying float; api_admin.go / api_report.go: setup, float page, ledger, report; earnings.go: P&L income, money
places, readiness, Activity), `web/src/plugins/recharge` (the till's 📲 menu, the wallet tender, the Returns field,
drawer counts, the admin pages). Writes only `recharge_*` and its own service products; every cash side is core's
(the sale, a supplier bill and payment, a locker move).
Tables: `recharge_carriers`, `recharge_devices`, `recharge_rates`, `recharge_packages`, `recharge_transactions`
(float rows), `recharge_counts`, `recharge_bill_tx` (bills paid from / cash outs into a bank), `recharge_config`
(own products); old ones read only: `recharge_device_sessions`, `recharge_carrier_sessions`, `recharge_topups`,
`recharge_bank_cards`, `recharge_card_tx`.

## The words the rules use

- **Device** — a SIM / phone / terminal of one carrier holding a **float** (e-money). Its **balance** is its last
  old close plus every float row since.
- **Move** — a float row from a sale line: a reload or package (float out), a bill paid or wallet deposit from a
  device (float out), a cash out (float in), a wallet tender (float in). **Commission** — what the carrier pays on a
  move (rate: percent + fixed; a package: its price − the float it uses), booked as its own float row linked to it.
- **Face value** — what the phone sent: the line's price × qty, before any discount.
- **Bank leg** — a bill paid from / cash out into a bank locker: core's locker ledger moves (recorded, never
  refused for a low balance), the bill detail kept in `recharge_bill_tx`.
- **Count** — what the phone shows against the balance; the difference is booked as a `count` float row.
- **Refill** — float bought from a dealer: a core supplier bill (on credit, paid now, or by cheque) and the float in,
  plus any bonus as commission.

## The grid

Columns: **Do** · **Till** (📲 menu, Pay) · **Sale** (core money) · **Float** · **Return** · **Earnings** (P&L,
report) · **Faults**.

| Feature | Do | Till | Sale | Float | Return | Earnings | Faults |
|---|---|---|---|---|---|---|---|
| Carriers, devices, rates | RCH-1 | RCH-1 | — a | RCH-1 | — a | — a | PLG-3,4,5,6 |
| Packages (admin / added at the till) | RCH-2 | RCH-2 | — a | — a | — a | — a | PLG-5,6 |
| Reload / package | RCH-3 | RCH-3 | RCH-3 | RCH-4 | RCH-6 | RCH-9 | PLG-4,6 |
| Bill payment / wallet deposit (device) | RCH-3 | RCH-3 | RCH-3 | RCH-4 | RCH-6 | RCH-9 | PLG-4,6 |
| Bill payment / cash out (bank) | RCH-5 | RCH-3 | RCH-5 | — a | RCH-6 | — a | PLG-4 |
| Cash out (device) | RCH-3 | RCH-3 | RCH-3 | RCH-4 | — b | RCH-9 | PLG-4 |
| Wallet tender (sale / credit payment) | RCH-7 | RCH-7 | RCH-7 | RCH-4 | — c | RCH-9 | PLG-4,5 |
| Count (drawer open / close, admin) | RCH-8 | RCH-8 | — a | RCH-8 | — a | RCH-9 | PLG-5, at once |
| Refill (counter / admin) | RCH-10 | RCH-10 | RCH-10 | RCH-10 | — a | RCH-9 | PLG-4,5 |
| Merge (a carrier's / own product) | RCH-11 | — a | — a | — a | — a | — a | PLG-2 |

a — not touched. b — a payout line isn't returned. c — the refund is core's (cash); the wallet money stays on the
device.

## Rules

```
RCH-1  A carrier has one pass-through service product its reloads and packages ring on; an enabled carrier's
       name is unique (case ignored). A device belongs to one carrier for ever (its float is that carrier's),
       is for reloads, money or both, and its opening float is booked once as 'opening' (no expense, no
       drawer move). Rates are 0–100% and amounts ≥ 0, per carrier and kind.
       Checked by: example, refusals.

RCH-2  A package belongs to one carrier, its price above 0 and the float it uses between 0 and its price; an
       active package's name is unique per carrier — added twice (or at once) at the till is one package. One
       added at the till is flagged until an admin reviews or saves it.
       Checked by: example, faults (at once).

RCH-3  A recharge line is an ordinary sale line: core's sale, payment and drawer rules hold and the auditor
       passes; pass-through face value is never revenue. Each line is booked once, in the sale's
       transaction: the float moves by its face value (user, 2026-11-21: a discount comes out of the shop's
       money, never the float's) (a reload, bill or deposit: price × qty out; a package:
       the float it uses × qty out; a cash out: its amount in) and its commission is booked linked to it. A
       line whose device or bank doesn't exist, or whose kind is unknown, is refused (the till never sends
       one); a low or negative balance never refuses (the money already moved on the phone).
       Checked by: example, ledgercheck (each line booked once, by its face value; commission linked).

RCH-4  A device's balance is its last old close plus every float row since, and is what the Float page, the
       till, money places and readiness show.
       Checked by: example, ledgercheck (money places agree).

RCH-5  A bank leg moves core's locker ledger by the line's amount (bill: out; cash out: in), linked to the
       sale, recorded (never refused for a low balance), with one bill row per line. A bank (or device)
       turned off since the line was rung still books — the money already moved (user, 2026-11-21); only one
       that doesn't exist is refused.
       Checked by: example, ledgercheck (the bill row and the locker move agree).

RCH-6  A return of a reload, package, bill or deposit line puts back what the carrier (or bank) returned:
       all (default), none (a wrong number: the float is gone, the commission was earned) or an amount, never
       more than left for the units returned; the commission on what came back is taken back. Returned in
       parts, the whole line never gives back more than left — the last part takes what remains. The refund
       itself is core's.
       Checked by: example, ledgercheck (float back ≤ float out per line; commission back ≤ commission).

RCH-7  A wallet tender puts the amount on the named device, with the carrier's wallet-payment commission, in
       the sale's (or the credit payment's) transaction; no device = refused.
       Checked by: example, refusals.

RCH-8  A count books exactly the difference between what the phone shows and the balance at that moment: a
       sale on the same device at the same time is either before it (in its expected) or after it — never
       counted twice or missed. A count below 0 is refused.
       Checked by: example, faults (a count and a sale at once), ledgercheck (count row = counted −
       expected).

RCH-9  Earnings for a range = commission rows + count differences (+ old data's service charges and closed
       sessions' bonus); face value never counts, a service charge is ordinary revenue, buying float is not
       a cost (old float expenses are added back).
       Checked by: example.

RCH-10 A refill is one change: a core supplier bill for the amount (with the dealer's invoice), paid now from
       a cash place / bank or by cheques for what's owed after the dealer's advance, or on credit; the float
       up by the amount, plus any bonus as commission. A cashier pays only from their own drawer, a cashier
       locker or a bank cashiers may use.
       Checked by: example, refusals, faults (the failure sweep).

RCH-11 A product merge moves a carrier's product and the plugin's own products to the kept one (PLG-2).
       Checked by: example, ledgercheck (no row names a merged-away product).
```
