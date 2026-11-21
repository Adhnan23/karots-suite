# Pharmacy (PHA)

Code: `plugins/pharmacy` (store.go: patients; store_scripts.go: prescriptions, owed, suggest; store_dispense.go:
dispenses; store_controlled.go: marks, the controlled register; store_doses.go; store_log.go; attributes.go;
merge.go), `web/src/plugins/pharmacy` (the till's 💊 panel, after-sale dispense, labels). Reads Product Plus
(generic name, strength) and Alternatives (medicine groups); writes only `pharmacy_*`.
Tables: `pharmacy_patients` (a core customer + profile), `pharmacy_prescriptions` + `_items` (a drug line names a
product and / or a medicine group, dose, qty), `pharmacy_dispenses` + `_items` (what a sale handed a patient, per
script line), `pharmacy_controlled_products` (the controlled mark + schedule), `pharmacy_medicines` (Rx-only, usual
dose, note), `pharmacy_doses` (the dose list), `pharmacy_cd_ledger` (the old hand-kept register, read only).

## The words the rules use

- **Medicine** — a product in a medicine group, Rx-only or controlled.
- **Dispense** — one record per sale: which patient, which script, who prescribed; one row per medicine sold, tied
  to the script line it filled (or none = off-script), with the days it covers. Names are kept as they were.
- **Given / owed** — a script line's tablets handed over (dispense rows net of what came back on their sale) and
  what it still needs.
- **Register** — a controlled drug's in / out list read from core's stock movements, each sale naming its dispense's
  patient and prescriber; **unrecorded** = a controlled sale (net of returns, since it was marked) with no dispense.

## The grid

Columns: **Do** · **Till** (💊 panel, hints) · **Sale** (core money / stock) · **Return** · **Register** · **Merge** ·
**Faults**.

| Feature | Do | Till | Sale | Return | Register | Merge | Faults |
|---|---|---|---|---|---|---|---|
| Patient: register / from customer / edit | PHA-1 | PHA-1 | — a | — a | — a | — a | PLG-3,4,5,6 |
| Prescription: write / edit / stop | PHA-2 | PHA-3 | — a | — a | — a | PHA-10 | PLG-3,4,5 |
| Add a script line to the cart | PHA-3 | PHA-3 | PHA-4 | — a | — a | — a | — b |
| Dispense (with the sale / a past receipt) | PHA-5 | PHA-5 | PHA-4 | PHA-6 | PHA-7 | PHA-10 | PHA-5, PLG-4 |
| Mark controlled / Rx-only, usual dose | PHA-8 | PHA-3 | — a | — a | PHA-7 | PHA-10 | PLG-4,5 |
| Freeze a controlled drug | PHA-8 | — a | — a | — a | PHA-7 | — a | — b |
| Refill list, called | PHA-9 | — a | — a | PHA-6 | — a | — a | PLG-5 |
| Dose list | PHA-11 | PHA-3 | — a | — a | — a | — a | PLG-5 |

a — not touched. b — read-only.

## Rules

```
PHA-1  A patient is exactly one core customer (never two patients for one customer); registering a walk-in
       reuses the customer with that phone. The profile never changes the customer's name, phone or balance.
       Checked by: example, ledgercheck (one patient per customer).

PHA-2  A prescription belongs to one patient; each line names a product, a medicine group or free text, with
       a qty ≥ 0. Editing keeps each line's id (its hand-over history and refill stay); a line left out is
       removed. Stopped scripts never remind.
       Checked by: example, refusals.

PHA-3  The till's hints never stop a sale: Rx-only, controlled, allergy notes and "owed" are shown, not
       enforced. Add puts the line's own product while in stock, else its group's best in-stock member, else
       the product anyway.
       Checked by: example.

PHA-4  A medicine sale is an ordinary sale: core's sale, stock and payment rules hold, and the auditor passes.
       Checked by: example, ledgercheck (core's checks after every battle test).

PHA-5  One dispense per sale, never for a void or missing sale, for a script of that patient only. Its rows
       are the sale's medicines (net of what already came back): a tagged line fills its script line, an
       untagged one the first line of its group (or product), other medicines off-script. Sent twice or two
       at once = one record (the second is refused, naming the patient). A patient picked at the till is
       recorded with the sale (its transaction; skipped, never refused, when something's off — a sale is
       never stopped); a past receipt is recorded by hand. (User, 2026-11-21.) A dispense row never
       gives more of a product than its sale sold.
       Checked by: example, faults (at once), ledgercheck.

PHA-6  A return shrinks what a script line was given and the days it covers (read net of returns); the
       dispense record itself is never rewritten. A refill comes due from the last hand-over's days.
       Checked by: example.

PHA-7  A controlled drug's register is core's stock movements for it: every movement once, in order, and the
       last balance equals the stock; each sale row names its dispense's patient and prescriber as they were,
       and a controlled sale with no dispense is on the unrecorded list until one is recorded. A controlled
       drug can't be frozen (counted in full).
       Checked by: example, ledgercheck (unrecorded list = controlled sales with no dispense).

PHA-8  The marks are one row per product: controlled (with its schedule) is always Rx-only; unmarking takes the
       product off the register list but keeps its history.
       Checked by: example, ledgercheck.

PHA-9  The refill list is the active, repeat (or tracked) script lines whose last hand-over runs out within the
       window, minus those called in the last few days; Called names a line that exists.
       Checked by: example, refusals.

PHA-10 A product merge moves script lines, dispense rows and the old register to the kept product; the marks
       are one row per product, the kept one's wins (PLG-2).
       Checked by: example, ledgercheck (no row names a merged-away product).

PHA-11 The dose list has each dose once (case and spaces ignored), doses a day ≥ 0; a dose used on a script or
       as a usual dose joins it by itself; deleting one never changes a script.
       Checked by: example, refusals.

PHA-12 The data holds its own shape: dispense rows' qty above 0, a dispense row's script line belongs to the
       dispense's patient, one dispense per sale, one patient per customer (a unique key).
       Checked by: ledgercheck.
```
