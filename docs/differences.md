# Different on purpose — karots-suite vs karots-pos

Everything below is a deliberate change, gathered from each milestone's plan ("Upgrades over
karots-pos", "Done:" notes) and your decisions. While you compare the two apps (M6.5,
`docs/acceptance.md`), use it to tell a deliberate change from a miss. For each row, write in
**Your call**: **keep**, **change** (say how) or **add back**.

Not listed: the **new look** (the dock instead of a sidebar, the Studio and Night palettes, the
till as desktop-only and the admin working on a phone) and the **karots-pos bugs fixed in both
apps** (status.md lists them with their commits). After those fixes both apps should read the
same figures, so a figure that differs and isn't below is a bug.

## Till and counter

| # | The suite | karots-pos | Your call |
|---|---|---|---|
| 1 | Resuming a held sale is refused while another sale is in the cart. | Silently replaced the live cart, so that sale was lost. | |

## Catalogue and stock

| # | The suite | karots-pos | Your call |
|---|---|---|---|
| 2 | Cashier menu cards are deleted outright (Delete); there is no disable / re-enable. | A card could be deactivated and reactivated. | |
| 3 | The stock batch sheet is titled "Batches of …", has **Batch price** (blank = follows the product) and **Set count** / **Remove batch** per batch; the whole-product "Adjust the count" is not on that sheet. | One adjust for the whole product. | |
| 4 | A product sheet import changes only the columns the sheet fills, so a name + price sheet leaves cost, category and supplier alone. A bad amount fails its row. *(Fixed in karots-pos too, `5305a0f`.)* | Unfilled columns were wiped, and "1,250.00" imported as 0. | |

## Purchasing and people

| # | The suite | karots-pos | Your call |
|---|---|---|---|
| 5 | A purchase return is made **against one received delivery**: only its lines, at most what's left of its lot, priced at the lot's real cost, and credited to that bill first. Stock that came on no delivery goes back as "old stock" at the lot's recorded cost. | Any product to any supplier at a typed cost. | |
| 6 | Purchase / delivery lines and return quantities have touch steppers (− qty +). | Typed quantities. | |
| 7 | A customer import matches the phone whatever its spacing, and a customer with no phone by name, so re-importing never duplicates anyone. *(Fixed in karots-pos too, `776fd8e`.)* | "077-123 4567" and phoneless rows came in as new customers each time. | |

## Money

| # | The suite | karots-pos | Your call |
|---|---|---|---|
| 8 | A locker is made active only by Archive / Bring back, and an archived locker can't be the till default. | Active was a box on the edit form. | |
| 9 | A paid expense's amount and source are fixed; **Void** puts the money back with its own CR- receipt. A plugin-owned expense can't be voided from Expenses. *(Void is in karots-pos too since `d66ac18`.)* | Delete / edit left the money out. | |
| 10 | Expenses default to this month and page 50 at a time; till sessions and the cash-flow ledger page too. | Whole lists (or the newest 500 rows). | |
| 11 | The Sales list shows cost, profit and margin per sale and in its totals (admins and managers). | The Sales list showed no profit. | |

## Reports

