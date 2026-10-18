# M7.9 — Reload & Bills (recharge) (design)

The user approved the rethink on 2026-10-18 ("do your recommended way … make this an upgrade and simpler").
Their asks, in their words:
- **Same payment path as repairs**: every customer money flow goes through the cart and the normal Pay screen
  (credit, split, card, receipt, print prompt, drawer), never a plugin payment form.
- **The bonus problem**: with 1,000 float a 100 package leaves **912**, not 900 (about 12 comes back), and paying
  the shop by wallet earns commission too. That is why karots-pos counts the float at open and close. Fix it
  properly, and **packages are things we will sell** too.
- Recommended answers to the open questions: never block on a low float; old "Float top-up" expenses added
  back on the P&L; drop "get money on credit" and the admin bill-pay page; an optional customer number on reloads;
  packages draw from the float (physical vouchers are ordinary products).
- **Later (not M7.9):** modems / SIMs with USSD or SIM Toolkit so the suite recharges and reads the float itself —
  `docs/ideas/recharge-modems.md`. The device stays the unit of float so a modem can plug in under it.

Plugin key stays `recharge` (old backups carry `goose_db_version_recharge` and `recharge_*`). Name **Reload & Bills**.

## 1. Why it changes

karots-pos grew recharge one request at a time:
- **P&L bug:** buying float books a core *expense* ("Float top-up") that the P&L subtracts, while reload face
  value is left out of revenue (`pass_through`) — a shop that buys Rs 100,000 of float shows a Rs 100,000 loss.
- **Session-bound float:** balance = this session's opening (or last close + `session_id=0` carry rows) + this
  session's moves; three different balance queries; empty pickers with no drawer; two tills on one SIM disagree.
- **Bonus found only at the count:** between counts every float figure is wrong, so "short" can't be told from
  "commission".
- **Own payment forms** for deposit / bill / get-money (own cash-given, change, charge, on-credit switch, kick);
  "on credit" via `customers.AddBalance` (no limit, missing from the statement); device moves not atomic.
- **Stops the counter:** a reload or bill is refused on a low float — after it was already sent from the phone.
- **Own reload reversal refund** outside core Returns.

## 2. What it does

