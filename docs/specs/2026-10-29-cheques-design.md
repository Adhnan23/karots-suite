# Cheques — design (2026-10-29)

## Why

Cheques are everyday money for small Sri Lankan shops: customers pay with them (at the counter and when settling
credit), and the shop pays suppliers and expenses with its own. Today the suite can only type a cheque number into a
"Reference" box, and the money moves at once as if the cheque had already cleared — so bank balances are wrong for
post-dated cheques, a bounced cheque has no clean way back, and nobody can see which cheques are waiting or due.

The owner's picture (their words, tidied): *a cheque is another payment method with the date it will arrive; it is
held like the card machine account — recorded, but not money yet — until we review it as collected, bounced or
cancelled. If it bounces or is cancelled, the amount goes back as owed. Remind me on the dashboard when a cheque is
due.*

## Goals

- **Cheque is a payment method everywhere money moves:** till sale, customer credit payment (counter Credit tab and
  admin), supplier payment (admin + counter Suppliers tab), expense (admin + counter Expenses tab). Wherever
  cash / card / online can be chosen today, cheque can be chosen too.
- **Several cheques in one payment**, each with its own **bank, cheque number, amount and cheque date**; they can be
  mixed with other methods in the same payment (e.g. two cheques + cash for the rest).
- **Received and given** cheques both tracked.
- **Nothing changes by itself.** Every cheque stays **Pending** until an **admin or manager** marks it **Cleared**,
  **Bounced** or **Cancelled**.
- **A Cheques page under Money** and **dashboard reminders** when cheques are due.

## Non-goals / anti-goals

- **No bounce charge** (owner: don't charge). No "don't accept cheques from this customer" flag.
- **No automatic status change** on the cheque date — the owner reviews each one.
- **Never stop a sale** (product rule): a bounce adds debt even past the customer's credit limit; a cheque at the till
  is never blocked by a credit limit.
- Not a plugin: every shop gets cheques, so it is core. Plugins don't need changes — their money already goes
  through the till's Pay and core money forms.
- No bank-statement import, no printing cheques, no "Deposited" in-between step.
- Walk-in sales can't be paid by cheque (a cheque needs someone to chase if it bounces).

## How it works

### A cheque

| Field | Received (from a customer) | Given (to a supplier / for an expense) |
|---|---|---|
| Bank | the customer's bank — text, suggested from a built-in list of Sri Lankan banks + names used before | — (it is the shop's own account, below) |
| Drawn on | — | **one of the shop's bank places** (`lockers` kind `bank`) — required |
| Cheque number | required | required |
| Amount | required, > 0 | required, > 0 |
| Cheque date | required (may be today, past or post-dated) | required |
| Who | the customer (required — a named customer) | the supplier, or the expense |
| Note | optional | optional |

A warning (not a block) when a received cheque repeats a bank + number already on file, or a given cheque repeats a
number on the same bank place.

### Statuses (both directions)

- **Pending** — taken or written; waiting.
- **Cleared** (with the date it cleared — today by default, can be set back to the cheque date) — received: the owner picks **which bank place** it went into (the last one used is pre-filled); given:
  the money leaves the bank place it was drawn on.
- **Bounced** — the amount goes back as **owed** (below). No charge.
- **Cancelled** — called off by agreement (paid another way, cheque torn up). **Exactly like a bounce** for the
  money and the balances; only the word differs.

Cleared, Bounced and Cancelled are **final**. While Pending, the bank / number / date / note can be corrected (not the
amount or the person — cancel and take a new one instead). Every change is written to Activity / audit with who and
when. Only **admin and manager** can change a status or correct details; a **cashier can take** a cheque (till,
counter Credit tab) and **write** one wherever they can already pay a supplier or expense today.

### The money (all through `cashflow.MoveTx`, inside the caller's transaction)

**Received cheque** — like the card machine account:
- When taken, the paid amount counts at once: the sale is paid / the customer's balance goes down, exactly like any
  other method. The money goes into a single **"Cheques in hand"** money place (a new locker kind `cheque`, created
  by the migration, one per shop, not deletable, not offered as an ordinary place to move money into or out of, not
  part of a till drawer).
