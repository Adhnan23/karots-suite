# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of every milestone and keep it
short: history lives in git (`git log -p -- docs/status.md` has every older handoff, finished plan and area-by-area
battle report). The catalogue and the Onboard / Shelf apps keep their own details in their READMEs.

## Now (2026-11-22)

**The suite is feature-complete for a shop's day and battle-tested end to end.** First shop: Iman Pharmacy
Kalpitiya (`iman-pharmacy-kalpitiya`, install id FC1A79F7, support PIN `make support-pin ID=FC1A79F7`), still
onboarding — products and one test sale, **no real sales yet**.

- **Battle tests: all done.** The 7 core areas (sales / returns, drawer and money, customers, stock, purchasing,
  recovery, reports), every plugin (groups 1–3, then Pocket Cashier) and installing / running a shop (backups, copy
  folder and cloud, self-update, Setup, Shop Builder). Rule book `docs/rules/`, auditor `internal/ledgercheck`,
  battle tests `internal/battle` + `plugins/<key>/battle`, proofs `proofs/`, mutation replays. Coverage map:
  https://claude.ai/artifact/GSvAu11weLXS7PjRD4G8u9 (Go with every plugin 82.9%, core 80.6%).
- **Released to Iman as 2026.11.22.1407** (commit `03edc4df`, notes written by hand, no Pocket Cashier mention).
  Iman updates before the first real sale (Setup → Data → Updates → Update); it backs up first and changes none of
  their products. Labels stay Classic (no label printer at hand); receipts printed for real on the XP-80T.
- **Rule first (AGENTS.md, 2026-11-22):** every change in behaviour starts in the rule book — rule, failing test,
  code. Only look-and-feel skips it.

**Last full run (2026-11-22):** Go, Vitest and check-core green; Playwright 252 passed, **1 open**: till "open the
till with a counted float" couldn't close a till left open because "untracked cash" was off ("banking to a locker
is required"). The reports test turned it off via the screenshots' data and now puts it back (`0e18a587`); not yet
proven the only cause — the test prints the server's refusal, so the next full run says. Playwright failures are
read from their trace, never waved off as flakes (the old "flakes" were all test bugs).

## Next — the user's call

1. Whatever comes from Iman's use once they sell.
2. **Iman whole-list fix**, when Iman has finished adding products (wait for the user): user exports Products →
   Sheets ▾ → Include ref + Include custom fields → match every row (catalogue repo `shops/iman-matches.csv`) → user
   approves the category tree → import the same CSV back (each row updates its product by ref) → delete empty
   categories, merge true duplicates → **then stock** (a full stock-take; no go-live tooling unless asked). Never
   wipe and re-add: refs keep barcodes, packs, lots, suppliers and sales. Iman's own categories and generics aren't
   reliable (new staff): propose categories from what each product is.
3. Ideas, built only when the user says:
   - **Alternatives → Group by fields** (no AI): pick product fields (Generic name + Strength); same values = one
     group, shown to untick wrong ones, saved as ordinary groups. Fields come through core's `ProductAttributeSet`
     hook, so Alternatives never names Product Plus.
   - **A supplier by field**: Product Plus rules like "Brand = Nestlé → supplier X" for products with no preferred
     supplier of their own (reorder, orders, a GRN's suggestion); own supplier wins; two rules → both, with a warning;
     set from the supplier's side too; a brand changing supplier = move that brand's products (`AdminSupplierReassign`
     narrowed). Needs a generic core seam (a fallback supplier) so core never names Product Plus.
   - A note box on the till's plain Hold (only Send to cashier has one).
   - Read the 84 unread mutation survivors in Setup's launcher and print helper (`cmd/setup/main.go`,
     `print_agent.go`) when a cashier PC's printer is next touched.
4. Later: the install stick, the whole-system check, Pocket Cashier on a phone / tablet at a shop (release 2 worked
   on a real phone against the dev PC, 2026-11-22).

## The family

- **Karots Suite** (this repo; pushes to GitHub `origin` and GitLab `gitlab`).
- **Karots Suite Onboard + Karots Shelf** (`~/Projects/flutter/karots_suite_onboard`, one Flutter repo, both apps on
  Android, Linux and Windows). `make` builds both APKs + both Linux apps into its `dist/`; Windows = GitHub Actions on
  push. Onboard = the user's own setup app (install ID 3EE0E57C); Shelf = shop staff's app, free, in Iman's build.
- **Karots Suite Catalogue** (`~/Projects/karots-suite-catalogue`, private): Onboard gets it as an **update file**
  built from the user's **latest** Onboard backup (never wipe and re-import). Catalogue work paused until a shop
  needs it. Karots files in the user's Downloads: `/mnt/nvme_512GB/Downloads/Karots/`.
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
Before one: `make test-all`.

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
layouts · **battle tests: 7 core areas, every plugin, installing and running a shop (2026-11-18 → 22)**.

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