| # | The suite | karots-pos | Your call |
|---|---|---|---|
| 12 | **Cash register report = the Till sessions page** (totals and export added there). | A separate Cash Register Report. | |
| 13 | **Low stock = the Reorder page** (with Print → `/print/low-stock`). | A separate Low Stock report. | |
| 14 | **Batches / expiry = the Batches page** (export added there); **Finance report = the Finance page** (export added). | Separate report pages. | |
| 15 | **Audit log = Activity's "Changes" source**, keyset-paged with no cap. | A separate Audit Log page capped at 500 rows. | |
| 16 | The **Returns report lists by return date** (a refund today on last month's sale shows today), refund = what was paid. The P&L still nets returns into the sale's own period. | Filtered by the sale's date. | |
| 17 | Top products: the Pareto and the shares are of **every product sold**, not just the ranked rows. | Of the 50 rows shown. | |
| 18 | Inventory valuation rows are valued **by lots**, so they add up to the headline. | Each row = qty × today's cost price. | |
| 19 | Losses & Recovery lists **damage and warranty losses** in the range, paged. | Damage only, all time, newest 200. | |
| 20 | Report filters live in the URL; there are **no redirects from karots-pos's old report URLs** (a new product has no old bookmarks). | — | |

## Data and settings

| # | The suite | karots-pos | Your call |
|---|---|---|---|
| 21 | Restore first **saves a copy of the current data** (the undo, listed as "Before restore"), refuses a bad file before touching anything, needs RESTORE typed, and then signs you out. Any session whose account the backup changed (another person on that id, another role) ends too — as it does when an admin changes someone's role. | Restored straight over the data with no copy; every session carried on, even as whoever now had its id. | |
| 22 | A backup from another version restores: columns the suite lacks are dropped and reported, and tables it lacks (plugins) are left out and listed. | Any schema difference failed the whole restore. | |
| 23 | A **data check** lists figures old versions saved wrongly (Setup → Data and after every restore). It changes nothing. | — | |
| 24 | Settings save **card by card**; the printer field takes a detected printer or a typed `tcp://` address in one box; a failed test print says why. | One long form and one Save; a separate network-address box. | |
| 25 | The **setup checklist** is a "Get the shop ready" card on the dashboard (admins, until done). | On the Setup hub page. | |
| 26 | The settings sheet import says which keys it kept as this shop's own and which aren't settings. | Reported only how many it imported. | |
| 27 | Self-update is **not in the suite yet**. It comes with the release tooling in M8. | Setup → Data update card, badge and banner. | |

## Plugins (M7)

| # | The suite | karots-pos | Your call |
|---|---|---|---|
| 28 | **Clearance:** an approved markdown **ends by itself** once the stock it was approved for has sold out or a fresh batch comes in; the product shows again if it's still slow. *(Your decision, 2026-10-11.)* | A markdown stayed on forever, so fresh stock sold at the clearance price. | keep |
| 29 | **Clearance:** Dismiss is **Snooze** — the product is hidden for one "slow after" period (60 days by default) and comes back if it still isn't selling; **Bring back** undoes it. *(Your decision.)* | Dismissed products never came back. | keep |
| 30 | **Clearance:** dead stock has **Disable all shown** behind a confirm. *(Your decision.)* | One Disable click per product. | keep |
| 31 | **Clearance:** when a batch is about to expire, the safe markdown is worked out from **that batch's own cost**. *(Your decision.)* | From the product's latest cost, which often allowed too little to sell it in time. | keep |
| 32 | **Clearance:** the suggested % is cut to one decimal (17.8%, never rounded up past the safe floor); a batch expiring **today** still counts as expiring; a running markdown stays on the list after it starts selling. | 17.826…%; a batch expiring today dropped off; a selling markdown vanished from the page. | |
| 33 | **Building a shop's binary:** the **Shop Builder** opens in the browser (tick plugins, OS, processor, binary name, keep or mint the install id, live build log, support PIN at the end) and runs on Windows, macOS and Linux with only Go and bun. *(Your request, 2026-10-11.)* | A terminal checkbox menu that needed make and bash. | keep |
| 34 | **Customer requests:** a second customer asking for the same thing is **Asked again** on the existing request (the till shows matches while typing); the list sorts by how many asked, and two requests can be merged. *(Your decision, 2026-10-11.)* | Every ask was a separate note; nothing counted demand. | keep |
| 35 | **Customer requests:** a request closes three ways — **Make product** (now with cost and supplier), **Link to an existing product**, or **Won't stock** with a reason the till shows the next asker; **Reopen** undoes it. *(Your decision.)* | Only Make product or delete. | keep |
| 36 | **Customer requests:** once its product has stock, a request moves to **To call** with each customer's phone as a call link and **Called ✓**; with no stock yet it shows "Waiting for stock". *(Your decision.)* | The phone was saved and never used. | keep |
| 37 | **Customer requests:** **Clear done** removes requests made into products with nobody left to call; won't-stock ones stay. | Clear converted removed every converted note. | |
| 38 | **Till:** when a search finds nothing, Quick item also offers **Not stocked? Note a request** with the search text (a generic till seam any plugin action can use). *(Your decision.)* | The request button was only in the action bar. | keep |
