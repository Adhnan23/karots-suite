# AI Assistant jobs, hints, Alternatives levels, packs on create: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Someone who knows nothing about the stock can still find and sell the right item: AI details become searchable hints, whole-list and plugin-aware AI jobs organise every product, Alternatives learns its levels, and packs can be set while creating a product.

**Architecture:** `plugins/aicatalog` owns a job framework (per-job, per-product record; prompt → preview → tick → apply → undo). Other plugins expose data through generic hooks in `internal/plugin` that never mention AI; a job appears when its hook is registered. Core stays plugin-inert: reorder pages render generic group fields from `ReorderNote`.

**Tech Stack:** Go, Echo, sqlx, goose, Postgres 17; React, TypeScript, TanStack Query, Tailwind tokens; Vitest, Playwright.

**Spec:** `docs/specs/2026-11-05-ai-assistant-and-alternatives-design.md`

## Global Constraints

- Core Go and `web/src` outside `web/src/plugins` never name a plugin (`make check-core`). Other plugins' code never mentions AI.
- AI Assistant never imports another plugin (a Go import pulls it into every build) and never writes another plugin's tables.
- Every job: suggestion → tick → apply. Fills empty values only; a redo shows old and new, and the old stays unless ticked. Prices and stock never change.
- Money and till untouched; nothing here can stop a sale.
- Prompts assume no kind of shop, except jobs that exist only with a plugin. Country stays Sri Lanka. Every prompt goes through `withCustom`. No "cashier's hint" in prompts.
- Theme tokens only, admin pages work at 360px, files ≤ 500 lines, `make types` after DTO changes.
- Next aicatalog migration `0009_`, next alternatives migration `0002_`; no core migration needed.
- Commits: one per step's end, on `main`, **no Claude or AI attribution**. Build through `make`. Never run `go test` during Playwright. `make core-only` before committing.
- No Release without the user's say. Hand tests leave no rows behind.

## Review Focus

1. **A chatbot reply naming a product outside the batch, or a field outside a product's scope:** dropped, never applied (Tasks 4, 9).
2. **A preview applied after the product changed, merged or was disabled:** that row is skipped and reported, the rest apply (Task 4).
3. **Undo after someone edited the value by hand:** the newer edit stays; undo only reverts values still as applied (Task 4).
4. **Hint text with `%`, `_`, Sinhala, HTML or thousands of characters:** searched literally, shown as plain text, capped at 2000 characters (Task 3).
5. **A group whose members have no sales, are all disabled, or whose order doesn't divide evenly:** even split, no crash, and shares add up exactly to the total (Task 7).

---

### Task 1: Packs and extra barcodes while creating a product

**Files:**
- Modify: `internal/api/dto_catalog.go` (`ProductRequest`), `internal/api/admin_products.go:142`, `internal/features/products/packs.go`, `web/src/features/admin/products/ProductForm.tsx:179-181`
- Create: `web/src/features/admin/products/NewPacks.tsx`
- Test: `internal/api/admin_products_create_test.go`, `web/e2e/admin-catalog.spec.ts`

**Interfaces:**
- Produces: `ProductRequest.Packs []PackRequest` and `ProductRequest.Barcodes []string` (`json:"packs,omitempty"`, `json:"barcodes,omitempty"`), honoured on create only; `func (s *Service) CheckPack(in PackInput) error` holding `SavePack`'s validation.

- [x] **Step 1: Write the failing tests** in `admin_products_create_test.go`:
  - `TestCreateWithPacksAndBarcodes`: POST with `packs:[{name:"Card",size:"10"}]`, `barcodes:["X1"]`; `GET /products/:id/packs` has one Card, and the barcode resolves to the product.
  - `TestCreateRefusesBadPackBeforeWriting`: size `"1"` gives 422 and no product named that exists.
  - `TestCreatePackOnDecimalUnitRefused`: a unit with `allow_decimal` gives 422 and no product.
  - `TestCreateBarcodeTakenKeepsProduct`: a barcode taken by another product gives 201 with `Message` naming the barcode, and the product exists.
- [x] **Step 2: Run** `go test ./internal/api -run TestCreate` (with `DATABASE_URL`). Expected: FAIL.
- [x] **Step 3: Implement.** Move validation into `CheckPack`, called by `SavePack`. In `AdminProductCreate`, check packs before `Create`, then after it call `SavePack` / `AddBarcode(…, "form")`; a failure after the product exists goes in `ProductSaved.Message`, never rolling back the product. `make types`.
- [x] **Step 4: Form.** `NewPacks.tsx` keeps a local list using `PacksBlock`'s fields and `NAMES`, plus a local extra-barcode list. Show it when `!product && !svc && !allowDecimal` and put both lists in the POST body. A duplicate (`copyOf`) starts empty.
- [x] **Step 5: e2e:** create a product with Card × 10 in the form, then reopen it and see the pack. Add to `admin-catalog.spec.ts`.
- [x] **Step 6: Run** `go test ./internal/api` and that spec. Expected: PASS. Update `docs/guide/stock.md`.
- [x] **Step 7: Commit** `feat(products): set packs and extra barcodes while creating a product`

