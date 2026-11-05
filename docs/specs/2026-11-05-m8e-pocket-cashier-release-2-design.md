# Pocket Cashier release 2: design

Status: written from the user's answers on 2026-11-05. Lines marked **(my call)** were not the user's words; everything
else is the user's decision. Release 1 is what `docs/guide/plugins/pocketcashier.md` describes.

## Why

The phone exists so that a shop **never goes back to pen and paper**. The PC or laptop stays the normal till. A phone is
for two moments only: being **portable while online** (walking with a customer, which doesn't happen often) and being
**fully offline** (the power is out). Between those it sits on a charger with the app open, syncing with nobody signed in.

Pocket Cashier doesn't ship to shops until this release is done.

## How it feels

It works like a synced database. Online, the phone's data is current. Offline, it has the same data as of the last sync.
**The screens and features are the same either way**; only the freshness differs, and the phone shows how old its data is
(as it does today). Everything a cashier does is saved on the phone, then sent to the PC at once or when the PC is back.

**Cashiers and managers only.** Sales people stay off the phone for good.

## Scope

**Coming to the phone, working online and offline:**

| Part | Notes |
|---|---|
| Credit | Take a payment or advance, or refund one |
| Returns | Find a sale from the last 30 days, take items back, refund |
| Receipts | See the shift's slips; printing needs the PC |
| Warranty | Find a unit by serial or receipt (last 30 days) and swap it |
| Write-off, Convert | Stock changes |
| Reload & Bills | Till menu, line hints, count at close, return field |
| Print & Copy | Till menu and price quotes |
| Combos | Till menu, cart suggestions, line hints |
| Pharmacy | Patient panel, line hints; a dose label needs the PC |
| Repairs | Repairs panel, line hints; a job slip needs the PC |
| Alternatives | Swap on a cart line |

**Coming to the phone, online only:** tickets from a sales person. Offline there are no sales people, so nothing is needed.

**Staying on the PC:** Labels, Suppliers, Expenses, Low stock, the Customer requests plugin.

## What the phone holds

- **Sales and slips of the last 30 days**, encrypted like the rest of the copy.
- **Everything needed to sell, whatever its age:** all products, categories, prices, lots, customers and balances, plus
  the plugin data the parts above need (patients, scripts, repair jobs, floats, price grids, combos, alternative groups).
- **(my call) Left out:** what a cashier doesn't need or may not see, such as profit, supplier bills and reports.
- **Older than 30 days:** online, the phone asks the PC; offline, the customer comes back later.

## Sync

Today any change makes the phone download the whole copy again. With a month of sales that is too heavy, so **sync has to
send only what changed**. New sales arrive one by one. The method is left to the builder; `plugins/pocketcashier/copy.go`,
`feed.go` and `web/src/plugins/pocketcashier/sync.ts` are the current code. Sign-in-free syncing and the encrypted store stay.

**First step of the build: measure.** Build the 30-day copy from a full Demo Room year and try it on a low-end phone, for
size, first download and speed of search. If it's too much, come back to the user before going further.

## Clashes: the PC settles them

There is **no phone-to-phone sync**. During a cut, two phones don't see each other's work until the PC is back.

The existing rule holds: **nothing is refused, it is flagged**, and flags appear under **Phones → What came in**.

- Credit paid against an out-of-date balance, or paid twice: recorded and flagged.
- A recharge the float couldn't cover: recorded and flagged.
- **(my call)** The same item returned or swapped twice: the money that really left the drawer is always recorded, stock
  isn't counted back twice, and it's flagged for an admin.
- **(my call)** Each new action kind goes through the same one-time ref and in-order replay as sales
  (`plugins/pocketcashier/replay.go`), so a repeated send is harmless.

## Plugins: the rule to keep

`AGENTS.md`: **Pocket Cashier adapts from its own side.** Changes go in Pocket Cashier, never in the plugin, and core
doesn't change for the phone. A plugin not compiled in must simply not appear, with no "is plugin X on?" branches.

What the builder has to solve, and why it's the hard part:

- The plugins' till panels call their own routes (`/x/<key>/till` and others) with the signed-in person's session, but
  syncing happens with nobody signed in. Pocket Cashier needs a way to hold what those panels read.
- Their writes (a new patient, a repair job, the after-sale calls of Combos, Print & Copy, Pharmacy and Repairs) have to
  queue and replay in order.
- **(my call)** Something created offline gets a temporary reference on the phone and its real id or number when the PC
  takes it. Later actions that used the temporary reference follow it.
- The panels were built for the desktop till and may not suit a phone.

**The user's decision (2026-11-05): a small generic seam is allowed.** A plugin may declare what it needs offline through
one generic hook, in the spirit of the `offline: true` flag on `slots.tillTenders`. The plugin's own logic doesn't change,
it never mentions Pocket Cashier, and any future plugin can use the same hook. Everything else stays on Pocket Cashier's
side. Update the rule in `AGENTS.md` to say this.

## Printing

Offline, nothing prints: receipts, dose labels, job slips. They can be printed from the PC afterwards. Online, slips print
through the server as they do today. The phone never opens the cash drawer.

## Rights

**(my call)** A person sees on the phone what their user rights give them on the PC's counter tabs, online and offline.

## Keeping the phone second

**Anti-goal: a phone that feels like the shop's main till.** It works fully when it's needed and never invites people to
use it otherwise.

- **(my call)** No prompts, banners or dashboard hints suggesting the phone. Setup stays under **Apps → Phones**.
- **(my call)** The guide describes it as the power-cut backup that also goes portable.
- **(my call)** Fix the open item from release 1: a cashier who moves back to the PC while the phone still shows them gets
  claimed back by the phone's next send. The PC should win.

## Not in this release

Phone-to-phone sync. Printing without the PC. A tablet layout. Changes to the PC till's layout (the user asked about using
it on an 11-inch tablet only as a question).

## Build order

1. The new sync, with the last 30 days on the phone (starting with the measurement).
2. Tickets, Credit, Returns, Receipts, Warranty.
3. Write-off, Convert.
4. The plugins.

## Done means

- A shop with every selling plugin runs a whole day on a phone with the PC off, then the PC takes it all in order, with
  stock, money and balances agreeing and every clash flagged.
- Tests follow the release 1 pattern (replay rules against the database, Vitest for the phone's logic, Playwright for flows).
- **Docs and legal:** `docs/guide/plugins/pocketcashier.md` + `make docs-shots`. A month of sales and patient details on a
  phone is a new legal question: add it to `docs/legal/README.md` and `lawyer-items.json`, then `make agreement`.
