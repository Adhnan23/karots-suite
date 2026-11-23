# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of every milestone and keep it
short: history lives in git (`git log -p -- docs/status.md` has every older handoff, finished plan and area-by-area
battle report). The catalogue and the Onboard / Shelf apps keep their own details in their READMEs.

## Now (2026-11-23)

**The suite is feature-complete for a shop's day and battle-tested end to end.** First shop: Iman Pharmacy
Kalpitiya (`iman-pharmacy-kalpitiya`, install id FC1A79F7, support PIN `make support-pin ID=FC1A79F7`): onboarding
almost done, **no real sales yet**. Released to Iman as **2026.11.23.1850** (the Batches sheet fix; 1441 had this week's sheet work).

- **Iman's catalogue is fixed and ready for a fresh start** (user, 2026-11-23: reset Iman's database, then import):
  `/mnt/nvme_512GB/Downloads/Karots/iman-reset/` holds `suppliers.csv`, `products.csv` (1,723: catalogue names, the
  new category tree, cleaned generics; Iman's barcodes, prices, suppliers), `cashier-menu.csv` and `README.md` (the
  steps: units tab / cap and the Generic Name / Company / Distributor fields first). Built by the catalogue repo's
  `shops/iman_reset.py` from an Onboard backup — rebuild it from a fresh backup if Iman's products change. After the
  reset, Onboard must re-pull Iman's products (its links point at the old ones).
- **The dev database (`suite_db`, `make dev`) is Iman's reset shop** (user, 2026-11-23): 1,723 products, all frozen,
  31 suppliers, the new tree, the cashier menu; sign in as the system user (PIN in `.env`). Dumps in
  `/mnt/nvme_512GB/karots/db-dumps/` (`iman_fresh-20261123.dump` = this data; `suite_db-before-iman-…` = the old
  starter data). The working tree has every plugin on (`make plugins P=all`): `make core-only` before committing.
- **This week's sheet work:** the cashier menu sheet (Data → Spreadsheets and Sheets ▾ on the menu page, MENU
  rules), the products sheet preview (old → new, all or nothing, a till's product skipped not waited on, IMP-1…5),
  "Empty cells clear the field" (IMP-6), Categories → Delete empty… (CAT-1, 2) and **the Alternatives sheet** (Sheet ▾
  on the Alternatives page: `Group, Tier, Product, Barcode`, the whole set, preview first, ALT-9…18; import admin
  only). Released 2026.11.23.1441.
- **Battle tests: all done** (7 core areas, every plugin, install / run). Rule first (AGENTS.md): rule, failing test, code.

**Last full run (2026-11-23, after the Alternatives sheet):** all green — Go, Vitest 1,228, check-core, Playwright
275 passed. Mutation on `plugins/alternatives`: 0 lived. Pocket reads
`KAROTS_POCKET_PORT` before `.env` is loaded (open bug): a dev shop must get it from the environment (`start.sh`
does). Playwright now uses 3100+ / Pocket 8543+ (2026-11-23), so a dev server on 3000 / 8443 can keep running. Two specs race (`goto('/login')` right after an API
logout: palettes, admin-people) — they pass on a rerun; make them wait for the redirect.

## Next — the user's call

0. **Next (decided with the user, 2026-11-23)** — the site review is audited (table in
   `docs/reviews/2026-11-23-site-review.md`); Demo Room + site deployed with noindex / descriptions / sitemap dates.
   In order:
   1. ~~Release refuses untested code~~ — done (BLD-6, 2026-11-23). Its first run caught a real bug: Batches → Set
      count / Not there set the total from old numbers on a fast click (fixed, STK-5). Full run green on b3e5fd57.
   2. **Low-spec benchmark** on the `karots-main` (Xubuntu) and `win10` VMs, Iman's 1,723 products + a Demo Room
      year: 2 cores / 4 GB → 2 / 2 GB → 1 / 2 GB; start-up, till search (< 0.3 s), a sale (< 1 s), big reports,
      backup, update. Lowest pass = minimum, one up = recommended; publish as tested, never invented.
   3. **One pass of wording** on the site, guides and legal pack (`LICENSE`, Shop Rules, `make agreement`) from the
      user's policies: one licence = the one PC running the Suite (cashier PCs and phones only connect); 3 months'
      warranty, **genuine bugs always fixed free**; service plan = cloud backups, support, major upgrades, small
      changes, maintenance (not "every new feature" / "help any time"); support without a plan is charged; the last
      cloud copy kept **30 days** after the plan ends (bucket lifecycle), local backups carry on; power cut = no
      finished sale lost, a UPS keeps you selling; testing worded as what we test; who-pays table; backups (6 h, keep
      28, copy folder, cloud sealed, local not encrypted); pharmacy register is a help, not NMRA compliance; AI
      prompts carry only product names, barcodes, categories; tested printers; Shelf = full product rights, cost
      included, given by the admin for setup only and taken away after (by design, user 2026-11-23: no code change).
   4. Open for the user: Cloudflare's managed robots.txt (AI bots blocked on the site; its `Allow: /` overrides the
      demo's `Disallow: /`, noindex still keeps the demo out).
1. **Next: Iman's `alternatives.csv`** in the catalogue repo beside the iman-reset files (generic name + strength
   from the names, AI where it helps; columns `Group, Tier, Product, Barcode`), for the user to import on the dev
   shop and check the preview. **Group by fields was dropped** (user, 2026-11-23): exact field values miss twins
   (generic names with several ingredients joined by "+"); AI groups better, a sheet is simpler. Later: the AI
   Assistant's group maker reads custom fields (today it sends only the name and its own notes, and skips names with
   no digit). Then **supplier by field**: 550 Iman products have no supplier, 431 of them a Distributor; 134
   distributor spellings vs 31 suppliers, so it needs a distributor → supplier mapping (several to one).
