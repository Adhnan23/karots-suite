# Purchasing: orders, deliveries, supplier returns, supplier pay, credits, cheques (area 5)

Code: `internal/features/purchases` (orders, deliveries, bills, lot cost), `internal/features/purchasereturns` (goods
sent back, debit notes), `internal/features/supplierpay` (payments, the waterfall, advances, discounts, refunds,
settling a new bill out of credit), `internal/features/suppliers` (balance, opening, statement),
`internal/features/cheques` (given and received cheques: clear, bounce, cancel, fix), `internal/features/payout`
(paying by place or cheque), `internal/api/admin_purchases.go`, `counter_receive.go`, `admin_purchase_returns.go`,
`admin_supplier_money.go`, `counter_suppliers.go`, `admin_suppliers.go`, `admin_cheques.go`. Losses a supplier
covers (credits, replacement charges) are area 6; only their effect on the balance is checked here.

## The words the rules use

- **Order** — a purchase in `draft`: what the shop asked for. Nothing moves until it is received.
- **Bill** — a received purchase (`received`, `partial`, `paid`): its **total** is what the supplier is owed for it;
  **paid** (`paid_amount`) is money and cheques put against it; **credited** (`credited_amount`) is what came off it
  with no money moving (goods sent back, a discount, a credit for a loss). **Left** = total − paid − credited.
