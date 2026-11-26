# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of every milestone and keep it
short: history lives in git (`git log -p -- docs/status.md` has every older handoff, finished plan and area-by-area
battle report). The catalogue and the Onboard / Shelf apps keep their own details in their READMEs.

## Now (2026-11-26)

**The suite is feature-complete for a shop's day, battle-tested end to end, and measured on low-spec PCs.** First
shop: Iman Pharmacy Kalpitiya (`iman-pharmacy-kalpitiya`, install id FC1A79F7, support PIN `make support-pin
ID=FC1A79F7`): onboarding almost done, **no real sales yet**. Latest Release to Iman: **2026.11.25.0951** (the Catalogue
plugin + Shelf's From catalogue; Shop Rules 1.7). Iman updates from Setup → Data → Updates.

**Waiting for the next Release** (committed 2026-11-26, not released): Pocket's port after `.env` (PKT-16), the
sign-in page's "Last closed" pill (DRAW-20), the faster sales list (RPT-38), the note on Hold (SALE-53), a cashier
PC's main-PC address pasted with a space before `http://` (SET-10). A Release needs a green `make test-all` on that
commit first (BLD-6). Next: this Release.

- **Iman's catalogue is ready for a fresh start** (user: reset Iman's database, then import):
  `/mnt/nvme_512GB/Downloads/Karots/iman-reset/` (`suppliers.csv`, `products.csv` 1,723, `cashier-menu.csv`,
  `README.md` with the steps), built by the catalogue repo's `shops/iman_reset.py` from an Onboard backup. After the
  reset Onboard must re-pull Iman's products.
- **The dev database (`suite_db`) is Iman's reset shop** (1,723 products, all frozen, 31 suppliers). Dumps in
  `/mnt/nvme_512GB/karots/db-dumps/`.
- **Selling terms (2026-11-24):** four plans (Core, Karots Suite, Easy Pay, Complete Shop), bugs always free, the
  service plan, one licence per PC — on suite.karots.lk and in the legal pack (Shop Rules 1.7, `LICENSE`, DRAFT for
  the lawyer). **PC:** minimum Core 2 Duo / 2 GB, recommended i3 2nd gen / 4 GB (`docs/guide/install.md`; bench
  results in `/mnt/nvme_512GB/karots/bench/results.jsonl`).

**Last full run (2026-11-24, on 68388099):** all green — Go, Vitest, Playwright, check-core (579 s). Since then
(2026-11-26) `make test`, Vitest and the till's Playwright spec pass; one flake seen (`TestRCH6_AmountBack`, Ideas).

## Next — the user's call

