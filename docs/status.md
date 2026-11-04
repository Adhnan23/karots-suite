# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of every milestone. Older
handoffs, finished plans and specs live in git history (`git log --diff-filter=D --name-only -- docs`).

## Now (handoff, end of 2026-11-04)

**Where things stand:** the suite is feature-complete for a shop's day. The first shop, **Aiman Pharmacy Kalpitiya**, is not installed yet. Newer Releases are in
the bucket (see `published` in `shops.json`), published with the user's go-ahead for the update power-cut test. **Don't publish more before the user says.**

**Waiting on the user, in no hurry:**
- A label roll, then the XP-365B on the real main PC: size, gap, drift, Image with Sinhala / Tamil, scanning a label back. Nothing has printed on it yet.
- The pharmacist's trial, then the real install.
- Aiman's charges; the business registration number (don't sign shops before it); the lawyer meeting with `docs/legal/karots-suite-lawyer-review.pdf`.

**Decisions made (user, 2026-11-04):**
- **Pocket Cashier ships only after its release 2.** The tablet test waits for it. It **stays on Aiman's profile**, but the user won't mention it to him (it still shows as Phones in admin).
- **Classic receipts are gone** (no switch; `print_as` dropped), with the hidden plain text kept for a picture that fails and for slips with no template. **Classic labels stay** until the roll test passes; then remove that switch and keep the same hidden backup. The user said to do nothing more on this for now.
- **Product local names don't print on receipts** and don't need to; only the shop's own local name does.
- **No cash drawer at Aiman for now** (opened with its key), so the drawer is untested on real hardware.

**Small things for later, when the user asks:**
- **Test backups in Aiman's cloud folder:** the user deletes them in Cloudflare after testing. The bucket's 30-day lock may refuse recent ones.
- Bigger: Pocket Cashier release 2 (counter pages, plugin menus, tickets on the phone; a ticket on a phone-only till waits unseen today); the whole-system check (a simulated year, the books balance).

**Aiman's setup (replaces the shop-visit list below):** new PC (i3 4th gen, 8 GB, 120 GB SATA SSD, 20") = **admin + main PC**, in the background; the **owner's own Linux
laptop = cashier PC**, with the XP-80T (receipts) through the print helper; XP-365B (labels) on the main PC as the shop's label printer; one wired 2D scanner. Another PC only when needed. File mode, cloud backup on.
Printer queues on this dev PC: `XP80T` (the new printer), `POS80` / `XP365` (older units; `POS80` is the default and holds stale test jobs: `cancel -a POS80` before plugging an old one in).

**VMs here (`virsh -c qemu:///system`):** `karots-main` (Xubuntu, on test build 1553 or 1524, hand-test data), `karots-cashier` (its clone, hostname `cashier-laptop`), `win10` (cashier PC of `karots-main`,
tested on Aiman's folder). Only one VM can hold the XP-80T at a time. Shop `windows-test` in `shops.json` is for Windows tests (every plugin, file mode, no cloud, never Released). Cloud folders `test-linux`, `_check` and the VMs' Aiman backups age out by Dec 2; never restore a VM backup to his shop.

**Passed by hand on real hardware (user, 2026-11-04):** the print helper on the real XP-80T, Linux and Windows cashier; a Windows cashier with a Linux main PC, approved from Linux; all ten receipt styles on paper; Windows printing, also after a restart; Setup adding the printer on Windows;
box purchase (5 boxes, +500, owed 15,000); barcode flow and removing a link; long sale (1 Box + 1 Card + 3); + New product; reprint; sale with the helper stopped; the main PC not printing on the cashier's printer; recall; supplier discount; a return's credit paying a delivery; Give back → Credited us clearing a bill with the rest paying the next.
**Not tried by hand:** a sales person's flow on this build, Credited us with the new prefill.

**Built today (details in git log):** Send to cashier reviewed and fixed. Print helper + Settings → Cashier PCs' printers (`internal/features/printmachines`, `cmd/setup/print_agent.go`; a request's printers follow its socket address). Setup makes the print queues (Linux: raw CUPS; Windows: Generic / Text Only on any free USB port), on install and Update. Appearance prints a style on any printer (`features/appearance/TryPrint.tsx`).
An unknown scan asks "a product you already have?" first on purchase screens and intake; + New product; a cashier's linked barcode waits on Items to review. Unit costs keep 4 decimals (`stock_movements.cost` stays 2: it's a total). Demo Room has a second cashier and a Sales person.

