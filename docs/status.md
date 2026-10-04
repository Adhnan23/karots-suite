# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of
every milestone.

## Status (2026-09-30)

**M3 Purchasing and people is COMPLETE** (`docs/plans/2026-09-30-m3-purchasing-people.md`). M0, M1 (till) and M2 (catalog + stock) were complete before it.

M3 delivered (dock: **Sell** → Customers; **Purchasing** → Purchases · Reorder · Purchase returns · Suppliers; **Setup** → Users, admin only; every page works at 360px):

- **Suppliers:** list with "we owe / they owe us" totals; a supplier page (balance either way, open bills, orders on the way, payments + refunds); pay (one amount waterfalled, bills first or old debt first, or split exactly across bills / old debt / advance) from any locker or open till, or by card / online; refund when they hold our credit; old-balance correction (audited); move products to another supplier; disable / re-enable; printable statement (`/print/statement/supplier/:id`).
- **Customers:** list with due / advances totals and owing · over limit · advance filters; a customer page (balance or advance, credit-limit bar, latest activity); collect (cash into a locker or till, card / online into its account, old debt first); old balance; printable statement.
- **Purchases:** orders and received lists; one entry screen to place / edit an order, receive a walk-in delivery or receive an order (cost prefilled, preferred supplier, due date from credit days, margin guard with a suggested price, forgotten items, short/extra per line, the rest kept on a new order, pay now, F4 / F10); PO print (together or one per supplier) and GRN (`/print/po`, `/print/grn/:id`).
- **Reorder:** low-stock worklist with a demand quantity (90-day sales × 14-day lead, else 2× reorder level) and sold week / month / last year; a cart that survives filter changes; one PO per supplier.
- **Purchase returns — upgraded (user, 2026-09-30):** made against one received delivery, only its lines, at most what's left of its lot, priced server-side at the lot's real cost (after line + bill discounts and free units), credited to that bill first. Stock that came on no delivery (opening / counted / intake) goes back as "old stock": pick product + lot, priced at the lot's recorded cost, off the oldest bills. karots-pos let any product go to any supplier at a typed cost.
- **Touch steppers** (− qty +, like the till cart and counter returns) on purchase / delivery lines (admin + counter) and return quantities.
- **Users:** staff cards, role, PIN reset (signs them out), cashier rights (counter operations, manage credit, see cost), own printer; you can't deactivate yourself; the last admin can't be deactivated or demoted.
- **Also:** Stock → batch sheet (the open item): "Batches of …", Batch price, per-batch Set count / Remove batch. Shared kit: `features/purchasing/` (LineEditor — now stacks at 360px — totals + margin guard, MoneySourcePicker), `features/people/` (OpeningSheet, StatementSheet). `GET /admin/money-sources` + `parseLocation`.
- **karots-pos bugs fixed here and in karots-pos (`ababde1`):** the supplier statement left out supplier refunds; the customer statement dropped a negative opening balance (a pre-system advance). Both made the running balance disagree with the real one.
- **karots-pos fixes of 2026-10-04 carried over** (`docs/karots-pos-fixes-2026-10-04.md`, before M4): a return refunds what was PAID (bill discount shared pro-rata, `sales.PaidFactorSQL`, also in the sales-list profit, Z-report Net and Returns report; the counter return preview shows the same figure); the whole-sale return is a line return of every remaining line; lot cost rounds to 2dp; the sales search finds a sale by the product sold (till word rules, no fuzzy); the counter lists (Returns / Receipts sales, cash, credit, warranty claims, warranty register) page 50 at a time with "Show older"; the dead legacy `sales.RegisterAPI` is gone. **Found while porting, fixed here and in karots-pos (`d55ac5f`):** returns on a part-paid credit sale each cleared up to the whole debt (over-crediting, no cash back); now only what earlier returns left.
- **Tests:** `go test ./...`, 100 web unit tests and **73 Playwright flows** green; `make verify-portable` passes. Parity: **196 rows ticked** (53 in M3). Legacy `RegisterAPI` files removed for customers, suppliers, purchases and purchase returns.

