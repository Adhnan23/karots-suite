# Core battle test — area 7: reports and the Z report = the raw data — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every report figure is a sum of the raw rows areas 1–6 hold to their rules, and reports showing the same
thing show the same number (`docs/rules/reports.md`, RPT-1–36, ZR-1–4). The user's answers (Q1–Q4) are built and
every bug found is fixed.

**Architecture:** Same layers as areas 1–6, shaped for read-only code. Reports store nothing, so the **auditor**
layer becomes an **agreement check** (`internal/battle/reports_agree_test.go`: `agree(t, shop, from, to)`). It calls
the real report endpoints and compares them with each other (RPT-30–34) and with plain sums over the raw rows
(RPT-1, 7, 9, 16, 17, 20–25). Every machine runs it after each day and over the whole run. Then case tests per rule,
the fixes, faults (a report read while sales land, RPT-36), and mutation on `internal/features/reports`. **No Lean:**
the only new maths is a refund's tax share, checked by cases.

**Spec:** `docs/specs/2026-11-18-core-battle-test-design.md`

**Session split (user, 2026-11-19):** desk work (Task 1) on day one; Tasks 2–8 the next day, one long run at a time.

## Global Constraints

- As areas 1–6: core only; every check and test names its rule; a rule changes only with the user's yes; a bug = a
  failing test first, then the fix, one commit each (`fix(reports): … (RPT-6)`).
- Reports stay read-only. RPT-36: each report reads inside one `REPEATABLE READ, READ ONLY` transaction (`Service`
  already takes a `Queryer`, so a `*sqlx.Tx` drops in).
- One SQL source per idea, shared by every report: **sold in period** (sales by `created_at`) and **returned in
  period** (`sale_returns.created_at`, refund before tax, cost back). Today `KeptRevenueSQL`, `ProfitAggSQL`,
  `PeriodSummary` and `Compute` each count returns their own way; after Task 4 they all use the shared pieces.
- Migration **00041** only for ZR-2: `sale_returns.session_id` (the returning cashier's open till, set as DRAW-6 sets
  a sale's; old returns are back-filled from the cash refund move when there is one, else the session open at that
  time, else NULL).
- `make test` grows by at most ~30 s; long runs detached, logs in `~/.cache/karots-logs/`. Never two long runs at once.

## Review Focus (suspected from reading the code, 2026-11-20)

- **RPT-6 (Q1):** `Compute`, `SalesByPeriod`, `PeakHours`, `KeptRevenueSQL` (top products, categories, services,
  product graph, product totals) and `ProductQtySold` (reorder demand: keep it on kept units, as forecasting wants)
  read `sale_items.returned_qty` on the sale's day. They should count `sale_return_items` on the return's day, at
  `refund_amount` × the sale's pre-tax share.
- **RPT-14:** `SalesByCashier` net = `total − tax − pass-through` with no returns at all.
- **ZR-2 (Q2):** `PeriodSummary` returns = `returned_qty` on the session's sales, at any time.
- **RPT-12 (Q3):** `preTaxFactorSQL`'s denominator holds pass-through positive lines, so they take a share of the
  bill discount that line reports then drop. Fix: the denominator and numerator use the shop's own lines only, with
  the pass-through face taken out first.
- **RPT-17 (Q4):** `TaxSummary` has no returns; add given back (refund × tax ÷ (total − payouts)) and owed.
- **RPT-4:** zero-cost revenue counts pass-through lines and returned units.
- **RPT-36:** `Compute` runs ~20 queries outside a transaction (and swallows errors on 10 of them with `_ =`: errors
  must surface, RPT-1).
