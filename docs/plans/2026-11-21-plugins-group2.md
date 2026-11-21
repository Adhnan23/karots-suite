# Plugin battle test — group 2: plugins that sell through the till — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Combos, Print & Copy and Pharmacy held to their rule books (`docs/rules/plugins/`, PLG + CMB / DOC / PHA),
every bug found fixed test-first, the user's four answers built (2026-11-21, `questions.md`).

**Architecture:** group 1's harness as is — `ledgercheck.Add` in `plugins/<key>/ledger.go`, the plugin-aware
template, `ltest.NewShop`, battle tests in `plugins/<key>/battle/` (one shop per test binary). New here: the three
plugins' sale records move onto core's `SaleLineHook` (inside the sale's transaction, `plugin.SaleLines`); their
after-sale calls, the till's `tillAfterSale` pushes and Pocket Cashier's offline routes for them go. A sale through
`ltest.NewShop` must carry `plugin` data on its lines (as the till does).

**Order:** combos → documents → pharmacy (after Product Plus + Alternatives, which group 1 passed).

## Global Constraints

- Core stays plugin-inert (`make check-core`). Every check / test names its rule. A bug = failing test first, then
  the fix, one commit each (`fix(combos): … (CMB-5)`).
- Per plugin: auditor checks (each proved to fire on a corrupted row) → case per rule → refusals → failure sweep
  over its writes → at once / sent twice → gremlins replayed against its own + battle tests → close gaps.
- A sale hook never refuses for a business reason (never stop a sale): an off tag is skipped, not an error.
- One long run at a time; long runs detached with logs in `~/.cache/karots-logs/`.

## Suspected (from reading, 2026-11-21)

- CMB-5 / DOC-6 / PHA-5: sale records by a separate after-sale call (lost on a blip; combos trusts the till's
  sets / net / discount; offline Pocket queues them with a temp sale number). User: move into the sale.
- CMB-6: combo report counts returned sets (user: take them off).
- CMB-1: a `fixed` deal larger than a set's price is accepted (the till caps it; fine) — check; a percent of 0?
- DOC-1: `PUT /services/:id` with a bad kind after a 404 check order; `Update` / `SetActive` don't report 0 rows.
- DOC-3: duplicate grid rows (same size / type / colour / side / min qty) — `PickPrice` picks one silently?
- PHA-5: `RecordDispense` reads scripts outside its transaction; the advisory lock casts the sale id to `int`.

### Task 1: Rule book — [x] written, questions answered
### Task 2: Combos — sold rows by the sale hook (CMB-5), returns off the report (CMB-6), checks, cases, faults — [ ]
### Task 3: Print & Copy — jobs by the sale hook (DOC-6), checks, cases, refusals, faults, report agreement — [ ]
### Task 4: Pharmacy — dispense in the sale (PHA-5), register, checks, cases, faults — [ ]
### Task 5: Mutation (gremlins on the three, replayed 9 side by side) — survivors read, tests added — [ ]
### Task 6: Docs (guides for the changed behaviour), full run with coverage, coverage map Plugins rows, status — [ ]
