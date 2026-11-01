# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of every milestone. Older
handoffs, finished plans and specs live in git history (`git log --diff-filter=D --name-only -- docs`).

## Now (2026-10-29)

**The suite is feature-complete for a shop's day** — till, stock, purchasing, money (cheques included), reports,
people, printing (Classic + Image, receipt and label styles), 22 palettes, ten plugins, file / server mode, Shop
Builder + karots-setup installer, self-update, backups + copies + cloud, the Demo Room, docs and the draft legal pack.
Karots Suite is its own product: it has no link to karots-pos (removed 2026-10-29).

**Next code work: M8e Pocket Cashier** — the phone cashier, an installed web app sold as the "Pocket Cashier app"
(plugin `pocket`, 5,000 LKR). Spec approved 2026-10-29: `docs/specs/2026-10-29-m8e-pocket-cashier-design.md`.
Spike passed on Android (OnePlus 13, 2026-10-29) and **iPhone 13 (iOS 27, 2026-10-30)**: shop-made CA trusted via
`.mobileconfig` + Certificate Trust Settings, Home Screen app with service worker, offline sell, replay in order.

**Resume here (on branch `m8e-pocket-cashier`, merged into main when done):** execute `docs/plans/2026-10-30-m8e-pocket-cashier.md` from **Task 26 (docs and legal)**.
Tasks 1–25 are done (1–19 on 2026-10-30, 20–25 on 2026-11-01): core seams (replayable sales + flags, tills at a given time, one person one device,
plugin listeners, the till copy, surfaces), the plugin's Go half (CA + phone certificate, pairing, copy + feed, replay,
review, guard, service worker) and the phone app (encrypted store, sync, pair + sign-in, Sell, Pay, Held, Menu, setup page), admin Phones, the
plugin ports in Builder + Setup, and `e2e/pocket.spec.ts` (pair, takeover, sell live, sell offline + replay) +
`e2e/pocket-shots.spec.ts` (UI gate, `SHOTS=pocket`). Plugin key is
**`pocketcashier`** (core has a `'pocket'` locker kind). The executor's ledger with every ruling is
`.superpowers/sdd/2026-10-30-m8e-pocket-cashier/progress.md` (git-ignored; keep it — the final review reads it).
Main rulings: a sale rung before its till's close counts now as `late` (`clock` = rung >5 min in the future only);
replay serialises per phone with an in-process lock (`sales.client_ref` UNIQUE is the backstop); the phone sells at
FEFO / shelf price (no lot or pack picker; a pack's barcode adds the pack). `internal/docscheck` fails `go test ./...`
until Task 26 adds the app to the docs — expected. UI gates for the phone screens are taken in Task 25.
The spike (`~/Projects/go/karots-pwaspike`) can go once Task 10 (certificates) is committed; its `state/` CA is
trusted by both test phones (remove that profile from them then).

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
