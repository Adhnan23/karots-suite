# Plugin battle test — Pocket Cashier — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pocket Cashier held to its rule book (`docs/rules/plugins/pocketcashier.md`, PLG + PKT), every bug found
fixed test-first, the user's two answers built (2026-11-21, `questions.md`): a call never runs twice (PKT-4), a cash
out / in from the phone is booked as made (PKT-10, DRAW-19).

**Architecture:** the groups' harness as is — `ledgercheck.Add` in `plugins/pocketcashier/ledger.go`,
`ltest.NewShop`, battle tests in `plugins/pocketcashier/battle/` (one shop per test binary; a paired, approved phone
sending `/api/v1/x/pocketcashier/actions` with its key). New core seam for DRAW-19: `cashregister.MovementInput.Remote`
(booked as made past the drawer / untracked setting, the session flagged). PKT-4: a `running` claim row in
`pocket_actions` (migration 0005) before a call runs; a repeat finding it doesn't run again and is flagged.

## Global Constraints

- Core stays plugin-inert (`make check-core`). Every check / test names its rule. A bug = failing test first, then
  the fix, one commit each. Pocket adapts from its own side: no plugin changes for the phone.
- Per layer: auditor checks (each proved to fire on a corrupted row) → case per rule → refusals → failure sweep over
  every action kind → at once / sent twice / a failure after the route answered → gremlins replayed → close gaps.
- One long run at a time; long runs detached with logs in `~/.cache/karots-logs/`.

## Suspected (from reading, 2026-11-21)

- PKT-4: a call's route commits, then recording it fails → the phone resends → it runs twice.
- PKT-5: only sales and calls wait on the PC's own trouble; open / close / cash out / in / hold / unhold become
  refusals for good on a database blip; unhold calls any error "already resumed".
- PKT-7 / DRAW-19: a refused cash out / in books nothing; cash out / in never opens a late till.

### Task 1: Rule book — [x] written, questions answered
### Task 2: Harness + checks — battle package (shop, phone, send), `ledger.go` (PKT-4, 7, 12), each proved to fire — [x]
### Task 3: Cases — PKT-1–3, 6, 8–15 through the real server; refusals (PLG-5) — [x] (found: a send's counts written only at batch end, fixed `cb85c544`)
### Task 4: Fixes — PKT-5 (retry every kind), DRAW-19 + PKT-10 (remote cash moves), PKT-4 (claim row, credit ref) — [ ]
  - [x] DRAW-19 / PKT-10: `Remote` cash out / in / close booked as made, till flagged; late till opened; a refusal books the cash
  - [ ] PKT-5 + PKT-4: `TestPKT5_FailureSweep` (battle/faults_db_test.go) is written and FAILS on purpose — open / close
    refused for good on a database blip. Next: a `trouble` mark on failures that are the PC's own (non-AppError or 5xx;
    make `depends on …` / unmarshal errors business errors), retried via `keepTrying` before any refused-cash booking;
    migration 0005 lets `pocket_actions.status` be `running`; non-sale actions write a `running` claim before running,
    `stored()` turns a found `running` into failed "may not have gone through — check" with `cash_settled = true`;
    the send row made lazily in the record statement (no empty sends left by a failure); credit payment call gets
    `clientRef` = action ref.
### Task 5: Faults — failure sweep over every action kind, at once, sent twice, failure after the route answered — [ ]
### Task 6: Phone side (Vitest) — outbox order / ack, temp numbers, the till's cases through the phone's sale request (PKT-14) — [ ]
### Task 7: Mutation (gremlins replayed 9 side by side) — survivors read, tests added — [ ]
### Task 8: Docs (guide for changed behaviour), full run with coverage, coverage map Plugins row, status — [ ]
