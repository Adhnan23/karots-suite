# Catalogue (CTL)

Code: `plugins/catalogue`, `web/src/plugins/catalogue`. Tables: `catalogue_items` (the shop's copy of the published
catalogue), `catalogue_state` (the version it holds, the last check), `catalogue_links` (a shop product made from a
catalogue item, and what it took), `catalogue_reviews` (a later catalogue change waiting on Items to review).

The catalogue is ours, published online by the Catalogue app (`~/Projects/karots-suite-catalogue`): versions, each
with the products added, changed and gone since the one before (`index.json`, `v<N>.json.gz`, `c<N>.json.gz`). A
shop's **main PC** keeps the whole of it and searches it locally; cashier PCs, Pocket Cashier and Karots Shelf ask the
main PC. Rs 500 a month, free while in testing (user, 2026-11-24).

## The words the rules use

- **The copy** — the shop's `catalogue_items`, at one published **version**.
- **Sync** — bringing the copy up to the latest version: the change files in order, or the whole version when the copy
  is empty, too far behind or a change file is missing.
- **Pick** — choosing a catalogue item while making a product: the product form, intake's New item, the till's quick
  item, a purchase's new product line, Pocket Cashier's quick item (online).
- **Link** — a shop product made from a pick: its catalogue id and what it took (name, unit, fields).
- **A field the shop has** — a Product Plus field whose name is the catalogue field's name (case and spaces ignored).

## The grid

Columns: **Do** · **Review** (Items to review) · **Core** (money, stock — PLG-1) · **Merge** (product merged) ·
**Offline** (no internet / phone off the network) · **Faults**.

| Feature | Do | Review | Core | Merge | Offline | Faults |
|---|---|---|---|---|---|---|
| Sync | CTL-2 | CTL-8 | CTL-1 | — a | CTL-4 | CTL-3, PLG-4,7 |
| Search | CTL-5 | — b | CTL-1 | — a | CTL-4 | PLG-5 |
| Pick → save | CTL-6 | CTL-7 | CTL-1 | — a | CTL-4 | CTL-10, PLG-3,4,5,6 |
| Link | CTL-7 | CTL-7 | CTL-1 | CTL-11 | — c | PLG-4 |
| A later catalogue change | CTL-8 | CTL-8 | CTL-1 | CTL-11 | — c | PLG-4 |
| Use the catalogue's / keep mine | CTL-9 | CTL-9 | CTL-1 | CTL-11 | — c | PLG-4,5,6 |
| Catalogue item gone (merged, dropped) | CTL-12 | CTL-12 | CTL-1 | — a | — c | PLG-4 |

a — a sync and a search never touch shop products. b — a search changes nothing. c — server-side, on the main PC.

## Rules

```
CTL-1  The plugin never moves money or stock: a pick only fills what the person is making, and core makes the
       product (and a quick item's stock) exactly as it does without the plugin. Never a price, cost, barcode,
       category or stock from the catalogue.
       Checked by: example (snapshot), ledgercheck.

CTL-2  After a sync the copy is exactly the published version: every id, name, unit, kinds and field; applying
       the change files in order ends the same as loading the whole version.
       Checked by: example (both ways compared), ledgercheck (the copy's count = the version's count).

CTL-3  A sync is all-or-nothing and only moves forward: a failed or cut download, a file whose SHA-256 doesn't
       match the index, or a bad file leaves the copy at its last good version; a gap (a change file missing)
       loads the whole version instead.
       Checked by: example (faults: bad hash, cut mid-way, missing change file, older index).

CTL-4  Without internet the shop keeps working from the copy it has: search, picks and saves work; a shop with
       no copy yet says so where it would search and the person types by hand. A phone off the shop network
       (Pocket Cashier offline) types by hand. Nothing ever blocks a form or a sale.
       Checked by: example.

CTL-5  Search is on the main PC's copy only (cashier PCs, Pocket Cashier, Karots Shelf ask the main PC), by
       name and field values, words in any order. A result the shop already has (linked, or a product with the
       same name) says so and names it.
       Checked by: example.

CTL-6  A pick fills the name, the unit (the shop's unit with that short name, else the shop's default) and the
       fields the shop has; a catalogue field the shop doesn't have is left out. What the person types over a
       filled value wins. Without Product Plus a pick fills the name and unit only.
       Checked by: example (with and without Product Plus).

CTL-7  A product saved from a pick is linked to its catalogue item and shows on Items to review ("From the
       catalogue") until it is marked reviewed. One product, one link.
       Checked by: example, ledgercheck (no link names a missing product).

CTL-8  A sync that changes a linked item's name, unit or a field the shop has puts that product on Items to
       review with what changed (catalogue then → now), unless the product already has those values. A change
       to a field the shop doesn't have, or to kinds, is not a review.
       Checked by: example.

CTL-9  On Items to review: "Use the catalogue's" writes only the changed name and fields (the name through
       core's rename, fields through Product Plus's own hook) and updates the link; a changed unit is shown but
       never applied (stock is counted in it: the person changes it on the form). "Keep mine" keeps the product
       as it is and updates the link, so the same change isn't asked again. Either clears that review.
       Checked by: example.

CTL-10 The same pick saved twice, or a save that fails, never leaves two links or a link without its product.
       Checked by: example (faults: sent twice, failure sweep).

CTL-11 A product merge moves the merged-away product's link and reviews to the kept product when it has none of
       its own; otherwise they go (PLG-2).
       Checked by: example (merge through core's API), ledgercheck.

CTL-12 A catalogue item merged into another (gone into): its links follow to the kept item, with a review when
       that changes something the shop has (CTL-8). Gone with nothing in its place: the link stays, nothing to
       review.
       Checked by: example.
```

## The core seam

Items to review is core's: a plugin adds its own products to it through a generic hook, with a note, and clears them
when a product is marked reviewed (`REV-1`, `REV-2` in [stock.md](../stock.md)). Core never names the plugin.
