# Stock freeze — design (2026-11-14)

## Why

A shop that is onboarding can't close for a stock take. The open boxes on the front shelf sell all day, so any count
of them is wrong by the time it's finished, and a GRN taken meanwhile gets eaten by sales of the old shelf stock.
Today a product with no stock makes the till ask for every sale ("found at till"), so a new shop (Iman: every product
came in at 0) is either nagged all day or fills up with corrections that mean nothing.

The user's idea: **count only what doesn't move** (closed boxes and sacks in the back, sealed medicine boxes), and
**freeze** stock: the till sells everything without asking and without reducing stock, while counts, GRNs and
adjustments still work. When a product's front shelf is nearly empty, someone **unfreezes** it, saying how many are
on the shelf, and from then on its sales reduce stock as normal.

Example (user): 20 boxes × 10 pencils = 200 counted, front not counted, frozen. Pencils sell; stock stays 200. A box
is opened → stock adjustment −10 → 190; those 10 sell without touching stock. Later, 3 left on the front + a new box
of 10 brought out → unfreeze, "13 on the shelf" → stock 190 + 13 = 203, and sales reduce it from now on.

## Goals

- A product can be **frozen**: its sales (till, Pocket Cashier, offline replay) never reduce its stock and never ask
  about stock.
- While frozen, everything that **counts** stock keeps working: GRN / receiving, stock take, adjustments, opening a
  box ("take from stock"), return to supplier.
- **Unfreeze one product**: sales person, cashier or admin; asks "How many are on the shelf now?" (0 allowed) and adds
  that to stock. That is the only shelf count the feature ever asks for.
- **Freeze / unfreeze the whole shop** (admin). Freezing marks every product frozen and makes **new products start
  frozen** until the shop is unfrozen. Unfreeze-all warns with the list of still-frozen products first (the admin may
  have counted the front shelves after closing).
- A **list of frozen products** (how far onboarding has got).
- Nothing becomes invisible: a damaged frozen item is still a write-off with its cost.

## Non-goals

- No "front" / "back" places stored. The stock number is what was counted plus GRNs; the shelf is simply not counted.
- No splitting a sale line into "front" and "back" quantities at the till (error-prone at a busy counter). Fetching 2
  from the back for a sale of 4 = a separate "take from stock 2" (counter tab, or later Shelf).
- No automatic unfreeze. People decide when.

## Anti-goals

- The till must never stop or nag a sale of a frozen product (user rule: never stop a sale).
- Core must not know a plugin. Plugins that can't allow freezing say so through a hook.

## Approaches considered

1. **Per-product freeze flag checked in the shared stock functions** (chosen). One column on products, one check where
   sales take stock. Small, and every caller (core and plugins) follows it.
2. A shop-wide "onboarding mode" only, ended on one day: simpler, but every product with open stock left that day
   starts wrong. The user wants product by product, plus the shop switch.
3. Front / back as two stored quantities: models the shop more literally, but every screen, report and plugin would
   have to learn two numbers. Rejected as too heavy for a temporary state.

## How it behaves (decided with the user)

| Where stock changes | Product frozen |
|---|---|
| Till sale, Pocket Cashier, offline replay, warranty replacement | Sold; stock and lots not reduced; no "not enough stock" question, no `oversold` flag; line cost = product cost price |
| Service materials (a copy job's paper), Documents plugin copy jobs | Material not reduced (the open ream is "the shelf"); still recorded as the plugin records it today |
| Combos / line groups | Each frozen part not reduced; Combos' "only N in stock" warning skipped for frozen parts |
| GRN / receiving, stock take, adjustment, "take from stock" | Normal |
| Conversion (sack → packs) | Source taken from stock as normal; target not added if the target product is frozen |
| Return to supplier | Normal (goods come from the counted stock) |
| Customer return / void | Added back only if the product is **not** frozen now (back on the shelf, which isn't counted while frozen) |
| Write-off (till Write-off tab, damage) | Asks **"From the shelf or the back?"** Both are recorded on the write-off list with cost; only "back" reduces stock |
| Expiry lots (pharmacy) | Not used up by sales; the till still asks which price when lots differ (price matters, count doesn't) |
| Reorder, low stock | Frozen products marked, not alerted |
| Reports | Profit uses the product cost price for frozen lines; stock value says "frozen products: counted stock only" |
| Old backups | Restore as not frozen |

**Plugins that can't freeze:** a new core hook, e.g. `plugin.FreezeGuard` (`CanFreeze(ctx, productID) (ok bool,
reason string)`), asked when freezing one product or all. **Pharmacy** refuses controlled drugs (its register keeps a
running balance; they're counted in full on day one). Freeze-all skips refused products and says how many and why.

**Unfreeze permission:** sales person, cashier, admin (user). Lives where those people already work: the till's
counter tab (find product → Unfreeze / Take from stock) and the admin product / stock screens.

**Onboard (the user's app):** can push products to a shop already frozen (a field on the push), so a new shop starts
in this mode from day one. Separate repo; follows once the suite side is done.

## Constraints

- Migration `00035_` (next free). Product flag + shop setting "new products start frozen".
- Money still only through `cashflow`; sales stay the source of truth for revenue (unchanged: only stock is skipped).
- Pocket Cashier: offline catalogue must carry the flag so the phone doesn't ask either; the change goes in Pocket
  Cashier's own offline route / data, never in a plugin (hard rule). Not shipped to shops before release 2 anyway.
- Every admin screen works at 360px; theme tokens only.
- Docs move with it: `docs/guide/stock.md`, `till.md`, the pharmacy plugin page (controlled drugs can't freeze),
  screenshots for the new screens (`shots.spec.ts`), Demo Room data only if a shot needs it.

## Left to the builder

- Column and setting names, DTO shapes, exact screens and wording (plain, short, like the rest of the till).
- How the till shows a frozen product (a small badge; no stock number shown while frozen).
- How "take from stock" is booked (an adjustment movement with its own type or reason, so the history reads
  "opened a box").
- Whether the frozen list is its own page or a filter on the stock page.
