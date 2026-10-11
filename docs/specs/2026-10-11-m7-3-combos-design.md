# M7.3 — Combos (design)

The user approved the rethink on 2026-10-11: **A–F all yes, plus a Combos report (G)**. The report is one
page, a table and a bar chart, and nothing more.

## 1. What karots-pos does (kept)

- **A combo** is a name, member products (each with a qty) and a discount. It needs at least **2 units**: two
  products, or one product ×2+ (buy-3-get-1-free = the item ×4).
- **It rings as real product lines**, one per member, discounted. Stock leaves per member, and returns /
  restocks work exactly like items sold one by one. **The receipt doesn't change** (layout is final): member
  lines with their discount, as today.
- **Live pricing:** the discount is worked out from current prices when the combo is added. No combo price
  is stored that could go stale.
- **Admin:** list with full → combo price, create / edit / enable / disable / delete.
- **Till:** a 🎁 Combos menu chip (tap a combo → its panel → Add), and a `🎁 Combo` badge on member products
  in search (`ProductBadgeProvider`, already in the suite).

## 2. What changes (user, 2026-10-11)

- **A. Exact money, and a third type.**
  - Types: **% off**, **Rs off** (per set) and **Combo price** (a set costs Rs X). `discount_type` is
    `percent | fixed | price`.
  - The till works the set's discount out from the **line's real price** (wholesale price on a wholesale
    sale, the same rule as `unitPriceFor`). karots-pos turned Rs off into a percent of the **retail** total,
    so a wholesale sale got the wrong discount. That's a karots-pos bug: fixed here, and as a bug-fix
    commit there (Task 10).
  - **Exact to the cent.** Off per set `D` = round2(full × v / 100), `min(v, full)` or `max(0, full − v)`.
    Each unit gets a fixed per-unit discount `floor_cents(price × D / full)`. The leftover cents (fewer than
    the units in a set) go +0.01 onto units, splitting a line into two when it must (e.g. Cola ×3 at −10.00
    and Cola ×1 at −10.01). The lines always sum to exactly `D` per set. Pure function, unit tested.
  - Every combo line uses `discountType: 'fixed'` (per unit), so a group's count can scale it exactly (B).
