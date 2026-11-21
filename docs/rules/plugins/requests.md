# Customer requests (REQ)

Code: `plugins/requests` (store.go, api.go = the till's add / ask again / similar, api_admin.go),
`web/src/plugins/requests`. Tables: `requests` (a wanted item; its row is also its first ask), `request_asks` (repeat
asks).

## The words the rules use

- **Status** (stored): **open** (wanted), **converted** (has a product, made from it or linked), **declined** (won't be
  stocked; a reason the till shows the next asker).
- **Stage** (worked out, never stored): open → **waiting** (converted, out of stock, someone to call) → **call** (in
  stock, someone to call) → **done** (nobody left to call, or declined).
- **Ask** — one customer asking; the request row is the first, `request_asks` the repeats.

## The grid

Columns: **Do** · **Asks** (who to call) · **Product** (core) · **Merge** (product merged) · **Faults**.

| Feature | Do | Asks | Product | Merge | Faults |
|---|---|---|---|---|---|
| Add (till) | REQ-1 | REQ-2 | — a | — a | PLG-5 |
| Ask again (till) | REQ-2 | REQ-2 | — a | — a | PLG-3,5 |
| Make a product | REQ-3 | REQ-1 | REQ-3 | REQ-7 | REQ-3, PLG-4,6 |
| Link a product | REQ-3 | REQ-1 | REQ-3 | REQ-7 | PLG-3,6 |
| Decline / reopen | REQ-5 | REQ-1 | — a | — a | PLG-5 |
| Merge two requests | REQ-4 | REQ-4 | — a | — a | REQ-4, PLG-4 |
| Called | REQ-8 | REQ-1 | — a | — a | PLG-5 |
| Clear done | REQ-6 | REQ-6 | — a | — a | PLG-4 |

a — not touched.

## Rules

```
REQ-1  A request's stage follows its status and its product's stock: open → open; converted with someone
       left to call → waiting (no stock) or call (in stock); otherwise done. The tab counts are the rows each
       tab lists.
       Checked by: example.

REQ-2  Asking again works only while the item is wanted (open, waiting) or declined; an ask is never lost —
       not by a merge, not by asking while a merge runs.
       Checked by: example, faults.

REQ-3  Make a product makes exactly one product and closes the request onto it; sent twice or twice at once
       it is still one product (the second is refused, nothing made). Link needs a product that exists. Both
       are refused unless the request is open.
       Checked by: example, faults.

REQ-4  Merge folds one open request into another open one: every ask of the merged one (its own row and its
       repeats) ends on the kept one, the count adds up, the merged one is gone. Never a request into itself.
       Checked by: example, faults (two merges at once, a merge beside an ask).

REQ-5  Decline needs a reason; reopen puts the request back to open with no product and no reason.
       Checked by: example, refusals.

REQ-6  Clear done deletes only converted requests with nobody left to call; declined ones stay (the till
       shows them to the next asker).
       Checked by: example.

REQ-7  A product merge points the requests at the kept product (PLG-2).
       Checked by: example, ledgercheck.

REQ-8  Called marks one ask; once every ask with a phone is called the request is done.
       Checked by: example.

REQ-9  The data holds its own shape: status open, converted or declined; a converted request names a
       product that exists; an open or declined one names none; a declined one has a reason.
       Checked by: ledgercheck.
```
