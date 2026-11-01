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

**Next session: core work for the first shop, a pharmacy** (decided with the user 2026-11-01). He catalogues and
bills medicines; other goods go in as **Quick items** (one "Other goods" category; the admin clears Review now and
then). He wants to stop overstocking (smart reorder — built), stop missing expiries (A below) and see what he lacks
(Low stock + requests plugin — built). No Pocket Cashier for him yet (agreed). Design each item with the user first
(spec → plan), then build test-first. Work in this order:

A. **Expiry — do first** (all core; the pharmacy plugin may only add extras through hooks):
   1. **Expiry warning window + 2. per-supplier days: DONE 2026-11-01** (one job; user chose per supplier, not per
      product). Settings → Stock has the shop's number (30 by default); a supplier's form can have its own (blank =
      the shop's). A lot follows the supplier of its delivery, else its product's preferred supplier, else the shop.
      Batches, the dashboard and the export use it; each lot shows its supplier. A warning only, at exactly the number
      (an owner who wants time to call types 70 for 60).
   3. **Claims before they're settled: DONE 2026-11-01.** "From this supplier" also lists close-to-expiry lots and is
      on the supplier page. **Told them** keeps the date; **Give back** (there and on Batches) takes a count, moves any
      difference between the product's lots, and waits on the supplier with no time limit until settled with the
      existing Recover form. It replaced "Write off & settle". Goods that **arrived damaged** still go through a
      supplier return from that delivery's lot (`BatchID`).
   4. **Supplier recall of a lot** (the pharmacist: the stock looks fine, but the supplier says the batch is bad and
      takes the whole batch back): returning the lot is already possible (Purchasing → Returns, that lot, all of it).
      Add: one "supplier recalled lot X" step that returns everything left of it; **hold the lot** so the till stops
      selling it until the rep comes; and **who bought it** (sales record each line's lot — customers on account,
      receipts for walk-ins) for medicines that need a call-back. They then **replace or give the money back** —
      already in core: the return credits their bills (or they refund cash), and a replacement is a normal delivery
      the credit pays for.
   **How it really works (the pharmacist, 2026-11-01)** — design to this, not to a strict rule:
   - The supplier gives the notice days (e.g. 60): tell them before that date. Told late, they *sometimes* refuse —
     so it's a warning and a reminder, never a block; they may change their mind and take it anyway.
   - Days passing ≠ the whole lot is claimed: some units sell meanwhile, so the claim is what's left when they take it.
   - The rep may come any time — a few days after being told, before expiry, or after it — and may still replace
     expired stock.
   - Made good as: the same product (often a **new lot at a different price** — same price or the shop pays the
     extra: **already in core**, Losses & Recovery restocks a new lot with expiry / price / extra), **money / credit**,
     nothing (written off) — or **a different product** as the replacement (an alternative medicine or biscuit):
     **missing**, a replacement always restocks the loss's own product.
   - **Goods that arrived damaged:** already in core — return them from that delivery's lot (Purchasing → Returns).
B. **Customers:** 5. **prices per customer / customer group** (only retail and wholesale today).
C. **Small ones:** 6. **discount received when paying a supplier** (a received bill can't be edited and a payment
   only pays, so 9,800 on a 10,000 bill leaves 200 owed — add "discount received" that closes the rest, lowers the
   payable and shows in money reports); 7. **drop `customers.loyalty_points`** (unused; loyalty will be a plugin).

**Parked:** the **open-price plugin** (goods tracked by value, never count — typed price, supplier bills at cost,
damage as an amount; build only if Quick item is too much work for a shop); staff commission and delivery orders
(only if a shop asks). **Future plugins, not now:** loyalty points, gift vouchers / credit notes, several shops.

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
