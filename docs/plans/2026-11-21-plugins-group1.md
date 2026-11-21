# Plugin battle test — group 1: product-data plugins — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Clearance, Customer requests, Product Plus, Alternatives and the AI Assistant held to their rule books
(`docs/rules/plugins/`, PLG + CLR / REQ / PPF / ALT / AIC), every bug found fixed test-first, the user's three answers
built, the failing `loose-versions` and `aicatalog:278` specs fixed.

**Architecture:** Core's layers, reached from plugin packages without core naming a plugin:
- `ledgercheck.Add(rule, sql)` — a generic seam; each plugin registers its checks in `plugins/<key>/ledger.go`.
- `ltest`'s template is named by core **and every compiled-in plugin's** migrations (today only core's — a plugin
  test would get a template without its tables, or a core test one with them).
- `ltest.NewShop(t)` — an exported throwaway shop (real server over `NewDB`, an admin and a cashier signed in,
  `Do`), so `plugins/<key>/battle_db_test.go` can drive the real API. A test binary holds exactly the plugins its
  package imports.

**Order:** harness → clearance → requests → productplus → alternatives → aicatalog (it works through Product Plus,
Pharmacy and Alternatives, so it goes after them and its tests compile them in, AIC-9).

## Global Constraints

- Core stays plugin-inert (`make check-core`). Every check / test names its rule. A bug = failing test first, then
  the fix, one commit each (`fix(requests): … (REQ-3)`).
- Per plugin: auditor checks (each proved to fire on a corrupted row) → case per rule → refusals → failure sweep
  over its writes → at once / sent twice → gremlins on the package → close gaps.
- One long run at a time; long runs detached with logs in `~/.cache/karots-logs/`.

## Suspected (from reading, 2026-11-21)

- CLR-2: approve takes any product id (none, a service, disabled) — no check. CLR-6: disable takes any ids, not
  transactional (one failing leaves the earlier ones disabled and says "failed").
- REQ-3: make-product twice at once makes two products (the request check and the create aren't one step).
  REQ-2/4: an ask added while a merge runs is deleted with the merged request (cascade); two crossed merges.
- ALT-5: hand split not rescaled when a member leaves (user's answer: rescale).
- AIC-7: combos take the 200 oldest products (user's answer: best sellers).

### Task 1: Rule book — [x]
### Task 2: Harness — [x] (`ltest.Main` / `Shared`: one shop per test binary; battle tests in `plugins/<key>/battle/`)
### Task 3: Clearance — [x] CLR-2, CLR-6 fixed
### Task 4: Customer requests — [x] REQ-3 (4 products from one request), REQ-2 (lost ask), merged-away link
### Task 5: Product Plus — [x] NaN/Inf/hex numbers, Fill in for missing products, 404s, saved field answered
### Task 6: Alternatives — [x] ALT-5 rescale built, saved change answered, loose-versions spec fixed
### Task 7: AI Assistant — [~] seam (appdb.InTx) + one-tx apply / undo, category date on undo. LEFT:
- [x] AIC-7: best sellers (90 days, then newest; the 50 batch cap no longer trims the 200), spec :278 passes
- [ ] Sweep groups job and setup job too (grouper / SavePack under the joined tx).
### Task 8: closing — [ ] `make mutate` (gremlins) on the five packages; docs: guide pages for CLR-6 (skipped
count) and ALT-5 (rescale); full run with coverage (`scripts/full-run.sh`); coverage map Plugins rows; status;
delete this plan.
