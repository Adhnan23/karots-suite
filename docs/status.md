# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of every milestone. Older
handoffs, finished plans and specs live in git history (`git log --diff-filter=D --name-only -- docs`).

## Now (handoff, 2026-11-09 night)

**Iman Released 2026.11.09.2233** (user asked; full run first: Go 63 packages, web 1,126, Playwright 240 + the
three specs re-run alone 27/27; `plugin-aicatalog:278` failed once in the full run as before). In it: **import by ref**
(`3ad406c`); receipts: no pack split, a pack line reads qty x single price with a "Pack price" saving line, whole
quantities without ".00", a discounted line shows one price (no "->"), savings on the amount side in every style
(Sections keeps its YOU SAVED block, user); **receipt text size** Normal / Large / Larger (Appearance → Receipt
style → Text size, shop-wide, migration 00033; the slip is laid out narrower and scaled up; barcode still scans);
Classic and the plain backup put a too-long line's amount on the next row instead of cutting it (58 mm).
Not yet printed on paper: one Larger receipt on the XP-80T.

**Next session:** the user goes to Iman tomorrow (2026-11-10): Setup → Data → Updates → Update on the main PC, and
helps add more items. When Iman finishes adding products: the whole-list fix (item 3 under "Earlier today", by ref),
then stock. Onboard + Shelf APKs rebuilt from the app repo's `3022c37` (markup + selling work out the cost; drafts
survive a closed app; the deferred minors). Catalogue: stationery kind for school / office goods (176 moved; no
re-import needed, user); Lion Gate dropped, Natraj = Nataraj (8 in); products made by hand in Onboard reach the
catalogue from a backup the user sends (no app feature, user).

## Earlier (handoff, 2026-11-09 evening)

**Done today:** the **Karots Suite Catalogue** (`~/Projects/karots-suite-catalogue`): 34,154 Sri Lankan pharmacy /
grocery / stationery products, imported by the user into Onboard (Wipe → Products only → Import; every build gets
fresh ids because a Wipe keeps the wiped ids as deleted). Onboard `5d3f852`: catalogue search reads every field and
forgives typos (APK rebuilt). The user's own catalogue mistakes are fixed inside the file.

**Next session:** (1) the user Pulls Iman in Onboard so its products match the new catalogue; (2) Iman is still adding
products: when they finish, Release import-by-ref, then the whole-list fix (item 3 below) and only then stock;
(3) missing brands get added to the catalogue as the user names them (Lion Gate was a mistake, dropped; Natraj is
spelled **Nataraj** in the sources: 8 pencils / erasers / geometry boxes already in, no Sri Lankan shop lists more);
hardware / electronics are a later catalogue pass of their own.

## Earlier today (2026-11-09 morning; user went to Iman to help them add products)

**The Karots Suite family (2026-11-09):** Karots Suite (this repo) · Karots Suite Onboard + Karots Shelf (one
Flutter repo, `~/Projects/flutter/karots_suite_onboard`) · **Karots Suite Catalogue** (`~/Projects/karots-suite-catalogue`,
git, local: 34,458 Sri Lankan pharmacy / grocery / stationery products, the builders, its README has the rules and
every source). Onboard imports the catalogue; the user wipes Onboard's catalogue and imports
`catalogue/Sri Lanka catalogue (import in Onboard).json` fresh, then syncs the shops. Missing brands are added as the
user names them (README → Adding a source). Karots PIN Vault is the user's own app for **all** their projects (Demo
Room, Suite shops' install IDs, Onboard…): never tie it to Karots Suite. The Downloads `Karots catalogue/` folder now
holds only the user's pre-wipe Onboard backup and the Iman checklist; paths to it further down are historical.

