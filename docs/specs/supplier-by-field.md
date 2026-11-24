# Supplier by field (Product Plus) — design

Agreed with the user 2026-11-24. Delete this spec and its plan when the work is done.

## Why

Some shops know a product's supplier from one of its fields: a spare-parts shop buys every Bosch part from one
supplier and every NGK part from another. Today a product has no supplier until its first order or delivery sets the
preferred supplier (PUR, "set if empty"). Until then it is missing from that supplier's Low stock list, order, count
sheet and product count. The shop should be able to say "Brand = Bosch → Lanka Auto Parts" once and have every Bosch
product with no supplier of its own follow it.

**Mainly for onboarding (user):** setting a supplier on every product while putting a shop's products in is slow.
Instead the shop fills a Company / Brand field (on the form, or a column in the products sheet — Product Plus already
carries its fields in sheets), then sets each value's supplier once. As time goes on, orders and deliveries give each
product its own real supplier and the rule supplier fades out by itself.

Not for Iman's 550 products with no supplier: those wait for their first order or delivery (user, 2026-11-24). Iman's
Distributor field is the importer, not who delivers, so a rule there would guess wrong.

## What the shop gets

In Product Plus, the shop picks **one field** (text or dropdown) as the supplier field. The screen lists every
distinct value that field holds, with how many products have it, and the shop sets a supplier for each value (or
none).

A product's supplier is then:

1. its own preferred supplier, if it has one;
2. else the supplier its value of the chosen field points to (the **rule supplier**);
3. else none.

The rule supplier counts **everywhere the preferred supplier does**, except where real history matters:

| Place | Uses the rule supplier? |
|---|---|
| Products list: supplier name + filter by supplier | yes |
| Product form / page: shown as a hint ("By Brand: Lanka Auto Parts"); the supplier picker stays empty | yes (hint only) |
| Low stock (counter tab and admin reorder): supplier column + filter | yes |
| Making orders from Low stock: the line's supplier | yes |
| Count sheet: filter by supplier | yes |
| Supplier page: product count | yes, shown apart ("12 own + 30 by Brand") |
| Smart reorder levels: the supplier's delivery rhythm | yes |
| Loss claims (`recovery/losses.go`) | **no** — user |
| Lots: who a lot belongs to (`stock.LotSupplierSQL`: expiry warn days, give back, recall) | **no** — claims too (my call) |
| Products sheet export, import preview (`import_diff.go`), loose versions copying the supplier | **no** — a round trip must never stamp a guess as the product's own |

Never written: Product Plus never sets `products.preferred_supplier_id`. Ordering or receiving keeps doing what it
does today: an order or delivery sets the preferred supplier if empty, and from then on the product's own supplier
wins. Ordering from Low stock through a rule therefore makes that supplier the product's own — wanted.

## Approaches (user picked 1)

1. **A generic hook that answers in product ids** (picked). Core asks the plugins "who supplies these products?" for
   products with no preferred supplier, and "which products does supplier X get by rule?" for filters (`preferred = X
   OR (preferred IS NULL AND id = ANY(ids))`). Same pattern as `ProductSearchContributor`. Always current, core never
   sees plugin tables. Cost: a small change at each read site.
2. A core table of fallback suppliers kept filled by plugins — stale the moment one refresh is missed. Rejected.
3. The plugin hands core a SQL fragment — core would run plugin SQL; breaks plugin-inert. Rejected.

Shape of the rule (user picked A): **one field per shop**. Several fields in order, or combinations of fields, were
rejected for now; either can be added later without changing this.

## How it's built

**Core seam** (`internal/plugin/hooks_product.go`):

```go
type SupplierFallback struct {
    // For returns, for products that have no preferred supplier, the supplier a plugin's rule gives
    // and why ("Brand: Bosch"). Omit an id for none.
    For func(ctx context.Context, productIDs []int64) (map[int64]FallbackSupplier, error)
    // Products lists the products whose rule gives supplierID (core keeps only those with no
    // preferred supplier).
    Products func(ctx context.Context, supplierID int64) ([]int64, error)
}
type FallbackSupplier struct { SupplierID int64; Why string }
```

Registered with `reg.AddSupplierFallback`. Core asks every registered fallback; the first answer for a product wins
(only Product Plus answers today). Core filters out disabled suppliers: a rule pointing at a disabled supplier counts
as none. A failing fallback is an error, not a silent "none" (the list would quietly lose rows).

One core helper (in `internal/features/products`) wraps both calls so each read site is a few lines: fill names after
loading rows, add the `OR id = ANY` to a supplier filter.

**Product Plus**: one new migration — the chosen field (one row, nullable) and `pp_supplier_map (value, supplier_id)`.
Only text and dropdown fields can be chosen. Values match trimmed and case-insensitively ("bosch" = "Bosch"). If the
chosen field is disabled or its type changes to one that can't be chosen, the rules are kept but ignored until it's
usable again. Choosing a different field clears the map (after a confirm). Product and category merges need nothing
new (values already merge).

**Moving a supplier's products** (Suppliers → move products to another supplier): rules pointing at the old supplier
move too, through a new generic hook in the same transaction (my call; core never names Product Plus).

**Screen**: a "Supplier by field" section on Product Plus's admin page: field picker (eligible fields only), then one
row per value — value, product count, supplier picker. Onboarding can bring many values (Iman's Distributor
field has 94), so: a search box, values with no supplier first, then by product count (most first). Admin kit components, theme tokens, works at 360 px. The
product form's hint comes from core (`fallbackSupplierName`, `fallbackSupplierWhy` on the product DTO), so core shows
it without knowing who gave it.

## Rule book, tests, docs

- Core rule (purchasing or stock area): a product's supplier = own, else a plugin's fallback, else none; claims and
  lots use only real suppliers; nothing but an order or delivery sets the own one. Product Plus rows PPF-10 onward for
  the field choice, matching, disabled field / supplier, moving a supplier.
- Ledgercheck: every `pp_supplier_map` supplier exists; the chosen field, if set, is text or dropdown.
- Battle case tests per rule; Low stock / order from it with a rule supplier; a claim never gets one.
- Guide `docs/guide/plugins/productplus.md` + a screenshot of the section (`shots.spec.ts`); README only if headline.

## Left to the builder

Exact rule ids and wording, DTO field names, the helper's name and where each read site calls it, the migration
number, and the hint's look on the product form (follow the mockups).
