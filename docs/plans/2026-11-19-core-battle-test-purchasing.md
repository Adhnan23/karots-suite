# Core battle test — area 5: purchasing (orders, deliveries, supplier returns, supplier pay, credits, cheques) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every rule of buying from suppliers — orders, deliveries and their bills, lot cost, goods sent back,
payments, advances, discounts, refunds, settling from credit, cheques given and received — written down
(`docs/rules/purchasing.md`), checked by machine, and every bug found fixed.

**Architecture:** Same layers as areas 1–4: rule book → auditor checks (`internal/ledgercheck/purchasing.go`) → case
tests per rule (`internal/battle/cases_purchasing_test.go`) → the random-days machine grows supplier actions and keeps
its own books per supplier and bill → faults (two money actions on one supplier, two receives of one order, power
cuts) → mutation. **No Lean this area:** the bill-discount share is the same largest-remainder / pro-rata maths
`Share.lean` already proves in spirit; PUR-3's "lots worth the total within a cent a line" is checked by cases and the
machine.

**Spec:** `docs/specs/2026-11-18-core-battle-test-design.md`

**Session split (user, 2026-11-19):** desk work (Task 1, Task 2) on day one; Tasks 3–7 (cases, machine, faults,
mutation, `make battle`, full run) the next day, one long run at a time.

## Global Constraints

- As areas 1–4: core only; ledgercheck read-only; every check and test names its rule; a rule changes only with the
  user's yes; a bug = failing test first, then the fix, one commit each (`fix(purchases): … (PUR-6)`).
- Lock order (area 4 lesson): the till, then stock rows, then lots, then the customer — **the supplier row goes last**
  (a delivery takes stock and lots before it touches the supplier; a payment from a till takes the till first). Every
  new lock gets an "at once" test beside a sale.
- `make test` grows by at most ~30 s; long runs in `make battle`, detached, logs in `~/.cache/karots-logs/`.
- Never run DB tests on `suite_db` while Playwright runs; never two long runs at once.

## Review Focus (suspected from reading the code, 2026-11-19)

- **PUR-6:** `ReceiveTx` reads the order with no row lock (`FindByID`), checks `status = 'draft'`, then deletes and
  re-inserts lines: two receives of one order at once can both pass → stock and the balance booked twice. Fix: lock
  the order row (`FOR UPDATE`) before the check.
- **SUP-7 / SUP-8:** `RefundTx` and `PayTx` read the supplier (`FindByID`) without holding it; the refund cap
  (`Credit(balance)`) and the old-debt cap (`OpeningUnlinked`) are read stale. Two refunds can take the balance above
  zero; two old-debt payments both pass and `PayOpening` clamps the second to 0 — money leaves, the balance doesn't
  move. Fix: `suppliers.LockByID` (as `customers.LockByID` in area 3), taken **after** any till / stock lock.
- **CHQ-4:** `unappliedAdvances` counts `amount − Σ allocations` of a payment whose cheque bounced; `reopened` and the
  cheque's status are ignored, so with another credit on hand (a debit note) a later bill is marked *paid* out of a
  bounced cheque. Fix: leave out payments whose cheques bounced / were cancelled (or subtract their unspent part).
- **PUR-4:** `parseLines` never calls `stock.CheckWhole`; a pack size of 2.5 passes ("more than 1"). Goods sent back
  (`purchasereturns.Create`) don't check whole units either.
- **SUP-2:** `AdjustOpening` takes a negative opening (`money.Parse`, no sign check; Create refuses it). Q2: a credit
  pays a newly set old debt down — today `opening_unlinked` can end above the balance (the auditor's SUP-2 check).
- **SUP-5 (Q1):** a typed advance (`req.Advance`) sits as credit while bills stay open; it should run through
  `Distribute` first (bills oldest first, then old debt), only the rest an advance.
- **SUP-9 (Q3):** `CreateTx`, `ReceiveTx`, `createDraftTx`, `purchasereturns.Create` never check `is_active`.
- **PUR-9 (Q4):** a frozen product's pieces out for sale: `DecrementGuarded` refuses when storage is short; do as
  STK-8 (to_shelf, then the return).
- **PUR-3:** a fully discounted line with a list cost gets lot cost = list cost (the "nothing paid → product's cost"
  branch); rule written as today's behaviour (as STK-7's blank-cost lot) — the case test pins it.
