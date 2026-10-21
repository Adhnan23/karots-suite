# Smart reorder levels — design (2026-10-21)

Source: `docs/ideas/smart-reorder-levels.md` (agreed in chat 2026-10-20/21). Mockup: https://claude.ai/artifact/CLQRCFzkRee981Yxi6BPcE (Reorder desktop · Levels look off · product form states · Settings → Stock).
Core feature (low stock, Reorder and `reorder_level` are core); no plugin and no new hook.

## Why

`reorder_level` is a number typed once (often 0, often never touched — most products arrive by quick add on the
receive screen with no level and no supplier). Low stock then rebuys slow movers and lets fast sellers (a pencil at
10 a day with level 10) run out. The shop should get a level that follows how the product actually sells and how
often its supplier actually comes, without setting anything up.

**Success:** the Reorder list is "what will run out before the next delivery", not "everything under a number".
**Anti-goals:** levels jumping around under a shop that didn't ask for it; one big customer bill inflating a level;
anything that blocks a sale or a delivery; a setup screen the shop has to fill in.

## Goals / non-goals

- Goals: Fixed or Auto per product; Auto learned nightly from sales speed, supplier rhythm and last year; one-off big
  bills ignored; Reorder page says *why*; Fixed levels that look badly off are offered a fix (owner decides).
- Non-goals: forecasting beyond "until the next delivery"; seasonality other than "same weeks last year"; the
  Alternatives plugin's tier levels (stay typed); automatic purchase orders.

## Data (migration `00008_reorder_auto.sql`)

```
products.reorder_auto   BOOLEAN NOT NULL DEFAULT false   -- existing products stay Fixed
products.reorder_start  INT     NOT NULL DEFAULT 0       -- the typed number Auto starts from
products.reorder_calc   JSONB                            -- last nightly result (both modes), NULL = never run
settings.reorder_default_days INT NOT NULL DEFAULT 7     -- supplier rhythm when nothing is known
settings.reorder_safety_days  INT NOT NULL DEFAULT 3
settings.reorder_ran_at TIMESTAMPTZ                      -- last nightly run
```

**`reorder_level` stays the effective level.** For Auto products the nightly run writes it. Every existing reader
(products list `LowStock`, reports low stock, product groups, till `lowStock`, dashboard count, low-stock print,
Topup) keeps working unchanged — that is the point of this shape.

`reorder_calc` = `{daily, recentDaily, lastYearDaily, rhythmDays, rhythmFrom: "product"|"supplier"|"default",
regular, safetyDays, calcLevel, learnedPct, historyDays, notSelling, cappedBills}` — the UI builds every reason line
from it. Old backups (suite or karots-pos) restore with the defaults → Fixed, no calc until the next run.

**New products start Auto, existing ones stay Fixed:** the migration adds the column `DEFAULT false` (every
existing row Fixed), then `SET DEFAULT true` — so every create path (product form, counter / purchase receive quick
add, import, plugins) makes Auto rows with no code change. `reorder_start` = the number typed (usually 0) — the
create insert writes it alongside `reorder_level`. A loose version copies its source's mode. Services ignore it.
**Import:** a reorder number on an Auto product updates its `reorder_start` only (stays Auto), so an export →
import round trip never freezes learned levels.

## The calculation (per active, non-service product, nightly)

All quantities in the product's base unit (packs already sell in units).

1. **Use per day.** Sales lines (`quantity − returned_qty`, sale not void), one figure per sale per product, plus
   conversion runs out of this product (a bag opened into loose rice is use of the bag, not capped).
2. **One-off big bills.** Normal bill size = the 75th percentile of the product's per-sale quantity over 180 days
   (needs ≥ 5 sales, else no capping). A sale above **3 ×** that counts as the median for speed only. Revenue, stock
   and reports are untouched. A regular big buyer raises the percentile, so real demand still counts.
