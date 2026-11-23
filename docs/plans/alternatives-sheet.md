# Alternatives sheet Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Export the shop's Alternatives as a sheet (`Group, Tier, Product, Barcode`) and import one back as the whole set, after a preview.

**Architecture:** Inside the Alternatives plugin, modelled on the cashier menu sheet (`internal/features/productgroups/menu.go` `Rebuild`, `internal/api/admin_data_menu.go`, `web/src/features/data/MenuSheet.tsx`). One core seam with no behaviour change: the header mapping moves from `internal/api/sheets.go` into `internal/sheet` so a plugin reads sheets the same way. Products are found with core's `products.(*Repository).FindForMenu`.

**Tech Stack:** Go (Echo, sqlx), `internal/sheet` (CSV / XLSX / ODS), React + TanStack Query, shadcn Dialog / DropdownMenu.

**Spec:** `docs/specs/2026-11-23-alternatives-sheet.md`

## Global Constraints

- Rule first (AGENTS.md): Task 1's rules are committed before any code; every test names its rule id.
- Core never names a plugin (`make check-core`). The core change is the generic sheet header helper only.
- Files ≤ 500 lines (Go and TS). Theme tokens only (`theme/guard.test.ts`). Admin screens work at 360px.
- Import is **admin only** (`middleware.RequireRole(auth.RoleAdmin)`, as `plugins/pocketcashier/api_phones.go:220`); export is admin + manager (`reg.Admin()`).
- No product, stock, money or product reorder level changes; only `alt_groups`, `alt_tiers`, `alt_members`.
- Shop-neutral wording and examples (USB drives, rice, cables), never pharmacy-only.
- Commits: no Claude / AI attribution. `make core-only` before each commit (the tree has every plugin on); stage named files only.
- Tests through `make test` (a Go package's own database), never a bare `go test` against `suite_db`.

## Review Focus

1. A sheet saved by Excel with a BOM, extra columns or `Group Name` / `Item` headers: still read (synonyms, BOM strip). Test in Task 2.
2. The same group typed twice with different case / spacing (`USB 32GB` vs `usb  32gb`): one group, the first spelling. Test in Task 4.
3. A product already in a hand-split tier staying put: its share is kept; a member leaving rescales (ALT-5). Test in Task 4.
4. Replace pressed after the file changed on disk: the previewed bytes are sent (as MenuSheet does). Test in Task 7 (Vitest).
5. Two imports at once: the second waits and then replaces (lock), never a mix. Test in Task 4.

---

### Task 1: Rules ALT-9 … ALT-18 in the rule book

**Files:**
- Modify: `docs/rules/plugins/alternatives.md` (grid row + rules)

- [ ] **Step 1: Add the grid row** `| The sheet (export, import) | ALT-9…18 | ALT-3 | ALT-6 | ALT-7 | PLG-4,5 |` and the code line (`sheet.go`, `api_sheet.go`).
- [ ] **Step 2: Write the rules**, each with `Tricky:` and `Checked by:` like MENU-1…16:
  - **ALT-9** Round trip: export then import unchanged gives the same groups, tiers, members, group / tier order, on / off, levels and splits. Tricky: a turned-off group; a disabled member; two tiers. An empty group or tier has no product line, so it doesn't survive (the preview lists it as removed).
  - **ALT-10** The sheet is the whole set: after Apply the groups are exactly what it says; a group, tier or member it doesn't name is gone.
  - **ALT-11** Preview changes nothing; Apply does exactly what the preview reported; Replace sends the previewed bytes and the screen says if Apply skipped a line the preview didn't.
  - **ALT-12** Apply is all or nothing; two imports at once never mix (the second waits).
  - **ALT-13** A line's product is found by barcode (any of its barcodes, or a pack's), then exact name (case and spacing ignored), never by id. Skipped and listed with line and reason: no match; a name two products share; a service; a disabled product not already in that tier; a line with no group; a product already placed by an earlier line in another group or tier ("already in <group> on line N"). The same product twice in one group and tier is one member. A product is never created.
  - **ALT-14** A sheet with no product lines is refused, never read as "remove every group".
  - **ALT-15** Groups match existing ones by name, tiers by name within their group (case and spacing ignored; blank tier = blank tier). A kept group keeps its id and on / off; a kept tier its level, Auto and on / off; a kept member its split share; a hand split that lost or gained members follows ALT-5. New groups and tiers start on, Auto level, Auto split. Groups and tiers take the sheet's order and the first spelling of their name.
  - **ALT-16** Only an admin imports; admin and manager export.
  - **ALT-17** Importing changes nothing outside Alternatives (products, stock, money, sales, product reorder levels); the auditor agrees before and after.
  - **ALT-18** A disabled product already in a tier stays, is exported, and is found again on import.
