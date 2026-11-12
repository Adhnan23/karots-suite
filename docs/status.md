# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of every milestone. Older
handoffs, finished plans and specs live in git history (`git log -p -- docs/status.md`, `git log --diff-filter=D
--name-only -- docs`). The catalogue and the Onboard / Shelf app keep their own details in their READMEs.

## Now (2026-11-10 night)

**The suite is feature-complete for a shop's day; work follows real use at the first shop, Iman Pharmacy Kalpitiya**
(`iman-pharmacy-kalpitiya`, install id FC1A79F7, support PIN `make support-pin ID=FC1A79F7`).

**Latest Release: 2026.11.09.2233** (user asked). Full run first: Go 63 packages, web 1,126, Playwright 240 passed +
the three specs re-run alone 27/27 (`plugin-aicatalog:278` failed once in the full run, as before). In it:
- **Import by ref** (`3ad406c`): Products → Sheets ▾ → Include ref; each row updates its own product even after a
  rename or with no barcode; another shop's ref is refused; a sheet without refs imports as before.
- **Receipt lines** (every style + the plain backup): one price per line ("qty x single price = amount"), a saving on
  its own line with its minus amount on the right, then Line total; no "->" net price; no pack split ("1 Card + 2"),
  a cheaper pack shows as a "Pack price" saving; quantities without ".00" unless a fraction. Sections keeps its own
  YOU SAVED block (user).
- **Receipt text size**: Appearance → Receipt style → Text size (Normal / Large / Larger, shop-wide, migration 00033).
  The slip is laid out narrower and the picture scaled up to the paper; the barcode still scans. Classic and the plain
  backup put a too-long line's amount on the next row instead of cutting the words (58 mm).
- Not yet on paper: one Larger receipt on the XP-80T.

Earlier Releases stay in the bucket (2026.11.09.0655, 2026.11.08.1330, …). **Don't publish another before the user
says.** `dist/iman-pharmacy-kalpitiya/` holds the build; `dist/aiman-pharmacy-kalpitiya/` is old and must not be installed.

## Next

**2026-11-12 (Iman asked): Compact receipt style + the shop's thank-you line** (`7d51d2f`, migration 00034) — the
eleventh style: logo beside the shop's details, one row per product, no barcode, ~38% shorter on 80 mm; Settings →
Thank-you line replaces every style's "Thank you" ("Get well soon!"). Not yet: the user's look on the dev server, a
print on the XP-80T, `make docs-shots` (receipt styles + settings shots), the e2e run, and a Release to Iman (wait for
the user).

**Where 2026-11-10 ended (catalogue day, no suite changes):** the user is at Iman on 2026-11-11 with two printed
lists, imports an update file into Onboard, adds products (picking from the catalogue, hand-typing the rest), pulls
Iman, then brings a new Onboard backup. Everything below lives in `~/Projects/karots-suite-catalogue` (its README is
the detail; `shops/iman-matches.md` for Iman).

0. **Morning of 2026-11-11: DONE.** Unique Pharmacy (+1,555) and Beyond Health (+420) in (`build/makers/pages.py`:
   product pages from the sitemap, ones the catalogue has skipped); the promo rule no longer drops "Soap Free" / "Sugar
   Free" products; mycare's whole range downloaded (9,802 of 9,887; 85 gone from the site). Catalogue **52,357**.
   The morning's update file was replaced by the full one (item 2). Karots files in the user's Downloads live in
   `/mnt/nvme_512GB/Downloads/Karots/` (`catalogue-updates/`, `onboard-backups/`, `iman-lists/`).