**NEXT: M4 Money** (spec §11): cashflow, lockers, expenses (+categories), bill-pay / get-money, settle card, receipts hub (S- / CR- / DP- / warranty), finance hub.

**Porting rule (M2, confirmed again in M3):** read karots-pos's `static/js/<screen>.js` and templ, not just the Go handler, and sweep the finished screens against them (the M3 sweep found F4/F10, Pay all, per-line variance, scan → barcode). Where karots-pos is flawed, don't copy the flaw — the user wants upgrades ("this is its upgrade").

## How we work

1. **Opus plans.** One plan per milestone in `docs/plans/`, with the same shape as the M0 plan: tasks with files, interfaces, test-first steps and a commit per step.
2. **Opus executes** the plan too (Sonnet was dropped on 2026-09-29). At every **UI gate** it takes screenshots in all four palette × mode combinations, reviews them itself, and carries on without waiting for approval unless something needs a product decision.
3. **Opus reviews** the whole milestone:
   - build, vet, `go test ./...`, web tests, Playwright
   - screenshots in all four combinations
   - diff reading
   - fixing what's found in one `fix:` commit
4. **No Claude or AI attribution** in any commit or PR, ever.
5. **Work on `main`, local, and unpushed** until the user says otherwise.

What the user rejects:

- **Plain or default-looking UI.** Stock shadcn cards, a karots-pos-like sidebar, and "Modules arrive later" placeholders have all been rejected. Every screen must look like the approved mockups.
- **Missing karots-pos features.** Always check karots-pos for what a screen does before planning it; the user expects nothing to be dropped.

## Design references

- `docs/design/theme.md` is the source of truth for tokens, palettes, login, the dock and the till layout.
- **Shell, login and palettes mockup:** https://claude.ai/artifact/MANYgogqsxC4evmSADNkia
- **Till and dashboard mockup (look only):** https://claude.ai/artifact/GkJjnypEes4GmHiHMDzFT5. Use its "Night Counter" and "Studio" directions. It shows the till mid-sale:
  - groups and a frequently-sold grid
  - a cart with qty stepper, per-line discount chip and expiry badge
  - a customer picker showing balance, advance and limit
  - Cash, Card, Online and On-account methods
  - split tenders and quick cash
  - Fill, bill discount %, and change with Keep options
  - an over-limit warning, the sale-complete overlay, and Hold / New sale

## Inputs for M4

- Read the spec's M4 section, then karots-pos `/admin/cashflow`, `/admin/lockers`, `/admin/expenses` (+categories), `/admin/cash-register`, `/admin/receipts`, `/admin/finance` handlers (`internal/web/register_admin.go`), their templ **and** JS, and the memory notes (`cashflow-lockers-plan.md`, `supplier-money-flows.md`, `receipts-dashboard-sales-kiosk-polish.md`, …).
- Reuse `MoneySourcePicker` / `GET /admin/money-sources` / `parseLocation` (lockers + open tills) and the M2/M3 admin kit. Money only through `cashflow.Move/MoveTx`.
- **Rules from the karots-pos fixes:** the receipts hub (S- / CR- / DP- / warranty) pages from day one (`?offset=` + `paged` / `MoreRows` in `features/counter`); an admin sales return builds on `ReturnTx` / `PartialReturnTx` and pays the cash part out of a picked locker or open till (`MoneySourcePicker`) with `cashflow.MoveTx` in the same tx, asking for a source only when there is cash to refund; plugin cash-outs (M7) go through cashflow, never `CashRegister.Withdraw`.
- M3 pay/collect forms offer "Print receipt" through the counter's thermal slip; the M4 receipts hub should list those CR- receipts too.
- **Still unticked — needs a real printer (POS80 + label printer) check:** no-sale (POST `/drawer/open`), POST `/print/:id`, POST `/money-receipts/:id/print`, POST `/receipts/credit/:id/print`, POST `/warranty/:claimId/print`, POST `/labels/send` (counter + admin), GET `/labels` (counter), GET `/receipt/:id`, GET `/orders/print`.
- **Not flow-checked yet:** receipts hub warranty tab (GET `/receipts/warranty`, `/receipts/warranty/:claimId`).
- **Deferred by the spec:** product / customer / supplier import + export → M6; `/reports/*` (purchases, low-stock + its print, dues, spend, count-sheet, inventory, …) and `/damage` → M5. The low-stock report should link to `/admin/reorder`.
- Test data: the dev DB is also the Go-test DB, so Go tests leave `T …` rows behind; run `make e2e-seed` before Playwright or screenshots.

