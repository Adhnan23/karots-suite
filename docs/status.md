# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of every milestone. Older
handoffs, finished plans and specs live in git history (`git log --diff-filter=D --name-only -- docs`).

## Now (2026-11-01)

**The suite is feature-complete for a shop's day** — till, stock, purchasing, money (cheques included), reports,
people, printing (Classic + Image, receipt and label styles), 22 palettes, ten plugins, file / server mode, Shop
Builder + karots-setup installer, self-update, backups + copies + cloud, the Demo Room, docs and the draft legal pack.
Karots Suite is its own product: it has no link to karots-pos (removed 2026-10-29).

**M8e Pocket Cashier, release 1: DONE 2026-11-01** (branch `m8e-pocket-cashier`, merged into main after the user's
go-ahead). Phones are tills on the shop's wifi (plugin `pocketcashier`, own HTTPS port 8443, shop-made CA, pairing
approved in admin → Phones, which also shows on the dashboard's Needs your attention). The phone sells as the PC till
does (lots, packs, weights, line and bill discounts, serials, split payments incl. Online, note tray, keep change,
customers with balance / limit, quick item, a scanned unknown code → existing item or new item), live or offline
from an encrypted copy; offline actions replay in order and are flagged, never refused. Drawer counted note by note;
no selling before it is open. User tested on the OnePlus 13 (pair, live, offline + replay, flags); iPhone 13 not
retested (no phone at hand). Full Playwright 222 passed.

**Before release 2: a checking session** (user, 2026-11-01) — go through things the user wants looked at first:
- Expiry warnings are a fixed 30 days in core (dashboard `admin_dashboard.go:138`, Stock → Batches
  `admin_batches.go:36`); only the Clearance plugin has its own window (default 14 days, for markdowns). Wanted: a
  shop-wide "warn N days before expiry" setting, maybe per category / product (milk 3 days, medicines 90).
- Expired / damaged stock: written off (damage), then made good only through a supplier (Losses & Recovery:
  replacement, paid / credited, or written off) or sent back first (Purchasing → Returns). Check: a supplier return
  takes stock FEFO and can't pick the expired or damaged lot; and whether "supplier only" fits (a rep's free goods,
  a branch swap…).

**Then: Pocket Cashier release 2** (spec decided in release 1): the counter pages on the phone (receipts, credit,
returns, suppliers, expenses…) and plugin menus / tabs, online first. Known open item: a cashier who moves back to
the PC while the phone still shows them is claimed back by the phone's next send.

**Waiting on the user (outside the code):** the business-name registration number (sole proprietorship applied
2026-10-28 — don't sign shops before it); the lawyer meeting with `docs/legal/karots-suite-lawyer-review.pdf`, then
apply the answers and drop the DRAFT line; the real-printer test when printers arrive.

**Left on purpose (only if a shop runs into it):** a bounced cheque that paid old debt moves the customer's linked
balance, not the old-debt part (the total owed is right); a partial supplier-cheque bounce reopens bills before the
payment's advance part (balance right, bill ageing may differ).

## Done

M0–M6 core (till, counter tabs, stock, purchasing, money, reports, people) · M7 plugins (clearance, requests,
productplus, combos, alternatives, documents, barcodes + merge, lots, AI Assistant, loose versions, packs, pharmacy,
repairs, recharge) · M8a file mode · M8b Shop Builder + karots-setup · M8c self-update + Release · M8c2 backup copies
+ cloud · live users · till left open · smart reorder levels · print as image + receipt styles · palettes (Dock theme,
22 built-ins, custom palettes) · labels as pictures + label styles · M8d Demo Room · docs + screenshots + legal pack ·
cheques (2026-10-29: a payment method wherever money moves; Money → Cheques; user tried it and accepted).

## How we work

1. **Plan, then execute.** A milestone gets a spec (brainstormed with the user) and one plan in `docs/plans/`, tasks
   with files, interfaces, test-first steps and a commit per step. Delete the plan and the spec when it's done.
2. **UI gates:** take screenshots in the palette × mode combinations, review them yourself, carry on without waiting
   unless something needs a product decision. Shots of a whole screen go through `make docs-shots`.
3. **Review** the whole milestone at the end (a fresh reviewer), fix what's found test-first.
4. **No Claude or AI attribution** in any commit or PR, ever.
5. **Work on `main`, local, and unpushed** until the user says otherwise.

What the user rejects: plain or default-looking UI (stock shadcn cards, "coming later" placeholders) — every screen
must look like the approved mockups.

## Design references

- `docs/design/theme.md` is the source of truth for tokens, palettes, login, the dock and the till layout; its
  mockups are in `docs/design/mockups/`.
- **Shell, login and palettes mockup:** https://claude.ai/artifact/MANYgogqsxC4evmSADNkia
- **Till and dashboard mockup (look only):** https://claude.ai/artifact/GkJjnypEes4GmHiHMDzFT5 ("Night Counter" and
  "Studio" directions).

## Open items

- **Login hero:** the tilted receipt card (yesterday's close) and the "Counter 1 / last closed" pills wait for an API
  that gives that data.
- **Till top bar:** the counter name on the drawer pill has no data source yet.