3. **Recent speed** = use in the last **30 days ÷ days it was in stock** in that window (stock rebuilt backwards from
   today's quantity through `stock_movements`). A fast seller that ran out must not look slow. Fewer than 10 in-stock
   days → use 90 days.
4. **Supplier rhythm** (days between deliveries):
   - the **product's own deliveries** (`stock_movements` type `purchase`, one per day) in the last 180 days, from any
     supplier — the preferred-supplier stamp is only whoever came first;
   - fewer than 3 deliveries → its preferred supplier's deliveries of anything (same rule);
   - nothing → `reorder_default_days`.
   Regular (longest of the last 4 gaps ≤ 1.5 × median gap) → median gap. Irregular → the longest of the last 4 gaps.
5. **Last year.** Use per day over the same window one year back: from today−1y to today−1y + rhythm + safety days
   (in-stock days rule as 3; restored karots-pos history counts). `daily = max(recent, lastYear)`.
6. **Level** `calcLevel = ceil(daily × (rhythm + safety))`. No use in 90 days and nothing last year →
   `calcLevel = 0`, `notSelling = true`.
7. **Learning** (Auto only). `w = min(1, historyDays / 56)`, `historyDays` = days since the first sale or delivery
   (or since created). `reorder_level = round(start × (1 − w) + calcLevel × w)`. `start = 0` (nothing typed) →
   `w = 1` from day one.
8. **Fixed products** get `reorder_calc` too (never their level). "Looks off" = `learnedPct = 100` and
   (`calcLevel ≥ 2 × level` or `calcLevel ≤ level / 2`) and they differ by ≥ 3 units.

**When it runs:** a background loop like the backup scheduler — on boot if `reorder_ran_at` is over 20 h old (after
1 min), then daily at 03:00 local. One product is recalculated at once when it is switched to Auto, so the form shows
a value immediately. Level changes by the run are not audited (noise); mode switches are.

**Suggested order on Reorder** uses the same numbers: `ceil(level + daily × rhythm − onHand)` (cover until the
delivery after the next), ≥ 0; plugin annotator suggestions still win when bigger. No `reorder_calc` and no sales →
today's rule (`2 × level − onHand`).

## Screens (mockup: Studio + Night, 360 px and desktop)

- **Product form → Reorder level.** Segmented **Auto | Fixed**.
  - Auto: the level read-only, with its reason: "Auto · 100 — sells 10 a day, supplier every 7 days, +3 safety days" or
    "Auto · learning (3 of 8 weeks) — 60 for now". A "Starting guess" box only while learning.
  - Fixed: the number box (as today). If it looks off: "Sales suggest 40 · Use 40".
  - Switching Fixed → Auto starts from the current number; Auto → Fixed keeps the current level as the number.
- **Reorder page** rows get a one-line reason under the name: "Auto 100 · 10 a day · every 7 days", "Last year this
  time: 15 a day · now 2 a day", "1 big bill ignored", a **Not selling** tag (no sales 90 days; still listed at 0
  stock, never hidden silently). A **Levels look off (N)** chip filters to Fixed products that look off (low or not),
  each with **Use N**, plus **Use all** and **Switch these to Auto instead**.
- **Settings → Stock:** "Supplier comes every [7] days when we don't know", "Safety days [3]", and **Switch to Auto**:
  category picker (with sub-categories) or "all products" → "Switch 124 Fixed products to Auto" with a confirm.
- **Dashboard low-stock count** follows on its own (it reads `reorder_level`).

## Left to the builder

File / package names (suggest `internal/features/reorder`), SQL shape (one pass per batch of products, not per
product round-trips), the DTO fields (`reorderAuto`, `reorderStart`, `reorderCalc`, setting keys), test data. The
exact numbers above (30/90/180 days, 56-day learning, 3×, 75th percentile, 1.5×, ≥ 3 units) are my calls and live
as constants in one place.

## Acceptance

- Go tests: capping a one-off bill; a regular big buyer not capped; out-of-stock days don't slow the speed; regular
  vs irregular rhythm; product → supplier → default fallback; last-year window wins; learning blend; Fixed never
  written; conversion use counts for the bag.
- Playwright: new product is Auto; Fixed → Auto shows a level and reason; Reorder shows the reason line, Not selling,
  Levels look off → Use; Settings switch a category to Auto.
- Old backup restores as Fixed. Screens checked in all four palette/mode combinations and at 360 px.
