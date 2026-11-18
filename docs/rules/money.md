# Drawer, cash flow, lockers and expenses (area 2)

Code: `internal/features/cashregister` (sessions, drawer moves, expected cash), `internal/features/cashflow`
(`Move/MoveTx` — the one way money moves; ledger + cash-flow totals; `TillLockerLeg`, `SaleNonCashLeg`),
`internal/features/lockers`, `internal/features/expenses`, `internal/features/payout` (supplier pay and expenses from
a place or by cheque), `internal/api/till_drawer.go`, `admin_locker_moves.go`, `admin_expenses.go`,
`counter_expenses.go`, `admin_cashflow.go`.

## The words the rules use

- **Place** — a till (a cashier's open session) or a locker (safe, pocket, bank, card-machine account, Cheques in
  hand). **Outside** (`cashflow.External`) is the far side of money in or out; nothing is written for it.
- **Drawer movement** (`cash_movements`) — the till's ledger: opening, closing (records), pay_in, withdrawal,
  expense, refund, credit_payment. **Locker row** (`locker_ledger`) — a locker's ledger; its balance is their sum.
- **Expected cash** = opening + cash takings − change + Σ drawer movements (pay-ins and credit collected in;
  withdrawals, cash refunds, expenses, supplier cash out). Cash takings and change come from the sales the cashier
  rang since the session opened (`sales.CashCollectedSince`).
- **Receipt** — the CR- money slip (`money_receipts`) every move writes, one per move.

## The grid

Columns: **Do** (what it books) · **Guard** (can it overdraw) · **Place** (which places may take part) · **Undo**
(void / correct) · **Drawer** (expected cash, close, over / short) · **Flow** (Cash flow page: in, out, moved, net;
cash on hand) · **Slip** (the CR- receipt) · **Faults** (power cut, twice, two at once).

| Feature | Do | Guard | Place | Undo | Drawer | Flow | Slip | Faults |
|---|---|---|---|---|---|---|---|---|
| Open, float from a locker | DRAW-9 | LOCK-1 | DRAW-13 | — d | DRAW-1 | CF-1 | LOCK-4 | DRAW-5 |
| Open, float from nowhere | DRAW-9 | — e | — e | — d | DRAW-1 | CF-1 | LOCK-4 | DRAW-5 |
| Cash sale, change | DRAW-1 (area 1) | DRAW-3 | DRAW-13 | RET (area 1) | DRAW-1,6 | CF-4 | — f | DRAW-7 |
| Payout line (sale below zero) | DRAW-2 | DRAW-3 | DRAW-13 | RET-12 | DRAW-1 | CF-4 | — f | DRAW-7 |
| Card / online takings | LOCK-12 | — e | LOCK-11 | RET-16 | DRAW-4 | CF-1 | LOCK-4 | DRAW-7 |
| Cash refund (a return) | DRAW-1 | DRAW-3 | DRAW-13 | — d | DRAW-1 | CF-1 | LOCK-4 | DRAW-16 |
| Withdraw (to a locker / nowhere) | DRAW-10 | DRAW-3, LOCK-1 | DRAW-12,13 | — d | DRAW-1 | CF-1 | LOCK-4 | DRAW-16 |
| Pay-in (from a locker / nowhere) | DRAW-10 | LOCK-1 | DRAW-13 | — d | DRAW-1 | CF-1 | LOCK-4 | DRAW-16 |
| Expense from the till | EXP-1 | DRAW-3 | DRAW-13 | EXP-2 | DRAW-1 | CF-1 | LOCK-4 | DRAW-16 |
| Supplier paid from the till | DRAW-1 (area 5) | DRAW-3 | LOCK-11 | — g | DRAW-1 | CF-1 | LOCK-4 | DRAW-16 |
| Credit collected / advance refunded in cash | DRAW-1 (area 3) | DRAW-3 | LOCK-11 | — g | DRAW-1 | CF-1 | LOCK-4 | DRAW-16 |
| Close, counted cash banked | DRAW-9,11 | — e | DRAW-12,13 | — d | DRAW-1,7 | CF-4 | LOCK-4 | DRAW-7,8 |
| Close, counted only | DRAW-9 | — e | DRAW-12 | — d | DRAW-1,7 | CF-2 | LOCK-4 | DRAW-7,8 |
| Close for them (admin) | DRAW-8 | — e | DRAW-12,13 | — d | DRAW-1,7 | CF-4 | LOCK-4 | DRAW-8 |
| Left open past the day (stale) | DRAW-15 | — e | — e | — d | DRAW-1 | CF-1 | — f | DRAW-7 |
| Locker made with an opening balance | LOCK-10 | LOCK-10 | — e | — d | — h | CF-3 | — f | — i |
| Locker → locker transfer | LOCK-2 | LOCK-1 | LOCK-6,11 | — d | — h | CF-1 | LOCK-4 | LOCK-1 |
| Bank charge / interest / count fix | LOCK-8 | LOCK-1 | LOCK-9 | — d | — h | CF-1 | LOCK-4 | — i |
| Settle card | LOCK-7 | LOCK-1 | LOCK-6 | — d | — h | CF-1 | LOCK-4 | — i |
| Expense from a locker | EXP-1 | LOCK-1 | LOCK-6,11 | EXP-2 | — h | CF-1 | LOCK-4 | — i |
| Expense by cheque | EXP-1 (area 5) | — e | — j | EXP-2 | — h | CF-5 | — j | — i |
| Expense void | EXP-2 | — e | EXP-2 | — d | DRAW-1 | CF-1 | LOCK-4 | — i |
| Expense paid again (cheque came back) | EXP-4 | LOCK-1 | LOCK-11 | EXP-2 | DRAW-1 | CF-1 | LOCK-4 | — i |
| Expense amount changed | EXP-3 | — e | — e | — d | — h | EXP-3 | — f | — i |
| Archived locker | LOCK-5 | LOCK-5 | LOCK-5 | — d | — h | CF-1 | — f | — i |

