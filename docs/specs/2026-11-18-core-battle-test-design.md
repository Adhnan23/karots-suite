# Core battle test — design

## Why

Iman is selling on the suite, and bugs reach them before we find them. A few days ago: a sale with a discount,
returned, asked to refund the **full price** (fixed in `d17d7b7`; the same mistake was hiding in the P&L, `5f108d3`).
Our tests check examples someone thought of; that bug lived where nobody had written the rule down. The user wants
the core's rules written for every scenario and the core proven against them **before** shops find the next bug —
core first, because everything else (plugins, Pocket Cashier, reports) builds on top of it.

## Goals

- **One rule book for all of core**: every money, stock, balance and document rule, in plain words, numbered, each
  scenario included (discounts × returns × packs × tenders × credit × cheques × freeze …). The user checks it; it is the
  truth the tests are measured against. It stays in the repo for good and changes with the product.
- Every rule checked by machine, in four layers that catch what the others miss (below).
- Every bug found is fixed test-first, one commit each, before the next Release.

## Non-goals

- Plugins and Pocket Cashier (next, on the same tools: plugins add their own rules and auditor checks).
- Proving the whole Go program, the SQL or the screens in Lean / Rocq (not realistic for one developer).
- Changing how the till behaves, except to fix a rule it breaks.

## Anti-goals (a failure even if it works)

- Tests that pass because they were written to match the code instead of the rule.
- A rule changed to fit the code without the user agreeing.
- Slowing `make test` so much it stops being run (the deep runs get their own make target).
- Anything at a shop that writes data or blocks the counter (never stop a sale).

## Approach (picked by the user, 2026-11-18)

Considered: (a) more example tests, (b) random / property tests only, (c) full formal proof of the system,
(d) layers: rule book + proof where it fits + auditor + random shop days + mutation testing. (a) and (b) only test
cases someone imagined and can't say what's still uncovered; (c) can't reach the database and screens. **(d) chosen.**

1. **Rule book** — `docs/rules/<area>.md`, rules numbered `SALE-12`, `RET-7`…; each says what must always be true and
   lists the tricky scenarios. Written from the code, the guides, git history (every past fix is a rule) and the
   user's shop knowledge. Rules I'm unsure of are marked **?** and asked in batches; work continues on the rest.
2. **Proof of the money maths (Lean 4)** — the pure arithmetic: line and bill discounts, how a bill discount is
   shared across lines, the paid value of a line, refunds after partial returns (the last unit takes what's left),
   packs, rounding, change, tender splits, drawer expected cash. A Lean model + proofs in `proofs/` (never in the
   binary). The model also writes test cases; Go **and** the till's TypeScript must give the same answers on them
   (the till's screen total = the server's total is itself a rule). Lean runs in GitHub Actions (slow internet here);
   the generated case file is committed.
3. **Auditor** — `internal/ledgercheck`: reads the database, checks every rule that can be seen in data (money moves
   balance; each drawer = opening + ins − outs; stock = sum of movements; sale lines − discounts = total = payments;
   returned ≤ sold, refunded ≤ paid; customer / supplier balances = bills − payments; cheque states agree with money;
   reports = raw data). Read-only. Runs after every DB test and in the Demo Room; nightly at a shop only if the user
   says so later.
4. **Random shop days** — `pgregory.net/rapid` state machines driving the **real API** with every core action (sell,
   hold, return part / whole, discounts, packs, every tender, credit, payouts, drawer open / close, receive, supplier
   return, supplier pay, cheques and bounces, conversions, write-offs, freeze, recovery, expenses), many days in
   random order, the auditor after each step; a failure shrinks to the shortest steps that break it. Plus faults:
   the server killed mid-request (power cut: each sale whole or absent), two tills at once, the same request twice.
   Till cart maths in TS gets `fast-check` properties.
5. **Mutation testing** — `gremlins` (Go) and Stryker (TS) on the core money / stock packages: every surviving mutant
   is a blind spot to close. Run by hand / a make target, not in `make test`.

## Order

Highest risk first, each area done through all layers before the next:
1. Sales, discounts, tenders, returns (and the till cart maths).
2. Drawer, cashflow, payouts, expenses, money sources.
3. Customers: credit, advances, opening balances, payments.
4. Stock: lots, packs, units, conversions, freeze, write-offs, give-back, recall.
5. Purchasing: purchases, receive, supplier returns, supplier pay (waterfall), credits, cheques.
6. Recovery and losses.
7. Reports and the Z report = the raw data.

## Decided

- Rule book first; tests cite rule numbers.
- Fixes go in core as found (test-first, one commit each); refactors only to pull money maths into plain functions.
- Deep runs: `make battle` (rapid days, faults) and `make mutate`; `make test` keeps a short rapid run.
- New test deps (rapid, fast-check, gremlins, Stryker) → `THIRD_PARTY_NOTICES.md` regenerated.

## Left to the builder

Exact package layout of the state machines, how many steps / days per run, which Lean structures model decimals
(exact rationals or integer cents), how the generated case file is shaped.