- **Cleared** → move from Cheques in hand to the chosen bank place.
- **Bounced / Cancelled** → move out of Cheques in hand (to outside), and the amount is added back to the customer's
  balance as debt, with a line on their statement: *"Cheque 123456 (BOC) bounced"* / *"… cancelled"*. A till sale
  stays a sale (stock, profit unchanged); only the money becomes owed.

**Given cheque** — the bank keeps the money until it is cashed:
- When written, the supplier payment / expense is recorded at once (the supplier's balance goes down; the expense
  appears in expenses), but **no money leaves the bank yet**.
- **Cleared** → the money leaves the bank place it was drawn on (dated the clearing day).
- **Bounced / Cancelled** → nothing leaves the bank. A supplier is owed the amount again (a line on the supplier's
  statement). An expense becomes **unpaid** again: it stays one expense, shows "Not paid — cheque bounced" and can be
  paid again (cash or a new cheque) without counting the expense twice.
- A supplier cheque that paid particular bills (allocations): on bounce / cancel those bills become unpaid again by
  the same amounts, so supplier dues and the purchase list stay right.
- Each bank place shows **Balance now** and **After cheques** (balance minus its pending given cheques). If After
  cheques is below zero, it shows a warning there and on the dashboard.

### At the till

- **Cheque** is a tender next to Cash / Card / Online / On account. Choosing it on a walk-in sale asks for a
  customer first (pick or quick-add — the same flow On account uses).
- The cashier can add several cheques; each is a line in the tender split. A cheque can't pay more than what's still
  due (no change is given on a cheque).
- The receipt prints each cheque: bank, number, date, amount. The till close is not affected (cheques are not in the
  drawer).
- Outside the till, a customer's cheque **may be more than they owe** — the rest becomes their advance, exactly like
  cash today (this is how a repair deposit / advance is paid by cheque).
- A return of a sale that was paid by cheque refunds the way returns do today; the cheque keeps its own life (if it
  later bounces, the customer owes it).

### The Cheques page (Money → Cheques)

- Two tabs: **Received** and **Given**. Filter by status (Pending first), date range, customer / supplier, bank place.
- Each row: cheque date, who, bank / drawn on, number, amount, status, and how many days until / since the date.
- Totals at the top: pending received (Cheques in hand), pending given (to be cashed), and due this week.
- Tap a cheque → its details, where it came from (sale / payment / expense, linked), its history, and the action
  buttons (Cleared… / Bounced / Cancelled / Fix details) for admin and manager.
- Works at 360 px (cards on phones, like every admin list).

### Dashboard reminders

- **Cheques to deposit** — received cheques that are Pending and whose date is today or earlier: count + total.
- **Your cheques due** — given cheques Pending with a date in the next 3 days or already past: count + total, with a
  warning when a bank place's After cheques goes below zero.
- Each opens the Cheques page with that filter.

### Elsewhere

