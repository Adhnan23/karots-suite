# Questions for the user

Squares in the grid marked **?** — only the shop owner's way of working can answer them. Each says what the suite does
**today**. Answered ones move into the rule (with the date) and leave this list.

## Area 1 — sales and returns

All five answered 2026-11-18 (RET-13, RET-16, RET-17, RET-19, SALE-35).

## Area 2 — drawer, cash flow, lockers, expenses

- **Q1 (DRAW-3) Cash out beyond what the drawer holds.** Today a withdrawal, a cash refund, an expense, supplier cash
  or an advance refund bigger than the drawer's expected cash is **refused** ("the till only has Rs …"), but a sale's
  change or a payout is never refused, so the expected can go below zero through a sale. Should a **cash refund to a
  customer** also never be stopped (refund, then flag the till for review), like a sale? Withdrawals and expenses would
  stay refused — staff can take those from a locker.
- **Q2 (DRAW-6) A cash sale when the cashier has no open till.** Today the server takes it (an offline sale sent after
  the till was closed; a phone till), but the cash lands in **no drawer**: no close counts it, so that close looks
  short or over by it with no reason shown. Proposal: put it in the till the cashier had open when it was rung; if
  that till is already closed, in the next one they open, and flag both for review with the sale's number.
- **Q3 (CF-2) Cash taken out into nowhere.** With "untracked cash" on, a close or withdrawal with no locker leaves no
  row: cash on hand falls, but the Cash flow page shows no money out. Should it show as **money out** ("taken
  out, not tracked") so the page's net still matches what the shop holds?
