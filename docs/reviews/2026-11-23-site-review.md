# Site review (2026-11-23) — input for the next session

An outside review of suite.karots.lk (ChatGPT, given to us by the user on 2026-11-23). **Not yet checked against the
code**: the next session audits it (evidence first, nothing changed until the audit is done), then fixes the site and
docs. Delete this file once the audit and its fixes are done.

## The user's commercial policies (authoritative — the site and legal pack must match these, never the reverse)

- One-time purchase; a perpetual right to use it under the licence; one device / installation per licence.
- Free move to a replacement PC, when the shop tells us and we arrange it.
- Three-month warranty at the start.
- **A genuine bug in Karots Suite is always fixed free**, warranty or service plan or not.
- New features are not automatically included after the first three months.
- Technical support without a service plan is charged.
- Optional service plan Rs 5,000 / month (Rs 60,000 / year): cloud backups, technical support, major upgrades, small
  feature changes, ongoing maintenance. Large new features and custom work are quoted separately.
- When the plan ends, new cloud backups stop; the last one stays the latest; local backups carry on, the shop's own.

## What the review flags (its priority order)

1. **Power cuts:** the homepage says it "keeps selling when the internet or the power goes"; the FAQ says completed
   sales survive and the till comes back. A PC without a UPS can't sell with no power. Say what's true: internet
   doesn't matter; a power cut loses nothing completed (battle-tested 2026-11-22: confirmed sales survive, in-flight
   ones vanish whole, up in 7–9 s); a UPS keeps it selling.
2. **Testing claims:** "every rule about money and stock is checked … before any update reaches you" and "built
   never to get money and stock wrong" — check against `docs/rules/`, the battle tests, power-cut tests, migrations
   from older versions, plugins, and whether tests really run before every Release. Word it as what we test.
3. **Service plan:** the pricing page says "every new feature as it comes out", "help any time", "and more" — that
   contradicts the policy above.
4. **Free bug fixes forever** isn't on the site; only the three-month warranty is, which reads as "pay after three
   months". Add a who-pays table (bug / new feature / custom work / support without a plan / hardware).
5. **Licence terms** aren't explained on the site: perpetual, one PC, reinstall after a format, a broken PC, moving,
   two PCs at once, who owns the software, copying, what if Karots Korp stops. Lawyer review for the licence itself.
6. **Cloud backup when the plan ends:** how long the last one is kept, download / restore after, reactivating.
7. **Backups explained:** local (where, how often), the copy folder (USB / network — set up by whom), cloud,
   restoring after a dead PC; a copy on the same disk doesn't survive that disk; encryption.
8. **Hardware:** publish tested minimums (RAM, disk, Windows / Linux versions, network for cashier PCs, UPS) —
   only from real tests, never invented.
9. **Karots Shelf permissions (security, not wording):** the guide says Shelf ignores other permissions, cost prices
   included, so a cashier without "see cost" sees it on Shelf. Check it, whether turning Shelf off ends a session at
   once, and what it audits. A real fix goes through the rule book and the user's yes.
10. **Pharmacy:** "controlled-drug register" must not read as meeting Sri Lankan regulations unless checked.
11. **AI Assistant privacy:** what gets pasted into a free chatbot (customer names, phones, prices?); a short warning.
12. **Printers:** tested (XP-80T on Linux, a Windows test) vs expected to work vs untested; connection types.

Also worth a line each in the guides (not bugs): selling below stock / no price is never blocked but needs a manager's
review; who reviews quick items; card / online refunds recorded as cash; opening balances and advances; credit over
the limit; negative stock and fixing it; imports with bad rows or duplicate barcodes; an update that fails after
migrating; the audit log; two tills selling the last unit.

The review also says: keep the detailed everyday guides — the risk is the promises, not missing features.
