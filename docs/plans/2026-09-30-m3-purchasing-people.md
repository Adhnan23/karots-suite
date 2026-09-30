# M3 Purchasing and people

> Lean plan, same shape as M2. Opus plans, executes and reviews. Each task is a vertical slice:
> karots-pos **behaviour** (handler + templ + `static/js/purchasing.js` / `reorder.js` + memory
> note — grep the templ's `name=` / `x-model=` fields and diff them against the new form) →
> DTOs (`internal/api/dto_people.go` / `dto_purchasing.go`, `make types`) → handlers
> (`internal/api/admin_<area>.go`) with a Go test → SPA screen → self-reviewed screenshots
> (4 combos, 1366 + 360) → one commit. **Don't copy karots-pos UI 1:1.**

**Goal:** every karots-pos Purchasing, Customers and Users admin route in the suite admin.
Spec §11 M3 · parity rows `register_admin.go` customers / suppliers / purchases / purchase-returns /
users. Dock: **Sell** gains Customers, **Purchasing** gains Purchases · Reorder · Purchase returns ·
Suppliers, **Setup** gains Users (admin only).

**Out of M3 (by spec):** customer/supplier import + export + templates (M6); `/reports/purchases`,
`/reports/low-stock` (+print), `/reports/topup-reorder`, supplier/customer dues + spend (M5).
The Reorder page built here is what M5's low-stock report links to.

**karots-pos memory to read first:** `supplier-money-flows.md` (credited_amount, refunds,
settlement ceiling = balance), `purchase-order-lifecycle.md` (draft → receive, keep remainder,
margin guard, forgotten items, GRN, demand reorder), `supplier-receiving-discounts.md` (discounts
lower lot cost), `editable-opening-balance-plan.md` (opening_unlinked, negative allowed, audit),
`duplicate-customer-supplier-guard.md` (admin create = 409 on dup; till keeps reuse),
`supplier-at-the-counter.md`, `credit-as-payment-type.md`.

## Shared design

- **API:** `/api/v1/admin/*` as in M2 (`RequireRole(admin, manager)`); `/admin/users*` adds
  `RequireRole(admin)`. Lists use `Page[T]`. Cost fields only when `CanSeeCost`.
- **Reuse, don't copy:** the counter's supplier `LineEditor` + `totals.ts` live in
  `features/purchasing/` and both the counter and admin import them. Counter + admin receive/pay go through the same Go service calls.
- **Money** only through `supplierpay` / `customers` / `cashflow.MoveTx`. Stock only through the
  purchases/purchasereturns services (lot-aware).
- **Opening adjust** is one shared `OpeningSheet` (Total, locked-to-transactions part read-only,
  new opening, live new total, note) for customers and suppliers; every adjust writes audit.
- **Statements, PO, GRN** are React print routes (`/print/supplier-statement/:id`,
  `/print/customer-statement/:id`, `/print/po?ids=&mode=combined|separate`, `/print/grn/:id`)
  with paper/ink tokens, like M2's label sheet.
- **Nav gate** gains `'admin'` (Users hidden from managers).
- Delete each ported feature's legacy `RegisterAPI` / `api.go` (customers, suppliers, purchases,
  purchasereturns) once its admin routes land.

## Tasks

- [x] **0. Stock batch sheet** (open item the user asked for). Title "Batches of …", **Batch
  price** label (blank = follows the product), per-batch **Set count** and **Remove batch**
  (count correction / write-off), drop the whole-product "Adjust the count" shortcut from that sheet.
- [x] **1. Shared purchasing kit.** LineEditor + totals move to `features/purchasing/` (the
  counter imports them). `GET /admin/money-sources` (lockers with balance + open tills) +
  `parseLocation` and `MoneySourcePicker` (cash → safe/pocket/till, card → card, online → bank).
  The counter PayForm stays the counter's: the admin pay form adds per-invoice allocations, an
  old-debt amount and any locker/till as source. `SupplierPicker`, `OpeningSheet`, nav entries and
  the `'admin'` gate land with their first screen (tasks 2, 5, 9).
- [ ] **2. Suppliers** (`admin_suppliers.go`). List (search, active/archived, owing / in credit),
  create/edit (name, contact person, phone, address, credit days, opening on create; dup name →
  409), archive/reactivate, **reassign** products to another supplier, opening adjust, **pay**
  (open-invoice allocations, pay opening, advance, method + source) and **refund** (capped at
  credit), payment history, statement print.
- [ ] **3. Customers** (`admin_customers.go`). List (search, active/archived, owing / advance,
  over limit), create/edit (name, phone, address, credit limit, opening on create; phone dup →
  409), delete/reactivate, opening adjust, **payment** (amount, method, money account, apply to
  opening), statement print.
- [ ] **4. Purchases list + detail** (`admin_purchases.go`). Orders (drafts) and Received tabs
  (supplier, dates, search), detail (lines ordered vs received, discounts, paid / credited /
  balance, links to GRN / edit / receive), delete draft (received = 422).
- [ ] **5. Order entry** — new PO, edit draft, **receive now** (walk-in GRN). Supplier picker
  (preferred-supplier prefill), product search lines (qty, free, cost, sell, item discount Rs/%,
  expiry), bill discount Rs/%, expected date, notes. Receive-now adds invoice no, due date, pay now
  (amount clamped to what's left after credit settlement, method, source).
- [ ] **6. Receive an order + prints.** Ordered (read-only) vs received, **+ Add forgotten item**,
  **keep remainder** as a new draft when short, **margin guard** (cost ≥ sell or below prior
  markup → flag + one-click suggested sell), invoice / due / pay now. `/print/po` (combined or one
  per supplier, shop ↔ supplier block) and `/print/grn/:id`.
- [ ] **7. Reorder** (`admin_reorder.go`). Items at/below reorder level (category filter), demand
  hint (90-day avg × 14-day lead − on hand, fallback 2× reorder), sold-last-year column, pick +
  qty + supplier (defaults to preferred) → one draft per supplier → open `/print/po`.
- [ ] **8. Purchase returns** (`admin_purchase_returns.go`). List, new (supplier, product lines
  with batch pick, qty, cost, reason, reference) → debit note crediting invoices oldest-first,
  read-only detail (no edit/delete — correct with an opposing entry).
- [ ] **9. Users** (`admin_users.go`, admin only). List (role, active), create/edit (name, phone,
  role, PIN / reset — forces re-login, can see cost, can manage credit, can handle suppliers,
  receipt printer), deactivate/reactivate. System user never listed; last admin can't be
  deactivated or demoted.
- [ ] **10. Flows, parity, docs.** Playwright `admin-purchasing.spec.ts` + `admin-people.spec.ts`
  (incl. 360px): supplier pay / refund / opening, PO → partial receive → remainder draft, return
  credits the invoice, reorder → drafts, customer payment / opening, user create + login + flags.
  Tick parity rows, status.md handoff to M4, ship check (types, vet, tests, build, plugin grep,
  ≤500 lines), milestone review + one `fix:` commit.
