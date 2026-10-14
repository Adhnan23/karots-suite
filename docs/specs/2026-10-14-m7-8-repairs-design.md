# M7.8 — Repairs (design)

The user approved the rethink on 2026-10-14: **A–I all as recommended** ("do everything according to your
recommendation A to I"). The user's own complaint: in karots-pos the till's repair collect uses **its own payment
form and inputs** instead of the till's. In the suite a repair is paid on the **normal Pay screen**.

Plugin key stays `repairs` (old backups carry `goose_db_version_repairs` and `repair_*`).

## 1. Why it changes

karots-pos built repairs before the till had plugin seams, so the plugin rings its own sale behind the cart
(own method / pay-now / account picker, own drawer kick, own print prompt, own on-account maths), holds deposits
in its own ledger (`repair_payments` + a `deposit` tender), prices parts when they're added (drift), pays the
repairer through its own expense form, and turns a 2-minute SIM reseat into a full job. It also has a money bug:
**cancelling a job with a deposit just closes it** — the deposit stays in the drawer and nothing can hand it back.

In the suite the job is a worksheet; money goes only through core: the cart and Pay, the customer's account
(advance), Expenses, Returns.

## 2. What it does (A–I)

- **A. 🔧 Repairs at the till.** A till action (`tillActions`, like 💊 Patient) opens the Repairs panel: a
  find box (ticket, name, phone, device), the open jobs (urgent / overdue first, due chips "due today",
  "overdue 2d"), **+ New job**, **⚡ Quick fix**. **Scanning a ticket on Sell** (`RPR000012`) opens that job.
  **Core seam (generic):** `TillAction.claimsText?(text): boolean` — the till's Enter asks each action before
  looking the text up as a product; a claiming action's panel opens with that text.
- **B. Job intake.** Customer from the core customer search / quick register (**required** — the device stays
  with us and we must be able to call), device, repair type, fault, who repairs it (blank = us), promised date,
  urgent, warranty days (default from settings; per-part mode hides it), notes. Type / device / repairer
  suggest from what was used. Ticket `R-0012` + code `RPR000012` (karots-pos sequence).
- **C. Job lines.** Parts (a product + qty, + warranty days in per-part mode) and labour lines (label +
  amount). The job shows an **estimate at today's prices**; the real price and any discount are the cart's at
  pickup. No per-part discount on the job (old rows keep theirs, shown read-only and applied at pickup).
  Status: Received → In progress → Ready (till and admin); Cancel (both).
- **D. Pickup = the normal till.** **Put on the bill** sets the sale's customer to the job's and adds the
  parts (`TillApi.addProduct`, the job's qty) and labour lines (`addServiceLine` on the labour product, the
  label as the line name, the amount as price), every line tagged `plugin.repairs = {job: id}`. A line hint
  shows "🔧 R-0012" on them; a cart hint shows the job and its deposit. Everything else is Pay as for any sale
  (tenders, split, change, credit, card account, receipt, print prompt, drawer). `tillAfterSale`: a sale with
  tagged lines → `POST /x/repairs/jobs/:id/collected {saleId}` → status collected, `sale_id`, warranty stamped.
  Holding or clearing the cart leaves the job open. A job already in the cart is not added twice.
- **E. Deposit = customer advance.** **Take a deposit** opens core's **CreditForm** (Pay mode) in the panel,
  amount and note pre-filled ("Deposit — R-0012") — core payment, core receipt, core drawer rules. The payment
  id is linked to the job (`repair_deposits`). At pickup Put on the bill pre-fills an **On account** tender of
  `min(job deposits, the customer's current advance, the bill)` (**core seam:** `TillApi.addTender(method,
  amount)`). If the advance was used up elsewhere the hint says so; nothing is blocked. A deposit bigger than
  the bill simply stays as the customer's advance; the cart hint offers **Refund the rest** (CreditForm in
  Refund mode). **Cancel** a job with a deposit → the panel offers the refund the same way. **Core seam:**
  `CreditForm` gains optional `amount`, `note`, `mode`, `onDone(result)`.
- **F. Quick fix** is not a job: a small form (what was done + price) adds a **labour line** to the cart
  ("Repair — SIM reseat"); parts are scanned as usual. It shows in sales under the Repairs category.