- **RPT-30 (found in Task 2):** the Sales report (`sales.Summarize`, the sales list's totals) counts revenue with
  tax in (`total − pass-through − returns`) and counts void sales (no status filter by default).
- **RPT-21:** supplier spend's `paid` = `paid_amount` now, not paid in the period: fine as labelled, a case pins it.

---

### Task 1: The rule book for area 7

- [x] **Step 1:** Grid + rules in `docs/rules/reports.md` (RPT, ZR).
- [x] **Step 2:** Q1–Q4 in `docs/rules/questions.md`; README links area 7 and the RPT / ZR ids.
- [x] **Step 3:** Asked; all four answered 2026-11-20 as suggested (a return on its own day; a Z = refunds given in
  its session, never changes; line reports share a bill discount over the shop's own lines; tax given back + owed).
- [x] **Step 4: Commit** — `docs(rules): area 7 rule book — reports and the Z report; user's answers`

### Task 2: The agreement check (`internal/battle/reports_agree_test.go`)

- [x] **Step 1:** `agree(t, shop, from, to)`: GET the P&L, sales by period (day / week / month), peak hours, top
  products (limit 200) + product totals, categories, services, cashiers, returns, tax, expenses, losses, dues (both),
  supplier spend, purchases, stock value, the dashboard (when the range is today), and the Z of every session in
  range. Assert RPT-30–34 between them and RPT-1, 7, 9, 16, 17, 20–25 against plain SQL over the raw rows (written
  fresh in the test, never the report's own SQL constants).
- [x] **Step 2:** (`ledger_reports_test.go`; skipped until Task 4 unless `KS_AREA7=1`. Found: RPT-6, 12, 14, 17,
  ZR-2 as expected; the Sales report has tax in its revenue and counts void sales (RPT-30); RPT-32 worded "on account
  left out" — the by-method list shows on account, which isn't received, RPT-7.) A fixed shop day that touches every grid row (sale with item + bill discount, pack, frozen, service,
  payout, pass-through + bill discount, tax shop, void, part / whole return the next day, every tender, credit,
  expense, damage, recovery + undo, count, delivery, supplier pay with discount, interest, over / short) → run
  `agree` over the sale day, the return day and both. Expected to FAIL today on RPT-6, 12, 14, 17, ZR-2: record which,
  each is a Task 4 fix.
- [ ] **Step 3:** (moved after Task 4: before the fixes every month fails RPT-6.) Run over a Demo Room year (gen test) per month and whole: every failure is a bug (→ Task 4), a
  wrong rule (→ the user) or seed data.
- [ ] **Step 4: Commit** — `test(battle): area 7 agreement check — reports against each other and the raw rows`

### Task 3: Case tests, one per `example` rule (`internal/battle/cases_reports_test.go`)

- [ ] **Step 1:** RPT-4 (pass-through and fully returned lines not "no cost"), RPT-5 (void nowhere), RPT-6 (return
  next month: last month unchanged, this month −refund; whole return nets 0 over both), RPT-10/11 (23:59 and 00:01
  shop time; Monday week; `to` whole day; a day with no sales is 0), RPT-12 (pass-through + 10% bill discount →
  product totals = P&L), RPT-13 (service with expenses only), RPT-14 (cashier A sells, B refunds), RPT-17 (tax back
  on a return, owed), RPT-20/21 (disabled debtor listed; draft not a purchase), RPT-24 (lot cost vs product cost, no
  lot), ZR-1–4 (a Z closed, then a return of its sale in the next session: the first Z unchanged, the second shows the
  refund; a cashier can't read another's Z), RPT-35 (sheet rows = page rows past one page).
- [ ] **Step 2:** Run; each failure → Task 4 (test first is already there).
- [ ] **Step 3: Commit** — `test(battle): area 7 cases, one per rule`

### Task 4: Fixes, one commit each, test first

- [x] **Step 0:** Remove the `KS_AREA7` skip in `ledger_reports_test.go`; each step below turns its failures green.
- [x] **Step 1:** Shared SQL pieces (`reports/flow.go`: `flowSQL` = the period's sold own lines + returns given in
  it, `soldSQL` / `returnedSQL` for time and people reports, `ownFactorSQL` — the bill discount over the shop's own
  lines, Q3; `snapshot`). `KeptRevenueSQL` stays for the documents / repairs plugins (checked in the plugin phase).
- [x] **Step 2:** RPT-6 — `Compute`, `SalesByPeriod`, `PeakHours` (a return in its hour), `TopProducts`,
  `ProductTotals`, `ProfitByCategory`, `ServiceProfit`, `ProductSalesByPeriod` on the shared pieces.
- [x] **Step 3:** RPT-14 — cashiers' net less the refunds each gave.
- [x] **Step 4:** ZR-2 — migration 00041 `sale_returns.session_id` (set by the return beside its cash move; back-fill);
  `PeriodSummary` returns by session; the Z print page and `ZReport` DTO (`make types`) say "Refunds given".
- [x] **Step 5:** RPT-17 — `TaxSummary` given back + owed; the Tax page and its sheet.
- [x] **Step 6:** RPT-37 (new rule: the Sales report and sales list are the period's receipts, so totals match rows)
  — revenue before tax in rows and totals, void out of the totals unless filtered for.
- [x] **Step 7:** RPT-4 (on the flow: own goods only, returns off on their day), RPT-36 for the P&L (`snapshot`; the
  10 swallowed errors now fail it). The dashboard and the Z still read in several snapshots — Task 6 tests them first.
- [ ] **Step 8:** Screens: Reports → P&L caption ("returns on the day they were made"), Tax, Z; check at 360 px and
  in two palettes; guide `docs/guide/reports.md` + the till's Z in `docs/guide/till.md`; `make docs-shots` for the
  shots that changed.

### Task 5: Machines run the agreement check

- [ ] **Step 1:** Every random-days machine (sales, money, customers, stock, purchasing + recovery) calls `agree`
  after each day and over the whole run; the sales machine's model also keeps its own revenue / cost / tax / refunds
  per day and per cashier, so RPT-1 is checked against the model too, not only SQL.
- [ ] **Step 2:** Short in `make test`, long in `make battle`. Commit.

### Task 6: Faults

- [ ] **Step 1:** RPT-36 — a P&L, dashboard and Z read in a loop while two tills sell and return: each answer agrees
  with itself (Σ by method = received, Σ periods = revenue). A power cut mid-return: no report counts half a return.
- [ ] **Step 2: Commit** — `test(battle): area 7 faults — a report is one moment`

### Task 7: Mutation

- [ ] **Step 1:** `make mutate` on `internal/features/reports` and `sales/service_summary.go`; replay LIVED / NOT
  COVERED against the area's battle tests (`scripts/replay-mutants.py`, absolute testdb path); kill or explain each.
- [ ] **Step 2: Commit** the tests that kill them.

### Task 8: Close the area

- [ ] **Step 1:** `make battle` (alone; read the log for FAIL — it exits 0), then the full run (`scripts/full-run.sh`);
  rerun timed-out Playwright specs alone.
- [ ] **Step 2:** Coverage map updated (https://claude.ai/artifact/GSvAu11weLXS7PjRD4G8u9, four JSON datasets).
- [ ] **Step 3:** `docs/status.md`: area 7 closed, what was found; next = the coverage push, then plugins simplest
  first. Delete this plan. Commit.
