# Core battle test — area 6: recovery and losses (warranty replacements, what a supplier makes good) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every rule of a loss after it leaves the shelf — warranty replacements (serial and by receipt) and what a
supplier does about a loss (a replacement, a credit, nothing), with its undo — written down (`docs/rules/recovery.md`,
WAR-1–6, REC-1–8), checked by machine, and every bug found fixed.

**Architecture:** Same layers as areas 1–5: rule book → auditor checks (`internal/ledgercheck/recovery.go`) → case
tests per rule (`internal/battle/cases_recovery_test.go`) → the purchasing machine grows losses and recoveries → faults
→ mutation. **No Lean this area:** the only maths is loss ÷ units (4 dp) and a blended lot cost, checked by cases.

**Spec:** `docs/specs/2026-11-18-core-battle-test-design.md`

**Session split (user, 2026-11-19):** desk work (Task 1, Task 2) on day one; Tasks 3–7 the next day, one long run at
a time.

## Global Constraints

- As areas 1–5: core only; ledgercheck read-only; every check and test names its rule; a rule changes only with the
  user's yes; a bug = failing test first, then the fix, one commit each (`fix(warranty): … (WAR-6)`).
- Lock order: the loss's row (damage movement / warranty unit / the sale for a by-receipt line), then stock rows
  (`stock.LockRows`, product-id order), then lots, then the supplier **last** (`suppliers.LockByID`, FOR NO KEY UPDATE).
  Every new lock gets an "at once" test beside a sale of the same product.
- Migration **00040** only if Task 3 needs it (REC-3's kept split, REC-8's lot cost before); old recoveries keep
  working (nothing to undo for them is fine: "recorded before undo existed").
- `make test` grows by at most ~30 s; long runs detached, logs in `~/.cache/karots-logs/`. Never two long runs at once.

## Review Focus (suspected from reading the code, 2026-11-20)

- **WAR-1 (Q5):** `RecordReplacement` checks only `status = 'replaced'`; a returned unit or one past its last day is
  swapped if the request skips the screen (`UnitClaim.tsx` hides Swap). Fix: refuse unless `Unit.UnderWarranty()`.
- **WAR-4:** `nonSerialLinesSQL` sums `sale_items.quantity` and never takes off returned units; `RecordReceiptReplacement`
  takes a `saleID` straight from the request and never checks the sale isn't void.
- **WAR-6:** `FindUnitByID` reads the unit with no lock; by receipt, nothing holds the sale line. Two swaps at once →
  two units out for one fault. Fix: `FOR UPDATE` on the unit; lock the sale row (`FOR NO KEY UPDATE`) for by-receipt.
- **REC-2 (Q3):** `AddStock` with `BatchID > 0` values the top-up at the lot's cost, ignoring loss ÷ units + extra;
  the `recovery` movement has `reference_type = 'recovery'` but no `reference_id`. Fix: blend the lot's cost; insert
  the recovery first and pass its id.
- **REC-3 (Q2):** no check the supplier is active for an extra charge; the form doesn't fill the lot's supplier
  (`RecoverSheet.tsx`, `claim.tsx`). `creditBills` doesn't keep what went on each bill (needed for REC-8 and an
  auditor check) — keep it (table `loss_recovery_bills` or a column; decide in Task 3).
- **REC-7:** `Record` takes the damage movement, then `AddStock` (stock row, lot), then `AddBalance` (supplier, an
  UPDATE without `LockByID`), and `creditBills` reads open bills after — check against a supplier payment and a
  delivery at once; a replacement beside a sale of the same product.
- **REC-8 (Q4):** new: `DELETE /recovery/:id` (admin + the counter's supplier group), service `Undo`; a button on
  Losses & Recovery and the supplier's claims (settled rows); guide `docs/guide/reports.md` (+ the counter's supplier
  page if shown there); `make docs-shots` if a shot shows it.

---

### Task 1: The rule book for area 6

- [x] **Step 1:** Grid + rules in `docs/rules/recovery.md` (WAR, REC).
- [x] **Step 2:** Q1–Q5 in `docs/rules/questions.md`; README links area 6 and the WAR / REC ids.
- [x] **Step 3:** Asked; all five answered 2026-11-20 as suggested (REC-3 any amount, the lot's supplier filled in,
  disabled: credit yes / extra charge no; REC-2 a top-up blends the lot's cost; REC-8 undo while nothing came after;
  WAR-1 refuse out-of-date and returned units) — built in Task 3 Step 3.
- [x] **Step 4: Commit** — `docs(rules): area 6 rule book — warranty replacements and recoveries; user's answers`

### Task 2: Auditor checks (`internal/ledgercheck/recovery.go`)

- [x] **Step 1: Failing test** `internal/battle/ledger_recovery_test.go` (the API builds the day): a serial unit sold
  and swapped, a by-receipt swap,  recoveries
  of each outcome (replacement into a new lot with an extra charge, paid on open bills with some left over, written
  off) → no violations; then corrupt one row at a time and assert that rule's id: REC-1 (a second recovery of one
  loss; loss worth ≠ the movement's cost), REC-5 (written off with recovered > 0; paid with no supplier; unapplied
  above the amount; a replacement recovered ≠ its loss), WAR-3 (a claim with no movement / two; movement cost ≠ the
  loss), WAR-2 (replacement unit's last day ≠ the old one's), WAR-4 (by-receipt replaced > sold − returned).
