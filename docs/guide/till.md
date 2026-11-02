# Selling at the till

The till is made for a desktop PC or a kiosk at the counter. Cashiers sign in with their phone
number and PIN. After a few idle minutes the till locks itself, and the same cashier or any admin
can unlock it.

![The sell screen](../images/till/sell-light.jpg)

## The sell screen

- **Left:** a search bar (it takes a scanner too), the group chips (Frequently sold, your cashier
  menu groups, plugin menus) and product cards with price and stock.
- **Middle:** the current sale. Change a quantity with − / +, set a pack or loose price, add a
  line discount, or tap a line for its info.
- **Right:** Retail or Wholesale, the customer (walk-in or a credit customer), the tenders
  (cash, card, online and the rest), quick cash buttons, the bill discount and **Complete sale**.

Search finds a product by its name or its **local name** (Sinhala / Tamil, set on the product), and
every word you type may match either. Scanning a barcode adds the product (any of its barcodes, or a pack's own). If a
search finds nothing, pressing Enter (or **+ Quick item** beside the search box) adds it as a **quick item** with a name
and price, and the item goes to admin's review list. A scanned barcode no product has asks first: **Existing item**
(the same item with a different barcode: find it, and the code becomes one of its barcodes) or **New item** (a quick
item with the code filled in). A sale is never
blocked: anything odd (selling below stock, a missing price) is sold first and flagged for review.

### Keys

| Key | Does |
|---|---|
| **F1** | Back to the menu. Clears the **search only**, never the cart |
| **F2 / F3** | Jump to search |
| **F4** | Hold the sale |
| **F9** | Bill discount |
| **F10** | Go to the amount; then **Enter** completes the sale |
| **Enter** after a sale | Start the next sale |
| **Esc** | Leave the field you're in |

Only **New sale** clears the cart. On a kiosk the browser's own keys are switched off: no F5
reload and no right-click outside text boxes.

### Paying

- Split the bill across tenders, use **Fill** to put the rest on one tender, and keep the change
  as the customer's advance (**Keep** / **Keep all**).
- **On account** sells on credit to a customer with a limit. A cashier can't go over the limit,
  but an admin can approve it.
- **Cheque** takes a cheque from a named customer (bank, number, date). It goes into Cheques in
  hand and prints on the receipt; an admin marks it cleared or bounced later (see
  [Money → Cheques](money.md#cheques)).
- When change is short in the drawer, the till warns you. It doesn't stop the sale.

### Holds

**Hold** (F4) parks the sale and frees the till. Resume a held sale from the Hold list. The till
won't resume one over a cart that already has items in it.

![The till in dark mode](../images/till/sell-dark.jpg)

## Send to cashier

For a shop where one person hands out the goods and another takes the money. Make the first a **Sales person** on the
Users page, with their own login.

- **The sales person** builds the cart as usual: products, quantities, line discounts, the customer (walk-ins too),
  plugin items. They have no till to open, no payment, no bill discount and no money tabs. They can **Hold** a cart for
  themselves. **Send to [cashier]** shows one button per cashier with a till open; with none open they hold it for now.
- **A ticket is on screen only; nothing prints.** It gets a two-digit number, random but unique among those waiting, shown
  big for the sales person to tell the customer. An optional name or note ("blue shirt") and the item list help the
  cashier match it to the person and the goods in front of them. **Undo** on the "sent" message, or tapping it under
  **Sent**, takes it back.
- **The chosen cashier** hears a beep and gets a popup with **Open**, on any till tab, without stopping the sale in
  progress. Every cashier with a till open can see every ticket under the cart, and open it if the customer comes to
  them. Finish or hold the current sale first. Only one till can open a ticket.
- **Not opened in 2 minutes:** it goes to the next open till, with a new popup. With only one till open, it stays.
- **The cashier takes the money** the normal way. The receipt prints, the drawer opens and stock moves only then, so a
  customer who walks away is just **Clear**, with nothing to return. **Sales** shows who prepared the sale.
- **Closing a till with tickets on it:** each one must first be opened and sold, passed to another open till, or sent
  back to the sales person's Held. A till an admin closes for someone passes its tickets on by itself.

A customer buying something small from the cashier is an ordinary sale; tickets keep waiting meanwhile. Tickets need the
main PC, and a sales person can't sign in to Pocket Cashier: in a power cut, one cashier sells from the phone or tablet.
Pocket Cashier can't open tickets yet.

## Opening and closing the till

Each cashier opens their till by counting the float note by note. Deposits and withdrawals move
cash to a [locker](money.md#lockers). At close the cashier counts again, the till shows what it
should hold, and any over / short is recorded. A till left open overnight shows on the dashboard,
where an admin or manager can close it for the cashier.

## Counter tabs

The tabs along the top do the counter's other jobs without leaving the till. **F1** on any tab
goes back to Sell.

### Returns

Find the sale by receipt number, customer, phone or product. Pick what's coming back and why; the refund is what was paid for it.

![Returns](../images/till/returns.jpg)

### Credit

Customers who owe the shop. Take a payment or an advance (by cash, card, online or one or more
cheques), or refund one.

![Credit](../images/till/credit.jpg)

### Receipts

Every slip of the shift (sales, cash, credit, warranty). Print one again at 80 or 58 mm, or as A4 / PDF.

![Receipts](../images/till/receipts.jpg)

### Write-off

Take damaged, expired or shop-use stock out, from the oldest batch first.

![Write-off](../images/till/write-off.jpg)

### Convert

Break open a pack (a 25 kg sack into 1 kg bags) and record what actually came out.

![Convert](../images/till/convert.jpg)

### Labels

Print barcode labels for a product.

![Labels](../images/till/labels.jpg)

### Low stock

What will run out before the next delivery, by supplier (if the cashier may see it).

![Low stock](../images/till/low-stock.jpg)

### Warranty

Find a unit by serial or receipt and swap it under warranty.

![Warranty](../images/till/warranty.jpg)

### Suppliers

Pay a supplier (cash, or a cheque on a bank account the cashier may use), receive stock or place
an order at the counter (if the cashier may handle suppliers).

![Suppliers](../images/till/suppliers.jpg)

### Expenses

Pay a shop expense from the drawer, or by cheque.

![Expenses](../images/till/expenses.jpg)

What each cashier sees here is set per user: see [People & users](people.md#users-and-rights).
