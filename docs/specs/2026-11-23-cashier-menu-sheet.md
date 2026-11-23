# Cashier menu sheet

## Why

Shops should get a fresh look without the tedious work of building the cashier menu (Admin → Groups) one tap at a
time. Two uses, both the user's (2026-11-23):

1. **Ready-made menus for new shops** — a pharmacy, a grocery, a bakery start from a good menu, not an empty one.
2. **Bulk edits with AI** — export the menu, let AI regroup / rename / add emoji / reorder it, import it back.

## Goals

- Export the whole cashier menu as a spreadsheet (CSV / XLSX / ODS, like every other sheet) and import it back.
- **The sheet is the whole menu** (user's choice A): after an import the menu is exactly what the sheet says;
  groups and items not in the sheet are gone.
- Before anything changes, the import shows what it will do; the admin confirms.
- Lives on **Data → Spreadsheets** as a **Cashier menu** row beside Settings (user), same Export ▾ / Import buttons.

## Non-goals

- Creating products. A product the sheet names that the shop doesn't have is skipped and listed, never made
  (a menu row has no price, unit or stock; for a new shop the products sheet goes in first).
- Plugin menus (Combos and other plugin chips at the till) — they're the plugins' own.
- A new menu editor screen. Admin → Groups stays as it is for small changes.

## Anti-goals

- A half-applied menu. Apply does exactly what the preview showed (skipped lines included) or nothing at all.
- A preview that's an estimate. The numbers shown are exactly what Apply does.
- Touching anything outside the menu: products, categories, stock, money.

## The sheet

One row per line, read top to bottom; **row order is till order** (groups among their siblings, products inside a
group).

| Column | Meaning |
|---|---|
| Group | the group's path, `Parent > Child > …` (the products sheet's category separator) |
| Emoji | the group's emoji (on a group row) |
| Active | `no` = a hidden group; blank / `yes` = shown (so a hidden group survives a round trip) |
| Product | the product's name (blank = a group row) |
| Barcode | the product's barcode (export fills it; import tries it first) |
| Product emoji | that product's emoji in this group |

- A row without a product defines the group: its emoji, Active, and its place in the order. A group that only ever
  appears on product rows is still made (in the order it first appears), so a sheet needs no group rows at all.
- A product row puts that product in the group. The same product may sit in several groups; twice in the same group
  counts once.
- A product is found by **barcode** (any of its barcodes or a pack's), then by **exact name** (case and spacing
  ignored). No match, or a name two products share → skipped and listed with its line.
- Export writes every group (also empty and hidden ones) as a group row followed by its products, depth-first in till
  order, so export → import gives back the same menu.

## Approaches

1. **Rebuild — picked (user, 2026-11-23).** In one transaction: delete every group (items follow by
   `ON DELETE CASCADE`), write the sheet's groups and items fresh. The dry run is the same code in a transaction that
   is rolled back, so the preview's numbers are exactly Apply's. Groups get new ids on every import; nothing outside
   the menu references `product_groups.id` (checked: only `product_group_items`), and Pocket Cashier takes the new
   menu on its next sync.
2. Match up and adjust (find groups by path, rename / move / add / remove what changed). More code and cases; keeps
   group ids, which nothing needs. Not picked.
3. A bulk-edit mode on the Groups screen. Can't be edited by AI outside the Suite. Dropped.

## How it works

- `GET /api/v1/admin/data/menu/export` (`?format=` as the other sheets) and
  `POST /api/v1/admin/data/menu/import` (multipart `file`; `?dry=1` = preview). Admin only, like the rest of Data.
- The result (both calls): groups and items it will have, groups and items removed compared with now, and the
  skipped lines (line + reason). Dry run → nothing kept.
- The Data page: Import → the preview in a dialog ("12 groups, 340 items; 3 groups and 40 items removed; 2 lines
  skipped" + the skipped list) → **Replace the menu** / Cancel. Not a toast (a question is a dialog).
- An empty sheet (header only) is refused, not read as "delete the whole menu".
- Reuses the sheet helpers (`readImportSheet`, `writeSheet`, `ExportMenu`, `apiUpload`); the groups service
  (`internal/features/productgroups`) gets the rebuild.

## Rule book first

A new behaviour, so it starts in `docs/rules/` before code: a `MENU` area (new prefix in the README) with its grid —
the menu's features (group, sub-group, hidden group, emoji, order, a product in a group, the same product in two
groups) × the later actions (export, import, import with a missing / shared-name product, a product merged or
disabled after, a setup-only backup / restore). Each rule gets a failing test first.

## Left to the builder

- Column header synonyms (e.g. `Category` → Group) and the exact skipped-line wording.
- Whether a disabled product can be put in the menu (follow what Admin → Groups allows today).
- Max lengths follow the columns (group name 80, emoji 16); a longer cell is a skipped line, not a cut.
- Docs: the Data page section of the guide + a screenshot of the preview dialog; README grid only if it's headline.