### Task 2: Replies survive real chatbots; prompts fit any shop

**Files:**
- Create: `plugins/aicatalog/repair.go`, `plugins/aicatalog/parse_real_test.go`
- Modify: `plugins/aicatalog/parse.go`, `plugins/aicatalog/prompts.go:13-36`

**Interfaces:**
- Produces: `func decodeReply(raw string, out any) error` (cuts with `jsonObject`, tries `json.Unmarshal`, then `repairQuotes`, else `errNoAnswer`); `func repairQuotes(s string) string`.

- [x] **Step 1: Write the failing tests** using the real replies from the spec:
  - `TestRepairInchMarks`: Gemini's `"specs":"Thread diameter: 10mm; Reach: 12.7mm (1/2"); Hex size: 16mm (5/8"); Type: …"` parses, and `Specs` contains `(1/2")`.
  - `TestRepairLeavesValidJSON`: valid JSON comes back byte-identical.
  - `TestRepairGivesUp`: `{"name":` returns `errNoAnswer`.
  - `TestSystemPromptAnyShop`: `IdentifyPrompt` doesn't contain `spare-part`, `Bearings`, `Automotive` or `cashier's hint`; it does contain the custom prompt when set.
- [x] **Step 2: Run** `go test ./plugins/aicatalog -run 'Repair|AnyShop'`. Expected: FAIL.
- [x] **Step 3: Implement.** `repairQuotes` scans strings: a `"` inside a string ends it only when the next non-space character is `,`, `}`, `]` or `:`; otherwise it is escaped. `parseIdentify` and `parseCleanup` use `decodeReply`. Prompt: "products of any kind of shop"; `specs` = "what a customer would ask to be sure it's the right item (what it fits or works with, size or strength, type, ingredients, what it's for)"; a neutral shallow-category example. Keep the JSON shape and the shallow rule.
- [x] **Step 4: Run** `go test ./plugins/aicatalog`. Expected: PASS (fix any test pinning old wording, without weakening it).
- [x] **Step 5: Commit** `fix(aicatalog): read replies with stray inch marks; prompts fit any shop`

### Task 3: Hints kept, shown, searchable, correctable

**Files:**
- Create: `plugins/aicatalog/migrations/0009_jobs.sql`, `plugins/aicatalog/hints.go`, `plugins/aicatalog/hints_db_test.go`, `web/src/plugins/aicatalog/HintSection.tsx`
- Modify: `plugins/aicatalog/hooks.go:54-65`, `plugins/aicatalog/api.go`, `web/src/plugins/aicatalog/index.tsx`, `internal/plugin/hooks_product.go`, `internal/app/plugins.go`, `internal/features/products` (till line), `internal/api/dto_till.go`, `internal/api/till_catalog.go`, `web/src/features/till/catalog/ProductTile.tsx`, `web/e2e/plugin-aicatalog.spec.ts`

**Interfaces:**
- Produces, SQL: `aicatalog_items.old_names TEXT NOT NULL DEFAULT ''`; GIN trigram index on `specs || ' ' || user_explanation || ' ' || old_names`. `aicatalog_job_items(job TEXT, product_id BIGINT REFERENCES products ON DELETE CASCADE, state TEXT CHECK (state IN ('applied','skipped','no_answer')), updated_at TIMESTAMPTZ DEFAULT now(), PRIMARY KEY (job, product_id))`, added to `dataGroup.Tables`.
- Produces, Go: `func (s *Store) MatchHints(ctx, q string) ([]int64, error)`; `func (s *Store) HintLines(ctx, ids []int64) (map[int64]string, error)`; `func (s *Store) SaveHint(ctx, id int64, specs, explanation string) error`; `func (s *Store) AddOldName(ctx, q appdb.Queryer, id int64, name string) error`; `const maxHint = 2000`.
- Produces, generic hook: `plugin.ProductTillLineProvider{Batch func(ctx, ids []int64) (map[int64]string, error)}`, `reg.AddProductTillLineProvider`, `TillProduct.Line string` (`json:"line"`). Best-effort: an error shows no line.
- Produces, route: `PUT /api/v1/x/aicatalog/items/:id` `{specs, explanation}`, `GET` the same.

- [x] **Step 1: Write the failing tests** in `hints_db_test.go`:
  - `TestMatchHints`: specs `Fits CT100` is found by `ct100` and by `100 ct` (every word must match, any order); not by `xyz`.
  - `TestMatchHintsLiteral`: `%` and `_` match only those characters.
  - `TestMatchOldName`: `AddOldName(…,"6201 ZZ")` is found by `6201`.
  - `TestSaveHintCaps`: 5000 runes of Sinhala saves 2000 runes, on a rune boundary.
  - `TestDetailRows`: rows are `AI hint` with specs and `What it is` with explanation; empty ones are omitted; `Note` row reads `AI suggestion, confirm before selling`.
  - `TestMergeKeepsJobItems`: merging moves job rows and old names; kept product's row wins.
  - Restore: extend `restore_db_test.go` so a backup without `aicatalog_job_items` still restores.