- **Delivery** — goods arriving: an order received, a delivery with no order (Receive now, the counter's walk-in), or a
  **bill with no lines** (something bought that isn't stock: a plugin's float top-up).
- **Balance** (`suppliers.outstanding_balance`) — what the shop owes the supplier. Below zero the supplier holds a
  **credit** for the shop (an **advance** paid before the goods, or goods sent back when nothing was owed).
- **Old debt** (`opening_unlinked`) — the still-unpaid part of the **opening** (`opening_balance`): debt from before
  the suite.
- **Payment** (`supplier_payments`) — money (or a discount) given to a supplier, put against bills by
  **allocations** (`supplier_payment_allocations`; `reopened` = what a bounced cheque took back off one).
- **Debit note** (`purchase_returns`) — goods sent back; its **total** comes off the balance and off bills
  (`purchase_return_allocations`).
- **Refund** (`supplier_refunds`) — money the supplier hands back out of a credit they hold.
- **Cheque** (`cheques`) — **given** (`out`: to a supplier or for an expense, drawn on a bank place) or **received**
  (`in`: from a customer, held in Cheques in hand). **Pending** until it is **cleared**, **bounced** or **cancelled**.

## The grid

Columns: **Do** (the bill and the balance) · **Stock** (lots and movements, area 4) · **Money** (where money goes,
area 2) · **Bills** (each bill's paid / credited / status) · **Undo** (put right) · **Statement** (supplier
statement, Owing, payables) · **Faults** (two at once, power cut, sent twice).

| Feature | Do | Stock | Money | Bills | Undo | Statement | Faults |
|---|---|---|---|---|---|---|---|
| Supplier made with an opening (by hand, sheet) | SUP-2 | — a | — a | — b | SUP-2 | SUP-10 | — e |
| Opening adjusted | SUP-2 | — a | — a | — b | SUP-2 | SUP-10 | SUP-8 |
| Order saved / edited / deleted / from reorder | PUR-5 | PUR-5 | — a | PUR-5 | PUR-5 | PUR-5 | — e |
| Delivery with no order (Receive now, walk-in) | PUR-1,2 | PUR-2,4 | — a | PUR-11 | PUR-9 | SUP-10 | PUR-2 |
| Order received (keep the rest on order) | PUR-6 | PUR-2,4 | — a | PUR-11 | PUR-9 | SUP-10 | PUR-6 |
| Free units, line discount, bill discount, packs | PUR-1 | PUR-3,4 | — a | PUR-11 | PUR-9 | SUP-10 | — e |
| Bill with no lines (a plugin's seam) | PUR-1 | — a | — a | PUR-11 | — d | SUP-10 | — e |
| New bill settled out of the supplier's credit | PUR-8 | — a | — a | PUR-8,12 | — d | SUP-10 | PUR-8 |
| Paid on delivery | PUR-7 | — a | SUP-3 | PUR-7 | CHQ-3 | SUP-10 | PUR-2 |
| Payment: bills oldest first / old debt first / typed per bill | SUP-4 | — a | SUP-3 | SUP-4, PUR-12 | CHQ-3 | SUP-10 | SUP-8 |
| Paid beyond what's owed (advance) | SUP-5 | — a | SUP-3 | SUP-5 | SUP-7 | SUP-10 | SUP-8 |
| Discount received | SUP-6 | — a | — a | SUP-6 | — d | SUP-10 | SUP-8 |
| Payment or expense by cheque (given) | CHQ-1 | — a | CHQ-1 | PUR-12 | CHQ-3 | SUP-10 | CHQ-6 |
| Given cheque cleared | CHQ-2 | — a | CHQ-2 | — c | — d | SUP-10 | CHQ-6 |
| Given cheque bounced / cancelled | CHQ-3 | — a | — a | CHQ-3,4 | — d | SUP-10 | CHQ-6 |
| Received cheque cleared / bounced / cancelled | CHQ-2,3 | — a | CHQ-2,3 | — c | — d | — c | CHQ-6 |
| Cheque details fixed | CHQ-5 | — a | — a | — c | CHQ-5 | — c | CHQ-6 |
| Goods sent back, against a delivery | PUR-9 | PUR-9 | — a | PUR-9 | — d | SUP-10 | PUR-9 |
| Goods sent back, stock from no delivery | PUR-9 | PUR-9 | — a | PUR-9 | — d | SUP-10 | PUR-9 |
| Sent back beyond what's owed (supplier credit) | PUR-9 | PUR-9 | — a | PUR-9 | SUP-7 | SUP-10 | PUR-9 |
| Refund received from the supplier | SUP-7 | — a | SUP-7 | — c | — d | SUP-10 | SUP-7 |
| Supplier disabled / brought back | SUP-9 | — a | — a | — c | SUP-9 | SUP-10 | — e |
| Credit for a loss, replacement charge | SUP-1 | area 6 | area 6 | PUR-8 | area 6 | SUP-10 | area 6 |

- `— a` Nothing of this kind moves.
- `— b` No bill: the opening is old debt, outside any bill.
- `— c` Doesn't touch it.
- `— d` No undo: put right by the opposite action (a payment for a bill, a refund for a credit, goods sent back for a
  delivery), so both stay on the statement.
- `— e` One action in one transaction on rows nothing else writes at the same moment.

## Rules

### Bills and deliveries

- **PUR-1** A bill's total = Σ its lines' net − the bill discount; a line's net = paid units × cost − the line's
  discount (fixed is per unit, percent off the line, never below 0 or above the line); the bill discount is fixed or a
  percent of the lines, never below 0 or above them. Free units add nothing to what is owed. A bill with no lines is
  just its amount. *ledgercheck, example.*
- **PUR-2** A delivery is one transaction: each line opens one lot (source purchase, tied to its line) of paid + free
  units and writes one purchase movement of the same quantity; the supplier is owed the total; the product's cost and
  price change only to a figure above zero (a free line never zeroes them); its usual supplier is filled in only when
  it has none. A power cut leaves all of it or none. *Found in: 6ee0595 (deliveries beside sales of the same
  products deadlocked, and the database refused one; never stop a sale).* *ledgercheck, example, fault.*
- **PUR-3** A lot's cost is what was really paid for its line — its net less its share of the bill discount (shared
  in proportion to the lines' nets) — over every unit that arrived, kept to 4 decimals; a line nothing reduced keeps
  its list cost exactly. So a delivery's lots are worth its total to within a cent a line, and sending the whole
  delivery back clears its bill exactly. A line that cost nothing takes the product's cost (as STK-7's blank-cost lot).
  *Found in: a039cfb, 20c8b97.* *example.*
- **PUR-4** Bought by the pack, a line is entered per pack and kept in units (packs × size, cost ÷ size); the pack size
  is a whole number above 1 (STK-17). A product counted in whole units gets whole units (STK-18): a delivery, an order
  or goods sent back of 2.5 pieces is refused. *Fixed in 4cd45e0.* *example.*
- **PUR-5** An order moves nothing — no stock, no balance, no money, not on the statement or the payables. Only an
  order can be edited or deleted; a received bill never is (it is put right by goods sent back or a credit). *example.*
- **PUR-6** An order is received once: its lines become what really came (a line that didn't come is left out);
  "keep the rest on order" puts every ordered item still short — read from the order's own lines, by product — on a
  new order. Two receives of the same order at the same moment book it once; the other is refused ("already
  received"). *Found in: debc28c, 5b5da99 (eight receives at once booked it eight times; an edit beside a receive turned the bill back into an order).* *example, fault.*
- **PUR-7** Paying on delivery pays only that bill, after its supplier's credit has settled it (PUR-8): never more
  than is left on it, and only what was applied leaves the till or bank. *example.*
- **PUR-8** A new bill is first settled out of what the supplier already holds for the shop: advances (as paid, oldest
  first), then goods-sent-back credits, then credits for losses (as credited) — never more than the credit really on
  hand (the balance before the bill), so an advance already refunded isn't spent again. *example, machine.*
- **PUR-9** Goods sent back name the lot going back on every line. Against a delivery, the lot came on it; with no
  delivery, the lot came on none. Each line is priced at its lot's cost (a blank cost → the product's cost), never a
  typed price. Stock and the lot go down (refused beyond what the lot holds, STK-3, STK-6) with a purchase-return
  movement; the balance drops by the total; the delivery's own bill is credited first, then other bills with something
  left, oldest first; what's beyond every bill stays as a supplier credit. A frozen product's pieces out for sale are
  first put back on the books (to_shelf), then sent back, as a frozen write-off (STK-8). *User, 2026-11-19 (Q4).
  Fixed in d99efa5.* *example, machine.*
- **PUR-10** A debit note's total = Σ its lines (units × cost, to the cent); what it credited to bills never passes
  it. *ledgercheck.*
- **PUR-11** A bill's paid and credited are never below zero and never add up to more than its total; it reads
  **paid** exactly when nothing is left, **received** when nothing was paid, otherwise **partial**. *ledgercheck.*
- **PUR-12** A bill's paid = Σ the money put on it (allocations of payments that aren't discounts) less what bounced
  or cancelled cheques took back off it. *ledgercheck.*
- **PUR-13** A bill's lines landed: its purchase movements add up to Σ its lines' paid + free units, and every line
  has its lot. *ledgercheck.*

### Supplier balance and money

- **SUP-1** A supplier's balance = their opening + bills + replacement charges + refunds received + their bounced /
  cancelled given cheques − debit notes − payments (discounts too) − credits for losses, to the cent, at every moment.
  *ledgercheck, machine.*
- **SUP-2** An opening is booked once, when the supplier is made (by hand or by a sheet; a sheet row for a supplier
  that exists leaves their opening alone) and is never below zero. Adjusting it moves the balance and the opening by
  the same amount and never touches what bills, payments and returns made. The old debt never passes the opening
  nor what they're owed: old debt set while the supplier holds a credit is paid down by that credit, as CUST-15.
  *User, 2026-11-19 (Q2). Fixed in f36d840.*
  *ledgercheck, example.*
- **SUP-3** A payment's money lands where it says: cash from a till or a cash locker, card / online from a bank
  account (LOCK-11, `86dacd6`), a cheque as given cheques that add up to it exactly (CHQ-1); paid from outside moves
  nothing. Its money receipt is its amount. *ledgercheck, example.*
- **SUP-4** A payment goes on bills oldest first (or the old debt first, when chosen); typed per bill, each amount
  goes where typed. It never pays a bill beyond what's left on it ("exceeds the balance on invoice", nothing saved),
  never pays another supplier's bill, and its old-debt part never passes the old debt. Its allocations never add up to
  more than it. *ledgercheck, example.*
- **SUP-5** What's paid beyond the bills and the old debt is an advance: the balance goes below zero and the next
  bills are settled from it (PUR-8). An advance typed while something is owed pays that first — open bills oldest
  first, then the old debt — and only what's beyond all of it stays as credit, so the payment queue never shows a bill
  owed while the shop holds credit with the same supplier. *User, 2026-11-19 (Q1). Fixed in aee7488.* *example, machine.*
- **SUP-6** A discount received lowers bills (credited, not paid) and the balance; no money moves, and it is never
  an advance — a discount beyond what's owed is refused. *example.*
- **SUP-7** A refund received is at most the credit the supplier holds; its money lands in a till or a cash locker
  (cash) or a bank account (card / online, LOCK-11); the balance walks back toward zero and never past it. Two refunds
  at the same moment never both pass the limit. *Found in: d7d2e0b (eight refunds of a Rs 300 credit all went through).* *example,
  fault.*
- **SUP-8** Two money actions on one supplier at the same moment — payments, old-debt payments, refunds, an opening
  adjusted, a delivery settled from credit — each land whole: the balance moves by exactly what each says, and no bill,
  old debt or credit is spent twice. *Found in: d7d2e0b (eight Rs 100 old-debt payments of Rs 300 took Rs 800 from the safe),
  6ee0595 (a payment's split read before it held the supplier was refused beside another).* *fault.*
- **SUP-9** A disabled supplier keeps their balance, bills and history: still on Owing and the payables, and can still
  be paid or refunded. A delivery, an order or goods sent back for one is refused ("bring them back first").
  *User, 2026-11-19 (Q3). Fixed in 3d401c6.* *example.*
- **SUP-10** The statement's running balance ends at the supplier's balance (SUP-1's rows, oldest first); Owing lists
  every supplier owed (disabled too), with the oldest bill that still has something left. *Report sums are area 7.*
  *example.*

### Cheques

- **CHQ-1** A given cheque (a supplier payment, paying on delivery, an expense) moves no money until it clears; the
  bill, balance or expense counts it as paid from the moment it's written; the bank place shows it in "after cheques"
  (CF-5). It is drawn on an active bank place. *example.*
- **CHQ-2** A cheque clears once, not before its date and not in the future: a given one takes its money out of its
  bank place then (it may take a bank below zero), a received one moves from Cheques in hand into the bank place it's
  paid into. *example.*
- **CHQ-3** A pending cheque can be bounced or cancelled, once, and is never refused for it. A received one leaves
  Cheques in hand and the customer owes it again (TEND-8). A given one to a supplier: they are owed it again and the
  bills it paid reopen, newest allocation first, never below nothing paid (a bill credits still cover stays paid). A
  given one for an expense makes the expense unpaid again (EXP-4). *Found in: 5f951cb.* *example.*
- **CHQ-4** A bounced or cancelled cheque's payment never pays a bill again: what it didn't put on bills is no advance.
  *Found in: ea3af73.* *example.*
- **CHQ-5** Only a pending cheque's details can be fixed (bank, number, date, note; a given one's bank place); its
  amount never changes. *example.*
- **CHQ-6** Two status changes of one cheque at the same moment: one wins, the other is refused ("already …").
  *fault.*
