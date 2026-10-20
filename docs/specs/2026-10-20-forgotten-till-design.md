# A till left open — warn, close & reopen, close for them (design, 2026-10-20)

## Why

The user: a cashier in a hurry goes home without closing their till. Today it just stays open — the next day's sales
land in yesterday's session, the Z report spans two days and the overnight cash is never counted. Agreed in chat:
**warn**, a till from an earlier day **must be closed and opened again**, and an **admin or manager can close it for
them**. The user stressed this is a crucial (money) section.

## Money rules that do NOT change

- A close is exactly today's `cashregister.Close`: expected = opening float + cash sales since `opened_at` (by that
  cashier, `sales.CashCollectedSince`) + drawer moves; counted − expected = over / short; the counted cash is banked
  into the chosen locker (or left in the drawer when untracked cash is allowed) through the same locker leg + money
  receipt. Same validation (a locker is required unless untracked cash is allowed).
- A close for someone else runs that **same code with the session owner as the user**, so the ledger rows, the
  receipt and every report read it exactly like the cashier's own close. Nothing about the amounts depends on who
  presses the button.
- Open is untouched (float, source locker, "Continue last count" when the setting allows it).
- No sale is ever refused by the server because of this. The sale path stays as it is.

## What is added

1. **Stale till** = an open session opened before today (shop day, `shopDay`) **and** at least 6 hours ago (so a shop
   open past midnight isn't stopped at 00:00). ponytail: fixed 6 h; a setting if a night shop needs it.
2. **Warn**
   - Dashboard → Needs your attention: "Kamal's till open since yesterday 18:02" (one per stale till), → Till sessions.
   - Who's on now: "Till open since yesterday 18:02" in warning colour.
3. **At the till (the cashier's own stale till):** when the cart is **empty**, the till shows the normal Close dialog
   with a note — "Your till from yesterday (opened 18:02) was never closed. Count the drawer to close it, then open
   today's." — and can't be dismissed. Close done → the normal Open dialog appears (as it does today with no session),
   where "Continue last count" offers the count just made (when that setting is on). A cart already in progress is
   never interrupted: the prompt waits until that sale is done.
4. **Close for them (admin + manager):** Till sessions → an open session → **Close for them**: the same count dialog
   (notes and coins, bank into locker). Records:
   - new column `cash_register.closed_by` (nullable user id; NULL = closed by its owner — every old row and every
     restored older backup reads correctly);
   - Activity: "closed Kamal's till for them: counted …, expected …, over/short …" under the admin's / manager's name;
   - Till sessions / Z report show "Closed by Admin for Kamal" and "closed late" when it closed on a later day.
   - **Refused while that cashier is serving** (Who's on now: cart changed in the last 5 min) — the same rule as the
     remote Lock, so a sale is never cut off mid-way.
5. If a cashier's till is closed for them while their screen is open, the till sees no session on its next refresh and
   shows the Open dialog, as it does today.

## Not doing

- No automatic close at night — money always needs a person to count or decide.
- No "close without counting" (not asked for). If the drawer can't be counted, don't close it yet.

## Known gap (exists today, not made worse)

The server accepts a sale with no open till (only the screen asks to open one). Such a sale's cash belongs to no
session's "expected". The serving check in (4) keeps "close for them" from creating that case.

## Sales with no till open (added, user agreed 2026-10-20)

Never refused (never stop a sale) — **flagged**: Dashboard → Needs your attention lists today's sales with cash taken
by a cashier who had no till open at that moment ("1 cash sale made with no till open — Kamal 10:42 · Rs 450"). Found
by query (no session of that cashier covers the sale's time); nothing stored. That cash sits in a drawer outside any
session, so the next opening count shows it as extra — the flag says why.
