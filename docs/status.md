# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of
every milestone.

## Status (2026-10-07)

**M5 Reports and dashboard is COMPLETE** (`docs/plans/2026-10-07-m5-reports.md`, 2026-10-07). M0–M4 were complete before it (M3 and M4 below).

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

- **M4 Money (2026-10-05)** — dock **Money** → Finance · Cash flow · Lockers · Till sessions · Receipts · Expenses; **Sell** → Sales; **Setup** → Denominations; every page works at 360px:
  - **Lockers:** tiles with live balances + total, archive / bring back, "At the till" default locker + untracked cash-out; per-locker ledger (paged, net from SQL, CR- reprint); Move money, Adjust (bank charge → Expense "Bank charges", interest, count up / down with a reason), Settle card (fee → Expense "Card settlement fee"). `parseAmount` is the money-string parser for every M4 handler.
  - **Expenses:** range / category / search, total from SQL, paged; record from a locker, open till or outside the shop; Void puts the money back (CR- `expense_void`), refused for a plugin-owned expense (`plugin.ExpenseOwner`).
  - **Cash flow:** cash on hand by place, range totals from SQL with own-place moves apart, opening balances on their own line, unified ledger paged + `/print/cashflow`.
  - **Till sessions** (expected / counted / over-short, session page with counts + Z) and **Denominations**; **Receipts hub** (Sales / Cash / Credit / Warranty + plugin kinds, paged, paper sheet with Print again); **Sales** (filters, totals over every match, CSV / XLSX / ODS, sale sheet with reprint and a return refunded from a picked locker / open till); **Finance** (Overview KPIs + trend + payment mix, P&L, Money owed; ECharts lazy chunk with table views).
  - **karots-pos money flaws fixed in both repos:** receivables vs customer advances (`9d1bccd`), expense delete left its cash out (`d66ac18`), Cash Flow totals from 500 rows / transfers counted twice (`9f17be1`), received money counted the change (`3c0cd9d`), only cash gets change (`c5553e7`), and from the M4 review an archived locker took / gave money and stayed the till default (`15f5554`). **Unpushed in karots-pos for the user to push: `c5553e7`, `15f5554`.**
  - **Tests:** `go test ./...`, 101 web unit tests and **80 Playwright flows** green (`admin-money.spec.ts`, `admin-sales.spec.ts` new); `make verify-portable` passes; core plugin-inert; every hand-written file ≤ 500 lines. Parity: **243 rows ticked** (47 in M4).

- **M5 Reports and dashboard (2026-10-07)** — Home is the dashboard; dock **Reports** → the hub + every report (shelf and Ctrl K list them; no tab row); **Sell** → Warranty; **Setup** → Activity (admin only). Every page works at 360px:
  - **Kit** (`features/reports/kit/`): `ReportPage` (Print + `ExportMenu`, `RangePicker noLatest`), `Kpi` / `KpiStrip` (▲▼ vs before), `BarCell`, `AgingBar`, `numbers.ts`, URL search helpers (`rangeSearch` / `reportQS`; **id filters are numbers** — strings were written quoted). Server: `reportRange` / `prevPeriod` / `wantsSheet` — every report answers `?format=csv|xlsx|ods` with every row.
  - **Reports:** Sales (+ hour window, drill-in; the receipts / export are the M4 Sales list's), Peak hours, Top products (Pareto of every product sold), Product sales (vs last year), Sales by cashier, Returns (by return date), Customer / Supplier dues (aging, disabled debtors), Spend by supplier, Purchases, Profit by category (tree), Expenses, Tax (tax-registered only), Inventory valuation (by lots, rows add up), Count sheet (+ `/print/count-sheet`), Top-up & reorder, Losses & Recovery (damage + warranty, Recover sheet), Recipe variance, Service profit. **Folded into existing pages:** Finance (+ export), Cash register = Till sessions (+ totals, export), Batches (+ export), Low stock = Reorder (+ `/print/low-stock`).
  - **Activity** keyset-paged by `(when, source, id)` (rows of one transaction all show), the audit log is its "Changes" source; never the system user. **Warranty register** reuses the counter's `/till/warranty*` endpoints and claim panels (`InfoSheet`). **Dashboard** per the approved mockup (KPIs + sparklines vs yesterday, trend vs the period before, payment mix, top sellers, needs attention, latest sales).
  - **karots-pos flaws fixed in both repos (all pushed):** P&L returns at the sticker price + trend ≠ P&L + UTC day buckets (`275d885`); cashiers merged by name, product-sales UTC days (`d8ee75f`); dues skipped disabled debtors, credited bills aged as unpaid, spend counted returned goods (`3e1b2e1`, `0fabbca`); the till's customer picker stopped at 100 customers (`cd8158f`); report days were the server process's / UTC days, not the shop's (`f93bf97`). Suite-only upgrades: Pareto of the whole, valuation rows by lots, losses list both kinds in range, activity uncapped.
  - **Tests:** `go test ./...`, 104 web unit tests and **87 Playwright flows** green (`admin-reports.spec.ts` new: every report at 1366 + 360 with no side scroll); `make verify-portable` passes; core plugin-inert; every hand-written file ≤ 500 lines. Parity: **296 rows ticked** (53 in M5).

**NEXT: M6 Data and settings** (spec §11): backup / restore, auto-backups, exports, imports (products / customers / suppliers), Settings, Appearance, setup / init, catalog sync endpoint. Done when a karots-pos backup restores into karots-suite. Start a fresh session: read this file, `AGENTS.md`, the spec, then write `docs/plans/<date>-m6-data-settings.md`. **Plan written: `docs/plans/2026-10-08-m6-data-settings.md`.**

**After M6, before plugins (user, 2026-10-07): M6.5 Acceptance and polish.** The core is then complete, so the user runs the suite side by side with karots-pos before any plugin is ported. M6 ends by preparing it:
- a runnable build + short run steps (suite next to karots-pos, own ports and DB);
- a real karots-pos backup restored into the suite, then the same day's figures compared in both (Z report, P&L, stock value, dues);
- a side-by-side checklist by area (till, counter, stock, purchasing, money, reports) for the user to work through;
- real hardware: POS80 receipt printer, label printer, drawer kick (the 11 printer-only parity rows), an old-screen kiosk PC, a phone for the admin.
- the user also runs a **core-only karots-pos** (bootstrap with no plugins) beside it, so the comparison is core against core; give the build steps for both;
- a **"different on purpose" list** (`docs/differences.md`, gathered from every milestone's "Upgrades over karots-pos", "Done:" notes and the user's decisions — e.g. Cash register = Till sessions, Returns by return date, audit = Activity "Changes", Low stock = Reorder, menu cards hard-delete) so the user can tell a deliberate change from a miss and answer keep / change / add back.
The user's notes become a bug / polish list fixed in small commits (same way of working). M7 Plugins starts after that. The M6.5 build is for the user to test, not a customer release (plugin shops can't switch until M7).

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

