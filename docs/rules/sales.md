# Sales, discounts and payment (area 1)

Code: `internal/features/sales` (`service.go` Create, `service_lines.go` pricing, `tender.go`, `packs.go`),
`internal/api/till_sales*.go`, the till's maths `web/src/features/till/cart/*.ts`.

## How a sale is valued today (the words the rules use)

- **Line gross** = qty × unit price, rounded to cents; a pack price replaces it for full packs (`PackGross`).
- **Item discount** — fixed = Rs per unit × qty; percent = % of line gross; clamped to 0 … line gross.
- **Line net** (`sale_items.subtotal`) = line gross − item discount. A payout line is negative, never discounted.
- **Line tax** = (line net − its share of the bill discount) × rate, per line, to the cent (only when the shop is
  tax-registered with a number).
- **Bill discount** — fixed, or % of the goods' net (Σ line net of goods, payouts left out); clamped to that net.
- **Total** = Σ gross − item discounts − bill discount + tax (with payouts, may be below zero).
- **Paid share of a line** = (total − payouts) shared over the goods' lines by their net, to the cent
  (`sales.PaidShares`; the lines add up to the sale exactly).

## The grid

Columns: **Sell** (total, payment, change) · **Ret** (return part / rest / whole — rules in [returns.md](returns.md))
· **Drawer** (expected cash, X / Z) · **Cust** (customer balance, statement) · **Stock** (on hand, lots, movements) ·
**Rep** (sales list, P&L, trend, tax, line reports) · **Slip** (receipt, reprint) · **Replay** (offline / double send).

| Feature | Sell | Ret | Drawer | Cust | Stock | Rep | Slip | Replay |
|---|---|---|---|---|---|---|---|---|
| Plain line | SALE-1,2 | RET-1,2,3 | DRAW-1 | — a | SALE-20 | SALE-30 | SALE-40 | SALE-50 |
| Item discount fixed / % | DISC-1,2 | RET-4 | DRAW-1 | — a | SALE-20 | SALE-31 | SALE-41 | SALE-50 |
| Bill discount fixed / % | DISC-3,4,5 | RET-4,5 | DRAW-1 | — a | SALE-20 | SALE-31 | SALE-41 | SALE-50 |
| Item + bill discount together | DISC-6 | RET-4,5 | DRAW-1 | — a | SALE-20 | SALE-31 | SALE-41 | SALE-50 |
| Pack price (Card ×12) | SALE-3 | RET-6 | DRAW-1 | — a | SALE-21 | SALE-30 | SALE-42 | SALE-50 |
| Loose / fractional qty | SALE-4 | RET-7 | DRAW-1 | — a | SALE-21 | SALE-30 | SALE-40 | SALE-50 |
| Picked lot price | SALE-5 | RET-8 | DRAW-1 | — a | SALE-22 | SALE-30 | SALE-40 | SALE-50 |
| Oversell (found at till) | SALE-6 | RET-1 | DRAW-1 | — a | SALE-23 | SALE-30 | SALE-40 | SALE-50 |
| Frozen product | SALE-7 | RET-9 | DRAW-1 | — a | SALE-24 | SALE-32 | SALE-40 | SALE-50 |
| Serial numbers | SALE-8 | RET-10 | DRAW-1 | — a | SALE-20 | SALE-30 | SALE-40 | SALE-50 |
| Service line (+ components) | SALE-9 | RET-11 | DRAW-1 | — a | SALE-25 | SALE-33 | SALE-40 | SALE-50 |
| Payout (money to customer) | SALE-10 | RET-12 | DRAW-2 | — a | — b | SALE-34 | SALE-43 | SALE-50 |
| Line group (sets) | SALE-11 | RET-13 | DRAW-1 | — a | SALE-20 | SALE-30 | SALE-44 | SALE-50 |
| Tax-registered shop | SALE-12 | RET-14 | DRAW-1 | — a | — b | SALE-35 | SALE-45 | SALE-50 |
| Cash, change, kept change | TEND-1,2,3 | RET-15 | DRAW-1,3 | TEND-4 | — b | SALE-36 | SALE-46 | SALE-50 |
| Card / online | TEND-5,6 | RET-16 | DRAW-4 | — a | — b | SALE-36 | SALE-46 | SALE-50 |
| Cheque | TEND-7 | RET-17 | DRAW-4 | TEND-8 | — b | SALE-36 | SALE-46 | SALE-50 |
| Split tender | TEND-9 | RET-15 | DRAW-1 | TEND-10 | — b | SALE-36 | SALE-46 | SALE-50 |
| On account (credit) | TEND-10,11 | RET-18,19 | DRAW-4 | TEND-12 | — b | SALE-37 | SALE-47 | SALE-50 |
| Over the credit limit | TEND-13 | RET-18 | DRAW-4 | TEND-12 | — b | SALE-37 | SALE-47 | SALE-51 |
| Plugin tender (seam) | TEND-14 | RET-15 | DRAW-4 | — a | — b | SALE-36 | SALE-46 | SALE-50 |
| Held sale / ticket | SALE-13 | — c | — c | — c | — c | — c | — c | SALE-52 |
| Review (offline replay) | SALE-14 | RET-1 | DRAW-1 | TEND-12 | SALE-20 | SALE-38 | SALE-40 | SALE-50,51 |
| Sales person (prepared by) | SALE-15 | RET-1 | DRAW-1 | — a | SALE-20 | SALE-39 | SALE-40 | SALE-50 |

