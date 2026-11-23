# Alternatives (ALT)

Code: `plugins/alternatives` (store.go groups + tiers, store_members.go, split.go, grouper.go, offline.go, api.go,
sheet.go + api_sheet.go the sheet),
`web/src/plugins/alternatives`. Tables: `alt_groups` → `alt_tiers` → `alt_members` (one membership per product).

## The words the rules use

- **Live** — an active product in an active tier of an active group. Search, badges, the till's swap chip, the ⓘ rows
  and the Reorder notes see live members only.
- **Tier level** — Auto (from its live members' levels) or typed; a typed 0 means "don't track".
- **Split** — how a low tier's order divides among its live members: by what sells (Auto) or by the shop's shares
  (whole percents adding up to 100).

## The grid

Columns: **Do** · **Till** (search, swap, ⓘ) · **Reorder** · **Merge** · **Faults**.

| Feature | Do | Till | Reorder | Merge | Faults |
|---|---|---|---|---|---|
| Group, tier (add, rename, on / off, delete) | ALT-1, ALT-8 | ALT-3 | ALT-6 | — a | PLG-4,5 |
| A member (add, move, remove) | ALT-2 | ALT-3, ALT-4 | ALT-6 | ALT-7 | PLG-3,4,6 |
| Hand split | ALT-5 | — a | ALT-5 | ALT-5 | PLG-5 |
| Tier level | ALT-6 | — a | ALT-6 | — a | PLG-5 |
| The sheet (export, import) | ALT-9…18 | ALT-3 | ALT-6 | ALT-7 | PLG-4,5 |

a — not touched there.

## Rules

```
ALT-1  A product sits in at most one tier; a tier belongs to one group.
       Checked by: ledgercheck (the database's keys), example.

ALT-2  Only an existing product that isn't a service joins; one already in another group moves only when
       asked to.
       Checked by: example, refusals.

ALT-3  The till sees only live members: search finds a group's live members, the badge and ⓘ name them.
       Checked by: example.

ALT-4  The swap chip offers in-stock live alternatives of the same group only, never the product itself.
       Checked by: example.

ALT-5  A hand split's shares are whole percents of the tier's members adding up to 100. When a member
       leaves (removed, moved to another tier, merged away) the rest are rescaled to whole percents adding
       up to 100 (largest remainder) and stored (user, 2026-11-21). A low tier's order always divides into
       exactly its shortfall; a disabled member gets none and the live ones share it by their shares.
       Checked by: ledgercheck, example.

ALT-6  A tier's level is its live members' levels added up when Auto, else what was typed; a low product its
       tier still covers is noted on Reorder, not asked for.
       Checked by: example.

ALT-7  A product merge keeps the kept product's membership; the duplicate's moves over only when the kept
       one has none (PLG-2), and the split follows ALT-5.
       Checked by: example, ledgercheck.

ALT-8  Deleting a group or tier removes its tiers and memberships, never a product.
       Checked by: example.

ALT-9  Export, then import that sheet unchanged, gives back the same set: the same groups, tiers and members, the
       same group and tier order, on / off, tier levels (typed or Auto) and hand splits.
       Tricky: a turned-off group; a disabled member; two tiers; a hand split. An empty group or tier has no product
       line, so it doesn't come back (the preview lists the group as removed).
       Checked by: example.

ALT-10 The sheet is the whole set (user, 2026-11-23): after Apply the groups are exactly what it says; a group, tier
       or member it doesn't name is gone.
       Tricky: a group dropped with its tiers; a member dropped from a group that stays.
       Checked by: example.

ALT-11 The preview changes nothing, and Apply does exactly what the preview reported. Replace sends the very sheet
       that was previewed (a file changed on disk afterwards isn't read again), and if Apply still skips a line the
       preview didn't, the screen says so.
       Checked by: example (also SheetMenu.test.tsx).

ALT-12 Apply is all or nothing; two imports at once never mix: the second waits for the first, then replaces it.
       Checked by: example.

ALT-13 A line's product is found by its barcode (any of its barcodes, or a pack's), then by its exact name (case and
       spacing ignored), never by an id, so a sheet made outside the shop fits. Skipped and listed with its line
       number and the reason: no match; a name two products share; a service; a disabled product not already in that
       tier; a line with no group; a product an earlier line already put in another group or tier. The same product
       twice in one group and tier is one member. A product is never created.
       Checked by: example.

ALT-14 A sheet with no product lines is refused; it is never read as "remove every group".
       Checked by: example.

ALT-15 A group matches an existing one by name, a tier one of its group's by name (case and spacing ignored; a blank
       tier matches the blank tier). A kept group keeps its on / off; a kept tier its level, Auto and on / off; a
       member that stays in its tier keeps its share; a hand split that lost or gained members follows ALT-5. New
       groups and tiers start on, with Auto level and split. Groups and tiers take the sheet's order and the
       spelling of their first line.
       Checked by: example.

ALT-16 Only an admin imports the sheet (it can remove every group); an admin or a manager exports it.
       Checked by: example.

ALT-17 Importing changes nothing outside Alternatives: products (their reorder levels too), stock, money and sales are
       as they were, and the auditor agrees before and after.
       Checked by: example, ledgercheck.

ALT-18 A disabled product already in a tier stays there through an import, is exported, and is found again.
       Checked by: example.
```
