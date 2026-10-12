# Idea: Pocket Cashier (after karots-suite ships)

Captured 2026-10-12 from the user, so it isn't forgotten. **Not planned yet** — start with a rethink and a spec
once karots-suite (M8) is done.

## Why (user, 2026-10-12)

In Sri Lanka a shop runs no matter what. If the system ever stops a sale (power cut, dead PC, a refusing prompt), the
shop drops the system, not the sale. **The system must never be the reason a sale can't happen.** Pocket Cashier is
that rule for power cuts.

## The idea

A phone app that is a **backup till**. While it can reach the shop's backend (LAN IP, a VPS, or Tailscale), it keeps a
local copy of what selling needs and refreshes it **every minute**: products and prices, stock and lots, customers
and credit, users and PINs, open till sessions, settings, and which plugins the shop has (their till data too).

If the PC dies (power cut, broken machine), the cashier **carries on selling from the phone**, using the last copy.

When the PC is back, the phone **does not merge databases**. It sends what it did as a list of actions (sales,
returns, drawer moves, write-offs), oldest first, and the backend **applies them on top of its own state**.

> Example: the phone's last sync was 8:30. The PC kept working until 8:45, then the power went. When it's back, the
> PC keeps everything it has up to 8:45, then replays the phone's sales and returns from after the cut, as if a
> cashier had rung them in that order.

## Why replay, not merge

- The backend stays the only truth. Every replayed sale goes through the same `sales` / `cashflow.Move` code as a
  normal sale, so stock, FEFO cost, credit, the drawer and the P&L stay right. There's no second copy of the domain logic.
- Whatever the PC did between the phone's last sync and the power cut is never overwritten.

## Things to work out (when this becomes a plan)

- **Duplicates, two layers (user, 2026-10-12):**
  1. *Exact:* every phone action carries a unique id, so a replay that's interrupted and sent again never
     double-books — skipped silently, no prompt.
  2. *Look-alike:* a replayed sale that closely matches one the PC already has (same items and total, times a few
     minutes apart — e.g. rung on both during the cut) is applied but **flagged for review**, and admin decides
     (keep both, or void one). Two customers buying the same tea is real, so this never auto-deletes.
- **Stock that moved meanwhile:** a phone sale of something that has since run out on the PC → treat it like
  found-at-till (the sale is real; the count is corrected and flagged for review), never refuse it after the fact.
- **Receipt numbers:** the phone needs its own series (e.g. `P-…`) so it never clashes with the PC's `S-…`.
- **The drawer:** the phone works as its own till session (opening float, cash in hand), closed when the replay
  lands.
- **Prices or credit that changed meanwhile:** the phone sold at the price it showed; replay keeps that price and flags
  differences for review.
- **Sign-in offline:** cashiers' PIN hashes kept on the phone; the system user's hourly PIN must still work.
- **Plugins:** only what can work offline (Print & Copy, combos, clearance prices); anything that needs the server
  live (recharge float, AI catalog) is greyed out offline. Each plugin says which it is.
- **Printing:** a Bluetooth / USB thermal printer from the phone.
- **Security:** the local copy is encrypted; a lost phone can be revoked from the backend.
- **Review screen:** after a replay, admin sees what came in from the phone and anything flagged.

## Fits with the suite today

- It's a JSON-only `/api/v1` backend, so a phone client is just another client.
- Sales are the source of truth and money only moves through `cashflow.Move`, so replay reuses both.
- Found-at-till already exists for "sold more than the count showed".
