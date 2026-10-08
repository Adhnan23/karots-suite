# M6.5 acceptance — karots-suite beside a core-only karots-pos

The core is complete (M0–M6). Before any plugin is ported, run the suite next to karots-pos
**built without plugins**, on a copy of a real shop's data, and work through the checklist below.
Your notes become the bug / polish list; plugins (M7) start after that.

This build is for you to test, **not for a shop**: a plugin shop's backup restores without its
plugin records (recharge, documents, repairs… come back in M7), so never take a plugin shop live
on it.

## 1. Run both side by side

Each app gets its own database and port, so the two never touch.

| | karots-suite | karots-pos (core only) |
|---|---|---|
| Database | `suite_accept` in the suite's Postgres (5433) | `pos_accept` in karots-pos's Postgres (5432) |
| Port | 3100 | 3000 |
| Build | `make build` → `bin/karots-suite` | `make bootstrap ARGS="-yes -name karots-pos-core"` → `dist/karots-pos-core` (`-yes` with no `-plugins` = core only) |

Stop any dev servers on 3100 / 3000 / 5173 first.

**karots-suite**

```bash
cd ~/Projects/go/karots-suite
make db-up
docker exec suite_db psql -U suite_user -d suite_db -c 'CREATE DATABASE suite_accept'
make build
set -a; . ./.env; set +a
export DATABASE_URL=postgres://suite_user:suite_password@localhost:5433/suite_accept?sslmode=disable
export BACKUP_DIR=$HOME/suite-accept-backups     # turns on automatic backups + keeps the safety copies
./bin/karots-suite -migrate && ./bin/karots-suite -init && ./bin/karots-suite
```

Sign in as the system user (`0000000001`, PIN from `POS_SYSTEM_PIN`, `2273` in dev) →
Setup → Data → **Restore** → pick the shop's `.json.gz` → type RESTORE. The report lists the
plugin tables it left out and the **data check** (see §3). Sign in again with the shop's own
logins — they came back with the backup.

**karots-pos (core only)**

```bash
cd ~/Projects/go/karots-pos
make db-up
docker exec pos_db psql -U pos_user -d pos_db -c 'CREATE DATABASE pos_accept'
make bootstrap ARGS="-yes -name karots-pos-core"
set -a; . ./.env; set +a
export DATABASE_URL=postgres://pos_user:pos_password@localhost:5432/pos_accept?sslmode=disable
./dist/karots-pos-core -migrate && ./dist/karots-pos-core -init && ./dist/karots-pos-core
```

Sign in as `0000000001` with the PIN the bootstrap printed (or `POS_SYSTEM_PIN` from `.env`),
then restore **the same file** at Setup → Data → Restore. Now both hold the same shop.
(Checked 2026-10-08: the bootstrap builds `dist/karots-pos-core` and leaves the tree clean.)

**Checking a backup without the UI** (row counts, receipt numbers, data check):

```bash
cd ~/Projects/go/karots-suite
DATABASE_URL=postgres://suite_user:suite_password@localhost:5433/suite_db?sslmode=disable \
KAROTS_POS_BACKUP=~/path/shop.json.gz go test -count=1 -v -run TestRestoreKarotsPosBackup ./internal/backup/
```

Add `KAROTS_POS_KEEP=1` to keep the restored database (`kpos_restore_db`) to point a server at.

Done in M6 (2026-10-08): the dev `pos_db` (all plugins), `printshop.gz` (1,593 rows),
`m-u-m-digital-20260922-142337.json.gz` (22,239 rows) and `pos-backup-20260917-133552.json.gz`
(22,275 rows) all restore: every core table has exactly the backup's rows, the next S- / CR- /
DP- numbers follow on, no column was dropped, and only plugin tables were left out.

### The demo shop both apps hold now (set up 2026-10-08)

Both apps were loaded from **one file**, `~/karots-accept/sunil-core-demo.json.gz`, so they hold
exactly the same records:

- **Sunil Super Mart**, core only: Demo Room's "core" shop with its plugins and combos removed,
  plus a tea counter so recipes are covered. 21 products (bulk rice / sugar sacks broken into
  1 kg bags by conversions, recipe goods), 3 suppliers with purchases, 4 credit customers,
  337 sales over eight weeks, a return, a repayment, expenses, till sessions.
- Every date was moved back one day (`shift-one-day.py`), so the history ends at **yesterday's
  close** and today starts empty. One till (Dilani's) is still open from yesterday.
- Logins: Ayesha (owner, admin) `0770000001`, Ruwan (manager) `0770000002`, cashiers Dilani /
  Kasun / Nadeesha `0770000003`–`5`, PIN **1234** for all. System user as usual.
- Where: karots-pos (core only, `dist/karots-pos-core`) on **:3000** with `pos_db`; the suite on
  **:3100** with `suite_accept`, automatic backups in `~/karots-accept/suite-backups`. Logs in
  `~/karots-accept/*.log`.
- **Receipt printer = the emulator** for both (`tcp://localhost:9100`): `docker start pos_printer`,
  then every printed slip shows at <http://localhost:8631/recus>. The drawer kick is off (no
  drawer to see); the 12 printer-only rows can be checked here except the drawer.

