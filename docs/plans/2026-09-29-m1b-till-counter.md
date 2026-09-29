# M1b Till: counter operations

> Lean plan. Opus plans, executes and reviews. Each task is a vertical slice:
> karots-pos **behaviour** (read its handler + template when starting the task) →
> DTOs (`internal/api/dto_counter.go`, `make types`) → handlers (`internal/api/counter_<area>.go`)
> with a Go test → SPA screen → self-reviewed screenshots (4 combos, 1366 + 1024) → one commit.
> **Don't copy karots-pos UI 1:1.** Redesign each screen to be quicker to use at a counter.

**Goal:** every karots-pos cashier route that isn't the sell screen, in the suite till.
Spec §6.5 · parity rows `internal/web/register_cashier.go` + `register_*` supplier/expense groups.

## Shared design (applies to every task)

- **Tabs:** `TillShell` shows `Sell · Returns · Credit · Receipts` plus a **More ▾** menu: Write-off, Convert,
  Labels, Warranty, Suppliers*, Expenses*, and plugin tabs (`slots.tillTabs`). *Only when
  `config.canHandleSuppliers` (karots-pos `MaySeeSuppliers`). Routes are `/till/<tab>`. They stay under
  `TillProvider`, so the cart survives tab switches.
- **Counter screen pattern** (`features/counter/`): a left **find** pane (search box, autofocused, Enter picks
  the first hit, list with keyboard ↑↓) and a right **act** pane (the form for the picked record). No full-page
  tables, and no modals for the main job. It works at 1024.
- **Money** goes only through the forked services (`cashflow.Move/MoveTx` inside them). Handlers never touch
  cash tables.
- **Printing:** thermal slips via Go (`/till/.../print`). A4/browser views are React print routes (`/print/...`).
- **Errors:** `apperr` codes. The SPA shows `toast.error(errMsg(e))`.

## Tasks

- [x] **1. Tabs + counter scaffold.** `TillShell` tab row + More menu, `slots.tillTabs`, `canHandleSuppliers`
  in `TillConfig`, a shared `CounterLayout` (find/act panes), and route stubs. Check: tabs at 1024/1366, and the
  cart survives Sell → Returns → Sell.
- [x] **2. Expenses** (`cashier_expenses.go`). List today's counter expenses, and record one (category, amount,
  note, paid from the drawer or a locker). Gated by the supplier flag.
- [x] **3. Write-off + Convert** (`cashier_damage.go`, `cashier_conversions.go`). Write-off: product search →
  lot → qty → reason. Convert: pick a conversion recipe → qty → run.
- [x] **4. Labels** (`cashier_labels.go`). Pick products and qty, then send to the label printer.
- [x] **5. Returns** (`cashier_returns.go`). Find a sale by receipt no/phone/today's list → returnable lines →
  qty per line → refund method (cash / to account) → slip.
- [x] **6. Credit** (`cashier_credit.go`). Customers with balance/advance → take a payment (cash/card/online) or
  refund an advance → debt receipt slip. Open to every cashier (karots-pos parity; only over-limit selling is gated).
- [ ] **7. Receipts + Z report** (`cashier_receipts.go`, `cashier.go` Receipt/ZReport). A hub with filters
  (sales · cash moves · credit · warranty) and a detail pane with reprint. Browser receipt view `/print/receipt/:id`.
  The Z report for the user's own session.
- [ ] **8. Warranty** (`cashier_warranty.go`). Look up by serial or by receipt → claim → replace (stock swap) →
  claim slip.
- [ ] **9. Suppliers at the counter** (`cashier_suppliers.go`, `cashier_supplier_pay.go`, `cashier_receive.go`).
  Supplier list → pay / refund, receive walk-in stock, order, receive against an order, quick-create supplier /
  product / wanted item.
- [ ] **10. Flows, parity, docs.** Playwright flows per tab (`web/e2e/counter.spec.ts`), tick parity rows,
  status.md handoff to M2, and the ship check (types, vet, tests, build, plugin grep, ≤500 lines).
