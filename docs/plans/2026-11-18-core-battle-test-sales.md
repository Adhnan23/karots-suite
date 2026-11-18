# Core battle test — area 1: sales, discounts, tenders, returns — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every rule of selling, discounting, paying and returning written down (including the ones nobody knew
about), checked by machine four ways, and every bug found fixed — building the tools the later areas reuse.

**Architecture:** A rule book built as a **grid** (every feature a sale can carry × every later action on it) so
unknown pairs like discount × return show up as empty cells. A read-only auditor (`internal/ledgercheck`) checks the
rules that are visible in data. A `rapid` state machine drives the real server (throwaway database, `app.BuildServer`,
core only) through random shop days with power cuts and two tills. A Lean 4 model proves the money maths and writes
test cases that Go and the till's TypeScript must both match. Mutation testing shows what the tests still miss.

**Tech Stack:** Go (`pgregory.net/rapid`, `gremlins`), TypeScript (`fast-check`, Stryker), Lean 4 (core only, no
Mathlib), GitHub Actions for Lean.

**Spec:** `docs/specs/2026-11-18-core-battle-test-design.md`

## Global Constraints

- Core only: the harness builds with the plugin lists core-only; core never imports `internal/battle` or Lean.
- `internal/ledgercheck` is read-only (SELECTs only; a test proves it).
- Every rule has an id (`SALE-n`, `DISC-n`, `TEND-n`, `RET-n`, `TILL-n`); every check and test names the rule it
  guards. A rule is never changed to fit the code without the user agreeing.
- A bug found = a failing test first, then the fix, one commit each (`fix(<area>): … (RET-4)`).
- The full run already takes ~1 hour (user, 2026-11-18): the battle tests add **at most 1 minute** to `make test`
  (≤ 200 steps per machine, skipped with `-short`); everything long goes in `make battle` / `make mutate`, which use
  their own throwaway databases so they can run beside other work. Speeding up the existing run is a separate job
  after area 1.
- Long runs run detached with logs in `~/.cache/karots-logs/` (power cuts here).
- New deps → `go run ./scripts/notices > THIRD_PARTY_NOTICES.md`.
- No AI attribution in commits.

## Review Focus

- **Rounding across lines:** a bill discount shared over lines that each round (3 × Rs 100, Rs 10 off → each line paid
  96.67, total 290.01 vs 290 paid). Returning every line must refund exactly what was paid — Task 4 pins it.
- **The till shows one total, the server charges another:** TS money is `number` (floats) in `totals.ts`; Go is
  decimal. Task 7 makes both answer the same cases.
- **Return after return:** a line returned in parts, then the rest, then a whole-sale return — refunds must add up to
  what was paid, never more, the drawer and credit following them. Task 5 machine + Task 4 cases.
- **Power cut mid-sale:** the sale, its stock moves, its payments and its drawer cash are all there or all gone.
  Task 6.
- **The same sale twice** (double Pay, a replayed `client_ref`, two tills selling the last unit). Task 6.

---

### Task 1: The rule book for area 1 (the grid)

**Files:**
- Create: `docs/rules/README.md` (format, ids, how a rule is checked, the questions list)
- Create: `docs/rules/sales.md` (SALE, DISC, TEND, TILL rules) and `docs/rules/returns.md` (RET rules)
- Create: `docs/rules/questions.md` (rules marked **?**, for the user)

- [ ] **Step 1: List the features a sale can carry** — read `internal/features/sales/*.go`, `service_types.go`,
  `web/src/features/till/cart/*.ts`, `docs/guide/till.md`, `docs/guide/money.md`. At least: item discount (fixed per
  unit / percent), bill discount (fixed / percent), packs, loose / units, batch-price pick, oversell, frozen stock,
  serials, service lines + components, recipes, payouts, line groups, customer, credit (on account, over limit),
  every tender (cash, card, cheque, discount method, plugin tender via seam), split tenders, change + kept change,
  held sales / tickets (sales person), replay (`client_ref`, review flags), price override / expect price.
