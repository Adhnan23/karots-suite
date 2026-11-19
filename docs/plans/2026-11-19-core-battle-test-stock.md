# Core battle test — area 4: stock (lots, packs, units, conversions, freeze, write-offs) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every rule of what the shop holds — on hand, lots, counts, write-offs, give back, recall, conversions,
packs, units, freeze — written down (`docs/rules/stock.md`), checked by machine, and every bug found fixed.

**Architecture:** Same layers as areas 1–3: rule book → auditor checks (`internal/ledgercheck/stock.go`) → case tests
per rule (`internal/battle/cases_stock_test.go`) → the random-days machine grows stock actions and keeps its own books
per product and lot → faults (a count / lot count / write-off / give back beside a sale, power cuts) → mutation.
**No Lean this area:** on hand is a sum; conversions' value split is checked by the auditor and the machine.

**Spec:** `docs/specs/2026-11-18-core-battle-test-design.md`

## Global Constraints

- As areas 1–3: core only; ledgercheck read-only; every check and test names its rule; a rule changes only with the
  user's yes; a bug = failing test first, then the fix, one commit each (`fix(stock): … (STK-4)`).
- `make test` grows by at most ~30 s; long runs in `make battle`, detached, logs in `~/.cache/karots-logs/`.
- Never run DB tests on `suite_db` while Playwright runs.

## Review Focus (suspected from reading the code, 2026-11-19)

- **STK-4 / STK-1:** `stock.Adjust` reads on hand with no row lock, then sets it absolutely: a sale between the read
  and the write is overwritten — on hand ≠ Σ movements (the delta was worked out from the old number). Fix: read it
  `FOR UPDATE` (the stock row first, then lots, the order a sale takes them).
- **STK-5:** `countLot` (`admin_stocktake_lots.go`) works the new total out from on hand read *outside* the
  transaction; a sale from another lot in between makes the lot's count land as no change (10 → count lot A 5 → 4,
  sale of 1 from B, Adjust sees 9 = 9 and does nothing). Fix: a lot count moves by the lot's own difference, inside
  one transaction.
- **STK-18:** Adjust, write-offs, `AddStock`, conversion runs into a whole-unit product take fractions; only the sale
  checks `UnitAllowDecimal`.