2. Iman's reset at the shop (the user), then the stock-take and the first real sale; a Release with this week's work
   first. Later product changes at Iman: a fresh Onboard backup → `shops/iman_reset.py` / the update file.
3. Open for the user: ~12 catalogue generics that look short of Iman's typed ones (D3 + K2: 1000D, D One 1000 /
   2000, Vidan D 2000; Chlofen DM, Sinarest Linctus + phenylephrine; Orofer + folic acid; Immune Up; GLA 120 +
   vitamin E; Nugesic: Iman says B vitamins, the catalogue nimesulide).
4. Ideas, built only when the user says:
   - **Onboarding without the mess** (for shop #2): products from the catalogue through Onboard into a ready-made
     category tree per shop kind, staff adding only what's missing; and a `tidy` command for a shop already messy.
   - **A supplier by field**: Product Plus rules like "Brand = Nestlé → supplier X" for products with no preferred
     supplier of their own; needs a generic core seam (a fallback supplier) so core never names Product Plus.
   - A note box on the till's plain Hold (only Send to cashier has one).
   - Read the 84 unread mutation survivors in Setup's launcher and print helper when a cashier PC's printer is next
     touched.
5. Later: the install stick, the whole-system check, Pocket Cashier at a shop.

## The family

- **Karots Suite** (this repo; pushes to GitHub `origin` and GitLab `gitlab`).
- **Karots Suite Onboard + Karots Shelf** (`~/Projects/flutter/karots_suite_onboard`, one Flutter repo, both apps on
  Android, Linux and Windows). `make` builds both APKs + both Linux apps into its `dist/`; Windows = GitHub Actions on
  push. Onboard = the user's own setup app (install ID 3EE0E57C); Shelf = shop staff's app, free, in Iman's build.
- **Karots Suite Catalogue** (`~/Projects/karots-suite-catalogue`, private): Onboard gets it as an **update file**
  built from the user's **latest** Onboard backup (never wipe and re-import). Catalogue work paused until a shop
  needs it. Karots files in the user's Downloads: `/mnt/nvme_512GB/Downloads/Karots/`.
- **Karots Suite site** (`~/Projects/karots-pos-site`, to be renamed `karots-suite-site`; the old POS site on its
  `archive/karots-pos` branch): suite.karots.lk, a Cloudflare Worker (`bunx wrangler deploy`). Guides, plugin pages
  and screenshots come from this repo (`bun run sync`; tour frames: `make tour-shots`). Prices live in its
  `src/data/pricing.ts` (2026-11-22 list; the legal pack still has the old one).
- **Web Demo Room**: demo.karots.lk, a container on the OVH VPS behind nginx-proxy-manager (`deploy/demo-room/`,
  `make demo-room-deploy` after `make demo`); still asks for the hourly code. The site offers it only when it answers.
- Karots PIN Vault is the user's own app for all their projects: never tie it to Karots Suite.

## Not yet tried for real

- The XP-365B label printer (nothing printed on it yet): its roll test decides whether Classic labels stay.
- Pocket Cashier at a shop. Known small gaps left on purpose: an offline repair, on-account sales not lowering the
  balance on the phone, a return's cash not compared with the PC's, no session after an offline sign-in following a
  cut, a refused sale's note without items or total. **User's call, open:** Pharmacy cuts the patient's name at 360 px
  and Print & Copy squeezes its total on the phone; a reload past the float isn't flagged under What came in.
- A sales person's flow on a real counter; a Search only Sell page layout on a real counter.
- A cash drawer (Iman has none for now).
- Recovering a shop already down from an update cut, through Setup → Update (needs the user at the VM).

## Waiting on the user, in no hurry

- Iman's charges; the business registration number (don't sign shops before it); the lawyer meeting with
  `docs/legal/karots-suite-lawyer-review.pdf`.