- **A. One running float per device.** Balance = Σ `float_delta` of the device's rows, shop-wide, no session.
  Every move writes rows inside the transaction that caused it. Device = a SIM / phone / terminal under a carrier,
  tagged **for reload**, **for money** or both (Mobitel's two floats = two devices); a **starting balance** on
  create (`opening` row, no expense); a **target float** (Top-up report readiness); rename / retire, never delete.
- **B. Commission booked as it happens.** Per carrier, a rate per kind — **reload**, **bill**, **deposit**,
  **cash out**, **wallet payment** — as a % and/or a fixed amount per transaction (blank = none). Each move writes
  its own `commission` row (+float) next to the move. **Packages** per carrier: name, price, **float used**
  (e.g. "Dialog 100 data", 100, 88) — commission = price − float used. Live balance after a 100 package from
  1,000: **912**.
- **C. Counts check what's left.** Open / Close till dialogs (drawer section): each float device with its
  expected balance pre-filled, count optional (blank = not counted). Admin **Count now** on the Float page. A
  count writes a `count` row (counted − expected): labelled **+ unexplained (promo?)** or **short**. The Float
  page **suggests a rate** from the last 30 days when counts keep coming out the same way ("counts suggest Dialog
  reload ≈ 3.4%, not 3% — update?"); one tap updates it, never automatic.
- **D. Till: 📲 Reload & Bills** (`tillMenu` root), everything a **cart line**, paid on the normal Pay:
  - **Reload** — carrier → device (skipped when only one) → amount (quick 50 / 100 / 200 / 500 + keypad) or a
    **package tile**; optional **customer number**. Line on the carrier's pass-through product ("Dialog reload ·
    077 123 4567", "Dialog 100 data"). Float −price, commission row, in the sale tx (`SaleLineHook`).
  - **Pay a bill** — account (a cashier-access **bank locker** or a **money device**), bill type (learned from
    use), reference / account number, amount → a pass-through **Bill payment** line ("Electricity · 0123456").
    Bank: `cashflow.MoveTx` locker → External (biller) in the sale tx; device: float −amount + commission.
  - **Send to a wallet** (deposit) — money device, number, amount → pass-through **Wallet deposit** line; float −.
  - **Cash out** — wallet withdrawal (money device) or bank get-money (bank locker), amount → a **payout line**
    ("eZ Cash withdrawal −5,000"); float / bank + amount in the sale tx. The cart may total below zero: Pay says
    **"Give the customer Rs 4,950"**. A customer who withdraws 5,000 and buys 1,000 of goods is one bill.
  - **Service charge** (any of the above, optional, default per kind in Carriers → rates) → a normal (not
    pass-through) **Service charge** line — real revenue.
  - Line hints "📶 Dialog · Shop phone" on every recharge line.
  - **Never blocked:** a float or bank that would go below zero still sells; the device / bank shows red on the
    panel and the Float page ("below zero — count it"). Bank legs pass `cashflow` as already-moved (no overdraw
    guard) — the bill was paid in the bank app before it was rung up.
- **E. Wallet tender.** A customer pays any sale by eZ Cash / mCash: `wallet` method, `tillTenders` detail
  picks the money device (live float shown); float + amount and a wallet-payment commission row in the sale tx.
  Also on **credit repayments** (till and admin CreditForm) through a generic core seam.
- **F. Buy float** (admin Float page; till panel → Float for users with supplier access): device, amount, pay
  from (core source picker, `RequirePlace(src, "")`, no card accounts), reference → one tx: `cashflow.MoveTx`
  src → External (party = carrier, core money receipt, slip via `slipOut`) + `refill` row (+float). **No
  expense** — cash turned into float is not a cost. Optional "carrier gave a bonus on this refill" amount →
  a `commission` row.
- **G. Reload didn't go through = core Returns.** Return the line in counter Returns (`/till/returns?sale=`);
  the refund follows core rules (cash from a till, credit back if sold on credit). Returning a recharge line asks
  **how much float came back** (all / none — wrong number / part) via the plugin's return field; the float row and
  the refund commit in the return's tx (`ReturnLineHook`); a returned reload reverses its commission row too.
  Payout lines are not returnable.
- **H. Admin: Apps → Reload & Bills** (360px): **Float** (devices with balance, target, last count, below-zero
  flag; Count now; Buy float; rate suggestions; earnings card), **Carriers** (carriers, devices, rates, packages,
  service-charge defaults), **Ledger** (every float row: date / carrier / device / kind filters, CSV), **Report**
  (earnings by carrier × kind for a range; counts history incl. old karots-pos sessions). Reports hub card,
  Cash Flow "Reload float" group (`MoneyLocationProvider`), Top-up readiness (target set on the device form —
  the readiness inline editor isn't rendered in the suite), Activity.
- **I. Earnings** (P&L line **Reload & Bills earnings**, `PLIncome`): Σ commission rows + Σ count differences in
  the range (+ old karots-pos service charges and closed-session bonuses before the suite migration). Service
  charges sold as lines are already revenue. A second line **Float bought earlier — not a cost** adds back old
  "Float top-up" expenses linked from `recharge_transactions.expense_id`, on their dates.
- **Dropped:** opening counts as the source of the balance (counts are checks now), session carry rows, the own
  deposit / bill / get-money forms, **get money on credit**, the **admin bill-pay page**, the overdraw refusals,
  the own reversal refund, `recharge_carriers` delete, RB- / RL- slips for new moves (sale receipt + core money
  receipt instead; old ones stay viewable and reprintable in a till Receipts kind "Reload & Bills (old)").

## 3. Core seams (generic — core never names the plugin)

1. **Payout lines.** `TillApi.addPayout({productId, name, amount, plugin})`: a line whose amount the shop pays
   out (stored as a negative-price `sale_items` row on a pass-through service product). A sale may total below
   zero only through payout lines; then the only tender is **cash out of the drawer** (Pay shows "Give the
   customer", one negative cash payment row, drawer expected follows); on-account / credit / card tenders are
   refused for a negative balance. Payout lines can't be discounted or returned; receipts print them as
   "Given −5,000" and the total as "Given to customer". Reports already skip pass-through lines.
2. **`plugin.ReturnLineHook{Key, ReturnTx}`** + return requests carry each line's `plugin` data (and a
   `ReturnField` slot: a plugin's small input on the Returns line, e.g. float came back). Mirror of SaleLineHook.
3. **Plugin tenders on customer payments.** `customers.RecordPaymentTx` accepts a registered plugin method and
   calls its `SettleTx` in the payment's tx (`TenderSettle.CustomerPaymentID`); CreditForm lists plugin tenders
   with their `tillTenders` detail; the payment row stores `paid_into`, never shown on the customer's slip.
4. **`cashflow.MoveInput.Recorded`** — money that already moved outside the suite (a bill paid in the bank app):
   skips the overdraw guard, keeps the place rule.

## 4. Data

karots-pos migrations 00001–00019 byte for byte (00003 adds `payment_method 'wallet'`), then
`00020_suite.sql`:
- `recharge_rates (carrier_id, kind, percent, fixed, charge)` — kind ∈ reload / billpay / deposit / cashout /
  wallet_in; `charge` = the default service charge for that kind. PK (carrier_id, kind).
- `recharge_packages (id, carrier_id, name, price, float_cost, is_active, sort)`.
- `recharge_counts (id, device_id, counted, expected, session_id NULL, created_by, created_at)`.
- `recharge_transactions`: types + `commission`, `count`, `cashout`, `carried`; columns `parent_id`
  (commission → its move), `package_id`, `customer_number`, `bank_locker_id`, `bill_type`.
- `recharge_config (key, product_id)` — the plugin's own products (Bill payment, Wallet deposit, Cash out:
  pass-through services; Service charge: a normal service), created once in a **Reload & Bills** category.
- **Old balances:** one `carried` row per device = karots-pos `DeviceBalances()` − Σ its `float_delta`, so the
  running balance equals what karots-pos showed. `carried` is never earnings. Old `recharge_device_sessions`,
  `recharge_bill_tx`, `recharge_topups`, `recharge_carrier_sessions` stay as read-only history.

ProductMerger: carrier and config product ids. DataGroup: every `recharge_*` table.

## 5. API (`/api/v1/x/recharge`)

Cashier: `GET /till` (carriers, devices with balance, packages, rates / charges, bill types, banks the
cashier may use), `GET /devices?purpose=reload|money`, `POST /counts` (drawer section; `{sessionId, counts[]}`),
`GET /old-slips?…`, `GET /old-slips/:kind/:id`, `POST /old-slips/:kind/:id/print`, `POST /refill` (supplier
access). Admin: `GET|POST|PUT /carriers`, `POST /carriers/:id/toggle`, `PUT /carriers/:id/rates`,
`GET|POST|PUT /packages`, `POST|PUT /devices`, `POST /devices/:id/toggle`, `GET /float`, `POST /counts`,
`GET /suggestions`, `POST /refill`, `GET /ledger` (+ `?format=csv`), `GET /report`.
Line hook `recharge` (sale, return); tender `wallet`.

## 6. Checks

Go: rate maths (percent + fixed, package commission), balance = Σ rows, count row sign, suggestion from counts;
DB: reload sale → float −100 +12; bill from bank (Recorded, below zero allowed); cash out cart netting to a
negative total → drawer −4,950; wallet tender + credit repayment by wallet; refill (no expense); return of a
reload with none / all float back; karots-pos-shaped restore → balances equal the old `DeviceBalances()`,
earnings add-back. Vitest: payout cart totals, Pay "Give the customer". Playwright `plugin-recharge.spec.ts`:
package reload + bill + service charge on one bill paid by credit; cash out + goods → give change; close till with
a count; admin rate suggestion; return a failed reload.