- `— d` There is no undo: a wrong move is put right by the opposite move (a pay-in for a withdrawal, a transfer back),
  so both stay in the books. Expenses are the one thing that voids.
- `— e` Money comes in, or nothing moves, so there is nothing to overdraw or to pick.
- `— f` Nothing moves money, so no receipt (sales print their own slip, area 1).
- `— g` The customer's or supplier's balance side is area 3 / area 5; only the drawer leg is checked here.
- `— h` No till takes part.
- `— i` One admin action in one transaction; the power cut and double-send of a single tx is covered by DRAW-5 / the
  fault tests of `Move` (LOCK-1).
- `— j` A cheque moves no money until it clears (area 5 owns cheques).

## Rules

### The drawer

- **DRAW-1** A drawer's expected cash = opening + cash takings − change + pay-ins + credit collected − withdrawals −
  cash refunds − expenses − supplier cash − advances refunded, to the cent. The expected stored at close = that sum
  worked out again later from the rows; the difference stored = counted − expected. *Checked by: ledgercheck,
  machine.*
- **DRAW-2** A payout leaves the drawer as change.
- **DRAW-3** Withdrawals, expenses, supplier cash and advance refunds never take the expected cash below zero (refused:
  "the till only has Rs …"; take it from a locker). Money given to a **customer** at the counter — change, a payout, a
  **cash refund** — is never stopped: it may take the expected below zero, and the session is then flagged for review.
  *User, 2026-11-18 (Q1). Today a cash refund beyond the drawer is refused and a sale's change isn't flagged.*
  *example, machine.*
- **DRAW-4** Card / online / cheque / on account never change the drawer's expected cash.
- **DRAW-5** One cashier has at most one open session; opening twice (double tap, two windows) opens one.
  *example.*
- **DRAW-6** Every cash sale lands in exactly one session of its cashier: the one open when it was rung; if that one is
  closed (an offline sale sent after the close), the next one they open — and both sessions are flagged for review
  with the sale's number. *User, 2026-11-18 (Q2). Today the server sells with no open session and that cash is in no
  drawer.* Each sale stores its session (`sales.session_id`, 00038); the drawer and the Z count by it.
  *ledgercheck, example.*
- **DRAW-7** A closed session is final: no sale, refund or move booked after its close changes it, and nothing that
  happened before the close is missed by it — a sale paid while the till is being closed lands either in it (counted
  in its expected) or after it. (A sale takes its till shared-locked; the close locks it first or waits.) *Checked by: ledgercheck (DRAW-1 recomputed), faults.*
