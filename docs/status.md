# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of every milestone. Older
handoffs, finished plans and specs live in git history (`git log --diff-filter=D --name-only -- docs`).

## Now (2026-11-04)

**Done 2026-11-04 (items 1 to 5 and 7 of the list below; Playwright 235 passed, 33 skipped; screenshots retaken):** the fresh review of Send to cashier and its fixes (plugin routes that move
money or stock now refuse a sales person; a till's own close is refused on the server while tickets wait, and with no till left at all
tickets go back to the sender's Held; ticket numbers have a unique index (duplicates did reproduce); only working non-sales people get
tickets; a hold keeps a plugin's payment rows, only typed cash is dropped; a sales person on an old phone copy: sales stand, flagged, drawer
moves refused). Demo Room: a second cashier and a Sales person in every shop. "One PC" is now **Built in (file mode)** in the Builder, Setup,
guides and order form. Repairs hides its money buttons from a sales person.
**Print helper (item 7):** Setup puts it on a cashier PC (`karots-setup -print-agent <main>`, runs as a service / sign-in task). It calls out
to the main PC, so nothing listens on the cashier PC and a changing address is fine. The admin approves the machine and picks its printers in
Settings → Cashier PCs' printers. A request's printers follow its socket address (`settings.WithMachinePrinters`), ahead of a person's own
printer. Setup now also makes raw CUPS queues for USB printers on Linux, on both roles. Checked: Go tests + the real helper binary against a
real server with a stand-in `lp`. **Real printer through the helper PASSED 2026-11-04** (user: VMs `karots-main` + `karots-cashier`, the XP-80T passed through to the cashier VM, approved in Settings, Test slip printed). A sale receipt from the cashier VM also came out of the XP-80T (2026-11-04). **Image vs Classic (user, 2026-11-04, test slips on the XP-80T):** the layout matches; Image is crisper and evens out letters Classic prints too tall, so Image looks the better one. Set Aiman to Image at install; Classic stays until labels are tested. **Not yet confirmed:** a real sale in Image with Sinhala / Tamil, reprint, the drawer, helper off during a sale, a print from the main PC; signed in so far as the system user + a cashier, so also make a real admin in the rehearsal. **Still to do: the tablet; XP-365B labels wait for a label roll (the user has none yet): buy one before the install, then test size, gap, drift, Image with Sinhala / Tamil and scanning on the real main PC. Not checked on Windows.** Still open from the review: a phone till counts as an
open till for tickets, though the phone has no ticket screen until release 2.

**Aiman's hardware changed 2026-11-03/04 (replaces the shop-visit list below):** bought one PC (i3 4th gen, 8 GB, 120 GB SATA SSD, 20" monitor),
one wired 2D scanner, Xprinter XP-80T (receipts), Xprinter XP-365B (labels), both USB. The owner didn't want a second desktop or a laptop
bought: the new PC is the **admin + main PC**, **his own laptop is the cashier**, and a **tablet or phone is Pocket Cashier** backup. Another PC
only when needed. The laptop runs Linux (the Release fits). **Printers:** XP-80T + drawer on the laptop through the print helper (receipt printer of that machine), XP-365B on the main PC (the shop's label printer). **Released 2026.11.04.0843** (Send to cashier, the review fixes, the print helper); not installed, the rehearsal is next: main VM + a cashier VM + the tablet, then the real printers. Main PC's profile is now "In the background" (the owner works on it; the laptop is the till). **Test Linux:** nothing uploaded after 2026-11-02 13:55 and
all VMs here are off, so nothing is running to stop; don't boot a VM with that test install on it without removing it first. Both printers show on the dev PC in CUPS (`POS80`, `XP365`). Go tests and Playwright print to the system default printer (`POS80` here) when a test has none set, and CUPS sent the waiting jobs on
2026-11-04: leave a real printer off the default while tests run, or give the tests a way not to print.

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