- **G. Pay the repairer** (admin + manager, job page): amount, from (core money source picker: lockers + open
  tills, the method↔place rule), note → a **core expense** (category Repairs, service = the labour product,
  "Repair R-0012 — paid Sunil") + `cashflow.MoveTx`, one tx; linked in `repair_payouts (job, expense)`. The job
  reads paid = old `repairer_paid` + its live linked expenses (a voided expense drops out by itself). Paid /
  Not paid yet badge.
- **H. Warranty.** A collected job within warranty offers:
  - **Free re-repair** — a new linked job (`rework_of`), no new warranty; its Put on the bill adds the lines at
    **100% off** (`addProduct` gains `discount`), so stock leaves and the cost shows as a loss.
  - **Return a faulty part** → core counter **Returns** opened on the pickup sale (**core seam:**
    `/till/returns?sale=<id>`), where "faulty" sends it to damage. Labour lines are services, refunded only if
    the cashier ticks them.
  Warranty mode **By days / By part** and **default warranty days** move to Settings (`settingsSections`).
- **I. Kept.** Ticket **label** on the label printer (core `POST /till/labels` with the code, top "R-0012 ·
  device", bottom the customer); **repair slip** on the receipt printer from Go (claim ticket while open; lines
  + warranty once collected; `?preview=1` like core slips); **deadline changes in the audit log**; **profit
  report** (revenue = the pickup sale's lines of the job's parts and the labour product, net of returns, − their cost − repairer paid; date presets);
  **Ready for pickup** report card; **📞 / WhatsApp "your device is ready"** on ready jobs; jobs in Activity.
  Admin: Apps → **Repairs** (Jobs with Open / Ready / Collected / Cancelled / All, search, drop-off dates; the job
  page; Report) — 360px.
- **Dropped.** The repairs receipts tab, the own deposit slip, the own collect / deposit forms, adopting a
  product called "Repair Service" by name (the plugin creates **Repair labour** in the Repairs category once
  and keeps its id).

## 3. Data

karots-pos migrations 0001–0005 byte for byte (0004 adds `payment_method 'deposit'`), then:

`0006_suite.sql`
- `repair_deposits (job_id → repair_jobs, payment_id BIGINT NOT NULL UNIQUE)` — a deposit is a core
  `customer_payments` row.
- `repair_payouts (job_id → repair_jobs, expense_id BIGINT NOT NULL UNIQUE)`.

Old data: every `repair_*` table restores. An old **open** job whose karots-pos deposit is still held
(`repair_payments` deposits − refunds − applied > 0) keeps the plugin's **`deposit`** tender (karots-pos's own
value): Put on the bill pre-fills it, and its `SettleTx` checks the held amount and records `applied` in
`repair_payments` inside the sale's tx. **Core seam:** `TenderMethod.Hidden` — not offered in the Pay picker
(a pre-filled row only). Cancelling such a job refunds it from the till (cashflow Till → External, `refund` row)
— the karots-pos bug fix, also made there as a bug-fix commit.

## 4. API (`/api/v1/x/repairs`)

Cashier (anyone signed in): `GET /jobs?q=&show=open|ready|collected|cancelled|all&from=&to=`, `POST /jobs`,
`GET /jobs/:id`, `PUT /jobs/:id`, `POST /jobs/:id/status`, `POST /jobs/:id/urgent`, `POST /jobs/:id/cancel`,
`POST|DELETE /jobs/:id/parts[/:pid]`, `POST|DELETE /jobs/:id/charges[/:cid]`, `POST /jobs/:id/deposits
{paymentId}`, `POST /jobs/:id/collected {saleId}`, `POST /jobs/:id/rework`, `POST /jobs/:id/refund-legacy`,
`GET /jobs/:id/bill` (lines + deposit pre-fill), `POST /jobs/:id/print`, `GET /suggest`, `GET /config`.
Admin: `POST /jobs/:id/payouts`, `GET /report`, `PUT /config`.

## 5. Checks

Go: `JobTotals` / estimate / due labels / warranty (ported tests); DB: create → lines → deposit link → bill →
collected; legacy deposit tender settle refuses more than held; payout + void; restore of a karots-pos-shaped
dump. Vitest: `claimsText`, `addTender`, `addProduct` discount. Playwright `plugin-repairs.spec.ts`: new job
with a deposit (credit form) → ready → scan ticket → Put on the bill → Pay → job collected and the advance used;
quick fix; cancel with deposit → refund; free re-repair at 0.
