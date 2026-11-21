# Print & Copy (DOC)

Code: `plugins/documents` (store.go: services, grid, quote; resolve.go: price and paper picks; extras.go;
jobs.go: the job ledger + report; api.go admin; api_till.go till; merge.go), `web/src/plugins/documents`
(till.ts: job lines, extras, sold jobs). Tables: `doc_service` (a core service product made a print service:
metered | custom, its category), `doc_price` (size × type × colour × side × min qty → unit price),
`doc_consumable` (paper per sheet by size / type), `doc_extra` (Urgent, Hole punch…: per job or per piece, may take
a product), `doc_job` (one per job sold or shop use), `doc_payout` (old karots-pos labour; read only).

## The words the rules use

- **Metered** — priced from the grid: the band = the price row of the job's size, type, colour and side with the
  largest min qty ≤ copies. **Custom** — the cashier types the price (CV, photo edit).
- **Sheets** — paper used: copies, halved (rounded up) for 2-side. Paper comes from `doc_consumable`; toner / ink per
  copy from core's recipe of the service product.
- **Job line** — a service sale line carrying `plugin.documents` (its options), its paper and toner as the line's
  components; an extra is a line beside it in the same group (its own product only, none of the service's recipe).
- **Shop use** — a job the shop makes for itself: stock out as `own_use`, a `doc_job` of kind `own_use`, no sale.

## The grid

Columns: **Do** · **Till** (panel, quote) · **Sale** (core money / stock) · **Return** · **Report** · **Merge** ·
**Faults**.

| Feature | Do | Till | Sale | Return | Report | Merge | Faults |
|---|---|---|---|---|---|---|---|
| Make / link a print service | DOC-1 | DOC-2 | — a | — a | — a | DOC-9 | PLG-3,4,5,6 |
| Edit kind, on / off | DOC-1 | DOC-2 | — a | — a | — a | — a | PLG-5 |
| Save the grid + paper | DOC-3 | DOC-2 | — a | — a | — a | DOC-9 | PLG-4,5 |
| Save extras | DOC-3 | DOC-2 | — a | — a | — a | DOC-9 | PLG-4,5 |
| Quote | DOC-4 | DOC-4 | — a | — a | — a | — a | PLG-5 |
| Sell a metered job (+ extras) | DOC-5 | DOC-4 | DOC-5 | DOC-7 | DOC-6 | — a | DOC-6 |
| Sell a custom job | DOC-5 | DOC-2 | DOC-5 | DOC-7 | DOC-6 | — a | DOC-6 |
| Shop use | DOC-8 | — c | DOC-8 | — d | DOC-6 | — a | PLG-4,5,6 |

a — nothing to move. c — till panel only. d — nothing sold, nothing to return.

## Rules

```
DOC-1  A print service is one core service product (never a stocked item, never linked twice), kind metered
       or custom, category copy / print / laminate / bind / other. Edits and on / off name a service that
       exists, else 404 and nothing changes.
       Checked by: refusals, ledgercheck (one service per product, the product is a service).

DOC-2  The till offers exactly the services that are on (service and its product active), each with every
       size × type × colour × side its grid prices, bands in rising min qty; laminate and bind ignore colour
       and sides.
       Checked by: example.

DOC-3  A grid saves whole or not at all: prices ≥ 0 at min qty ≥ 1, no two rows for the same size, type,
       colour, side and min qty; paper is a stocked product (never a service) at a qty above 0 per sheet;
       an extra has a name, a price ≥ 0, a qty above 0 and, if it takes stock, a stocked product.
       Checked by: refusals, ledgercheck.

DOC-4  A quote is the band's unit price × copies; sheets = copies (2-side: rounded up half); paper and toner
       are what the grid's paper and the service's recipe say for those sheets and copies. Copies ≤ 0, an off
       service, a custom service or an option with no price is refused. Go and the till agree (parity).
       Checked by: example (table), quote_parity_test.

DOC-5  A job sale is an ordinary sale: core's sale, discount, stock and payment rules hold; the job's paper and
       toner leave stock as the line's components (an extra takes only its own product), and the auditor
       passes.
       Checked by: example, ledgercheck (core's checks after every battle test).

DOC-6  Every job line of a sale has exactly one job row, linked to that line, its qty, price and total copied
       from the line (never from the till's word) — written with the sale (its transaction), never for a
       line that isn't a job, never doubled by a resend. A void sale's jobs leave the report. The price is
       trusted like any service price. (User, 2026-11-21: the sale-line hook, not an after-sale call.)
       Checked by: example, ledgercheck (job rows against sale_items).

DOC-7  A returned job (core Returns, whole group with its extras, RET-13) takes its copies and revenue off the
       report; the sheets stay (that paper is gone). The refund is core's (RET-4, RET-5).
       Checked by: example.

DOC-8  Shop use takes exactly the quote's paper and toner out of stock as own_use with one job row, all or
       nothing; too little counted stock is refused (409) and changes nothing.
       Checked by: example, failure sweep.

DOC-9  A product merge: paper and extras move to the kept product (a service already taking that size from it
       keeps one row); two print services become the kept product's one, the duplicate's jobs moving to it
       (PLG-2).
       Checked by: example, ledgercheck (no row names a merged-away product).

DOC-10 The report counts kept copies and revenue the way core counts a line (returns off, bill discount
       shared — RPT-12), and its revenue for a period equals the job lines' kept revenue in core's Sales
       report for those services.
       Checked by: example (ltest.Agree-style against core's report).
```
