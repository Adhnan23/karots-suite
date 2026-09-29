# M2 Catalog and stock

> Lean plan, same shape as M1b. Opus plans, executes and reviews. Each task is a vertical slice:
> karots-pos **behaviour** (read its handler + templ + memory note when starting the task) →
> DTOs (`internal/api/dto_catalog.go` / `dto_stock.go`, `make types`) → handlers
> (`internal/api/admin_<area>.go`) with a Go test → SPA screen → self-reviewed screenshots
> (4 combos, 1366 + 360) → one commit. **Don't copy karots-pos UI 1:1.**

**Goal:** every karots-pos Inventory + catalog-setup admin route, in the suite admin (dock areas
Inventory and Setup). Spec §11 M2 · parity rows in `internal/web/register_admin.go`.

**Out of M2 (by spec):** reports (`/reports/*` incl. batches, low-stock, count-sheet, inventory → M5),
product / customer / supplier CSV import + export (M6), `/damage` + `/damage/table` (merged into the
Losses report, M5).

## Shared design (applies to every task)

- **API:** `/api/v1/admin/*`, `h.protected` + `RequireRole(admin, manager)` (karots-pos `ag`).
  Lists return `{items, total, page, pageSize}` (`Page[T]`). Cost fields only when `CanSeeCost`.
- **Admin kit** (`web/src/components/admin/`, Task 0): `PageHead` (title, breadcrumb Area / Page,
  area page-tabs, actions), `DataList` (column defs → table at ≥768, card list below), `Pager`,
  `SearchBox` (debounced, URL search param), `FormSheet` (dialog on desktop, bottom sheet on phone),
  `Field`, `ConfirmButton`, `EmptyState`, `Pill`. Filters live in typed route search params.
- **Nav:** `theme/nav.ts` gains the Inventory and Setup pages as they land; the shelf and command
  palette pick them up. Stock-take + Intake entries hidden when `settings.stockTakeEnabled` is off.
- **Money** only through forked services. Stock moves only through `stock.Service` (lot-aware).
- **No new deps** unless a screen needs them: plain controlled forms + server `apperr` fields;
  TanStack Table/Form get added only if a screen proves it needs them.

## Tasks

- [x] **0. Admin kit + API group.** `registerAdmin` group, `Page[T]`, kit components, nav wiring,
  phone More sheet lists real pages. Check: shelf, palette, page tabs at 1366 + 360.
- [x] **1. Units** (`admin_units.go`). List, create, edit, delete (in-use guard).
- [x] **2. Categories** (`admin_categories.go`). Nested tree, create/edit/move parent, delete,
  quick-create from a picker (`FindOrCreateByPath`). Shared `CategoryPicker` (with inline create).
- [x] **3. Products** (`admin_products.go`). Filtered, paged list (search, category, status,
  low stock), form (all product fields, cost/markup/selling two-way, wholesale, barcode + generate,
  supplier, unit, reorder, serial/warranty, expiry tracking), disable/reactivate, barcode assign.
  **Items to review** queue + count badge on the dock.
- [x] **4. Services & recipes** (`admin_services.go`, `admin_recipes.go`). Service list/create,
  recipe editor (ingredients + qty) on any product.
- [x] **5. Cashier menu** (`admin_groups.go`). Group tree, create/edit/move/disable/reactivate,
  link products with emoji, reorder.
- [x] **6. Stock movement** (`admin_stock.go`). Levels list (search, category, low), movement
  history (product, type, who, dates, paged), adjust (lot-aware), per-product lots.
- [x] **7. Batches** (`admin_batches.go`). Grouped by product, expired / expiring banners, write off
  one or all expired, per-lot selling price.
- [x] **8. Stock-take** (`admin_stocktake.go`). Count screen (category filter, counted vs on-hand,
  apply), printable count sheet (`/print/count-sheet`), CSV import; all gated by the toggle.
- [x] **9. Stock intake** (`admin_intake.go`). One search box → restock or create, cost⇄markup⇄
  selling, supplier, opening qty, label print, session list with undo. Gated by the toggle.
- [ ] **10. Conversions** (`admin_conversions.go`). List, create/edit, disable/reactivate, run,
  run history.
- [ ] **11. Labels + damage** (`admin_labels.go`, `admin_damage.go`). Admin label designer reusing
  the counter label components; admin write-off form (lot pick, reason).
- [ ] **12. Flows, parity, docs.** Playwright `web/e2e/admin-catalog.spec.ts` + `admin-stock.spec.ts`
  (incl. 360px), the M1 leftover flows (credit refund, supplier refund, warranty by receipt, quick
  create), tick parity rows, status.md handoff to M3, ship check (types, vet, tests, build, plugin
  grep, ≤500 lines), then the milestone review + one `fix:` commit.
