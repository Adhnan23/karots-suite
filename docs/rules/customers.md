# Customers: credit, advances, opening balances, payments (area 3)

Code: `internal/features/customers` (balance, opening, payments, advance refunds, statement), the on-account and
kept-change parts of `internal/features/sales/service.go`, the return's debt clearing in `service_returns.go`
(area 1), a cheque's bounce in `internal/features/cheques/effects.go`, `internal/api/admin_customers.go`,
`counter_credit.go`, `till_customers.go`, `cheques_parse.go`.

## The words the rules use

- **Balance** (`customers.outstanding_balance`) — what the customer owes now. Above zero they owe; below zero the
  shop holds an **advance** for them.
- **Old debt** (`opening_unlinked`) — the part of the balance that is still-unpaid debt from before the suite
  (the **opening**, `opening_balance`, as entered or adjusted). The rest is the **linked** part: what sales made.
- **On account** — the part of a sale put on the customer's balance (`total − paid_amount` of a sale that has it).
- **Kept change** (`sales.rounding_to_account`) — change the customer left, booked as an advance.
- **Payment** (`customer_payments`) — money the customer brings (a DP- slip); an **advance refund** is a payment row
  with a negative amount and no DP- number (its proof is the CR- money receipt).

## The grid

Columns: **Do** (what it does to the balance) · **Money** (where money lands, area 2) · **Slip** (DP- / CR- / the sale
receipt) · **Undo** (put right) · **Limit** (credit limit, available) · **Statement** (the customer's ledger) ·
**Dues** (dues list, receivables, advances) · **Faults** (twice, two at once, power cut).

| Feature | Do | Money | Slip | Undo | Limit | Statement | Dues | Faults |
|---|---|---|---|---|---|---|---|---|
| Customer made with an opening | CUST-2 | — a | — b | CUST-4 | CUST-8 | CUST-10 | CUST-12 | CUST-3 |
| Customer imported (sheet) | CUST-2 | — a | — b | CUST-4 | CUST-8 | CUST-10 | CUST-12 | CUST-3 |
| Opening adjusted | CUST-4 | — a | — b | CUST-4 | CUST-8 | CUST-10 | CUST-12 | CUST-15 |
| Sale on account | TEND-11 | DRAW-4 | TEND-12 | RET-18,19 | TEND-13, CUST-8 | CUST-10 | CUST-12 | CUST-8, SALE-50 |
| Sale on account using an advance | CUST-7 | DRAW-4 | SALE-47 | RET-18,19 | CUST-8 | CUST-10 | CUST-12 | CUST-8 |
| Change kept as an advance | TEND-4 | DRAW-1 | — c | — d | CUST-8 | CUST-10 | CUST-12 | SALE-50 |
| Return on a credit sale | RET-18,19 | DRAW-1 | RET-21 | — d | CUST-8 | CUST-10 | CUST-12 | — e |
| Payment, cash / card / online | CUST-5 | LOCK-11 | CUST-6 | — d | CUST-8 | CUST-10 | CUST-12 | CUST-13, CUST-16 |
| Payment, old debt first | CUST-9 | LOCK-11 | CUST-6 | — d | CUST-8 | CUST-10 | CUST-12 | CUST-13 |
| Payment by cheque | CUST-5 | TEND-8 | CUST-6 | TEND-8 | CUST-8 | CUST-10 | CUST-12 | CUST-13 |
| Payment by a plugin tender (seam) | CUST-5 | TEND-14 | CUST-6 | — d | CUST-8 | CUST-10 | CUST-12 | CUST-13 |
| Paid more than owed | CUST-7 | LOCK-11 | CUST-6 | CUST-11 | CUST-8 | CUST-10 | CUST-12 | CUST-13 |
| Advance refunded | CUST-11 | DRAW-3 | LOCK-4 | — d | CUST-8 | CUST-10 | CUST-12 | CUST-11 |
| Cheque bounced / cancelled | TEND-8 | LOCK-1 | — c | — d | CUST-8 | CUST-10 | CUST-12 | — e |
| Credit limit changed | CUST-8 | — a | — b | CUST-8 | CUST-8 | — f | CUST-12 | — e |
| Phone changed | CUST-3 | — a | — b | — d | — f | — f | — f | CUST-3 |
| Customer disabled / brought back | CUST-14 | — a | — b | CUST-14 | CUST-17 | CUST-10 | CUST-12 | — e |

- `— a` No money moves.
- `— b` No slip (the admin change is in the audit log).
- `— c` No slip of its own: kept change is on the sale's receipt; a bounce is on the cheque's CR- money receipt.
- `— d` No undo: put right by the opposite action (a payment for a credit sale, an advance refund for an overpayment, a
  sale for a return), so both stay on the statement.
- `— e` One action in one transaction; its power cut and double send are covered by the area it belongs to.
- `— f` Doesn't touch it.

## Rules

### The balance

- **CUST-1** A customer's balance = their opening + every sale's on-account part − kept change − what returns
  cleared − payments + advances refunded + bounced / cancelled cheques of theirs, to the cent, at every moment.
  *Checked by: ledgercheck, machine.*
- **CUST-2** An opening is booked once, when the customer is made (by hand, at the till or by a sheet); a customer
  reused by phone, or a sheet imported again, never adds it again. An opening at making is never below zero.
  *example.*
- **CUST-3** One active customer per phone (spaces and dashes ignored): making one with a phone in use reuses that
  customer, and changing a phone to, or bringing back a customer with, one in use is refused. Otherwise one person's
  debt splits across two accounts. *Found in: 3ed448a (import doubled customers). ledgercheck, example.*
- **CUST-4** Adjusting the opening moves the balance and the opening by the same difference and leaves the linked part
  unchanged; it may go below zero (an advance from before the suite); it is in the audit log with before and after.
  *example.*

### Payments and advances

- **CUST-5** A payment lowers the balance by exactly its amount and lands its money in the same transaction: cash in a
  till or cash locker, card / online in a card or bank account (LOCK-11), a cheque in Cheques in hand (TEND-8), a
  plugin tender by its settle. No debt goes down with no money anywhere. *Found in: e003a72, f9341c1. ledgercheck,
  example.*
- **CUST-6** Each payment's slip has its own DP- number and shows the balance before and after: after = before −
  amount, and before = the balance at that moment, even when two payments land at once. *ledgercheck, example.*
- **CUST-7** Paying more than owed leaves the rest as an advance. The next sale on account uses the advance first
  (the balance simply adds up); its receipt shows how much came from the advance (SALE-47). *example, machine.*
- **CUST-8** Available credit = limit − balance (an advance adds to it). Two sales on account at once are each checked
  against the balance after the other, so together they never pass the limit unflagged (TEND-13). Changing the limit
  never changes the balance. *example, fault.*
- **CUST-9** "Old debt first" pays the old debt up to what is left of it, then the linked part, then leaves an advance;
  a plain payment pays the linked part first and the old debt only past it. The old debt never goes below zero unless
  it was set below zero (CUST-4), and never above the opening. *ledgercheck, example.*
- **CUST-11** An advance refund hands back at most the advance; two at once never together pass it. The cash comes
  from a till (refused beyond what the drawer holds, DRAW-3) or a cash locker, and gets a CR- receipt. *example,
  fault.*
- **CUST-15** The old debt is never more than what the customer owes: an advance they hold when an old debt is set
  pays it down at once (an advance of 500, old debt set to 1,000 → old debt 500, balance 500). *User, 2026-11-19
  (Q1). Today both stand side by side and the next payment quietly shrinks the old debt.* *ledgercheck, example.*
- **CUST-16** The same payment sent twice (double tap, Pay again after a lost answer) is one payment: it carries a
  one-time reference, and the second send gets the first back and books nothing. *User, 2026-11-19 (Q2). Today two
  are recorded.* *example, fault.*
- **CUST-17** A sale on account to a disabled customer goes through (never stop a sale) and is flagged for review,
  "customer disabled". *User, 2026-11-19 (Q3). Today it goes through unflagged.* *example.*
- **CUST-13** Two payments for one customer at the same moment both count: the balance drops by both (CUST-1), each
  slip right (CUST-6). *fault.*

### What the shop sees

- **CUST-10** The statement's running balance ends at the customer's balance: every part of CUST-1 is a line on it
  (kept change too). *example, machine.*
- **CUST-12** The dues list shows everyone who owes, disabled customers too; receivables = what is owed (balances
  above zero) and advances are counted apart (balances below zero). *Found in: e3c2674. example.*
- **CUST-14** Disabling a customer keeps their balance and history; they can still pay and be handed an advance back.
  Bringing them back changes nothing on their account. *example.*
