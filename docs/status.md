# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of
every milestone.

## Status (2026-10-14, M7.8 repairs done)

**M7.5 Print & Copy (documents) is DONE (2026-10-12)** (plan `docs/plans/2026-10-12-m7-5-documents.md`, spec
`docs/specs/2026-10-12-m7-5-documents-design.md`). The all-plugins demo in `~/karots-accept` (under M7.4) predates it.

**Product rule (user, 2026-10-12): the system must never stop a sale.** Sri Lankan shops keep selling regardless;
if the system gets in the way they drop it. Prefer "sell, then flag for review" (found-at-till, look-alike duplicates)
over refusing; design every plugin and M8 with this in mind.

**Idea for after the suite ships (user, 2026-10-12):** Pocket Cashier — a phone backup till that syncs every
minute and replays its offline sales on top of the backend. See `docs/ideas/pocket-cashier.md`.
**M8 big picture (user, 2026-10-12):** one Shop Builder that knows every shop (install id, OS, plugins, file /
server mode, update channel), builds and updates each one — forked from karots-pos `cmd/release` + `internal/update`. The
phone app is not in M8 (later, Pocket Cashier). See `docs/ideas/shop-builder-m8.md`.
**For M8 — VAT tax invoice (user, 2026-10-14):** before the first VAT-registered shop goes live, print the IRD's tax
invoice format (Gazette 2481/22, in force 1 July 2026; not issuing a valid one is an offence) as an extra print for
registered shops only — get the gazette's field list from an accountant first, and have the shop's accountant check one
printed invoice and the Tax report. SSCL / income tax stay with the accountant (reports give the turnover).
**For M8 (so it isn't blocked later):** a unique client id on every sale request (a repeat is booked once), receipt
series per device (`P-…` beside `S-…`), and a sale may carry the time it really happened.

**M7.6d Loose versions DONE (2026-10-13)** (plan `docs/plans/2026-10-13-m7-6d-loose-versions.md`, differences 79–81):
Make a loose version… (core `POST /admin/products/:id/loose`, one tx via `conversions.MakeLoose` + `products.CreateTx`;
plugin extras validate before, save after, as a plain create); `stock.DepleteFEFOLots`; a run opens one loose batch per bag
(expiry / batch no / cost carried — karots-pos fix `16b7b97`, pushed) and takes **Got** (`toQty`); slot
prop `ProductFormSection.kind` ('copy' | 'loose'); `TillApi.swapLine(key, productId, qty?)`; alternatives `AltDTO.unit /
sameUnit`. Value per batch is exact to half a cent a unit (cost_price is 2 dp per unit). User asked: the lost weight is
not a write-off — it is in the loose cost (lower margin). User (2026-10-14): that is correct; no "expected vs got" column in run history.
Checks: 150 Playwright flows (dev DB reseeded), go test all-plugins + core-only, 175 web unit tests, verify, verify-portable.
**User tried M7.6d on the :3100 demo (2026-10-13): "everything looks good"** (demo binary updated; the previous one is
`~/karots-accept/karots-suite-all.m76c`). User decision: no expense on a conversion run — staff do it as part of their
day, wages already cover it; the lost weight is in the loose cost.
**M7.7 pharmacy — rethink DONE (2026-10-14), split in two (user: "do the recommended way"):**
- **M7.7a Packs (core) — DONE (2026-10-14)** (spec `docs/specs/2026-10-14-m7-7a-packs-design.md`, plan
  `docs/plans/2026-10-14-m7-7a-packs.md`; `docs/differences.md` 82–86). User's problem: card→loose conversions bloat a
  pharmacy catalogue (every card needs a "(loose)" twin; customers ask 2–3 tablets, "2 days' worth", 14 of a 12-card).
  Built: `product_packs` (migration `00005`; pack barcodes in `product_barcodes.pack_id`; `sale_items.pack_label`);
  full packs ring at the pack price, never dearer (`sales.PackGross` = web `cart/packs.ts`, one JSON fixture); till
  chooser (1 tablet / 1 Card), card barcode adds a card, line split "= 1 Card + 2" + **+ Card**, **Pack…** at the till
  (size typed by the cashier → Items to review); receipt "14 pcs (1 Card + 2)"; product form **Packs** block; stock
  split in lists; purchases / receive / intake **by the card** (`packSize`: payable exact, stock in units); **Fold
  <card> into packs…** for old twins (+ rename). **Core bug fixed in both repos:** `products.Rename` wrote a missing
  `updated_at` column (always failed) — karots-pos `2d58d9f` (recharge carrier renames).
  Checks (2026-10-14): 154 Playwright flows (`packs.spec.ts` 4 new; dev DB reseeded), `go test -p 1` core-only and
  all plugins, 192 web unit tests, `make verify`, `make verify-portable`. Dev server for shots ran on :3101.
- **M7.7b Pharmacy (plugin) — DONE (2026-10-14)** (spec `docs/specs/2026-10-14-m7-7b-pharmacy-design.md`, plan
  `docs/plans/2026-10-14-m7-7b-pharmacy.md`; `docs/differences.md` 87–93; parity 30 pharmacy rows ticked). Built as approved
  below, plus: Add for a **repeat** script is a full fill every visit (a one-off: what is still owed); the after-sale record
  fills **any of the patient's active scripts** (tag first, then medicine group); a cashier medicine search
  (`GET /x/pharmacy/medicines`) for the till's prescription sheet; plugin.json **`requires`** (Shop Builder brings Product
  Plus + Alternatives along). **karots-pos bug fixed (`51e6c6d`):** editing a prescription deleted and re-inserted its drug
  rows, cutting every hand-over link (refill reminders stopped). A restored karots-pos backup leaves the suite's new columns
  blank; readers fall back to live names / the product's group. Checks (2026-10-14): **158 Playwright flows** (`plugin-pharmacy.spec.ts` 4 new; dev DB reseeded with `make e2e-seed` first — a dirty DB failed 7 unrelated flows), `go test -p 1` core-only and all plugins, 214 web unit tests, `make verify`, `make verify-portable`. Final review: self-review (no reviewer agent).
  **User decision (2026-10-14): no allergy ⇄ medicine check** (which chemicals clash with which allergy types gets
  complicated — the red allergy banner is enough). Register paging not built (add if a register gets slow).
  **Playwright stays one worker, one server (user, 2026-10-14)** — don't propose parallel servers; Playwright reseeds
  the e2e DB when it starts its own server (`c60e25b`).
  Approved rethink (keep · change · drop vs karots-pos):
  **A** dispense during the sale — till action 💊 Patient (pick / register, allergies in red, scripts with due badges,
  Add a script line as its alternatives group's best in-stock member); after-sale records the dispense from the sale
  lines (pieces by group, days = pieces ÷ doses/day); "Record for a past receipt" when forgotten; optional generic seam
  `TillApi` customer get/set (patient becomes the sale's customer). Drops the Dispense tab, sale picker, typed qty/days.
  **B** patient = core customer + profile (keep), patient page at 360px. **C** prescriptions: a drug line points at an
  alternatives group (free text allowed), dosage understands **1-0-1**, repeat / stop / edit kept. **D** refills due =
  call list (📞, WhatsApp, Called today), same per-drug logic (lead 5 days, one-off grace 30). **E** controlled register
  read from core stock movements (no double entry); OUT rows carry patient + doctor snapshots; a controlled sale
  without a patient still sells → **Needs a record** list. **F** no medicine wizard: core form + Product Plus fields
  (Generic name / Strength set up by the shop — pharmacy must not write Product Plus data) + Alternatives + packs;
  pharmacy's form section = Controlled (schedule) + Prescription-only (hint, never blocks). **G** till ⓘ "⚠ Controlled".
  **H** dispensing log + CSV. **I** requires Product Plus + Alternatives. Old data: all 7 `pharmacy_*` tables restore;
  old `pharmacy_cd_ledger` becomes a read-only "Old register". Days helper "1-0-1 × 2 days = 4" lives in pharmacy.
- **M7.7b follow-ups from the demo (user, 2026-10-14):** Days… moved onto a medicine's cart line (core seam
  `TillApi.setLineQty`); a script line's Add has linked days ⇄ tablets boxes; a **dose list the shop builds by using it**
  (`GET /x/pharmacy/doses`: every prescription dose + usual dose with its most common per-day, plus 1-0-1 / tds… —
  user chose this over a managed dose table; a table only if printed dose labels need it); product **Usual dose**
  (migration `0008`); **Controlled only for a prescription medicine** (server keeps controlled ⇒ ℞).
- **Dose labels (user, 2026-10-14):** 🏷 Label on a medicine's cart line prints a dose sticker on the label printer via a
  new core **text-only label** (`tspl.Input.Lines`, `POST /till/labels/text`; ½ × · now print as ASCII look-alikes):
  with a patient — name + date, medicine × qty, dose + its words, days · how to take it; without — medicine × qty + date
  first. **Pharmacy → Doses** (migration `0009`, `pharmacy_doses`): pre-filled with the usual doses (put back whenever the
  list is empty, e.g. after a full restore), learns doses used elsewhere (flagged "new" to name), add / edit / delete.
  **How to take it** (instruction) on prescription lines, product form and Days… — common ones + those used (memory).
- **Returns roll back the script (2026-10-14):** a prescription line's given tablets and run-out days are read net of
  core Returns on the dispense's sale; the dispense keeps what was sold (↩ in history / log); the controlled register
  follows returns via stock movements. Dose sheet / till Label show a **true-to-print preview** (`/till/labels/text/preview`,
  `cut` = doesn't fit); prescription lines have days ⇄ tablets. **Final checks (2026-10-14):** full Playwright 128 passed
  + the 4 files that failed while `go test` ran alongside re-run alone 40/40; `go test -p 1` all plugins (44 ok); 219 web
  unit tests. Packs show on the product form for a saved, non-service product whose unit is whole (pcs) — rice in kg
  uses a loose version instead (user asked how medicines get packs).
- **Demo updated for M7.7a + M7.7b (2026-10-14):** `~/karots-accept/karots-suite-all` (all plugins) on :3100 against
  `suite_accept`; previous binary `karots-suite-all.m76d`, DB backup `suite_accept-before-m77-2026-10-14.dump`. Pharmacy
  demo data: Glucophage / Metfor (group Metformin 500mg, Card ×10; Glucophage out of stock), Amoxil (℞), Diazepam 5mg
  (controlled, Schedule II), Panadol (Card ×12); patients Nimal Perera (Penicillin allergy, repeat script Metformin 1-0-1 ×60
  + Panadol tds ×30) and Kamala Silva. No fake sales. Printer emulator `pos_printer` on :8631.
- **M7.8 Repairs (plugin) — DONE (2026-10-14)** (spec `docs/specs/2026-10-14-m7-8-repairs-design.md`, plan
  `docs/plans/2026-10-14-m7-8-repairs.md`; `docs/differences.md` 94–101; parity 32 repairs rows ticked). User's complaint
  that drove it: karots-pos's repair collect used **its own payment form and inputs** — in the suite **all repair money
  goes through core**: **Put on the bill** (job lines tagged 🔧 R-0012, customer set, deposit pre-filled as **On account**
  via new seam `TillApi.addTender`) → the normal Pay; after-sale marks the job collected. Deposit = the customer's
  **advance** (core CreditForm, new `opts` amount / note / mode / onDone), so a job needs a registered customer; cancel →
  refund through the same form; Quick fix = a labour line; Pay the repairer = core expense (`repair_payouts`, live);
  Free re-repair = lines at 100% off (`addProduct` `discount`); faulty part = core Returns (`/till/returns?sale=`). Other
  core seams: `TillAction.claimsText` (a scanned RPR ticket opens the panel), `TillApi.lines`, `TenderMethod.Hidden`
  (karots-pos held deposits settle via the hidden `deposit` tender). Labour product **Repair labour** (Repairs category),
  made on first use, never adopted by name. Warranty by days / part on Apps → Repairs → Warranty.
  **karots-pos bug fixed (`679829e`):** cancelling a job with a deposit left it in the drawer; the till's job panel now
  cancels and refunds in one tx, admin cancel refuses while a deposit is held.
  Checks (2026-10-14): **164 Playwright flows** passed (24 shot steps skipped; `plugin-repairs.spec.ts` 4 new), `go test -p 1`
  core-only and all plugins (45 ok; `TestTillCatalog` failed once right after Playwright on the shared DB, then 4 clean runs),
  221 web unit tests, `make verify`, `make verify-portable`. Self-reviewed gates: till 1366 Night dark + Studio light, admin
  360 + 1366 (fixed two 360px overflows: grid columns pinned to minmax(0,1fr)).
  **Gaps closed (2026-10-14, user asked):** the profit report shares a bill discount (`sales.PaidFactorSQL`, as core's
  P&L); pickup is recorded **inside the sale's transaction** by a new generic core hook **`plugin.SaleLineHook`** (the till
  sends each cart line's `plugin` data with the sale; `sales.WithLineHook`, dispatched per plugin key at the composition
  root) — no after-sale call, a failed sale leaves the job open, another customer paying still collects (never stops a
  sale). Pharmacy's dispense could move onto the same hook later (still after-sale).
- **Core check — discounts in finance reports (user asked, 2026-10-14):** a real sale with a line discount and a bill
  discount (`internal/features/reports/discount_db_test.go`). Line discounts were right everywhere; the **bill discount was
  missing from every line-level report** (categories, top products, product trend, service profit, Print & Copy) — fixed
  with one rule `reports.KeptRevenueSQL` (pre-tax paid factor), differences 102, karots-pos `71f61bc`. Open question for the
  user: the P&L's revenue is the sale total **including tax** while line reports are pre-tax (same as karots-pos; matters
  only for a shop that charges tax).
