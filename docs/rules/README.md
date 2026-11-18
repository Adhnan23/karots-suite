# The rule book

What must **always** be true in Karots Suite core, written in plain words, so tests can be measured against it
instead of against what the code happens to do. A rule nobody wrote down is a bug nobody can test for: the discount ×
return bug (`d17d7b7`) lived in such a gap for weeks.

## How it's built: the grid

Each area has a **grid**: every *feature* a record can carry (rows) × every *later action* on it (columns). Each square
is one of:

- a rule id (`RET-4`) — what must be true when that feature meets that action;
- `—` — it can't happen (the reason is given under the grid);
- **?** — nobody knows yet; it is asked in [questions.md](questions.md) and its test waits for the answer.

An empty square is a gap. New feature → a new row; new screen or report → a new column; fill every square before
the feature is done.

## A rule

```
RET-4  A line's refunds never add up to more than what was paid for that line.
       Tricky: returned in parts (1 + 1 + 1 of 3); bill discount shared; a pack price.
       Found in: d17d7b7 (refunded the sticker price).
       Checked by: ledgercheck, machine, lean.
```

- **Ids** never change meaning and are never reused. `SALE` selling and totals · `DISC` discounts · `TEND` payment
  and change · `RET` returns · `TILL` the till's screen vs the server · `DRAW` the drawer (area 2).
- **Checked by** — `ledgercheck` (the auditor sees it in the database, `internal/ledgercheck`), `machine` (random
  shop days, `internal/battle`), `lean` (proved in `proofs/`), `example` (a fixed case test).
- A rule changes only when the user agrees; the commit says so.

## Areas

1. [Sales, discounts, payment](sales.md) · [Returns](returns.md) — area 1
2. Drawer, cashflow, payouts, expenses, money sources — area 2
3. Customers: credit, advances, payments — area 3
4. Stock: lots, packs, units, conversions, freeze, write-offs — area 4
5. Purchasing, supplier pay, supplier returns, cheques — area 5
6. Recovery and losses — area 6
7. Reports and the Z report — area 7
