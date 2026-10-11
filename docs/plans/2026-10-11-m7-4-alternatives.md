# M7.4 Alternatives — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: use superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> Same shape as M7.3. Opus executes and reviews, one commit per step, and UI gates are self-reviewed (4 combos, 1366 + 360).

**Goal:** Alternatives, reinterpreted: optional tiers, a swap chip on short cart lines, alternatives in the ⓘ
popup, one reorder surface (core's Reorder page, filtered before paging), membership from the product form,
visible health.

**Spec:** `docs/specs/2026-10-11-m7-4-alternatives-design.md`. Fork source: karots-pos `plugins/alternatives/` (read-only).

## Global constraints

- Core never references a plugin. The core changes are the generic seams in spec §2 B and E. `make check-core` after every task.
- `enabled_plugins.go` / `enabled-plugins.ts` committed core-only.
- Plugin JSON in `plugins/alternatives/api.go`, mirrored in `web/src/plugins/alternatives/types.ts`.
- Theme tokens only, 360px admin, 4 combos. No AI attribution in commits.

## Review focus

1. Coverage / low maths match karots-pos (active members only, level 0 untracked) — Task 1.
2. Reorder: hide-covered filters before paging; pulled-in members respect the filters; core-only unchanged — Task 3.
3. Swap keeps qty, merges into an existing plain line, never touches group lines — Task 4.
4. An old backup restores with no mapping — Task 7.

---

### Task 1: Plugin Go half — tables, store, hooks
- [ ] `0001_alternatives.sql` byte for byte; store (groups, tiers, members, summaries, search, badges, coverage, low tiers, till alternatives, membership); DB tests; hooks (search, badge, detail rows, reorder annotator, product extras).
- [ ] Commit `feat(alternatives): groups, optional tiers and members — karots-pos tables kept; search, pin, ⓘ rows, reorder notes, product form extras`

### Task 2: Plugin API
- [ ] Routes in spec §4; tests (cashier 403 on admin, 409 move, audit). Commit `feat(alternatives): JSON API`

### Task 3: Core seam — reorder annotations
- [ ] `ListQuery.AlsoIDs`; `ReorderRow.note/covered`, `ReorderList.annotated`, `?hideCovered=1` filtered before paging; Go test with a fake annotator; Reorder page toggle + note. `make types`.
- [ ] Commit `feat(reorder): plugin notes on the Reorder page and Hide covered items, filtered before paging (generic seam)`

### Task 4: Core seam — swap a cart line
- [ ] `swapProduct` + reducer `swap` + `TillApi.swapLine`; Vitest. Commit `feat(till): swap a cart line's product, keeping its qty (generic seam)`

### Task 5: Plugin React half (UI gate)
- [ ] Apps tile + page (group list with tier status, Low only, CSV; group page with tiers ↑↓, members, health, picker with Move here?); product form section; till swap chip.
- [ ] Gate shots: 4 combos at 1366 + 360 (list, group page), till chip. Commit `feat(alternatives): Apps → Alternatives, product form section and the till's swap chip`

### Task 6: E2E
- [ ] `plugin-alternatives.spec.ts`: make a group with tiers, pin + search at till, swap a short line, ⓘ rows, reorder hide covered, product form membership, 360px. Commit.

### Task 7: Restore + docs
- [ ] `restore_db_test.go` (karots-pos rows, no mapping); parity rows; `docs/differences.md`; `docs/status.md`. Full checks: `make verify`, `go test ./...` core-only and all plugins (`-p 1`), web unit, Playwright, `make verify-portable`. Commit `docs: M7.4 done — Alternatives`