To load it again (both apps stopped): reset `pos_db` with `./dist/karots-pos-core -reset`, drop
and re-create `suite_accept` and `-migrate`, then restore the file in each app's Setup → Data.
`coreonly-generator.go.txt` (in the same folder) shows how the file was made from Demo Room's
generator; it lived untracked in karots-pos for one run.

## 2. The same day's figures in both

Pick a busy day in the backup and read the same figures in each app. They should agree. Where
they don't, check §3 and `docs/differences.md` first — a difference listed there is deliberate.

| Figure | karots-suite | karots-pos |
|---|---|---|
| Z report of a closed till | Money → Till sessions → the session → Z report | Money → Cash Register → the session |
| P&L for the day / month | Money → Finance → Profit & loss (pick the range) | Reports → Finance / P&L |
| Stock value | Reports → Inventory valuation | Reports → Inventory Valuation |
| What customers owe | Reports → Customer dues | Reports → Customer Dues |
| What you owe suppliers | Reports → Supplier dues | Reports → Supplier Dues |
| Cash in each locker | Money → Lockers | Money → Cash Lockers |

## 3. The data check

Both apps read the same stored rows. Old karots-pos versions **saved** some figures wrongly
before the fixes found while porting. The suite's data check (Setup → Data, and after every
restore) lists those rows; it changes nothing. On the real backups above it found:

- `m-u-m-digital` (2026-09-22): one sale refunded more than was paid.
- `pos-backup-20260917`: the same refund, and **6 sales marked returned with no return
  recorded** (the old whole-sale return) — they are missing from both apps' Returns report.

Decide per finding: keep, correct by hand (the suite's opening-balance / adjust tools), or ask
for a one-off repair.

## 4. Checklist by area

Work through each in both apps. Note anything missing, wrong, slower or confusing.

**Till (desktop / kiosk)**
- [ ] Sell by search, barcode and group cards; frequently-sold grid; quick-sell handoff (`KQ…X…`)
- [ ] Lots: "which price?", FEFO, expiry prompt (sell anyway / write off / back), found-at-till count up
- [ ] Discounts: per line, bill %, Fill; cash / card / online / on account; split tenders; quick cash
- [ ] Change with Keep / Keep all as advance; over-limit (cashier refused, admin approves)
- [ ] Hold / resume (the suite refuses to resume over a live cart), New sale, F1–F10 keys, ⟲ Menu
- [ ] Open / close the till with denominations, deposit / withdraw to a locker, no-sale, auto-lock
- [ ] Receipt prints (POS80), ask-before-print on and off, reprint

**Counter tabs**
- [ ] Returns by receipt (refund = what was paid), credit collection, advances and refunds
- [ ] Receipts (sales, cash, credit, warranty), write-off, conversions, labels, warranty claims
- [ ] Supplier actions at the counter (pay, receive, order) with "handle suppliers" on and off; expenses

**Stock**
- [ ] Products (add, edit, barcode, review queue), categories, units, cashier menu, services & recipes
- [ ] Stock levels and movements, batches and expiry, stock-take, intake, conversions, labels, damage
- [ ] Import a product sheet, export it, re-import a price-only sheet (only prices change)

**Purchasing**
- [ ] Order → print PO → receive (short / extra / forgotten item, keep the rest), GRN, margin guard
- [ ] Reorder worklist → orders per supplier; purchase returns against a delivery
- [ ] Suppliers: pay (bills / old debt / split), refund, old balance, statement

**People**
- [ ] Customers: collect, old balance, credit limit, statement; import / export
- [ ] Users: roles, PIN reset, cashier rights, own printer; last admin protected

**Money**
- [ ] Lockers (move, adjust, settle card), expenses (record, void), cash flow, till sessions, receipts hub, sales list + admin return, Finance

**Reports & dashboard**
- [ ] Every report at a real range, on desktop and a phone; exports open in Excel; Activity

**Data & settings**
- [ ] Settings (each card), logo, printer test slip; backup (all three kinds); restore; automatic backups; settings sheet to the other app's shop and back; setup checklist

## 5. Real hardware

| Device | What to check | Parity rows it ticks |
|---|---|---|
| POS80 receipt printer | sale receipt, reprint, money / credit / warranty slips, Z, test slip from Settings | POST `/settings/printer/test`, POST `/print/:id`, POST `/money-receipts/:id/print` (admin + cashier), POST `/receipts/credit/:id/print` (admin + cashier), POST `/receipts/warranty/:claimId/print`, POST `/warranty/:claimId/print`, GET `/receipt/:id`, GET `/orders/print` |
| Cash drawer on the printer | opens on cash events and no-sale (pin 2, else pin 5), not on card-only | POST `/drawer/open` |
| Label printer | labels from the counter and admin, the default size from Settings | POST `/labels/send` (counter + admin), GET `/labels` (counter) |
| Old-screen kiosk PC | till fits under 1080p, kiosk Konami exit (admin / system) | — |
| A phone | every admin page and report at 360 px, Data and Settings | — |
