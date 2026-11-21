# Plugin battle test — group 3: plugins with money of their own — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Repairs and Reload & Bills held to their rule books (`docs/rules/plugins/`, PLG + REP / RCH), every bug
found fixed test-first, the user's three answers built (2026-11-21, `questions.md`).

**Architecture:** groups 1–2's harness as is — `ledgercheck.Add` in `plugins/<key>/ledger.go`, `ltest.NewShop`,
battle tests in `plugins/<key>/battle/` (one shop per test binary). New core seam: `plugin.SaleLine.Gross` (the
line's price × qty before discount) so a pass-through float moves by face value (RCH-3).

**Order:** repairs → recharge (riskiest last: float per device, own tender, P&L income).

## Global Constraints

- Core stays plugin-inert (`make check-core`). Every check / test names its rule. A bug = failing test first, then
  the fix, one commit each.
- Per plugin: auditor checks (each proved to fire on a corrupted row) → case per rule → refusals → failure sweep
  over its writes → at once / sent twice → gremlins replayed against its own + battle tests → close gaps.
- A sale hook never refuses for a business reason; recharge refuses only a line it can't book at all (RCH-3).
- One long run at a time; long runs detached with logs in `~/.cache/karots-logs/`.

## Suspected (from reading, 2026-11-21)

- REP-1: `SetStatus` / `UpdateJob` / `AddPart` / `AddCharge` check "open" before writing, not in the write — a cancel
  landing with the pickup sale turns a collected job cancelled.
- REP-2: removing another job's part or labour answers 200 and changes nothing (should be 404).
- REP-8: the report sums every labour-product line of the pickup sale per job — two jobs on one sale count each
  other's labour (and a part product also sold loose). Fix: `repair_pickup_lines` written by the sale hook.
- RCH-3: package float = one `float_cost` whatever the qty; reload / bill / deposit float = the net after a line
  discount (user: face value); a turned-off bank refuses the sale (user: book it).
- RCH-6: a line returned in parts rounds each part — can give back a cent more / less than left.
- RCH-8: a count locks the device row but a sale doesn't, so a sale committing between the count's read and its
  write is counted twice.

### Task 1: Rule book — [x] written, questions answered
### Task 2: Repairs — checks, cases, refusals, failure sweep, at once (REP-1–10) — [ ]
### Task 3: Recharge — face value seam, checks, cases, refusals, failure sweep, at once (RCH-1–11) — [ ]
### Task 4: Mutation (gremlins on both, replayed 9 side by side) — survivors read, tests added — [ ]
### Task 5: Docs (guides for changed behaviour), full run with coverage, coverage map Plugins rows, status — [ ]