- **Tax (user chose, 2026-10-14):** the P&L, sales trend, peak hours and sales by cashier count revenue **before tax**; the
  Finance page shows **Tax collected — owed to the tax office** on its own line (and in the sheet). **Tax is charged only by a
  VAT-registered shop with its registration number** (`sales.TaxOnSQL`, read by the sale and by the till's product prices);
  otherwise product tax rates are ignored (hint on the product form; Settings refuses "registered" without a number).
  karots-pos still charges tax whatever the setting — not changed there (its live shops would suddenly stop charging tax);
  a shop moving over that charges VAT must enter its number in Settings → Tax.
- **Demo updated for M7.8 (2026-10-14):** `~/karots-accept/karots-suite-all` (all plugins) on :3100 against `suite_accept`;
  previous binary `karots-suite-all.m77`, DB backup `suite_accept-before-m78-2026-10-14.dump`. Repairs demo parts (Phone
  parts): Galaxy A12 screen 4,000, iPhone 11 battery 5,500, USB-C charging port 900 — 5 each. No demo jobs.
- **Till cash & touch (user, 2026-10-14, after M7.8):** quick-cash chips by the shop's notes (640 → Exact · 650 · 700 ·
  1,000 · 2,000 · 5,000; never a note wasted), Exact / Remaining for card & other methods, one-line pills, Exact spans two.
  Touch keys (till header ✋ Auto · On · Off, per device): a **Notes / Keypad** switch — tinted banknote tiles + coins
  (tap 500 ×1 + 100 ×2 = 700; Undo steps back one tap at a time, Clear empties) or a keypad typing into the focused box. Every slip now follows "ask before printing"
  (`slipOut`, admin too); labels get a preview. Demo on :3100 has all of it.
  Note tray **Undo** (one tap back at a time) and **Clear**. **Combos auto apply** (Settings → Combos, per shop, off by
  default; `combos_settings`, migration combos 0003): a full set in the cart rings as the combo by itself — part sets
  still only hint; a combo the cashier breaks apart isn't re-applied (the Apply card offers it) until the cart empties —
  the plugin's per-sale notes (✕ hidden, broken) live in sale meta, so an emptied cart / New sale forgets them.
