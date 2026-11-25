# Plan — the Catalogue plugin (spec: `docs/specs/catalogue-plugin.md`, rules: CTL, REV)

Rule first, a failing test naming it, then code; one commit per task; `make test` green before each commit.

- [x] 0. Rules: `docs/rules/plugins/catalogue.md` (CTL-1…12), REV-1/2 in `stock.md`, spec + plan.
- [x] 1. Core review hook (REV-1, 2): `plugin.ReviewSource` + the review list / Mark reviewed / `reviewActions` slot;
      a test plugin in the test proves notes, one row per product, clear.
- [x] 2. Core: quick item and intake-products take `extra` (ProductExtras); `productAssists` renders in QuickItem and
      the purchase NewProduct (`where`); `AssistFill.unitId`. Tests: an extra on a quick item reaches its hook.
- [x] 3. Plugin skeleton: plugin.json, migrations (4 tables), data group, status route, page; `make plugins P=catalogue`.
- [x] 4. Sync (CTL-2, 3): index → changes / whole, SHA-256, one transaction, forward only; boot + 03:30 + Check now.
      Tests against a local test server: both ways equal, bad hash, cut, missing change file, older index.
- [x] 5. Search (CTL-5) + the assist UI (From catalogue next to Identify; form, intake, quick, purchase).
- [x] 6. Pick → save (CTL-6, 7, 10): Validate / Save, link, empty Product Plus fields filled; review note "From the
      catalogue". With and without Product Plus.
- [x] 7. Later changes (CTL-8, 9, 12): sync's review step, Use the catalogue's / Keep mine; gone into.
- [x] 8. Merge (CTL-11), ledger checks (CTL-2 count, CTL-7 links), battle file, faults (PLG-4, 6).
- [x] 9. Pocket Cashier: offline route answers "offline"; online quick item shows the assist (CTL-4).
- [x] 10. Shop Builder bakes the address; `CATALOGUE_DIR` in the suite's `.env`; dev `KAROTS_CATALOGUE_URL`.
- [x] 11. Demo Room: a small sample copy (user, 2026-11-24) so its shops can pick. Docs + legal (README, guide, shots,
      First Schedule, §15 "in the service plan", agreement), docscheck.
- [ ] 12. `make test-all`, look at every screen at 360 px light + dark, Iman's profile gets the plugin; delete spec +
      plan; status.
