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

### Task 1: Rule book — [x] written, questions answered, committed
### Task 2: Harness (ledgercheck.Add, plugin-aware template, ltest.NewShop) — [ ]
### Task 3: Clearance — [ ]
### Task 4: Customer requests — [ ]
### Task 5: Product Plus — [ ]
### Task 6: Alternatives (+ loose-versions spec) — [ ]
### Task 7: AI Assistant (+ aicatalog:278) — [ ]
### Task 8: Full run with coverage, coverage map Plugins rows, status — [ ]
