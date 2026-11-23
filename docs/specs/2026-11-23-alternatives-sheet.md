# Alternatives sheet (export, import with a preview)

## Why

Finding which products are twins is the slow part of Alternatives, in every kind of shop. A sheet lets that work
happen anywhere: any AI (ChatGPT, the AI Assistant, a script) or a person in a spreadsheet groups the products, and
the shop imports the result after seeing what will change. For a new shop we can make its alternatives file the way
Iman's reset files were made (`products.csv`, `suppliers.csv`, `cashier-menu.csv`) and import it on day one.

Replaces the dropped "Group by fields" idea (user, 2026-11-23: exact field values miss twins, e.g. generic names
holding several ingredients joined by "+"; AI does the grouping better, the sheet is simpler). Making the AI
Assistant's group maker read custom fields is a separate, later piece of work.

## Goals

- **Sheet ▾ on the Alternatives page:** Export and Import, CSV / XLSX / ODS like the cashier menu sheet.
- One row per product: `Group, Tier, Product, Barcode`. Tier blank = a group without tiers.
- **The sheet is the whole set of Alternatives** (like the cashier menu, MENU-2): after Apply the groups are exactly
  what the sheet says; a group, tier or member the sheet doesn't name is gone.
- **A preview first** (like the menu and products sheets): groups added / removed, products added / moved / taken
  out, skipped lines with their line number and reason. Apply does exactly what the preview showed, all or nothing.
- What a sheet can't carry survives for a group still named in it: on / off, typed reorder levels, hand splits.

## Non-goals

- No product is ever created, renamed or changed; no stock, money or product reorder level moves.
- No "add only" mode: the whole-set model is the only one (user's call, 2026-11-23).
- The AI Assistant's group maker and custom fields: later, separately.
- The old CSV (one row per tier with stock totals) goes: it couldn't be imported back, and the page shows stock.

## Anti-goals

- A removal the owner didn't see in the preview.
- An empty or half-read sheet wiping the groups.
- Anything shop-kind specific in wording or logic.

## Approaches

One sensible way: the plugin's own export / import routes and a preview screen modelled on the cashier menu sheet
(`internal/features/productgroups/menu.go`, `internal/api/admin_data_menu.go`, `web/src/features/data/MenuSheet.tsx`).
A core Data-page entry would need core to know Alternatives (it can't); doing it through the AI Assistant would tie
it to a paid plugin.

## How it works

**Columns** (header row; case and spacing ignored; synonyms like the menu sheet's: `group name` → Group,
`quality` / `level` → Tier, `item` / `name` → Product, `code` / `sku` → Barcode).

**Export** (admin + manager, like the page): every group, on or off, in page order; tiers in their order; members in
their order, disabled products too. Product = its name, Barcode = its main barcode.

**Import** (admin only — it can remove every group; the menu sheet is admin only too):

- A line's product is found by its barcode (any of its barcodes, or a pack's), then by its exact name (case and
  spacing ignored), never by id, so a sheet made outside the shop fits. Core's `products.FindForMenu` already does
  exactly this; the plugin calls it (plugin → core read).
- **Skipped and listed** (line number + reason): no product found; a name two products share; a service (ALT-2);
  a line with a product but no group; a product already placed by an earlier line in another group or tier
  ("already in <group> on line N"). The same product twice in the same group and tier is one member.
- A group is matched to an existing one by name (case and spacing ignored), a tier inside it by name (blank = the
  blank tier). A matched group keeps its id, on / off and its matched tiers' levels and Auto; a hand split whose
  members changed is rescaled (ALT-5). New groups and tiers start on, Auto level, Auto split. A group whose lines are
  all skipped isn't made (or, if it existed, is removed and listed as removed).
- Order: groups in the order they first appear, tiers likewise within a group, members in line order.
- A sheet with no product lines is refused, never read as "remove every group".
- Preview = the same code run in a transaction that's rolled back; Apply sends the very rows that were previewed and
  says if it skipped something the preview didn't. Two imports at once: the second waits (a transaction lock).

**Screen:** Sheet ▾ (Export, Import…) replaces the CSV button. Import opens a file picker, then a preview sheet:
counts at the top (groups added / kept / removed, products placed / moved / taken out, lines skipped), then the
removed groups and the skipped lines listed, then **Replace** / Cancel. Works at 360px, theme tokens, admin kit.

**Core seam (no behaviour change):** the sheet read / write helpers (`writeSheet`, `readImportSheet`, `cellGetter`
in `internal/api`) move to an exported core package so a plugin reads and writes sheets the same way. Core still
never names a plugin.

## Rules (rule book first)

A new row in `docs/rules/plugins/alternatives.md`: **The sheet (export, import)** × Do / Till / Reorder / Merge /
Faults, with rules modelled on MENU-1…13 (ids ALT-9…): round trip (export then import unchanged gives the same
groups, tiers, members, order, on / off, levels, splits), whole set, preview = apply, all or nothing and one at a
time, finding a product, empty sheet refused, a product on two lines, matched groups keep their settings, admin only,
nothing outside Alternatives changes (the auditor agrees before and after), a disabled product stays and round-trips.

Tests: db tests per rule; a battle case per rule (`plugins/alternatives/battle`); a Vitest for the preview screen;
one Playwright spec (export → edit → import → preview → Replace → the page shows it).

## Docs

`docs/guide/plugins/alternatives.md` (the sheet; how to group with any AI: export the products sheet with its
fields, ask the AI for this sheet, import it) and a screenshot of the preview (`shots.spec.ts`).

## After this ships

Iman's `alternatives.csv`, built in the catalogue repo next to the iman-reset files (generic name + strength from
the names, AI where it helps), imported by the user after checking the preview.

## Left to the builder

File layout, the exact synonym list, the preview's counts wording, the new core sheets package's name, the lock.
