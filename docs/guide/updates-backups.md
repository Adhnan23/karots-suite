# Updates & backups

![Data page](../images/install/data.jpg)

Everything here is on **Setup → Data** (admins only).

## Updates

Shops **never update by themselves**. When you **Release** a new build from the Shop Builder, it
is signed with your release key and put in the shop's folder in the Karots bucket. At the shop:

1. **Setup → Data → Updates** shows the version running and whether a newer one is published.
2. Press **Update**. The suite downloads the update, checks its signature, backs the shop up
   (`…-before-update-<version>`), swaps the program and restarts. A power cut at any moment leaves
   a program that starts: the old one or the new one.
3. If the new version fails to start, it rolls back to the old one by itself, and the old one keeps
   selling on the updated data.

**If an update fails after it changed the database:** nothing is lost. An update only ever adds to the database
(new columns and tables), so the old version still reads and sells on it. If the new version doesn't stay up for 2
minutes, twice, the PC goes back to the old one by itself and **Setup → Data → Updates** says so. The shop keeps
selling; call us and we fix it in the next update. The backup made just before the update
(`…-before-update-<version>`) is there too, but it is rarely needed.

> Keep a copy of `SUITE_RELEASE_KEY`. Shops only accept updates signed with it.

A changed **profile** (plugins, ports, database mode) needs the Builder and **karots-setup →
Update**, not just this button. See [Install](install.md#changing-a-shop-later).

## Backups

- **Automatic backups** run on the main PC every 6 hours, and the newest 28 are kept (about a
  week; your installer can change both). In file mode they're in `data/backups`; in server mode,
  in the install folder's `backups`. A backup is one moment: one taken while the tills sell
  restores whole.
- **Safety copies** — taken before a restore, an update or a mode change, and backups brought
  back from Karots cloud — don't count towards the newest kept; each stays 30 days.
- **Download a backup** any time:
  - **Everything:** the whole shop. Use this to move to a new computer.
  - **Choose what to include:** leave out some plugins' records.
  - **Catalogue & setup only:** products, contacts, users and settings, with no sales or money
    and balances at zero. Use it to start a new shop from an old one.
- **Restore** a backup file. The suite holds the tills still for a moment, saves a safety copy
  and brings the backup back in one go, so nothing sold at that moment is lost. The report lists
  what came back and anything odd it found in the data.

## Backup copies

Every automatic backup can also be copied away from the PC:

| | Copy folder | Karots cloud |
|---|---|---|
| Where | A USB stick, another drive or a network share | Karots' own storage, one folder per shop |
| Who turns it on | The shop's admin, on the Data page (or preset in the Builder) | Karots, for paying shops |
| Files | Plain backup files, so the owner can bring the shop back with only the file | Encrypted per shop, about one a day |
| Switch off | The admin | The admin can switch it off; only Karots can switch it on |

- **A backup on the PC's own disk dies with that disk.** Keep the copy folder on a USB stick or
  another PC, and take the stick home now and then.
- **Backups on the PC and in the copy folder are plain files, not encrypted.** Keep the stick
  somewhere safe: it holds your sales, customers and staff.
- **Karots cloud comes with the service plan** (and your first 3 months). When the plan ends, new
  cloud copies stop; the last one is kept 30 days, and Karots brings it back if you ask in that
  time. The backups on your PC and in the copy folder carry on.

A copy that hasn't worked for a day shows on the dashboard. **Backups here…** on each copy lists
its files, and **Restore** brings one back through the normal restore.

## Bringing a shop back

On a new PC, run **karots-setup** with the shop's folder and a backup. Setup offers it
under **Bring back a shop**. See [Moving a shop](install.md#moving-a-shop-to-another-pc).

## Spreadsheets

**Setup → Data → Spreadsheets** moves lists in and out as CSV, Excel or ODS files. Products, customers and suppliers
have their sheets on their own pages (linked from here); two live on this card:

- **Settings:** copies one shop's receipt, tax and till rules to another. The shop's identity, printers and default
  locker stay as they are.
- **Cashier menu:** the till's whole menu (Setup → Cashier menu) as one sheet, so a menu can be built or reworked in a
  spreadsheet — by hand or by AI — instead of tapped together card by card. The same sheet is under **Sheets ▾** on Setup → Cashier menu
  itself (admins).

### The cashier menu sheet

| Group | Emoji | Active | Product | Barcode | Product emoji |
|---|---|---|---|---|---|
| Drinks | 🥤 | | | | |
| Drinks > Soft drinks | | | Coca Cola 1050ml | 4791234500017 | 🥤 |
| Drinks > Soft drinks | | | 7 Up 1L | | |
| Specials | ⭐ | no | | | |

- **The order of the lines is the order at the till.** A line without a product sets up a group (its emoji, and
  `no` under Active to hide it); a line with a product puts that product in the group. Groups can sit inside groups
  (`Parent > Child`), and a group named only on product lines is made anyway.
- **A product is found by its barcode, then by its exact name** (capitals and extra spaces don't matter) — never by
  the shop's own numbers, so a menu made in one shop imports into another that has the same products. A product the
  shop hasn't got, or a name two products share, is skipped and listed. Products are never made from this sheet.
- **Importing replaces the whole menu.** Before anything changes you see what the sheet will do — how many groups
  and items, what goes from today's menu, which lines are skipped — and choose **Replace the menu** or **Cancel**.
  It's all or nothing, and products, stock and sales don't change. A sheet with no menu lines is refused.

**Setting up a new shop:** bring the products in first (Products → Sheets), then the cashier menu. A menu prepared on
another copy of the shop (with the same products) fits as long as names or barcodes match.