- [ ] **Step 3: Commit** `docs(rules): ALT-9…18 — the Alternatives sheet`

### Task 2: Core seam — sheet header mapping in `internal/sheet`

**Files:**
- Modify: `internal/sheet/sheet.go`, `internal/api/sheets.go:28-69`
- Test: `internal/sheet/sheet_test.go`

**Interfaces:**
- Produces: `sheet.Columns(header []string, synonyms map[string]string) map[string]int` (lower-cased, trimmed, BOM stripped, synonyms applied, first duplicate wins) and `sheet.Cell(col map[string]int, rec []string, key string) string` (trimmed, "" when absent).

- [ ] **Step 1: Failing test** `TestColumns`: header `["﻿Group Name", " Tier ", "Item", "Group"]` with synonyms `{"group name":"group","item":"product"}` → `{"group":0,"tier":1,"product":2}`; `TestCell`: missing key → `""`, `" x "` → `"x"`, short record → `""`.
- [ ] **Step 2: Run** `make test` (or `go test -exec bin/testdb ./internal/sheet/`) → FAIL (undefined).
- [ ] **Step 3: Implement** both in `internal/sheet/sheet.go`; `readImportSheet` and `cellGetter` in `internal/api/sheets.go` call them (behaviour unchanged).
- [ ] **Step 4: Run** `go test ./internal/sheet/ ./internal/api/ -run 'Columns|Cell|Menu|Import' -exec bin/testdb` → PASS.
- [ ] **Step 5: Commit** `refactor(sheet): header mapping in internal/sheet so plugins read sheets the same way`

### Task 3: Export

**Files:**
- Create: `plugins/alternatives/sheet.go` (types + `SheetLines`), `plugins/alternatives/api_sheet.go` (handlers)
- Modify: `plugins/alternatives/alternatives.go` (routes)
- Test: `plugins/alternatives/sheet_db_test.go`

**Interfaces:**
- Produces: `type SheetLine struct{ Line int; Group, Tier, Product, Barcode string }`; `func (s *Store) SheetLines(ctx context.Context) ([]SheetLine, error)` — every group (on and off) by `sort_order, lower(name), id`, tiers by `sort_order, id`, members by `lower(product name), product_id`; Barcode = `products.barcode` (main). Route `GET /api/v1/x/alternatives/sheet?format=csv|xlsx|ods` on `reg.Admin()`, filename `alternatives`, columns `["Group","Tier","Product","Barcode"]`, written with `sheet.FormatFrom` + `sheet.Write` like `internal/api/sheets.go:17`.

- [ ] **Step 1: Failing test** `TestSheetLinesOrder` (ALT-9 setup): two groups (one turned off), one with tiers `Genuine` / `Cheap`, a disabled member → lines in that order with tier names, the turned-off group present, the disabled member present (ALT-18).
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement.** **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `feat(alternatives): export the groups as a sheet (ALT-9, ALT-18)`

### Task 4: Rebuild (the import's core)

**Files:**
- Modify: `plugins/alternatives/sheet.go` (keep ≤ 500 lines; split `sheet_write.go` if needed)
- Test: `plugins/alternatives/sheet_db_test.go`

**Interfaces:**
- Consumes: `SheetLine` (Task 3); `rescaleSplit(ctx, tx, tierID)` (`rescale.go`).
- Produces:
  ```go
  type FindProduct func(ctx context.Context, barcode, name string) (id int64, reason string, err error)
  type Skipped struct{ Line int; Reason string }
  type SheetResult struct {
      Groups, Members                  int // in the new set
      GroupsAdded, GroupsRemoved       int
      Placed, Moved, TakenOut          int // members new to Alternatives / changed tier / gone
      Removed                          []string // names of groups removed
      Skipped                          []Skipped
  }
  func Rebuild(ctx context.Context, tx *sqlx.Tx, lines []SheetLine, find FindProduct) (SheetResult, error)
  ```