- **B. A combo stays together in the cart.**
  - Its lines sit under one header: **🎁 Milk + Cookies ×1 · save Rs 40**, with − / + / ×.
  - − / + changes the number of sets (every line's qty = per-set qty × count). × removes the whole combo.
  - Member lines don't have their own qty or discount controls (serial boxes still work).
  - Removing one member asks **"Break the combo? The other items go back to full price."** Yes removes that
    line and clears the group and discount from the rest.
  - A normal scan of the same product never merges into a combo line. It gets its own full-price line.
  - The group survives hold / resume and a page reload (it's on the cart line).
- **C. The till spots combos.**
  - When the cart holds every member of a combo in its window as **plain lines** (not in a group, no
    discount, no picked lot, not a service), a strip appears above the total:
    **🎁 Milk + Cookies ×2 — save Rs 80 · Apply · ✕**.
  - ×N is how many full sets the cart holds. Apply takes those units off the plain lines (a line left at 0
    disappears) and adds the combo group, in one step.
  - ✕ hides that combo's hint until the next sale. Several combos can show; applying one re-checks the others.
- **D. Dates.** Optional **Starts** / **Ends** (dates, inclusive, shop's local date). Outside them a combo
  isn't on the till menu, isn't hinted and doesn't badge. Admin shows **Scheduled** / **Ended**.
- **E. Health.**
  - A member that's disabled (or deleted, which is a soft delete) or a service blocks the combo: the till
    panel says "Shampoo is disabled — can't add" with Add off, and the admin card flags it in red.
  - Out of stock is a yellow note only. Adding goes through the normal found-at-till prompt at checkout.
  - New members must be active, non-service products (checked on save). Old karots-pos combos that break
    this restore as they are and show the flag.
- **F. Sold count on each card:** "Sold 23× · Rs 4,600 · last 2 days ago", from `combo_sales` (G).
- **G. Combos report** (Reports → More → Combos):
  - A period picker, then one row per combo: **sets sold, sales (net), discount given, bills**, sorted by
    sets. Bar chart of sets per combo above the table (top 10). 360px: cards via the admin kit.
  - "Discount given" is what the shop spent on the promotion. Totals line at the foot. CSV export.
  - Counts from the day the plugin is in the suite. Voided sales are left out; returns aren't netted (a
    returned combo still counts as sold — noted on the page).

## 3. Core seams (generic; no plugin named)

1. **Line groups in the cart** (`web/src/features/till/cart`):
   - `CartLine.group?: { id: number; label: string; per: number }`. `id` is unique in the cart (a seq
     value), `per` is the line's qty in one set.
   - Reducer actions: `addGroup` (resolved products + per-set lines + count + `take` + `plugin`),
     `groupCount {id, count}`, `breakGroup {id, key}`, `removeGroup {id}`.
   - `addProduct` / `quickSet` never merge into a grouped line.
   - `CartPanel` renders grouped lines under a header (label, ×count, saving = Σ discount × qty, − + ×).
     The break confirm is the in-app dialog.
2. **TillApi** grows `saleType` and
   `addGroup({ label, count, items: [{ productId, per, discount }], take?: [{ key, qty }], plugin? }): Promise<string | null>`.
   Core fetches each product (`/till/products/:id`, via the query cache). A product it can't get → the
   group isn't added and the error text is returned ("Shampoo is not available"). `plugin` lands on every
   line's `plugin` field, so `tillAfterSale` sees it.
3. **Slot `tillCartHints: { key, Hint: ComponentType<{ till: TillApi; lines: CartLine[] }> }`**, rendered
   above the cart total.

Nothing changes on the server for core: the sale posts lines with fixed per-unit discounts as today. A
core-only build renders no hints and never creates a group.

## 4. Data and old backups

- karots-pos `0001_combos.sql` copied **byte for byte**.
- **New `0002_window_sales.sql`:**
  - `combos.starts_on DATE NULL`, `combos.ends_on DATE NULL`.
  - `combo_sales (id, combo_id BIGINT NULL REFERENCES combos ON DELETE SET NULL, name TEXT, sale_id BIGINT
    NOT NULL, sets NUMERIC, net NUMERIC, discount NUMERIC, created_at)`, unique `(sale_id, combo_id)`.
    Name is a snapshot, so a deleted combo's history stays in the report.
- Data group `combos`: `combos`, `combo_items`, `combo_sales`.
- **A karots-pos backup restores with zero mapping:** no dates (always on), the old `percent` / `fixed`
  types mean the same, no sales history.

## 5. API (`/api/v1/x/combos`)

| call | who | does |
|---|---|---|
| `GET /` | admin, manager | combos with members (name, qty, retail price, active, service, stock), window, health, sold summary |
| `POST /` · `PUT /:id` | admin, manager | create / edit (name, type, value, starts, ends, items); ≥ 2 units, active non-service members, price type needs value > 0; audited |
| `POST /:id/active` `{active}` · `DELETE /:id` | admin, manager | as karots-pos; audited |
| `GET /till` | cashier+ | combos in their window with members incl. `sellingPrice`, `wholesalePrice`, `active`, `isService`, `stock` |
| `POST /sold` `{saleId, combos: [{comboId, sets, net, discount}]}` | cashier+ | records `combo_sales` (sale must exist; upsert on the unique key) |
| `GET /report?from=&to=` | admin, manager | rows + totals for G |

## 6. Testing

- **Go:** store DB tests (create / edit validation, window filter, health flags, badge only in window,
  sold upsert and report excluding voided sales); API tests (403 for a cashier on admin routes); restore test
  with a karots-pos-shaped `combos` / `combo_items` archive.
- **Web unit:** the allocation (each type, retail and wholesale, cents split, 100% off, sums exact × count);
  the hint matcher (sets count, skips grouped / discounted / lot / service lines); reducer group actions
  (add with take, count, break, remove, no merge, hold round-trip).
- **e2e** (`plugin-combos.spec.ts`): create a combo price deal in admin → till menu Add → group header,
  + to ×2, totals exact → complete; scan the members one by one → hint → Apply; break a combo; a disabled
  member blocks Add; dates hide a combo; report shows the sale; 360px admin.

## 7. Out of scope

- Mix-and-match ("any 3 from this category") — a different engine; revisit if a shop asks.
- Automatic apply without the cashier tapping Apply.
- Returns netted out of the report.
- Showing the combo name on the receipt (receipt layout is final).
