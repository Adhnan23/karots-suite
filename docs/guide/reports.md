# Reports & dashboard

The dashboard shows today against yesterday the moment you sign in; the reports cover sales,
profit, stock, suppliers and money for any period, and work on a phone.

## Dashboard

![Dashboard](../images/admin/dashboard.jpg)

The first page after sign-in. It shows:

- **Sales today**, **gross profit**, **cash in the tills** and **owed to you**, each against
  yesterday.
- **Sales this period against the one before**, over 7, 14, 30 or 90 days, as a chart or a
  table.
- **How customers paid today**.
- **Top sellers** for the period.
- **Needs your attention**: a till left open, cash sales with no till open, customers over
  their credit limit, expired or expiring lots, low stock, quick items to review, and backup
  copies that have stopped. Each one links to where you fix it.
- **Who's on now** (admins only): the staff signed in right now and what they're doing.

![Dashboard, dark](../images/admin/dashboard-dark.jpg)

## Reports

![All reports](../images/reports/index.jpg)

Pick a report, set the period, then print it or **Export** it to Excel. Every report works on a
phone.

| Sales and customers | Money and profit | Inventory and suppliers |
|---|---|---|
| Sales: receipts, profit and the trend, against the period before | Finance and P&L | Inventory valuation, at cost and at selling price |
| Top products by revenue, units or profit | Profit by category | Batches and expiry |
| Product sales: one product over time, beside last year | Expenses by category | Stock count sheet (printable) |
| Sales by cashier | Cash register: till sessions, over / short | Top-up and reorder, against the money on hand |
| Peak hours by day and hour | Tax charged, given back on returns, and owed | Purchases and what's still to pay |
| Returns | Service profit | Spend by supplier |
| Customer dues, and for how long | Losses and recovery | Supplier dues and what's overdue |
| | Recipe variance | |

**How the numbers are counted.** Every report agrees with the others and with the P&L. A return
counts on **the day it was given back**, at the refund actually given, so a month already over
never changes when something comes back later. Tax is never revenue: the Tax report holds it, with
the tax given back on returns and the tax still owed. Airtime and other money passing through is
never revenue either, and a bill discount comes off the shop's own goods. The **Sales** report is
the period's receipts, each less whatever came back on it so far, so its totals match the receipts
listed.

![Sales report](../images/reports/sales.jpg)

![Top products](../images/reports/top-products.jpg)

![Peak hours](../images/reports/peak-hours.jpg)

![Profit by category](../images/reports/profit-by-category.jpg)

The **Sales** page under Sell lists every sale. Open one to see its lines, print it again or take
goods back.

![Sales](../images/admin/sales.jpg)

## On a phone

<p>
  <img src="../images/phone/dashboard.jpg" width="220" alt="Dashboard on a phone">
  <img src="../images/phone/sales.jpg" width="220" alt="Sales report on a phone">
  <img src="../images/phone/top-products.jpg" width="220" alt="Top products on a phone">
</p>

To watch the shop from home, owners connect their phone to the shop PC over Tailscale. The suite
itself stays on the shop's network.
