# Cashier Menu Sheet Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Export the whole cashier menu (Admin → Groups) as a sheet and import it back, where the sheet becomes the whole menu, with an exact preview first — on Data → Spreadsheets.

**Architecture:** The groups package gets a `Rebuild` that, inside one transaction, deletes every group and writes the sheet's groups and items fresh; a dry run is the same call in a transaction that is rolled back, so the preview's numbers are Apply's. Products are found by a new products-repo lookup (barcode, then exact name, never id). Two admin routes under `/api/v1/admin/data/menu/…`; a **Cashier menu** row on the Data page opens a confirm dialog with the preview.

**Tech Stack:** Go (Echo, sqlx, Postgres 17), the existing sheet helpers (`internal/sheet`, `writeSheet`, `readImportSheet`, `cellGetter`), React + TS (`apiUpload`, `ExportMenu`, shadcn `Dialog`), battle tests (`internal/battle`), Playwright.

**Spec:** `docs/specs/2026-11-23-cashier-menu-sheet.md`

## Global Constraints

- The sheet is the whole menu: after Apply the menu is exactly the sheet; groups and items not in it are gone.
- Apply does exactly what the preview showed (skipped lines included) or nothing at all.
- Columns, in this order: `Group`, `Emoji`, `Active`, `Product`, `Barcode`, `Product emoji`. Group path separator `>` (spaces around it ignored), like the products sheet's category.
- Row order is till order (siblings' `sort_order` = order of first appearance; items' `sort_order` = row order in the group).
- Products: barcode first (any barcode or a pack's), then exact name (case and spacing ignored), never by id; no match or a name two products share → skipped with its line. Never create products.
- An empty sheet (no data rows) is refused: `the sheet has no menu lines`.
- Group name ≤ 80, emoji ≤ 16 (the columns); a longer cell → that line skipped, never cut.
- Admin only (the Data group). Nothing outside `product_groups` / `product_group_items` changes.
- The confirm is a dialog, never a toast. Every screen works at 360 px, every palette, light and dark; theme tokens only.
- Files ≤ 500 lines. Rule first (`docs/rules/`), failing test, then code. No AI attribution in commits.

## Review Focus

- A sheet AI rewrote with a header in another case or order (`group`, `PRODUCT`, columns shuffled) → still read (header lookup, not position). Test in Task 3.
- A group named only on product rows, never on its own row, nested two deep (`A > B > C`) → all three levels made, in first-appearance order. Test in Task 2.
- The same group path written with different spacing (`Drinks>Soft` vs `Drinks > Soft`) → one group. Test in Task 2.
- A product disabled in the shop but in the menu → export writes it and import finds it again (round trip stays exact). Test in Task 2.
- A blank line in the middle of the sheet (all cells empty) → ignored, not a skipped line, not a group named "". Test in Task 3.

---

### Task 1: Rule book — the MENU area

**Files:**
- Create: `docs/rules/menu.md`
- Modify: `docs/rules/README.md` (the ids line: add `MENU` the cashier menu sheet)

- [ ] **Step 1: Write `docs/rules/menu.md`**: the grid (rows: group · sub-group · hidden group · group emoji · order · product in a group · product emoji · same product in two groups; columns: export · import · import with a missing / shared-name product · product merged after · product disabled after · setup-only backup + restore), every square a rule id or `—` with its reason. Rules, each in the README's format (`Tricky:` / `Checked by:`):
  - MENU-1 export → import gives back the same menu (names, emoji, Active, order, items, item emoji).
  - MENU-2 the sheet is the whole menu: what it doesn't name is gone after Apply.
  - MENU-3 a dry run changes nothing and reports exactly what Apply then does.
  - MENU-4 Apply is all or nothing.
  - MENU-5 a product is found by barcode (any of its barcodes or a pack's), then by exact name; never by id; no match or a shared name → that line skipped and listed; products are never created.
  - MENU-6 an empty sheet is refused.
  - MENU-7 row order is till order.
  - MENU-8 a hidden group stays hidden through a round trip.
  - MENU-9 a product twice in the same group is one item (the first row's emoji).
  - MENU-10 a cell longer than its column skips that line.
  - MENU-11 only an admin can export or import.
  - MENU-12 nothing outside the menu changes (products, categories, stock, money: the auditor agrees before and after).
  - MENU-13 a merged product's items move to the product it merged into (existing merge, `merge_tables.go`); a disabled product stays in the menu and round-trips.
- [ ] **Step 2: Commit** — `docs(rules): MENU — the cashier menu sheet (export, import, the sheet is the menu)`

### Task 2: `Rebuild` and the menu lines (groups package)

**Files:**
- Create: `internal/features/productgroups/menu.go`
- Create: `internal/features/productgroups/menu_db_test.go`
- Modify: `internal/features/products/products.go` (add `FindForMenu`)

**Interfaces:**
- Produces:
  - `type MenuLine struct { Line int; Group, Emoji, Active, Product, Barcode, ProductEmoji string }` (`Line` = the sheet's line number, header = 1)
  - `type Skipped struct { Line int; Reason string }`
  - `type MenuResult struct { Groups, Items, RemovedGroups, RemovedItems int; Skipped []Skipped }` (`Removed*` = how many of the old groups / items are not in the new menu, compared by group path and by path + product id)
  - `type FindProduct func(ctx context.Context, barcode, name string) (id int64, reason string, err error)` (`reason` ≠ "" = skip the line with it)
  - `func (r *Repository) MenuLines(ctx context.Context) ([]MenuLine, error)` — export order: depth-first by `sort_order, id`; each group as a group row, then its items by `sort_order`; inactive groups and disabled products included; `Active` = `"no"` for a hidden group, `""` otherwise
  - `func Rebuild(ctx context.Context, tx *sqlx.Tx, lines []MenuLine, find FindProduct) (MenuResult, error)` — counts the old menu, `DELETE FROM product_groups`, writes the new one
  - `func (r *products.Repository) FindForMenu(ctx context.Context, barcode, name string) (int64, string, error)` — `barcodeIs` match ignoring `is_active`; else `lower(regexp_replace(trim(name), '\s+', ' ', 'g'))` equal; 0 rows → `"no product with this barcode or name"`; 2+ by name → `"two products have this name"`

- [ ] **Step 1: Write the failing tests** (`ltest.NewDB(t)`, products made with plain inserts):
  - `TestRebuildRoundTrip` (MENU-1, 7, 8): build a menu with the existing repo calls (2 top groups, one hidden; a sub-group; 3 items with emoji, one a disabled product) → `MenuLines` → `Rebuild` with those lines → `MenuLines` again equals the first (ignoring `Line`).
  - `TestRebuildReplacesWholeMenu` (MENU-2): old menu has groups `Old`, `Keep`; lines name only `Keep` + a new `New` → after: groups exactly `Keep`, `New`; `RemovedGroups == 1`.
  - `TestRebuildMakesGroupsFromProductRows`: lines with only product rows under `A > B > C` and `A>B` → groups A, A>B, A>B>C (one `A>B`), order of first appearance.
  - `TestRebuildSkips` (MENU-5, 9, 10): a finder returning a reason for one line, the same product twice in one group, a 81-char group name, a 17-char emoji → `Skipped` has those three lines with reasons; the duplicate is one item carrying the first row's emoji.
  - `TestRebuildAllOrNothing` (MENU-4): a finder that returns an error on the third line → `Rebuild` returns the error; caller rolls back; `MenuLines` equals the old menu.
  - `TestFindForMenu` (MENU-5): by a pack's barcode; by name with different case and double spaces; a disabled product found; two products sharing a name → the shared-name reason.
- [ ] **Step 2: Run, see them fail** — `make test` filtered: `go build -o bin/testdb ./scripts/testdb && ./bin/testdb template >/dev/null && go test -exec "$PWD/bin/testdb exec" ./internal/features/productgroups/ ./internal/features/products/ -run 'Rebuild|FindForMenu'` → FAIL (undefined: `Rebuild`, `FindForMenu`).
- [ ] **Step 3: Implement** `menu.go` and `FindForMenu`. Group key = the path's parts trimmed and joined with `" > "`; parent created before child; a group row sets emoji / Active; `Line` with every cell empty is ignored.
- [ ] **Step 4: Run, see them pass** (same command) → PASS.
- [ ] **Step 5: Commit** — `feat(groups): rebuild the cashier menu from sheet lines in one transaction (MENU-1…10)`

### Task 3: The admin routes

**Files:**
- Create: `internal/api/admin_data_menu.go`
- Modify: `internal/api/dto_data.go` (`MenuImportResult`), `internal/api/admin_data.go:23-29` (routes), then `make types`
- Test: `internal/api/admin_data_menu_test.go`

**Interfaces:**
- Consumes: Task 2's `MenuLines`, `Rebuild`, `FindForMenu`; `writeSheet`, `readImportSheet`, `cellGetter`, `db.WithTx`.
- Produces:
  - `GET /api/v1/admin/data/menu/export` (`?format=csv|xlsx|ods`, file `cashier-menu.<ext>`)
  - `POST /api/v1/admin/data/menu/import` (multipart `file`; `?dry=1` = preview, rolled back) → `MenuImportResult`
  - `type MenuImportResult struct { Groups, Items, RemovedGroups, RemovedItems int; Skipped []string; Applied bool }` (json camelCase; `Skipped` as `"line 7: two products have this name"`)
  - header synonyms: `category`/`menu`/`group path` → `group`; `item`/`name` → `product`; `code`/`sku` → `barcode`; `shown`/`visible` → `active`; `item emoji` → `product emoji`. Required column: `group`.

- [ ] **Step 1: Write the failing tests**: `TestAdminMenuSheet` — manager gets 403 on both routes (MENU-11); a header-only CSV → 400 `the sheet has no menu lines` (MENU-6); a CSV with shuffled, upper-case headers and a blank middle line, `?dry=1` → 200 with the expected counts and `Applied == false`, and `SELECT count(*) FROM product_groups` unchanged (MENU-3); export returns `Content-Disposition` with `cashier-menu.csv` and the header row exactly `Group,Emoji,Active,Product,Barcode,Product emoji`. (Apply is proven on a battle shop in Task 4: it would wipe this package's shared menu.)
- [ ] **Step 2: Run, see it fail** — `go test -exec "$PWD/bin/testdb exec" ./internal/api/ -run TestAdminMenuSheet` → FAIL (404).
- [ ] **Step 3: Implement** the handlers; the import runs `Rebuild` inside `db.WithTx`, and for `dry=1` returns a sentinel error after building the result so the transaction rolls back; audit Apply like the settings import.
- [ ] **Step 4: Run, see it pass**; `make types` → `MenuImportResult` in `web/src/api/types.gen.ts`.
- [ ] **Step 5: Commit** — `feat(data): cashier menu sheet routes — export, preview (dry run), replace (MENU-3, 6, 11)`

### Task 4: Battle cases on a real shop

**Files:**
- Create: `internal/battle/cases_menu_test.go`
- Modify: `internal/battle/cases_stock_test.go:13-35` (`upload` decodes the reply into `out` when not nil, like `Do`)

- [ ] **Step 1: Write the failing tests** (`t.Parallel()`, `newShop(t)`, admin client):
  - `TestMENU2ReplaceOnARealShop`: make a menu through the Groups routes; export CSV; edit it (drop one group, add one, reorder two rows); `?dry=1` → result R; Apply → result equals R with `Applied`; the Groups routes now show exactly the edited menu in the edited order (MENU-2, 3, 7).
  - `TestMENU5AnotherShopsMenu`: export from shop A, import into shop B that has the same products made in a different order (different ids) plus one product missing → items land on B's products; the missing one is in `Skipped` (MENU-5).
  - `TestMENU12NothingElseMoves`: sell something, then Apply a new menu → `ltest.Check` passes and product, stock and money row counts / totals are equal before and after (MENU-12).
  - `TestMENU13MergeAndDisable`: a product in the menu merged into another → its item is on the survivor; a disabled one → still exported and re-imported (MENU-13).
- [ ] **Step 2: Run, see them fail** before the `upload` change — `go test -exec "$PWD/bin/testdb exec" ./internal/battle/ -run TestMENU -short`.
- [ ] **Step 3: Fix `upload`**; anything else failing is a real bug in Tasks 2–3: fix it there, test first.
- [ ] **Step 4: Run, see them pass.**
- [ ] **Step 5: Commit** — `test(battle): MENU cases on real shops — replace, another shop's menu, nothing else moves, merge and disable`

### Task 5: The Data page row and its dialog

**Files:**
- Create: `web/src/features/data/MenuSheet.tsx`
- Modify: `web/src/features/data/DataPage.tsx` (render `<MenuSheet />` under the Settings block inside `Sheets`)
- Test: `web/e2e/admin-data.spec.ts` (new test)

**Interfaces:**
- Consumes: `MenuImportResult`; `apiUpload<T>(path, field, file, extra)`; `ExportMenu url=…`; `Dialog` from `@/components/ui/dialog`.
- Produces: `export function MenuSheet(): JSX.Element`.

- [ ] **Step 1: Write the failing e2e test** `cashier menu sheet: preview, cancel, replace` (import `test` from `e2e/fixtures.ts`): as admin, make two groups via the API; on `/admin/data` the **Cashier menu** row is visible; upload a CSV naming one group and one missing product → the dialog shows `1 group`, `1 group removed`, `1 line skipped` and the skipped line text; **Cancel** → the API still lists both groups; upload again → **Replace the menu** → the API lists one group and a success toast shows.
- [ ] **Step 2: Run, see it fail** — `cd web && bunx playwright test e2e/admin-data.spec.ts -g "cashier menu"` (stop `make dev` first).
- [ ] **Step 3: Implement `MenuSheet`**: the row as the Settings block (title **Cashier menu**, hint `Build or rework the till's menu in a spreadsheet. Importing replaces the whole menu — you'll see what changes first.`, `ExportMenu url="/api/v1/admin/data/menu/export"`, Import button); on file → `apiUpload(…'/admin/data/menu/import', 'file', f, {})` with `?dry=1` → dialog: the four numbers, the skipped lines (scrolls, wraps at 360 px), **Replace the menu** (re-uploads the same file without `dry`, then toast `Menu replaced: N groups, M items`, invalidate queries) and **Cancel**. Theme tokens only.
- [ ] **Step 4: Run, see it pass**; `cd web && bunx tsc -b`; `make test-web`.
- [ ] **Step 5: UI gate** — screenshots of the dialog at 360 px and desktop in Studio + Night, light and dark; look at them yourself (card edges, long skipped lines).
- [ ] **Step 6: Commit** — `feat(data): Cashier menu row on Data → Spreadsheets — preview what changes, then replace`

### Task 6: Docs, status, done

**Files:**
- Modify: `docs/guide/updates-backups.md` (a **Cashier menu sheet** section: the columns, "the sheet is the whole menu", products matched by barcode / name, skipped lines, the preview; the new-shop workflow: products sheet first, then the menu)
- Modify: `docs/status.md` (Done list + the preview-shop workflow line)
- Delete: `docs/specs/2026-11-23-cashier-menu-sheet.md`, this plan

- [ ] **Step 1: Write the guide section and status lines**; retake `install/data` (`make docs-shots` with a Demo Room) so the new row shows.
- [ ] **Step 2: Full run** — `make test-all` (detached, log in `~/.cache/karots-logs`) → green; `make check-core`.
- [ ] **Step 3: Review the whole milestone** against the spec; fix what's found test-first.
- [ ] **Step 4: Commit** — `docs: the cashier menu sheet in the guide and status; spec and plan done`
