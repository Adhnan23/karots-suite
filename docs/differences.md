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
| 39 | **Product Plus:** a field can be limited to chosen **categories** (their subcategories included); it shows only for those products on every surface — form, intake, search, list line, till ⓘ, label, Fill in, sheets. A product moved out keeps its values, hidden until it moves back. *(Your decision, 2026-10-11.)* | Every field applied to every product. | keep |
| 40 | **Product Plus:** **Fill in** sets one or more fields across many products in a grid (category, Missing only, search); a bad cell is marked with its reason and the rest still save. *(Your decision.)* | One product at a time on its form. | keep |
| 41 | **Product Plus:** **required** no longer needs a default; the field list shows "N missing", which opens Fill in on those products. The product form and intake still refuse a blank required field. *(Your decision.)* | A required field had to have a default, so nothing was ever really missing. | keep |
| 42 | **Product Plus:** changing a default asks **Only new products** (preselected — existing ones keep what they showed) or **Change existing products too**; a value someone chose stays even when it equals the default; a type change that would lose values says how many and needs a tick; search and the list line see the default too. *(Your decision.)* | A default change silently moved every product, a chosen value equal to the default was dropped, and a type change lost values without a word. | keep |
| 43 | **Products → Sheets ▾:** **Include custom fields** adds one column per field to export, template and import (off = today's sheet); a bad custom cell skips only that value with a note. *(Your decision.)* | Custom fields never left the app. | keep |
| 44 | **Products → Sheets ▾:** **Skip disabled** (on by default); off exports disabled products too with an `active` column (yes / no), and import honours it. *(Your decision, 2026-10-11.)* | Disabled products never exported. | keep |
| 45 | **Product Plus:** the three show switches are one **Where it shows** chip row (Till ⓘ · Label · Stock intake). | Three separate switches. | |
| 46 | **Product Plus:** a field can't take a label the products sheet already uses (Name, Barcode, Price, SKU, Qty, active…) or another field's label, so sheet columns never clash. *(Your request, 2026-10-11.)* | Any label went; a field called "Price" would have been read as the selling price on import. | keep |
| 47 | **Combos:** a third kind, **Combo price** (a set costs Rs X), beside % off and Rs off; the discount is worked out from the line's real price (wholesale on a wholesale sale) and adds up **to the cent**, splitting a line when only some units take the last cent. *(Your decision, 2026-10-11.)* | % or Rs off only; Rs off became a rounded percent of the **retail** total, so a wholesale sale got the wrong discount (fixed in karots-pos too). | keep |
| 48 | **Combos:** a combo **stays together in the cart** under one header (− / + sets, saving, ×); removing one item asks "Break the combo?" and the rest go back to full price; the same combo again adds sets. *(Your decision.)* | Loose discounted lines: removing one kept the discount on the rest. | keep |
| 49 | **Combos:** the till **spots combos**: full sets scanned one by one show "🎁 Milk + Cookies ×2 — save Rs 80 · Apply"; part of a combo shows a chip on the line ("add Cookies · save Rs 40") that completes it. *(Your decision.)* | Only the 🎁 menu; scanning the items charged full price. | keep |
| 50 | **Combos:** optional **Starts / Ends** dates; outside them a combo isn't offered, hinted or badged (admin shows Scheduled / Ended). *(Your decision.)* | On until disabled by hand. | keep |
| 51 | **Combos:** **health** — a disabled member blocks the combo (till Add off, red flag in admin); short stock is a yellow note; new members must be active products, not services. *(Your decision.)* | A combo with a disabled member was still offered. | keep |
| 52 | **Combos:** each card shows **Sold N× · Rs X · last sold …**, and **Reports → Combos** shows sets, sales, discount given and bills per combo for a period, with CSV. Counts from the suite only; returns aren't netted. *(Your decision.)* | No sales figures for a combo. | keep |
| 53 | **Receipts:** a combo prints as **one block** — "Milk + Cookies (combo)", sets × set price → combo price, You saved, Line total, then its items without prices; the A4 receipt and sale views group the same way; Returns tags each item with its combo. *(Your decision, 2026-10-11.)* | One discounted line per item. | keep |