- [x] **Step 2: Run** `go test ./plugins/aicatalog -run 'Hint|Merge|Detail'`. Expected: FAIL.
- [x] **Step 3: Implement** the migration and store. Register `AddProductSearchContributor{Match: MatchHints}` and the till-line provider. Escape `\ % _` for `ILIKE`; split on whitespace. `HintLines` gives the first 80 runes of specs (else explanation). Extend `mergeProduct`.
- [x] **Step 4: Core seam.** Add the hook, wire it like `MetaProvider` in `internal/app/plugins.go`, fill `TillProduct.Line` in the till catalogue handlers, `make types`, render one truncated muted line on `ProductTile` when non-empty. Test in `internal/api` with a fake provider.
- [x] **Step 5: `HintSection.tsx`:** a `productFormSections` section, shown when `productId > 0`: two text boxes saved through the route, plain text, label **AI hint**.
- [x] **Step 6: e2e** in `plugin-aicatalog.spec.ts`: pick an Identify answer with specs `Fits CT100`, save, search `CT100` at the till, the tile shows and ⓘ shows the hint. Screenshot the till at 1280 in Studio and Night, light and dark; review it.
- [x] **Step 7: Run** `make test`-scoped packages and the spec, `make check-core`. Expected: PASS.
- [x] **Step 8: Commit** `feat(aicatalog): searchable AI hints at the till and on the product page`

### Task 4: Job framework

**Files:**
- Create: `plugins/aicatalog/jobs.go`, `plugins/aicatalog/jobs_apply.go`, `plugins/aicatalog/jobs_db_test.go`, `web/src/plugins/aicatalog/JobPage.tsx`, `web/src/plugins/aicatalog/jobs.ts`, `web/src/plugins/aicatalog/jobs.test.ts`
- Modify: `plugins/aicatalog/cleanup.go` (routes), `plugins/aicatalog/apply.go:354` (`revertOp`), `web/src/plugins/aicatalog/AssistantPage.tsx`, `api.ts`, `types.ts`

**Interfaces:**
- Produces, Go:
  ```go
  type Job struct {
      Key, Label, Desc string
      Warning  string // shown on the page
      OneByOne bool   // no tick-all
      Available func(*Plugin) bool
      Prompt  func(ctx context.Context, p *Plugin, b Batch) (string, error)
      Parse   func(ctx context.Context, p *Plugin, b Batch, reply string) ([]JobRow, error)
      Apply   func(ctx context.Context, p *Plugin, tx *sqlx.Tx, r JobRow) ([]UndoOp, error)
  }
  type Batch struct{ Products []JobProduct; Options json.RawMessage; Settings Settings }
  type JobProduct struct{ ID int64; Name, Category, Hint string; CategoryID int64 }
  type JobRow struct {
      ProductID int64; Kind, Key, Label, Old, New, Reason string
      IsNew, Change, Ticked bool
      Extra map[string]string
  }
  type UndoOp struct{ Kind string; ProductID int64; Key, Applied, Previous string }
  var jobs []Job // registered in order
  ```
- Produces, routes (admin): `GET /aicatalog/jobs`; `POST /aicatalog/jobs/:job/next` `{size, categoryId, filter, ids, options, reply}` answers like `cleanup` (prompt in manual mode, preview otherwise) with `remaining`; `POST /aicatalog/jobs/:job/apply` `{runId, rows, answered}`; undo uses the existing `/aicatalog/cleanup/undo`.
- Consumes: `aicatalog_job_items` (Task 3), `PastePanel`, `useAsk`, `decodeReply`.

- [x] **Step 1: Write the failing tests** in `jobs_db_test.go`, with a fake job:
  - `TestNextSkipsAnswered`: of 3 products, an `applied` and a `skipped` one are not offered; `filter:"skipped"` returns them.
  - `TestRedoIDs`: `ids` offers those products whatever their state.
  - `TestParseDropsForeignIDs`: a reply naming a product outside the batch yields no row.
  - `TestApplyMarksAnswered`: ticked row → `applied`; unticked → `skipped`; asked with no row → `no_answer`.
  - `TestApplySkipsChanged`: a row whose product is now disabled, merged, or whose `Old` no longer matches is reported skipped and the rest apply.
  - `TestUndoLeavesNewerEdit`: undo reverts only when the current value equals `Applied`.
  - `TestUnavailableJob404`: `Available` false gives 404 and it isn't listed.
