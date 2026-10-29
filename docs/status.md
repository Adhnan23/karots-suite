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
Android spike passed (shop-made CA, offline sell, replay); **iPhone 13 test first** (throwaway server in
`~/Projects/go/karots-pwaspike`, `./pwaspike`), then the plan.

**Resume here (handoff 2026-10-29):**
1. The user runs the spike (`! cd ~/Projects/go/karots-pwaspike && ./pwaspike`; ufw already allows 8080 and 8443).
   Keep its `state/` folder: the OnePlus trusts that CA.
2. iPhone in **Safari** → `http://<PC IP>:8080` → "iPhone: download profile (.mobileconfig)" → Settings → Profile
   Downloaded → Install ("Not Verified" is normal) → General → About → **Certificate Trust Settings** → turn on
   "Karots Shop CA (spike)" → `https://<PC IP>:8443` → Share → Add to Home Screen → open from the icon. All four
   checks must be green.
3. Offline test: stop the server → close and reopen the app from the icon → sell → "Waiting to send" goes up →
   start the server → Send to PC. Sales show as `REPLAYED` in the server output, in order.
4. **Pass:** run superpowers:writing-plans on the spec (release 1 only; core seams first, then the plugin, then the
   docs and legal for the app at 5,000 LKR). **Fail** (no trust switch, no service worker, or no offline open):
   rework the spec's HTTPS section with the user before planning.

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
