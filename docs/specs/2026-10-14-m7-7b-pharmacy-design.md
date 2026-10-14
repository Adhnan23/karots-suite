# M7.7b — Pharmacy (design)

The user approved the rethink on 2026-10-14 ("A to I perfect … do the recommended way"). Builds on M7.7a packs: a
medicine is **one product counted in tablets** with a Card pack, so every quantity here is tablets and no conversion
math is needed. Requires **Product Plus + Alternatives** (karots-pos decision: the pharmacy always ships with both).
Core stays plugin-inert: pharmacy attaches through the seams in §8 only.

## 1. What karots-pos does (kept)

- **Patient = core customer + clinical profile** (`pharmacy_patients`: dob, sex, NIC, allergies, notes; one per
  customer). Walk-ins are registered inline.
- **Prescriptions** (`pharmacy_prescriptions` + `_items`): doctor, reg no, date, notes, repeat + interval, stop /
  resume (`active`), edit in place; items = drug (product or free text), dosage text, quantity, doses per day.
- **Dispensing** (`pharmacy_dispenses` + `_items`) linked to a core sale; per-drug refill rows (`prescription_item_id`,
  `days_supply`).
- **Refills due**: per drug, last hand-over + days covered; repeat scripts show from 5 days before run-out and stay;
  one-offs show until 30 days past run-out; stopped scripts never.
- **Controlled drugs** (`pharmacy_controlled_products`: schedule) with a per-drug register and running balance;
  returns roll it back; patient + doctor names are snapshots.
- Till ⓘ "⚠ Controlled drug · <schedule>".

## 2. What changes (approved A–I + packs)

