# Repairs (REP)

Code: `plugins/repairs` (store.go: jobs, parts, labour, deposits, payouts; pickup.go: the sale hook; api.go /
api_money.go: the routes, the bill, deposits, re-repairs, held deposit refund, paying the repairer; store_list.go:
lists, suggestions, the report; calc.go: estimate, warranty, due), `web/src/plugins/repairs` (the till's 🔧 panel, Put
on the bill, the job page, labels). Writes only `repair_*`; its money is core's: a deposit is a core customer
payment (the customer's advance), pickup is the normal till and Pay, the outside repairer is a core expense.
Tables: `repair_jobs`, `repair_parts`, `repair_charges` (labour), `repair_deposits` (job ↔ core customer payment),
`repair_payouts` (job ↔ core expense), `repair_payments` (old held deposits: deposit / refund / applied),
`repair_config` (labour product, warranty mode and default, ticket counter), `repair_pickup_lines` (job ↔ the
sale lines that collected it).

## The words the rules use

- **Open** — received, in progress or ready. **Closed** — collected or cancelled; a closed job never changes.
- **Pickup** — the sale whose lines are tagged with the job (Put on the bill); it marks the job collected.
- **Deposit** — a core customer payment linked to the job: the customer's advance, spent like any advance.
- **Old held deposit** — cash a job took as its own tender before deposits were advances: deposits − refunds −
  applied, never below 0.
- **Payout** — a core expense paying the outside repairer, linked to the job.

## The grid

Columns: **Do** · **Till** (🔧 panel, Put on the bill) · **Sale** (core money / stock) · **Return** · **Report** ·
**Merge** · **Faults**.

| Feature | Do | Till | Sale | Return | Report | Merge | Faults |
|---|---|---|---|---|---|---|---|
| Open / edit a job, status, urgent, cancel | REP-1 | REP-1 | — a | — a | — a | — a | PLG-3,4,5,6 |
| Parts and labour on an open job | REP-2 | REP-3 | REP-4 | — a | REP-8 | REP-10 | PLG-3,4,5 |
| Put on the bill → pickup sale | REP-3 | REP-3 | REP-4, REP-5 | REP-6 | REP-8 | — a | PLG-4,6 |
| Deposit (link a customer payment) | REP-7 | REP-3 | REP-4 | — a | — a | — a | PLG-3,5,6 |
| Old held deposit: spend at pickup / hand back | REP-7 | REP-3 | REP-4 | — a | — a | — a | PLG-4,6 |
| Pay the repairer | REP-9 | — a | REP-9 | — a | REP-8 | — a | PLG-4,5 |
| Warranty re-repair (free) | REP-5 | REP-3 | REP-4 | REP-6 | REP-8 | — a | PLG-5 |
| Settings (warranty mode, default days) | REP-5 | — a | — a | — a | — a | — a | PLG-5 |

a — not touched.

## Rules

```
REP-1  A job belongs to one core customer (its name and phone copied as they were). Its ticket number and
       code are unique and follow the shop's counter. Only an open job changes: a status, an edit, a part,
       labour or a cancel on a closed job is refused, also when it lands at the same moment as its pickup
       (a collected job is never cancelled or set back to ready).
       Checked by: example, faults (at once), ledgercheck (closed ⇔ its time stamped).

REP-2  A part is a stocked product (never a service), qty above 0, warranty 0–3650 days; labour is an amount
       ≥ 0 on the labour product. Removing a part or labour names one of this job's own, else it is refused.
       Checked by: example, refusals, ledgercheck (no part names a service).

REP-3  Put on the bill adds the job's parts at today's prices (an old row's discount kept), its labour as
       labour-product lines, every line tagged with the job, and pre-fills On account with the job's
       deposits — never more than the customer's advance now nor the estimate. A free re-repair's lines are
       priced 0. The till's prices are trusted like any line.
       Checked by: example.

REP-4  A pickup is an ordinary sale: core's sale, stock, payment and customer rules hold and the auditor
       passes. A deposit spent at pickup is the customer's advance going on the sale (core); an old held
       deposit is spent as its own tender, never more than is held.
       Checked by: example, ledgercheck (core's checks after every battle test; old held ≥ 0).

REP-5  The pickup sale marks each tagged job collected, in its transaction, once: it links the sale, stamps
       the time and starts the warranty (by days = the job's days; by part = the latest part's; a free
       re-repair has none of its own — it runs on the original's). A job already closed, or gone, is left as
       it is and the sale stands (never stop a sale). A re-repair is opened only from a collected job still
       in warranty.
       Checked by: example, faults (sent twice, at once), ledgercheck (collected ⇔ a sale that exists).

REP-6  A return of pickup lines is core's: the job stays collected and its warranty stands; the report counts
       what was kept.
       Checked by: example.

REP-7  A deposit is a core customer payment of the job's own customer, on one job only. An old held deposit
       is handed back from the drawer only once the job is closed, all of what is held, once (two at once =
       one refund).
       Checked by: example, refusals, faults (at once), ledgercheck (a deposit's payment is the job's
       customer's; old held never below 0).

REP-8  The report is one row per job collected in the range: revenue and cost are the pickup sale's lines
       tagged with that job (kept qty, bill discount shared as core's reports share it), the repairer's pay is
       its payouts. Two jobs collected on one sale never count each other's lines.
       Checked by: example, ledgercheck (a pickup line belongs to the job's sale).

REP-9  Paying the repairer is one change: a core expense (Repairs, the labour service) out of a cash place
       or by cheques, and its link to the job. Never for a cancelled job; an amount above 0; a voided expense
       drops out of what was paid.
       Checked by: example, refusals, faults (the failure sweep).

REP-10 A product merge moves the job's parts and the labour product to the kept product (PLG-2).
       Checked by: example, ledgercheck (no row names a merged-away product).
```
