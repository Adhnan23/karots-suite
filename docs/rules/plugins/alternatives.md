# Alternatives (ALT)

Code: `plugins/alternatives` (store.go groups + tiers, store_members.go, split.go, grouper.go, offline.go, api.go),
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
```
