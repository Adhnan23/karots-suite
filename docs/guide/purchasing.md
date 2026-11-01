# Purchasing

Suppliers, what you order from them, what arrives and what goes back. You'll find it under
**Purchasing**.

## Suppliers

![Suppliers](../images/purchasing/suppliers.jpg)

The top of the page shows what you owe all suppliers and what they owe you. Each supplier has
credit days, a running balance and a statement you can print. **Pay** a supplier against chosen
bills, against an old debt, or split across them, from a locker or the till. An old balance from
before the suite can be entered as an opening balance.

A supplier who must be told about expiring stock in good time gets their own **Warn before expiry**
number on their form; see [Batches and expiry](stock.md#batches-and-expiry).

## Reorder

![Reorder](../images/purchasing/reorder.jpg)

Everything that will run out before its supplier comes again, with how much to buy. The
quantities come from the [smart reorder levels](stock.md#smart-reorder-levels). Tick the items,
check the suppliers and make the orders: one purchase order per supplier. **Print** gives a
paper list to take round.

## Purchase orders and receiving

![Purchases](../images/purchasing/purchases.jpg)

- **New order:** pick a supplier and add lines, or build the order from Reorder. Print or save
  the PO.
- **Receive delivery:** tick off what came. Short, extra or forgotten items are handled, and the
  rest of the order can stay open. A **margin guard** warns when a new cost leaves too little
  margin at the current price. Receiving prints a **GRN**.
- Pay now or leave it on credit. The **Received** tab shows each delivery and what's still owed
  on it.

## Returns to suppliers

![Purchase returns](../images/purchasing/returns.jpg)

Send stock back against a delivery: short expiry, slow movers, damage. Each return takes the
stock off and comes off what you owe that supplier.

## Giving a batch back

When a supplier takes a batch back, or you want to give it to them, and you'll settle it later:

1. **From this supplier**, on the supplier's page, lists their expired batches and those close to
   expiry. Press **Told them** to keep the date you told them. It's only a record.
2. Press **Give back**, here or on **Inventory → Batches**, and type what you counted on the shelf.
   All of it goes back.
3. It then waits on that supplier as "given back". Press **Settle** when they replace it or pay for
   it, in full or part. Until then it counts as a loss in reports, like any write-off.

Batches get mixed on the shelf, so the count wins. If it's more than the system says, the extra
comes from the product's other batches. If it's less, the rest moves onto its next batch. Only when
there's no other batch to take it does the stock total change, as an adjustment in **Movements**.
Any other mix-up shows at the next stock-take. Units that move between batches of different cost
blend into that batch's cost, so the value of your stock stays right.

A batch with no supplier can't be given back until its product has a preferred supplier.