Approach (not determined by the signature): `LOCK TABLE alt_groups IN EXCLUSIVE MODE`; load the old groups / tiers / members (with `split_share`, `split_auto`); key = `lower(strings.Join(strings.Fields(s), " "))`. Walk lines in order building sheet groups → tiers → product ids (skips per ALT-13; `is_service` and `is_active` read from `products`; a disabled product is accepted only if it already sits in that matched tier). Drop groups / tiers left empty. Then write: match by key (old duplicates: the first by `sort_order, id` is kept, the rest removed); update kept groups' / tiers' name and `sort_order` (index in the sheet); insert new ones (`is_active` true, `level_auto` true, `split_auto` true, `reorder_level` 0); delete unmatched tiers and groups; members: delete those not in the sheet, `INSERT … ON CONFLICT (product_id) DO UPDATE SET tier_id = EXCLUDED.tier_id, split_share = NULL` only for those whose tier changed or are new; then `rescaleSplit` every hand-split tier that lost or gained a member. Empty `lines` → `apperr.BadRequest("the sheet has no product lines")` (ALT-14).

- [ ] **Step 1: Failing tests** in `sheet_db_test.go`, a fake `FindProduct` by name over the test's own tagged products, each test named for its rule:
  - `TestSheetRoundTrip_ALT9`: build groups (tiers, a typed level 7, a hand split 60/40, a turned-off group), `SheetLines` → `Rebuild` → same `SheetLines`, same ids, level 7, shares 60/40, still off.
  - `TestSheetWhole_ALT10`: a group not in the sheet is removed (`Removed` names it); a member dropped from a kept group is `TakenOut`.
  - `TestSheetFind_ALT13`: lines for an unknown name, a service, a disabled product new to the tier, a product on two groups (second skipped "already in A on line 2"), the same product twice in one tier (one member, no skip) → `Skipped` with those line numbers and reasons.
  - `TestSheetEmpty_ALT14`: no lines → error, groups unchanged.
  - `TestSheetKeeps_ALT15`: `usb  32GB` matches `USB 32GB` (kept id, sheet's spelling); a moved member leaves a 60/40 split → the rest rescaled to 100; a new tier starts Auto.
  - `TestSheetTwoAtOnce_ALT12`: two goroutines, two transactions, different sheets → the final state is exactly one sheet, never a mix.
- [ ] **Step 2: Run** `make test` (package `plugins/alternatives`) → FAIL.
- [ ] **Step 3: Implement `Rebuild`.**
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `feat(alternatives): rebuild the groups from a sheet (ALT-10, 12–15)`

### Task 5: Import route (preview and apply)

**Files:**
- Modify: `plugins/alternatives/api_sheet.go`, `plugins/alternatives/alternatives.go`, `plugins/alternatives/api.go` (DTO)
- Test: `plugins/alternatives/api_test.go`

**Interfaces:**
- Consumes: `sheet.Columns`, `sheet.Cell` (Task 2); `Rebuild` (Task 4); `products.NewRepository(p.core.DB).FindForMenu` as the `FindProduct`.
- Produces: `POST /api/v1/x/alternatives/sheet/import[?dry=1]` (multipart `file`, admin only), answering
  `type SheetImportDTO struct { Groups, Members, GroupsAdded, GroupsRemoved, Placed, Moved, TakenOut int; Removed []string; Skipped []string; Applied bool }` (JSON camelCase; `Skipped` as `"line N: reason"`, never null). Synonyms: `group name`/`alternative`→group, `quality`/`level`→tier, `item`/`name`→product, `code`/`sku`→barcode; required column `group`. Preview = `Rebuild` in `appdb.WithTx` returning a sentinel error to roll back (as `admin_data_menu.go:64-77`). One audit entry on Apply: `alternatives replaced from a sheet: N groups, M products`.

- [ ] **Step 1: Failing tests**: `TestSheetImportManagerRefused_ALT16` (403 for a manager, export 200 for a manager); `TestSheetPreviewWritesNothing_ALT11` (dry=1 → counts, tables unchanged; then apply → same counts, `applied: true`); `TestSheetNoLines_ALT14` (header only → 400).
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement.** **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `feat(alternatives): import the sheet with a preview (ALT-11, 14, 16)`

### Task 6: Battle cases

**Files:**
- Modify: `plugins/alternatives/battle/battle_db_test.go` (or a new `cases_sheet_db_test.go` beside it)

- [ ] **Step 1: Cases on the real server**, one per rule, through the HTTP routes as admin / manager: ALT-9 (export → import the exported bytes → export again: byte-equal), ALT-10, ALT-11, ALT-13 (a pack's barcode finds its product), ALT-16, ALT-17 (`ltest.Check` and `ltest.MoneyAndStock` unchanged across an import), ALT-18.
- [ ] **Step 2: Run** `make test` for the battle package → PASS (they pass against Tasks 3–5; if one fails, fix the code, never the case).
- [ ] **Step 3: Commit** `test(alternatives): battle cases for the sheet (ALT-9…18)`

### Task 7: Sheet ▾ on the Alternatives page

**Files:**
- Create: `web/src/plugins/alternatives/SheetMenu.tsx` (Export ▸ CSV / Excel / ODS, Import…, the preview dialog)
- Modify: `web/src/plugins/alternatives/AlternativesPage.tsx` (replace the CSV button and `download`), `web/src/plugins/alternatives/labels.ts` + `labels.test.ts` (drop `groupsCsv`), `web/src/plugins/alternatives/types.ts` (`SheetImport` mirroring `SheetImportDTO`)
- Test: `web/src/plugins/alternatives/SheetMenu.test.tsx`

**Interfaces:**
- Consumes: Task 5's route and DTO; `apiUpload`, `ExportMenu` (`@/features/reports/kit/ExportMenu`), `meQuery` (Import shown only to an admin).

Copy (fixed): dialog title **Replace your alternatives?**; description **Your groups become exactly what this sheet says. Products, stock and sales don't change.**; lines `N groups and M products in the new set.`, `X new groups, Y removed` (removed names listed), `P products placed, Q moved, R taken out`, `K lines skipped` (scroll list); buttons **Cancel** / **Replace**; toast `Alternatives replaced: N groups, M products`; toast warning for lines skipped that the preview didn't show (as `MenuSheet.tsx:41`).

- [ ] **Step 1: Failing Vitest** `SheetMenu.test.tsx`: the previewed bytes are what Replace sends even after the picked File changes (ALT-11); a manager sees Export but not Import (ALT-16); removed group names are listed.
- [ ] **Step 2: Run** `cd web && bunx vitest run src/plugins/alternatives` → FAIL. **Step 3: Implement.** **Step 4: Run** → PASS; `bunx tsc -b` clean.
- [ ] **Step 5: Screenshots** of the page and the preview at 360px and desktop, Studio + Night, light + dark; review them yourself (card edges, long names wrapping).
- [ ] **Step 6: Commit** `feat(alternatives): Sheet ▾ — export and import with a preview`

### Task 8: Playwright, docs, screenshot

**Files:**
- Modify: `web/e2e/plugin-alternatives.spec.ts`, `docs/guide/plugins/alternatives.md`, `web/docs-shots/shots.spec.ts` (+ `web/e2e/shot-prep-alternatives.ts` if the preview needs data), `docs/images/plugins/…`

- [ ] **Step 1: Playwright test** `the sheet: export, edit, import, preview, Replace`: export CSV, change one line's group, add a line with an unknown product, import → the preview shows 1 moved and 1 skipped line → Replace → the page shows the new group. Add a 360px check of the dialog. Run `cd web && bunx playwright test e2e/plugin-alternatives.spec.ts` → PASS.
- [ ] **Step 2: Guide**: a "Sheet" section in `alternatives.md` — export, edit, import with the preview, the whole-set rule, and "group with any AI: export Data → Products with its fields, ask the AI for a sheet with columns Group, Tier, Product, Barcode, import it here".
- [ ] **Step 3: Screenshot**: add the preview shot to `shots.spec.ts`, `make docs-shots`, look at the result.
- [ ] **Step 4: Commit** `docs(alternatives): the sheet — guide and screenshot`

### Task 9: Whole run and handoff

- [ ] **Step 1:** `make core-only`, `make check-core`, `make test-all` (detached, logs on disk) → all green.
- [ ] **Step 2:** `make mutate` on `plugins/alternatives/sheet*.go`; read the survivors, close real gaps test-first.
- [ ] **Step 3:** Update `docs/status.md` (Now / Next / Done); delete this plan and the spec; commit `docs(status): the Alternatives sheet is done`.
- [ ] **Step 4:** Then (separate work, catalogue repo): Iman's `alternatives.csv` beside the iman-reset files.