1. **The 40 checklist: DONE 2026-11-11 from box photos** (catalogue `9e90d54`; photos in `~/Desktop/40list/`). Iman: 47
   confirmed, 0 waiting, NEX VITA sold out (no box; `sold_out` in the decisions). Found on the boxes: "Nvitor D" is a real
   product (Nvitor-D, ACE), **not a typo — keep it in Onboard**; Iman typed cefuroxime for Mahacef (box: cefixime), +K2 for
   Vidan D 2000 (none), 1200 IU for Dee K 2000 (2000); Becosip is tablets; a decision key may be `"name | generic"` (two
   Sithara products share each name). The 113 safety-net list (`iman-lists/iman-matches-to-glance-2026-11-10.pdf`) is
   not answered yet.
   **Afternoon: Iman's 104 new products matched** (backup `onboard-backups/karots-onboard-backup-20261111-1328.json`:
   Iman 652 products, typed on Iman's own PC; none hand-typed in Onboard). 53 confirmed, only NEX VITA missing; Baurs
   celecoxib = Unichem's via A Baur & Co (NMRA register). Catalogue 52,429 (`b9da22f`). **Update file for the user:**
   `catalogue-updates/karots-catalogue-update-2026-11-11-3.json` (from the 13:28 backup: 24 new, 15 refreshed, 0
   doubles) — **imported and checked** (backup `karots-onboard-backup-20261111-1428.json`: 52,538 products, all 39 as
   sent, 0 doubles, Concor 2.5mg deleted, Iman's 652 intact). **Catalogue work paused (user, 2026-11-11):** nothing
   more until a shop needs it; missing generics stay as they are.
   - **Iman's own categories and generics aren't reliable** (user: new staff, little English; ZyQ was under
     Antibiotics). The whole-list fix (item 3) proposes categories from what each product is and takes box / catalogue
     details over theirs; the user approves the tree.
   - Still open, not urgent: the safety-net glance (the 113 list + ~10 brand / close picks among the 104), and
     Immune Up's real contents (mycare / Topcure disagree; a box photo settles it).
   - Next new Iman products: `python3 shops/iman_matches.py <latest backup>` marks them "new since last list".
   - **2026-11-12 (user at Iman again):** backup `karots-onboard-backup-20261112-1626.json`: Iman 739 products (+87),
     **all matched**. CoQTen, Nifigen ER 20 and NEX VITA from box photos (`Karots/iman-photos-2026-11-12/`). The
     user's Epnone 50 / Solotrate SR 30 / Tel OD 80 showed the catalogue held one medicine under two spellings
     ("Tel OD 20" / "Tel Od 20mg"): **278 such pairs merged, only when the maker is the same** (user); the shop's
     spelling wins. Catalogue 52,157 (catalogue `d10fe14`). **Update file for the user:**
     `catalogue-updates/karots-catalogue-update-2026-11-12.json` (39 refreshed, Nex Vita new, 278 extra copies
     deleted; none of them the user's or in a shop) — **imported and checked** (backup `karots-onboard-backup-20261112-1643.json`: 52,266 products, all 40 as sent, 278 deleted, 0 doubles, Iman 739 and 470 shop links intact).
2. **Generic clean-up: DONE 2026-11-11, and in the user's Onboard** (backup `Downloads/Karots/onboard-backups/
   karots-onboard-backup-20261111-0910.json` checked: 52,515 products, 13,005 generics all in line, 0 doubles, the
   user's 261 products and Iman's 549 shop links intact). The morning file was built from an old backup and made 114
   doubles; a deletion-only file removed them. **Rule: an update file only from the user's latest backup.**
   Fixed the same day: `onboard_update.py` counts a product as the user's when Onboard stamped it (microseconds) or
   its uid is in `build/onboard_yours.json` (every uid ever theirs, seeded from the 10 Nov backup); Onboard `b131b5b`
   reads the deleted list once per import (not yet in an installed build).
3. **Iman whole-list fix**, when Iman has finished adding products (wait for the user): user exports Products →
   Sheets ▾ → Include ref + Include custom fields and sends the CSV → match every row (start from
   `shops/iman-matches.csv`) → user approves the category tree → import the same CSV back (each row updates its own
   product by ref; `category` takes a path) → delete old empty categories, merge true duplicates → **then stock**
   (Iman's trial month sells with "sell anyway"; a full stock-take starts them fresh — no trial / go-live tooling
   unless the user asks). Never wipe and re-add: refs keep barcodes, packs, lots, suppliers and sales.
4. Later, user's call: the install stick, the whole-system check, Pocket Cashier on a real phone / tablet.

## The family

- **Karots Suite** (this repo; pushes to GitHub `origin` and GitLab `gitlab`).
- **Karots Suite Onboard + Karots Shelf** (`~/Projects/flutter/karots_suite_onboard`, one Flutter repo, two Android
  flavours; Linux = Onboard). APKs in `~/Downloads` (`karots-onboard.apk`, `karots-shelf.apk`, built from `3022c37`;
  `--flavor onboard|shelf`). Onboard = the user's own setup app (install ID 3EE0E57C's hourly code). Shelf = shop
  staff's app, free; real-phone hand test PASSED 2026-11-09; in Iman's build since 2026.11.09.0655.
- **Karots Suite Catalogue** (`~/Projects/karots-suite-catalogue`, private, 50,065 products): Onboard gets it as an
  **update file** (`build/onboard_update.py`, from the user's latest Onboard backup: same ids, merges in place; the
  user's own edits win). The first full import was a wipe + import; never again unless the user asks.
- Karots PIN Vault is the user's own app for all their projects: never tie it to Karots Suite.

## Not yet tried for real

- **Pocket Cashier release 2** (on `main` since 2026-11-08): the user tried it on 2026-11-11 (dev server, the Demo
  Room's every-plugin year restored, a 10-step test list) — "looks fine"; they'll report anything they find. Known small gaps left on purpose: an offline repair keeps "New-xxxx" in its sale line; offline
  on-account sales don't lower the balance on the phone (can raise a false stale flag); a return's cash isn't
  compared with the PC's; after a cut an offline sign-in has no session until signing in again (printing and tickets
  fail quietly); a person who must change their PIN gets calls refused while their sales stand; a refused sale's note
  has no items or total. **User's call, open:** Pharmacy cuts the patient's name at 360 px and Print & Copy squeezes
  its total (a plugin layout change the Pocket rule forbids without the user's say); a reload past the float isn't
  flagged under What came in; a sale made during a cut can't be returned on the phone until the PC has it.
- A sales person's flow on this build; Credited us with the new prefill.
- The XP-365B label printer (nothing printed on it yet): its roll test decides whether Classic labels stay.
- A cash drawer (Iman has none for now).
- Recovering a shop already down from an update cut, through Setup → Update (needs the user at the VM).

## Waiting on the user, in no hurry

- Anything from Iman's use. Iman's charges; the business registration number (don't sign shops before it); the
  lawyer meeting with `docs/legal/karots-suite-lawyer-review.pdf`.
- Test backups in the old `aiman-pharmacy-kalpitiya` cloud folder age out by about Dec 4 (30-day lock; the user
  deletes them in Cloudflare, not me). Keep `karots-main` / `karots-cashier` VMs off or expect uploads there.

## Decisions and notes to keep

**Iman's setup:** new PC (i3 4th gen, 8 GB, 120 GB SSD, 20" 1600x900) = admin + main PC, file mode, cloud backup on;
the owner's Linux laptop = cashier PC with the XP-80T (receipts) through the print helper; XP-365B (labels) on the main
PC; one wired 2D scanner. Plugins: aicatalog, alternatives, combos (the user's gift, not charged), pharmacy,
pocketcashier (the user won't mention it to him), productplus. Add printer models that pass to
`docs/guide/printing.md` as known-good. Al Raj, Mum Digital and MUM Motors stay on Karots POS.

**Receipts:** pictures in the shop's style (Classic receipts gone; the hidden plain text stays for a failed picture
and slips with no template). Product local names don't print; only the shop's own local name does. **Check slips on
the till, not only in Appearance, and with real data** (the till is zoomed: a slip once measured small and drew full
size, "TOTA / L"; `shoot()` cancels the zoom). Classic labels stay until the roll test passes.

**Send to cashier:** a Users choice **Sales person** (`users.sends_to_cashier`): same Sell screen, no money; the
server's allow-list is `internal/api/sales_person.go`, money routes carry `middleware.RequireHandlesMoney`. A ticket is
a held sale with `to_cashier` (random number unique among those waiting); unopened after 2 minutes it moves on;
closing a till deals with its tickets first; `sales.prepared_by` records who prepared it. Sales people stay off Pocket
Cashier for good.

**How expiry really works (the pharmacist, 2026-11-01):** the supplier's notice days are a warning and a reminder,
never a block; the claim is what's left when the rep takes it; made good as the same product (Losses & Recovery
restocks it), money or credit, or nothing. A different product is deliberately not a feature: settle as Credited
us, enter what they bring as a delivery, and the credit pays that bill (oldest open bills first; the rest pays the
next, `loss_recoveries.credit_unapplied`). Damaged on arrival = a supplier return from that delivery's lot. No "who
bought it" list for recalls unless a dangerous medicine needs customers called (sales record each line's lot).

**Dropped (user):** prices per customer / customer group; promo videos. **Parked:** the open-price plugin; staff
commission; delivery orders; advance orders. **Future plugins, not now:** loyalty points, gift vouchers / credit
notes, several shops.

**Left on purpose (only if a shop runs into it):** a same-product Replaced it lot doesn't remember that supplier (set a
preferred supplier); a bounced cheque that paid old debt moves the linked balance, not the old-debt part; a partial
supplier-cheque bounce reopens bills before the payment's advance part.

**Power cuts (Iman has generators, no UPS):** tested on `karots-main` 2026-11-04 (file mode): confirmed sales survive,
in-flight ones vanish whole, up in 7-9 s; receipt numbers jump ~30 after a cut. Backups and downloaded updates are
forced to disk (`internal/durable`). Not tested: the gap between Swap's two renames, the real SSD.

## Working here

- **Tests:** `make test` (Go, one package at a time); `make test-web`; `make e2e-seed && cd web && bunx playwright
  test` with ports 3000 / 5173 free and nothing else on `suite_db`. Nothing reaches a real printer. Run long tests
  detached with logs in `~/.cache/karots-logs/`. Before a Release: the full run, then `make check-core`.
- **Unreliable e2e tests** (each failed once in a full run and passed alone): `palettes.spec.ts:45`,
  `plugin-repairs.spec.ts:145`, `plugin-aicatalog.spec.ts:278`. `TestTodayTraded` (Demo Room) once failed just after 09:00.
- **Type-check the build the way it runs:** `cd web && bunx tsc -b` (a plain `tsc --noEmit -p .` misses test files
  and unused imports, and the Playwright build then fails).
- **Power cuts happen here.** After one: `git fsck`, `make check-core` (a cut during a build leaves the plugin lists on:
  `make core-only`, then `git checkout -- internal/spa/dist/.gitkeep`).
- **Dev server:** `make plugins P=all && make migrate && make dev`; dev logins Admin 0771234567 / 1234, Cashier
  0771111111 / 1111, system user 0000000001 / 2273. `make core-only` before committing.
- **VMs** (`virsh -c qemu:///system`): `karots-main` (Xubuntu, snapshot `before-powercut`), `karots-cashier`, `win10`.
  Only one VM can hold the XP-80T. Shop `windows-test` in `shops.json` is for Windows tests (never Released).
  Printer queues here: `XP80T`, `POS80` (default; `cancel -a POS80` before plugging an old one in), `XP365`.
- **The user tests with real sales and scans of real paper** (phone scans in `/mnt/nvme_512GB/Downloads/`,
  screenshots in `~/Pictures/Screenshots/`). Don't print on the real printer until a picture looks right.

## Done

M0–M6 core (till, counter tabs, stock, purchasing, money, reports, people) · M7 plugins (clearance, requests,
productplus, combos, alternatives, documents, barcodes + merge, lots, AI Assistant, loose versions, packs, pharmacy,
repairs, recharge) · M8a file mode · M8b Shop Builder + karots-setup · M8c self-update + Release · M8c2 backup copies
+ cloud · live users · till left open · smart reorder levels · print as image + receipt styles · palettes (22
built-ins + custom) · labels as pictures + label styles · M8d Demo Room · docs + screenshots + legal pack · cheques ·
M8e Pocket Cashier release 1 and 2 · pharmacy expiry work · Send to cashier · print helper + cashier PCs' printers ·
power-cut fix · AI Assistant jobs + hints + Alternatives Auto levels · default unit · Karots Shelf · import by ref ·
receipt line rules + text size.

## How we work

1. **Plan, then execute.** A milestone gets a spec (brainstormed with the user) and one plan in `docs/plans/`, tasks
   with files, interfaces, test-first steps and a commit per step. Delete the plan and the spec when it's done.
2. **UI gates:** take screenshots in the palette × mode combinations, review them yourself, carry on without waiting
   unless something needs a product decision. Shots of a whole screen go through `make docs-shots`.
3. **Review** the whole milestone at the end (a fresh reviewer), fix what's found test-first.
4. **No Claude or AI attribution** in any commit or PR, ever.
5. **Work on `main`, local**; the user pushes.

What the user rejects: plain or default-looking UI (stock shadcn cards, "coming later" placeholders) — every screen
must look like the approved mockups.

## Design references

- `docs/design/theme.md` is the source of truth for tokens, palettes, login, the dock and the till layout; its
  mockups are in `docs/design/mockups/`.
- **Shell, login and palettes mockup:** https://claude.ai/artifact/MANYgogqsxC4evmSADNkia
- **Till and dashboard mockup (look only):** https://claude.ai/artifact/GkJjnypEes4GmHiHMDzFT5

## Open items

- **Login hero:** the tilted receipt card (yesterday's close) and the "Counter 1 / last closed" pills wait for an API
  that gives that data.
- **Till top bar:** the counter name on the drawer pill has no data source yet.
