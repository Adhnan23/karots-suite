# Reports and the Z report = the raw data (area 7)

Code: `internal/features/reports` (P&L `Compute`, by period, peak hours, products, categories, services, cashiers,
returns, tax, expenses, supplier spend, losses), `internal/api/admin_reports_*.go` (the Reports pages and their sheet
downloads), `admin_dashboard.go` (the admin home), `counter_z.go` + `sales.PeriodSummary` (the Z report),
`products/valuation*.go` (stock value), `customers.Owing` / `suppliers.Owing` (dues). Nothing here writes: a report
is only ever a sum over rows areas 1–6 already hold to their rules. The Cash flow page is area 2 (CF); a single
sale's list row is area 1 (SALE-30).

## The words the rules use

- **Kept** (a sale line) — units sold less units returned. **Kept revenue** — what the kept units brought in before
  tax: the line's paid share (SALE: bill discount shared out) less tax, for the kept units.
- **Revenue** (the P&L's) — every non-void sale's total less tax, less pass-through face (airtime, bill face value,
  payouts — money passing through, SALE-34), less returns.
- **Cost** (COGS) — kept units × the cost stored on the sale line (frozen and service lines included, SALE-32,33).
- **The shop's day** — midnight to midnight in the shop's time zone (`datetime.Location`), never UTC.
- **Period** — a from–to range of shop days, the last day whole.
- **Snapshot** figures (dues, advances, supplier credit, stock value, till cash) are *now*, whatever the period.

## The grid

Rows are everything areas 1–6 put in the books; columns are where it shows. **P&L** (Reports → Profit & loss, the
dashboard's today / yesterday) · **Time** (by period, peak hours, the dashboard's charts and spark, one product's
graph) · **Lines** (top products, product totals, categories, services) · **People** (sales by cashier, the Z
report) · **Ret** (the Returns page) · **Tax** · **Pages** (dues, supplier spend, purchases, losses, stock value,
top-up) · **Agree** (two reports showing the same thing show the same number).

| Feature | P&L | Time | Lines | People | Ret | Tax | Pages | Agree |
|---|---|---|---|---|---|---|---|---|
| Plain sale, item / bill discount | RPT-1 | RPT-10 | RPT-12 | RPT-14, ZR-1 | — a | RPT-17 | — a | RPT-30 |
| Pack, loose qty, picked lot, oversell | RPT-1 | RPT-10 | RPT-12 | RPT-14, ZR-1 | — a | RPT-17 | — a | RPT-30 |
| Frozen product, service line | RPT-2 | RPT-10 | RPT-12,13 | RPT-14, ZR-1 | — a | RPT-17 | — a | RPT-30 |
| Pass-through line, payout | RPT-3 | RPT-10 | RPT-12 | RPT-14, ZR-1 | — a | RPT-17 | — a | RPT-30 |
| Tax-registered shop | RPT-1 | RPT-10 | RPT-12 | RPT-14 | RPT-16 | RPT-17 | — a | RPT-30 |
| Stocked line with no cost | RPT-4 | — b | RPT-12 | — b | — a | — a | RPT-24 | — b |
| Void sale | RPT-5 | RPT-5 | RPT-5 | RPT-5, ZR-1 | — a | RPT-5 | — a | RPT-30 |
| Return (part, rest, whole, discount shared) | RPT-6 | RPT-10 | RPT-12 | RPT-14, ZR-2 | RPT-16 | RPT-17 | — a | RPT-31 |
| Tenders: cash + change, card, cheque, split, plugin | RPT-7 | — b | — b | ZR-3 | — a | — a | — a | RPT-32 |
| On account, kept change, advance used | RPT-7,8 | — b | — b | ZR-3 | — a | — a | RPT-20 | RPT-32 |
| Customer payment, opening balance, advance | RPT-8 | — b | — b | — c | — a | — a | RPT-20 | — b |
| Expense (any money source, a service's) | RPT-9 | — b | RPT-13 | — c | — a | — a | RPT-25 | — b |
| Damage, give back, warranty replacement | RPT-9 | — b | — b | — c | — a | — a | RPT-22 | RPT-33 |
| Own use, staff, count (adjust) | RPT-9 | — b | — b | — c | — a | — a | RPT-24 | — b |
| Recovery (any outcome), its undo | RPT-9 | — b | — b | — c | — a | — a | RPT-22 | RPT-33 |
| Delivery, goods sent back, supplier credit | — d | — b | — b | — c | — a | — a | RPT-21 | RPT-34 |
| Supplier payment, discount received, cheque | RPT-8,9 | — b | — b | — c | — a | — a | RPT-20,21 | RPT-34 |
| Locker interest | RPT-9 | — b | — b | — c | — a | — a | — a | — b |
| Till over / short, withdrawal, pay-in | RPT-9 | — b | — b | ZR-3,4 | — a | — a | RPT-23 | — b |
| Stock on hand, lots, frozen stock | — d | — b | — b | — c | — a | — a | RPT-24 | — b |
| Period edges (shop's day, a week, a year) | RPT-11 | RPT-11 | RPT-11 | RPT-11 | RPT-11 | RPT-11 | RPT-11 | RPT-30 |
| A sheet download (csv / xlsx / ods) | RPT-35 | RPT-35 | RPT-35 | RPT-35 | RPT-35 | RPT-35 | RPT-35 | RPT-35 |

- `— a` That report doesn't show this kind of row.
- `— b` Not shown there by design (the P&L's lines and the Cash flow page hold it).
- `— c` The Z report is one till session's sales and drawer; this never passes through a till's sales (its drawer
  money is area 2's DRAW rules and the Z's moves list, ZR-4).
- `— d` Bought stock is not profit or loss until it is sold or lost; the P&L shows payables only as a snapshot (RPT-8).

## Rules

### Profit & loss

- **RPT-1** Revenue = Σ (total − tax) over the period's non-void sales, less pass-through face, less returns (RPT-6);
  cost = Σ kept units × the line's stored cost; gross profit = revenue − cost; margin = gross profit ÷ revenue (0 when
  revenue ≤ 0). To the cent of the raw rows. *machine (oracle).*
- **RPT-2** A frozen line's cost and a service's components' cost are in cost (SALE-32, SALE-33).
- **RPT-3** Pass-through lines and payouts are never revenue, cost or profit (SALE-34) — in the P&L and every report
  that shows revenue.
- **RPT-4** "Sold with no cost": the revenue of stocked (non-service, non-pass-through) **kept** lines whose stored
  cost is 0, and how many — so a margin that flatters itself says so. *Found by reading: pass-through lines and fully
  returned lines are counted today.*
- **RPT-5** A void sale is in no report, anywhere.
- **RPT-6** A return counts **on the day it was made** (never the sale's day), at the refund actually given before
  tax (`sale_return_items.refund_amount` less its tax share), and its cost (returned units × the line's stored cost)
  comes back off cost that day — in the P&L, by period, peak hours, the dashboard and every line report. A period
  already over never changes when something comes back later; a sale returned whole nets to 0 revenue and 0 cost over
  all time, to the cent. *User, 2026-11-20 (Q1): today returns come off the sale's day, so last month's profit changed
  when something came back this month, and the P&L disagreed with the Returns page.*
- **RPT-7** Received = Σ tendered − change over the period's sales; by method, cash is net of change (SALE-36);
  on-account parts are not received (SALE-37). Each method's line = Σ that method's payments.
- **RPT-8** Receivables = Σ balances of customers who owe (advances apart, disabled included); payables = Σ balances of
  suppliers owed (credits apart). Both are now, not the period's, and equal the Dues pages' totals.
- **RPT-9** Net profit = gross profit − expenses (by expense date) − losses (damage incl. give back, warranty
  replacements, at their movements' worth) − own use − staff − stock corrections (counts: written off positive, found
  negative) + recoveries (live ones; an undone recovery is gone, REC-8) + locker interest + discounts received +
  till over / short (sessions closed in the period). Each line = its raw rows' sum. Stock put on the books is
  not a count and never profit: an opening quantity (a product sheet), a product's very first stock however it came
  in (users' Q5), and + Add batch / intake with no bill (Q6) — *found 2026-11-20: each was booked as profit by its
  whole worth; migration 00042 marks the old rows.* A `to_shelf` move is none of
  these (STK-24).

### By time and peak hours

- **RPT-10** The period rows (day / week / month), peak-hour cells and the dashboard's spark and charts each add up to
  the P&L's revenue and gross profit for the same range, to the cent; a day with no sales is 0, not missing.
- **RPT-11** Every report puts a row on the shop's day of its time (a sale at 23:59 shop time is that day, 00:01 the
  next), weeks Monday–Sunday; "to" includes its whole last day; the dashboard's today and yesterday are shop days.

### Lines: products, categories, services

- **RPT-12** A line report's revenue is the period's sold revenue (SALE-31: the bill discount shared, tax out) less
  the period's refunds before tax (RPT-6), its cost likewise; quantity = sold − returned in the period. The bill
  discount is shared over the shop's own lines only: a pass-through line never takes a share in a report, so product
  totals = P&L revenue and gross profit to the cent. Product totals = Σ the ranked products; Σ categories (every
  category, nothing dropped) = product totals. *User, 2026-11-20 (Q3): the till and receipts don't change.* *machine
  (oracle).*
- **RPT-13** Service profit = the service's kept revenue − its components' cost − expenses tagged to it in the period;
  a service with only expenses still shows.

### People: sales by cashier and the Z report

- **RPT-14** Sales by cashier: each sale under its cashier (never the sales person, SALE-39); Σ cashiers' count, gross,
  discount and net = the P&L's sales count, gross (before discount), discount, revenue; a return comes off the net
  of whoever gave the refund, on its day (RPT-6). *Found by reading: net is not net of returns today, so Σ cashiers ≠
  the P&L as soon as anything comes back.*
- **ZR-1** A Z report counts exactly its till session's non-void sales (DRAW-6: `sales.session_id`, never a time
  window): count, gross (before discount), discount, total.
- **ZR-2** A Z report's returns are the refunds given **in that session** (by its cashier while it was open, whatever
  day the goods were sold), cash and otherwise; net = its sales' total − those refunds. A refund paid from a till
  belongs to the session its cash left — even one opened while the return waited (*found by make battle's power
  cuts, 2026-11-20*). A closed Z never changes.
  *User, 2026-11-20 (Q2): today it counts returns on its own sales at any time, so a Z printed at close changed when
  one of its sales came back next week.*
- **ZR-3** A Z report's by-method lines = Σ its sales' payments per method, cash net of change; its cash line +
  opening + pay-ins − withdrawals − refunds − expenses paid from it = expected cash (DRAW-1), and its difference =
  counted − expected (DRAW area).
- **ZR-4** Every drawer move of the session is in the Z's moves list, once, with its amount; a cashier sees only their
  own Z.

### Returns page and tax

- **RPT-16** The Returns page counts each return on the day it was made: Σ refunds = Σ `sale_return_items.refund_amount`
  in the period; receipts = distinct sales; most returned ranked by refund.
- **RPT-17** The tax report: each day's tax = Σ tax of that day's sales, base = Σ (total − tax); the period's total =
  the P&L's tax. It also shows the tax **given back** on returns (each refund's tax share, on the return's day) and
  **tax owed** = charged − given back. *User, 2026-11-20 (Q4).* A sale mixing tax rates still waits on RET-14.

### The pages that show balances and stock

- **RPT-20** Customer dues: every customer who owes (disabled included), balance = their balance (CUST rules), oldest
  unpaid and its age; over limit flagged only when a limit is set. Total = RPT-8's receivables.
- **RPT-21** Supplier dues total = payables (RPT-8); overdue = oldest unpaid bill past the supplier's credit days.
  Supplier spend and purchases count received deliveries only (never drafts), on the delivery's day: spend = total −
  credited, due = total − paid − credited per bill (PUR, SUP rules); Σ purchases' due for one supplier's bills in the
  period ≤ that supplier's balance + what was paid ahead.
- **RPT-22** Losses & Recovery: damage and warranty counts and worth = the P&L's losses; recovered = the P&L's
  recoveries; net = lost − recovered; its lines = one per loss, each with its live recovery (if any).
- **RPT-23** Top-up: each open till at its expected cash (DRAW-1), each active locker at its balance (LOCK rules),
  reorder need = Σ (reorder level − on hand) × cost for active, stocked, unfrozen products at or under their level.
- **RPT-24** Stock value: cost = Σ lots' remaining × lot cost, plus any on hand no lot accounts for × the product's
  cost; retail likewise at the lot's price (0 = the product's); a category = Σ its whole subtree; "missing cost" = on
  hand with neither a lot cost nor a product cost. Frozen stock is shown apart, never lost.
- **RPT-25** Expenses by category = Σ expenses by expense date; Σ categories = the P&L's expenses.

### Reports agree with each other

- **RPT-30** Over any period: P&L revenue = Σ period rows = Σ peak-hour cells = Σ cashiers' net (RPT-14); P&L tax =
  the tax report's total; P&L sales count = Σ period counts = Σ cashiers' counts; the dashboard's today = the P&L of
  today. *machine (oracle), after every random day.*
- **RPT-37** The Sales report and the sales list are the period's **receipts**: count = the P&L's (void left out
  unless asked for), each receipt's revenue before tax less pass-through face and what came back on it so far — so
  their totals match their rows. They differ from the P&L only by returns that cross the period's edge; the Sales
  report's trend is the P&L's (RPT-10). *Found in Task 2: revenue had tax in, and void sales were counted. Found
  in Task 6: returns came off at a share worked out again, not the refund given, a cent off per few returns.*
- **RPT-31** Over any period: the P&L's returns = the Returns page's refunds less the tax report's given back;
  Σ Z reports' returns of the sessions = the refunds given in those sessions.
- **RPT-32** Σ the P&L's by-method lines, on account left out, = received; Σ Z reports' by-method lines of the sessions = the same sales'
  by-method lines in the P&L (sales with no session apart, DRAW-6).
- **RPT-33** Losses & Recovery and the P&L show the same losses and recoveries for the same period (RPT-22).
- **RPT-34** Σ supplier dues = the P&L's payables; Σ supplier credit = the P&L's supplier credit.
- **RPT-35** A sheet download holds every row of the report for the period (never one page), the same numbers the
  page shows.
- **RPT-36** One report is one moment: every figure on a page (the P&L's ~20 sums, the dashboard, a Z) is read from
  the same snapshot, so a sale landing while it is read is in all of its figures or none. *Found by reading: each
  sum is its own query today.*
- **RPT-38** A report's work grows with the period's sale lines, never with the number of products the shop has: each
  sale's own figures (pass-through face, own line nets, cost) are worked out once per sale, and a line finds its
  product by its id. A year's reports on a full shop (bench: ~40,000 sales, ~2,000 products) answer within the
  hardware minimums (`docs/guide/install.md`). *Found by the low-spec bench (2026-11-24): a per-sale lookup walked the
  products list from the start for every sale — Finance (year) took 35 s on a fast PC once Iman's products were in,
  and a 50,000-product catalogue would make it far worse.*
