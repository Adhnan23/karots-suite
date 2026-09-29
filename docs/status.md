# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of
every milestone.

## Status (2026-09-29)

**M1a Till: sell is COMPLETE** (plan `docs/plans/2026-09-28-m1a-till-sell.md`), awaiting Opus review. M0 Foundation is complete and reviewed.

What M1a delivered (the till at `/till`):

- **Drawer:** open with a counted float (continue last count, draft recovery), close and reconcile (also in sign-out mode), withdraw and deposit with the money cue, no-sale, the CR- slip prompt, and the drawer pill. An empty count closes at zero.
- **Catalogue:** a scan/search box (instant clear for scanners), group chips and drill-down, frequently sold, ⓘ info (cost gated), and a quick item for unknown codes.
- **Cart, lots and prompts:** a qty stepper, line discounts, lot labels and spill, serials, which-price and expired-lot prompts (sell anyway / write off / back), and the clearance suggestion via a plugin seam.
- **Pay and tenders:** a customer picker (add customer, edit limit), cash/card/online/on-account/plugin tenders, quick cash, fill, split, keep change or round to account, and a bill discount.
- **Checkout prompts:** shortfall → on account, the over-limit admin approval, found at till (sell more than the count), and a single-post guard.
- **Printing:** thermal printing in Go. "Ask before printing" shows an instant change overlay (print / next sale); auto-print sends one job with the drawer kick merged in and shows a change banner in the cart.
- **Hotkeys:** F1 (clear search only), F2/F3, F4 hold, F9 discount, F10 → amount → Enter, Enter for the next sale, Esc, and a scroll-wheel guard on number inputs.
- **Persistence:** the cart survives auto-lock and reload (sessionStorage per user); held sales live on the server.
- **Slots:** `web/src/plugins/slots.ts` (tillMenu, tillActions, drawerSections, tillTenders, tillBeforeCheckout, tillAfterSale) plus Go `TenderMethods`.
- **Security fix:** the login rate limiter now keys on the socket IP. It trusted `X-Forwarded-For`, so a spoofed header bypassed the PIN brute-force guard. karots-pos likely has the same hole (Echo's default `RealIP`); fix it there too.
- **Tests:** `go test ./...`, 75 web unit tests and 25 Playwright e2e tests (13 golden till flows) are all green, and `make verify-portable` passes. The e2e specs share `web/e2e/fixtures.ts`, which waits out the login limiter.

**NEXT: M1b Till: counter operations.** Opus writes the plan (`docs/plans/<date>-m1b-till-counter.md`), executes it, and reviews it.

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

## Inputs for the M1b plan

- **Scope:**
  - returns and refunds, write-off (the damage page), convert
  - credit collection, advances and refunds
  - the receipts hub with reprints, the browser receipt view and the Z report
  - labels, warranty
  - suppliers and expenses at the counter
- **Tabs:** add them to the tab row in `TillShell` (today it has Sell only).
- **Cart across tabs:** the till state lives in `TillProvider`, so the cart survives switching to Returns. Keep it that way.
- **Real-printer check (POS80) during review** for the unticked parity rows: no-sale (POST `/drawer/open`), POST `/print/:id`, POST `/money-receipts/:id/print`. POST `/damage` stays unticked until M1b ports the page (M1a only does the expired-lot write-off).
- The karots-pos sources and memory below still apply.

### karots-pos till sources (from the M1 inputs)

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
- **`docs/parity.md`:** 539 karots-pos routes (212 of them plugin routes), 17 ticked. Tick rows only when a route is ported AND flow-checked.