- `UnapplyPayment` sets `received` whenever paid reaches 0 even if credits cover the bill — read: credits can't cover
  a bill that was fully paid, so likely fine; the auditor's PUR-11 check will say.

---

### Task 1: The rule book for area 5

- [x] **Step 1:** Grid + rules in `docs/rules/purchasing.md` (PUR, SUP, CHQ; past `fix(` commits as rules).
- [x] **Step 2:** Q1–Q4 in `docs/rules/questions.md`; README links area 5.
- [x] **Step 3:** Asked; all four answered 2026-11-19 as suggested (SUP-5 a typed advance pays what's owed first,
  SUP-2 a credit pays newly set old debt down, SUP-9 nothing delivered for a disabled supplier, PUR-9 frozen pieces
  sent back like a frozen write-off) — built in Task 3 Step 3.
- [x] **Step 4: Commit** — `docs(rules): area 5 rule book …`

### Task 2: Auditor checks (`internal/ledgercheck/purchasing.go`)

- [x] **Step 1: Failing test** `internal/battle/ledger_purchasing_test.go` (the API builds the day): a supplier with an
  opening, an order received with free units + a line discount + a bill discount + a pack line, a delivery with no
  order paid on delivery in cash, a payment oldest first by card, an old-debt payment, an advance, a new bill settled
  from it, a discount, goods sent back against a delivery and with no delivery, a refund, a cheque payment cleared and
  one bounced, an opening adjusted → no violations; then corrupt one row at a time and assert that rule's id: SUP-1
  (balance off a cent; a payment's amount changed), SUP-2 (old debt above the opening / above the balance), PUR-1
  (total ≠ subtotal − discount; subtotal ≠ Σ lines), PUR-10 (debit note total ≠ Σ lines; allocations above it),
  PUR-11 (paid + credited > total; status paid with something left), PUR-12 (paid ≠ Σ money allocations − reopened),
  PUR-13 (a line with no lot; movements ≠ paid + free), SUP-3 (a payment's money receipt ≠ its amount; cheques ≠ a
  cheque payment), SUP-4 (allocations above the payment).
- [x] **Step 2: Run, fail; write the checks; run, pass.**
- [x] **Step 3:** (grocery year clean, 91 s; dev DB: no area-5 violations, only the old hand-set seed stock under SALE-20 / STK-2) Run `Run` over a Demo Room year (`ledgercheck` gen test) and the dev DB: every violation is a bug
  (→ Task 3 test), a wrong rule (→ the user) or seed data (fix the seeder).
- [x] **Step 4: Commit** — `test(ledgercheck): area 5 checks — supplier balances, bills, debit notes, payments`

### Task 3: Case tests, one per `example` rule (`internal/battle/cases_purchasing_test.go`)

- [ ] **Step 1:** One test per example rule: PUR-1 (fixed / percent line and bill discounts, clamps, free units owe
  nothing), PUR-2 (lot + movement per line; cost / price refreshed only by > 0; usual supplier only when empty), PUR-3
  (free units + line discount + bill discount → lots worth the total within a cent a line; whole delivery sent back
  clears its bill exactly; a nothing-paid line at list cost), PUR-4 (packs in units; pack 2.5 and 2.5 pieces refused
  on delivery, order and goods sent back), PUR-5 (order moves nothing; a bill can't be edited / deleted), PUR-6 (lines
  as received; a line that didn't come; keep the rest; received twice refused), PUR-7 (pay on delivery capped after
  credit settles it), PUR-8 (advance → return credit → loss credit order; refunded advance not spent), PUR-9 (named lot
  only; wrong delivery refused; lot cost not typed price; own bill first, then oldest; beyond bills → credit), SUP-2
  (opening once; sheet leaves an existing supplier's; negative refused; adjust delta), SUP-3 (cash from bank refused,
  card from safe refused — `86dacd6`; outside moves nothing), SUP-4 (oldest first / old debt first / typed; beyond a
  bill refused, nothing saved; another supplier's bill refused), SUP-5, SUP-6 (discount beyond owed refused, no
  money), SUP-7 (refund above credit refused; lands in the right place), SUP-9, SUP-10 (statement ends at the
  balance; Owing lists a disabled supplier), CHQ-1 … CHQ-5. Each ends with the auditor.
- [ ] **Step 2:** Run; each failure → fix test-first, one commit each.
- [ ] **Step 3: The user's answers** (Q1–Q4), test-first, one commit each: SUP-5 typed advance through the waterfall,
  SUP-2 credit pays new old debt + no negative opening, SUP-9 refuse for a disabled supplier, PUR-9 frozen pieces sent
  back via to_shelf.
- [ ] **Step 4: Commit** — `test(battle): area 5 cases — one per rule`

### Task 4: Random days with suppliers (`internal/battle/machine_purchasing_test.go`)

- [ ] **Step 1:** The machine keeps books for two suppliers (one with an opening) and each bill: order → receive
  (part, keep the rest), delivery with no order (free units, discounts, packs), pay on delivery, payment (oldest first
  / old debt first / typed / beyond owed), discount, goods sent back (against a delivery / none), refund, cheque given
  → cleared / bounced, opening adjusted, supplier disabled / brought back. After every step: the auditor, each balance
  and bill = the books (SUP-1, PUR-11, PUR-12), each lot from a delivery = the books, a refused action changes nothing.
- [ ] **Step 2:** Short run in `make test`; `make battle` long, detached, until 20,000 steps clean.
- [ ] **Step 3: Commit** — `test(battle): random shop days buying from and paying suppliers`

### Task 5: Faults (`internal/battle/faults_purchasing_test.go`)

- [ ] **Step 1:** PUR-6: two receives of one order at once — booked once. SUP-7 / SUP-8: two refunds at once, two
  old-debt payments at once, a payment beside a delivery settling from credit, an opening adjusted beside a payment —
  the balance = SUP-1's rows, nothing spent twice. CHQ-6: clear and bounce of one cheque at once. A delivery beside
  sales of the same product (lock order: stock, lots, then the supplier); a payment from a till beside a sale on it.
- [ ] **Step 2:** Power cut mid-delivery / mid-payment / mid-return (a purchasing worker in `TestBattlePowerCut`):
  each whole or absent, the auditor clean.
- [ ] **Step 3:** Each failure → fix test-first, one commit each; then `test(battle): area 5 faults`.

### Task 6: Mutation

- [ ] **Step 1:** `make mutate` on `internal/features/purchases`, `purchasereturns`, `supplierpay`, `suppliers`,
  `cheques`, `payout`; survivors replayed against the battle tests (`scripts/replay-mutants.py` in a scratch copy,
  absolute testdb path); each surviving mutant in money / stock code gets the test that kills it.
- [ ] **Step 2: Commit** — `test: area 5 mutants killed`

### Task 7: Close area 5

- [ ] **Step 1:** Q1–Q4's tests passing; `docs/guide/` purchasing / suppliers pages say what changed.
- [ ] **Step 2:** Full run (`scripts/full-run.sh`, `COVER=1`), one clean `make battle`.
- [ ] **Step 3:** Coverage map updated (re-upload its four JSON datasets); `docs/status.md` — area 5 done, bugs found
  and fixed.
- [ ] **Step 4:** Fresh review of the area; fix findings test-first.
- [ ] **Step 5:** Commit — `docs(status): battle test area 5 done`; delete this plan. Area 6's plan replaces it.
