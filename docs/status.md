# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of
every milestone.

## Status (2026-09-29)

**M2 Catalog and stock is COMPLETE** (`docs/plans/2026-09-29-m2-catalog-stock.md`). M0 and M1 (till: sell + counter) were complete before it.

M2 delivered (admin dock areas **Inventory** and **Setup**; every page works at 360px):

- **Admin kit** (`web/src/components/admin/`): PageHead, DataList (table ≥768px, cards below, action columns on both), FormSheet (dialog / bottom sheet), PickerShell, Pager, fields, ConfirmButton (red or plain), EmojiPicker (searchable, curated shop aisles — a till PC has no emoji keyboard). Breadcrumb + area page tabs in the shell, dock current-area dot + badges, phone More sheet lists every page.
- **Setup:** Units · Categories (nested tree, add a sub-category in place, move with a no-loop guard, delete lifts children; shared CategoryPicker with inline find-or-create) · Cashier menu (nested cards, reorder, products per card with emoji; **Delete really deletes** — a card is layout, not a record) · Barcode labels (the counter's designer + a browser-printable **label sheet** `/print/labels`, also on the counter).
- **Inventory:** Products (search incl. numeric barcodes, category with sub-categories, running low, disabled; form with cost⇄markup⇄selling, reprice stock on hand, barcode generate, duplicate; + Add barcode on rows) · Items to review (finish backfills past sale cost) · Stock (on hand + per-product lots with lot prices; adjust from the lot that's short; record write-off; movements with who/kind/period + CSV/Excel) · Batches (expired first, write off one or all) · Stock-take (count screen kept across pages, 2+ lot-price warning, count sheet CSV/XLSX/ODS download + import) · Stock intake (scan-or-type restock/create, 1.4 markup, rapid loop + Another like last, labels with top/bottom slots + preview, session undo) · Conversions (run, edit, disable/enable, run history + CSV) · Services & recipes (recipe cost split, what's left of the price).
- Stock-take and intake follow the Settings toggle (API 403, route redirect, nav hidden).
- **Where the suite deliberately differs from karots-pos** (user-approved): cashier-menu cards are deleted, not soft-disabled; counts on stock-take survive paging; dialogs never use `window.confirm`.
- **Bugs fixed on the way:** a numeric barcode in a search box was dropped from the URL (router parses params as JSON — `lib/search.ts`); a scanned code + Enter raced the product picker's debounce; label barcodes are inline SVG so they draw and print at once; DataList actions were missing on phone cards; counter "New product" row was covered by its own search results.
- **Tests:** `go test ./...`, 97 web unit tests and **64 Playwright flows** (one worker; flows make their own expired lots) are green; `make verify-portable` passes. Parity: **143 rows ticked** (92 in M2, incl. the M1 leftovers now flow-checked).

**NEXT: M3 Purchasing and people** (spec §11): suppliers (+statements, money flows, receiving discounts, returns), POs → receive/GRN, reorder, customers (+statements, credit, opening balance), users/permissions. Opus writes the plan, executes it and reviews it.

**Porting rule learned in M2:** read karots-pos's `static/js/<screen>.js` and templ, not just the Go handler (intake was first built from the handler and missed its label slots, rapid loop and generate). Where karots-pos is flawed, don't copy the flaw.

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

## Inputs for M3

- Read the spec's M3 section, then karots-pos admin suppliers / purchases / purchase-returns / customers / users routes (`internal/web/register_admin*.go`), their templ **and** `static/js/purchasing.js` + `reorder.js`, and the karots-pos memory notes (`supplier-money-flows.md`, `supplier-at-the-counter.md`, …).
- Reuse the M2 admin kit and pickers (ProductPicker, CategoryPicker, supplier options). The counter already has supplier pay/receive/order (`features/counter/suppliers/`) — the admin screens should share its line editor and totals, not copy them.
- **Still unticked — needs a real printer (POS80 + label printer) check:** no-sale (POST `/drawer/open`), POST `/print/:id`, POST `/money-receipts/:id/print`, POST `/receipts/credit/:id/print`, POST `/warranty/:claimId/print`, POST `/labels/send` (counter + admin), GET `/labels` (counter), GET `/receipt/:id`, GET `/orders/print`.
- **Not flow-checked yet:** receipts hub warranty tab (GET `/receipts/warranty`, `/receipts/warranty/:claimId`).
- **Deferred by the spec:** product/customer/supplier import + export → M6; `/reports/*` (batches, low-stock, count-sheet, inventory, …) and `/damage` → M5.
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

- **Till top bar:** the drawer pill and hotkey hints are done. The counter name on the pill has no data source yet.
- **karots-pos deviation:** resuming a held sale is refused while another sale is in the cart. karots-pos silently replaced the cart, which lost a live sale.
- **Login hero:** add the tilted receipt card (yesterday's close) and the "Counter 1 / last closed" pills once an API gives that data.
- **Dock:** areas appear as their pages land (M1–M6). Descriptions come from karots-pos's `templates/layouts/admin.templ`.
- **M7:** `internal/plugin/hooks.go` comments still describe the templ/Alpine-era cashier URLs. Rewrite them when the plugin slots are built.
- **`docs/parity.md`:** 539 karots-pos routes (212 of them plugin routes), 51 ticked. Tick rows only when a route is ported AND flow-checked.
