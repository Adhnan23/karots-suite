# Core battle test — area 3: customers (credit, advances, opening balances, payments) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every rule of a customer's balance — opening, sales on account, kept change, returns, payments, advances,
bounced cheques — written down (`docs/rules/customers.md`), checked by machine, and every bug found fixed.

**Architecture:** Same layers as areas 1 and 2: rule book → auditor checks (`internal/ledgercheck/customers.go`) →
case tests per rule (`internal/battle/cases_customers_test.go`) → the random-days machine grows customer actions and
keeps its own books per customer → faults (two payments / refunds / credit sales at once, power cuts) → mutation.
**No Lean this area:** a balance is a sum; the auditor (CUST-1) and the machine check it.

**Spec:** `docs/specs/2026-11-18-core-battle-test-design.md`

## Global Constraints

- As areas 1–2: core only; ledgercheck read-only; every check and test names its rule; a rule changes only with the
  user's yes; a bug = failing test first, then the fix, one commit each (`fix(customers): … (CUST-11)`).
- `make test` grows by at most ~30 s; long runs in `make battle`, detached, logs in `~/.cache/karots-logs/`.
- Never run DB tests on `suite_db` while Playwright runs.

## Review Focus (suspected from reading the code, 2026-11-19)

- **CUST-11:** `RefundAdvanceTx` reads the balance with no row lock — two refunds at once can each pass and hand back
  twice the advance.
- **CUST-6 / CUST-13 / CUST-9:** `RecordPaymentTx` reads the customer unlocked: two payments at once print the same
  "before"; an old-debt-first payment works its split from a stale old debt, so `PayOpening` clamps and the balance
  drops by less than was paid (CUST-1).
- **CUST-8 / TEND-12:** the sale reads the customer unlocked: two sales on account at once both pass the limit
  unflagged, and `customer_balance_after` on the receipt is wrong. One fix: `customers.Repository.LockByID`
  (`SELECT … FOR UPDATE`) used by sales, payments and refunds.
- **CUST-10:** `Statement` never lists kept change (`rounding_to_account`), so its running balance ends off the real one.
- **CUST-3:** `Update` and `Reactivate` don't check the phone: two active customers can share one.
- **Q1:** `AdjustOpening` can set an old debt larger than the balance; `AddBalance`'s clamp then moves it on the next
  payment.

---

### Task 1: The rule book for area 3

- [x] **Step 1:** Grid + rules in `docs/rules/customers.md` (CUST; past `fix(` commits as rules).
- [x] **Step 2:** Q1–Q3 in `docs/rules/questions.md`; README links area 3.
- [x] **Step 3: Commit** — `docs(rules): area 3 rule book — customers' balances, payments, advances; questions Q1–Q3`
- [x] **Step 4:** Asked; all three answered 2026-11-19 as suggested (CUST-15, CUST-16, CUST-17 — built in Task 3 Step 3).

### Task 2: Auditor checks (`internal/ledgercheck/customers.go`)

- [x] **Step 1: Failing test** `internal/battle/ledger_customers_test.go` (the API builds the day): a customer made with
  an opening, a sale on account, kept change, a return on the credit sale, cash / card / cheque payments, old debt
  first, an overpayment, an advance refund, a bounced cheque, an opening adjusted → no violations; then corrupt one
  row at a time and assert that rule's id: CUST-1 (balance off by 0.01; a payment row with its amount changed),
  CUST-3 (a second active customer with the same phone, spaced differently), CUST-5 (a cash / card payment without its
  money move; a cheque payment without its cheque), CUST-6 (after ≠ before − amount; DP- numbers are unique in the database), CUST-9 (old debt above the opening), CUST-15 (old debt above the balance).
