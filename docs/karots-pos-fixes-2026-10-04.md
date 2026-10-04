# karots-pos fixes to carry into karots-suite (2026-10-04)

**Read this before starting M4.** On 2026-10-04 a batch of fatal bugs was fixed in
the frozen karots-pos (11 commits, `b3100a6..fed43dc` on its `main`, no migrations).
karots-suite forked karots-pos's domain code (`internal/features/*`), so **most of
these bugs are in karots-suite too**. Check the M0–M3 work against each item below
and fix it here **before moving on**. Where an item belongs to a later milestone,
keep the rule in mind when you build that milestone.

Under the porting rule ("don't copy a flaw — upgrade it"), use the karots-pos
behaviour below as the minimum and design the karots-suite UI in its own style.
The karots-pos commits are the reference: `git -C ~/Projects/go/karots-pos show <sha>`.

## At a glance

"karots-suite now" was checked against karots-suite's code on 2026-10-04, at commit
`0b44de6`.

| # | karots-pos commit | Area | karots-suite now | Action |
|---|---|---|---|---|
| 1 | `b3100a6` | Purchase return against its delivery, priced at lot cost | **Already done**: it came FROM karots-suite (`6fcab43` + `0b44de6`) | Nothing |
| 2 | `c99b978` | Lot cost rounds to 2 decimals | **Bug present**: `internal/features/purchases/service_receive.go:37` uses `DivRound(gotQty, 4)` | Fix now (M3 area) |
| 3 | `b256484` | A return refunds what was PAID (the bill discount is shared out) | **Bug present**: `internal/features/sales/service_returns.go:243`; the preview in `internal/api/counter_returns.go:43` has it too | **Fix now (M1b counter returns)** |
| 4 | `3f77ef3` | The whole-sale return records its return and refund | **Bug present**: old `Return()` + `MarkItemFullyReturned` | Fix now |
| 5 | `4e36047` | Admin returns pay the cash refund from a picked locker or till | The counter already does it right (`counter_returns.go:137`, `MoveTx` in the same tx). The admin side isn't ported yet. `internal/features/sales/api.go` still defines `/api/sales/:id/return` + `/partial-return` (no cash moved). It's **dead code**: `sales.RegisterAPI` is never called | Delete that leftover `RegisterAPI` and its handlers. Use this rule when the admin sales returns land |
| 6 | `4665473` | Receipt lists page instead of stopping silently at the newest rows | **Bug present**: `counter_receipts.go:21,40`, `counter_returns.go:80`, `counter_warranty.go:126` all use `Limit: 50`, and nothing tells the user that more rows exist | Fix now (counter). **The M4 receipts hub must page from day one** |
| 7 | `27fbfd7` | Sales search also finds a sale by a product sold on it | **Missing**: `store_query.go:88,189` matches only receipt no / name / phone | Fix now |
| 8 | `f76bda2` | …using the till's word rules (any order, separators squashed, no fuzzy) | **Missing** | Same as 7 |
| 9 | `e36a057` | Recharge: partial reload reversal | No `plugins/` yet | When recharge is ported |
| 10 | `7ac1d14` | Plugin cash-outs go through cashflow, not `CashRegister.Withdraw` | No `plugins/` yet | When the plugins are ported |
| 11 | `fed43dc` | Neutral refusal text; old return JSON API removed | Text **already neutral** in karots-suite. API: dead code, see #5 | Delete it (#5) |

## Decisions the user made (don't re-ask)

- **Lot cost:** 2 decimals, no 4-decimal costing. The residue is accepted: a lot whose cost doesn't divide evenly (1000 over 12 units = 83.33) leaves a few cents owed on the bill when all of it is returned.
- **Free units on a delivery:** blended into the lot cost (10 paid + 2 free at 1000 = 83.33 each). A return credits that blended cost.
- **Bill discount on a refund:** shared pro-rata across the lines by their net value. If a cashier puts a discount "meant for one item" on the bill, a return of a *different* item still takes a share. The system can't know the intent; only an item discount carries it. Tell cashiers to use the item discount for that.
- **Returned goods** go back on the shelf at their normal price and keep their cost. A discount is the customer's, not the goods'.
- **Sales search:** match any word order and squash separators, but **no typo rescue** (fuzzy matching on sales history surfaces unrelated receipts).
- **Print & Copy returns:** a print job is a **full loss**. Nothing goes back to stock. Core returns refuse service lines; the plugin's "Reverse job" is the only way back.
- **Recharge reversal:** the carrier may return all, part, or none of a reload. The refund to the customer is a separate amount (it defaults to what came back).

---

