# Clearance (CLR)

Code: `plugins/clearance` (store.go: live markdowns, candidates, dead stock; api.go), `web/src/plugins/clearance`.
Tables: `clearance_markdowns` (one per product), `clearance_settings` (one row).

## The words the rules use

- **Markdown** — a percent off a product's price the admin approved (`status = 'approved'`), or a **snooze**
  (`'dismissed'`, hides the product for one stale period).
- **Live** — approved, above 0 %, the product has stock, and no lot came in after it was approved (fresh stock sells at
  full price). Only a live markdown reaches the till: the card badge, the apply / skip prompt and the ⓘ row.
- **Floor** — cost × (1 + minimum margin %); the cost is the expiring lot's own when the product is listed for expiry.
- **Dead** — active, not a service, no stock, nothing moved or sold for `dead_months`.

## The grid

Columns: **Do** · **Till** (badge, prompt, ⓘ) · **Core** (money, stock — PLG-1) · **Merge** · **Faults** (twice, at
once, a failure, bad requests).

| Feature | Do | Till | Core | Merge | Faults |
|---|---|---|---|---|---|
| Approve a markdown | CLR-2 | CLR-1, CLR-4 | PLG-1 | CLR-8 | PLG-3,4,5,6 |
| A markdown ends (sold out, fresh lot) | CLR-1 | CLR-1 | — a | CLR-8 | — b |
| Suggested percent | CLR-3 | — c | — a | — c | — b |
| Snooze / bring back | CLR-5 | CLR-5 | PLG-1 | CLR-8 | PLG-4,5 |
| Dead stock, disable | CLR-6 | — c | PLG-1 | — c | PLG-4,5 |
| Settings | CLR-7 | — c | — a | — c | PLG-5 |

a — nothing to move. b — read-only. c — not shown there.

## Rules

```
CLR-1  The till sees exactly the live markdowns: badge, prompt and ⓘ row name the same products, and a
       markdown stops being live by itself once its stock is gone or a lot arrives after it was approved.
       Checked by: example.

CLR-2  Approve takes a percent above 0 and under 100 for an active product that isn't a service; anything
       else is refused and changes nothing. A percent that takes the price under the floor is the admin's
       call: allowed, and the page shows it in red.
       Checked by: example, refusals.

CLR-3  The suggested percent never takes the new price under the floor; it is the shop's default when that
       fits, else the most that fits (one decimal, cut down), else 0.
       Checked by: example (table + random prices).

CLR-4  At the till a markdown is an ordinary line discount the cashier may skip: core's discount and return
       rules hold (a marked-down line refunds what was paid for it, RET-4).
       Checked by: example.

CLR-5  A snoozed product is never live and is hidden from the list for one stale period; bringing it back
       shows it again at once.
       Checked by: example.

CLR-6  Disable retires only products still dead when it runs (a product that got stock or a sale since the
       page loaded is skipped, and the answer says how many); each goes through core's reversible disable.
       (User, 2026-11-21.)
       Checked by: example.

CLR-7  Settings take days and months of at least 1, a suggested markdown above 0 and under 100 %, a margin
       of 0 or more.
       Checked by: refusals.

CLR-8  A product merge keeps the kept product's markdown; the duplicate's moves over only when the kept one
       has none (PLG-2).
       Checked by: example, ledgercheck.

CLR-9  The data holds its own shape: one row per product, status approved or dismissed, a snooze at 0 %, an
       approved markdown above 0 and under 100 %, settings in range, no row for a product that doesn't exist.
       Checked by: ledgercheck.
```