`— a`: only an on-account / cheque / kept-change tender touches a customer's balance (rows TEND). `— b`: payment
methods and tax don't move stock. `— c`: a held sale or ticket is not a sale until it's paid; nothing is booked.

## Rules

### Selling and totals

- **SALE-1** A sale's total = Σ line gross − Σ item discounts − bill discount + tax, to the cent; stored total,
  stored subtotal / discount / tax and the lines agree. *Checked by: ledgercheck, lean.*
- **SALE-2** Prices come from the server's catalogue (or the picked lot), never from the request — except a service's
  price override and a Review sale's expect price (SALE-14). *Checked by: example.*
- **SALE-3** Full packs ring at the pack price, the rest at the unit price; a pack never costs more than its units
  bought loose; wholesale follows its own pack rule. *Checked by: lean (PackGross), example.*
- **SALE-4** Whole-only units refuse fractions; loose quantities round the line gross to the cent once.
- **SALE-5** A picked lot sets the line's price and is the lot depleted (a frozen product: price only).
- **SALE-6** Overselling never leaves stock below zero: the count is corrected up first (FoundAtTill), then sold.
- **SALE-7** A frozen product sells without touching stock or asking.
- **SALE-8** Serial lines carry exactly qty unique serials, each recorded as a warranty unit of this sale.
- **SALE-9** A service line holds no stock; its components are depleted and its cost is their FEFO cost.
- **SALE-10** Only payouts take a total below zero; the money goes out as cash change, with no other tender.
- **SALE-11** A line group is kept together; adding the same group again adds sets.
- **SALE-12** Tax is charged only by a shop registered for it with a number; otherwise product rates are ignored.
- **SALE-13** A held sale or ticket books no money, stock or balance until it is paid.
- **SALE-14** A Review sale (offline replay) never stops: a different catalogue price is overruled by the price
  shown at the till and flagged; over-limit credit is flagged; oversell is flagged.
- **SALE-15** Prepared-by names only an existing person, or nobody; it never costs the sale.

### Discounts

- **DISC-1** An item discount is never below 0 or above the line gross; fixed is per unit × qty.
- **DISC-2** A percent item discount is % of the line gross (after a pack price).
- **DISC-3** A bill discount is never below 0 or above the goods' net (after item discounts); payouts are never
  discounted.
- **DISC-4** A percent bill discount is % of the goods' net after item discounts, rounded to the cent once.
- **DISC-5** A bill discount is shared over the goods' lines so the **shares add up to the bill discount exactly**,
  each share ≤ its line. *Today each line's share rounds alone — see RET-5.* *Checked by: lean, ledgercheck.*