- Test backups in the old `aiman-pharmacy-kalpitiya` cloud folder age out by about Dec 4 (30-day lock; the user
  deletes them in Cloudflare, not me). Keep `karots-main` / `karots-cashier` VMs off or expect uploads there.

## Decisions and notes to keep

**Iman's setup:** new PC (i3 4th gen, 8 GB, 120 GB SSD, 20" 1600x900) = admin + main PC, file mode, cloud backup on;
the owner's Linux laptop = cashier PC with the XP-80T (receipts) through the print helper; XP-365B (labels) on the main
PC; one wired 2D scanner. Plugins: aicatalog, alternatives, combos (the user's gift, not charged), pharmacy,
pocketcashier (the user won't mention it to him), productplus. `dist/aiman-pharmacy-kalpitiya/` is old: never install
it. Al Raj, Mum Digital and MUM Motors stay on Karots POS.

**Releases:** only when the user says. Notes are cut from feat/fix commit titles; for anything but a tiny release,
write them by hand and publish through the Builder's release API (`POST /api/shops/<key>/release` with `{"Notes"}`).
Before one: `make test-all` (Release refuses code it hasn't passed, BLD-6).

**Receipts:** pictures in the shop's style; the hidden plain text stays for a failed picture and slips with no
template. Product local names don't print; only the shop's own local name does. Check slips on the till, not only in
Appearance, and with real data. More receipt styles only when a shop asks.

**Send to cashier:** a Users choice **Sales person** (`users.sends_to_cashier`): same Sell screen, no money; the
server's allow-list is `internal/api/sales_person.go`, money routes carry `middleware.RequireHandlesMoney`. A ticket is
a held sale with `to_cashier`; unopened after 2 minutes it moves on; closing a till deals with its tickets first.
Sales people stay off Pocket Cashier for good.

**How expiry really works (the pharmacist, 2026-11-01):** the supplier's notice days are a warning, never a block;
the claim is what's left when the rep takes it; made good as the same product, money or credit, or nothing. A
different product: settle as Credited us, enter what they bring as a delivery, the credit pays that bill. Damaged on
arrival = a supplier return from that delivery's lot.

**Direction:** each shop gets its own cashier page and receipt (what customers see), not fixed themes. Idea, not
built: a practice mode (sales and printing that don't count).

**Dropped (user):** prices per customer / customer group; promo videos. **Parked:** open-price items (memory
`open-price-items`), staff commission, delivery orders, advance orders. **Future plugins, not now:** loyalty points,
gift vouchers / credit notes, several shops.

**Left on purpose (only if a shop runs into it):** a same-product Replaced it lot doesn't remember that supplier; a
bounced cheque that paid old debt moves the linked balance, not the old-debt part; a partial supplier-cheque bounce
reopens bills before the payment's advance part; RET-14 (mixed tax rates, a line's refund share) needs line tax
stored; two phone edits at the very same moment can still pair a phone with two customers (the auditor flags it).

**Power cuts (Iman has generators, no UPS):** confirmed sales survive, in-flight ones vanish whole, up in 7–9 s;
receipt numbers jump ~30 after a cut. Backups, updates and the program swap are cut-safe (battle-tested 2026-11-22).

## Working here

- **Tests:** `make test-all` (~9½ min, side by side; Go and Vitest at `nice 15`), `make battle` (~29 min). Details in
  AGENTS.md → Dev workflow and **Rules and battle tests**. Long runs: detached, logs on disk, NVMe, mutant shards in
  a memory cap (`/tmp` is RAM here — check `df -h /tmp` before and after).
- **Type-check the build the way it runs:** `cd web && bunx tsc -b`.
- **Power cuts happen here.** After one: `git fsck`, `make check-core` (a cut during a build leaves the plugin lists
  on: `make core-only`, then `git checkout -- internal/spa/dist/.gitkeep`). Stage named files, never `git add -A`
  after a build (it once committed the dist `.gitkeep` away).
- **Dev server:** `make plugins P=all && make migrate && make dev`; dev logins Admin 0771234567 / 1234, Cashier
  0771111111 / 1111, system user 0000000001 / 2273. `make core-only` before committing.
- **VMs** (`virsh -c qemu:///system`): `karots-main` (Xubuntu, snapshot `before-powercut`), `karots-cashier`, `win10`
  (SSH with this PC's key; printer passthrough: `virsh … attach-device win10
  /mnt/nvme_512GB/vmhd/win10-printer-hostdev.xml --config`). Only one VM can hold the XP-80T. Shop `windows-test` in
  `shops.json` is for Windows tests (never Released). Printer queues here: `XP80T`, `POS80` (default; `cancel -a
  POS80` before plugging an old one in), `XP365`.
- **The user tests with real sales and scans of real paper** (phone scans in `/mnt/nvme_512GB/Downloads/`,
  screenshots in `~/Pictures/Screenshots/`). Don't print on the real printer until a picture looks right.

## Done

M0–M6 core (till, counter tabs, stock, purchasing, money, reports, people) · M7 plugins (clearance, requests,
productplus, combos, alternatives, documents, barcodes + merge, lots, AI Assistant, loose versions, packs, pharmacy,
repairs, recharge) · M8a file mode · M8b Shop Builder + karots-setup · M8c self-update + Release · M8c2 backup copies
+ cloud · live users · till left open · smart reorder levels · print as image + receipt styles · palettes · labels as
pictures + label styles · M8d Demo Room · docs + screenshots + legal pack · cheques · M8e Pocket Cashier release 1 and
2 · pharmacy expiry work · Send to cashier · print helper + cashier PCs' printers · AI Assistant jobs · default unit ·
Karots Shelf · import by ref · receipt line rules + text size · stock freeze · keyboard-only Sell page · Sell page
layouts · Alternatives sheet · cashier menu sheet (Data → Spreadsheets and Sheets ▾ on the menu page, MENU rules) · products sheet preview (old → new, all or nothing, IMP rules) · **battle tests: 7 core areas, every plugin, installing and running a shop (2026-11-18 → 22)**.

## How we work

1. **Rule first:** a change in behaviour starts in `docs/rules/` (AGENTS.md hard rule), then a failing test, then code.
2. **Plan, then execute.** A milestone gets a spec (brainstormed with the user) and one plan in `docs/plans/`; delete
   both when it's done.
3. **UI gates:** screenshots in the palette × mode combinations, reviewed yourself; stop only for product decisions.
4. **Review** the whole milestone at the end; fix what's found test-first.
5. **No Claude or AI attribution** in any commit or PR, ever. **Work on `main`, local**; the user pushes.

What the user rejects: plain or default-looking UI — every screen must look like the approved mockups.

## Design references

- `docs/design/theme.md` is the source of truth for tokens, palettes, login, the dock and the till layout; mockups in
  `docs/design/mockups/`.
- **Shell, login and palettes mockup:** https://claude.ai/artifact/MANYgogqsxC4evmSADNkia
- **Till and dashboard mockup (look only):** https://claude.ai/artifact/GkJjnypEes4GmHiHMDzFT5

## Open items

- **Login hero:** the tilted receipt card (yesterday's close) and the "Counter 1 / last closed" pills wait for an API
  that gives that data.
- **Till top bar:** the counter name on the drawer pill has no data source yet.