## 2. Lot cost to 2 decimals (`c99b978`)

`internal/features/purchases/service_receive.go`, `lotCost`:

```go
return costBasis.DivRound(gotQty, 2) // was 4 — the column is DECIMAL(12,2) anyway
```

`stock_batches.cost_price` and `purchase_return_items.cost_price` are
`DECIMAL(12,2)`. Computing 4 decimals only let the DB truncate silently. Update
the freebie test case to expect `83.33`.

karots-pos also added two end-to-end tests in `purchasereturns_test.go`. Port them;
karots-suite's return code is the same:

- `TestReturnFromMixedDiscountDelivery`: receive via `purchases.CreateTx` with
  10 @ 100 + 2 free at 10% off the item, 5 @ 200, and Rs 190 off the bill (owed 1710).
  The lots are 67.50 and 180.00. Returning 3 of A and 2 of B credits **562.50**.
- `TestReturningWholeDeliveryClearsItsBillExactly`: return everything, credit 1710,
  and `credited_amount` = 1710.

## 3. A return refunds what was PAID (`b256484`) — the big one

**Bug:** `PartialReturnTx` refunded `line net ÷ qty × qty`. The line net already has
the item discount off, but **the bill discount was ignored**. Example: 3 @ 100 at 10%
off the item, plus 2 @ 50, with Rs 10 off the bill. Paid **360**; returning everything
refunded **370**. The REFUND slip, the CR- refund receipt, the drawer cash and the
customer-credit reduction all came from that one number.

**Data model reminder:** `sale_items.subtotal` is the line net after its item
discount. `sales.discount` is item discounts **plus** the bill discount.
`sales.total` is what was paid, with tax added.

**Rule:** refund per line = line net × (`sales.total` ÷ Σ line nets). The **last**
units of a line take exactly what's left of that line's paid value, so a line
returned in parts adds up to the cent.

`internal/features/sales/store_returns.go`:

```go
// One factor every reader multiplies by, so the refund, the sales list, the
// day-end summary and the Returns report all agree with the cash that went back.
const PaidFactorSQL = `COALESCE(s.total / NULLIF((SELECT SUM(x.subtotal) FROM sale_items x WHERE x.sale_id = s.id), 0), 1)`

func (r *Repository) PaidFactor(ctx context.Context, saleID int64) (decimal.Decimal, error) {
	var f decimal.Decimal
	err := r.q.GetContext(ctx, &f, `SELECT `+PaidFactorSQL+` FROM sales s WHERE s.id = $1`, saleID)
	return f, err
}

func (r *Repository) RefundedForItem(ctx context.Context, saleItemID int64) (decimal.Decimal, error) {
	var v decimal.Decimal
	err := r.q.GetContext(ctx, &v, `SELECT COALESCE(SUM(refund_amount), 0) FROM sale_return_items WHERE sale_item_id = $1`, saleItemID)
	return v, err
}
```

In `PartialReturnTx`, read `factor` once (before the loop), then per line:

```go
linePaid := it.Subtotal.Mul(factor).Round(2)
var lineRefund decimal.Decimal
if qty.Equal(it.ReturnableQty()) { // the last of this line: take exactly what's left
	prior, err := saleRepo.RefundedForItem(ctx, it.ID)
	if err != nil { return apperr.Internal("failed to read earlier refunds", err) }
	lineRefund = decimal.Max(linePaid.Sub(prior), decimal.Zero)
} else {
	lineRefund = linePaid.Mul(qty).Div(it.Quantity).Round(2)
}
```

(`it` is read **before** `AddReturnedQty`, so `ReturnableQty()` is the pre-return value.)

**The SQL readers must use the same factor**, or the drawer and reports disagree with
the cash that actually went back:

- `store_query.go` `saleProfitAgg`: `ret_val = COALESCE(SUM(...) FILTER (...), 0) * ` + `PaidFactorSQL`
- `service_summary.go` (day-end / Z-report Net): `SUM((si.subtotal / NULLIF(si.quantity,0)) * si.returned_qty * ` + `PaidFactorSQL` + `)`
- `reports/reports_sales.go` `Returns` (`refund_value`): same, times `sales.PaidFactorSQL`. `reports` imports `sales`; there's no cycle.
- **karots-suite only:** the counter return **preview** in `internal/api/counter_returns.go:43`
  (`RefundUnit`) must show the same figure, or the screen promises 90 and the
  drawer pays 87.57. Give the preview the line's paid value; it's simplest to
  expose `PaidFactor` and compute `linePaid` the same way.

Leave the product and revenue reports alone. They count goods at line level, before
the bill discount, by design.