- [x] **Step 2: Run** `go test ./plugins/aicatalog -run 'Next|Redo|Parse|Apply|Undo|Unavailable'`. Expected: FAIL.
- [x] **Step 3: Implement.** Apply runs in one transaction and writes one `aicatalog_optimize_runs` row; a new `revertOp` kind `job` handles `UndoOp`. Size clamps to 1..50, default 10. Batches keep id order. `Prompt` is called through `withCustom`.
- [x] **Step 4: `jobs.ts` + test:** pure `initialTicks(rows)` honours `Ticked`; `tickAll(rows, key, oneByOne)` is a no-op when `oneByOne`. Vitest.
- [x] **Step 5: `JobPage.tsx`:** job picker, category filter, batch-size stepper from 1, `Skipped / no answer` filter, `PastePanel`, rows grouped by product showing old → new, reason, a **new** tag, warning banner, Apply, Undo, and "next batch". A new tab **Jobs** on `AssistantPage`, path `jobs/<key>`. Works at 360px.
- [x] **Step 6: Run** the Go and Vitest tests. Expected: PASS.
- [x] **Step 7: Commit** `feat(aicatalog): job framework; batches, ticks, undo, skipped list`

### Task 5: Identify a whole list

**Files:**
- Create: `plugins/aicatalog/job_identify.go`, `plugins/aicatalog/job_identify_test.go`
- Modify: `plugins/aicatalog/apply.go` (rename records the old name), `web/e2e/plugin-aicatalog.spec.ts`

**Interfaces:**
- Consumes: `Job`, `AddOldName`, `normPath`, `decodeReply`. Reply shape: `{"items":[{"id":0,"name":"","category":"","specs":"","explanation":"","confident":true}]}`.
- Produces: rows of `Kind` `hint`, `name`, `category`.

- [x] **Step 1: Write the failing tests**, using the real replies:
  - `TestIdentifyUnknownIsNoAnswer`: ChatGPT's `Unknown Product (XYZQ 77)` / `Unclassified` with `confident:false` and Gemini's blank row both give no rows.
  - `TestIdentifyTicks`: hint ticked; category ticked only when uncategorised, `IsNew` for a new path; a differing category and every name unticked.
  - `TestIdentifyKeepsPriceStock`: apply changes neither.
  - `TestRenameKeepsOldName`: applying a name adds the old name, which `MatchHints` finds. Clean up's `names` op does too.
  - `TestRedoShowsOld`: a product with a hint gives `Old` set and starts unticked.
- [x] **Step 2: Run.** Expected: FAIL.
- [x] **Step 3: Implement** the job, sending the shop's category paths like `identifyUserMsg`. Add a "tick all names" action on `JobPage` for `Key == "name"`.
- [x] **Step 4: e2e:** paste a canned reply for two products, tick, apply, the next batch excludes them, Undo restores.
- [x] **Step 5: Run** package tests and spec. Expected: PASS.
- [x] **Step 6: Commit** `feat(aicatalog): identify a whole list in batches`

### Task 6: Alternative group maker

**Files:**
- Create: `plugins/aicatalog/job_groups.go`, `plugins/aicatalog/job_groups_test.go`, `plugins/alternatives/grouper.go`, `plugins/alternatives/grouper_db_test.go`, `web/src/plugins/aicatalog/PageAction.tsx`
- Modify: `internal/plugin/hooks_product.go`, `web/src/plugins/slots.ts`, `web/src/plugins/alternatives/AlternativesPage.tsx:103`, `web/src/plugins/combos/CombosPage.tsx:94`, `web/src/plugins/aicatalog/index.tsx`, `plugins/alternatives/alternatives.go`

**Interfaces:**
- Produces, Go hook:
  ```go
  type ProductGroupInfo struct{ ID int64; Name string; Tiers []ProductGroupTier; Members []int64 }
  type ProductGroupTier struct{ ID int64; Name string }
  type ProductGrouper struct {
      Groups func(ctx context.Context) ([]ProductGroupInfo, error)
      Create func(ctx context.Context, name string, productIDs []int64) (int64, error) // skips products already grouped
      AddTo  func(ctx context.Context, tierID, productID int64) error
      Delete func(ctx context.Context, groupID int64) error
      Remove func(ctx context.Context, productID int64) error
  }
  // reg.AddProductGrouper(g); plugin.ProductGroupers()
  ```
- Produces, slot: `pageActions: [] as PageAction[]`, `type PageAction = { key: string; page: string; Action: ComponentType<{ open: (draft: PageDraft) => void }> }`, `type PageDraft = { name: string; productIds: number[] }`. Pages render `slots.pageActions.filter(a => a.page === '<own key>')` in `PageHead.actions`.

