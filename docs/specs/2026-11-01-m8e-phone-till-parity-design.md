# M8e release 1, part 2 — the phone sells like the PC till

**Status:** approved in chat 2026-11-01 ("looks right, write the plan"). Part of M8e release 1: release 1 is not
done, and `m8e-pocket-cashier` is not merged, until this is in.

## Why

Testing on a real phone showed the Pocket Cashier's Sell and Pay are far thinner than the PC till: no discounts, no
weights, one payment, no note tray, one price per product. The user's rule: **the phone does exactly what the PC
cashier does, in a phone format.** The layout is free (it is a different page, designed for one hand), the
behaviour is the PC till's.

## Goals

Everything the PC Sell page does for ordinary selling, on the phone, live and offline:

**Cart**
1. Discount per line: fixed (per unit) or percent, as the PC.
2. Discount on the whole bill: fixed or percent.
3. Typed quantity, decimals where the product allows them (loose items by the kg).
4. "Which price?" when a product's lots sell at different prices (or the lot that would sell is expired), as the PC.
5. Choosing a pack (Card ×12…) when adding, as well as scanning a pack's code.
6. Serial numbers for serial-tracked products, as many as the quantity.
7. Service lines: price and description, as the PC.
8. Retail / wholesale switch for the sale.
9. Quick item: sell something not in the catalogue yet, created for admin review — live only. Many shops never
   catalogue everything, so it must be one tap away: a **"+ Quick item" button beside the search box**, and offered by
   itself when a scan or search finds nothing. A scanned code becomes its barcode; with no barcode the cashier types
   the name and price (the barcode can stay blank or be generated). The phone can scan the barcode with the camera.
10. Product info (ⓘ): what the card knows; cost only for people allowed to see it.

**Paying**
11. Several payments on one sale (add tender lines, remove them), as the PC.
12. The note tray: tap the notes and coins the customer hands over (the shop's own Denominations), undo, clear.
13. A keypad for amounts.
14. The full quick-cash guesses (`quickCash`, up to 8).
15. Keep the change (no coins) and round the kept change to the customer's account, as the PC.
16. Change to give, broken down note by note and coin by coin (`notesFor`, the shop's notes and coins).
17. Add a new customer — live only.
18. The picked customer's balance and credit limit, as the PC shows them.
19. Online (bank) payments into a bank account, as well as card into a card account.

## The PC till too (core first)

The PC till has the Quick item form but reaches it only by pressing Enter on a search that found nothing. It gets the
same **"+ Quick item" button beside its search box** (as karots-pos had), built in core before the phone's. Nothing
else on the PC changes.

## Non-goals (this round)

- Plugin menus, tabs, swaps and suggestions on the phone (release 2; Pocket Cashier adapts from its own side —
  AGENTS hard rule). Plugin tenders already work.
- Printing from the phone (still: online, the receipt prints at the counter).
- The PC's keyboard shortcuts (F-keys).

## Anti-goals

- Re-implementing pricing on the phone. The phone keeps using the PC till's cart reducer and logic
  (`web/src/features/till/cart/*`): same totals, discounts, packs, lots, rounding. A phone total that differs from
  the PC's for the same cart is a bug.
- Stopping a sale. Anything the PC would ask about (oversell, over the limit, a shortfall) is taken and flagged by
  the server, as now. The phone may *show* the same warnings the PC shows, but Finish always goes through.
- A cramped copy of the PC screen. One-handed, thumb-reachable, sheets over the cart rather than tiny inline fields.

## Constraints

- Release 1 rules still hold (spec `2026-10-29-m8e-pocket-cashier-design.md`): never pops the drawer, one
  person one device, offline actions applied in order and flagged, files ≤ 500 lines, theme tokens only, every
  palette light and dark, 390 px phone screens, core plugin-inert.
- Offline: everything except quick item and new customer works from the copy. Those two need the PC (the server
  makes the record); offline they say so in place.
- The server already accepts every field the phone needs on a sale (`SaleItem`: discount, discountType,
  priceOverride, batchId, serials, description; `SaleBody`: bill discount, changeGiven, onAccountRounding, several
  payments). The phone sends the PC till's own request (`buildSaleRequest`) plus `expectPrice` and `total`.

## What the server needs (decided)

- **Lot prices in the copy:** `TillCopy.priceOptions` = what `/till/price-options` answers (product id → lots), so
  Which price? works offline. Part of the copy's version.
- **Shop notes and coins in the copy:** `TillCopy.denominations` (face value + note/coin), for the note tray and the
  change breakdown offline.
- **Bank accounts:** already in `copy.places` (kind `bank`); the phone offers Online when one is cashier-accessible.
- **Serials never refuse a phone sale:** in `Review` mode, a serial already on record or repeated in the sale is
  kept and flagged (new flag kind `serial`, note names the serial), not refused. A missing serial count is also
  flagged, not refused. Outside Review nothing changes.
- **Line `expectPrice`** stays the unit price the phone showed (lot price when a lot was picked).

## Left to the builder

- Screen layout: how the cart line opens (tap → a line sheet with qty keypad, discount, serials, price for
  services), where bill discount and retail/wholesale live (a cart summary sheet is the suggestion), how the note
  tray / keypad share the Pay page.
- Which PC components are reused as-is versus rebuilt for the phone (NoteTray, TouchPad/TouchKeys, PricePick,
  PackPick, CustomerPicker/AddCustomer, ChangeBanner logic). Reuse logic always; reuse markup where it fits 390 px.
- Tests: vitest for each new pure piece; the e2e spec gains a parity flow (line discount + bill discount + a
  weighed item + Which price + split cash/card + keep change) checked against the same cart's PC total.