**Test to port** (`return_discount_db_test.go`): the 3 @ 100 −10% + 2 @ 50, −10 on the
bill sale. Returning 1 of A gives **87.57**, the last 2 of A give **175.13**, B gives
**97.30**, and the refunds total exactly **360**. `ret_val` for the sale is also 360.

> **Test-harness gotcha found here:** the helper registered its DB cleanup with
> `t.Cleanup`, but the test did `defer conn.Close()`. Cleanups run *after* defers,
> so every delete hit a closed connection and **left sales behind** in the dev DB.
> Register `t.Cleanup(func(){ conn.Close() })` *before* the helper's cleanup
> (cleanups run last-in, first-out).

## 4. The whole-sale return records its refund (`3f77ef3`)

**Bug:** `Service.Return` restocked and dropped the customer credit, but recorded **no
`sale_returns` row and no refund**. So it never appeared on the refund slip, the Returns
report or the Z-report Net. It also refused a sale that had been partly returned.

**Fix:** it's now a line return of every remaining line, through the same code:

```go
func (s *Service) ReturnTx(ctx context.Context, tx *sqlx.Tx, id, userID int64) (*Detail, decimal.Decimal, int64, error) {
	// FindByID → NotFound / "already returned"; load items;
	// any remaining IsService line → Conflict("this sale includes a service item, …");
	// lines = every ReturnableQty() > 0 line; none → Conflict("nothing left on this sale to return")
	return s.PartialReturnTx(ctx, tx, id, PartialReturnInput{Lines: lines}, userID)
}
```

`Return` wraps `ReturnTx` in `WithTx`. Delete `MarkItemFullyReturned`; nothing else
uses it. Test: `TestWholeSaleReturnRecordsWhatWasPaid` (records 360 over 2 lines, the
status is `returned`, and a second call is refused).

## 5. Admin returns move the cash (`4e36047`)

**Bug:** in karots-pos, admin returns (the line return modal, and the whole-sale
return) recorded the refund but **no money left anywhere**. Only the cashier return
took it out of the till. An admin has no till.

**Rule:** a return and its cash refund commit together. The cash refund (the
non-credit part) leaves a **named** place: the cashier's own till, or for admin, a
**picked locker or open till** (karots-suite already has `MoneySourcePicker` +
`GET /admin/money-sources` + `parseLocation`). Ask for the source only when there is
cash to refund; a return taken wholly off the customer's credit moves nothing. The
refund is `cashflow.MoveTx{From: <source>, To: External, ReceiptKind: "refund", Ref: {sale_return, id}}`.

**For karots-suite now:** the counter is already correct. `internal/features/sales/api.go`
still holds the legacy `RegisterAPI` with the `/api/sales/:id/return` +
`/partial-return` handlers. They record refunds and move no cash, and karots-pos removed
them (`fed43dc`). Nothing calls `sales.RegisterAPI` in karots-suite (it's dead, like the
other legacy `RegisterAPI` files M3 removed), so delete it so it can't be wired back in. Build the admin sales return (when it lands) on `ReturnTx` /
`PartialReturnTx` + `MoveTx` from the picked source.

## 6. Receipt lists page (`4665473`)

**Bug:** every receipts tab cut off silently: Sales 100 (admin) / 50 (cashier), Cash,
Credit, Warranty, and the plugin tabs (500, or a whole month). Nothing said older
rows existed; you could only find them by searching.

**karots-pos fix:** fetch `Limit = pageSize + 1` with `Offset = (page-1) × pageSize`. If
the extra row came back, there is a **Next** page, with no `COUNT(*)` needed.
Prev/Next keep the tab's search, date range and filters. 50 per page. The domain
filters gained `Offset`:

- `cashflow.ReceiptFilter.Offset` → `LIMIT $5 OFFSET $6`
- `warranty.ClaimFilter.Offset` → `LIMIT $4 OFFSET $5`
- `customers.DebtFilter.Offset` → `LIMIT $n OFFSET $n+1`
- `sales.ListFilter` already had `Offset`
- for later: recharge `LedgerFilter.Offset`, documents `JobFilter.Offset`, repairs
  `CollectedFilter{Limit, Offset}` (zero Limit = every row, which the reports still use)

**For karots-suite:** the counter lists at `Limit: 50` (`counter_receipts.go:21,40`,
`counter_returns.go:80`, `counter_warranty.go:126`) have the same flaw. Page them, or
use infinite scroll, which suits a React list better; either way, never stop silently.
**The M4 receipts hub (S- / CR- / DP- / warranty) must page from the start.**

## 7–8. Sales search finds the product sold (`27fbfd7`, `f76bda2`)