- [ ] **Step 2: List every later action on a sale** — whole return, partial return (in parts, then the rest), return to
  credit / cash / card, reprint, sales list totals and profit, Z / X report, the drawer's expected cash, the
  customer's balance and statement, stock and lot quantities, P&L, sales trend, tax report, cheque bounce.
- [ ] **Step 3: Write the grid** in `sales.md` / `returns.md`: one table, features × actions. Each cell is a rule id,
  `—` (can't happen, with why), or **?**. Read git history for every past fix in this area
  (`git log --oneline -- internal/features/sales internal/features/reports web/src/features/till/cart | grep -i fix`)
  — each becomes a rule with a "found in" note. Each rule: one sentence of what is always true, the tricky cases, and
  how it is checked (`ledgercheck`, `machine`, `lean`, `example`).
- [ ] **Step 4: Put every ? in `questions.md`**, grouped, plain words, each with what the code does today.
- [ ] **Step 5: Commit** — `docs(rules): sales and returns rule book — the feature × action grid; questions for the user`
- [ ] **Step 6: Send the user the questions** (one message). Carry on with the rules that have no ?.

### Task 2: The auditor (`internal/ledgercheck`)

**Files:**
- Create: `internal/ledgercheck/check.go` (types + `Run`), `internal/ledgercheck/sales.go` (area 1 checks)
- Test: `internal/ledgercheck/sales_db_test.go`

**Interfaces:**
- Produces: `type Violation struct { Rule string; Ref string; Detail string }`;
  `func Run(ctx context.Context, q sqlx.QueryerContext) ([]Violation, error)` — runs every registered check;
  `func Check(t testing.TB, ctx context.Context, q sqlx.QueryerContext)` in `ledgercheck/ltest` (fails the test with
  every violation listed). Checks are plain SQL; each returns rows that break its rule.

- [ ] **Step 1: Write the failing test** — on a throwaway database: a clean discounted sale + partial return → `Run`
  returns no violations; then, one at a time, corrupt a row (sale total off by 0.01; a payment missing; returned_qty
  > quantity; refunds of a line > its paid value; stock on hand ≠ sum of movements) and assert exactly that rule's id
  comes back. Plus: `Run` inside a `READ ONLY` transaction succeeds (it writes nothing).
- [ ] **Step 2: Run it, see it fail** — `go test ./internal/ledgercheck/` → undefined `Run`.
- [ ] **Step 3: Write the checks** for every area-1 rule Task 1 marked `ledgercheck`.
- [ ] **Step 4: Run, pass.**
- [ ] **Step 5: Run `Run` against the dev DB and the Demo Room's year** (`make demo-dev` shop) — every violation is a
  real bug or a wrong rule: write each down in `docs/rules/questions.md` or as a failing test for Task 4.
- [ ] **Step 6: Commit** — `test(ledgercheck): the auditor — area 1 checks (sales, discounts, tenders, returns)`

### Task 3: The battle harness

**Files:**
- Create: `internal/battle/shop_test.go` (throwaway DB + server), `internal/battle/fixtures_test.go` (products: plain,
  packs, loose, service, serial, frozen; a customer with credit; cash + card + cheque tenders), `TestMain`
- Modify: `Makefile` (`battle:` target), `go.mod` (`pgregory.net/rapid`)

**Interfaces:**
- Consumes: `app.BuildServer` (as `internal/demo/gen/gen_db_test.go` `openShopDB` does), `gen.NewClient` / `Do`.
- Produces: `func newShop(t testing.TB) *shop` with `admin, till1, till2 *gen.Client`, `db *sqlx.DB`, product ids by
  kind; `func (s *shop) audit(t testing.TB)` = `ltest.Check`; `func steps() int` (200 in `make test`,
  `KS_BATTLE_STEPS` in `make battle`, default 20000).

- [ ] **Step 1: Write `TestShopOpens`** — newShop, open a till, sell one line in cash, close the till, `audit` clean.
- [ ] **Step 2: Run it, fail; build the harness; run it, pass.**
- [ ] **Step 3: `make battle`** = `KS_BATTLE_STEPS=20000 go test -run Battle -timeout 6h ./internal/battle/`, logs to
  `~/.cache/karots-logs/battle-<date>.log`. Regenerate `THIRD_PARTY_NOTICES.md`.
- [ ] **Step 4: Commit** — `test(battle): a throwaway shop on the real server for random shop days`

### Task 4: Known-shape cases first (the rounding pair and every grid cell marked `example`)

**Files:**
- Test: `internal/battle/cases_test.go` (one test per rule id, table-driven), fixes where they fail

- [ ] **Step 1: Write `TestRET_ReturnEveryLineRefundsWhatWasPaid`** — 3 lines × Rs 100, bill Rs 10 off (paid 290);
  return each line whole, one at a time → refunds sum to exactly 290.00; drawer down 290.00; then the same sale
  returned whole in one go → 290.00. Also 7 lines × Rs 33.33 with 5% off, and a line of 3 returned 1 + 1 + 1.
- [ ] **Step 2: Run** — expected to fail today (each line's paid value rounds alone: 96.67 × 3 = 290.01).
- [ ] **Step 3: Fix** in `internal/features/sales`: the bill discount is shared over lines **once, so the lines add
  up to the total exactly** (largest remainder, ties to the earlier line); a line's paid value comes from that share
  everywhere — returns, `PaidFactorSQL` users (reports, Z, sales list). If that needs the share stored, it is
  migration `00037_sale_item_paid` (`sale_items.paid`, back-filled with the same sharing) — say so to the user before
  committing a migration.
- [ ] **Step 4: Write the other `example` cells** from Task 1, one test each, run, fix each failure test-first.
- [ ] **Step 5: Commit per fix**, then `test(battle): area 1 cases — one per rule`.

### Task 5: Random shop days (the state machine)

**Files:**
- Test: `internal/battle/sales_machine_test.go`

**Interfaces:**
- Consumes: `newShop`, `audit`, `steps`.
- Produces: `type salesMachine` with rapid actions — `OpenTill`, `Sell` (1–8 random lines from every fixture kind,
  random qty incl. pack / loose amounts, random item discounts fixed / percent incl. 0 and 100%, random bill
  discount, random tender split incl. credit + over-limit, cheque, overpaid cash + kept change), `Hold`, `Resume`,
  `SendToCashier`, `ReturnPart`, `ReturnRest`, `ReturnWhole`, `Payout`, `CloseTill`, `NextDay`; and a **model** in
  plain Go of what every sale was paid and every line has refunded.

- [ ] **Step 1: Write `TestBattleSales`** — `rapid.Check` with `t.Repeat` over the actions; after every action:
  `audit`, and model checks — refunds per line ≤ paid, per sale ≤ paid, a fully returned sale refunds exactly what was
  paid, drawer expected cash = model's cash in − out, customer balance = model's.
- [ ] **Step 2: Run short** (`go test ./internal/battle/ -run TestBattleSales`) — each failure shrinks to a few steps:
  turn it into a Task 4 style case test, fix test-first, commit.
- [ ] **Step 3: Run long** (`make battle`, detached) — same loop until a 20,000-step run is clean.
- [ ] **Step 4: Commit** — `test(battle): random shop days for selling, paying and returning`

### Task 6: Faults — power cuts, double Pay, two tills

**Files:**
- Test: `internal/battle/faults_test.go`

**Interfaces:**
- Produces: `func startServer(t, dsn) (url string, kill func())` — builds `./cmd/server` once in `TestMain`, runs it
  on a free port against the throwaway DB; `kill` is SIGKILL.

- [ ] **Step 1: Write `TestBattlePowerCut`** — loop: start, fire random sales / returns over HTTP, SIGKILL at a random
  moment (0–200 ms), start again, `audit` → no violations, and every sale either complete (lines, stock, payments,
  drawer cash) or absent.
- [ ] **Step 2: Write `TestBattleTwoTills`** — two tills sell the last units of one product and return the same sale
  at once; the same `client_ref` sent twice; Pay sent twice → one sale, never refunds above paid, `audit` clean.
- [ ] **Step 3: Run, fix each failure test-first, commit** — `test(battle): power cuts, double Pay and two tills`

### Task 7: The till's maths (TypeScript)

**Files:**
- Test: `web/src/features/till/cart/totals.property.test.ts`, `checkout.property.test.ts`
- Modify: `web/package.json` (`fast-check` dev dep)

- [ ] **Step 1: Write fast-check properties** — for random carts: `billDiscount` never exceeds the lines' net, no
  line's discount exceeds its gross, totals never negative without a payout, `tenderIssue` agrees with paid vs due,
  `notesFor` / `quickCash` / `keptAmount` give change that adds up, `buildSaleRequest` carries exactly the cart's
  lines and discounts (the server recomputes from these).
- [ ] **Step 2: Run** (`cd web && bun run test totals.property`) — fix failures test-first (float drift is the likely
  one: round at the same places Go does).
- [ ] **Step 3: Commit** — `test(till): property tests for the cart's money maths`

### Task 8: Lean — prove the money maths, then hold Go and TS to it

**Files:**
- Create: `proofs/lakefile.toml`, `proofs/lean-toolchain`, `proofs/Karots/Money.lean` (amounts as integer cents),
  `proofs/Karots/Sale.lean` (line net, item discount, bill share, paid value, refunds), `proofs/Karots/Cases.lean`
  (writes `proofs/cases/sales.json`)
- Create: `.github/workflows/proofs.yml` (elan + `lake build` + regenerate cases + fail if they differ)
- Test: `internal/features/sales/lean_cases_test.go`, `web/src/features/till/cart/lean.cases.test.ts`

- [ ] **Step 1: Install elan + the Lean toolchain locally** (~200 MB once, no Mathlib) — ask the user first because
  of the slow internet; if no, iterate through the Actions run only.
- [ ] **Step 2: State the theorems** — line shares of a bill discount sum to the discount exactly; each share ≤ its
  line; refunds of a line over any split of its quantity sum to its paid value; refunds of a whole sale = total paid;
  no refund sequence pays more than was paid; change = paid − due ≥ 0 when paid ≥ due.
- [ ] **Step 3: Prove them** (`lake build` green, no `sorry`).
- [ ] **Step 4: Generate 2,000 cases** (seeded) → `proofs/cases/sales.json`: inputs + every expected amount.
- [ ] **Step 5: Go and TS read the cases** — the Go functions (bill share, paid value, line refund) and the TS
  `lineDiscount` / `billDiscount` must give each expected amount to the cent. Run both; fix differences test-first.
- [ ] **Step 6: Commit** — `test(proofs): Lean proof of the sale and refund maths; Go and the till held to its cases`

### Task 9: Mutation testing

**Files:**
- Modify: `Makefile` (`mutate:` target), `web/package.json` (Stryker dev deps), `web/stryker.config.json`

- [ ] **Step 1: `make mutate`** — `gremlins unleash` on `internal/features/sales`, `internal/features/cashregister`,
  `internal/features/cashflow` with `DATABASE_URL`; Stryker on `web/src/features/till/cart`. Detached, logs on disk.
- [ ] **Step 2: For each surviving mutant in money / stock code:** add the test that kills it (or note why it's
  harmless in `docs/rules/README.md`). Commit per batch — `test(sales): close mutation gaps — …`
- [ ] **Step 3: Regenerate `THIRD_PARTY_NOTICES.md`, commit.**

### Task 10: Close area 1

- [ ] **Step 1:** User's answers from `questions.md` folded into the rules; their tests written and passing.
- [ ] **Step 2:** Full run: `make test`, `make test-web`, Playwright, `make check-core`, one clean `make battle`.
- [ ] **Step 3:** `docs/status.md` — area 1 done, bugs found and fixed, what a Release would carry; `docs/guide/
  developer.md` gets a short "Rules and battle tests" section (where the rule book is, `make battle`, `make mutate`,
  `proofs/`).
- [ ] **Step 4:** Fresh review of the area; fix findings test-first.
- [ ] **Step 5:** Commit — `docs(status): battle test area 1 done`. The next area's plan replaces this one.