## Inputs for M6

- **Routes to port** (unticked in `docs/parity.md`): `/backup`, `/data` (+ `/data/backup`, `/data/backup-file`, `/data/restore`, `/data/restore-file`, `/data/settings/export` + import), `POST /restore`, `/settings` (GET / PUT, `/settings/logo` + clear, `/settings/printer/test`), `/setup`, and the product / customer / supplier `export` · `import` · `import/template` rows.
- **Self-update (karots-pos `fd2607f`, 2026-10, ~1,300 lines) — moved WHOLE to M8 (user, 2026-10-08; no suite shop exists before then):** `internal/update` (check a release feed, download gzipped in the background with progress, install only a release signed by the release key whose SHA-256 matches, **back up first**, swap the binary keeping `.old`, relaunch in place — `syscall.Exec` on Linux / a detached copy on Windows — and put the old one back after two failed boots), `cmd/server/update.go`, admin-only `GET /update/badge`, `POST /update/check`, `POST /update/apply`, the update card on Setup → Data, a Setup badge and the dashboard banner (admins). Read its memory / AGENTS notes and `update_test.go` before porting. **Decide first (product):** karots-suite is a separate product, so it needs its own release feed / key / bucket path — never install a karots-pos binary over the suite or the reverse.
- **Self-update, release side → M8 (Ship):** `cmd/release` (`make release`, `release-add` picker form, `release-list`, `release-keygen`, notes), upload to R2 over its S3 API with retry, `shops.json` (a shop's install id + plugin set), `bootstrap -install-id` / `-X`.
- **Already in the suite:** `internal/backup` (restore works for a non-superuser owner, `8e33688`), the narrow `PUT /admin/settings/till-cash` (M4), System → Appearance (M0). Settings must keep owning `stock_take_enabled` (gates Stock-take / Intake), `tax_registered` (gates the Tax report, `/me.taxRegistered`), the default locker / untracked cash, receipt style.
- **Restore must check damage from old write-side bugs** (memory): data written by karots-pos before the fixes above (e.g. returns valued at the sticker price are only a *reading* bug — nothing to repair; check the write-side ones listed in the M3/M4 notes).
- **Exports:** reuse `sheets.go` `writeSheet` (csv / xlsx / ods) and the kit's `ExportMenu`.
- **Still unticked, needs a real printer (POS80 + label printer):** no-sale (POST `/drawer/open`), POST `/print/:id`, POST `/money-receipts/:id/print` (admin + cashier), POST `/receipts/credit/:id/print` (admin + cashier), POST `/receipts/warranty/:claimId/print`, POST `/warranty/:claimId/print`, POST `/labels/send` (counter + admin), GET `/labels` (counter), GET `/receipt/:id`, GET `/orders/print`.
- **Plugin seams waiting for M7:** `slots.reportCards` (hub "More"), `plugin.ActivityContributor` (now gets `Filter.Before` / `Limit` — honour them, the merge filters with `Cursor.After`), `plugin.CollectReadiness` (Top-up; the old `EditURL` inline editor isn't rendered — give it a React slot then).
- Test data: the dev DB is also the Go-test DB. Run `make e2e-seed` before Playwright or screenshots, and kill :3100 / :5173 first. The reports spec sells milk only — the till flows need the seeded soap lots.

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