- **DRAW-8** Two closes at once (the cashier and an admin's Close for them) close it once and bank the cash once; Close
  for them books the money as the cashier's own close and records who did it. *faults, example.*
- **DRAW-9** Opening with a float from a locker takes it from that locker; with none, the float is money already in
  the drawer (only the receipt is written). A banked close puts the counted cash into the locker; an unbanked close
  writes only the receipt. The session's opening / closing records equal its opening / closing cash. *ledgercheck.*
- **DRAW-10** A withdrawal or pay-in with a locker moves the same amount the other way in that locker, in the same
  transaction; without one, only the drawer changes. *ledgercheck.*
- **DRAW-11** After a close that banked the cash the drawer starts empty: "continue with the last count" is not
  offered. An empty drawer opens and closes at zero. *Found in: d41bccc, 7dd16d2, e320d05. example.*
- **DRAW-12** With "untracked cash" off, cash leaving a till (withdraw, close with cash in it) must name a locker.
  *example.*
- **DRAW-13** A till holds only cash: its float, deposits, withdrawals and close use a till or a cash locker, never a
  bank, card-machine account or Cheques in hand; the till's default locker is a cash locker. *Found in: fcf6aa2,
  f9341c1, 6382df9. example.*
- **DRAW-14** Times: a till never opens or closes in the future (5 minutes' slack), never opens before its last close,
  never closes before it opened. *example.*
- **DRAW-15** A session left open past its day is marked stale; the cashier is asked to close it before selling on;
  an admin or manager may close it for them (DRAW-8). *example.*
- **DRAW-16** Two cash-outs at once from one drawer (two windows, a refund and a withdrawal) can't together take more
  than it holds (DRAW-3). *faults.*
- **DRAW-17** A counted drawer (notes × count) adds up to the amount recorded. *example.*

### Lockers and moves

- **LOCK-1** A locker's balance is the sum of its rows. A move the shop makes never takes a locker below zero unless
  it allows it; money that already moved outside the suite (a payment recorded after the bank's app made it) is
  written as it happened. Two moves at once out of one locker can't together overdraw it. *machine, faults* (not
  ledgercheck: a cleared cheque may rightly take a bank below zero).
- **LOCK-2** A transfer between lockers is two rows, equal and opposite, each naming the other. *ledgercheck.*
- **LOCK-3** A till ↔ locker row names the session and matches the drawer movement of the same moment, opposite in
  sign. *ledgercheck.*
- **LOCK-4** Every move writes exactly one CR- receipt for its amount, in the same transaction as its rows.
  *machine.*
- **LOCK-5** An archived locker takes and gives nothing, on every path; archiving the till's default locker clears
  that default. *Found in: 114ffde. example.*
- **LOCK-6** A card-machine account fills only from card takings and empties only through Settle card: never a
  transfer, an expense, a withdrawal or a void into it. *Found in: 12d146c, c532c90. ledgercheck, example.*
- **LOCK-7** Settle card takes the gross from the card account: the fee leaves as a bank charge booked as expense
  "Card settlement fee", the rest goes to the bank; the fee is below the gross. *ledgercheck, example.*
- **LOCK-8** A bank charge is booked as expense "Bank charges" of the same amount, in the same transaction; interest
  is money in; a count correction up or down needs a note. *ledgercheck, example.*
- **LOCK-9** Cheques in hand is never changed by hand (no transfer, adjust or expense) — only cheques move it (area
  5). *Found in: 14f27b9. example.*
- **LOCK-10** An opening balance is its own row ("open_balance"), negative only for a locker that allows it, and is not
  money in. *example.*
- **LOCK-11** Each payment method has its places: cash = a till or a cash locker; card = a card-machine account;
  online = a bank account. A forged or stale pick is refused. *Found in: 86dacd6, f9341c1. example.*
- **LOCK-12** A card / online tender at the till puts its amount into the account picked, linked to the sale, in the
  sale's transaction. *ledgercheck.*

### Expenses

- **EXP-1** An expense paid now moves its amount out of the place in the same transaction as the expense row; one
  paid by cheque moves nothing until the cheque clears (area 5); one paid from outside moves nothing. What left for an
  expense = its amount − what is still unpaid. *ledgercheck.*
- **EXP-2** Voiding an expense puts back exactly what left for it (amount − unpaid), into a place the user picks (a
  till or a non-card locker, not archived); an unpaid one voids with no money; a pending cheque stops the void; an
  expense a plugin owns can't be voided here. *ledgercheck (a voided expense's money came back in full), example.*
- **EXP-3** A paid expense's amount can't change (void it and record it again); category, note, date and service
  can. *example.*
- **EXP-4** Pay again pays only what is unpaid, once. *example.*

### Cash flow page

- **CF-1** Over any range: net = till cash sales + money in + opening balances − money out; a move between the
  shop's own places is neither in nor out and counts once in "moved". *Checked by: machine.*
- **CF-2** With every close banked and nothing untracked, the net over a range = the change in what the shop holds
  (every locker + every open drawer's expected). Cash taken out into nowhere (an unbanked close or withdrawal, when
  "untracked cash" is on) shows as **money out** ("taken out, not tracked"). *User, 2026-11-18 (Q3). Today it leaves
  no row: cash on hand falls but the page shows no money out.* *machine.*
- **CF-3** Cash on hand never counts Cheques in hand. *Found in: 14f27b9. example.*
- **CF-4** A till's cash sales are counted once: as till sales when rung; the close that banks them is a move, not
  money in. *machine.*
- **CF-5** A cheque counts in "after cheques", never in cash on hand, until it clears (area 5).

## Where each `example` rule is tested

`internal/battle/cases_money_test.go`: DRAW-5, DRAW-8, DRAW-12, DRAW-13 (+ LOCK-11 at the till), DRAW-17, LOCK-9,
LOCK-10. Older tests that already pin a rule: DRAW-11 `api/till_drawer_test.go` (TestTillDrawerLifecycle,
TestTillNoContinueAfterBankedClose); DRAW-12 `cashregister/withdraw_policy_test.go`; DRAW-14 `cashregister/at_db_test.go`;
DRAW-15 `cashregister/stale_test.go`; LOCK-1 overdraw, LOCK-5, LOCK-7, LOCK-8 `api/admin_locker_moves_test.go`; LOCK-5
default cleared `api/admin_lockers_test.go`; LOCK-6 `api/money_places_test.go`; LOCK-11 (suppliers)
`api/money_places_test.go`; EXP-1, EXP-2, EXP-3 `api/admin_expenses_test.go`; EXP-4 `api/expense_bounce_test.go`;
CF-3 `api/admin_cashflow_test.go` (TestAdminCashflowChequesNotCash).
