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
  and change · `RET` returns · `TILL` the till's screen vs the server · `DRAW` the drawer · `LOCK` lockers and moves · `EXP` expenses · `CF` the Cash flow page (area 2) · `CUST` customers' balances, payments and advances (area 3) · `STK` stock, lots, counts, write-offs, conversions, freeze (area 4) · `PUR` orders, deliveries, bills, goods sent back · `SUP` supplier balances and money · `CHQ` cheques given and received (area 5) · `WAR` warranty replacements · `REC` recoveries: what a supplier makes good on a loss (area 6).
- **Checked by** — `ledgercheck` (the auditor sees it in the database, `internal/ledgercheck`), `machine` (random
  shop days, `internal/battle`), `lean` (proved in `proofs/`), `example` (a fixed case test).
- A rule changes only when the user agrees; the commit says so.

## Proved in Lean (`proofs/`, `make proofs`)

| Proof | Rules | What it shows for every possible amount |
|---|---|---|
| `Karots/Refund.lean` | RET-4, RET-7 | A line returned in any parts never refunds more than was paid; all back = exactly what was paid |
| `Karots/Share.lean` | DISC-5, RET-5, SALE-35 | Sharing an amount over lines: rounded-down shares never pass it and fall short by fewer cents than there are lines, so the shares add up exactly, each within a cent |

`lake exe cases` writes `proofs/cases/*.json` from the proven models; the Go server (`sales/lean_cases_test.go`) and
the till (`counter/returns/lean.cases.test.ts`) must give every case to the cent. GitHub Actions (`proofs.yml`)
rebuilds the proofs and fails if the cases are stale.

## Areas

1. [Sales, discounts, payment](sales.md) · [Returns](returns.md) — area 1
2. [Drawer, cash flow, lockers, expenses](money.md) — area 2
3. [Customers: credit, advances, opening balances, payments](customers.md) — area 3
4. [Stock: lots, packs, units, conversions, freeze, write-offs](stock.md) — area 4
5. [Purchasing, supplier pay, supplier returns, cheques](purchasing.md) — area 5
6. [Recovery and losses: warranty replacements, what a supplier makes good](recovery.md) — area 6
7. Reports and the Z report — area 7
