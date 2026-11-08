# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of every milestone. Older
handoffs, finished plans and specs live in git history (`git log --diff-filter=D --name-only -- docs`).

## Now (handoff, 2026-11-05)

**Where things stand:** the suite is feature-complete for a shop's day. The first shop was misnamed: it is **Iman Pharmacy Kalpitiya**, key `iman-pharmacy-kalpitiya` (renamed 2026-11-04). **The user is handling its install from here (2026-11-05: main PC installed and packed): not on our list unless the user brings it back.**
Latest Release: **2026.11.08.1330** (batch expiry fix for the Onboard app); before it **2026.11.08.1219** (searchable supplier picker; the Onboard app's packs call and export ids); before it 2026.11.08.0750 (Pocket Cashier release 2 + Settings → Identify custom fields) (AI Assistant jobs, hints, Alternatives levels; 2026.11.04.2021 is also kept in the bucket). **Don't publish another before the user says.** `dist/iman-pharmacy-kalpitiya/` holds that build; `dist/aiman-pharmacy-kalpitiya/` is old and must not be installed (old cloud folder).

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
trip (`test/shelf_live_test.dart`). A fresh review found 4 important problems, all fixed test-first. **Not yet:** no real
phone has run it; Iman's build doesn't have it (needs the next Release, user's call). Onboard's own `live_test` count
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
