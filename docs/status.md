# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of
every milestone.

## Status (2026-09-29)

**M1 Till is COMPLETE**: M1a sell (`docs/plans/2026-09-28-m1a-till-sell.md`) and M1b counter operations (`docs/plans/2026-09-29-m1b-till-counter.md`). M0 is complete and reviewed.

M1b delivered (till tabs: Sell · Returns · Credit · Receipts + More ▾ · Ctrl K palette):

- **Returns:** find a sale (receipt/name/phone, day filter), pick lines, faulty → write off, reason, account-first refund; the cash refund leaves the drawer in the same tx; refund slip.
- **Credit:** customers who owe / hold an advance; take a payment (cash, card/online to an account, note) or refund an advance; DP- debt slip.
- **Receipts hub:** sales, cash moves (kind filter), credit payments, warranty claims and plugin kinds (`slots.tillReceiptKinds`); paper preview with an 80/58 mm switch; print again at that width; A4/PDF view; X/Z report page (also linked from the close dialog).
- **Write-off** (damage / shop use / staff, lot pick), **Convert** (break a pack), **Labels** (designer, preview, format, size presets + custom → TSPL).
- **Warranty:** by serial (swap keeps the cover, issues from stock) or by receipt for items without a serial.
- **Suppliers** (supplier flag): pay one amount via the forked waterfall, refund an advance, receive a delivery (walk-in or against an order, line + bill discounts, free qty, expiry, pay-now from drawer/locker/account, keep-the-rest), place an order with wanted-by + printable PO, new products met on a delivery.
- **Expenses** (supplier flag): category chips, paid from the drawer or a locker, tagged to a service.
- **Fixes along the way:** a find box's Enter no longer picks a stale result (scanner race); hotkeys see the latest cart before paint; a till closed with nothing counted closes at zero; the login limiter trusts the socket, not `X-Forwarded-For`.
- **Tests:** `go test ./...`, 77 web unit tests and 37 Playwright flows are green; `make verify-portable` passes.

**NEXT: M2 Catalog and stock** (see the milestone list in the spec). Opus writes the plan, executes it and reviews it.

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

## Inputs for M2

- Read the spec's M2 section, then karots-pos admin products / categories / units / stock / stock-take / labels routes (`internal/web/register_admin*.go`) and their notes in the karots-pos memory.
- **Still unticked from M1 — needs a real printer (POS80 + label printer) check:** no-sale (POST `/drawer/open`), POST `/print/:id`, POST `/money-receipts/:id/print`, POST `/receipts/credit/:id/print`, POST `/warranty/:claimId/print`, POST `/labels/send`, GET `/labels`, GET `/receipt/:id`, GET `/orders/print`.
- **Ported, Go-tested, not yet flow-checked:** credit advance refund (GET `/credit/refund/:id`, POST `/credit/:id/refund`), supplier refund (POST `/:id/refund`), warranty by receipt (GET `/warranty/by-receipt`, POST `/warranty/by-receipt/replace`), receipts warranty tab, quick supplier / product create (`/new`, `/products`, `/products/wanted`). Add flows for them early in M2.

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
