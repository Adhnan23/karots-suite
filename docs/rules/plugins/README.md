# The rule book for plugins

Each plugin is held to the same layers as core (`docs/rules/README.md`): a grid of its features × every later action,
an auditor check per rule the data can show, a case test per rule, faults, mutation. A plugin's checks live in its own
package (`plugins/<key>/ledger.go`, registered with `ledgercheck.Add`), so they run wherever the plugin is compiled in
— its own battle tests, the Demo Room's year (`internal/demo/gentest`) — and core never names them.

A plugin's battle tests are `plugins/<key>/battle_db_test.go`: a throwaway shop on the real server with that plugin
compiled in (`ltest.NewShop`), the core auditor and the plugin's own checks after every test.

## Rules every plugin keeps (PLG)

```
PLG-1  A product-data plugin never moves money or stock: none of its actions changes a sale, a payment, a
       till, a locker, a balance, a lot or a stock movement, and core's auditor passes after every one.
       (Plugins that sell — groups 2 and 3 — keep core's sales rules instead, each in its own file.)
       Checked by: example (a snapshot of every money and stock table before and after), ledgercheck.

PLG-2  A product merge leaves no row of the plugin naming the merged-away product. Where the plugin keeps
       one row per product, the kept product's own row wins and the duplicate's moves over only when the kept
       one has none.
       Checked by: example (merge through core's API), ledgercheck (no row names a merged-away product).

PLG-3  The plugin never makes a row naming a product, category or record that doesn't exist: such a request
       is refused (4xx) and changes nothing.
       Checked by: example (refusals).

PLG-4  Every write is all-or-nothing: a database failure at any statement leaves the plugin's tables and
       core's as they were.
       Checked by: example (the failure sweep, ltest.NewFaultyDB).

PLG-5  A bad request (missing, out of range, wrong role) is refused with a clear 4xx and changes nothing.
       Checked by: example (refusals).

PLG-6  The same request sent twice, or two at once, ends as one: never two products, two asks or two rows
       where one was meant.
       Checked by: example (faults: at once, sent twice).

PLG-7  The plugin's data group lists every table it owns, so a backup carries all of it and a restore brings
       it back whole.
       Checked by: example (every table the plugin's migrations make is in its data group).
```

## Plugins

Group 1 — product data only (2026-11-21):
[Clearance](clearance.md) (CLR) · [Customer requests](requests.md) (REQ) · [Product Plus](productplus.md) (PPF) ·
[Alternatives](alternatives.md) (ALT) · [AI Assistant](aicatalog.md) (AIC)

Group 2 — plugins that sell through core's till (2026-11-21):
[Combos](combos.md) (CMB) · [Print & Copy](documents.md) (DOC) · [Pharmacy](pharmacy.md) (PHA)

Group 3 — plugins with money of their own (2026-11-21):
[Repairs](repairs.md) (REP) · [Reload & Bills](recharge.md) (RCH)