**Next session, in order:**
1. **Grocery catalogues** (user, 2026-11-09): look at Cargills Food City, Keells and Lanka Sathosa online product lists
   the same way as mycare.lk (names, generic/brand, company; **never barcodes, prices or packs**), build an Onboard
   backup file (products only, no supplier, `updated_at` 2020 so a shop's own products always win on a name match).
   The first catalogue's builder scripts (`names.py`, `build_catalogue.py`, `nmra_parse.py`, `combine.py`) were lost
   with the scratchpad; today's are kept in `Karots catalogue/Scripts/`. The Sri Lanka pharmacy catalogue (mycare.lk
   11 vendor lists + NMRA "List of Registered Drugs By brand-generic" xls 02.11.2026, plus 2026-11-09's additions) is
   one file in `/mnt/nvme_512GB/Downloads/Karots catalogue/` with `Backups/` and the Iman checklist PDF (32 items).
2. **Release import-by-ref to Iman** before the whole-list fix (`3ad406c`: Products → Sheets ▾ → Include ref; a ref
   updates its own product even after a rename / no barcode; another shop's ref is refused). Not Released yet.
3. **Iman whole-list fix** (agreed 2026-11-09; Iman is still adding products, wait for the user): Release import-by-ref →
   user exports Products → Sheets ▾ → Include ref + Include custom fields and sends the CSV → match every row to the
   catalogue, fix name / generic / company / unit / category, list rows to clarify with Iman → user approves the
   category tree → import the same CSV back (each row updates its own product by ref; the `category` column takes a
   path and builds the tree) → delete old empty categories, merge true duplicates → **then stock** (held until the
   list is clean). **Not** wipe and re-add: refs keep barcodes, packs, lots, suppliers and any sales.
4. User's Onboard plan: install the 09:54 APK over the old one → Wipe all (saves a backup first) → add the support
   secret + Iman → import the Sri Lanka catalogue → pull Iman (their products land in Review).

**Done 2026-11-09 afternoon:** the user's catalogue edits checked (backup 13:42): Obimin and Epo 1000mg had been typed
over (restored as their own products), the two "Gamma Linolenic Acid" entries are the catalogue's Alaris Gla 100 / 120
(merged), Natocare = DHA + folic acid (Alaris), Pregmax + Maxeval = MCW Healthcare (India, same barcode prefix), Enova =
inositols + methylfolate + D3. Supplements aren't on the NMRA list (registered as food), so 334 missing OTC products
came from Lanka Chemist's whole shop (605; name, brand, generic, pack; no maker on the site). One file:
`Karots catalogue/1 Import in Onboard (in this order)/1 Fixes + Lanka Chemist.json` (CSVs in `2 To read (CSV)/`);
scripts in `Karots catalogue/Scripts/`. Health Guard's site was down (try it next). Onboard `5d3f852`: catalogue search reads
every field (generic, company, category, supplier, barcodes), forgives typos, exact and name matches first (~20 ms on
7.6k). **Iman's own products aren't touched** (user): the matching sweep waits until they've added everything.
Then sources 1–5 (medicine focus; user skipped Cargills / Keells): `1 Import in Onboard (in this order)/2 More
medicines.json`, 273 new: NMRA borderline list 9 (the devices list is hospital kit, skipped), Siddhalepa's shop 96
(Ayurvedic medicines only), Lakpura's Link Natural + Beam 20, Jeewaka Pharmacy 144 (supplements, nutrition, dressings),
Astron 4 (the rest were already in via NMRA). Dead ends: the Department of Ayurveda publishes no product list (its site
is hacked, spam links: don't download from it); SLADC sits behind a bot wall; Health Guard (503) and carelink.lk (502)
were down: retry them; pharmacylanka.com left out (sells alprazolam / Ritalin online: not a source to trust); GS Health,
Sunshine and Hemas publish no product lists. Scripts: `Karots catalogue/Scripts/more/`.
Health Guard (still down) came from the Wayback Machine instead: 212 archived listing pages (health, pharma, devices,
mother & baby) → 1,242 names → 246 new after dedupe (841 already in, 132 not medicine):
`1 Import in Onboard (in this order)/3 Health Guard.json`. Names only (no generic / company on the
listings); some are years old. Scripts: `Karots catalogue/Scripts/healthguard/`. **All merged into one list (8,493):** `Karots catalogue/Sri Lanka catalogue
2026-11-09 (import in Onboard).json` + its `(to read).csv` (Source column); the separate files above are gone. Also
in the folder: `Backups/`, `Scripts/`, the Iman checklist. Onboard APK rebuilt from `5d3f852`.
**Later 2026-11-09, the whole pharmacy range (user: take every real product, sizes / flavours are different products):**
rebuilt every source with a variant-safe duplicate rule (only form words like tablets / capsules / original / box are
ignored; any size or flavour word makes it a different product; spacing, hyphens and "50+" = "50 Plus" are the same):
**10,494 products, 0 duplicates** (mycare.lk + NMRA 7,610, Health Guard 1,192 incl. its beauty / hair / personal-care /
baby / pet / supports sections from the Wayback Machine, Jeewaka 961 whole shop, Lanka Chemist 447, Siddhalepa 242,
Link + Beam 20, NMRA borderline 9, Astron 4, fixes 9). 22 duplicate pairs inside the first catalogue removed and 5
earlier additions that were duplicates: they go out as deletions in the same file, so phones that imported them
lose them too; earlier additions keep their ids. Non-medicine products carry their type in Generic Name (Sunscreen,
Knee Support, Pet Care…). The rebuild reads `Backups/Sri Lanka catalogue before variant rebuild.*` (keep it);
`Scripts/rebuild/build_all.py` + `dedupe.py` (self-check: `python3 dedupe.py`) and the downloaded pages are there
(the scripts' relative paths point at the old scratchpad layout). **Grocery collected (same evening):** the user will **wipe their Onboard catalogue and import one clean file at the end**
(then sync with the shop), so the export now has no deletions and keeps no ids. Rules (user): every real product; no
categories (each shop gets its own tree later, AI-made for that shop's size: idea); Generic Name only on medicines /
supplements; kinds = Onboard's shop kinds (pharmacy, grocery, stationery, textile…; hardware / electronics in a later
pass of its own). Arpico (myarpico.com OpenCart, all 5,081 by category → 4,970 new) and Keells (sitemap's 10,383
till-style names, abbreviations expanded by `keells_build.py`, ~1,000 still-cut-short names left out → 8,503 new):
**23,958 products** in the one file. Kapruka (kapruka.com product pages, 2,905 shelf products) is fetching slowly
(it answers 429 to more than one worker). Scripts: `Karots catalogue/Scripts/grocery/`
(`export.py` builds the file; the pharmacy range is kept in `Backups/Pharmacy range 2026-11-09 (10,494).*`).
**Later the same evening:** + Cargills (its public search box, ~420 brand / snack / stationery words: 2,588 new; its
token API was off-limits), Atlas Axillia product catalogue 2024 PDF (264), Atlas shop pages and Kapruka (shelf groups
+ children/stationery) **still downloading** → **27,499 products** so far. Everything now lives on the data disk:
`Karots catalogue/Scripts/work/`: `./run.sh` starts / resumes the slow downloads (each skips what it saved),
`./finish.sh` turns whatever is downloaded into the one catalogue file (safe any time). Stop a download with
`pkill -f kap.py` / `pkill -f atlas.py` in a command of its own. Still missing: Faber-Castell.
**Next: grocery** (user's list: ice cream —
Elephant House, Cargills Magic, Kotmale; soft drinks — EGB, Kist, Suncrush, Coca-Cola, Pepsi, 7Up; dairy — Kotmale,
Highland, Pelwatte, Anchor; instant noodles — Maggi, Prima, Kist; biscuits — Munchee, Maliban…), from an
online grocery shop's whole list, kind `grocery`, same rules.

**Done 2026-11-09 in Onboard (app repo, all committed):** cost code (Onboard only; letters → digits in the box, letters
shown under it; syncs with the shop), optional "÷ how many" and Markup × (both apps; markup leads only when typed);
costs round to 2 places; lock = install-ID code, then the phone's unlock for 24 hours (fresh install / cleared storage
needs the code; Android cloud backup off); Wipe (products / suppliers / review / shops / secrets, or all) after the
code and a saved backup; Backup popup scrolls; big catalogues (batched detail loads past SQLite's 32,766 values,
product indexes DB v7, one-transaction import: 7.6k import 8 s → 0.9 s, 200k open 21 s → 3.3 s). Millions would need
paged SQL search (not built; only if a real catalogue gets near it). Final review findings fixed; the deferred minors were fixed
2026-11-09 evening (app `a757337`, `3022c37`): ÷ refuses negative / Infinity, commas and spaces ignored, any script's code
letters, a broken synced cost code reads as none, drafts kept in the database; and (user's find) markup + selling price
with no cost work the cost out, either order.


**Done 2026-11-09:** full Playwright run with free ports: 251 passed, 33 skipped, 1 known-unreliable failure
(`plugin-aicatalog.spec.ts:278`; the whole file passed alone, 15/15). **Iman Released 2026.11.09.0655** (default unit,
Karots Shelf access, Onboard's calls); `shops.json` committed. The shop takes it from Setup → Data → Updates.

**Still to do:**
4. **Karots Shelf hand test PASSED** (user, 2026-11-09: "works perfectly" on a real phone). The steps, kept for a re-test: `make plugins P=all && make dev`; on Users make a cashier and tick **Can use Karots Shelf**; install
   `~/Downloads/karots-shelf.apk` on the phone (USB + `adb install`, or copy the file); phone on the same Wi-Fi, shop
   address = this PC's LAN IP (`ip -4 addr`), port 3000. Try: sign in, Pull, New product with stock + expiry, Quick
   add by scan, edit a product, go out of Wi-Fi range and save (goes to Waiting to push, sent on return), lock and
   unlock offline, untick the cashier on the PC (phone shows "Ask the owner…"), a second person on the same phone.
   Afterwards: kill `air` by pid, `make core-only`, free the ports before any Playwright run.
5. APKs stay in `~/Downloads` (user): `karots-shelf.apk` and `karots-onboard.apk`, both built from the app repo's
   `3022c37` (2026-11-09 evening; Android builds need `--flavor onboard|shelf`).

**Done 2026-11-08 (all committed on `main`, nothing pushed):**
- **Onboard fixes** (app repo): a scan counts only after 3 matching frames + EAN/UPC check digit; phone unlock works
  (FragmentActivity) and falls back to the phone's PIN/pattern; support secrets travel as a passphrase-sealed copy
  (AES-256-GCM); an unsaved New product is kept as a draft; leaving an edited product asks first.
- **Default unit** (Suite `9b5372e`): see below.
- **Karots Suite Shelf** built and reviewed: see below. Go 63 packages, web 1,114, app 108 + live round trip passed.


**Where things stand:** the suite is feature-complete for a shop's day. The first shop was misnamed: it is **Iman Pharmacy Kalpitiya**, key `iman-pharmacy-kalpitiya` (renamed 2026-11-04). **The user is handling its install from here (2026-11-05: main PC installed and packed): not on our list unless the user brings it back.**
Latest Release: **2026.11.09.2233** (see Now); before it 2026.11.09.0655; earlier **2026.11.08.1330** (batch expiry fix for the Onboard app); before it **2026.11.08.1219** (searchable supplier picker; the Onboard app's packs call and export ids); before it 2026.11.08.0750 (Pocket Cashier release 2 + Settings → Identify custom fields) (AI Assistant jobs, hints, Alternatives levels; 2026.11.04.2021 is also kept in the bucket). **Don't publish another before the user says.** `dist/iman-pharmacy-kalpitiya/` holds that build; `dist/aiman-pharmacy-kalpitiya/` is old and must not be installed (old cloud folder).

**Built, hand-tested and Released 2026-11-05:** AI Assistant jobs, searchable hints, Alternatives levels (details in `docs/guide/plugins/aicatalog.md`, `alternatives.md`).
- **Hints:** the chatbot's specs and explanation are searchable at the till, shown on the tile (`AI hint: …`) and ⓘ, editable on the product page. A replaced name stays searchable.
- **Jobs tab:** Identify a whole list, Setup details, Alternative group maker, Fill in details, Prescription and controlled marks, Usual dose; Combo suggestions from the Combos page. Framework `plugins/aicatalog/jobs*.go`; record `aicatalog_job_items`.
- **Generic seams:** hooks `ProductTillLineProvider`, `ProductGrouper`, `ProductAttributeSet`; `ReorderNote.Exact/Block/BlockKey/BlockLine`; slot `pageActions`. Other plugins don't know AI exists; AI Assistant never imports them.
- **Alternatives:** Auto levels (members' levels added up; `alt_tiers.level_auto`), order split by 90-day sales or manual shares. Low groups always pull in every brand, so a row's share is never a short order. The till's Low stock tab hides covered by default.
- **Core:** packs and extra barcodes while creating a product; the Pay panel fits at 1280; dashboard low-stock count follows coverage.
- **Not decided:** saving a chatbot's dose reasons to the hint (left out for now). Real replies are pinned as tests; Rx marks and doses are unverified against Sri Lankan rules, so the warnings stay.
- **Ideas for after Iman:** one stick that installs Xubuntu + Karots (autoinstall through Ventoy, or a golden image); take notes on Iman's install first.
- **Dev DB note:** `0002_auto_levels.sql` was edited before release (`level_auto` default false, so a restored older backup keeps typed levels). An old dev DB needs `make db-down` and re-migrate, or `ALTER COLUMN level_auto SET DEFAULT false`.
- **Tests 2026-11-05, final:** Playwright 248 passed, 33 skipped; web 939; `make test` passed. The user hand-tested every job on real Gemini and ChatGPT replies; bugs found were fixed. Partial builds compile, but no one has clicked through a running one. `TestTodayTraded` (Demo Room) failed once just after 09:00 and passed on two later runs: looks time-of-day dependent, not proven.

**Built and Released 2026-11-08 (2026.11.08.0750):** AI Assistant **Settings → Identify** ticks the Product Plus fields Identify also fills (a pharmacy's generic name): the product form's and intake's ✨ Identify (full and short prompt; Use this fills More details, replacing what's there) and Identify a whole list. Core seam: `AssistFill.sections` → `ProductFormSection` `suggested`. The user tested it on the dev server: works. Product sheets already carry Product Plus fields (Sheets ▾ → Include custom fields).

**Left 2026-11-08 (session end):** the Suite's searchable supplier picker (`85543a5`): full run 2026-11-08 passed (Go 63 pkgs, web 1,114, Playwright 251 + the AI spec's 15 re-run alone after one unexplained failure); the working tree has every plugin on for the dev server (`make core-only` before committing). Onboard app since: category tree picker, searchable supplier field, Suppliers tab + shop suppliers, named support secrets, light/dark switch, pushed products show in the shop list; 58 tests. No APK built yet.

**Onboard app since (2026-11-08 afternoon):** opens with its own install ID **3EE0E57C**'s hourly code (`make support-pin ID=3EE0E57C`; only that ID's seed is in the app), secrets added after under the key icon; team sync (team key, same Wi-Fi, every 2 min), backup file, one-off device sync, auto-pull from a connected shop, deletions travel; a product's batches (see, set count / expiry / price, add; Suite `POST /admin/stock/batches/:id/expiry` `4ad924f`, not yet Released). Then (user's direction): products are the point; suppliers are a list of who supplies what (one usual supplier per product, no per-supplier prices; the catalogue keeps the selling price, cost is typed at the shop); supplier lists in a standard CSV; Quick add (scan, cost/price/qty/expiry) brings supplier and unit; units made in shops when missing; Make a copy; barcode list; disable in shop; count sheet; keyring fallback. 85 app tests + live round trip pass; APK in ~/Downloads/karots-onboard.apk.

**Karots Suite Shelf BUILT 2026-11-08 (shop staff's app, free):** Android, a second flavour of the Onboard repo
(`--flavor shelf -t lib/main_shelf.dart`, id `lk.karots.karots_suite_shelf`, home screen "Karots Shelf"); APK in
`~/Downloads/karots-shelf.apk`, installed by the user's team only. One shop, signed in with the person's own phone + PIN;
admin / manager always, cashier / sales person with **Users → Can use Karots Shelf** (`users.can_use_shelf`, migration 00032).
The tick is the whole trust (ignores other rights). A Shelf session carries `app=shelf` in its token: it never takes the
person's one device, and `shelfGate` holds it to the route list in `internal/api/shelf.go` (plugins add theirs with
`reg.AllowShelf`); untick → `SHELF_NO_ACCESS` at the next call. Offline: PIN checked against a salted hash on the phone
(5 wrong → online sign-in only), a queue per person, a save that fails for the network goes in the queue. No catalogue,
no Alternatives, no supplier balances, suppliers made from it never owe money; Stock-take off → staff are told to ask the
owner. Guide: `docs/guide/shelf.md`. Tests: Go 63 packages, web 1,114, Playwright users/auth 10, app 108 + a live round
trip (`test/shelf_live_test.dart`). A fresh review found 4 important problems, all fixed test-first. Real phone: the
user's hand test passed 2026-11-09; Iman has it since Release 2026.11.09.0655. Onboard's own `live_test` count
assertion fails on today's dev DB (also on the pre-Shelf code: dev data, reseed before relying on it).

**Default unit 2026-11-08:** Units page → Default unit (`units.is_default`, starts as Piece); new products, quick items,
services and plugin items start with it; Onboard/Shelf use the shop's default for its new products.

**Karots Suite Onboard BUILT 2026-11-08 (user's own setup app, not for shops):** Flutter, Android + Linux, repo `~/Projects/flutter/karots_suite_onboard` (its README says what it does). The user has used it; its design and plan are deleted (git history). Suite side: `GET /admin/products/packs` and `?ids=1` on the products export. Tests: 43 app tests; a live round trip against the dev Suite with every plugin passes (`LIVE_SUITE=http://localhost:3000 flutter test test/live_test.dart`). **Not yet:** the user hasn't run it; no real phone has run it; Iman's Release doesn't have the two Suite additions yet (pull needs them: the next Release). Signs in with the hourly PIN, or the Suite's public dev PIN 2273 on dev / test builds.

**Pocket Cashier release 2 is MERGED to `main` (2026-11-08; built 2026-11-06). The user merged it before a real-phone hand test ("it will work"); full suite re-run on 2026-11-08 before the merge: Go 63 packages, web 1,114, Playwright 259 passed, 33 skipped.** The phone is the backup till: the last 30 days of sales, Credit, Returns, Receipts, Warranty, Write-off, Convert, tickets (online), and the plugins' own screens (Reload & Bills, Print & Copy, Combos, Pharmacy, Repairs, Alternatives), the same on or off the PC. Details in `docs/guide/plugins/pocketcashier.md`; how it works is under Pocket Cashier in `AGENTS.md`. Tests 2026-11-06: Go all passed, web 1,114, Playwright 259 passed, 33 skipped. `pocket-plugins.spec` runs a whole day with the PC off and checks the books. A fresh review found 3 critical and 6 important problems; all fixed test-first.
**Not yet done:** no real phone or tablet has run this (timings were 4x-throttled Chromium on this PC, 30,000 sales: store 4.3 s, open 1.8 s, search 45 ms); the user hasn't hand-tested it. Sizes from Demo years: everything 3,577 sales, 330 KB gzip; about 1 KB raw per sale.
**Known small gaps, left on purpose:** a repair made offline keeps "New-xxxx" in its sale line's text (the job itself gets its real number); on-account sales made offline don't lower the balance shown on the phone, and can raise a false stale-balance flag; a return's cash isn't compared with the PC's figure; an offline sign-in has no session after a cut until signing in again, so printing and tickets fail quietly; a person who must change their PIN gets calls refused while their sales stand; a refused sale's note has no items or total.
**User's call, open:** Pharmacy cuts the patient's name short at 360 px and Print & Copy squeezes its total (fixing them means a layout-only change in the plugin, which the Pocket rule forbids without the user's say); a reload past the float isn't flagged under What came in (red float on the Reload page instead); a sale made during a cut can't be returned on the phone until the PC has it.

**Next:** the user's hand test on a real phone (still not done; release 2 is already on `main`); then the install stick or the whole-system check.

**Test backups:** Cloudflare refused deleting them (the 30-day lock), and they sit in the old `aiman-pharmacy-kalpitiya` folder. The new key means a new cloud folder, so they shouldn't reach his Bring back list; they age out by about Dec 4. **Keep `karots-main` / `karots-cashier` off or expect uploads there** (they run old builds). Keep `SUITE_RELEASE_KEY`. His install id, channel and cloud key stayed in `shops.json`. His support PIN: `make support-pin ID=FC1A79F7`.

**Screen:** the user looked at the till on a 20" 1600x900 monitor, like his: fine, roomy, no zoom. The dev DB was reset with `-seed` for that (Admin 0771234567 / 1234, Cashier 0771111111 / 1111).

**Waiting on the user, in no hurry:**
- Anything the user reports back from Iman's install. The XP-365B label test (nothing has printed on it yet) decides Classic labels.
- Iman's charges; the business registration number (don't sign shops before it); the lawyer meeting with `docs/legal/karots-suite-lawyer-review.pdf`.

**Decisions made (user, 2026-11-04):**
- **Pocket Cashier ships only after its release 2.** The tablet test waits for it. It **stays on Iman's profile**, but the user won't mention it to him (it still shows as Phones in admin).
- **Classic receipts are gone** (no switch; `print_as` dropped), with the hidden plain text kept for a picture that fails and for slips with no template. **Classic labels stay** until the roll test passes; then remove that switch and keep the same hidden backup. The user said to do nothing more on this for now.
- **Product local names don't print on receipts** and don't need to; only the shop's own local name does.
- **No cash drawer at Iman for now** (opened with its key), so the drawer is untested on real hardware.

**Small things for later, when the user asks:**
- Recovering a shop already down from an update cut through Setup -> Update: untested, needs the user at the VM.
- VM commands prompt for a password each connection until the user is in the `libvirt` group; keep one `virsh` connection open for a batch.
- **Test backups in Iman's cloud folder:** the user deletes them in Cloudflare after testing. The bucket's 30-day lock may refuse recent ones.
- Bigger: the whole-system check (a simulated year, the books balance).

**Iman's setup:** new PC (i3 4th gen, 8 GB, 120 GB SATA SSD, 20") = **admin + main PC**, in the background; the **owner's own Linux
laptop = cashier PC**, with the XP-80T (receipts) through the print helper; XP-365B (labels) on the main PC as the shop's label printer; one wired 2D scanner. Another PC only when needed. File mode, cloud backup on.
Printer queues on this dev PC: `XP80T` (the new printer), `POS80` / `XP365` (older units; `POS80` is the default and holds stale test jobs: `cancel -a POS80` before plugging an old one in).

**VMs here (`virsh -c qemu:///system`):** `karots-main` (Xubuntu, Release 1524, hand-test data, snapshot `before-powercut`; 1524 has the old update code), `karots-cashier` (its clone, hostname `cashier-laptop`), `win10` (cashier PC of `karots-main`,
tested on Iman's folder). Only one VM can hold the XP-80T at a time. Shop `windows-test` in `shops.json` is for Windows tests (every plugin, file mode, no cloud, never Released). Cloud folders `test-linux`, `_check` and the VMs' Iman backups age out by Dec 2; never restore a VM backup to his shop.

**Passed by hand on real hardware (user, 2026-11-04):** the print helper on the real XP-80T, Linux and Windows cashier; a Windows cashier with a Linux main PC, approved from Linux; all ten receipt styles on paper; Windows printing, also after a restart; Setup adding the printer on Windows;
box purchase (5 boxes, +500, owed 15,000); barcode flow and removing a link; long sale (1 Box + 1 Card + 3); + New product; reprint; sale with the helper stopped; the main PC not printing on the cashier's printer; recall; supplier discount; a return's credit paying a delivery; Give back → Credited us clearing a bill with the rest paying the next.
**Not tried by hand:** a sales person's flow on this build, Credited us with the new prefill.

**Built today (details in git log):** Send to cashier reviewed and fixed. Print helper + Settings → Cashier PCs' printers (`internal/features/printmachines`, `cmd/setup/print_agent.go`; a request's printers follow its socket address). Setup makes the print queues (Linux: raw CUPS; Windows: Generic / Text Only on any free USB port), on install and Update. Appearance prints a style on any printer (`features/appearance/TryPrint.tsx`).
An unknown scan asks "a product you already have?" first on purchase screens and intake; + New product; a cashier's linked barcode waits on Items to review. Unit costs keep 4 decimals (`stock_movements.cost` stays 2: it's a total). Demo Room has a second cashier and a Sales person.

**The receipt bug worth remembering:** the till is zoomed, so a slip was measured small and drawn full size: "TOTA / L", in Chromium, 8 of 10 styles. `shoot()` cancels the page zoom; `appearance-receipts.spec` covers all ten. Appearance's samples hid it because admin pages don't zoom. **Check slips on the till, not only in Appearance, and with the shop's real data.**

**Power-cut test on `karots-main` (2026-11-04, file mode):** a generator doesn't remove the cut, it comes after it, and Iman has no UPS (he has generators). Sales: confirmed sales all survive, in-flight ones vanish whole, stock and cash agree, up in 7-9 s; receipt numbers jump about 30 after a cut (PostgreSQL, nothing lost). Found and fixed (`559cab3`, `internal/durable`): backups and a downloaded update weren't forced to disk. A cut left 0-byte backups, and on 1524 a cut 6-20 s after an update's swap left the shop not starting (3 of 3; the same update with no cut was fine). On the fixed code (1912 updating to 1928): 8 cuts 0-28 s after the swap all came up on the new version, cuts during download and backup stayed on the old one, 8 cuts just after startup left no empty backup. The VM has no shell, so "empty program" is the likely cause of the old failure, not seen. Not tested: the tiny gap between Swap's two renames (left as it is; Setup -> Update would recover it), a shop down that way, the real SSD (one plug-pull on the real PC before real data). The VM is back on its `before-powercut` snapshot (1524, hand-test data).

**Working here:**
- **Tests:** `make test` runs one package at a time. Nothing reaches a real printer now (`testing.Testing()`, `KAROTS_NO_SPOOL` for Playwright's server). Playwright resets `suite_db`; never run it with `go test` on `suite_db`, or two builds at once (they share the plugin lists). `appearance-receipts.spec` resets the receipt style when it ends. Full Playwright 2026-11-04: 235 passed, 33 skipped, none failed; `make test` passed. Only the test for a scan before Enter and notes changed after.
- **Power cuts happen here.** After one, check before anything else: `git fsck`, `make check-core` (a cut during a build leaves the plugin lists on: `make core-only`, then `git checkout -- internal/spa/dist/.gitkeep`). One cut emptied the newest commit's objects: back up `.git`, remove the empty files, point `main` at the last good commit, recommit.
- **The user tests with real sales and scans of real paper** (phone scans land in `/mnt/nvme_512GB/Downloads/`, screenshots in `~/Pictures/Screenshots/`). Don't print on the real printer yourself until a picture looks right.
- A Windows or cashier install takes the shop's whole `dist/<key>/` folder.

## Decisions and notes to keep

**Send to cashier (for any shop, not just pharmacies):** a Users choice **Sales person** (`users.sends_to_cashier`, role stays cashier): same Sell screen, no money; the server keeps an allow-list
(`internal/api/sales_person.go`) and plugin routes that move money carry `middleware.RequireHandlesMoney`. A ticket is a held sale with `to_cashier` (on screen only, random number unique among those waiting); the chosen
open till gets a popup, every open till sees the list, one opens it, unopened after 2 minutes it moves on, closing a till deals with its tickets first; `sales.prepared_by` records who prepared it. A ticket keeps plugin lines,
their data and a plugin's pre-filled payment; `e2e/ticket.ts` runs the money plugins through one. **Pocket Cashier release 2 must let a cashier open tickets; sales people stay off Pocket Cashier for good** (user: in a power cut there is no network, one cashier sells on the tablet).

**How expiry really works (the pharmacist, 2026-11-01); design to this, not to a strict rule:**
- The supplier gives the notice days (e.g. 60). Told late, they *sometimes* refuse, so it's a warning and a reminder, never a block; they may take it anyway.
- Days passing doesn't mean the whole lot is claimed: units sell meanwhile, so the claim is what's left when they take it.
- The rep may come any time (after being told, before expiry, or after it) and may still replace expired stock.
- Made good as the same product (often a new lot at a different price: Losses & Recovery restocks it with expiry / price / extra), money or credit, or nothing (written off). **A different product is deliberately not a feature** (user): settle as Credited us, enter what they bring as a normal delivery, and the credit pays that bill; it goes on open bills oldest first, and what's left pays their next delivery (`loss_recoveries.credit_unapplied`).
- Goods that arrived damaged: a supplier return from that delivery's lot. What was sold before a recall isn't chased (user), so there is no "who bought it" list; add one only if a dangerous medicine ever needs customers called (sales record each line's lot).

**Dropped (user):** prices per customer / customer group; retail and wholesale stay the only price types.

**Parked:** the open-price plugin (goods tracked by value, never count; only if Quick item is too much work for a shop); staff commission and delivery orders (only if a shop asks). **Future plugins, not now:** loyalty points, gift vouchers / credit notes, several shops.

**Idea, parked (user: no time now):** a whole-system check before shops rely on it: a test that the books balance over a simulated year (money, supplier and customer balances, stock, profit), then break-it scenarios, then fresh reviewers by area.

**Unreliable e2e tests (each failed once in a full run and passed alone, not explained):** `palettes.spec.ts:45`, `plugin-repairs.spec.ts:145`, `plugin-aicatalog.spec.ts:278` (combo suggestion, 2026-11-08).

**Left on purpose (only if a shop runs into it):** a same-product **Replaced it** lot doesn't remember that supplier, so with no preferred supplier on the product it warns at the shop's number and has no Give back (set a preferred supplier); a bounced cheque that paid old debt moves the customer's linked balance, not the old-debt part (the total owed is right); a partial supplier-cheque bounce reopens bills before the payment's advance part (balance right, bill ageing may differ).

**First shop's profile:** Iman Pharmacy Kalpitiya (`shops.json`): Xubuntu, file mode, cloud backup on; plugins aicatalog, alternatives, combos, pharmacy, pocketcashier, productplus. Combos is the user's gift to this first shop, not charged. Al Raj, Mum Digital and MUM Motors stay on Karots POS (their profiles were removed on purpose). Add the printer models that pass to `docs/guide/printing.md` as known-good.

## Done

M0–M6 core (till, counter tabs, stock, purchasing, money, reports, people) · M7 plugins (clearance, requests,
productplus, combos, alternatives, documents, barcodes + merge, lots, AI Assistant, loose versions, packs, pharmacy,
repairs, recharge) · M8a file mode · M8b Shop Builder + karots-setup · M8c self-update + Release · M8c2 backup copies
+ cloud · live users · till left open · smart reorder levels · print as image + receipt styles · palettes (Dock theme,
22 built-ins, custom palettes) · labels as pictures + label styles · M8d Demo Room · docs + screenshots + legal pack ·
cheques · M8e Pocket Cashier release 1 and 2 · pharmacy core work (warning days, give back, recall, credits on bills, supplier discount) · Send to cashier · print helper + cashier PCs' printers · receipts always pictures · power-cut test + fix · AI Assistant jobs + hints + Alternatives Auto levels.

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
