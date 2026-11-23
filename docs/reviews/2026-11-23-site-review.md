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

## Audit (2026-11-23, evidence from the code; nothing changed yet)

| # | Verdict | Evidence |
|---|---|---|
| 1 Power cuts | **Review is right.** Hero says "keeps selling when … the power goes" (`Hero.astro:12`). FAQ + Trust card are true. | `internal/battle/powercut_test.go` (real server killed mid-sale/return/payment/delivery; auditor clean), `powercut_install_test.go` (backups, updates). Fix: "a power cut loses no finished sale; a UPS keeps you selling". |
| 2 Testing claims | **Half right.** Rule book, auditor, battle days, power cuts, previous Release on the new schema (`upgrade_test.go` UPD-6) are real. "Before any update reaches you" is our habit (`make test-all` before a Release), **not enforced**: the Builder's Release runs no tests (`cmd/bootstrap/app_release.go`). "Never get money and stock wrong" is a promise; word it as what we test. |
| 3 Service plan | **Review is right.** `pricing.ts` SERVICE.gets = "Every new feature", "Help any time", "And more" — against the policy. |
| 4 Bugs free forever | **Missing everywhere, and the legal pack says the opposite:** LICENSE 9.2 + Second Schedule 1 = bugs and updates free **for the first year**, then paid; site says 3 months' warranty. Policy: 3 months, bugs always free. |
| 5 Licence terms | **Conflict.** LICENSE 3.1 = "as many of the Shop's own computers there [at the Site] as it needs"; policy = one device / installation per licence. Site's FAQ: "more counters can use more PCs". Nothing on the site about reinstall, moving (LICENSE 3.3 allows a move at the Site), ownership, Karots stopping. |
| 6 Cloud after the plan | **Conflict with policy.** Bucket lifecycle deletes every copy after **30 days** (`docs/cloud-backup-bucket.md` step 3) — so "the last one stays the latest" lasts 30 days. Stopping cloud = Builder untick + Release + Update (no remote switch). Second Schedule 3 freezes after 7 days late. |
| 7 Backups | Facts for the site: every 6 h, keep 28 (7 days) in `<install>/backups` (`cmd/bootstrap/profiles.go:77`); copy folder set in the Builder then the Data page; cloud sealed per shop key before upload (`internal/backup/seal.go`); local files and copies are **not** encrypted. Uninstall keeps a final backup. |
| 8 Hardware | **No minimums anywhere** (`docs/guide/install.md`). Only real data: Iman's i3 4th gen / 8 GB / 120 GB SSD, Xubuntu VMs, a Windows 10 VM. |
| 9 Shelf permissions | **Confirmed.** A cashier with the Shelf tick reaches `GET products/export`, `GET/PUT/DELETE products/:id`, `stock/adjust` … (`internal/api/shelf.go` shelfCore) with no `can_see_cost` check: sees and sets cost, can **delete products** and adjust stock. Untick works at once (gate re-checks the flag every request). Writes land in Activity under the person. |
| 10 Pharmacy | No compliance claim is made, but none is denied either. Add one line: "a record to help you, not a check that you meet NMRA rules". |
| 11 AI privacy | Prompts carry product names, barcodes, categories and the shop's custom instructions only — no customers, patients, phones or prices (`plugins/aicatalog/prompts.go`, `job_*.go`). Site can say so plainly. |
| 12 Printers | `docs/guide/printing.md` already lists tested (XP-80T, USB, Linux; Win10 Generic/Text Only test slip). Site FAQ says "common … printers" with no tested list; XP-365B labels untested. |

The "also worth a line" guide items are not audited yet.