- Customer and supplier statements show cheque payments with bank + number, and bounces / cancels as their own lines.
- Money reports: Cheques in hand is its own line wherever money places are totalled (Finance "what the business is
  worth", cash flow by place); it is not counted as cash or bank.
- Docs: `docs/guide/money.md` (+ till and people pages), a screenshot of the Cheques page and a dashboard reminder
  (`make docs-shots`), Demo Room data with a handful of cheques in every state so the shots aren't empty.

## Constraints

- Core only; plugin-inert (AGENTS hard rule). Money only through `cashflow.Move/MoveTx`. Sales stay the source of
  truth for revenue.
- New migration `00014_…`; adding `cheque` to the `payment_method` enum and `cheque` to `lockers_kind_check`
  (`ALTER TYPE … ADD VALUE` can't be used in the same transaction that adds it — split or order accordingly).
  `customer_payments.method` is text already.
- Old backups and karots-pos imports have no cheques and must still restore.
- DTOs in `dto.go` → `make types`; files ≤ 500 lines; theme tokens only; 360 px.

## Ripple effects — check every one (owner: "this is a core change, be careful")

Found by searching for code that lists payment methods or money-place kinds (2026-10-29). The plan must have a task
or an explicit "no change, because…" for each; the final review re-runs the search
(`git grep -nE "'online'|\"online\"|kind *(=|IN|NOT IN)|lockersOf\("`) to catch new ones.

**Payment methods (cash / card / online / credit) are listed in:**
- Go: `internal/features/sales/service.go` (`coreTender`, account checks, `SplitTender`), `service_summary.go`
  (takings by method → X / Z report, till close), `store_query.go`, `sales.go`, `internal/features/reports/reports.go`,
  `internal/api/admin_dashboard.go`, `cashflow/ledger.go`, `customers/payments.go`, `supplierpay/supplierpay.go`,
  `internal/api/counter_credit.go`, `counter_suppliers.go`.
- Web: till `pay/Tender.tsx`, `cart/checkout.ts`, `sales/filters.ts` (sales list filter by method),
  `people/customers/CustomerPay.tsx`, `counter/credit/CreditForm.tsx`, `counter/suppliers/PayForm.tsx` +
  `StockForm.tsx`, `purchasing/suppliers/SupplierMoney.tsx`, `purchasing/purchases/PurchaseEntry.tsx`,
  `purchasing/MoneySource.tsx`, `money/expenses/ExpenseForm.tsx`.
- Printing: receipts, reprints, X / Z reports, customer / supplier statements, money receipts — every slip kind in
  both **Classic** (`internal/escpos`) and **Image** (`features/print/templates`, all ten receipt styles) must show a
  cheque line properly.

**Money-place (locker) kinds are branched on in:** `internal/features/lockers/lockers.go`, `internal/api/admin_settings.go`
(default till locker excludes bank / card — must exclude `cheque`), `money/lockers/LockersPage.tsx` + `LockerMoves.tsx`
(Cheques in hand: no manual moves, no edit / delete, no new `cheque` lockers), till `Tender.tsx`, counter forms,
`web/demo/swatch.ts`. Every place picker in the app must leave Cheques in hand out.

**Shared pay-out piece (so plugins don't each build cheques):** core gets one Go helper for *giving* money — method
(cash place / bank / **cheque lines**) → books the supplier payment or expense and either moves the money now or
records the given cheques — and the shared React "paid from" picker (`purchasing/MoneySource.tsx` /
`MoneySourcePicker`) gets a **Cheque** choice with the cheque-lines editor. Core forms and plugins all use these two;
nothing in core names a plugin.

**Plugins (core must stay plugin-inert; plugins only adjust their own code). Owner's examples, 2026-10-29:**
- **Recharge — float fill by cheque** (owner): the refill (`plugins/recharge/api.go` `RefillInput`, admin + counter
  refill, `FloatSheets.tsx`) pays a dealer as a core supplier bill "paid now from a source". It must offer Cheque via
  the shared picker + Go helper; a bounced / cancelled refill cheque leaves the dealer owed (core supplier side) —
  the float already added stays (the dealer still sent it).
- **Repairs — customer pays by cheque** (owner): pickup goes through the till's Pay → the cheque tender works; the
  **advance / deposit** (owner) is a core customer payment → cheque works there too. A bounced advance cheque puts the
  amount back on the customer's balance, so the job's usable advance shrinks; `plugins/repairs/calc.go` already reads
  the live balance — check a job's money panel after a bounce.
- **Repairs — paying the outside repairman by cheque** (owner): `plugins/repairs/api_money.go` `payout` creates a core
  expense and moves the money itself. It must use the shared Go helper + picker so a cheque is possible. A bounced /
  cancelled payout cheque makes that expense unpaid again; the job's "repairer paid" (`calc.go` `repairerPaid`) must
  not count an unpaid expense.
- **Recharge** picks lockers by kind (`plugins/recharge/hooks.go`, `api.go` sets method `online`,
  `web/src/plugins/recharge/FloatSheets.tsx`) and adds its own `payment_method` value (`walletMethod`, from a
  karots-pos migration) — the new enum value must not collide or reorder; its float / dealer flows must not offer
  Cheques in hand. Its supplier bills (paid now / on credit) go through core supplier payment, so a cheque there
  just works — check it.
- **Every plugin that moves money itself** (searched 2026-10-29 — only two): `plugins/repairs/api_money.go` (payout
  above; line ~221 hands a cancelled job's deposit back from the till — cash refund, no cheque needed) and
  `plugins/recharge/api.go` (refill above) + `plugins/recharge/hooks.go` (sale / carrier legs into bank lockers —
  no cheque, but they pick lockers by kind, so they must never see Cheques in hand).
- **Plugin tenders** (`internal/plugin/hooks.go`, `sales.WithPluginTenders`): `cheque` becomes a core tender, so it
  must be in `coreTender` and never dispatched to the plugin hook.
- **Repairs, Pharmacy, Documents, Combos, Clearance, Requests, Product Plus, Alternatives, AI Assistant**: their money
  goes through the till's Pay — a cheque tender reaches them for free. Check that a repair pickup / pharmacy sale /
  print job can be paid by cheque with a named customer, and that any plugin report that splits by method still adds
  up.
- Run the full test suite **with every plugin compiled in** (`make plugins P=all`) and core-only; `make check-core`.

**Data and safety:**
- `internal/features/datacheck` compares stored money rows: a given cheque's deferred bank move and a supplier
  payment with no money move yet must not be reported as damage.
- Backups / restore (`-backup` / `-restore`, Setup's bring back, cloud copies): an old backup without the cheque
  tables and the new enum values must restore and migrate; a new backup with cheques must restore.
- karots-pos import (`-import-pos`): its data has no cheques; must still import.
- Audit / Activity: status changes logged; the system user stays excluded as today.
- Self-update: the migration runs on boot after an update (BootGuard) — it must be safe to run on a shop's real data.

**Demo Room (`internal/demo/gen`):**
- `gen/money.go` and `gen/load.go` pick methods / lockers by kind — give the five built-in shops some cheques in
  every state (pending received / given, cleared, one bounced, one cancelled), dated by the time machine, so the
  Cheques page and the dashboard reminders aren't empty. The **Sim** (shop selling on its own) may take an
  occasional cheque from a named customer.
- Demo years are cached (`cmd/bootstrap/demoyears.go`): changing the gen must invalidate the cache (it keys on what
  shapes the data — check).
- Plugin gens (`DEMO_SHOP=<key>`) must still build.

**Tests and docs:** Go DB tests for every money path (take, clear, bounce, cancel × sale / credit payment / supplier
/ expense; balances and statements after each); Vitest for the till tender split; Playwright flows for a cheque sale,
the Cheques page review and a counter cheque; existing e2e must stay green (never weaken a test). Guide pages +
`make docs-shots`; README feature grid if it's headline.

## Left to the builder

- Table shape (one `cheques` table with direction + links to the sale payment / customer payment / supplier payment
  / expense is the obvious one), how the till's tender split carries the cheque fields, and how the expense
  "unpaid again" state is stored.
- The built-in Sri Lankan bank list.
- Where exactly the supplier-side "owed again" entry lives (it must show on the supplier statement and in supplier
  dues).

## Calls made by Claude (check these)

- One word **Cleared** for both directions (collected / cashed).
- One shared "Cheques in hand" place, not one per bank.
- No change given on cheques at the till (a cheque can't exceed what's due there); outside the till an extra
  amount becomes the customer's advance.
- Clearing date defaults to today and can be set back as far as the cheque date.
- Bounced / cancelled supplier cheques reopen the bills they paid.
- Refill float already added stays when its cheque bounces (the dealer still owes nothing back; the shop owes the
  dealer).
- Details editable only while Pending; amount and person never editable.
- "Your cheques due" looks 3 days ahead.
- Bank suggested from a list + past names; duplicate number = warning only.
- A bounced/cancelled expense cheque reopens the same expense as unpaid instead of deleting it or adding another.
