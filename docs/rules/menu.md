# The cashier menu sheet

Code: `internal/features/productgroups` (menu.go: the sheet's lines, `Rebuild`), `internal/features/products`
(`FindForMenu`), `internal/api/admin_data_menu.go` (the Data page's **Cashier menu** row: export, preview, replace),
`web/src/features/data/MenuSheet.tsx` (the Data row, and **Sheets ▾** on Admin → Cashier menu: the same import and preview).

The cashier menu (Admin → Groups) is the till's tree of groups and the products in them. It holds no money and no
stock. What the sheet must never do is **leave a half-changed menu, change something it didn't show first, or touch
anything outside the menu**. Its use (the user, 2026-11-23): a shop's menu is built or reworked in a spreadsheet —
by hand or by AI, often on a preview shop — and imported into the real shop. So the "auditor" for this area is the
**round trip** (menu → sheet → the same menu) and `ledgercheck` passing before and after.

## The words the rules use

- **Line** — one row of the sheet, numbered as the sheet shows it (the header is line 1).
- **Group row** — a line with no product: it sets a group's emoji, Active and place in the order.
- **Item** — a product in a group (`product_group_items`), with its own emoji.
- **Path** — a group named with its parents, `Drinks > Soft drinks` (spaces around `>` don't count).
- **Preview** — the import run with `?dry=1`: everything Apply would do, then thrown away.

## The grid

Columns: **Export** · **Import** · **Missing / shared product** (the sheet names a product the shop hasn't got, or a
name two products share) · **Merged after** (the product is merged into another) · **Disabled after** (the product
is turned off) · **Setup backup** (a "catalogue & setup only" backup, restored).

| Feature | Export | Import | Missing / shared product | Merged after | Disabled after | Setup backup |
|---|---|---|---|---|---|---|
| Group (top level) | MENU-1,16 | MENU-1,2,3,4,6 | — a | — b | — b | MENU-14 |
| Sub-group (any depth) | MENU-1 | MENU-1,2,15 | — a | — b | — b | MENU-14 |
| Hidden group | MENU-8 | MENU-8 | — a | — b | — b | MENU-14 |
| Group emoji | MENU-1 | MENU-1,10 | — a | — b | — b | MENU-14 |
| Order | MENU-7 | MENU-7 | MENU-7 | MENU-13 | MENU-13 | MENU-14 |
| Product in a group | MENU-1 | MENU-1,5,12 | MENU-5 | MENU-13 | MENU-13 | MENU-14 |
| Product emoji | MENU-1 | MENU-1,9,10 | MENU-5 | MENU-13 | MENU-13 | MENU-14 |
| Same product in two groups | MENU-1 | MENU-9 | MENU-5 | MENU-13 | MENU-13 | MENU-14 |

a — a group isn't a product: only product lines can name a missing one. b — merging and disabling act on products,
not groups.

Who may: MENU-11 (every square). Nothing else moves: MENU-12 (every Import square).

## Rules

```
MENU-1  Export, then import that sheet unchanged, gives back the same menu: the same groups at the same places, the
        same names, emoji, Active and order, the same items with the same emoji.
        Tricky: a hidden group; an empty group; a disabled product in a group; three levels deep. Two groups with the
        same name under the same parent come back as one, holding both groups' items (the sheet can't tell them apart).
        Checked by: example.

MENU-2  The sheet is the whole menu: after Apply the menu is exactly what the sheet says; a group or an item the sheet
        doesn't name is gone.
        Tricky: a group dropped with its sub-groups and items; an item dropped from one group but kept in another.
        Checked by: example.

MENU-3  The preview changes nothing, and Apply does exactly what the preview reported: the same groups and items, the
        same removed counts, the same skipped lines. Replace sends the very sheet that was previewed (a file changed on
        disk afterwards isn't read again), and if Apply still skips a line the preview didn't, the screen says so.
        Checked by: example (also MenuSheet.test.tsx).

MENU-4  Apply is all or nothing: if anything fails part way, the menu is exactly as it was before. Two imports at
        once never mix: the second waits for the first, then replaces it.
        Tricky: a failure on a line after groups were already written; two admins pressing Replace together.
        Checked by: example.

MENU-5  A line's product is found by its barcode (any of the product's barcodes, or a pack's), then by its exact name
        (case and spacing ignored) — never by an id, so a menu made in one shop imports into another. No match, or a
        name two products share, skips that line and lists it with its line number and the reason. A product is
        never created.
        Tricky: the same products made in another order in another shop; a pack's barcode; "Panadol  500MG".
        Checked by: example.

MENU-6  A sheet with no menu lines (only a header, or only blank lines) is refused; it is never read as "remove the
        whole menu".
        Checked by: example.

MENU-7  The order of the lines is the order at the till: sibling groups in the order they first appear, items in the
        order of their lines. Skipped lines leave no gap.
        Checked by: example.

MENU-8  A hidden group (Active = no) stays hidden through export and import; a blank Active means shown. no, n,
        false, 0, off and hidden all mean hidden (any case: AI writes booleans). When a group has several group rows,
        the last one's emoji and Active win.
        Checked by: example.

MENU-9  A product twice in the same group is one item, with the first line's emoji; the same product in two
        different groups is an item in each.
        Checked by: example.

MENU-10 A cell longer than its column (a group name over 80, an emoji over 16) skips that line and lists it; nothing is
        cut short.
        Checked by: example.

MENU-11 Only an admin can export or import the menu.
        Checked by: example.

MENU-12 Importing changes nothing outside the menu: products, categories, stock, money and sales are as they were, and
        the auditor agrees before and after.
        Checked by: example, ledgercheck.

MENU-13 A product merged into another keeps its places in the menu on the product it merged into (the merge's own
        rule); a disabled product stays in the menu and is exported and found again on import.
        Checked by: example.

MENU-14 A "catalogue & setup only" backup carries the menu and its restore brings it back as it was (BAK-10).
        Checked by: example.

MENU-15 A group named only on product lines is still made, with every level of its path, in the order it first
        appears; the same path written with different spacing is one group.
        Checked by: example.

MENU-16 A group's name never holds ">" (Admin → Groups refuses it, as Categories does): the sheet reads ">" as a path,
        so such a name couldn't survive a round trip.
        Checked by: example.
```
