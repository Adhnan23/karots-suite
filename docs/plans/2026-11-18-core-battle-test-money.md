# Core battle test — area 2: drawer, cash flow, lockers, expenses — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every rule of the drawer, money moves between places, lockers, expenses and the Cash flow page written
down (`docs/rules/money.md`), checked by machine, and every bug found fixed — on the tools area 1 built.

**Architecture:** Same layers as area 1: rule book grid → auditor checks (`internal/ledgercheck/money.go`) → case
tests per rule (`internal/battle/cases_money_test.go`) → the random-days machine grows money actions and keeps its own
books for every drawer and locker → faults (two cash-outs at once, a sale during a close, two closes) → mutation.
**No Lean this area:** the drawer and cash flow are sums, nothing to prove past what the auditor and machine check.

**Spec:** `docs/specs/2026-11-18-core-battle-test-design.md`

## Global Constraints

- As area 1: core only; ledgercheck read-only; every check and test names its rule; a rule changes only with the
  user's yes; a bug = failing test first, then the fix, one commit each (`fix(money): … (DRAW-16)`).
- `make test` grows by at most ~30 s; long runs in `make battle`, detached, logs in `~/.cache/karots-logs/`.
- Never run DB tests on `suite_db` while Playwright runs.

## Review Focus (suspected from reading the code, 2026-11-18)

- **DRAW-16:** `cashregister.adjust` (Withdraw) checks what the drawer holds **outside** its transaction and without
  locking the session — two withdrawals at once can both pass. `cashflow.MoveTx` locks; `adjust` doesn't.
- **DRAW-7:** a sale doesn't lock the drawer session, and its `created_at` is its transaction's start: a sale paid
  while an admin closes the till can be timed inside the session yet missed by the close.
- **DRAW-6 (Q2):** the server sells with no open session; that cash is in no drawer.
- **Dead code:** `cashregister.RecordCreditCash / RecordRefundCash / RecordSupplierCash` have no callers (everything
  goes through `cashflow.MoveTx`) — delete.

---

### Task 1: The rule book for area 2

- [x] **Step 1:** Grid + rules in `docs/rules/money.md` (DRAW, LOCK, EXP, CF; past `fix(` commits as rules).
- [x] **Step 2:** Q1–Q3 in `docs/rules/questions.md`; README links area 2; DRAW-1…4 moved from `returns.md`.
- [x] **Step 3: Commit** — `docs(rules): area 2 rule book — drawer, lockers, expenses, cash flow; questions Q1–Q3`
- [x] **Step 4:** Asked; all three answered 2026-11-18 (DRAW-3, DRAW-6, CF-2 — each changes behaviour, built in Task 3).

### Task 2: Auditor checks (`internal/ledgercheck/money.go`)

- [x] **Step 1: Failing test** `internal/battle/ledger_money_test.go` (the API builds the day): a clean day (open from a locker, cash sale,
  withdraw to a locker, expense from the till, transfer, settle card, banked close, a voided expense) → no
  violations; then corrupt one row at a time and assert that rule's id: DRAW-1 (stored expected off by 0.01;
  difference ≠ counted − expected), DRAW-9 (opening record ≠ opening cash; banked close without its locker row),
  DRAW-10 / LOCK-3 (a till↔locker row without its drawer twin), LOCK-1 (a no-negative locker below zero), LOCK-2 (a
  transfer half), LOCK-6 (a card account row that isn't a sale, payment or settle), LOCK-7 / LOCK-8 (a bank charge
  without its expense), LOCK-12 (a card payment without its account row), EXP-1 (a paid expense whose money ≠ amount −
  unpaid), EXP-2 (a void that put back less).
- [x] **Step 2: Run, fail; write the checks; run, pass.** LOCK-1 left to the machine (cleared cheques may overdraw).
- [x] **Step 3:** (2026-11-18: grocery year clean; dev DB: no area 2 violations) Run `Run` over the Demo Room year (`ledgercheck` gen test) and the dev DB: every violation is a bug
  (→ Task 3 test) or a wrong rule (→ the user).
- [x] **Step 4: Commit** — `test(ledgercheck): area 2 checks — drawer, lockers, expenses`

### Task 3: Case tests, one per `example` rule (`internal/battle/cases_money_test.go`)

