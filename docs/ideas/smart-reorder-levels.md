# Smart reorder levels (planned 2026-10-20 — build next session, before Arch / Demo Room)

Planning only so far (user + Claude, chat). Core feature, not a plugin: low stock, Reorder and `reorder_level` are
core, and a plugin would have to change what core calls "low stock".

## The problem (user)

`reorder_level` is a fixed number typed once. Low stock lists everything under it (or at zero), including items
that hardly sell, so the order list rebuys slow movers; fast sellers (a pencil selling 10 a day with level 10) run out.
The Reorder page already shows sold week / month / last year and suggests a qty from 90-day sales × 14-day lead.

## Agreed in chat

1. **Fixed or Auto per product.** Fixed = today's typed number. Auto = calculated. Existing products stay Fixed on
   update (nothing shifts under a shop); new products start Auto.
2. **Auto starts at the typed number and adapts gradually** as history builds (not a switch after N days): day 1 =
   100 % the typed level, sliding toward the calculated one (guess: fully learned after ~8 weeks of sales and a few
   deliveries). Shown as "Auto · learning (3 weeks)" / "Auto: 100 (sells 10 a day, supplier every 7 days)". Recalculated
   nightly.
3. **Level = typical daily sales × (supplier rhythm + safety days).**
4. **Supplier rhythm is learned from recorded deliveries (GRNs)** of the product's preferred supplier — weekly
   suppliers vs irregular ones; for irregular ones plan for the longest recent gap; too little history / no supplier
   → a shop-wide default (Settings, e.g. 7 days). No setup asked of the shop.
5. **Look ahead, with last year:** the level covers what will sell until the next delivery; use the higher of recent
   speed and the **same coming weeks last year** (exercise books: supplier comes 20 Nov, last year 20 Nov–1 Dec sold
   15/day → order for 15/day, not today's 2). Restored karots-pos history counts. The Reorder page says why ("Last
   year this time: 15 a day · now 2 a day"). The suggested order qty uses the same look-ahead.
4b. **Most products are added with no supplier (user, 2026-10-21)** — the first delivery stamps it
   (`SetPreferredSupplierIfEmpty`, never overwrites), and products are often quick-added on the receive screen
   (`LineEditor` "New product"). So the rhythm comes from **the product's own deliveries, from any supplier**
   (the preferred-supplier stamp is just whoever came first and may be stale); not enough of those → that
   supplier's rhythm across all its deliveries → the shop default. A quick-added product has no typed level:
   Auto starts from the shop default rhythm × early sales instead of a typed number.
6. **One-off big bills don't count** (user: "randomly someone might buy a large qty — this shouldn't affect"): each
   product learns its normal bill size; a bill far above it (guess > 3×) counts as a normal-sized bill for the speed
   only — revenue, stock, reports untouched. Regular big buyers raise the normal size, so real demand still counts.
7. **Slow sellers drop out of low stock by themselves** (their Auto level falls); Reorder tags "Not selling" rather
   than hiding silently.
8. **Fixed products** are flagged for review only when their number looks badly off (suggested vs current, Apply /
   Apply all — owner decides).

## Still open

- Where the owner steers it: product form (Fixed / Auto + reason), Reorder page (reason per suggestion), Items to
  review (badly-off Fixed ones) — and a whole-category switch to Auto?
- Exact numbers: safety days, learning period, the outlier factor, recent window (30 d with 90 d fallback?).
- Products sold by packs / loose versions / units: speed in base units.
- Dashboard "low stock" count follows the new rule.

Next step: rethink against karots-pos's reorder code + memory, then a spec + mockup for the user.