- [x] **Step 2: Run, fail; write the checks; run, pass.**
- [x] **Step 3:** (2026-11-19: grocery year clean; dev DB: no area 3 violations — only seed stock's SALE-20, area 4) Run `Run` over a Demo Room year (`ledgercheck` gen test) and the dev DB: every violation is a bug
  (→ Task 3 test) or a wrong rule (→ the user).
- [x] **Step 4: Commit** — `test(ledgercheck): area 3 checks — customers' balances and payments`

### Task 3: Case tests, one per `example` rule (`internal/battle/cases_customers_test.go`)

- [x] **Step 1:** (CUST-2: the back office refuses a phone in use, the till reuses — rule worded so; a sheet's opening is held by `import_dedup_test`) One test per example rule: CUST-2 (reuse by phone and a re-import add no opening; a negative opening
  refused), CUST-3 (phone changed to one in use / bringing back a customer whose phone is taken — refused), CUST-4
  (adjust up, down, below zero; linked unchanged; audit row), CUST-5 (card with no account, cash into a bank refused;
  each method's money where it belongs), CUST-6 (slip numbers and before / after), CUST-7 (overpay → advance → the
  next sale on account shows "from advance"), CUST-8 (limit lowered below the balance: balance unchanged, the next
  sale on account refused, a Review sale flagged), CUST-9 (old debt first, plain payment, overflow into an advance),
  CUST-10 (statement ends at the balance after every kind of line), CUST-11 (more than the advance refused; from a
  till beyond the drawer refused), CUST-12 (dues and advances totals, a disabled customer who owes), CUST-14 (disabled
  customer pays and is refunded). Each ends with the auditor.
- [x] **Step 2:** (real: CUST-3 phone clash 561e399, CUST-10 statement missed kept change 59fe908) Run; each failure → fix test-first, one commit each.
- [x] **Step 3: The user's answers** (CUST-15 ae56d64, CUST-17 c30a601, CUST-16 05136c1 — after Task 5 Step 1's lock), test-first, one commit each: CUST-15 (`AdjustOpening` caps the old debt at
  the balance, never below zero unless set negative with no advance), CUST-16 (`clientRef` on counter + admin
  payments, unique per customer — likely migration 00039 `customer_payments.client_ref`; the counter screen sends
  one per Pay), CUST-17 (a sale on account to a disabled customer gets a review flag).
- [x] **Step 4: Commit** — `test(battle): area 3 cases — one per rule`

### Task 4: Random days with customers (`internal/battle/machine_customers_test.go`)

- [x] **Step 1:** (`machine_customers_test.go`; the drawer and every statement checked too) The machine keeps books for three customers (one with an opening, one with a low limit, one
  disabled mid-run). Actions: sale on account (part / whole, over the limit by confirm, Review), kept change to
  account, return of a credit sale, payment by cash / card / cheque / old debt first / more than owed, advance refund
  from the till or a locker, bounce a cheque, adjust the opening, change the limit, disable / bring back. After every
  step: the auditor, each balance = the books (CUST-1), each old debt = the books (CUST-9), the drawer = the books
  (DRAW-1), each statement ends at the balance (CUST-10), a refused action changes nothing.
- [x] **Step 2:** (3,000-step run clean, 197 s) Short run in `make test`; `make battle` long, detached, until 20,000 steps clean.
- [x] **Step 3: Commit** — `test(battle): random shop days with customers paying, borrowing and getting advances back`

### Task 5: Faults (`internal/battle/faults_customers_test.go`)

- [x] **Step 1:** (done before CUST-16, which needs the lock; all three real — 8 refunds of a Rs 300 advance all went through, payments at once lost Rs 100 off the balance, two tills passed the limit by Rs 200 unflagged; fixed by `LockByID`, 29a532b; + the same Pay sent 4× at once is one) CUST-11: two advance refunds at once that together pass the advance — one refused. CUST-13 / CUST-6:
  several payments at once (plain and old debt first) — balance drops by all, every slip's before / after chain right.
  CUST-8: two sales on account at once that together pass the limit — one refused (or flagged on Review); each
  receipt's balance after right (TEND-12).
- [x] **Step 2:** (clean; found the server crashing after its first backup when BACKUP_INTERVAL=0 — 6ef249f) Power cut mid-payment / mid-refund (the real binary killed): each whole or absent, the auditor clean.
- [x] **Step 3:** (+ two returns at once cleared one debt twice, RET-19 — cf14231) Each failure → fix test-first, one commit each; then `test(battle): area 3 faults`.

### Task 6: Mutation

- [ ] **Step 1:** `make mutate` on `internal/features/customers` (+ the on-account parts of `sales`); survivors replayed
  against the battle tests; each surviving mutant in money code gets the test that kills it.
- [ ] **Step 2: Commit** — `test: area 3 mutants killed`

### Task 7: Close area 3

- [ ] **Step 1:** Q1–Q3's tests passing; `docs/guide/people.md` says what changed. `docs/status.md` gets the user's
  later idea (2026-11-19): a note box on the till's plain Hold (only Send to cashier has one) — after all areas pass.
- [ ] **Step 2:** Full run (`scripts/full-run.sh`, `COVER=1`), one clean `make battle`.
- [ ] **Step 3:** Coverage map updated (re-upload its four JSON datasets); `docs/status.md` — area 3 done, bugs found
  and fixed.
- [ ] **Step 4:** Fresh review of the area; fix findings test-first.
- [ ] **Step 5:** Commit — `docs(status): battle test area 3 done`; delete this plan. Area 4's plan replaces it.