**The receipt bug worth remembering:** the till is zoomed, so a slip was measured small and drawn full size: "TOTA / L", in Chromium, 8 of 10 styles. `shoot()` cancels the page zoom; `appearance-receipts.spec` covers all ten. Appearance's samples hid it because admin pages don't zoom. **Check slips on the till, not only in Appearance, and with the shop's real data.**

**Power-cut test on `karots-main` (2026-11-04, file mode):** a generator doesn't remove the cut, it comes after it, and Aiman has no UPS (he has generators). Sales: confirmed sales all survive, in-flight ones vanish whole, stock and cash agree, up in 7-9 s; receipt numbers jump about 30 after a cut (PostgreSQL, nothing lost). Found and fixed (`559cab3`, `internal/durable`): backups and a downloaded update weren't forced to disk. A cut left 0-byte backups, and on 1524 a cut 6-20 s after an update's swap left the shop not starting (3 of 3; the same update with no cut was fine). On the fixed code (1912 updating to 1928): 8 cuts 3-28 s after the swap all came up on the new version, cuts during download and backup stayed on the old one, 8 cuts just after startup left no empty backup. The VM has no shell, so "empty program" is the likely cause of the old failure, not seen. Not tested: the tiny gap between Swap's two renames (left as it is; Setup -> Update would recover it), a shop down that way, the real SSD (one plug-pull on the real PC before real data). The VM is back on its `before-powercut` snapshot (1524, hand-test data).

**Working here:**
- **Tests:** `make test` runs one package at a time. Nothing reaches a real printer now (`testing.Testing()`, `KAROTS_NO_SPOOL` for Playwright's server). Playwright resets `suite_db`; never run it with `go test` on `suite_db`, or two builds at once (they share the plugin lists). `appearance-receipts.spec` resets the receipt style when it ends. **First job next session: run the full Playwright once.** It last passed (235) before the last changes: the prefill, both Setups, the test fixes. Its final run was stopped on purpose.
- **Power cuts happen here.** After one, check before anything else: `git fsck`, `make check-core` (a cut during a build leaves the plugin lists on: `make core-only`, then `git checkout -- internal/spa/dist/.gitkeep`). One cut emptied the newest commit's objects: back up `.git`, remove the empty files, point `main` at the last good commit, recommit. Backup from that repair: `../karots-suite-git-backup-20261104-1611` (delete when happy).
- **The user tests with real sales and scans of real paper** (phone scans land in `/mnt/nvme_512GB/Downloads/`, screenshots in `~/Pictures/Screenshots/`). Don't print on the real printer yourself until a picture looks right.
- A Windows or cashier install takes the shop's whole `dist/<key>/` folder.

The list under "Before that" is finished, except where noted above.

## Before that (2026-11-02)

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

**Done 2026-11-02 (the list agreed 2026-11-01):** the fresh review of the 2026-11-01 money changes, the unreliable
`presence.spec.ts:13`, and the docs screenshots (Demo Room rebuilt). The review fixed: a discount on old debt reading as
money paid on a later bill; one loss being settled several times on a double tap; Give back booking found stock when units
outside any lot covered the count; a lot dated today reading as expired on admin pages while the till sold it normally;
bills saying "Returned" for any credit (now "Credited"). A bounced cheque on a discounted bill was already right and now has
a test. Presence: beats go one at a time (it failed 6 of 12 runs before, passes 12 of 12). Full Playwright: 227 passed,
33 skipped.

**Where the user is (end of 2026-11-02):** Aiman Pharmacy is Released (13:55 build has New supplier on intake) and being
rehearsed by hand in a Xubuntu VM: system user -> admin -> users, products and suppliers on the spot, batches, cloud backup (one
backup reached the cloud at 14:03). Fix whatever that rehearsal turns up before the real install. Old release files are deleted.
`karots-suite-backups` has a 30-day bucket lock on purpose, so `test-linux`, `_check` and the VM's Aiman backups age out by
themselves (by about Dec 2): pick by date on his Bring back list, and never restore a VM backup to his shop. Something is still
running as Test Linux and uploading (13:55 that day): stop it. Not done: an automated first-day test; `make test` still runs Go
packages in parallel; the Builder's "One PC" name misleads (file mode does serve other devices on the network).

**Next session, in this order (agreed 2026-11-02; Send to cashier is committed, not yet in a Release):**
1. The user's two-browser hand test of Send to cashier **passed 2026-11-02 ("works flawlessly")**. Still: a fresh review of that work
   (How we work, 3).
2. **Demo Room:** add a Sales person login, with a second cashier in a shop, so tickets can be shown. Its years rebuild anyway (a new
   migration changes their stamp). The Demo Room already on the user's laptop is fine for printer shopping.
3. `make docs-shots` with the new screens (Send panel, tickets, the close screen, Users).
4. Builder's "One PC" name, the guide's "Server for several PCs" and the order form's choices: file mode with cashier PCs is real.
5. Hide or clear up Repairs' **Take a deposit** for a sales person (the server refuses it; the button still shows).
6. Stop whatever still uploads as Test Linux. New Release for Aiman, then rehearse the whole set: main VM + cashier VM + the tablet.
7. **Printers per machine, and USB on a cashier PC (user asked, design it first):** today the main PC does all printing, so a USB printer
   only works on the main PC, and a cashier's own printer is set per user (Users page, `tcp://…`), not per machine. Wanted: a counter's
   printer belongs to that counter, USB or network. Not needed for Aiman if its receipt printer has a LAN port.
   User chose the recommended way: a small helper Setup puts on a cashier PC reports its USB printers to the main PC and passes jobs
   to them; the admin picks machine → printer; a job follows the machine the cashier is on.
   **Image vs Classic:** the user tests Image receipts and labels on the printers bought 2026-11-03. If they work, drop Classic.
   Decide after that test: a failed label picture falls back to Classic today, so that fallback needs an answer first.
   When the printers arrive (not before; user): build a printer test together (languages + logo, long slip, speed, drawer once, label
   drift, labels scanned back with the 2D scanner).
8. Then the list below.

**Next:**
1. **Hand testing, still not confirmed by the user:** recall, a credit paying a delivery, the discount (Give back: "worked well").
2. **A trial with the pharmacist.**
3. **Time-of-day tests:** `TestChequesInEveryState` (internal/demo/gen) and `TestShopFloorFilled` (internal/demo/gentest)
   failed at about 09:00 on 2026-11-02, also on the code before that day's work, and passed from 12:35. Not looked into
   further. **Run Go tests with `-p 1`:** in parallel the demo tests and others collide in one database and fail at random.
4. **Real printers:** the user is shopping for them with the Demo Room on a laptop (see `printer-buying-plan` in memory).
   Add the models that pass to `docs/guide/printing.md` as known-good.

**First shop: Aiman Pharmacy Kalpitiya** (`shops.json`): three machines + a tablet (below), Xubuntu, file mode, cloud backup on; plugins
aicatalog, alternatives, combos, pharmacy, pocketcashier, productplus. Combos is the user's gift to this first shop, not charged. The old profiles (Al
Raj, Mum Digital, MUM Motors) were removed on purpose: those shops stay on Karots POS.