- **STK-17:** `parsePack` accepts a size of 2.5 (`Round(3)`), on a whole unit.
- **STK-2:** a reduction whose FEFO lots don't cover it (stock already outside lots) swallows the shortfall; dev
  `-seed` stock has no lots (the dev DB's SALE-20 / STK-2 noise). Find every path that can make on hand ≠ Σ lots.
- **STK-7:** a conversion's two movements record worth 0 (value moves only through the lots) — decide in the rule
  whether conversion movements carry worth (the P&L doesn't sum them); the auditor checks value out = value in.
- **Q1:** `DepleteFEFOLots` takes an expired or recall-stopped lot first when no lot is named.

---

### Task 1: The rule book for area 4

- [x] **Step 1:** Grid + rules in `docs/rules/stock.md` (STK; past `fix(` commits as rules).
- [x] **Step 2:** Q1–Q4 in `docs/rules/questions.md`; README links area 4.
- [x] **Step 3: Commit** — `docs(rules): area 4 rule book — stock, lots, counts, write-offs, conversions, freeze; questions Q1–Q4`
- [x] **Step 4:** Asked; all four answered 2026-11-19 as suggested (STK-21 good lots first, STK-15 no merging a recalled lot, STK-18 unit changes refused while fractions / packs, STK-4 a "type it in before selling" note) — built in Task 3 Step 3.

### Task 2: Auditor checks (`internal/ledgercheck/stock.go`)

- [x] **Step 1:** (STK-11 left to the cases and the machine: a conversion's lots aren't linked to its run, so the data can't show value out = value in) **Failing test** `internal/battle/ledger_stock_test.go` (the API builds the day): opening stock, intake
  into a new lot and a named one, a count up and down, a lot count, a sale FEFO and by picked lot and by pack, an
  oversell, a return restocked, damage / own use / staff (named lot and FEFO), a give back with a difference, a
  recall, a conversion run with a short weigh-out, a lot merge, freeze → put out for sale → unfreeze, a warranty
  replacement → no violations; then corrupt one row at a time and assert that rule's id: STK-2 (a lot's remaining off
  by 1; a lot of another product), STK-3 (on hand below zero), STK-7 (a count's movement with worth 0 when it moved
  units at a cost), STK-11 (a conversion run whose lots in are worth more than the lots out by > 0.01 a lot),
  STK-18 (a fraction on hand of a whole-unit product), STK-17 (a pack of 2.5).
- [x] **Step 2: Run, fail; write the checks; run, pass.**
- [x] **Step 3:** (grocery year: STK-18 on two packets a tea recipe uses by the cup — rule worded so, user 2026-11-19; dev DB: the e2e fixture's hand-set stock had no movements / no lot — `e2e-till.sql` now writes them; a till quick-add's opening was rightly worth 0 — the STK-7 check skips it) Run `Run` over a Demo Room year (`ledgercheck` gen test) and the dev DB: every violation is a bug
  (→ Task 3 test), a wrong rule (→ the user) or seed data (fix the seeder).
- [x] **Step 4: Commit** — `test(ledgercheck): area 4 checks — lots add up, never below zero, worth, conversions`

### Task 3: Case tests, one per `example` rule (`internal/battle/cases_stock_test.go`)

- [x] **Step 1:** (real: STK-17 pack of 2.5 3240ab4, STK-7 sheet opening worth 0 c9500bf, STK-18 fractions everywhere but the sale f359a19; the other rules held — STK-3,4,5,6,8–14,16,19,20,22 passed as written) One test per example rule: STK-3 (write-off / put out for sale / conversion / warranty beyond on
  hand refused, nothing changed), STK-4 (count up, down, to 0; worth at lot cost), STK-5 (lot count: other lots
  unchanged), STK-6 (named short lot refused; top-up keeps cost / price / expiry; new lot blank cost = product's),
  STK-8 (each reason its type; unknown refused; frozen out-for-sale write-off), STK-9 (counted more / less / 0, with
  and without other lots; value kept), STK-10 (recall, stop selling, undo), STK-11 (default and short weigh-out; lots
  carried), STK-12 (the guide's pencil example end to end), STK-13 (three frozen cases), STK-14 (fold), STK-15
  (merge: weighted cost, earliest expiry, rows kept), STK-16 (product merge), STK-17, STK-18, STK-19, STK-20,
  STK-21, STK-22 (stock value = lots), STK-24. Each ends with the auditor.
- [x] **Step 2:** Run; each failure → fix test-first, one commit each.
- [x] **Step 3: The user's answers** (Q1–Q4), test-first, one commit each. (STK-21 933875f, STK-15 63ac4a8, STK-18 units dbc84cb, STK-4 note dc87986)
- [x] **Step 4: Commit** — `test(battle): area 4 cases — one per rule`

### Task 4: Random days with stock (`internal/battle/machine_stock_test.go`)

- [ ] **Step 1:** The machine keeps books for a few products (one whole-unit with packs, one decimal loose product
  fed by a bag conversion, one frozen part of the run) and each of their lots. Actions: intake (new / named lot),
  count, lot count, sale (FEFO / picked lot / pack / oversell), return, write-off (each reason, named / FEFO), give
  back, recall, conversion run (short weigh-out), lot merge, freeze / put out for sale / unfreeze, warranty
  replacement. After every step: the auditor, each on hand and lot = the books (STK-1, STK-2), stock value = the books
  (STK-7), a refused action changes nothing.
- [ ] **Step 2:** Short run in `make test`; `make battle` long, detached, until 20,000 steps clean.
- [ ] **Step 3: Commit** — `test(battle): random shop days counting, writing off, converting and freezing stock`

### Task 5: Faults (`internal/battle/faults_stock_test.go`)

- [ ] **Step 1:** STK-4: a count while sales land (many at once) — on hand = Σ movements, the count's movement =
  counted − what was there. STK-5: a lot count beside a sale from another lot. STK-23: write-offs / conversions /
  give back racing a sale for the last units — never below zero, one refused; give back beside a sale from the same
  lot.
- [ ] **Step 2:** Power cut mid-count / mid-conversion / mid-give-back (the real binary killed): each whole or absent,
  the auditor clean.
- [ ] **Step 3:** Each failure → fix test-first, one commit each; then `test(battle): area 4 faults`.

### Task 6: Mutation

- [ ] **Step 1:** `make mutate` on `internal/features/stock` and `internal/features/conversions`; survivors replayed
  against the battle tests; each surviving mutant in stock / value code gets the test that kills it.
- [ ] **Step 2: Commit** — `test: area 4 mutants killed`

### Task 7: Close area 4

- [ ] **Step 1:** Q1–Q4's tests passing; `docs/guide/stock.md` says what changed.
- [ ] **Step 2:** Full run (`scripts/full-run.sh`, `COVER=1`), one clean `make battle`.
- [ ] **Step 3:** Coverage map updated (re-upload its four JSON datasets); `docs/status.md` — area 4 done, bugs found
  and fixed.
- [ ] **Step 4:** Fresh review of the area; fix findings test-first.
- [ ] **Step 5:** Commit — `docs(status): battle test area 4 done`; delete this plan. Area 5's plan replaces it.