- [ ] **Step 1: Write the failing tests:**
  - `TestGroupsRealReply`: both chatbots' prompt 4 replies give groups `{1,2}` and `{6,7}`.
  - `TestGroupsDropBad`: ids outside the category, a single-member new group, or already-grouped products are dropped.
  - `TestGroupsNeedDetails`: a product with no digit in its name and no hint isn't sent as a candidate.
  - `TestGroupPromptRules`: the prompt asks for same type and same size / rating / fit, and for medicines same ingredient, strength and form.
  - `TestJoinMultiTierNeedsTier`: joining a multi-tier group has no tier chosen and can't apply until one is sent; a single-tier group fills it.
  - `grouper_db_test.go`: `Create` makes one tier, Auto level (after Task 7, assert `level_auto` there), single-tier name blank; `Delete` undoes.
  - `TestUnavailableWithoutGrouper`: no hook → job not listed.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.** Batches run per category, sending every ungrouped name and hint in that category; `size` counts products to place, and a big category splits alphabetically (override `Batch` selection via `Job.Select`, added to `Job` and defaulted). Existing groups are listed in the prompt. Rows: `Kind` `group` carries members and reason; joining a multi-tier group shows a tier picker with no default.
- [ ] **Step 4: Slot + button.** Alternatives and Combos render the generic slot; AI Assistant pushes **✨ Suggest groups** (for `alternatives`), linking to its job page.
- [ ] **Step 5: e2e** in `plugin-alternatives.spec.ts`: paste a canned reply, tick, apply, the group appears, Undo removes it. Build with `make plugins P=aicatalog` alone: no button and no job, and the app runs. `make check-core`. Grep: no `aicatalog` or `AI` in `plugins/alternatives`, `web/src/plugins/alternatives`, `plugins/combos`, `web/src/plugins/combos`.
- [ ] **Step 6: Commit** `feat(aicatalog): alternative group maker through a generic grouper hook`

### Task 7: Alternatives learns levels and splits orders

**Files:**
- Create: `plugins/alternatives/migrations/0002_auto_levels.sql`, `plugins/alternatives/split.go`, `plugins/alternatives/split_test.go`, `plugins/alternatives/levels_db_test.go`
- Modify: `plugins/alternatives/store.go:55,93`, `plugins/alternatives/store_members.go:154-205`, `plugins/alternatives/api.go`, `plugins/alternatives/alternatives.go:87-108`, `web/src/plugins/alternatives/GroupPage.tsx:42-63`, `types.ts`, `api.ts`, `docs/guide/plugins/alternatives.md`

**Interfaces:**
- Produces, SQL: `alt_tiers.level_auto BOOLEAN NOT NULL DEFAULT true`, set `false` where `reorder_level > 0`; `alt_tiers.split_auto BOOLEAN NOT NULL DEFAULT true`; `alt_members.split_share INT` (NULL, percent).
- Produces, Go: `func splitOrder(total int, weights []int64) []int` (largest remainder, sums exactly to `total`; all-zero weights split evenly; extra units go to the highest weight first, ties by position); `Tier.Level` = effective level; `func fasterBrand(manual map[int64]int, sold map[int64]int64) (id int64, ok bool)` (a brand whose sales share exceeds its manual share by ≥ 20 points, with ≥ 10 sold in total).
- Consumes: `Tier` used throughout; `alt_tiers.reorder_level` remains the typed number.

- [ ] **Step 1: Write the failing tests:**
  - `TestSplitOrder`: `(100,[70,20,10])→[70,20,10]`; `(10,[1,1,1])→[4,3,3]`; `(5,[0,0])→[3,2]`; `(0,[3,1])→[0,0]`; the sum always equals total.
  - `TestAutoLevelSumsMembers`: members at levels 10 and 20 → tier level 30; a disabled member isn't counted; no live members → level 0, not low.
  - `TestMigrationKeepsTyped`: tier with typed 40 stays fixed 40; tier with 0 becomes Auto.
  - `TestTypedZeroMeansOff`: `level_auto=false, reorder_level=0` → not tracked, never covered.
  - `TestNeedSplitBySales`: level 50, stock 12 → total 88, split by 90-day sales; even split with none.
  - `TestManualSplit`: shares 50/50 → 44/44; `fasterBrand` true when one brand holds 80% of sales; nothing changes without a tap.
  - `TestLowMembersUsesAuto`: `LowTierMemberIDs` uses the effective level.
- [ ] **Step 2: Run** `go test ./plugins/alternatives`. Expected: FAIL.
- [ ] **Step 3: Implement.** `tierLevelSQL` snippet shared by `tierCols` and `LowTierMemberIDs`: `CASE WHEN t.level_auto THEN (SUM of live members' reorder_level) ELSE t.reorder_level END`. Sales via `reports.ProductQtySold` over 90 days. `reorderNotes` sends each member's share.
- [ ] **Step 4: Group page:** Auto / typed level like products, a line saying how Auto came out, Automatic / Manual split with percent inputs that must total 100, and a suggestion with a Use button when `fasterBrand`. Tests in `GroupPage`'s spec. Shots at 360px.
- [ ] **Step 5: Run** package tests, `plugin-alternatives.spec.ts`. Expected: PASS; update any test pinning the old default `0` deliberately, without weakening what it checks.
- [ ] **Step 6: Commit** `feat(alternatives): groups learn their level; orders split by what sells`