Customers lose receipts; staff remember *what* was sold. The sales search (receipt
no / customer name / phone) now also matches a sale by any of its lines. One helper
is used by **both** `List` and `Summarize`, so the totals match the rows:

```go
func searchMatch(p string) string { // p = the query's placeholder, e.g. "$9"
	return `(` + p + `::text IS NULL OR
	       s.receipt_no ILIKE '%' || ` + p + ` || '%' OR
	       c.name       ILIKE '%' || ` + p + ` || '%' OR
	       c.phone      ILIKE '%' || ` + p + ` || '%' OR
	       EXISTS (SELECT 1 FROM sale_items si JOIN products pr ON pr.id = si.product_id
	               WHERE si.sale_id = s.id
	                 AND (pr.barcode = ` + p + ` OR NOT EXISTS (
	                      SELECT 1 FROM regexp_split_to_table(lower(btrim(` + p + `::text)), '\s+') AS tok
	                      WHERE NOT (
	                          lower(pr.name || ' ' || COALESCE(si.description, '')) LIKE '%' || tok || '%'
	                          OR translate(lower(pr.name || ' ' || COALESCE(si.description, '')), ' -_/.', '')
	                             LIKE '%' || translate(tok, ' -_/.', '') || '%')))))`
}
```

- **EXISTS, not a join:** a sale with several matching lines is listed (and counted)
  once.
- **Every word must match within ONE line** (product name + line label, e.g. a plugin
  line "A4 colour ×20"), in any order. "mineral 1l" finds "Mineral Water 1L";
  "flipflop" finds "Flip Flop". A scanned barcode matches exactly.
- `' -_/.'` mirrors `products.squashChars` (karots-pos `products/search.go`). Keep
  them in sync.
- Update the search placeholders to say "/ product".
- Test: `TestSalesSearchFindsSaleByProductOnce` (one row per sale, `Summarize.Count`
  equals the rows, "disc test" and "A returndisc" match, "test zzz" doesn't).

## 9–10. Plugins (for when they're ported)

**Rule for every plugin cash-out:** money a plugin hands out goes through
`plugin.Core.Cashflow` (`Move` / `MoveTx`, Till → External) with a CR- receipt, **never**
`CashRegister.Withdraw`. `Withdraw` is refused whenever the shop disables untracked
cash-out (`settings.allow_untracked_cash = false`). In karots-pos that silently broke,
on such shops, all of these:

| Plugin flow | Receipt kind | Notes |
|---|---|---|
| Recharge: reload reversal refund | `refund` | Same tx as the float reversal (was 2 steps, so it could refund and then fail) |
| Recharge: customer withdrawal (cash out, float in) | default (`payment`) | Shared `payCustomer` helper |
| Recharge: bill get-money, device and on-credit cash advance | default (`payment`) | Same |
| Print & Copy: "Reverse job" refund | `refund` | One tx with the stamp that marks the job reversed + the cancelling expense (`Expenses.CreateInTx`). **Full loss, no restock** |
| Print & Copy: labour paid out of a till | `expense` | Till → External, recorded as an expense movement |
| Repairs: refund of a deposit larger than the final bill | `refund` | `Party` = the customer |

All six were verified live in the Demo Room with untracked cash-out off.

**Recharge partial reload reversal (`e36a057`):** the store function takes
`returned decimal.Decimal` (0..reload amount) instead of a mode.
- `returned > 0`: a `reversal` row of exactly `returned`, so the float goes up by that much.
- `returned == 0`: `reversal_lost` (no float move).
- Whatever didn't come back stays gone; the un-recovered refill expense already
  carries that loss, so don't book it twice.

The handler takes `returned` and `refund` separately (each 0..amount). The UI fills
both with "Failed — full float back" / "Wrong number — nothing back" buttons. The
refund follows `returned` until edited, and a note says how much the shop absorbs when
the refund is bigger than what came back. Test: `TestReverseReload_PartialReturnsWhatCameBack`.

**Service lines are not returnable through core returns.** The refusal text must stay
plugin-neutral (karots-suite already says "service item"). A sale with a service line
can't be whole-returned; its other lines can be line-returned.

---

## Suggested order

1. #3 + #4 (refunds; this is money leaving the drawer wrongly), with their tests, then
   the counter return preview.
2. #5: delete the dead legacy `sales.RegisterAPI` (its return handlers move no cash).
3. #2 (lot cost) + its two purchase-return tests.
4. #7–8 (search) and #6 (counter list paging).
5. Record what was carried over in `docs/status.md` (like the `ababde1` line), then start M4
   with #6's paging rule and #5's refund rule in mind.