- [x] **Step 1:** Delete the dead `Record*` helpers (commit `refactor(cashregister): …`).
- [x] **Step 2:** (all pass; older tests mapped in money.md) One test per example rule: DRAW-5 (open twice), DRAW-8 (close for them books as the cashier's,
  records who), DRAW-11, DRAW-12, DRAW-13 (bank / card / Cheques in hand refused for every till cash path), DRAW-14,
  DRAW-15, DRAW-17, LOCK-5, LOCK-6 (transfer / expense / void into a card account refused), LOCK-7, LOCK-8, LOCK-9,
  LOCK-10, LOCK-11, EXP-2 (unpaid void, pending cheque, plugin-owned refused), EXP-3, EXP-4, CF-3. Each ends with the
  auditor.
- [x] **Step 3:** Run; each failure → fix test-first, one commit each. (none failed)
- [x] **Step 3b: The user's answers (d3e892f, 47484da, 457960c; CF-2 also counts a float from nowhere as money in), test-first, one commit each:** DRAW-3 (a cash refund beyond the drawer goes
  through; change / payout / refund taking the expected below zero flags the session's `review_note`), DRAW-6 (a cash
  sale with no open session: booked to the session open at its `rung_at`, else the cashier's next one; both flagged
  with the sale number — needs a sale → session link, likely `sales.session_id`, migration 00038), CF-2 (an untracked
  close / withdrawal shows as money out on the Cash flow page).
- [x] **Step 4: Commit** — `test(battle): area 2 cases — one per rule`

### Task 4: Random days with money (`internal/battle/machine_test.go` + `machine_money_test.go`)

- [x] **Step 1:** (`machine_money_test.go`; found: over / short missing from Cash flow) The machine keeps books for both tills and every locker (a cash safe, a no-negative safe, a bank, a
  card account). New actions: open (float from a locker / none), withdraw / pay-in (locker / none), expense from the
  till / a locker / outside, void an expense (back to a till / locker), transfer, bank charge, interest, settle card,
  close (banked / counted only, the right amount or over / short), close for them, card / online sales. After every
  step: auditor, both drawers' expected = the books (DRAW-1), each locker's balance = the books (LOCK-1), a refused
  move changes nothing, Cash flow page totals for the day agree (CF-1, CF-2, CF-4) and receipts = moves (LOCK-4).
- [x] **Step 2:** (3,000-step run clean) Short run in `make test`; `make battle` long, detached, until 20,000 steps clean.
- [x] **Step 3: Commit** — `test(battle): random shop days move money between tills and lockers`

### Task 5: Faults (`internal/battle/faults_money_test.go`)

- [x] **Step 1:** (DRAW-16 was real: 8 at once all passed, drawer −3,000 — fixed) DRAW-16: two withdrawals at once that together pass the drawer (and a refund + a withdrawal) — one
  must be refused. LOCK-1: two transfers out of one no-negative locker at once.
- [x] **Step 2:** DRAW-7: a sale paid while an admin closes that till — afterwards the close's expected = the
  auditor's (the sale is in it or after it). DRAW-8: the cashier and an admin close at the same moment — one close,
  cash banked once.
- [x] **Step 3:** (found: a sale with no till committing after the next open — per-cashier lock) Power cut mid-move (the real binary killed during withdrawals / closes / expenses): each move whole
  or absent, the auditor clean.
- [x] **Step 4:** Each failure → fix test-first, one commit each; then `test(battle): area 2 faults`.

### Task 6: Mutation

- [ ] **Step 1:** `make mutate` on `internal/features/cashregister`, `cashflow`, `expenses`, `payout`, `lockers`;
  each surviving mutant in money code gets the test that kills it.
- [ ] **Step 2: Commit** — `test: area 2 mutants killed`

### Task 7: Close area 2

- [ ] **Step 1:** Q1–Q3's tests passing (Task 3b); `docs/guide/money.md` / `till.md` say what changed.
- [ ] **Step 2:** Full run (`make test-all COVER=1`), one clean `make battle`.
- [ ] **Step 3:** Coverage map updated (re-upload its four JSON datasets); `docs/status.md` — area 2 done, bugs found
  and fixed; `docs/guide/money.md` if a behaviour changed.
- [ ] **Step 4:** Fresh review of the area; fix findings test-first.
- [ ] **Step 5:** Commit — `docs(status): battle test area 2 done`; delete this plan. Area 3's plan replaces it.
