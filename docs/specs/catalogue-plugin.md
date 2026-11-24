# Spec — the Catalogue plugin (online catalogue, piece 2)

Rules: `docs/rules/plugins/catalogue.md` (CTL-1…12) and `docs/rules/stock.md` (REV-1, REV-2). Piece 1 (the
catalogue repo's Catalogue app + Publish) is done; piece 3 is Shelf's From catalogue, piece 4 Onboard sync.
Deleted with its plan when done.

## Decisions (user, 2026-11-11 → 2026-11-24)

- A paid plugin: **Rs 500 a month, free while in testing**. Iman gets it in the next Release.
- The shop's **main PC keeps the whole catalogue** (not split by shop kind) and checks for changes; search is local,
  instant and works offline. Cashier PCs, Pocket Cashier and Shelf search through the main PC.
- Picked at: **From catalogue next to Identify** (product form, intake's New item), the cashier's **quick item**,
  **receiving** (a purchase's new product line). What's picked goes to **Items to review**; a later catalogue fix to a
  product the shop took shows there too.
- Fills only what the shop has: name, unit, and the Product Plus fields with the same names. Never a price, cost,
  barcode, category or stock. Fixes made in a shop don't go back to the catalogue.
- Not paying = the shop's copy is removed: later, with the service plan.

## Where the catalogue comes from

- The published folder: `<UPDATE_BASE_URL>/<CATALOGUE_DIR>/` (the catalogue repo's README). The Shop Builder bakes
  it into every shop build (`-X karots-suite/plugins/catalogue.baseURL=…`, ignored when the plugin isn't compiled in;
  `CATALOGUE_DIR` copied into the suite's `.env`). `KAROTS_CATALOGUE_URL` overrides it (dev, tests).
- No address → the plugin says "No catalogue address in this build" on its page and the pickers show nothing to pick.

## The plugin (`plugins/catalogue`)

- **Tables**: `catalogue_items` (id, name, unit, kinds, fields jsonb, a search text with a trigram index),
  `catalogue_state` (one row: version, fields list, checked_at, last error), `catalogue_links` (product_id unique,
  item_id, took jsonb = name / unit / fields as filled), `catalogue_reviews` (product_id unique, item_id, changes
  jsonb). Data group: all four (PLG-7).
- **Sync** (CTL-2, 3, 8, 12): at boot when the last check is over 20 hours old, every night at 03:30, and **Check
  now** on its page. Reads `index.json`; same version → done; else the change files after the copy's version in
  order (SHA-256 checked), or the whole version when the copy is empty, a change file is missing or the index is
  older than the copy (refused: only forward). Everything in one transaction, with the links' review step (CTL-8, 12).
- **Search** `GET /api/v1/x/catalogue/search?q=` (signed-in staff; the till, admin, Pocket and Shelf): words in any
  order over name + field values, best first, 20 results; each says the shop product it already matches (CTL-5).
  `GET …/status`: version, count, last check, error, has address.
- **Pick → save** (CTL-6, 7, 10): the pickers send `extra.catalogue = {itemId}`; `ProductExtras` Validate checks the
  item exists, Save writes the link (what was taken = the product's saved name, unit and the shop's field values) and
  fills a field the shop has that was left empty, through Product Plus's attribute set.
- **Items to review**: through the new core review hook (REV-1, 2): "From the catalogue" for a new link until
  reviewed; "The catalogue changed: Generic Name X → Y" for a review row, with **Use the catalogue's** / **Keep mine**
  (CTL-9).
- **Merge** (CTL-11): a `ProductMerger` hook.
- **Page** (Apps → Catalogue): version, products, last check, Check now, the address's state.

## Core seams (generic, plugin-inert)

- **Review hook** (`plugin.ReviewSource`): `Notes(ctx) → product id → notes`, `Clear(ctx, productID)`; core's review
  list ORs them in and Mark reviewed calls Clear. Web slot `reviewActions` (buttons on a row by its notes' plugin).
- **Quick item and the purchase line's new product** take `extra` like the product form and run the same
  `ProductExtras` hooks; the `productAssists` slot also renders there (`where: 'quick' | 'purchase'`), and
  `AssistFill` gains `unitId`.

## Phones and other PCs

- Cashier PCs use the main PC's pages: nothing to do. Pocket Cashier: the till's quick item renders the assist
  online through `phoneTillApi`; offline the catalogue route answers "offline" and the person types (CTL-4) — a
  change in `pocketcashier` only. Shelf: piece 3.

## Docs and legal

README plugin table + badge, `docs/guide/plugins/catalogue.md` + its row, `shots.spec.ts` key + `make docs-shots`,
the First Schedule tick box, Shop Rules §15 row (Rs 500 / month, free while in testing) + the Everything row's count,
`make agreement`; `internal/docscheck` passes. **Demo Room** (user, 2026-11-24): a small sample copy embedded in the demo, loaded into each demo shop that has the
plugin, so picking works there without network.
