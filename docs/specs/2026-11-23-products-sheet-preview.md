# Products sheet: preview before import

## Why

Fixing a shop's whole product list from a sheet (Iman: names, categories, generics, units by ref, 2026-11-23) must be
safe to do at the shop: see every change first, then apply it in one go. Today's import applies row by row at once and
only reports counts afterwards. The user (2026-11-23): "build the preview so it is just a change inside import
product" — no new page, no new button elsewhere; a wipe-and-reimport was considered and rejected (it loses barcodes,
packs, prices, suppliers and history).

## What changes (Products → Sheets ▾ → Import)

- Choosing a file runs a **preview**: the same import in a transaction that is rolled back. The import sheet shows
  the counts (to update · new · skipped), **each changed product with only its changed fields, old → new** (name,
  local name, barcode, extra barcodes, category path, unit, prices, tax, reorder level, supplier, warranty, serial,
  Active, and the requested plugin columns, e.g. Generic Name — only when that plugin is in the shop), the new
  products by name, and the skipped lines with their reasons. A search box filters the changes.
- **Import** then sends the very bytes that were previewed (a copy taken at preview) and applies them **all or
  nothing**: one transaction; a line that fails is skipped exactly as the preview said (its own savepoint), a
  database failure changes nothing. If Apply skips a line the preview didn't, the result says so.
- Everything else about the import stays: matching (ref, barcode, name), "Also match by name", Include ref / plugin
  columns, partial updates (only the columns a sheet fills change), opening_qty rules (STK-24), the report.
- Customers and suppliers sheets are unchanged (not asked for).

## Approaches

1. **Dry run of the real import + a field diff (picked).** Every write the import makes joins one transaction
   carried in the context (`db.InTx`), each row in its own savepoint; preview rolls the whole thing back. The diff
   reads each touched product before and after its row through the same transaction. Exact by construction.
2. Compute the changes from the sheet without writing. Would have to re-implement matching, partial updates,
   category / supplier creation and plugin validation, and could disagree with what Import does. Dropped.
3. Wipe products and re-import (the user's idea). Loses barcodes, packs, prices, suppliers, stock and sales; against
   the "never wipe and re-add" rule. Dropped.

## Must hold

- The preview changes nothing anywhere (every table, plugins' too) — a battle test compares the whole database.
- Category and supplier creation, barcodes and plugin columns all write through the import's transaction (today
  categories and suppliers commit on their own: fixed).
- Apply = the preview: same counts, same changes, same skipped lines.
- Admin / manager as today (the products sheet's own permission). Core names no plugin: plugin columns come through
  `plugin.ProductSheetColumns` (Export reads before / after).

## Rule book

A new `IMP` group in `docs/rules/stock.md` (the catalogue lives there): IMP-1 preview changes nothing; IMP-2 Apply =
preview; IMP-3 all or nothing (a failing line is skipped whole, a database failure changes nothing); IMP-4 only the
sheet's columns change, and the preview lists exactly those; IMP-5 Replace sends the previewed bytes.