- **Till & login polish (user, 2026-10-15):** cart / pay columns grow on wide screens (1080p: 480 / 440 px); touch-sized
  line chips ("Set pack"); a cart-header **Clear**; product cards 4 a row at 1080p with the price pinned to the foot, a
  tinted initial, counter-safe facts (packs + price, warranty, serial, Sold by kg, barcode's last 4) and **Low stock**
  (`TillProduct.lowStock`, at/below the reorder level) — never cost or wholesale (the customer sees the screen).
  Login / lock: centred clock, bigger centred form; lock shows "Locked N min ago" and an open sale's item count.
  **Clock easter egg** (7 taps): a games menu — Coin catch, Blocks, Shop Rush (see AGENTS.md hidden behaviours).
  **Future idea (not now):** sell the games as plugins — the Shop Builder compiles each in, all, or none.
- **M7.9 Reload & Bills (recharge) — rethink APPROVED (2026-10-18)**, spec `docs/specs/2026-10-18-m7-9-recharge-design.md`,
  plan `docs/plans/2026-10-18-m7-9-recharge.md`. User: every customer money flow through the cart + Pay (as repairs) —
  incl. **cash out as payout lines** (a cart may total below zero: "Give the customer"); **commission booked as it
  happens** (rates per carrier per kind, packages with float used) so 1,000 − a 100 package reads 912; counts at open /
  close only check what's left (rate suggestions); running float per device (no session); buying float is **not an
  expense** (karots-pos P&L bug, fix there too); never block on a low float. Dropped: get money on credit, admin
  bill-pay page. **Future idea:** modems with USSD / SIM Toolkit (`docs/ideas/recharge-modems.md`).
- **M7.9 Reload & Bills — DONE (2026-10-18)** (spec `docs/specs/2026-10-18-m7-9-recharge-design.md`, plan
  `docs/plans/2026-10-18-m7-9-recharge.md`; `docs/differences.md` 108–118; parity 46 recharge rows ticked). Built:
  core **payout lines** (`TillApi.addPayout`; a sale may total below zero, Pay says *Give the customer*, receipts print
  *Given* / *GIVEN*), `plugin.ReturnLineHook` + `slots.returnLineFields`, plugin tenders on **credit repayments**,
  `cashflow.MoveInput.Recorded`, `DrawerSection.on`, `ServiceLineInput.noDiscount`; the plugin: running float per
  device (karots-pos's own formula — a restored backup reads as it did), commission per carrier × kind booked with each
  move, packages (+ **New package** at the till, flagged until *Looks right*), bank bills, cash out, wallet tender,
  Returns "how much came back?", optional folded float check at Close only (user: not every open/close), admin Float
  (rate suggestions, a week-old-count reminder, Count now, Buy float), Carriers, Ledger + CSV, Report. **karots-pos fix
  `e8eb07b`:** buying float booked an expense (the P&L counted every refill as a loss). Also fixed: the thermal slip
  prints a line's own label; the login page fit at 360px (since the 10-15 polish); the pinned total strip wrapped.
  Checks (2026-10-18): Playwright 171 + the 2 failures fixed and re-run (recharge 4/4, shell, auth — 15/15), 24 shot
  steps skipped; `go test -p 1` core-only and all plugins (46 ok); 229 web unit tests; `make verify`; verify-portable.
- **Demo updated for M7.9 (2026-10-18):** `~/karots-accept/karots-suite-all` (all plugins) on :3100 against `suite_accept`;
  previous binary `karots-suite-all.pre-m79`, DB backup `suite_accept-before-m79-2026-10-18.dump`. Recharge demo data:
  Dialog (reload 3%, bill charge 30, deposit charge 20, cash out 0.5%, wallet 1%; Shop phone 8,500, target 20,000;
  packages Night 50, Data 100, Data 299, Anytime 549), Mobitel (reload 2.5%; Reload SIM 4,200 and mCash SIM 12,000 —
  two floats on one SIM; Data 200), bank BOC current account. No demo sales.
- **M7.9 follow-ups from the demo (user, 2026-10-18) — DONE (2026-10-18):** wallet tender → **Mobile money** + icon;
  bill account numbers suggested; **float dealers are normal suppliers** (differences 119): a float purchase is a core
  supplier bill (`purchases.CreateBillTx` — a received purchase with no lines; `received_by` / `paid_by` stay empty for
  user 0), paid now = `supplierpay.PayTx` on that bill + `cashflow.MoveTx` (cash for a till / cash locker, online for a
  bank), on credit = the bill alone; refill rows link it (recharge `00021`: `recharge_transactions.purchase_id`,
  `recharge_carriers.supplier_id` = usual dealer, set on Carriers). Counter: core slot **`supplierJobs`** (replaced
  `supplierActions`) — a job in the supplier Workspace Seg; recharge adds **📲 Float**. Checks: purchases bill DB test,
  recharge API test (paid now settles, on credit owes), `plugin-recharge.spec.ts` flow 5 (manager), counter +
  admin-purchasing specs (27 passed), `go test -p 1` all plugins, 229 web unit tests, `make verify`. Shots self-reviewed
  (counter Suppliers 1366/1024 × 4, Carriers 360/1366 × 4; preps `float`, `recharge-carriers`). Old refills unchanged.
- **Report print / PDF held only the rows on screen (user, 2026-10-18):** paged reports (Sales, Returns, Losses,
  Inventory, Activity) now load every row before Print / Ctrl+P (`ReportPage rows=`; `e2e/report-print.spec.ts`). An
  A4-PDF audit of every report found no other gap (no clipping; long reports run onto more pages). karots-pos fixed too
  (`dfb5871`: `?all=1` sheet + core.js `reportPrint`; Recipe Variance's "showing X of Y" note had swapped numbers).
  The demo stays on :3100. Old karots-pos RB-/RL- slips section dropped (everything is a sale; `4804a4e`).
- **Future idea (user, 2026-10-18):** modems / SIMs with USSD or SIM Toolkit (`docs/ideas/recharge-modems.md`).
- **NEXT:** advance / special orders (parked "after M7.10" = after recharge) — rethink proposal first; then M8 Ship.
  (The roadmap below still shows the old numbers, each plugin moved up one.)
**Demo for the review (2026-10-13):** all plugins on :3100 against `suite_accept` (binary `~/karots-accept/karots-suite-all`,
started with `.env` + `DATABASE_URL=…/suite_accept` + `BACKUP_DIR=~/karots-accept/suite-backups`; printer emulator
`pos_printer` on :8631). Backup before M7.6 migrations: `~/karots-accept/suite_accept-before-m76-2026-10-13.dump`; the
previous build is `karots-suite-all.m75`. Stop it before a full Playwright run (or test on :3101).
**Changed during the review (2026-10-13, user asks):** AI Assistant — **Copy only** is the default (keeps the chat the user
already has; Copy & open Gemini / ChatGPT / Claude optional), **own chatbots** (name + https address, this browser only,
removable; built-ins fixed), **Short prompt (chat already set up)**, identify sees **what each category holds** (3 newest
example items, "group only", "has sub-categories"), a **new category / new sub-category** badge on options, clean-up
**Stop here** between parts and its preview keeps real products outside the part. Alternatives — the product form's group
is a **searchable picker with + New group** (no tiers there; groups stay independent of categories). The box now shows
even before the first group exists (user 2026-10-14: keep it — + New group works from there). Re-run 2026-10-14: the full Playwright suite
(150 passed, 24 shot steps skipped) and `go test -p 1 ./...` with every plugin — all green.
M7.6, split three ways (user, 2026-10-13):
- **M7.6a (core) — extra barcodes + Merge products — DONE (2026-10-13)** (spec
  `docs/specs/2026-10-13-m7-6a-barcodes-merge-design.md`, plan `docs/plans/2026-10-13-m7-6a-barcodes-merge.md`):
  - `00003_product_barcodes.sql`: `product_barcodes` (source, added, last scanned), `products.barcode_scanned_at`,
    `merged_into_id`; a trigger keeps one code on one product (its own extra may become main). One lookup
    (`barcodeIs`) for scan / search / import / generate; the till scan and intake restock stamp the last scan.
  - Product form **Other barcodes** (Make main, Remove, Add); intake and purchase entry **Same as an existing one**
    (admin only — the counter's LineEditor never links); products sheet **extra_barcodes** column (`;`).
  - **Merge into…** (form footer; Items to review → Same as an existing one): preview + confirm, price choice
    (keep its own price on its lots / follow the kept one), refused across units / service vs stock / held bill.
    `products.Merge` moves every FK (test reads the catalog), collapses set-up doubles, retires the duplicate
    (hidden everywhere). Generic **`ProductMerger`** hook — all six plugins implement it.
  - User note (2026-10-13): handy for cheap brands that **change barcode every batch** — each new code joins at intake.
  - Checks (2026-10-13): `make verify`, `go test ./...` core-only and with every plugin (`-p 1`), 156 web unit tests,
    **134 Playwright flows** (`barcodes.spec.ts` 3, `products-merge.spec.ts` 2 new), `make verify-portable`,
    `docs/differences.md` 65–67. No karots-pos routes (suite-only feature).
  - e2e fixture: pages a spec opens itself (`browser.newPage()` in a beforeAll) now wait out the login limiter too —
    the extra sign-ins tipped plugin-alternatives' beforeAll into a 429.
  - Review fixes: setting the main barcode to one of its own extras keeps the old main as an extra (form + import),
    in one transaction; the held-bill check reads only the cart's product spots (both cart shapes). `/dist/`
    (Shop Builder output) is git-ignored.
- **M7.6b (core) — batches DONE (2026-10-13)** (spec `docs/specs/2026-10-13-m7-6b-lots-design.md` §1–9, plan
  `docs/plans/2026-10-13-m7-6b-lots.md`; `docs/differences.md` 68–73):
  - `stock.AddStock` (top up a named batch — qty received too — or open one with cost / price / expiry) and
    `stock.MergeBatches` (into the oldest; no movement; emptied rows stay at 0). Batches sheet **+ Add batch**,
    **Merge batches** (weighted cost / earliest expiry / oldest price, all editable).
  - Stock-take **batch by batch** (count, cost, own price, + batch); count sheet one row per batch (`batch`,
    `expiry` columns; old sheets import as before). Wholesale stays per product.
  - Intake asks **which batch** (`matchBatch`), expiry on restock and new items; undo comes off that batch.
  - Write-off → **supplier claim prompt**; Recover → **Replaced it like intake** + extra per unit (owed or paid
    now), migration **`00004_recovery_extra_charge.sql`**; **settle on the receive pages** (admin + counter).
  - **Bug fixed in both repos:** the supplier statement left out recovery credits — karots-pos **`bd5fdff`
    UNPUSHED** for the user.
  - Checks (2026-10-13): `go test ./...` core-only and with every plugin (`-p 1`), 161 web unit tests,
    **138 Playwright flows** (`batches.spec.ts` 4 new; write-off specs now expect the prompt), `make verify-portable`,
    `make check-core`. Self-reviewed (no fresh reviewer this session).
- **M7.6c — AI Assistant (aicatalog) DONE (2026-10-13)** (spec `docs/specs/2026-10-13-m7-6c-aicatalog-design.md`, plan
  `docs/plans/2026-10-13-m7-6c-aicatalog.md`; `docs/differences.md` 74–78; parity 28 aicatalog rows → **418** ticked):
  - **✨ Identify** under the Name field of the product form (so Items to review) and intake's New item — core slot
    `productAssists` (fill name / category / extra); a pick creates a missing category path and is learnt on save
    (`aicatalog_items`, ⓘ "AI identified · name — specs"). Sure answer or up to 4 options, "None of these — let me explain".
  - **Paste mode first** (default for a fresh shop / no key): Copy & open Gemini / ChatGPT / Claude (one reused tab, a tab
    already on the chatbot is only focused), the reply is read on paste / Enter, Chrome reads the clipboard on return
    (**Firefox: Ctrl+V** — it allows clipboard reads only right after a click), short follow-up prompts per tab, Full prompt.
    API optional (presets, Load models, Test AI); an API failure offers the same paste panel.
  - **Clean up**: categories / uncategorized / names (naming style) / duplicates (keep = more sales → core `MergeConfirm`,
    now exported); apply ticked = one run per chunked session, Undo last clean-up, Delete empty categories.
  - **Core seams (generic):** `DataGroup.Secret` (blank in every dump AND every restore — the key never travels);
    slot `productAssists`; **`CategoryMerger`** hook (user asked for the long-term fix: a category merge lets each plugin
    re-point its own category ids and returns a note; undo hands the note back) — Product Plus keeps field scopes on the
    kept category. Bulk entry, Insights, default markup dropped; suite migration `0008_suite.sql` (karots-pos 0001–0007 byte for byte).
  - Checks (2026-10-13): `make verify`, `go test ./...` core-only and with every plugin (`-p 1`), 168 web unit tests,
    **145 Playwright flows** (`plugin-aicatalog.spec.ts` 7 new, incl. an API-mode flow against a local fake provider),
    `make verify-portable`. Self-reviewed (the user runs one whole-M7 review next).

- **M7.5 delivered (user: "do whatever you recommended", A–I, 2026-10-12):**
  - **A** a print service is a core service (`doc_service` = the link: kind, category, on/off); Apps → Print & Copy
    (new or from an existing service; Edit = core ProductForm, Toner & ink = core RecipeEditor).
  - **B** price grid: size rows × B&W/Colour × 1/2-side, **bulk bands** "from N copies" (the user's bulk-discount
    question: bands + the ordinary line discount; combos not involved). **C** paper per sheet by size (2-side =
    ⌈n/2⌉), toner/ink per copy via the recipe, one resolver (`resolve.go`).
  - **D** till: 🖨 chip → service chips → job panel (bands, "N more for Rs X each", Enter adds, cheapest size first);
    a job line's qty is fixed. **E** custom = typed price + note. **F** labour worklist dropped (expense tagged to the
    service). **H** Shop use (in-place confirm, one tx). **I** Reports → Print & Copy (copies / sheets / sales / shop
    use, returned copies off, CSV); jobs recorded after the sale (`tillAfterSale`), linked to their sale line.
  - **G (core):** a service line can be returned — refunded, nothing restocked, its cost booked as a `damage`
    movement (qty 0) so P&L Losses keep the wasted paper. Works for recipe services (coffee) too. A recharge veto
    hook waits for M7.10.
  - **Core seams (generic):** `TillMenuRoot.claims` (+ Panel `product`), `ServiceLineInput.fixedQty`,
    `stock.ConsumeTx`. **Core fixes:** group cards now carry plugin pins (they were always empty), chart tables have
    column gaps, a service line's corrected material count reads "material count corrected" (was "found at till").
  - **Data:** karots-pos 00001/2/4/5 + its new **00006** byte for byte; suite **00007** splits paper vs recipe
    (`restore_db_test.go`). **karots-pos bug fixed (`56e6fcd`, unpushed):** a fractional any-size consumable (toner)
    rounded up to a whole cartridge per job.
  - **Till search finds services (user, 2026-10-12):** `GET /till/products?services=1`, sent only by the Sell screen
    (search + Enter); stock screens (write-off, labels, purchasing) still leave services out. A print service found
    this way opens its job panel; a coffee adds as before.
  - **Checks (2026-10-12):** `make verify`, `go test ./...` core-only and with every plugin (`-p 1`), 149 web unit
    tests, **128 Playwright flows** (`plugin-documents.spec.ts` new, 6), `make verify-portable`, parity **390** (22 documents), `docs/differences.md` 59–63.

**M7.4 Alternatives is DONE (2026-10-11)** (plan `docs/plans/2026-10-11-m7-4-alternatives.md`, spec
`docs/specs/2026-10-11-m7-4-alternatives-design.md`). **Demo with every plugin (2026-10-12, for the user to try):** `~/karots-accept/karots-suite-all` (built with
all 5 done plugins) runs on :3100 against `suite_accept` (plugin tables added), printer emulator
`pos_printer` at http://localhost:8631/recus (shop receipt printer set to `tcp://localhost:9100`). Plugin data
from `scripts/seed-plugins.py` (+ 3 new 0-stock products: Nadu Rice, Keeri Samba, Fanta 1L; clearance settings
lowered to 21 days / 15%). Core-only copy from before: `~/karots-accept/suite_accept-core-only-2026-10-12.dump`
(`pg_restore`). **Stop it before Playwright.** The user's notes from it become small fix commits.

- **M7.4 delivered (user chose A–G, 2026-10-11):**
  - **A** tiers optional: a group starts as a plain list (one blank-named tier, "Reorder when the group has ≤ N");
    Add a quality tier names the first one in the same step; ↑ / ↓ best first; delete an empty tier / a group.
  - **B** swap at the till: a plain line over its stock shows "Out of stock — swap for ⇄ …" (best tier, then most
    stock, up to 3); Swap keeps the qty (merges into a plain line of that product). **On the user's warning** the same
    swaps sit **inside the found-at-till prompt** at checkout, so "Sell anyway" isn't the only way out (no fake count).
  - **C** ⓘ rows "Alternative · Genuine — SanDisk 32GB — 14 left · Rs. 1,800.00". **D** pin "⇄ Genuine" / "⇄".
  - **E** one reorder surface: core Reorder notes + **Hide covered items** (pulls in whole low tiers), filtered before
    paging; the overview page is gone — the group list has tier totals vs level, Low, Low only, CSV.
  - **F** product form / intake section (group, tier, None; a duplicate starts in the source's group).
    **G** disabled members greyed with Remove; picker "In <group>" + **Move here?**.
  - **Core seams (generic):** `ListQuery.AlsoIDs`, `ReorderRow.note/covered` + `ReorderList.annotated/hidden` +
    `?hideCovered=1` (the `ProductReorderAnnotator` hook finally wired); `TillApi.swapLine` + reducer `swap`
    (`lines.ts swapProduct`); slot `tillShortHints` (inside the found-at-till prompt); line hints now span the cart
    line (combos' chip too); `ProductPicker` `hint` prop.
  - **Data:** karots-pos `0001_alternatives.sql` byte for byte, no new migration; an old backup restores with no
    mapping (`plugins/alternatives/restore_db_test.go`).
  - **Checks (2026-10-11):** `make verify`, `go test ./...` core-only and with every plugin (`-p 1`), 144 web unit
    tests, **122 Playwright flows** (`plugin-alternatives.spec.ts` new, 6), `make verify-portable`. Parity **368**
    (15 alternatives). `docs/differences.md` rows 54–58.
  - **Open items fixed (user chose, 2026-10-12):** a member in a low tier suggests its share of the tier's need
    (2 × level − total, split evenly, rounded up; generic `ReorderNote.Suggest`, the larger figure wins); the
    found-at-till prompt focuses the best swap (Enter swaps; with no swaps "Sell N" keeps it).

**M7.3 Combos is DONE (2026-10-11)** (plan `docs/plans/2026-10-11-m7-3-combos.md`, spec
`docs/specs/2026-10-11-m7-3-combos-design.md`).

- **M7.3 delivered (user chose A–H, 2026-10-11; clickable till mockup https://claude.ai/artifact/VpUnYn5UQSZLigFRasLivZ, look A):**
  - **A** three kinds — Combo price · % off · Rs off — exact to the cent from the line's real price (wholesale too);
    leftover cents split a line. Pure `web/src/plugins/combos/pricing.ts`.
  - **B** a combo stays together in the cart (header − / + sets, save, ×; removing an item asks to break it; the
    same combo again adds sets). **C** the till spots full sets scanned one by one (strip: Apply) and part sets
    (chip on the line: add the rest). **D** Starts / Ends dates. **E** health: a disabled member blocks Add.
    **F** sold count per card. **G** Reports → More → Combos (sets, sales, discount given, bills, CSV).
  - **H (receipt):** a combo prints as one block ("Name (combo)", sets × set price → combo price, You saved, Line
    total, then its items) — the one approved change to the frozen receipt layout; A4 receipt and sale views
    group the same way; Returns tags each item.
  - **Core seams (generic):** cart line groups (`CartLine.group`, reducer `addGroup` / `groupCount` /
    `breakGroup` / `removeGroup`, `TillApi.addGroup` + `saleType`), slots `tillCartHints` / `tillLineHints`, and
    sale line groups — **core migration `00002_sale_line_groups.sql`** (`sale_items.line_group`, `_label`,
    `_sets`; old sales have none), `SaleItemRequest.group`, `SaleLine.group*`, `escpos` `printGroup`.
  - **Data:** karots-pos `0001_combos.sql` byte for byte + `0002_window_sales.sql` (dates, `combo_sales`); an old
    backup restores with no mapping (`plugins/combos/restore_db_test.go`).
  - **karots-pos bug fixed in both:** Rs-off combos on a wholesale sale (karots-pos `d83f02c`, **unpushed** for
    the user).
  - **Checks (2026-10-11):** `make verify`, `go test ./...` core-only and with every plugin (`-p 1`), 137 web unit
    tests, **116 Playwright flows** (`plugin-combos.spec.ts` new, 6), `make verify-portable`. Parity **353** (9
    combos). `docs/differences.md` rows 47–53.
  - **Also fixed:** plugin reports now show on the dock's Reports shelf too (`theme/nav.ts`), not only on the
    Reports page.

**M7.2 Product Plus is DONE (2026-10-11)** (plan `docs/plans/2026-10-11-m7-2-productplus.md`, spec
`docs/specs/2026-10-11-m7-2-productplus-design.md`). 

- **M7.2 delivered (user chose A–F, 2026-10-11):**
  - **Fields per category** (subcategories inherit); every surface (form, intake, search, list line, ⓘ, label,
    Fill in, sheets) shows a field only in scope; a product moved out keeps its values, hidden.
  - **Fill in** (Apps → Product Plus → Fill in): fields × products grid, Missing only, category, search, per-cell
    errors, "N changes · Save", leaving unsaved asks (in-app dialog). "N missing" on a field opens it there.
  - **Required is real:** no default needed; missing = in scope, no value, blank default; form / intake refuse blank.
  - **Safer edits:** default change asks Only new (old default written onto untouched products) / Change existing
    too; a chosen value equal to the default is kept; a lossy type change needs a tick (409 `DATA_LOSS` otherwise).
    The form sends only edited values, the product's own and blank required ones — an untouched field keeps
    reading the default (so "Change existing" still moves it).
  - **Core seams (generic):** `slots.productFormSections` + `ProductRequest.extra` (Validate before the write,
    unknown key 400), `products.MetaProvider` (list line), `ProductSheetColumns` (Sheets ▾ "Include …" toggle on
    export / template / import). **Core feature F:** Sheets ▾ **Skip disabled** (off adds an `active` column; import
    honours it and matches a disabled product instead of duplicating it).
  - **Label / Stock intake chips confirmed** (gate shots): a Label field is a top/bottom line option in the shared
    label Designer; intake New item shows only intake fields.
  - **Data:** karots-pos 0001–0004 byte for byte + `0005_field_scope.sql`; an old backup restores with **no mapping**
    (`plugins/productplus/restore_db_test.go`: every field applies everywhere, nothing missing, search works).
  - **Checks (2026-10-11):** `make verify`, `go test ./...` core-only and with every plugin (`-p 1`), 114 web unit
    tests, **110 Playwright flows** (`plugin-productplus.spec.ts` new, Skip disabled in `admin-data.spec.ts`),
    `make verify-portable`. Parity **344** (8 productplus). `docs/differences.md` rows 39–46.
  - **Found, for later milestones:** a karots-pos DB that used repairs / recharge has `payments.method` values
    `deposit` / `wallet` the suite enum lacks, so `TestRestoreKarotsPosBackup` against this machine's karots-pos dev DB
    fails until M7.9 repairs / M7.10 recharge add them (their migrations must). karots-pos's dev DB has no pp rows.
  - **No sheet clashes (user, 2026-10-11):** a field label the products sheet already reads (its columns, synonyms
    like Price / SKU / Qty, `active` — core's `plugin.Core.ProductSheetNameTaken`) or another field's label is
    refused; a field keeping its own label (an old karots-pos one) can still be edited.

**M7.1 Customer requests is DONE** (plan `docs/plans/2026-10-11-m7-1-requests.md`, spec
`docs/specs/2026-10-11-m7-1-requests-design.md`); M7.0 Plugin groundwork + Clearance before it. Rule (user, 2026-10-11): **core never references a plugin
and never changes for one plugin — only generic hooks.** M0–M6.5 summaries below.

- **M7.1 delivered (user chose A–E, 2026-10-11):**
  - **Asks add up:** the till's Note request shows matching requests while typing — **Asked again** adds this
    customer instead of a duplicate; admin rows say "Asked 3× · last asked 2 days ago", most-asked first; Merge.
  - **Three ways to close:** Make product (+ cost, supplier), Link to an existing product, Won't stock with a
    reason the till shows the next asker; Reopen. Edit changes the note and each customer's name / phone.
  - **Waiting list:** a request with a product and an uncalled phone shows "Waiting for stock", then **To call**
    once the product has stock (`tel:` links, Called ✓), then Done. Clear done spares won't-stock rows.
  - **Generic till seam:** `TillAction.noMatch` — Quick item (a search that found nothing) lists those actions
    with the search text (`text` prop on the panel). Core names no plugin.
  - **Data:** karots-pos `0001_requests.sql` byte for byte + `0002_asks.sql` (`reason`, `called_at`,
    `request_asks`); the request row is its first ask, so an old backup restores with **no mapping**
    (`plugins/requests/restore_db_test.go`). Apps → Customer requests (Open · To call · Done, 360px cards).
  - **Checks:** `make verify`, `go test ./...` core-only and with every plugin (with plugins in, run it with
    `-p 1`: TestAdminData's product count races with plugin DB tests on the shared dev DB), 111 web unit tests,
    **102 Playwright flows** (`plugin-requests.spec.ts` new), `make verify-portable`. Parity **336** (7 requests).
    `docs/differences.md` rows 34–38. Real karots-pos backups with requests weren't on this machine to try.
  - **Parked (user, 2026-10-11): special / advance orders** — see "Idea after M7.10" below.

- **M7.0 delivered:**
  - **Core is plugin-free and checked:** `make check-core` (in `make verify`) fails on any plugin name in
    core (code, comments, tests) or a committed plugin selection. A sale paid with a method no plugin
    settles is refused before anything is written (`sales.WithPluginTenders(known, leg)`).
  - **Plugins start:** `internal/app/plugins.go` runs every plugin's Setup over the wired services
    (`plugin.Core.Sales` is the wired one), mounts plugin routes at **`/api/v1/x`** (Admin = admin +
    manager, Cashier = signed in, Public), and feeds the till's product hooks (badges, sale
    suggestion, ⓘ rows, label fields, search). `Plugin.Key()`; `/me.plugins` lists keys.
  - **Go hooks are data/behaviour only.** Old templ-era hook → React slot: AdminNavEntry →
    `slots.apps` + `slots.adminPages` (`/admin/x/<key>/*`); CashierTab → `tillTabs`; CashierMenuRoot →
    `tillMenu`; DrawerSection → `drawerSections`; ReceiptTab → `tillReceiptKinds`; ReportCard →
    `reportCards`; PaletteEntry → apps feed Ctrl K. **Removed with no slot yet** (add one with the
    first plugin that needs it): CashierSupplierAction (recharge's float purchase), ReadinessRow's
    inline target editor (recharge), ProductRowAction (productplus?).
  - **Shop Builder** (`cmd/bootstrap`, user request): `make bootstrap` opens it in the browser — tick
    plugins, OS / processor, binary name, keep a shop's install id or mint one, live build log, support
    PIN at the end. Flags for scripts. Runs on Windows / macOS / Linux with only Go + bun (no make).
  - **Clearance** (`plugins/clearance`, `web/src/plugins/clearance`), reinterpreted (user, all four):
    a markdown ends itself when its stock sells out or a fresh lot arrives; dismiss = snooze for one
    stale period (+ Bring back); dead stock "Disable all shown"; expiring lots floor at their own cost.
    karots-pos migrations byte-for-byte — "ended" / "snoozed" come from `updated_at`, so old backups
    restore with no mapping. Apps → Clearance page (two tabs, Rules sheet, 360px cards), till badge /
    prompt / ⓘ through the core hooks. `docs/differences.md` rows 28–33.
  - **Closing checks (2026-10-11):** `make verify` (check-core, vet, core-only build, `tsc -b`), `go test ./...`,
    108 web unit tests, **97 Playwright flows** green with every plugin built in (`plugin-clearance.spec.ts`
    new), `make verify-portable` passes. Restore of a karots-pos-shaped clearance backup is tested
    (`plugins/clearance/restore_db_test.go`); the user's real backups weren't on this machine to try.
    Parity: **329 rows ticked** (5 clearance). Both open items fixed after: admin-shots' prep steps moved to `web/e2e/shot-prep.ts`;
    clearance writes refresh the page even when they fail.
- **Inputs for M7.1+ (seams deferred to their first user, still generic):** `TenderMethod.Options` /
  `NotAtTill`; `SaleLineHook{Key, SettleTx}` (hooked cart lines settled inside the sale tx); credit
  repayments taking a plugin method with `paid_into`; the three slots above. A plugin's JSON types
  live in the plugin (mirrored by hand in its `web/src/plugins/<key>/types.ts`). e2e specs that need a
  plugin are `web/e2e/plugin-<key>.spec.ts`.

M3 delivered (dock: **Sell** → Customers; **Purchasing** → Purchases · Reorder · Purchase returns · Suppliers; **Setup** → Users, admin only; every page works at 360px):

- **Suppliers:** list with "we owe / they owe us" totals; a supplier page (balance either way, open bills, orders on the way, payments + refunds); pay (one amount waterfalled, bills first or old debt first, or split exactly across bills / old debt / advance) from any locker or open till, or by card / online; refund when they hold our credit; old-balance correction (audited); move products to another supplier; disable / re-enable; printable statement (`/print/statement/supplier/:id`).
- **Customers:** list with due / advances totals and owing · over limit · advance filters; a customer page (balance or advance, credit-limit bar, latest activity); collect (cash into a locker or till, card / online into its account, old debt first); old balance; printable statement.
- **Purchases:** orders and received lists; one entry screen to place / edit an order, receive a walk-in delivery or receive an order (cost prefilled, preferred supplier, due date from credit days, margin guard with a suggested price, forgotten items, short/extra per line, the rest kept on a new order, pay now, F4 / F10); PO print (together or one per supplier) and GRN (`/print/po`, `/print/grn/:id`).
- **Reorder:** low-stock worklist with a demand quantity (90-day sales × 14-day lead, else 2× reorder level) and sold week / month / last year; a cart that survives filter changes; one PO per supplier.
- **Purchase returns — upgraded (user, 2026-09-30):** made against one received delivery, only its lines, at most what's left of its lot, priced server-side at the lot's real cost (after line + bill discounts and free units), credited to that bill first. Stock that came on no delivery (opening / counted / intake) goes back as "old stock": pick product + lot, priced at the lot's recorded cost, off the oldest bills. karots-pos let any product go to any supplier at a typed cost.
- **Touch steppers** (− qty +, like the till cart and counter returns) on purchase / delivery lines (admin + counter) and return quantities.
- **Users:** staff cards, role, PIN reset (signs them out), cashier rights (counter operations, manage credit, see cost), own printer; you can't deactivate yourself; the last admin can't be deactivated or demoted.
- **Also:** Stock → batch sheet (the open item): "Batches of …", Batch price, per-batch Set count / Remove batch. Shared kit: `features/purchasing/` (LineEditor — now stacks at 360px — totals + margin guard, MoneySourcePicker), `features/people/` (OpeningSheet, StatementSheet). `GET /admin/money-sources` + `parseLocation`.
- **karots-pos bugs fixed here and in karots-pos (`ababde1`):** the supplier statement left out supplier refunds; the customer statement dropped a negative opening balance (a pre-system advance). Both made the running balance disagree with the real one.
- **karots-pos fixes of 2026-10-04 carried over** (`docs/karots-pos-fixes-2026-10-04.md`, before M4): a return refunds what was PAID (bill discount shared pro-rata, `sales.PaidFactorSQL`, also in the sales-list profit, Z-report Net and Returns report; the counter return preview shows the same figure); the whole-sale return is a line return of every remaining line; lot cost rounds to 2dp; the sales search finds a sale by the product sold (till word rules, no fuzzy); the counter lists (Returns / Receipts sales, cash, credit, warranty claims, warranty register) page 50 at a time with "Show older"; the dead legacy `sales.RegisterAPI` is gone. **Found while porting, fixed here and in karots-pos (`d55ac5f`):** returns on a part-paid credit sale each cleared up to the whole debt (over-crediting, no cash back); now only what earlier returns left.
- **Tests:** `go test ./...`, 100 web unit tests and **73 Playwright flows** green; `make verify-portable` passes. Parity: **196 rows ticked** (53 in M3). Legacy `RegisterAPI` files removed for customers, suppliers, purchases and purchase returns.

- **M4 Money (2026-10-05)** — dock **Money** → Finance · Cash flow · Lockers · Till sessions · Receipts · Expenses; **Sell** → Sales; **Setup** → Denominations; every page works at 360px:
  - **Lockers:** tiles with live balances + total, archive / bring back, "At the till" default locker + untracked cash-out; per-locker ledger (paged, net from SQL, CR- reprint); Move money, Adjust (bank charge → Expense "Bank charges", interest, count up / down with a reason), Settle card (fee → Expense "Card settlement fee"). `parseAmount` is the money-string parser for every M4 handler.
  - **Expenses:** range / category / search, total from SQL, paged; record from a locker, open till or outside the shop; Void puts the money back (CR- `expense_void`), refused for a plugin-owned expense (`plugin.ExpenseOwner`).
  - **Cash flow:** cash on hand by place, range totals from SQL with own-place moves apart, opening balances on their own line, unified ledger paged + `/print/cashflow`.
  - **Till sessions** (expected / counted / over-short, session page with counts + Z) and **Denominations**; **Receipts hub** (Sales / Cash / Credit / Warranty + plugin kinds, paged, paper sheet with Print again); **Sales** (filters, totals over every match, CSV / XLSX / ODS, sale sheet with reprint and a return refunded from a picked locker / open till); **Finance** (Overview KPIs + trend + payment mix, P&L, Money owed; ECharts lazy chunk with table views).
  - **karots-pos money flaws fixed in both repos:** receivables vs customer advances (`9d1bccd`), expense delete left its cash out (`d66ac18`), Cash Flow totals from 500 rows / transfers counted twice (`9f17be1`), received money counted the change (`3c0cd9d`), only cash gets change (`c5553e7`), and from the M4 review an archived locker took / gave money and stayed the till default (`15f5554`). **Unpushed in karots-pos for the user to push: `c5553e7`, `15f5554`.**
  - **Tests:** `go test ./...`, 101 web unit tests and **80 Playwright flows** green (`admin-money.spec.ts`, `admin-sales.spec.ts` new); `make verify-portable` passes; core plugin-inert; every hand-written file ≤ 500 lines. Parity: **243 rows ticked** (47 in M4).

- **M5 Reports and dashboard (2026-10-07)** — Home is the dashboard; dock **Reports** → the hub + every report (shelf and Ctrl K list them; no tab row); **Sell** → Warranty; **Setup** → Activity (admin only). Every page works at 360px:
  - **Kit** (`features/reports/kit/`): `ReportPage` (Print + `ExportMenu`, `RangePicker noLatest`), `Kpi` / `KpiStrip` (▲▼ vs before), `BarCell`, `AgingBar`, `numbers.ts`, URL search helpers (`rangeSearch` / `reportQS`; **id filters are numbers** — strings were written quoted). Server: `reportRange` / `prevPeriod` / `wantsSheet` — every report answers `?format=csv|xlsx|ods` with every row.
  - **Reports:** Sales (+ hour window, drill-in; the receipts / export are the M4 Sales list's), Peak hours, Top products (Pareto of every product sold), Product sales (vs last year), Sales by cashier, Returns (by return date), Customer / Supplier dues (aging, disabled debtors), Spend by supplier, Purchases, Profit by category (tree), Expenses, Tax (tax-registered only), Inventory valuation (by lots, rows add up), Count sheet (+ `/print/count-sheet`), Top-up & reorder, Losses & Recovery (damage + warranty, Recover sheet), Recipe variance, Service profit. **Folded into existing pages:** Finance (+ export), Cash register = Till sessions (+ totals, export), Batches (+ export), Low stock = Reorder (+ `/print/low-stock`).
  - **Activity** keyset-paged by `(when, source, id)` (rows of one transaction all show), the audit log is its "Changes" source; never the system user. **Warranty register** reuses the counter's `/till/warranty*` endpoints and claim panels (`InfoSheet`). **Dashboard** per the approved mockup (KPIs + sparklines vs yesterday, trend vs the period before, payment mix, top sellers, needs attention, latest sales).
  - **karots-pos flaws fixed in both repos (all pushed):** P&L returns at the sticker price + trend ≠ P&L + UTC day buckets (`275d885`); cashiers merged by name, product-sales UTC days (`d8ee75f`); dues skipped disabled debtors, credited bills aged as unpaid, spend counted returned goods (`3e1b2e1`, `0fabbca`); the till's customer picker stopped at 100 customers (`cd8158f`); report days were the server process's / UTC days, not the shop's (`f93bf97`). Suite-only upgrades: Pareto of the whole, valuation rows by lots, losses list both kinds in range, activity uncapped.
  - **Tests:** `go test ./...`, 104 web unit tests and **87 Playwright flows** green (`admin-reports.spec.ts` new: every report at 1366 + 360 with no side scroll); `make verify-portable` passes; core plugin-inert; every hand-written file ≤ 500 lines. Parity: **296 rows ticked** (53 in M5).

- **M6 Data and settings (2026-10-08)** — dock **Setup** → Settings (admin + manager) · Data (admin); dashboard "Get the shop ready" card (admins); **Sheets ▾** on Products, Customers, Suppliers. Every page works at 360px:
  - **Data:** backup (everything / choose plugin groups / catalogue & setup only), restore (upload or from the backup folder) behind a typed RESTORE — the file is checked first, a **safety copy** of the current data is saved (listed as "Before restore"), a backup from another schema version loads the common columns and lists the plugin tables it left out, then everyone is signed out; automatic backups list; settings sheet export / import (keeps this shop's install id, printers, default locker); **data check** (`internal/features/datacheck`): figures old karots-pos versions saved wrongly, read-only.
  - **Settings:** every karots-pos setting, card by card, receipt preview, logo kept offline, printer test slip (needs a printer — its parity row waits for M6.5), `slots.settingsSections` for M7.
  - **Import / export** for products, customers, suppliers (CSV / XLSX / ODS, template, per-line report). **Stock Capture** syncs from the suite unchanged (`POST /api/auth/login`, `GET /api/sync/catalog`, `legacy_sync.go`).
  - **Real backups restored** (`TestRestoreKarotsPosBackup`, opt-in): dev `pos_db`, `printshop.gz`, `m-u-m-digital-20260922-142337` (22,239 rows), `pos-backup-20260917-133552` — every table matches, receipt numbers follow on. The data check found 1 over-refund and 6 "returned" sales with no return record in the user's own backups (`docs/acceptance.md` §3).
  - **karots-pos bugs fixed here and in karots-pos — UNPUSHED, for the user to push: `776fd8e`** (customer import duplicated "077-123 4567" and phoneless customers), **`5305a0f`** ("1,250.00" imported as 0; a name + price sheet wiped cost / category / supplier, customers' limit / address, suppliers' contact / terms; Excel number cells; CSV line numbers), **`9fa1158`** (a demoted admin kept admin rights until the token expired).
  - **Milestone review (`fix:` commit):** a session now ends when its account's name or role changes — after a restore another shop's user could have the same id, and a demoted admin kept admin rights (editing yourself re-issues your session); settings saves skip the slow printer lookup (`lpstat`, seconds); "groups=" alone means no plugin groups.
  - **Self-update moved whole to M8** (user, 2026-10-08).
  - **Tests:** `go test ./...`, 104 web unit tests and **92 Playwright flows** green (`admin-data.spec.ts` new); `make verify-portable` passes; every hand-written file ≤ 500 lines. Parity: **324 rows ticked** (26 in M6).

**M6.5 closed (2026-10-08).** The user compared the suite with a core-only karots-pos on one demo
shop (`docs/acceptance.md` § "The demo shop both apps hold now").
- **Demo left clean:** both apps were re-loaded from `~/karots-accept/sunil-core-demo.json.gz` at the
  end (identical: 21 products, 337 sales to Oct 7, Rs. 802,570.00, Dilani's till open) and stopped.
  **To bring it back:** `docker start suite_db pos_db pos_printer`, then start both apps as in that
  section (karots-pos `dist/karots-pos-core` on :3000 with `pos_db`; the suite `bin/karots-suite` on
  :3100 with `suite_accept`, `BACKUP_DIR=~/karots-accept/suite-backups`). Receipts print to
  http://localhost:8631/recus. Demo PINs: owner 1234, manager 2222, cashiers 1111 / 1112 / 1113;
  **system user 2273 in both** (karots-pos-core's computed `-support-pin` was refused; its `.env`
  `POS_SYSTEM_PIN` works). **Stop the :3100 demo before Playwright** — it reuses whatever runs there.
- **Done in M6.5:** Karots icon + installable app; receipt credit line → "Karots Business Suite" +
  contact (a receipt redesign was rejected and reverted — the layout stays as karots-pos's); the till
  top bar (`afd86dc`: drawer chip with a panel, shortcuts behind a keyboard button with pin, tabs as a
  dynamic row) — kept, the user worked in it all through M6.5.
- **Fixed from the user's notes:** Settings stock-take toggle updates the menu without a reload;
  Batches' edit sheet behind a visible "Edit batches" button; kiosk lockdown ported from karots-pos
  `core.js` (F-keys, browser combos, right-click; F1 on a counter tab → Sell) with the **system user
  exempt** (suite-only); Lockers "At the till" alignment; switch knob readable in dark; the dock shelf
  rebuilt per theme.md §6 (icon tiles, lead card, fixed heights); **ask before printing shows the real
  receipt** (`?preview=1` on every slip endpoint, decoded from the printer bytes by `escpos.Preview`)
  in the sale overlay and one Print / Skip prompt (`SlipAsk`) for drawer + every counter tab; Print on
  the sale overlay also starts the next sale; Returns' first-line highlight no longer clipped.
- **Found in the self-review and fixed:** Suppliers / Expenses send a cashier without the right back
  to Sell (a typed URL showed a blank page); Labels' Print pinned to the panel foot; on a short kiosk
  screen a pinned Total + Still to pay / Change line over Complete sale (`TotalStrip`); Warranty
  placeholder in the normal font.
- **Closing checks:** `go vet`, `go test ./...`, 106 web unit tests, **92 Playwright flows** green;
  `make verify-portable` passes; touched files ≤ 500 lines; core still plugin-inert.
- **Left open (not blocking M7):** the 12 printer-only parity rows (the emulator can check all but
  the drawer); the shelf lead card's live figure (theme.md §6); Receipts / Sales sheets could use the
  true slip preview instead of their HTML look-alike (offered, not asked for); the §2 same-day figure
  comparison was not reported back as a checklist.
- **Idea for M8 (user, 2026-10-08):** a **file mode** for single-PC shops — binary + `.env` + a
  `data/` folder, the binary running its own embedded Postgres (as Demo Room's `internal/demodb`
  does), so no database install. A shop that grows moves to server mode by: backup in the suite →
  install PostgreSQL → set `DATABASE_URL` in `.env` → start (migrations run) → restore. Server
  mode stays as it is. Not built.
- **Idea after M7.10 (user, 2026-10-11): special / advance orders** — bakery cakes ("1kg chocolate,
  name 'Amaya', Friday"), print jobs, party orders: a customer order with a due date, a deposit taken
  at the till (money must land somewhere real), ready → collected, the balance rung as a sale. karots-pos
  shops misused Customer Requests for this (Demo Room's bakery / comms seeds). Revisit once every plugin
  is ported; likely its own plugin or a requests extension. Not built.
- **Idea for M8 (user, 2026-10-08): Practice mode** for the system user. A "Practice mode" button
  (dashboard / hidden system menu) copies the whole shop into a throwaway database (the suite's own
  backup → restore) and opens a second suite process on it at its own port (e.g. :3200) with a
  "PRACTICE — nothing here is real" banner; sell, print, return, close the till, debug the cashier
  side freely; Exit deletes the copy. Only the system user's session moves; staff keep selling on the
  real shop. No domain code changes (a sidecar process, not in-memory fakes — rejected as too risky).
  Pairs with file mode (the copy is a temp folder). Option then: block the system user from selling
  on the real shop ("use Practice mode"). **Until then the system user can still sell for real** (user:
  keep it, it's how they debug the cashier side) — its sales show "System" as the cashier.

**M6.5 Acceptance and polish (user, 2026-10-07) — prepared in M6: `docs/acceptance.md` + `docs/differences.md`.** The user runs the suite side by side with a core-only karots-pos before any plugin is ported. What it covers:
- a runnable build + short run steps (suite next to karots-pos, own ports and DB);
- a real karots-pos backup restored into the suite, then the same day's figures compared in both (Z report, P&L, stock value, dues);
- a side-by-side checklist by area (till, counter, stock, purchasing, money, reports) for the user to work through;
- real hardware: POS80 receipt printer, label printer, drawer kick (the 11 printer-only parity rows), an old-screen kiosk PC, a phone for the admin.
- the user also runs a **core-only karots-pos** (bootstrap with no plugins) beside it, so the comparison is core against core; give the build steps for both;
- a **"different on purpose" list** (`docs/differences.md`, gathered from every milestone's "Upgrades over karots-pos", "Done:" notes and the user's decisions — e.g. Cash register = Till sessions, Returns by return date, audit = Activity "Changes", Low stock = Reorder, menu cards hard-delete) so the user can tell a deliberate change from a miss and answer keep / change / add back.
The user's notes become a bug / polish list fixed in small commits (same way of working). M7 Plugins starts after that. The M6.5 build is for the user to test, not a customer release (plugin shops can't switch until M7).

**Porting rule (M2, confirmed again in M3):** read karots-pos's `static/js/<screen>.js` and templ, not just the Go handler, and sweep the finished screens against them (the M3 sweep found F4/F10, Pay all, per-line variance, scan → barcode). Where karots-pos is flawed, don't copy the flaw — the user wants upgrades ("this is its upgrade").

## How we work

1. **Opus plans.** One plan per milestone in `docs/plans/`, with the same shape as the M0 plan: tasks with files, interfaces, test-first steps and a commit per step.
2. **Opus executes** the plan too (Sonnet was dropped on 2026-09-29). At every **UI gate** it takes screenshots in all four palette × mode combinations, reviews them itself, and carries on without waiting for approval unless something needs a product decision.
3. **Opus reviews** the whole milestone:
   - build, vet, `go test ./...`, web tests, Playwright
   - screenshots in all four combinations
   - diff reading
   - fixing what's found in one `fix:` commit
4. **No Claude or AI attribution** in any commit or PR, ever.
5. **Work on `main`, local, and unpushed** until the user says otherwise.

What the user rejects:

- **Plain or default-looking UI.** Stock shadcn cards, a karots-pos-like sidebar, and "Modules arrive later" placeholders have all been rejected. Every screen must look like the approved mockups.
- **Missing karots-pos features.** Always check karots-pos for what a screen does before planning it; the user expects nothing to be dropped.

## Design references

- `docs/design/theme.md` is the source of truth for tokens, palettes, login, the dock and the till layout.
- **Shell, login and palettes mockup:** https://claude.ai/artifact/MANYgogqsxC4evmSADNkia
- **Till and dashboard mockup (look only):** https://claude.ai/artifact/GkJjnypEes4GmHiHMDzFT5. Use its "Night Counter" and "Studio" directions. It shows the till mid-sale:
  - groups and a frequently-sold grid
  - a cart with qty stepper, per-line discount chip and expiry badge
  - a customer picker showing balance, advance and limit
  - Cash, Card, Online and On-account methods
  - split tenders and quick cash
  - Fill, bill discount %, and change with Keep options
  - an over-limit warning, the sale-complete overlay, and Hold / New sale

## Inputs for M6.5 and M7

- **M6.5:** follow `docs/acceptance.md`. Fix the user's notes in small commits (same way of working); tick the 12 printer-only rows (the 11 below + POST `/settings/printer/test`) once checked on real hardware.
- **Still unticked, needs a real printer (POS80 + label printer):** no-sale (POST `/drawer/open`), POST `/print/:id`, POST `/money-receipts/:id/print` (admin + cashier), POST `/receipts/credit/:id/print` (admin + cashier), POST `/receipts/warranty/:claimId/print`, POST `/warranty/:claimId/print`, POST `/labels/send` (counter + admin), GET `/labels` (counter), GET `/receipt/:id`, GET `/orders/print`, POST `/settings/printer/test`.
- **M7 shape (user, 2026-10-08) — not started:** M7 is split into parts, each with its own plan,
  parity rows, e2e and review. **M7.0 Groundwork** first (bootstrap writes `enabled-plugins.ts`, the
  React slots, `internal/plugin/hooks.go` rewritten for the SPA, one tiny plugin proving the path),
  then one part per plugin, smallest first: M7.1 clearance (5 rows) · M7.2 requests (7) · M7.3
  productplus (8) · M7.4 combos (9) · M7.5 alternatives (15) · M7.6 documents (22) · M7.7 aicatalog
  (28) · M7.8 pharmacy (30) · M7.9 repairs (32) · M7.10 recharge (46). Reorder if a shop needs one sooner.
- **Reinterpret, don't replicate (user, 2026-10-08):** karots-pos's plugins grew one request at a
  time; the suite rebuilds each one as it should have been designed, seen whole. Every plugin part
  starts with a **rethink step before its plan**: read the whole karots-pos plugin (Go, templ, JS,
  data), list everything it does, mark what was awkward / duplicated / bolted on, and propose a better
  version (keep · change · merge, and how the old data maps over) for the user to decide. **Hard
  constraint:** a karots-pos shop's plugin data must still come across by backup → restore —
  changed tables need a mapping on restore, and no capability a shop relies on disappears silently.
- **Plugin seams waiting for M7:** `slots.reportCards` (hub "More"), `slots.settingsSections` (Settings), `plugin.DataGroup` (Data → choose what to include; a plugin that doesn't register one is lumped into core), `plugin.ActivityContributor` (gets `Filter.Before` / `Limit` — honour them, the merge filters with `Cursor.After`), `plugin.CollectReadiness` (Top-up; the old `EditURL` inline editor isn't rendered — give it a React slot then). A restored plugin shop's plugin tables come back only once its plugin is compiled in.
- **karots-pos plugin/money fixes of 2026-10-09 (`91e2651..847b091`) — design M7.0 around them.** Already carried into core: card/online credit payments and sale tenders must name an account, cash for customers / sale-return refunds only from a cash locker or till, admin ↑ Refund of an advance, loyalty off the UI. Still for M7. The rule behind all of it: **money must always land somewhere real** (a drawer, a cash locker, a card/bank account, or a plugin's ledger), never "recorded only".
  - **`TenderMethod`** = `Value`, `Label`, `SettleTx` (books the plugin's money inside the sale/payment tx), `Options(ctx, actorID)` (where the money can land, e.g. devices; an empty list hides the method; the server re-checks the picked option) and `NotAtTill` (only the plugin may ring it; the till API refuses it). **An unknown method is a validation error, checked before the payment row is inserted.** Today core breaks this twice: `sales.PaymentInput` validates `oneof=… wallet …` (names recharge's method), and `pluginTenderLeg` in `internal/app/services.go` returns nil (records only) for a method nobody settles. Fix both in M7.0.
  - **Credit repayments take plugin methods too** (till and admin), e.g. eZ Cash / mCash into a money-enabled reload device float. The payment row stores `paid_into` (karots-pos migration 0075), shown on staff receipt lists only, **never on the customer's slip** (it shows the method's proper label). `customers.RecordPaymentTx` currently refuses anything but cash/card/online, so extend it through the seam, not by naming a plugin.
  - **`SaleLineHook{Key, SettleTx}`**: a plugin's cart line carries `hook` + opaque `meta`, and the sale hands each hooked line (with its net amount) to the plugin inside the sale tx. An unknown hook refuses the sale. This replaces post-checkout calls such as a reload's float decrease or a document job's log, which must never be best-effort after the sale. The reload float check runs server-side.
  - **Every sale-ringing service is the wired one.** Plugins must get `svc.Sales` (non-cash + plugin legs) in `plugin.Core`, never a bare `sales.NewService`. karots-pos's plugin copy had no legs, so repair collections sent card money nowhere.
  - **Repairs:** apply the held deposit on the collection sale as repairs' **own** `deposit` method (`NotAtTill`), not recharge's `wallet`. Taking a deposit = drawer cash + job record in one tx via `cashflow.MoveTx`, with a receipt. Collect picks the card/bank account for card/online.
  - **Core never names a plugin:** generic till events (`pos-add-lines`, not combos' `pos-add-combo`), suggestion heading = the plugin's `SaleSuggestion.Label`, reorder filter text = `ProductReorderAnnotator.FilterLabel`, and a plugin ships its own front-end code (in the SPA: its module / routes / slots, not a global script). After any core change, grep core for every plugin name and expect **zero hits**. That includes the "e.g. recharge wallet" comments in `internal/features/sales` and `internal/app/services.go`.
- **Money place rule (karots-pos `dab04f7..f3f0d0d`, carried here `86dacd6..12d146c`, 2026-10-09).** `lockers.Accepts` + `cashflow.RequirePlace(Tx)`: **cash ↔ a till or a cash-holding locker, card / online ↔ a bank account, no method (expense, transfer, refill) ↔ either, and a card-machine account ↔ nothing** — its money leaves only through Settle card. Core is done (supplier pay / refund / pay-on-delivery on admin and the counter, expenses + void, locker transfers). **For M7:** recharge's float refill (admin + counter) must call `RequirePlace(src, "")` and drop card accounts from its picker; recharge's admin *Bill payment & cash* keeps its account side free (minus card) but its **cash side is a till or cash locker only**; repairs' *pay repairer* calls `RequirePlace(src, "")`. Reach it through `plugin.Core.Cashflow`, never a copy of the rule.
- **M8 (Ship) — self-update, both sides (user, 2026-10-08):** the shop side (karots-pos `fd2607f`: `internal/update` — check a release feed, download gzipped with progress, install only a signed release whose SHA-256 matches, **back up first**, swap keeping `.old`, relaunch in place, roll back after two failed boots; `cmd/server/update.go`; admin `GET /update/badge`, `POST /update/check`, `POST /update/apply`; the card on Setup → Data, a Setup badge, the dashboard banner) and the release side (`cmd/release`, R2 upload, `shops.json`, `bootstrap -install-id` / `-X`). The suite needs its **own** feed / key / bucket path — never install a karots-pos binary over the suite or the reverse.
- Test data: the dev DB is also the Go-test DB. Run `make e2e-seed` before Playwright or screenshots, and kill :3100 / :5173 first. The e2e server keeps backups in `./backups` (git-ignored); `admin-data.spec.ts` restores a backup it took seconds earlier, so the data is the same for later specs.

### karots-pos till sources (reference kept from the M1 inputs)

The till must do **everything** karots-pos's till does, in the new design. The karots-pos sources to read are in `~/Projects/go/karots-pos`:

- **`static/js/pos.js` + `pos-*.js`** (Alpine mixins). The areas:
  - **`pos-session.js`:** open/close the register (cash plus a plugin float side by side), withdraw and deposit, no-sale drawer kick, shared-drawer "continue last count".
  - **`pos-catalog.js`:** hierarchical group cards (`product_groups`), the frequently-sold grid, customers, held sales.
  - **`pos-cart.js`:**
    - lots and batches: the "which price?" prompt for per-lot pricing, FEFO, the expiry prompt (sell anyway / write off / back), lot-aware removal
    - found-at-till: a count corrects stock UP when a product is missing
    - service and recipe lines
  - **`pos-input.js`:** barcode/search input, quick-sell handoff (`KQ<barcode>X<qty>`).
  - **`pos-payment.js`:**
    - tenders; card and online go to a picked money account (non-cash leg)
    - change, with Keep (rounding) or Keep all as advance
    - fill remaining; bill and per-item discounts
  - **`pos-checkout.js`:**
    - credit / on-account; a credit customer leaves the tender unpicked, so there's no silent cash default
    - the over-limit override for users with `can_manage_credit`
    - oversell confirm and plugin sale suggestions (such as a clearance markdown)
    - instant change display, then print fire-and-forget as ONE print job
- **Hotkeys:**
  - F1 / "⟲ Menu" resets the **search only**; only New Sale clears the cart.
  - F2 / F3 focus search, F4 hold, F9 discount, F10 pay (amount, then Enter).
  - Enter completes the sale; Escape closes the current dialog.
- **Other till features:**
  - the ⓘ product-info popup (cost shown only with `can_see_cost`)
  - supplier actions at the counter (pay, receive, order, behind the `can_handle_suppliers` flag)
  - cashier expenses
  - credit collection (name/phone search), customer advances and refunds
  - returns by receipt and warranty by receipt
  - the drawer kick (a setting)
  - receipts: thermal ESC/POS, with the receipt style from Appearance
  - kiosk auto-fit on screens under 1080p
  - money-direction cue: green ↓ means cash in, red ↑ means cash out
- **Detailed history of each till feature and its bugs:** karots-pos session memory at
  `~/.claude/projects/-home-karots-Projects-go-karots-pos/memory/MEMORY.md`, in the "TILL UX FIXES" section and linked files such as `credit-as-payment-type.md`, `change-kept-rounding.md`, `till-expiry-warning.md`, `batch-dual-pricing-design.md`, `supplier-at-the-counter.md` and `till-checkout-change-and-print-perf.md`.
- **Plugin till actions** (Reload, Photocopy and similar) are slots filled in M7. M1 must leave the slot: a cashier Quick-action / DrawerSection-style area in the till.

## Open items carried forward

- **Product import / export** stays in M6 (user confirmed 2026-09-29).

- **Till top bar:** the drawer pill and hotkey hints are done. The counter name on the pill has no data source yet.
- **karots-pos deviation:** resuming a held sale is refused while another sale is in the cart. karots-pos silently replaced the cart, which lost a live sale.
- **Login hero:** add the tilted receipt card (yesterday's close) and the "Counter 1 / last closed" pills once an API gives that data.
- **Dock:** areas appear as their pages land (M1–M6). Descriptions come from karots-pos's `templates/layouts/admin.templ`.
- **Flaky Go test:** `TestAdminIntake` fails on a fresh `-reset -seed` DB ("Stock-take is turned off in Settings") and passes once other tests have run. It relies on a setting it never turns on. Seen at `a3cdf08` too, so it isn't from the 2026-10-09 money fixes. Fix: have the test turn stock-take on itself.
- **`docs/parity.md`:** 539 karots-pos routes (212 of them plugin routes), 51 ticked. Tick rows only when a route is ported AND flow-checked.