### Task 8: Low stock understands groups

**Files:**
- Create: `web/src/features/purchasing/reorder/blocks.ts`, `blocks.test.ts`, `ReorderBlock.tsx`, `internal/api/reorder_groups_test.go`
- Modify: `internal/plugin/hooks_product.go:107-116`, `internal/api/reorder_rows.go:115-131`, `internal/api/admin_reorder.go`, `internal/api/dto_purchasing.go:188`, `internal/api/admin_dashboard.go:133`, `web/src/features/purchasing/queries.ts:28`, `web/src/features/purchasing/reorder/ReorderPage.tsx`, `LowStockPaper.tsx`, `web/src/features/counter/lowstock/LowStockTab.tsx`, `web/src/routes/_authed/print/low-stock.tsx`, `plugins/alternatives/alternatives.go`, `web/e2e/counter-lowstock.spec.ts`, `docs/guide/purchasing.md`

**Interfaces:**
- Produces: `ReorderNote` gains `Exact bool` (Suggest replaces core's figure), `Block, BlockLine string`, doc-commented generically. `ReorderRow` gains `Block`, `BlockLine` (JSON `block`, `blockLine`). `blocks(rows): Block[]` groups rows sharing `block`, keeps others alone, keeps list order by first appearance.
- Consumes: Task 7's split.

- [ ] **Step 1: Write the failing tests:**
  - `TestExactReplacesOwnNeed`: fake annotator with `Exact:true, Suggest:3`; sales-based own need 40 → `Suggested == "3"`. Without `Exact`, max as today.
  - `TestTillListHidesCovered`: `/till/low-stock?all=1&hideCovered=1` hides a covered row, pulls in a low group's member.
  - `TestDashboardCountsCoverage`: with a covered product and a low-group member, the attention count matches the Reorder list's total.
  - `blocks.test.ts`: mixed rows, a block spanning pages of `data.items`, empty.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.** `Suggested`: `Exact` wins, else `decimal.Max`. Dashboard uses `reorderRows(…, hideCovered=true)` when annotators exist. Alternatives fills `Block` (group · tier), `BlockLine` (`LOW: 12 left in the group · reorder at 50`), `Exact`, and `Suggest` from the split. `make types`.
- [ ] **Step 4: Pages.**
  - `ReorderBlock.tsx` renders the heading, line and member rows.
  - Reorder page: rows in a block sit together, each still ticked and editable alone.
  - Till tab: sections stay by supplier for the reps, with blocks inside each supplier; hide covered by default with a **Show covered** toggle; notes shown. A group's brand from another supplier appears under that supplier with the same heading line.
  - Printed list: same `hideCovered` and headings.
  - A product with no group looks exactly as today.
- [ ] **Step 5: e2e** in `counter-lowstock.spec.ts`: a covered twin hidden then shown by the toggle; a low group shows one block; "Order these" fills split quantities. Shots at 1280 and 360, Studio and Night.
- [ ] **Step 6: Run** `go test ./internal/api`, Vitest, the specs, `make check-core`. Expected: PASS.
- [ ] **Step 7: Commit** `feat(reorder): low stock shows groups; till tab and dashboard respect coverage`

### Task 9: Fill in details (generic attributes + Product Plus)

**Files:**
- Create: `plugins/aicatalog/job_fields.go`, `plugins/aicatalog/job_fields_test.go`, `plugins/productplus/attributes.go`, `plugins/productplus/attributes_db_test.go`
- Modify: `internal/plugin/hooks_product.go`, `plugins/productplus/hooks.go`, `web/src/plugins/aicatalog/JobPage.tsx` (tick list via `options`)

**Interfaces:**
- Produces, Go hook:
  ```go
  type ProductAttribute struct{ Key, Label, Type string; Options []string; CategoryIDs []int64 } // no CategoryIDs = all products, sub-categories included by the provider
  type KnownValue struct{ Value, Meaning string; Extra map[string]string }
  type ProductAttributeSet struct {
      Key        string
      Attributes func(ctx context.Context) ([]ProductAttribute, error)
      AppliesTo  func(ctx context.Context, productID int64) ([]string, error) // attribute keys in scope
      Values     func(ctx context.Context, ids []int64) (map[int64]map[string]string, error)
      Known      func(ctx context.Context, attr string, limit int) ([]KnownValue, error) // most used first
      Set        func(ctx context.Context, productID int64, attr, value string) error
      AddKnown   func(ctx context.Context, attr string, v KnownValue) error // nil when not supported
  }
  // reg.AddProductAttributeSet(s); plugin.ProductAttributeSets()
  ```

- [ ] **Step 1: Write the failing tests**, using prompt 5's real replies:
  - `TestFieldsOnlyTicked`: unticked fields are neither asked for nor applied, even if the reply has them.
  - `TestFieldsPerCategory`: a medicine is asked for `generic_name`, a part for `fits_model`, in one batch; a reply giving a part `generic_name` is dropped.
  - `TestFieldsKnownValues`: the prompt lists existing values capped at 30 per field, most-used first; `GSK` reused isn't `IsNew`; a new value is.
  - `TestFieldsDropdown`: a value outside the options is dropped, compared case-insensitively.
  - `TestFieldsEmpty`: ChatGPT's `{"field":"fits_model","value":""}` gives no row.
  - `TestFieldsNeverOverwrite`: an existing value gives a row `Old` set, unticked, only on Redo.
  - `TestFieldsBadTypes`: `abc` for a number or a bad date is reported, others save.
  - `attributes_db_test.go`: scope via `appliesSQL` including sub-categories; `Set` uses `coerce`.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement** the Product Plus provider, then the job (`Key: "fields"`, available when a set with key `productplus` exists). Page text says run after categories are right. Tick list starts with nothing ticked, so a rack number is never guessed by default.
- [ ] **Step 4: e2e:** paste a canned reply on a build with Product Plus; only ticked fields save.
- [ ] **Step 5: Commit** `feat(aicatalog): fill in the shop's own fields, respecting scope and types`

### Task 10: Setup details

**Files:**
- Create: `plugins/aicatalog/job_setup.go`, `plugins/aicatalog/job_setup_test.go`

**Interfaces:**
- Reply: `{"items":[{"id":0,"has_expiry":true,"pack_name":"Card","pack_size":10,"reason":""}]}`. Available always. Applies through `products.Service` (`Update` for expiry, `SavePack` for the pack with blank prices).

- [ ] **Step 1: Write the failing tests:**
  - `TestSetupExpiryChangeUnticked`: suggesting yes on a current no is `Change:true`, unticked; `null` gives no row.
  - `TestSetupPackBlankPrice`: applying leaves pack prices 0, so size × unit price applies.
  - `TestSetupPackSkips`: existing same-name pack, size ≤ 1, a decimal unit, or a service give no row.
  - `TestSetupUndoPack`: undo removes the pack, unless it has been edited since.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(aicatalog): suggest expiry and pack size`

### Task 11: Pharmacy marks

**Files:**
- Create: `plugins/pharmacy/attributes.go`, `plugins/pharmacy/attributes_db_test.go`, `plugins/aicatalog/job_marks.go`, `plugins/aicatalog/job_marks_test.go`
- Modify: `plugins/pharmacy/pharmacy.go`

**Interfaces:**
- Consumes: `ProductAttributeSet` with `Key: "pharmacy"`, attributes `rx_only` (bool), `controlled` (bool), `dose` (text). `Set` is read-modify-write over `Flags` / `SetFlags`, so other marks survive. `controlled` true implies `rx_only` true, the rule in `0008`.
- Job: `Key: "marks"`, `Warning: "No suggestion does not mean cleared. The chatbot can miss one. The pharmacist still checks the list."`

- [ ] **Step 1: Write the failing tests:**
  - `attributes_db_test.go`: setting `rx_only` keeps `dose` and `controlled`; `controlled` also sets `rx_only`; `Values` reads them.
  - `TestMarksRealReply`: prompt 6's replies give rows for each medicine, with reason.
  - `TestMarksNull`: `rx_only:null` gives no row and the product is `no_answer`, which stays visible under the filter.
  - `TestMarksUnticked`: every mark starts unticked; a change from yes to no is `Change:true`.
  - `TestMarksCategoryFilter`: `categoryId` limits the batch, so cleaning items aren't asked about.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run.** Expected: PASS. Controlled register page still reads the mark.
- [ ] **Step 5: Commit** `feat(aicatalog): suggest Rx-only and controlled marks for the pharmacist to check`

### Task 12: Usual dose

**Files:**
- Create: `plugins/aicatalog/job_dose.go`, `plugins/aicatalog/job_dose_test.go`
- Modify: `plugins/pharmacy/attributes.go` (`Known` from `Doses`, `AddKnown` via `CreateDose`, with `Extra["perDay"]`)

**Interfaces:**
- Job: `Key: "dose"`, `OneByOne: true`. Reply: `{"items":[{"id":0,"dose":"","new":false,"meaning":"","per_day":0,"reason":""}]}`.
- Prompt includes: the whole dose list with meanings and per-day; "reuse exact wording"; "a new dose gives a short form like 1-0-0 or every 6 hours"; "as needed means per_day 0"; "leave dose empty if it depends on the patient"; usual adult dose.

- [ ] **Step 1: Write the failing tests**, with real replies:
  - `TestDoseLiteralNew`: ChatGPT's `"dose":"new"` is never a dose; the row has no `New`, shows the meaning, and can't apply until the pharmacist types a form.
  - `TestDoseBlankNew`: Gemini's `dose:""`, `new:true` behaves the same.
  - `TestDoseOnList`: `1-1-1` isn't new; case and spacing are normalised to the list's own wording.
  - `TestDoseClaimedButMissing`: `new:false` with a dose not on the list counts as new.
  - `TestDoseEmptyWarfarin`: empty dose, `new:false` gives no row, and `no_answer`.
  - `TestDoseNeverReplaces`: a product with a dose isn't offered except on Redo, and stays unticked.
  - `TestDoseNewAddsToList`: applying a new dose adds it with words and `per_day`; undo removes it only if nothing else uses it.
  - `TestDosePerDayAsNeeded`: the prompt says as needed = 0; the preview shows `per_day`, editable.
  - `TestDoseNoTickAll`: `OneByOne` is true; the page has no tick-all.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.** Row inputs for dose form, meaning and per day when new.
- [ ] **Step 4: e2e:** a canned reply, approving one row; the medicine's usual dose fills a prescription line.
- [ ] **Step 5: Commit** `feat(aicatalog): suggest usual doses from the shop's own dose list`

### Task 13: Combo suggestions

**Files:**
- Create: `plugins/aicatalog/job_combos.go`, `plugins/aicatalog/job_combos_test.go`, `web/src/plugins/aicatalog/ComboSuggest.tsx`
- Modify: `web/src/plugins/combos/CombosPage.tsx`, `web/src/plugins/combos/ComboSheet.tsx`, `web/src/plugins/aicatalog/index.tsx`

**Interfaces:**
- Consumes: `slots.pageActions` with `page: 'combos'`; `open(draft)` makes the page open `ComboSheet` with `draft.name` and `draftOf` lines from `draft.productIds`, and price blank.
- Route: `POST /aicatalog/combo-suggest` `{reply, categoryId}`, same manual flow; no apply route, because the only save is the normal combo form. Reply: `{"combos":[{"name":"","product_ids":[],"reason":""}]}`.

- [ ] **Step 1: Write the failing tests:**
  - `TestCombosRealReply`: prompt 8's replies give 3 suggestions.
  - `TestCombosDrop`: foreign ids, fewer than 2 products, or a product marked `rx_only` by any attribute set are dropped; no price is read even if a reply has one.
  - `TestCombosPromptNoPrices`.
  - Combos page Vitest or e2e: `open` fills the sheet with products, price is empty, and nothing saves until Save.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.** Suggestions list with reason and **Use** on each. AI Assistant's Jobs tab links to the Combos page only when `slots.apps` has `combos`.
- [ ] **Step 4: Run.** Expected: PASS. Grep: `plugins/combos` and `web/src/plugins/combos` still don't mention AI.
- [ ] **Step 5: Commit** `feat(aicatalog): suggest combos from a button on the Combos page`

### Task 14: Docs and screenshots

**Files:**
- Modify: `docs/guide/plugins/aicatalog.md`, `alternatives.md`, `combos.md`, `pharmacy.md`, `productplus.md`, `docs/guide/stock.md`, `docs/guide/purchasing.md`, `web/docs-shots/shots.spec.ts`, `web/e2e/shot-prep-aicatalog.ts`, `web/e2e/shot-prep-alternatives.ts`, Demo Room gens if a shot needs hints, `README.md` if headline

- [ ] **Step 1:** Write the guides in the user's plain style, including the warning on Rx and doses and that hints are hints.
- [ ] **Step 2:** Add shots for the Jobs tab, a low-stock group block and a hint at the till, filling empty states first. Run `make docs-shots` and review the images.
- [ ] **Step 3:** Run `go test ./internal/docscheck`. Expected: PASS. No price or legal change.
- [ ] **Step 4: Commit** `docs: AI Assistant jobs, hints, group levels, packs on create`

### Task 15: Test, review, fix, harden

- [ ] **Step 1:** `make test`, `make test-web`, `make e2e-seed && cd web && bunx playwright test` (kill anything on 3000/5173 first), `make check-core`, `make verify`. Expected: all green, compared with 235 passed / 33 skipped.
- [ ] **Step 2:** A fresh reviewer goes through the whole change against the spec and this plan's Review Focus (superpowers:requesting-code-review).
- [ ] **Step 3:** Fix every finding test-first; one commit per fix. Include bugs found near this work.
- [ ] **Step 4:** Harden: paste every real reply from the spec through the live screens; a build with AI Assistant alone, and one with each plugin alone; every Dock palette spot check in both modes on the new screens; 360px on the new admin pages.
- [ ] **Step 5:** Re-run Step 1. Update `docs/status.md` and `AGENTS.md` (hooks, slot, migrations), delete this plan and the spec, `make core-only`.
- [ ] **Step 6: Commit** `docs(status): AI Assistant jobs and Alternatives levels done`