### karots-pos till sources (reference kept from the M1 inputs)

The till must do **everything** karots-pos's till does, in the new design. The karots-pos sources to read are in `~/Projects/go/karots-pos`:

- **`static/js/pos.js` + `pos-*.js`** (Alpine mixins). The areas:
  - **`pos-session.js`:** open/close the register (cash plus a plugin float side by side), withdraw and deposit, no-sale drawer kick, shared-drawer "continue last count".
  - **`pos-catalog.js`:** hierarchical group cards (`product_groups`), the frequently-sold grid, customers, held sales.
  - **`pos-cart.js`:**
    - lots and batches: the "which price?" prompt for per-lot pricing, FEFO, the expiry prompt (sell anyway / write off / back), lot-aware removal
    - found-at-till: a count corrects stock UP when a product is missing
    - service and recipe lines
  - **`pos-input.js`:** barcode/search input, quick-sell handoff (`KQ<barcode>X<qty>`).
  - **`pos-payment.js`:**
    - tenders; card and online go to a picked money account (non-cash leg)
    - change, with Keep (rounding) or Keep all as advance
    - fill remaining; bill and per-item discounts
  - **`pos-checkout.js`:**
    - credit / on-account; a credit customer leaves the tender unpicked, so there's no silent cash default
    - the over-limit override for users with `can_manage_credit`
    - oversell confirm and plugin sale suggestions (such as a clearance markdown)
    - instant change display, then print fire-and-forget as ONE print job
- **Hotkeys:**
  - F1 / "⟲ Menu" resets the **search only**; only New Sale clears the cart.
  - F2 / F3 focus search, F4 hold, F9 discount, F10 pay (amount, then Enter).
  - Enter completes the sale; Escape closes the current dialog.
- **Other till features:**
  - the ⓘ product-info popup (cost shown only with `can_see_cost`)
  - supplier actions at the counter (pay, receive, order, behind the `can_handle_suppliers` flag)
  - cashier expenses
  - credit collection (name/phone search), customer advances and refunds
  - returns by receipt and warranty by receipt
  - the drawer kick (a setting)
  - receipts: thermal ESC/POS, with the receipt style from Appearance
  - kiosk auto-fit on screens under 1080p
  - money-direction cue: green ↓ means cash in, red ↑ means cash out
- **Detailed history of each till feature and its bugs:** karots-pos session memory at
  `~/.claude/projects/-home-karots-Projects-go-karots-pos/memory/MEMORY.md`, in the "TILL UX FIXES" section and linked files such as `credit-as-payment-type.md`, `change-kept-rounding.md`, `till-expiry-warning.md`, `batch-dual-pricing-design.md`, `supplier-at-the-counter.md` and `till-checkout-change-and-print-perf.md`.
- **Plugin till actions** (Reload, Photocopy and similar) are slots filled in M7. M1 must leave the slot: a cashier Quick-action / DrawerSection-style area in the till.

## Open items carried forward

- **Product import / export** stays in M6 (user confirmed 2026-09-29).

- **Till top bar:** the drawer pill and hotkey hints are done. The counter name on the pill has no data source yet.
- **karots-pos deviation:** resuming a held sale is refused while another sale is in the cart. karots-pos silently replaced the cart, which lost a live sale.
- **Login hero:** add the tilted receipt card (yesterday's close) and the "Counter 1 / last closed" pills once an API gives that data.
- **Dock:** areas appear as their pages land (M1–M6). Descriptions come from karots-pos's `templates/layouts/admin.templ`.
- **M7:** `internal/plugin/hooks.go` comments still describe the templ/Alpine-era cashier URLs. Rewrite them when the plugin slots are built.
- **`docs/parity.md`:** 539 karots-pos routes (212 of them plugin routes), 51 ticked. Tick rows only when a route is ported AND flow-checked.
