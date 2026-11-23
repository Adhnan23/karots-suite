# Group by fields (Product Plus → Alternatives)

## Why

Making Alternatives groups by hand is slow, and the AI group maker needs the AI Assistant and money per run. Most
shops already have a field that says what a product *is* (Brand, Type, Generic Name, Model…). The slow part is
finding the products that share it (user, 2026-11-23: grouping is the main problem; splitting or moving a few by hand
takes little time). This finds them and proposes the groups in one go; the owner splits off what doesn't belong
(another size, say) and checks every group before it's saved. It's for every kind of shop (grocery, hardware, electronics, pharmacy): nothing here knows about
medicines.

## Goals

- A **Group by fields** button on the Alternatives page, there only when Product Plus is compiled in.
- The owner picks one or more Product Plus fields; products whose values match in all of them are proposed as a group.
- A review before anything is saved: each proposed group with its products; untick products and **Move to a new
  group** (another size: Zart 25 vs Zart 50 share a generic name but are never alternatives, not even in another
  tier), skip whole groups, edit names, then **Save**. Saved groups are ordinary Alternatives groups (one tier, Auto level and split).

## Non-goals

- No AI, no new field (no Strength field), no reading sizes out of names (user, 2026-11-23: moving the few by hand is
  quick), no tiers.
- No joining products to groups that already exist; products already in a group are left out (later, if asked).
- No undo of a run as a whole: a wrong group is deleted on its own page, like any group.
- No core change and no Alternatives change.

## Anti-goals

- A wrong group saved without the owner seeing it: the till would offer the wrong swap.
- Anything pharmacy-shaped in wording, examples or logic.

## Approaches

1. **Picked:** a button on the Alternatives page, the grouping worked out on the server without AI, a review screen,
   save as ordinary groups.
2. A no-AI job inside the AI Assistant: reuses its review screens, but only exists where the AI Assistant is
   compiled in, and grouping isn't an AI job. Dropped.
3. Through a spreadsheet (a Group column, import back): the owner groups by hand, which this is meant to save.
   Maybe a later extra.

**Who owns it (my call):** Product Plus, the way the AI Assistant adds "✨ Suggest groups": its React half pushes a
`pageActions` entry for `page: 'alternatives'` (a link to its own sub-page), and its Go half creates groups through
`plugin.ProductGroupers()` (`Create`), the hook Alternatives already offers. So the button shows only when Product
Plus is in (the user's wish), Alternatives never names Product Plus, and Product Plus never writes `alt_*` tables. If
no grouper is registered, the routes answer "No Alternatives in this shop" and nothing shows (the button only lives
on the Alternatives page anyway).

## How it works

**Screen:** `/admin/x/productplus/group` (Product Plus's admin page, sub-path `group`), reached from the button.
Works at 360px, theme tokens only, built from the admin kit (PageHead, Field, chips, DataList-style cards).

1. **Pick:** chips of the shop's enabled Product Plus fields (any type except textarea / bool / date); at least one.
   **Find groups**.
2. **Review:** "N groups · M products", each group a card: an editable name (default: the picked values joined with
   " · ", e.g. `Losartan`, `Anchor · Milk powder`), its products as ticked rows (name, category, stock), sorted by
   name so sizes sit together. **Move unticked to a new group** puts the unticked products in a new card just under
   it (named like its parent, to edit), which can be split again. Untick a card's header to skip the whole group. A
   group with fewer than 2 ticked products isn't saved; an unticked product not moved stays ungrouped. Cards sorted
   by size, biggest first. All of this lives in the browser until Save.
3. **Save** → "Saved N groups (M products)" with a link back to Alternatives.

**Who is proposed** (server, `POST /api/v1/x/productplus/group/preview`):

- Active, non-service products not already in a group (from the grouper's `Groups`).
- With a non-empty value in **every** picked field (a field's scope doesn't matter: a value is a value).
- Key = each picked value, trimmed, spaces collapsed, lower-case.
- Groups of 2 or more products. No cap on count; the review shows them all (a long list scrolls).

**Save** (`POST /api/v1/x/productplus/group/save`, admin + manager like Alternatives): `{groups: [{name, productIds}]}`
→ one transaction: for each group, `Create(name, ids)` (it skips products grouped in the meantime and makes no group
with fewer than 2 left). Answers how many groups and products were saved. One audit entry per run.

## Rules (rule book first)

A new row in `docs/rules/plugins/productplus.md`: **Group by fields** × Save / Show / Field change / Merge / Faults,
with rules along these lines (ids PPF-10…):

- **PPF-10** The proposal holds only active, non-service products not already grouped, each with a value in every
  picked field; a group is two or more products whose normalised values are all equal.
- **PPF-11** Saving makes only the ticked groups, each with only its ticked products, through the grouper hook, in
  one transaction; a product grouped since the preview is skipped, a group left with under 2 isn't made.
- **PPF-12** Nothing is written by Find groups; leaving the review writes nothing.
- Field change / disabled field: a disabled field isn't offered (PPF-8). Merge: not touched (groups follow ALT-7).

Tests: a Go table test for the grouping key; a Vitest for the review's split / skip logic; db tests for preview / save per rule; a battle case per
rule; one Playwright spec (pick → review → untick, move to a new group → save → both groups on Alternatives). Ledgercheck: ALT-1 already guards
the result.

## Docs

`docs/guide/plugins/productplus.md` (Group by fields) and a line in `alternatives.md`; a screenshot of the review
(`shots.spec.ts`, with fields filled in the Demo Room's shop if needed). No price or legal change.

## Left to the builder

File layout inside `plugins/productplus` and `web/src/plugins/productplus`, the exact unit regex, the audit wording,
how stock is shown on a review row, and whether the review uses `DataList` or its own cards.