- **A. Dispense during the sale.** Till action **💊 Patient** (a panel):
  - search patients (name / phone / NIC) or any customer (made a patient on pick), or **register** a walk-in (name,
    phone, allergies);
  - the patient's **allergies in a red banner**; their scripts with each drug's due badge (overdue / due soon /
    covered until) and **Add** per drug (→ §8 `addProduct`: the drug's product, or its alternatives group's best
    in-stock member, quantity = what's still owed for the script (qty − dispensed), the line tagged with the script row);
  - **Use for this sale** sets the sale's customer (receipt name, history) and remembers patient + script on the sale.
  - After the sale commits, the dispense records itself (`tillAfterSale`): patient, script, the sale; each script row
    gets the tablets sold for it — tagged lines first, else lines whose product shares the row's alternatives group;
    days = tablets ÷ doses per day. Untagged medicine lines are kept as off-script items.
  - **Record for a past receipt** (panel, empty cart): pick a recent receipt → same record.
  - Drops: the Dispense tab, sale picker, "last undispensed sale", typed quantity and days.
- **B. Patients page** (Apps → Pharmacy): list (search, due count), patient page (profile, allergies, scripts,
  dispensing history with ↩ on returned sales), works at 360px.
- **C. Prescriptions**: a drug line picks a **medicine group** (alternatives group) or a product, or free text; dosage
  understands Sri Lankan **1-0-1** (morning-noon-night → 2 a day; 1-1-1 → 3; 0-0-1 → 1; "1/2-0-1/2" → 1) and fills
  doses per day (editable).
- **D. Refills due = call list**: patient, phone (📞 tel: link, **WhatsApp** wa.me link with "Your <drug> runs out on
  <date> — we have it ready"), drug, due, **Called** (stamps `called_at` on the script row; hides it for 3 days),
  Stop reminders. Report card "Pharmacy — Refills due (N)".
- **E. Controlled register read from core stock movements** for a controlled product: IN = purchase / intake /
  return / recovery / adjust up / conversion in; OUT = sale (patient + doctor from the dispense snapshot, else
  "— no record yet") / damage / write-off / adjust down. Running balance from movements; the opening balance is the
  product's stock before the first shown row. **Needs a record**: controlled sales with no dispense → **Add record**
  (pick patient + doctor; makes the dispense). Printable per-drug page (A4 print route).
- **F. No medicine screens.** Pharmacy's product-form section: **Controlled** (+ schedule) and **Prescription-only**
  (`pharmacy_medicines`: product_id, rx_only; schedule stays in `pharmacy_controlled_products`). Generic name /
  strength are the shop's own Product Plus fields (pharmacy never writes Product Plus data); the page shows a hint
  when no "Generic" field exists.
- **G. Till hints** (never block): a line of a controlled product shows **"⚠ Controlled — add the patient (💊)"**
  until a patient is on the sale; a prescription-only line shows **"℞ Prescription medicine"**; ⓘ shows controlled +
  schedule and Rx-only.
- **H. Dispensing log** (Apps → Pharmacy → Dispensed): date range, Controlled only, CSV.
- **I. Days helper**: in the 💊 panel (no script) "1-0-1 × [2] days = 4" → **Add 4** to the picked product.

## 3. Data

- karots-pos `0001`–`0006` **byte for byte** (`pharmacy` goose table) so an old backup restores with no mapping.
- Suite `0007_suite.sql`:
  - `pharmacy_medicines (product_id PK → products ON DELETE CASCADE, rx_only bool)`;
  - `pharmacy_dispenses` + `patient_name`, `prescriber_name` (snapshots; backfilled from the joins);
  - `pharmacy_prescription_items` + `group_id` (→ alt_groups ON DELETE SET NULL; backfilled from the product's
    membership) + `called_at timestamptz`.
- The old `pharmacy_cd_ledger` stays: shown read-only as **Old register** under the drug (not in the balance — its
  hand-typed INs duplicated purchases).
- `ProductMerger`: re-point product ids in prescription items, dispense items, controlled, medicines.
- DataGroup "Pharmacy": every `pharmacy_*` table.

## 4. API (`/api/v1/x/pharmacy`)

| Route | Who | What |
|---|---|---|
| `GET /patients?q=` · `POST /patients` · `GET/PUT /patients/:id` | cashier+ | search / register (customer + profile) / profile |
| `POST /patients/from-customer/:cid` | cashier+ | make a customer a patient (idempotent) |
| `GET /patients/:id/scripts` | cashier+ | scripts with per-drug due + owed + suggested product |
| `POST /prescriptions` · `PUT /prescriptions/:id` · `POST /prescriptions/:id/active` | cashier+ | write / edit / stop-resume |
| `POST /dispenses` (`{saleId, patientId, prescriptionId?, lines:[{productId, rxItemId?}]}`) | cashier+ | record against a sale (server reads the sale) |
| `GET /sales/recent?q=` | cashier+ | receipts to record against (past receipt) |
| `GET /refills` · `POST /refills/:rxItemId/called` | admin, manager | call list |
| `GET /controlled` · `GET /controlled/:pid` · `GET /controlled/missing` | admin, manager | register list / one drug / needs a record |
| `GET /dispenses?from&to&controlled=1` (+ `.csv`) | admin, manager | log |
| `GET /till-info?ids=` | cashier | controlled / rx-only flags for cart hints |

## 5. Screens

Apps → **Pharmacy** (`/admin/x/pharmacy`): tabs Patients · Refills due · Controlled · Dispensed. Patient page with
Scripts (+ New prescription sheet), History. Till: 💊 Patient action panel, line hints, ⓘ rows. Print route for a
controlled drug's register (plugin admin page with print CSS).

## 6. Checks

- Go: 1-0-1 parser; refill due rules (karots-pos tests ported); dispense from a sale (tagged + by group, days);
  register from movements (purchase in, sale out with patient, return in, running balance); needs-a-record; restore of
  a karots-pos pharmacy backup (`restore_db_test.go`); merger.
- Web: dosage parser, due badge.
- Playwright `plugin-pharmacy.spec.ts`: register a patient at the till, write a script, Add from it, sell → dispense
  recorded with days; controlled line without patient → hint → sale goes through → Needs a record → add; refills call
  list; 360px pages.

## 7. Out of scope

Prescription photo (no file storage yet), SMS sending, insurance / claims, dose labels printing (later with labels).

## 8. Core seams (generic, no plugin named)

- `TillApi.customerId` + `setCustomer(id | null)` — read / set the sale's customer.
- `TillApi.addProduct(productId, { units?, plugin? })` — add a product line by id (pack chooser skipped; plugin data on
  the line, kept on held sales).
- `TillApi.saleMeta` / `setSaleMeta(key, value)` — a plugin's notes on the sale in progress (kept on hold / resume,
  cleared by New sale), handed to `tillAfterSale` as `ctx.meta` with `ctx.customerId`.
