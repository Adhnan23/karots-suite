# Pocket Cashier (PKT)

Code: `plugins/pocketcashier` (pair.go / api_phones.go: pairing, approve, block, release, remove; guard.go +
replay.go `signedIn`: one person, one device; copy.go / recent.go / parts.go: what the phone keeps; replay.go: the
actions endpoint, one result per ref; replay_sale.go: sale, open, close, cash in / out, hold, unhold; replay_call.go:
calls through the real routes, temp numbers, refused cash; lookalike.go; review.go: What came in),
`web/src/plugins/pocketcashier` (sync.ts: the outbox sent in order; store/: the encrypted copy and outbox;
offline/: the screens' calls answered from the copy and saved as `call` actions; sale.ts: the till's cart maths).
Writes only `pocket_*`; every sale, drawer move, payment and plugin record is made by core's services or a real
route, as the person who did it on the phone.
Tables: `pocket_ca`, `pocket_phones`, `pocket_sends`, `pocket_actions`.

## The words the rules use

- **Action** — one thing done on the phone (sale, open, close, withdraw, deposit, hold, unhold, call), with a
  one-time **ref**, the person and the time it happened. Online it is sent at once; offline it waits in the
  **outbox** and goes, oldest first, when the PC answers.
- **Call** — an action that runs a real route (credit, return, warranty, write-off, convert, a plugin's write) as
  that person. Only routes on `call_routes.json`.
- **Watched** — the screen waits for this very answer (online): a refusal is shown and no money moved. **Handed** —
  the cashier went ahead (offline, or the answer was lost): the cash already moved.
- **Temp number** — what the phone calls something it made offline (`temp:<ref>`, or a number 9e15 up) until the PC
  gives the real id.
- **The PC's own trouble** — a 5xx (a restart, a database blip): the action waits and is tried again, never refused
  for it until 20 tries.

## The grid

Columns: **Do** · **Arrives** (replayed on the PC) · **Twice** (sent again, at once, after a lost reply, a power
cut) · **PC knew better** (price, stock, limit, till closed, person off, already done on the PC) · **Cash** (what
moved in the drawer when the PC refuses) · **Review** (What came in, flags) · **Faults**.

| Feature | Do | Arrives | Twice | PC knew better | Cash | Review | Faults |
|---|---|---|---|---|---|---|---|
| Pair, approve, block, release, remove | PKT-1 | — a | PKT-1 | — a | — a | — a | PLG-5 |
| One person, one device | PKT-2 | PKT-2 | — b | PKT-2 | — a | — a | — b |
| The copy, recent sales, parts | PKT-3 | — a | — b | — a | — a | — a | PKT-3 |
| Sale | PKT-6 | PKT-5,6 | PKT-4 | PKT-6 | PKT-7 | PKT-12,13 | PKT-4,5 |
| Open / close the drawer | PKT-10 | PKT-5,10 | PKT-4 | PKT-10 | PKT-7 | PKT-12 | PKT-5 |
| Cash out / cash in | PKT-10 | PKT-5,10 | PKT-4 | PKT-10 | PKT-7 | PKT-12 | PKT-5 |
| Hold / unhold | PKT-11 | PKT-5,11 | PKT-4 | PKT-11 | — a | PKT-12 | PKT-5 |
| Call (core counter, plugin writes) | PKT-9 | PKT-5,9 | PKT-4 | PKT-9 | PKT-7 | PKT-12 | PKT-4,5 |
| Temp numbers | PKT-8 | PKT-8 | PKT-4 | PKT-8 | PKT-7 | PKT-12 | PKT-8 |
| Phone's sums (total, change) | PKT-14 | PKT-14 | — b | PKT-6 | — a | — a | — b |
| Backup / restore | PLG-7 | — a | — a | — a | — a | — a | PKT-15 |

a — not touched. b — reads only, or the phone's own maths.

## Rules

```
PKT-1  Only an admin approves a phone (by its code or from the list); a phone's key works only while it is
       active — blocked is refused at once on every phone route, waiting is refused, unknown is refused. A
       phone waiting may ask again (its request is replaced); a paired or blocked phone pairs again only after
       Remove. Block and Remove free the person on it.
       Checked by: example, refusals.

PKT-2  One person, one device: whichever sign-in came last holds them. A send from a phone never takes a
       person back from the PC they moved to; work they did on the phone before still goes through. A phone not
       heard from for a minute keeps its person (they may be selling offline) until an admin releases it.
       An admin or the system user never holds a phone and never acts through one.
       Checked by: example.

PKT-3  Only an active phone gets the copy, recent sales and the plugins' parts; the same version answers 304;
       a part that fails fails the call (the phone keeps its last good parts).
       Checked by: example, refusals.

PKT-4  Every action is applied at most once, whatever the sends: twice, two at once, after a lost reply, or
       a power cut while it applies. A repeat gets the first result back and changes nothing. An action is
       noted as starting before it runs; a repeat that finds it started but never finished doesn't run again —
       it is flagged on What came in, "may not have gone through — check" (user, 2026-11-21: never twice). A
       credit payment carries the action's ref as its one-time reference (CUST-16), so it is always exact.
       Checked by: example, faults (at once, sent twice, a failure after the route answered), ledgercheck
       (one sale per sale action's ref).

PKT-5  Actions are applied in the order the phone made them. The PC's own trouble (a 5xx, a database
       failure) on any kind of action stops the batch there; it and what follows are tried again, and only
       after 20 tries does it count as refused. A refusal for a business reason never stops the batch.
       Checked by: example, faults (the failure sweep over every kind).

PKT-6  A phone sale is an ordinary sale: core's sale rules hold and the auditor passes. It keeps the price
       the phone showed, and is never refused for stock, serials, a credit limit, the clock, a person turned
       off or made a sales person meanwhile — it is flagged instead. It counts at the time it was rung when
       its till has been open since; otherwise now, in an open till (one opened for it, flagged, when there
       is none). A phone clock in the future counts now.
       Checked by: example, ledgercheck (core's), battle (a cut's sales replayed).

PKT-7  Cash that moved is booked exactly once. When the PC refuses something the cashier already handed
       over, the cash it moved (a sale's net cash, a call's cash in / out) is booked as a move into or out
       of that person's till (one opened for it when none), and the note says so; a refusal the cashier
       watched books nothing; a lost reply is booked by the retry that says it was handed, once. A refused
       sale is claimed before its cash is booked, so a stop between the booking and its record never books
       it again; a booking the PC's own trouble stopped is tried again, never given up (PKT-5).
       Found in: 2026-11-22 full run (a refused sale's record failing after its cash was booked booked it
       twice on the retry; a database blip while booking left it "couldn't be booked" for good).
       Checked by: example, failure sweep (a refused sale), ledgercheck (every handed, refused action with
       cash is settled; never more refused-on-phone moves than such actions).

PKT-8  A temp number becomes the real id of what this phone's earlier action made, in a path or a body; an
       action depending on one that was refused or never arrived is refused, naming it; a number nothing was
       given stays as it is. Never another phone's, never the wrong row.
       Checked by: example.

PKT-9  A call runs only a route on the list, as the person who did it, with that route's own checks and
       rights; the route decides, the same as at the counter. A call older than 5 minutes says when it
       happened; a balance the phone showed that the PC no longer had is flagged.
       Checked by: example, refusals.

PKT-10 Drawer actions go through core's drawer: open once (already open → skipped), at its own time when it
       can be; close the person's till at its own time when after the open and not in the future. A cash
       out / in already happened: it is booked as made, into or out of the locker it named, even past what
       the PC thinks the drawer holds, in a till opened for it when none; the till is flagged (DRAW-19, user,
       2026-11-21). Only a locker that no longer exists makes it a refused-on-phone cash move (PKT-7).
       Checked by: example.

PKT-11 A phone's held sale is one of the shop's held sales; resuming one already resumed elsewhere is
       skipped — one held cart never becomes two sales.
       Checked by: example.

PKT-12 What came in lists every send with its actions; a send's counts (actions, refused, flagged) agree
       with its rows; a flagged sale shows its flags.
       Checked by: example, ledgercheck (a send's counts).

PKT-13 A replayed sale with the same items and total as another within 10 minutes is flagged as a
       lookalike; both stand.
       Checked by: example.

PKT-14 The phone's total, change and refunds come from the till's own maths (the same functions), so the
       sale the PC books has the total the phone showed (TILL-1, TILL-2).
       Checked by: example (the till's cases through the phone's sale request).

PKT-15 A backup carries the shop's certificate and every phone; restored, the phones keep working (PLG-7).
       Checked by: example.
```

## Suspected (from reading, 2026-11-21)

- PKT-4: a call's route commits, then recording the action fails (database blip, power cut) → the phone sends it
  again and the call runs twice (a return refunded twice, a write-off twice). Sales are safe (`sales.client_ref`).
  The ponytail note in `replay.go` names it.
- PKT-5: only sales and calls wait on the PC's own trouble; open, close, cash out / in, hold and unhold turn into
  refusals for good on a database blip. Unhold calls *any* error "already resumed".
- PKT-7: a cash out / in refused by the PC (more than the PC thinks the drawer holds, the till closed meanwhile,
  untracked cash off) books nothing, though the cash moved. Cash out / in never opens a late till.