1. **Iman's visit** (the user has the reset files in `iman-reset/`): reset the database, import suppliers, products
   and cashier menu, then **the Alternatives sheet** (`alternatives.csv`: 243 groups, 971 products; same generic,
   strength and form; imported on the dev shop 2026-11-23; the user's re-import on 2026-11-26 matched it row for row, so it's proven), then the stock-take and the first
   real sale. (Iman's 550 products with no supplier get one from their first order / delivery.)
2. Ideas, built only when the user says:
   - **Payment tracking in the Builder** (memory `payment-reminder-idea`) — needed before many Easy Pay shops; a fresh
     session with a design talk first (user, 2026-11-26).
   - Recharge through modems (memory `recharge-modems-idea`) — parked until the user has the hardware.
   - Dropped (user, 2026-11-26): a ready category tree per shop kind (each shop has its own; Iman's was tidied with AI).
   - A flaky test to look at: `TestRCH6_AmountBack` (recharge battle) failed once in a full `make test`, passed 8/8
     alone — it audits the shared shop while other tests write to it.
3. Later: the whole-system check, Pocket Cashier at a shop.

## The family

- **Karots Suite** (this repo; pushes to GitHub `origin` and GitLab `gitlab`).
- **Karots Suite Onboard + Karots Shelf** (`~/Projects/flutter/karots_suite_onboard`, one Flutter repo, both apps on
  Android, Linux and Windows). `make` builds both APKs + both Linux apps into its `dist/`; Windows = GitHub Actions on
  push. Onboard = the user's own setup app (install ID 3EE0E57C); Shelf = shop staff's app, free, in Iman's build.
- **Karots Suite Catalogue** (`~/Projects/karots-suite-catalogue`, private; its Catalogue app + Publish online since 2026-11-24): Onboard gets it as an **update file**
  built from the user's **latest** Onboard backup (never wipe and re-import). Catalogue work paused until a shop
  needs it. Karots files in the user's Downloads: `/mnt/nvme_512GB/Downloads/Karots/`.
- **Karots Suite site** (`~/Projects/karots-pos-site`, to be renamed `karots-suite-site`; the old POS site on its
  `archive/karots-pos` branch): suite.karots.lk, a Cloudflare Worker (`bunx wrangler deploy`). Guides, plugin pages
  and screenshots come from this repo (`bun run sync`; tour frames: `make tour-shots`). Prices live in its
  `src/data/pricing.ts` (the four plans; the legal pack's Shop Rules §15 says the same).
- **Web Demo Room**: demo.karots.lk, a container on the OVH VPS behind nginx-proxy-manager (`deploy/demo-room/`,
  `make demo-room-deploy` after `make demo`); still asks for the hourly code. The site offers it only when it answers.
- Karots PIN Vault is the user's own app for all their projects: never tie it to Karots Suite.
- **Karots secrets** (`~/Projects/karots-secrets`, private GitHub `karots-secrets`, 2026-11-26): every key and `.env`
  of the family and the user's other projects (`files.txt`) in one gpg-encrypted file; `./secrets.sh save` after any
  key or `.env` change, then push. The password lives only with the user (paper + PIN Vault).

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
  deletes them in Cloudflare, not me). Keep `karots-main` / `karots-cashier` VMs off or on the isolated `bench`-style
  net: `karots-main` still holds an old install with cloud on.

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
layouts · Alternatives sheet · cashier menu sheet (MENU rules) · products sheet preview (IMP rules) · **battle tests: 7 core
areas, every plugin, installing and running a shop (2026-11-18 → 22)** · Release needs a green test-all (BLD-6) ·
low-spec bench + RPT-38 year reports 35 s → 1 s · the new terms, four plans, system requirements, site redesign
(2026-11-24) · **Supplier by field** (Product Plus: one Brand / Company field gives products with no supplier of their
own a supplier, until their first order or delivery; core hook `plugin.RuleSupplier`, SUP-11/12, PPF-10…13; test-all
green on 0e56c4a, 2026-11-24) · **online catalogue** (2026-11-25: the Catalogue app + Publish to R2, the
Catalogue plugin `plugins/catalogue` CTL rules, Shelf's From catalogue, Onboard ☁ / ⇄ sync, the site's Catalogue page;
all hand-tested; catalogue **version 3** online 2026-11-26 with 8 of Iman's generics fixed) · terms defaults confirmed
(2026-11-24) · Pocket's port read after `.env` (PKT-16, 2026-11-26; in the next release) · the site review's guide one-liners (live 2026-11-26) · the sales list and its totals 3–14× faster (RPT-38, 2026-11-26: a page 0.51 → 0.04 s, Sales (year) 0.98 → 0.39 s)
· a note on Hold (SALE-53, 2026-11-26) · Setup's launcher and print helper: the 84 mutation survivors read
(2026-11-26; SET-10 this PC's choices, SET-11 the print helper; 45 killed, the 39 left are the process entry points
— start / launch / worker / headless, the helper's endless loop — and timers) · the sign-in page's
"Last closed" pill (DRAW-20, 2026-11-26; the receipt card and Counter pill dropped: the page is public) · Onboard /
Shelf on Windows (2026-11-26: the catalogue secret on GitHub, Windows build green; the apps' fixed key =
`~/.config/karots/karots-apps.keystore`, this PC's old debug key, so phones update without a reinstall).

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

None. (The till top bar's counter name was dropped, user 2026-11-26: the suite has no counter names.)