**Shop visit, 2026-11-02: the shop wants more than the Released profile.** Admin PC in the back office (admin only, no
selling), a cashier PC at the counter, a cashier laptop, and their tablet with Pocket Cashier as a backup (this replaces "No
Pocket Cashier for him yet"). Released as 2026.11.02.1958 with `pocketcashier`, file mode kept; not installed. The user buys the hardware 2026-11-03 (two desktop sets, a laptop, one receipt printer, one label printer, two 2D scanners, no UPS). Install plan (user): the same three files on
all three; admin PC first as main, "In the background" (must stay on while the shop trades), then the laptop and the counter PC
as cashier PCs with its address. To settle: the printers (a USB printer on a cashier PC can't be reached: Setup installs CUPS on
the main PC only, so network printers, `tcp://host:9100`, set per user), and what is charged (Shop Rules: extra cashier PC 3,000 each, Pocket Cashier 5,000). Before
the real install, rehearse the whole set: a second VM as a cashier PC, plus the tablet (only phones tested so far).

**Send to cashier, built 2026-11-02 for this shop (their way of working):** one or two people hand out medicine, a cashier takes the
money. New Users choice **Sales person** (`users.sends_to_cashier`, role stays cashier): same Sell screen, no money; the server
keeps an allow-list (`internal/api/sales_person.go`) and plugin routes that move money carry `middleware.RequireHandlesMoney`.
A ticket is a held sale with `to_cashier` (on screen only, random number unique among those waiting); the chosen open till gets a
popup, every open till sees the list, one opens it, unopened after 2 minutes it moves on, closing a till deals with its tickets
first; `sales.prepared_by` records who prepared it. For any shop, not just pharmacies. **Pocket Cashier release 2 must let a cashier open
tickets; sales people stay off Pocket Cashier for good** (user: in a power cut there is no network, one cashier sells on the tablet).
A ticket keeps plugin lines, their data and a plugin's pre-filled payment (a repair deposit); `e2e/ticket.ts` runs combos, pharmacy,
documents, recharge and repairs through one. Tests at the end of the day: Playwright 234 passed, 33 skipped. No shop toggle for one-person-one-device: each person has their own login.
Hardware decided: all Xubuntu; the receipt printer and drawer at the counter PC. Tablet type to be checked at the shop; price
agreed separately.

**Idea, parked 2026-11-02 (user: no time now):** a whole-system check before shops rely on it: a test that the books balance over
a simulated year (money, supplier and customer balances, stock, profit), then break-it scenarios, then fresh reviewers by area.

**Also 2026-11-02:** Print & Copy extras (Urgent, Stapling, Hole punch) no longer draw the service's recipe: Urgent is a
priority charge, not more copying (user). Demo Room: food and medicine lots have expiry dates, and each such shop opens with
a few lots expired or close to it, from a short-dated delivery 14 days back; the mixed shop buys each category from a
supplier of its own kind.

Also done 2026-11-01: dev server moved to port 3000 (Karots POS no longer runs beside it); no Apps icon on the dock when no
plugin registers an app. **Testing tip:** run Go tests in a temporary database so hand-test data stays clean, and remember
Playwright resets `suite_db`.

**Core work for the first shop, a pharmacy: BUILT 2026-11-01** (decided with the user that day). He catalogues and bills
medicines; other goods go in as **Quick items** (one "Other goods" category; the admin clears Review now and then). He
wants to stop overstocking (smart reorder), stop missing expiries (A below) and see what he lacks (Low stock + requests
plugin). No Pocket Cashier for him yet (agreed). A trial with the pharmacist is the natural next step after the list above.

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
   4. **Supplier recall of a lot: DONE 2026-11-01.** **Recall** on a lot: keep selling (just listed on the supplier's
      panel), or stop selling (the till and phone warn as for an expired lot, Sell anyway still works; dashboard line).
      It stays in stock until **Give back**, which claims only what's left; **Undo recall** reverses it. The user:
      what was sold before is not chased, so there is no "who bought it" list; add one only if a dangerous medicine
      ever needs customers called (sales record each line's lot).
   **How it really works (the pharmacist, 2026-11-01)** — design to this, not to a strict rule:
   - The supplier gives the notice days (e.g. 60): tell them before that date. Told late, they *sometimes* refuse —
     so it's a warning and a reminder, never a block; they may change their mind and take it anyway.
   - Days passing ≠ the whole lot is claimed: some units sell meanwhile, so the claim is what's left when they take it.
   - The rep may come any time — a few days after being told, before expiry, or after it — and may still replace
     expired stock.
   - Made good as: the same product (often a **new lot at a different price** — same price or the shop pays the
     extra: Losses & Recovery restocks a new lot with expiry / price / extra), **money / credit**, nothing (written
     off). **A different product** is deliberately not a feature (user, 2026-11-01): settle as **Credited us**, enter
     what they bring as a normal delivery, and the credit pays that bill. Credited us goes on open bills oldest first;
     what's left pays their next delivery (`loss_recoveries.credit_unapplied`).
   - **Goods that arrived damaged:** already in core — return them from that delivery's lot (Purchasing → Returns).
(`customers.loyalty_points`, unused, dropped 2026-11-01.)

**Discount received when paying a supplier: DONE 2026-11-01.** A **Discount** method on the Pay form (supplier page, not the
counter): settles bills like a payment, no money moves, can't pass what's owed; its own statement line and profit line.

**Dropped 2026-11-01 (user):** prices per customer / customer group; retail and wholesale stay the only price types.

**Parked:** the **open-price plugin** (goods tracked by value, never count — typed price, supplier bills at cost,
damage as an amount; build only if Quick item is too much work for a shop); staff commission and delivery orders
(only if a shop asks). **Future plugins, not now:** loyalty points, gift vouchers / credit notes, several shops.

**Then: Pocket Cashier release 2** (spec decided in release 1): the counter pages on the phone (receipts, credit,
returns, suppliers, expenses…) and plugin menus / tabs, online first. Known open item: a cashier who moves back to
the PC while the phone still shows them is claimed back by the phone's next send.

**Unreliable e2e tests (seen 2026-11-01, not looked into):** `palettes.spec.ts:45` and `plugin-repairs.spec.ts:145` each
failed once in a full run and passed alone.

**Waiting on the user (outside the code):** the business-name registration number (sole proprietorship applied
2026-10-28 — don't sign shops before it); the lawyer meeting with `docs/legal/karots-suite-lawyer-review.pdf`, then
apply the answers and drop the DRAFT line; the real-printer test when printers arrive.

**Left on purpose (only if a shop runs into it):** a same-product **Replaced it** lot doesn't remember that supplier, so
with no preferred supplier on the product it warns at the shop's number and has no Give back (set a preferred supplier); a bounced cheque that paid old debt moves the customer's linked
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