- **DISC-6** Item and bill discounts together never take a line's paid value below 0.

### Payment and change

- **TEND-1** Paid + on account ≥ total, or the sale is refused (never a silent credit).
- **TEND-2** Only cash gets change: change ≤ cash handed over (+ a payout's amount).
- **TEND-3** Change given ≤ change due; what the customer leaves stays in the drawer (rounding gain) or goes to their
  account as an advance — never both, never lost.
- **TEND-4** Kept change to an account lowers that customer's balance by exactly the kept amount.
- **TEND-5** A card / online payment names the account its money lands in; that account rises by exactly that amount.
- **TEND-6** Card / online / cheque never exceed what's due.
- **TEND-7** A cheque needs its bank, number and date, and a customer.
- **TEND-8** A cheque is money in hand until it clears; a bounce puts it back on the customer's account (area 5).
- **TEND-9** Each payment row is stored once; zero rows are not stored (except the one cash row of a payout sale).
- **TEND-10** A part-account sale lands exactly on the total: no change against money not paid.
- **TEND-11** The on-account part raises the customer's balance by exactly that part; the sale's status is credit.
- **TEND-12** The balance printed on the receipt = the customer's balance right after this sale.
- **TEND-13** Over the limit is refused, unless allowed by the cashier's confirm or a Review sale (then flagged).
- **TEND-14** A payment method no one settles is refused before anything is written; a plugin tender settles inside
  the sale's transaction.

### Everywhere else

- **SALE-20** Each stocked line writes one sale movement of −qty; on hand = Σ movements. *ledgercheck.*
- **SALE-21** Packs and loose quantities move stock in base units.
- **SALE-22** A picked lot is the lot that goes down; otherwise FEFO.
- **SALE-23** After an oversell, on hand is 0, never negative.
- **SALE-24** A frozen product's stock doesn't move on sale.
- **SALE-25** A service's components move; the service itself doesn't.
- **SALE-30** The sales list, P&L and trend count each sale once, at its total less payouts, on the shop's day.
- **SALE-31** Line-level reports share the bill discount onto lines the same way refunds do (DISC-5).
- **SALE-32** A frozen line's cost still counts in profit.
- **SALE-33** A service line's profit is its price − its components' cost.
- **SALE-34** A payout is not revenue.
- **SALE-35** Tax is charged on what the customer actually pays for the goods: **after item and bill discounts**
  (a discount given at the sale lowers the taxable value — the usual VAT rule). The tax report and the receipt show
  that tax. *User left it to us (Q3, 2026-11-18); done: the bill discount is shared over the lines to the cent and each
  line's tax is on what's left.* Only tax-registered shops are touched.
- **SALE-36** By-method totals: each method's takings once; cash takings less change.
- **SALE-37** Credit sales appear under receipts / dues, not as cash.
- **SALE-38** A Review sale's flags are visible in the sale and the review list.
- **SALE-39** Prepared-by shows on the sale, never changes its money.
- **SALE-40** The receipt shows exactly the stored sale: lines, discounts, total, payments, change.
- **SALE-41** Discounts on the receipt add up to the sale's discount.
- **SALE-42** A pack line shows its pack label.
- **SALE-43** A payout prints as money given, not as a negative item.
- **SALE-44** A group prints as one block with its sets.
- **SALE-45** Tax prints only for a registered shop.
- **SALE-46** Payments and change on the receipt = stored payments and change.
- **SALE-47** A credit receipt shows the due, any advance used, and the balance after (TEND-12).
- **SALE-50** The same `client_ref` sent twice (double Pay, a replay, two at once) is one sale; the second gets the
  first back and books nothing.
- **SALE-51** A Review sale is never refused for price, oversell or credit limit; it is flagged.
- **SALE-52** A held sale / ticket is paid at most once.

### The till's screen vs the server

- **TILL-1** The total, discounts, change and refund the till shows = what the server stores, to the cent, for every
  cart (the till's maths is TypeScript numbers, the server's is decimal). *Checked by: lean cases, fast-check.*
- **TILL-2** The return screen's refund for n units = the refund the drawer pays (RET-3).
