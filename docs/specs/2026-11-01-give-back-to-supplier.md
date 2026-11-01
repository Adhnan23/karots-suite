# A3: give a lot back to its supplier

Item A3 of `docs/status.md`, for the first shop (a pharmacy). All core. Delete this spec with its plan when done.

## Why

The warning list (A1) tells the owner which lots are close to expiry and whose they are. After that the system can't
follow what happens: the owner tells the supplier, a rep comes some days later and takes what's left, and the supplier
makes it good later still. Today that lives in the owner's head, and only expired lots show on "From this supplier".

## What the user decided

- A lot goes back for any reason: the supplier wants it, or the shop wants to give it (close to expiry, expired, damaged).
- **Count first.** Batches get mixed on the shelf, so the system's number for a lot can't be trusted. The count wins.
- Differences never stop anything; the stock is adjusted, as at the till when something sells at zero.
- Settling works as damage and expiry do today: replacement, money (whole or part), or nothing.
- For the differences, the user left the choice to me (see the rule below).

From the pharmacist (`docs/status.md`): telling the supplier late is a warning, never a block; the rep may come at any
time, before or after expiry; the claim is what's left when they take it.

## Goals

1. Show lots on the warning list on "From this supplier", as well as expired ones.
2. Record that the supplier was told, on a lot, with the date.
3. **Give back:** count a lot, hand it over, and have it wait on that supplier until settled.
4. Settle it with the existing Recover form.
5. Reach all this from the supplier page and from the Batches page, where the owner sees the warning.

## Not in this work

- Recall of a whole lot, holding it from sale, who bought it (A4).
- Replacing with a different product (separate item in A).
- Any change to Purchasing → Returns, which stays for returning against a delivery's bill.
- Reminders, or anything that blocks. Nothing here may ever stop a sale or a hand-over.
- Plugins, Pocket Cashier.

## How it behaves

### The panel

`SupplierClaimsPanel` ("From this supplier") gets a third group, **Close to expiry**: lots of this supplier for which
`stock.Batch.ExpiringSoon` is true. Expired lots stay in their own group. The panel also appears on the admin supplier
page (`SupplierPage.tsx`), which doesn't have it today. It keeps its place on purchases and the counter's Suppliers tab.

Each close-to-expiry and expired lot has two actions:
- **Told them:** records today's date on the lot (`stock_batches.supplier_told_on`, new nullable date). The row then
  says "told 3 Nov". Pressing it again moves the date to today. It changes nothing else.
- **Give back:** opens the count sheet. On expired lots it replaces today's "Write off & settle" button: that action
  wrote off the system's number without a count, which the user's rule replaces.

### The Batches page

A lot with a supplier gets **Give back**, and shows "told 3 Nov" when set. A lot with no supplier has no button; it
can still be written off as today. Setting a preferred supplier on the product gives it one.

### The count sheet

Shows the product, the lot's expiry, and the system's number. One field: **Counted on the shelf**, starting empty so
it has to be typed. A short line says what the difference will do before saving. Saving hands over everything counted.

A count of zero is allowed when the system number is above zero: nothing is handed over, nothing waits on the
supplier, and the lot's whole number moves as a difference (the rule below).

Giving back part of a lot isn't a separate path here: the existing **Record write-off** covers a few damaged pieces.

### When the count differs

The product's total changes only by what was handed over, and differences move between lots, because batches being
mixed is the reason counts differ.

| Count against the system | What happens |
|---|---|
| **More** (12, system 10) | The extra 2 move into this lot from the product's other live lots, next-to-sell first. Whatever those lots can't cover is a plain stock adjustment up. |
| **Less** (7, system 10) | The 3 move out of this lot onto the product's next-to-sell other live lot. With no other live lot, they are a plain stock adjustment down. |

The lot always ends at zero, so it leaves the warning list. If some really were lost, the next stock-take catches it.
Every lot change is written to Movements with who did it and a note naming the hand-over, so the total's history adds up.

### The hand-over

In one transaction: settle the difference, then remove the counted amount through the existing damage path
(`consumeTx`, reason `damage`, naming the lot). This produces the same loss a write-off does, valued at this lot's cost,
so the existing reports, the waiting list and the Recover form all work unchanged.

The movement note says `given back to <supplier>`, and the waiting row shows it: "12 given back 3 Nov". A hand-over is
counted as a loss until it's settled, like any write-off today; the money or replacement then shows as recovered.

### Settling

The existing **Settle** button and `RecoverSheet`: replacement (new lot with its own expiry, price and extra), money
(whole or part, lowers what's owed), or nothing.

Unsettled write-offs drop off the waiting list after 180 days (`claimWindow`). A rep can take longer than that to
settle, so the waiting list must keep given-back lots until they're settled, with no time limit. Ordinary write-offs
keep the 180 days.

## Constraints

- Core only; `make check-core` stays green.
- Files stay at 500 lines or fewer.
- Every screen works at 360px, in every palette and both modes, using theme tokens.
- The next migration is `00020_`; update the list in `AGENTS.md`.
- The admin and counter routes both come from `registerSupplierClaims`, so the counter can do this too, as it can settle
  today.
- A follow-up question is a dialog, never a toast.
- Docs: `docs/guide/purchasing.md` and `docs/guide/stock.md`; retake the Batches and suppliers screenshots (the Batches
  one is also out of date since A1).

## My calls, for checking

- The Batches page and the supplier page both get the actions, so the owner never has to go looking.
- One count field, not separate "counted" and "handing over" fields. Part of a lot goes through Record write-off.
- "Told them" is one tap with today's date, not a date picker, and only the latest date is kept.
- "Rep took it, pending" isn't a separate status: a given-back lot waiting on the Recover form already means exactly that.
- Differences use the next-to-sell other lot, so any oddity clears soonest.
- Expired lots lose "Write off & settle" in favour of Give back; "Write off all" on the Batches page stays.

## Left to the builder

Names, the exact way a given-back movement is marked so the waiting list can keep it past 180 days, and the wording.