- [x] **Step 2: Run, fail; write the checks; run, pass.**
- [x] **Step 3:** (grocery year clean, 89 s; dev DB: no area-6 violations) Run over a Demo Room year (`ledgercheck` gen test) and the dev DB: every violation is a bug (→ Task 3
  test), a wrong rule (→ the user) or seed data (fix the seeder).
- [x] **Step 4: Commit** — `test(ledgercheck): area 6 checks — recoveries and warranty replacements`

### Task 3: Case tests, one per `example` rule (`internal/battle/cases_recovery_test.go`)

- [ ] **Step 1:** One test per example rule: WAR-1 (returned unit refused; out-of-date refused, last day allowed;
  already replaced refused), WAR-2 (serial used anywhere refused; last day and customer carried), WAR-3 (FEFO lot cost;
  no stock refused, nothing saved; frozen: movement 0, cost booked), WAR-4 (sold 3, returned 1 → 2 swaps, the third
  refused; void sale refused; two lines of one product summed), WAR-5 (no sale, drawer or customer change), REC-1
  (second recovery refused; unreplaced unit refused), REC-2 (new lot at loss ÷ units + extra; top-up blends; another
  product's lot refused; whole units; new price moves the product), REC-3 (credit on bills oldest first, old debt,
  the rest waits and settles the next bill; any amount; the lot's supplier offered; disabled: credit yes, extra charge
  refused), REC-4, REC-6 (another supplier's lot / settled loss never listed; give back of another's lot refused),
  REC-8 (undo each outcome back to exactly before; refused after a sale of the units / the credit spent). Each ends
  with the auditor.
- [ ] **Step 2:** Run; each failure → fix test-first, one commit each.
- [ ] **Step 3: The user's answers** (Q1–Q5), test-first, one commit each: WAR-1 server refuses; REC-2 blended top-up
  + the movement names its recovery; REC-3 the lot's supplier filled in on the form + disabled extra refused + the
  kept split (migration 00040 if needed); REC-8 undo (service, route, button, guide).
- [ ] **Step 4: Commit** — `test(battle): area 6 cases — one per rule`

### Task 4: Random days with losses (`internal/battle/machine_purchasing_test.go` or `machine_recovery_test.go`)

- [ ] **Step 1:** The purchasing machine (it keeps every supplier's books) gains: write-off of a supplier's lot, give
  back, a serial sale + swap, a by-receipt swap, a recovery of a waiting loss (each outcome, an extra charge), an undo.
  After every step: the auditor, each balance and bill = the books (SUP-1, REC-3), each lot = the books, the claims
  list = the machine's waiting losses, a refused action changes nothing.
- [ ] **Step 2:** Short run in `make test`; `make battle` long, detached, until 20,000 steps clean.
- [ ] **Step 3: Commit** — `test(battle): random shop days with losses and what suppliers make good`

### Task 5: Faults (`internal/battle/faults_recovery_test.go`)

- [ ] **Step 1:** WAR-6: two swaps of one unit, two by-receipt swaps of a line's last unit — one each. REC-1: eight
  recoveries of one loss at once — one. REC-7: a recovery (replacement, credit) beside a sale of the product, a
  supplier payment, a delivery settling from credit, a refund — books hold, nothing refused that should pass (never
  stop a sale). REC-8: two undos; an undo beside a sale of the units. Power cuts: a recovery worker in the power-cut
  runs (each recovery whole or absent).
- [ ] **Step 2:** Fix what they find, test-first, one commit each.
- [ ] **Step 3: Commit** — `test(battle): area 6 faults`

### Task 6: Mutation

- [ ] **Step 1:** `make mutate` on `internal/features/recovery` and `internal/features/warranty`; replay LIVED / NOT
  COVERED against the battle tests in a scratch copy (`scripts/replay-mutants.py`, absolute testdb path), record which
  test kills each.
- [ ] **Step 2:** Close the money gaps (a test per survivor that matters, naming its rule); list the rest.
- [ ] **Step 3: Commit** — `test: area 6 mutants killed — …`

### Task 7: Close the area

- [ ] **Step 1:** `make battle` (20,000 steps clean), then the full run (`scripts/full-run.sh`), one at a time;
  Playwright timeouts beside Go's coverage run rerun alone before calling them failures.
- [ ] **Step 2:** Guides (`docs/guide/reports.md` Losses & Recovery: undo, the lot's supplier; the warranty guide:
  refused swaps), screenshots if a screen changed (`make docs-shots`).
- [ ] **Step 3:** Coverage map (https://claude.ai/artifact/GSvAu11weLXS7PjRD4G8u9): re-upload its four JSON datasets.
- [ ] **Step 4:** `docs/status.md` (area 6 closed: what was found and fixed); delete this plan. Commit.
