# Where we are (handoff between sessions)

Read this first in a fresh session, together with `AGENTS.md`. Update it at the end of every milestone. Older
handoffs, finished plans and specs live in git history (`git log -p -- docs/status.md`, `git log --diff-filter=D
--name-only -- docs`). The catalogue and the Onboard / Shelf app keep their own details in their READMEs.

## Now (2026-11-19)

**The suite is feature-complete for a shop's day; work follows real use at the first shop, Iman Pharmacy Kalpitiya**
(`iman-pharmacy-kalpitiya`, install id FC1A79F7, support PIN `make support-pin ID=FC1A79F7`).

**Core battle test (user's top priority, 2026-11-18): area 1 of 7 — sales, discounts, payment, returns — built and
being closed** (spec `docs/specs/2026-11-18-core-battle-test-design.md`, area 1 plan in git history, `4b4b68a`).
Why: Iman found a discount sale refunding the full price; nobody had written that rule down. The method, kept for
every area:
- **Rule book** `docs/rules/`: a grid of every feature a sale can carry × every later action on it; each square a
  numbered rule (SALE, DISC, TEND, RET, TILL), "can't happen", or a question for the user (`questions.md`).
- **Auditor** `internal/ledgercheck` (read-only; after every battle test and over a Demo Room year).
- **Battle** `internal/battle`: a throwaway shop on the real server (template DB copies, `ltest.NewDB`), fixed
  cases per rule, random shop days (`rapid`, own price check), power cuts (the real binary killed mid-sale), two tills,
  double Pay. `make test` runs a short version; `make battle` the long one (20,000 steps clean).
- **Proofs** `proofs/` (Lean 4.15.0 in `~/.local/bin`, `make proofs`, GitHub Actions `proofs.yml`): part refunds
  never pass what was paid; sharing an amount over lines adds up exactly. Go and the till are held to 6,000 cases.
- **Mutation** `make mutate` (gremlins on sales 91%, Stryker on the till's money maths 86%); the money gaps got tests.
Found and fixed (test-first, not yet Released): a bill-discounted sale returned line by line over-refunded (RET-5);
a line returned in many small parts over-refunded (RET-4, proved); a returned serial item couldn't be resold
(RET-10, migration 00037); a paid-off credit sale refunded as shop credit (RET-19); double Pay after a lost answer
made two sales (SALE-50: the till's Pay carries a one-time reference); the till's total and the return screen's refund
could be a cent off the server's (TILL-1, TILL-2). User's answers built: combos come back whole, in whole sets (RET-13);
a cheque that hasn't cleared asks give-it-back or cash (RET-17); card refunds in cash (RET-16); a bill discount lowers
tax (SALE-35). Known gap, not built: RET-14 (mixed tax rates, a line's refund share) needs line tax stored.
**Area 1 closed 2026-11-18** (full run green but the known `aicatalog:278`; coverage map https://claude.ai/artifact/GSvAu11weLXS7PjRD4G8u9 — core Go 76.6%, screens' unit tests 61%). **Agreed flow (user):** area 2 next session; after every area a full run (`scripts/full-run.sh`) and the coverage map updated; when all 7 core areas are solid, each plugin the same way. **Area 2 closed 2026-11-18** (drawer, cash flow, lockers, expenses; rule book `docs/rules/money.md`, plan in git
history): auditor checks (`ledgercheck/money.go`), a case per rule, random money days (`TestBattleMoney`), faults
(at-once moves, a sale during a close, power cuts while money moves), mutants replayed against the battle tests. User's
answers built: a cash refund is never stopped (the till is flagged instead, DRAW-3); a sale rung with no till open lands
in the cashier's next till, both flagged (DRAW-6; each sale stores its till session, migration 00038 — the drawer, the
close and the Z count by it); the Cash flow page shows untracked till cash, and over / short, as money in / out (CF-2).
Found and fixed: two withdrawals at once overdrew the drawer (DRAW-16); a sale paid during a close was missed by it
(DRAW-7); a no-till sale could commit after the next open and stay in no till (power cuts). Not yet Released. Full run
green in 9m52s (Go, Vitest, all Playwright, check-core); coverage map updated. **Area 3 closed 2026-11-19**
(customers: credit, advances, opening balances, payments; rule book `docs/rules/customers.md`, plan in git history):
auditor checks (`ledgercheck/customers.go`: the balance rebuilt from its rows, one phone per active customer, every
payment's money landed, slips chain, old debt never past the opening or the balance), a case per rule, random
customer days (`TestBattleCustomers`), faults (at once, power cuts), mutants replayed against the battle tests.
User's answers built: an advance pays a newly set old debt down (CUST-15); a credit payment sent twice is one
(CUST-16, `clientRef`, migration 00039); a sale on account to a disabled customer is flagged (CUST-17). Found and
fixed — most were two things at once, all fixed by holding the customer's row (`customers.LockByID`): eight advance
refunds of a Rs 300 advance all went through (CUST-11); payments at once lost part of what was paid off the balance
and printed wrong slips (CUST-13, CUST-6); two tills passed the credit limit unflagged (CUST-8); two returns cleared one
debt twice, on two sales or two lines of one (RET-18, RET-19); an admin collecting into a cashier's till while that cashier sold
on account deadlocked (now till first, then customer). Also: two active customers could share a phone (CUST-3); the
statement missed kept change (CUST-10); `BACKUP_INTERVAL=0` crashed the server after its first backup; the full run's
Go tests failed to build while Playwright switched the plugins on (`full-run.sh` now builds that server first).
Known, not fixed: two phone edits at the very same moment can still pair a phone (needs a unique index after a
duplicate clean-up; the auditor flags it); `sales/service.go` is 539 lines (over 500 before this area — split it
when next touched); the Opening sheet could say when an advance paid part of the old balance. Not yet Released.
Full run: Go, Vitest and check-core green; Playwright 256 passed, 4 timed out with Go's coverage run beside it and
passed alone (admin-catalog, admin-money, admin-reports, palettes). Coverage map updated (core Go 76.8%, core
screens 59.2%; customers' files 80–91%).
**Area 4 closed 2026-11-19** (stock: lots, packs, units, counts, write-offs, give back, recall, conversions, freeze;
rule book `docs/rules/stock.md`, STK-1–24, plan in git history): auditor checks (`ledgercheck/stock.go`: lots add up
to on hand, never below zero, every movement's worth, whole packs and units), a case per rule, random stock days
(`TestBattleStock`: every lot held to the books), faults (at once, power cuts with a stock worker), mutants replayed
against the battle tests by a script (every survivor killed or deleted but 2 equivalent, the plugin freeze guard and a
vanished-product cost lookup). User's answers built: good lots first when no lot is named (STK-21); no merging a
recalled lot (STK-15); unit changes refused while fractions or packs, a recipe may use part of a packet (STK-18);
counts say "type it in before selling again" (STK-4). Found and fixed: a count saved while sales landed overwrote
them (on hand 5–14 above its movements, STK-4); a lot counted while another lot sold lost units (STK-5); a sale of a
picked lot was cancelled by a deadlock when a count / write-off / give-back of that lot landed at once (STK-23, never
stop a sale); whole-unit products took fractions everywhere but the sale (STK-18); a pack of 2.5 pieces (STK-17); a
sheet's opening stock booked worth Rs 0 (STK-7); the Playwright fixture's hand-set stock had no movements or lot;
`TestTillCatalog` failed for a few minutes every few hours (its barcode was clock digits its names carry). The full
run found one more, older: a cash refund at the till deadlocked against a sale on the same till and a sale was refused
(DRAW-3; the return now takes the till first, as a sale does). Not yet Released. `make battle` 20,000 steps clean
(39 min). Full run: Go, Vitest (1,218) and check-core green; Playwright 271 passed alone (3 timed out with Go's
coverage run beside it: appearance-layout, palettes, till open). Coverage map updated (core Go 77.2%, core screens
59.2%; stock 81.1%, conversions 75.6%).
**Area 5 closed 2026-11-20** (purchasing: orders, deliveries, goods sent back, supplier pay, credits, cheques; rule
book `docs/rules/purchasing.md`, PUR-1–13, SUP-1–10, CHQ-1–6, plan in git history): auditor checks
(`ledgercheck/purchasing.go`), a case per rule, random days buying from and paying two suppliers
(`TestBattlePurchasing`: the machine keeps every balance, old debt and bill by the rule book), faults (at once, a
purchasing worker in the power cuts), mutants replayed against the battle tests (136 killed; the 91 left are
database-failure branches, list filters, wording, and cheque paths areas 1–3 test). User's answers built: a typed
advance pays what's owed first (SUP-5); a credit pays new old debt down, a supplier's opening never below zero
(SUP-2); nothing delivered, ordered or sent back for a disabled supplier (SUP-9); frozen pieces out for sale go back
like a frozen write-off (PUR-9, a tick on the return line); Q5 (2026-11-20): the last units of a lot to go back take
what's left of their line's share, so a whole delivery sent back clears its bill to the cent (PUR-3, PUR-10; a line
that cost nothing gives nothing back). Found and fixed: eight receives of one order at once booked it eight times,
and an edit beside a receive turned the bill back into an order (PUR-6); eight refunds of a Rs 300 credit all went
through (the shop then owed Rs 500) and old-debt payments took Rs 800 for Rs 300 (SUP-7, SUP-8: payments and
refunds now hold the supplier); deliveries, sales and payments from a till deadlocked or refused each other (39 of
64 at once) — stock rows are now taken in product-id order by sales, deliveries and returns, a till money comes from
is held first, a payment's split is read while it holds the supplier; a bounced cheque's unspent advance still paid
later bills (CHQ-4); pieces and packs of 2.5 took fractions on deliveries, orders and returns (PUR-4); a whole
delivery sent back left a cent of credit (PUR-3). `sales/service.go` split back under 500 lines. Not yet Released.
`make battle` 20,000 steps clean (41 min). Full run: Go, Vitest and check-core green; Playwright 246 passed, 6 timed
out with Go's coverage run beside it and passed alone (admin-money, admin-data, admin-people, admin-reports,
appearance-receipts, batches). Coverage map updated (core Go 77.3%, core screens 59.2%; purchases 81.7%, supplier
pay 81.4%, supplier returns 81.3%, suppliers 78.5%, cheques 80.2%). Left for later: a return naming the same lot on
two lines (the screen can't) prices the last units off the first only; kit components and sale returns' restock
don't take the product-id lock order yet. **Area 6 closed 2026-11-20** (recovery and losses: warranty swaps and what a supplier makes good; rule book
`docs/rules/recovery.md`, WAR-1–6, REC-1–8, plan in git history): auditor checks (`ledgercheck/recovery.go`), a case
per rule, the purchasing machine writes suppliers' lots off, settles and undoes them (`machine_recovery_test.go`), faults
(swaps, settlements and undos at once, swaps beside returns, recoveries beside sales / deliveries / payments, a recovery
worker in the power cuts), mutants replayed against the battle tests (97 killed; the rest are list filters, paging,
wording and database-failure branches). User's answers built (Q1–Q5): a credit may be any amount; the lot's supplier is
filled in on the form (the write-off answer and Losses lines carry it), a disabled one takes a credit but no extra
charge; a replacement topped into a lot blends its cost; **Undo** on Losses & Recovery while nothing came after it
(migration 00040: the lot and its cost before, a credit's bills / old debt / what waited, one recovery per loss); the
server refuses swaps of out-of-date or returned units. Found and fixed: a serial unit could be swapped twice at once and
a by-receipt line past what was kept (no lock; WAR-6); by receipt counted returned units and took a void sale (WAR-4);
the server swapped returned / out-of-date units if the screen was skipped (WAR-1); a replacement's stock never named its
recovery and a top-up ignored the loss and the extra (REC-2). `make battle` found two auditor checks too strict, not
money: a write-off worth under half a cent books Rs 0 (STK-7, user's yes) and a credit's old-debt drop may pass it when
the supplier already held credit (REC-3). Not yet Released. `make battle` 20,000 steps clean (purchasing re-run alone,
12 min). Full run: Go, Vitest and check-core green; Playwright 224 passed, 8 timed out beside Go's coverage run and
passed alone. Coverage map updated (core Go 77.3%, screens 59.2%; recovery 78.3%, warranty 76.1%). Not covered by a
machine: warranty swaps (no till in the purchasing machine; cases and faults hold them). Dead code swept after it (`deadcode` with every plugin on, `70c14ad`):
the old template context, `JWTAuth`, two unused till-session lists; test-only helpers moved into their tests. **Area 7 (reports and the Z report = the raw data) closed 2026-11-20 — all 7 core areas done** (rule book `docs/rules/reports.md`,
RPT-1–37, ZR-1–4, plan in git history). Reports store nothing, so the auditor
layer is an **agreement check** (`internal/ledgercheck/ltest/agree.go`): every report through the real API against
plain sums over the raw rows and against each other — over a fixed day touching every grid row, in every random-days
machine (every 25th check and each run's end; the sales machine's revenue against its own books), after every power
cut, and over a grocery year (the Demo Room's tests, `TestYearReportsAgree`, in make test); the check is `ltest.Agree`, so
the Demo Room and the battle tests share it. Users' answers built (Q1–Q6, all as
suggested): a return counts on the day it was given, at the refund given (RPT-6); a Z shows the refunds given in its
session and never changes (ZR-2, migration 00041); line reports share a bill discount over the shop's own lines
(RPT-12); the Tax report shows tax given back and owed (RPT-17); a product's first stock and + Add batch / intake with
no bill are stock on the books, never profit (RPT-9, Q5, Q6). Found and fixed: returns came off the sale's day; sales
by cashier ignored returns; product / category reports took a discount share onto airtime; the Sales report counted
tax as revenue, counted void sales and took returns off at a re-worked share instead of the refund given (RPT-37);
Finance and the dashboard read their trend / spark outside the P&L's moment and disagreed with it under load, and the
P&L swallowed 10 errors (RPT-36: one `reports.Snapshot` per page); **opening stock (a product sheet, a first count,
+ Add batch) was booked as profit by its whole worth** — the demo's net profit read Rs 63,245.95 for Rs 285.95, and
Iman's first month is overstated the same way until this ships (migration 00042 marks the old rows). Mutation:
gremlins on `reports` (the rest is SQL), and `scripts/report-mutants.py` plants 21 SQL mistakes — all killed. Screens
checked at 360 px and desktop, light and dark (Tax, Z, P&L). Also fixed: Pocket Cashier's tests stopped building
after the dead-code sweep (`deadcode` ignores tests — run `go vet ./...` after one). Small, not fixed (area 2): a
cash refund's drawer move is stored and shown as a "Withdrawal" on the Z (numbers right, label wrong). `make battle` (49 min): every machine clean; the power cuts found a cash refund paid by a till
opened while the return waited missing from that till's Z (fixed: a refund belongs to the session its cash left,
ZR-2) and a check reading too soon after a cut (it now waits for the database to go quiet); power cuts then clean three
times. Full run: Go, Vitest green; check-core green once the year test moved out of core; Playwright 262 passed beside
Go's coverage run, admin-reports and palettes passed alone, **loose-versions (Alternatives plugin) fails the same way
on the commit before area 7** — its till opens late and the spec misses the dialog; left for that plugin's pass.
Coverage map updated (core Go 77.6%, reports 80.7%, screens 59.2%). Not yet Released.
**Next (user, 2026-11-20): push core toward 100% coverage** — what's
left after the areas is mostly database-failure branches (`if err != nil { return apperr.Internal(…) }`), list
filters and screens; the failure branches need a way to make the database fail on purpose (a test DB wrapper that
fails the Nth query), so plan that as its own piece of work. The coverage map still shows the run before the sweep;
area 7's full run updates it. **Coverage push, first pass (2026-11-20):** full run with coverage: Go 76.5% (12.5% uncovered is database-failure branches only, 11.0% real logic, mostly `internal/api` handlers Playwright reaches, `cmd/` and plugins; core money and stock code has little left), screens 58.5%; 6 Playwright timeouts beside Go's coverage run passed alone. The uncovered logic in money / stock read branch by branch: **two bugs found and fixed** — a counted coin under a cent made the drawer round away from its own count (DRAW-17, `7ce4e802`), and a conversion whose earlier shares rounded up to the whole yield lost the last lot's value (STK-11, `93384d4b`); cases added for change given above due (TEND-3), a zero-cost picked lot (SALE-5), a frozen product's gone lot (SALE-7), a warranty swap on the Losses page vs the P&L (RPT-22/33 — no random day swaps a unit), re-importing a customer with a balance (CUST-2), supplier pay spreading over a paid bill / a negative amount. Dead code dropped (`stock.Damage`, `Conversion.Runnable/MaxRuns`; checked against plugins too). `cashflow.go:420` (a Move between a till and a locker) has no caller — the drawer's own locker leg does that. Open question for the user: a failed read of "untracked cash" lets the cash out (`cashregister/movements.go:36`, fails open). Next: `make mutate` on sales, stock, supplier pay; the database-failure wrapper. **Coverage push, 2026-11-21 (pick up here):** money + stock code 85.2% → 87.4% of statements (633 left), Go total
77.6%. **Bugs found and fixed:** a saved withdraw / pay-in answered as an error, so a cashier repeating it took the
cash out twice (DRAW-18, `a7cb15c4`); the same for goods sent back to a supplier (PUR-14, `676e8165`); "untracked cash"
unreadable now counts as off (DRAW-12, user's choice, `2227ee70`); a sale naming a missing customer and an order /
delivery line for a merged-away product were 500s, now clear refusals (`7f5f1b24`, `f99f8dd9`); the Demo Room year
failed to build on some days (egg tray bigger than the stock, `906e8d4e`). **Failure sweep** (`ltest.NewFaultyDB`,
`internal/battle/faults_sweep*_test.go`): 33 core money / stock actions, each database statement failed in turn,
every table unchanged, auditor passes — seconds to run. **Refusals** (`internal/battle/refusals*_test.go`): 84 bad
requests refused with nothing changed. **Mutant replays** done for sales, stock, supplier pay against the whole
battle package; cases added for what lived (STK-7 blank-cost lot write-off, RPT-9 opening on a stocked product /
a count's new lot, SALE-5/7 frozen picked lot, RET-8 non-picked return, SALE-9/25 recipe, SALE-15, RET-3); what
still lives behaves the same, is plugin-only or is labels / limits. Not reachable through the API (left as guards):
a negative opening / closing cash (a negative note count reads as none); a bill discount above the goods is capped
(DISC-3). **Refusals, third batch (2026-11-21, `a1b756db`):** 52 more bad edits and lookups
(`refusals_last_test.go`) — one bug: a loss settled naming a supplier that doesn't exist was a 500 (now "supplier not
found"). Of the 112 uncovered refusals in money / stock code (fresh coverage run, Go 77.9%), 26 are now reached; the
other 86 are almost all guards behind a check the handler makes first, plus recovery undo's old-data conflicts (rows
from before migration 00040) and lots / tickets by a wrong id; disabling or bringing back a supplier that doesn't exist
is a harmless no-op (204). **Small branches (2026-11-21):** 171 read; the money ones got cases — two cheques
of one payment bouncing in turn (CHQ-3), a blank-cost own lot sent back at the product's cost (PUR-9), an all-free
delivery sent back credits nothing (PUR-10), a lot's price through a price change "applied to stock on hand" (SALE-5,
never tested ticked before), a new supplier from a sheet carries its opening once (SUP-2); the whole-sale return path
nothing called was dropped (`48c4c576`). The rest are guards (settle's loops stop at the top first; a picked lot
can't be short while its product isn't — a live sale refuses a short pick, a Review sale drops it), list filters /
limits Playwright reaches (Go-only coverage doesn't count them), and old-data paths (a negative supplier opening). Left
for the plugin pass: `pocketcashier/listener.go` `publicCert` is unused. **Full run with coverage
(2026-11-21, 8 min):** Go, Vitest (1,218) and check-core green; Playwright 266 passed, `admin-money` timed out beside
Go's coverage run and passed alone. Coverage map updated: core Go 79.9% (was 77.8%), the money packages 88.9% (was
81.8% — supplier returns 94.1, drawer 93.2, money moves 92.2, selling 92.0, purchases 90.6), screens 59.2%.
**Core coverage push done. Next: plugins, simplest first (another session).**
**Plugins group 1 closed 2026-11-21** (clearance, requests, Product Plus, Alternatives, AI Assistant; plan in git history):
rule book `docs/rules/plugins/` (PLG + CLR / REQ / PPF / ALT / AIC); harness in `ltest` (plugin checks via
`ledgercheck.Add`, template named by plugin migrations too, one shop per test binary — plugin hooks are
process-wide — so battle tests live in `plugins/<key>/battle/`). Found and fixed: clearance marked down products
that don't exist / services / disabled (CLR-2), Disable took products no longer dead (CLR-6, user's answer: skip);
Make product sent 8× at once made 4 products, an ask during a merge was lost, a link to a merged-away product
(requests); number fields took NaN/Inf/hex, Fill in stored values for missing products (Product Plus); hand splits
now rescale to 100 (ALT-5, user's answer); saved changes answered as errors (Product Plus, Alternatives); the AI
Assistant's job could apply with no undo after a failure — new core seam `appdb.InTx` (user's choice): Product Plus,
Alternatives and Pharmacy stores join a caller's transaction. Then: Suggest combos sends the 200 best sellers of 90
days topped up with the newest (AIC-7, user's answer; it sent the 50 oldest, so a new product never got an idea —
also what made `plugin-aicatalog.spec.ts:278` fail); the failure sweep covers the groups and setup jobs and found
the groups job reading three statements per group the shop keeps (now one query). **Mutation:** gremlins' mutants
of each plugin replayed against its own and battle tests (`scripts/replay-mutants.py` takes `TESTPKG` and a
`--dry-run` log — a plugin package's own tests reach little), 9 side by side on the NVMe (~2 h, the AI Assistant's
892 most of it): 954 of 1,138 caught; every survivor read, tests added where real (field moves, sheet export, type
changes rewriting values, a manual split's offer, the product form's tier, the AI client, clean-up filters and parts,
hints, AI-on mode, limits); what lives is prompt wording (an empty section header), sort ties, batch-size defaults and
equivalents. Full run with coverage: Go, Vitest (1,218) and check-core green; Playwright 264 passed, admin-reports
timed out beside Go's coverage run and passed alone. Coverage map has a Plugins table (reached: clearance 92.1%,
requests 93.9%, Product Plus 93.5%, Alternatives 89.3%, AI Assistant 88.6%); Go total with plugins 80.0%. The AI
Assistant's tests took 36 s a run (each built a new database with every migration): now a template copy, 11 s (run
one at a time — each test's Setup adds to core's process-wide hooks, so side by side they race).
**Plugins group 2 closed 2026-11-21** (Combos, Print & Copy, Pharmacy; plan in git history): rule book
`docs/rules/plugins/` (CMB / DOC / PHA), checks in each plugin's `ledger.go`, battle tests in `plugins/<key>/battle/`.
User's answers built: what a sale did for a plugin is written **in the sale** — combos' sold rows, Print & Copy's jobs
and a pharmacy dispense (patient picked at the till) were after-sale calls, lost on a blip and taking the till's word;
now the sale hook writes them from the sale's own lines (new core seam: a sale carries its plugins' own data, the
till's sale meta, `SaleRequest.plugin` → `plugin.SaleLines.Data`; Pocket Cashier sends it too); the combo report
takes returned sets off (migration combos 0004: a sold row names its lines); the till's combo / print prices are
trusted like any line. Also found and fixed: a saved combo / print grid / extras list answered as failed when the
read-back failed (PLG-6); a new print service left a stray product when its link failed; registering a patient kept
the customer when the patient part failed (core's customer Create / Get join a caller's tx, `appdb.InTx`); a script
naming a missing product was a 500; the dispense lock cast the sale id to int. **Mutation:** 442 of 471 caught (combos
61/68, Print & Copy 129/151, Pharmacy 252/252); survivors read, tests added (a one-line combo, name limits, grid
order…). Coverage map's Plugins rows: combos 91.9%, Print & Copy 91.8%, Pharmacy 85.0%; Go total with plugins 80.8%.
Full run: Vitest 1,219 and check-core green; Go and Playwright green after fixing what this group broke (the Demo
Room's generator and Pocket's offline patient); `admin-money` timed out beside Go's coverage run, passes alone.
**Raised, not changed:** saving a product with a plugin's section (Pharmacy marks, Product Plus fields) is not one
change — core saves the product first and says so if a plugin's part fails (`internal/api/product_extras.go`, by design).
**Plugins group 3 closed 2026-11-21** (Repairs, Reload & Bills; plan in git history): rule book `docs/rules/plugins/`
(REP / RCH), checks in each plugin's `ledger.go`, battle tests in `plugins/<key>/battle/`. User's answers built: the
float moves by **face value** (a discount is the shop's money; new core seam `plugin.SaleLine.Gross`), a device or bank
turned off since a line was rung still books (only a missing one is refused), a pickup returned in full stays collected.
Found and fixed — Repairs: a cancel / status landing with the pickup left a collected job stamped cancelled (every write
now checks "open" under the job's row lock); the report counted a second job's labour and a part sold loose on the same
bill (`repair_pickup_lines`, migration repairs 0007, old jobs filled in as the report read them); saved changes answered
as failed when the read-back failed (PLG-6); a settings read failing gave a new job no warranty; removing another job's
part was a silent 200. Reload & Bills: a package of two used one package's float; a turned-off bank refused the sale; a
line returned in parts left cents of float / commission (cumulative parts now); **a count and a sale at once counted the
sale twice** (every float writer locks its devices, lowest id first); the same package added at once on two tills
refused one; a drawer count could lose its session. **Mutation:** 499 of 502 caught (Repairs 264/266, Reload & Bills
235/236); the two real ones got tests, the last is equivalent (`till:0` refused later). Coverage map's Plugins rows:
Repairs 87.7%, Reload & Bills 85.8%; Go total with plugins 81.8%. Full run: Go, Vitest (1,219), check-core green;
Playwright 231 passed, 6 timed out beside Go's coverage run (admin-money, admin-data, admin-people, admin-reports, auth
konami, till open with a float) — all 47 tests of those files pass alone. The Demo Room year passes every new check.
**Pocket Cashier battle-tested (2026-11-22) — every plugin is done.** Rule book PKT-1–15 (`docs/rules/plugins/pocketcashier.md`),
auditor checks PKT-4 / 7 / 12, battle cases in `plugins/pocketcashier/battle/`. **Found and fixed:** a send's counts were
written only at batch end; a cash out / in or close made on the phone is now booked as made (DRAW-19, past the PC's drawer
count, a late till opened, flagged); the PC's own trouble (database away, a 5xx) refused open / close / cash moves / hold /
unhold for good — now every kind waits its turn first (PKT-5); anything but a sale is claimed (`running`, migration 0005)
before it runs, so a stop between the work and its record never runs it again — it shows "may not have gone through —
check" (PKT-4); a call with no one-time reference gets the action's ref (CUST-16). **Core:** a failed account lookup said
"your account is no longer active" (401) on a database blip, signing a till out — now a 500. The failure sweep over every
kind passes; every kind sent two at once and again is done once; PKT-14 fast-check (the phone's request and total = the
till's). **Mutation:** 233 of 290 caught; the 5 on money paths and pairing's limit edges got tests, the rest equivalent
or OS-level. Coverage map's Plugins row: Pocket Cashier 84.6%; Go total with plugins 82.8%. Full run: Vitest (1,221),
Playwright (271), check-core green; Go green but one audit beside another test (PKT-7 counted a running action's cash
too early — the check now counts it; passed 3 runs after).
**Next: battle-test installing and running a shop (another session, user 2026-11-21, below).**
**After Pocket Cashier (user, 2026-11-21): battle-test installing and running a shop the same way** — rule book first:
karots-setup (install main / cashier PC, file and server mode, Update, mode change, Uninstall with its final backup,
Bring back a shop), self-update (signed manifest, download, swap, BootGuard rollback), backups (automatic, copy folder,
cloud), the Shop Builder (shop builds, Release vs Test build, plugin lists back to core-only). Then a fresh `make
battle`, and the real-printer and real-phone tests, before the Release.
**Idea (user, 2026-11-21), after the battle tests:** Alternatives → **Group by fields**, no AI: the admin picks one or
more product fields (Generic name + Strength for a pharmacy; any field elsewhere) and products with the same values
become one group, shown first to untick wrong ones, then saved as ordinary groups. The fields come through core's
generic `ProductAttributeSet` hook (Product Plus registers its fields there), so Alternatives never names Product
Plus: with no field plugin compiled in, there is nothing to pick. Pharmacy stays shipped with Product Plus and
Alternatives (user: pharmacies need generic names, and they group medicines).
**Idea (user, 2026-11-21), after the battle tests:** **a supplier by field** — Product Plus rules like "Brand = Nestlé
→ supplier X": a product with no preferred supplier of its own (`products.preferred_supplier_id`) is ordered from the
rule's supplier (Reorder, purchase orders, a GRN's suggested supplier); its own supplier always wins. Needs a generic
core seam (a fallback supplier for products that have none, asked by reorder / purchasing) so core never names
Product Plus. User's answers: two rules matching one product → it shows under both suppliers, with a warning; the first
delivery still fills in the product's own supplier (that supplier has it too; skippable). Why: at Iman most suppliers
were typed product by product — a brand per product and a supplier per brand is far less work. A supplier brings
many brands: set it from the supplier's side too (this supplier brings brands A, B, C). When a brand changes
supplier: change the rule once; to be designed — offer to move that brand's products whose own supplier is the old
one too (else the first-delivery fill-in keeps them on it). Core already moves ALL of a supplier's products to another
(`AdminSupplierReassign`, Suppliers page); the brand case needs only one brand's products moved, so it would be that
move narrowed to the rule's products.
**Plugin order (decided 2026-11-21, from each plugin's size, tables, hooks and what it moves):**
1 clearance → 2 requests → 3 productplus → 4 alternatives (fix the failing `loose-versions` spec) → 5 aicatalog —
product data only, no money or stock (lighter: merge hooks keep their rows, own data consistent, never touches money /
stock, offline data). 6 combos → 7 documents → 8 pharmacy (after productplus + alternatives; its controlled-drug
register must agree with stock) — sell through core's till (core's money rules hold; the plugin adds prices, stock,
records). 9 repairs → 10 recharge (riskiest: float per device / carrier, own tender, P&L income, carriers paid as
suppliers; thinnest tests, 582 lines for 2,176) — own money, full rule book like a core area. 11 pocketcashier last
(replays every till plugin offline). The coverage map gets a Plugins section (one row per plugin: code reached, tests,
passed or not) when the first one passes. **One group per session (user, 2026-11-21):** next session = group 1
(1–5), then group 2 (6–8), then group 3 (9–10), then Pocket Cashier; each session ends with a full run, the coverage
map's plugin rows and this file updated. **Speed (user,
2026-11-21):** test Postgres and the Go cache live on the NVMe, `synchronous_commit=off`; replays run 9 shards side
by side (`scripts/replay-mutants.py … i/n`, `SKIP=` resumes) — stock replay ~80 min → ~10; `make test-all` 688 s →
~490 s. Full runs still show the known Playwright flakes (`till.spec` open with a float, sometimes `admin-money`,
`admin-data`) and Vitest 5 s timeouts under load; all pass alone.
**Plugins after area 7 (user, 2026-11-20): plan them simplest first**, each through the
same layers; apps that build on other plugins (Pocket Cashier picks up every plugin's till features) come last, once
the plugins they use have passed.
**Later idea (user, 2026-11-19), after all areas pass:** a note box on the till's plain Hold (only Send to cashier
has one; held sales are named after the customer or first item). **No Release until all 7 core areas and every plugin have passed** (user, 2026-11-18;
Iman is still onboarding, no real sales yet) — the fixes wait on `main` until then. **Mistake to avoid:** never run DB tests on `suite_db` while Playwright runs (it broke 4
specs once); battle / ledgercheck tests use their own databases and are safe beside it.

**Sell page 2.0: DONE, merged to main and Released to Iman as 2026.11.17.2028 (2026-11-17, binary only, `48a7e04`)**
(user hand-tested: "everything looking good"). Notes written by hand in the Builder, also covering 1525's keyboard
Sell page. At Iman: Setup → Data → Updates → Update; nothing changes on their till until a layout is set (system
user → Appearance → Sell page layout).
Full run before the merge: Go 64 packages, web 1,197, Playwright 259 passed + `appearance-layout.spec.ts` (fixed: an
exact name; + a Search only test) and `plugin-aicatalog.spec.ts` alone 17/17 (`:278` failed once in the full run, as
before). **Demo Room** rebuilt (`dist/demo`), with a new Demo moves cheat: **Open Appearance (as the Karots
technician)** — the launcher signs in as the system user itself (`cmd/demo/system.go`), so a customer can be shown
palettes, receipt styles and Sell page layouts. Asked for by the pharmacy
the user showed the suite to (the one that asked for the keyboard till): "shrink the products, widen the cart". System
user → Appearance → **Sell page layout**, per screen size the shop's tills report (window CSS px, so Windows scaling
counts; "used by …" names; same-size screens share one; user: only sizes differ, never per monitor): 20 arrangements
(every placement: any column order, any two stacked with the third full height on either side, or Search only), drag the widths (Payment ≥ 330,
Cart ≥ 340, Products ≥ 300 px, checked again by the server), product cards Large / Medium / Small / **List**, text
size Smaller 90 / Normal / Larger 110. Nothing saved = exactly today's layout. To make Text size move everything, the
Sell page's 22 px sizes became one scale (`text-till-*` in `app.css`, times `--till-text`; `theme/guard.test.ts`
keeps `features/till` on it); plugin Sell page pieces moved onto it too. `cn` now knows `text-till-*` is a font size
(`lib/utils.ts`). Migration 00036 (`settings.sell_layouts`, `screen_sizes`). Pocket Cashier untouched. Guide:
`docs/guide/appearance.md#sell-page-layout`. Added during the hand test (2026-11-17): every placement (20
arrangements, incl. **Search only**: search bar across the top with a drop-down list, plugin menus on the button row,
groups in Browse = F1 pop-up), a preview with sample words and px / % on each section, **Start from** (Scanner counter,
Touch counter, Square screen), **Try here** (the real Sell page with the unsaved layout; no till open = look only),
and the cursor back in search after 20 s idle. **Direction (user):** instead of fixed themes, each shop gets its own
cashier page and receipt (what customers see; nobody shows the admin pages), so it feels made for them; it's all the
same pieces. **Idea, not built:** a practice mode (sales and printing that don't count) — needs its own design.
Not done: a Search only layout on a real counter at a shop.

**Keyboard-only Sell page: Released to Iman as 2026.11.15.1525 (2026-11-15, binary only)** (user hand-tested on a dev
server: "so far so good"; a pharmacy asked if the till is fully keyboard workable). ↑ ↓ in search pick a tile, Enter
adds; F5 held sales, F6 the last line's quantity (↑ ↓ other lines, Delete removes), F7 its discount (↑ ↓ other lines),
F8 customer, F10 again = next payment method, F12 new sale / clear, `/` = Rs ⇄ % in a line or bill discount; Ctrl K runs
every other Sell page button (`theme/pageActions.ts`, filled by `checkout/SellActions.tsx`); hints sit on the controls.
Guide: `docs/guide/till.md#keys`. Full run first: web 127 files, Playwright 240 passed + 3 failed → `cheques`, `till`,
`till-shots`, `plugin-aicatalog` re-run alone 37/37 after fixing the customer pick (typed + Enter at once picked
Walk-in; `aicatalog:278` as before); no Go changes since 0927. **Its published notes show "���" for an ↑** (the Builder
cut the first letter by bytes, fixed `1725978`) and list that customer fix, which no shop ever had: the user said leave
it; **write the next Release's notes by hand** so they also say what 1525 brought. Not on paper or a real counter yet.

**Stock freeze: DONE, merged to main and Released to Iman as 2026.11.15.0927 (2026-11-15, binary only, notes written by hand)** (the user hand-tested it on a dev server and
said "it worked"). A frozen product sells without touching stock or asking; what's **in storage** is counted, what's
**out for sale** isn't (screen wording, never front/back/shelf). Admin freezes a product (Stock → On hand → Freeze) or
the shop (Stock → Frozen → Freeze the shop: new products start frozen). Anyone at the till unfreezes with "how many
are out for sale now" (Frozen tab, or the cart line mid-sale as a dialog). Adjust on a frozen product asks what
happened (Put out for sale / Damaged → out for sale or in storage / Count changed); a conversion into a frozen loose
product asks out for sale / into storage only when the bag isn't frozen. Every damage reaches the damage list and
Losses (an out-for-sale write-off comes back as `to_shelf`, never an adjustment). Pharmacy refuses controlled drugs
(`plugin.FreezeGuard`). Migration 00035. Guide: `docs/guide/stock.md#freezing-stock-while-a-shop-starts`.
Full run before the merge: Go 63 packages, web 1,177, Playwright 256 passed + `plugin-aicatalog.spec.ts` alone 15/15
(`:278` failed once in the full run, as before). Not done: Unfreeze on the phone cart (Pocket ships after release 2);
Onboard pushing products already frozen (separate repo). At Iman: Setup → Data → Updates → Update.

**Previous Release: 2026.11.12.1741** (user asked). Full run first: Go 63 packages, web 1,174, Playwright 251 passed +
`plugin-aicatalog.spec.ts` and `appearance-receipts.spec.ts` re-run alone 18/18 (`plugin-aicatalog:278` failed once in
the full run, as before). In it:
- **Compact**, the eleventh receipt style (`7d51d2f`): logo beside the shop's name, address and phone (no logo: centred);
  receipt number + date on one line; one row per product (Item · Qty · Price · Amount), a small line only for a
  saving / warranty / serial; no barcode; ~38% shorter on 80 mm. Iman's style is still what it was: switch it at the
  shop (system user → Appearance → Receipt style → Compact) after the update.
- **Thank-you line** in Settings (migration 00034, `receipt_thanks`): replaces every style's "Thank you" and the plain
  backup's; empty = each style's own wording. Iman wants "Get well soon!"-type text — they set it themselves.
- 2026.11.12.1736 went out a few minutes earlier with notes cut at a "; " in the commit title (the notes keep a title
  only up to " — " or "; "); 1741 replaced it with written notes via the Builder app.
- Not yet on paper: a Compact receipt and a Larger one on the XP-80T.

Earlier Releases stay in the bucket (2026.11.09.0655, 2026.11.08.1330, …). **Don't publish another before the user
says.** `dist/iman-pharmacy-kalpitiya/` holds the build; `dist/aiman-pharmacy-kalpitiya/` is old and must not be installed.

## Next

**More receipt styles only when a shop asks for a different one** (user, 2026-11-12).

**Where 2026-11-10 ended (catalogue day, no suite changes):** the user is at Iman on 2026-11-11 with two printed
lists, imports an update file into Onboard, adds products (picking from the catalogue, hand-typing the rest), pulls
Iman, then brings a new Onboard backup. Everything below lives in `~/Projects/karots-suite-catalogue` (its README is
the detail; `shops/iman-matches.md` for Iman).

0. **Morning of 2026-11-11: DONE.** Unique Pharmacy (+1,555) and Beyond Health (+420) in (`build/makers/pages.py`:
   product pages from the sitemap, ones the catalogue has skipped); the promo rule no longer drops "Soap Free" / "Sugar
   Free" products; mycare's whole range downloaded (9,802 of 9,887; 85 gone from the site). Catalogue **52,357**.
   The morning's update file was replaced by the full one (item 2). Karots files in the user's Downloads live in
   `/mnt/nvme_512GB/Downloads/Karots/` (`catalogue-updates/`, `onboard-backups/`, `iman-lists/`).
1. **The 40 checklist: DONE 2026-11-11 from box photos** (catalogue `9e90d54`; photos in `~/Desktop/40list/`). Iman: 47
   confirmed, 0 waiting, NEX VITA sold out (no box; `sold_out` in the decisions). Found on the boxes: "Nvitor D" is a real
   product (Nvitor-D, ACE), **not a typo — keep it in Onboard**; Iman typed cefuroxime for Mahacef (box: cefixime), +K2 for
   Vidan D 2000 (none), 1200 IU for Dee K 2000 (2000); Becosip is tablets; a decision key may be `"name | generic"` (two
   Sithara products share each name). The 113 safety-net list (`iman-lists/iman-matches-to-glance-2026-11-10.pdf`) is
   not answered yet.
   **Afternoon: Iman's 104 new products matched** (backup `onboard-backups/karots-onboard-backup-20261111-1328.json`:
   Iman 652 products, typed on Iman's own PC; none hand-typed in Onboard). 53 confirmed, only NEX VITA missing; Baurs
   celecoxib = Unichem's via A Baur & Co (NMRA register). Catalogue 52,429 (`b9da22f`). **Update file for the user:**
   `catalogue-updates/karots-catalogue-update-2026-11-11-3.json` (from the 13:28 backup: 24 new, 15 refreshed, 0
   doubles) — **imported and checked** (backup `karots-onboard-backup-20261111-1428.json`: 52,538 products, all 39 as
   sent, 0 doubles, Concor 2.5mg deleted, Iman's 652 intact). **Catalogue work paused (user, 2026-11-11):** nothing
   more until a shop needs it; missing generics stay as they are.
   - **Iman's own categories and generics aren't reliable** (user: new staff, little English; ZyQ was under
     Antibiotics). The whole-list fix (item 3) proposes categories from what each product is and takes box / catalogue
     details over theirs; the user approves the tree.
   - Still open, not urgent: the safety-net glance (the 113 list + ~10 brand / close picks among the 104), and
     Immune Up's real contents (mycare / Topcure disagree; a box photo settles it).
   - Next new Iman products: `python3 shops/iman_matches.py <latest backup>` marks them "new since last list".
   - **2026-11-12 (user at Iman again):** backup `karots-onboard-backup-20261112-1626.json`: Iman 739 products (+87),
     **all matched**. CoQTen, Nifigen ER 20 and NEX VITA from box photos (`Karots/iman-photos-2026-11-12/`). The
     user's Epnone 50 / Solotrate SR 30 / Tel OD 80 showed the catalogue held one medicine under two spellings
     ("Tel OD 20" / "Tel Od 20mg"): **278 such pairs merged, only when the maker is the same** (user); the shop's
     spelling wins. Catalogue 52,157 (catalogue `d10fe14`). **Update file for the user:**
     `catalogue-updates/karots-catalogue-update-2026-11-12.json` (39 refreshed, Nex Vita new, 278 extra copies
     deleted; none of them the user's or in a shop) — **imported and checked** (backup `karots-onboard-backup-20261112-1643.json`: 52,266 products, all 40 as sent, 278 deleted, 0 doubles, Iman 739 and 470 shop links intact).
   - **2026-11-14 (user at Iman):** backup `karots-onboard-backup-20261114-1759.json`: Iman 988 products (+249), all
     matched but Metrokeb-200. Box photos (`Karots/iman-photos-2026-11-14/`): Om-3 = 500mg, Nex3 1000 (fish oil,
     Nexus), Tri Bull = Tribull Tabs, Livo-D a supplement. T4 50 Montpellier takes the place of three catalogue names
     (user); Onboard had saved **Thyronorm 50mcg twice** (same microsecond: an Onboard double-save?) → the unlinked
     one deleted. "Omega 3" generics no longer cut to "Omega". Catalogue 52,156 (catalogue `6932e0f`). **Update file:**
     `catalogue-updates/karots-catalogue-update-2026-11-14.json` (37 products, 10 deleted: none linked to Iman; METROLEB-400 merged with Metroleb 400mg) —
     **imported and checked** (backup `karots-onboard-backup-20261114-1818.json`: 52,263 products, all 37 as sent, 10
     deleted, 0 doubles, Iman 988 and its 674 links intact). Metrokeb-200 = Metroleb 200 (typo); IMOI-15 from its box.
   - **2026-11-15:** backup `karots-onboard-backup-20261115-1955.json`: Iman 1,238 products (+250), **all matched**
     (user confirmed the four same-brand picks). Iman renamed Linagliptin 5mg → "Linagliptin 5mg (SPC)": the
     catalogue takes their name (catalogue `d5998dd`). Onboard already up to date: **no update file needed**.
2. **Generic clean-up: DONE 2026-11-11, and in the user's Onboard** (backup `Downloads/Karots/onboard-backups/
   karots-onboard-backup-20261111-0910.json` checked: 52,515 products, 13,005 generics all in line, 0 doubles, the
   user's 261 products and Iman's 549 shop links intact). The morning file was built from an old backup and made 114
   doubles; a deletion-only file removed them. **Rule: an update file only from the user's latest backup.**
   Fixed the same day: `onboard_update.py` counts a product as the user's when Onboard stamped it (microseconds) or
   its uid is in `build/onboard_yours.json` (every uid ever theirs, seeded from the 10 Nov backup); Onboard `b131b5b`
   reads the deleted list once per import (not yet in an installed build).
3. **Iman whole-list fix**, when Iman has finished adding products (wait for the user): user exports Products →
   Sheets ▾ → Include ref + Include custom fields and sends the CSV → match every row (start from
   `shops/iman-matches.csv`) → user approves the category tree → import the same CSV back (each row updates its own
   product by ref; `category` takes a path) → delete old empty categories, merge true duplicates → **then stock**
   (Iman's trial month sells with "sell anyway"; a full stock-take starts them fresh — no trial / go-live tooling
   unless the user asks). Never wipe and re-add: refs keep barcodes, packs, lots, suppliers and sales.
4. Later, user's call: the install stick, the whole-system check, Pocket Cashier on a real phone / tablet.

## The family

- **Karots Suite** (this repo; pushes to GitHub `origin` and GitLab `gitlab`).
- **Karots Suite Onboard + Karots Shelf** (`~/Projects/flutter/karots_suite_onboard`, one Flutter repo, both apps on
  Android, Linux and Windows). `make` builds both APKs + both Linux apps into its `dist/` (`make install-linux` = app
  menu launchers); Windows = GitHub Actions on push (both zips; kept in `dist/windows/`). Device unlock for 24 h after
  the code: Android screen lock, Windows Hello, Linux password (polkit). Onboard = the user's own setup app (install ID
  3EE0E57C's hourly code); on Windows for the Iman owner's laptop (tested in `win10` 2026-11-12). Shelf = shop staff's
  app, free; real-phone hand test PASSED 2026-11-09; in Iman's build since 2026.11.09.0655.
- **Karots Suite Catalogue** (`~/Projects/karots-suite-catalogue`, private, 50,065 products): Onboard gets it as an
  **update file** (`build/onboard_update.py`, from the user's latest Onboard backup: same ids, merges in place; the
  user's own edits win). The first full import was a wipe + import; never again unless the user asks.
- Karots PIN Vault is the user's own app for all their projects: never tie it to Karots Suite.

## Not yet tried for real

- **Pocket Cashier release 2** (on `main` since 2026-11-08): the user tried it on 2026-11-11 (dev server, the Demo
  Room's every-plugin year restored, a 10-step test list) — "looks fine"; they'll report anything they find. Known small gaps left on purpose: an offline repair keeps "New-xxxx" in its sale line; offline
  on-account sales don't lower the balance on the phone (can raise a false stale flag); a return's cash isn't
  compared with the PC's; after a cut an offline sign-in has no session until signing in again (printing and tickets
  fail quietly); a person who must change their PIN gets calls refused while their sales stand; a refused sale's note
  has no items or total. **User's call, open:** Pharmacy cuts the patient's name at 360 px and Print & Copy squeezes
  its total (a plugin layout change the Pocket rule forbids without the user's say); a reload past the float isn't
  flagged under What came in; a sale made during a cut can't be returned on the phone until the PC has it.
- A sales person's flow on this build; Credited us with the new prefill.
- The XP-365B label printer (nothing printed on it yet): its roll test decides whether Classic labels stay.
- A cash drawer (Iman has none for now).
- Recovering a shop already down from an update cut, through Setup → Update (needs the user at the VM).

## Waiting on the user, in no hurry

- Anything from Iman's use. Iman's charges; the business registration number (don't sign shops before it); the
  lawyer meeting with `docs/legal/karots-suite-lawyer-review.pdf`.
- Test backups in the old `aiman-pharmacy-kalpitiya` cloud folder age out by about Dec 4 (30-day lock; the user
  deletes them in Cloudflare, not me). Keep `karots-main` / `karots-cashier` VMs off or expect uploads there.

## Decisions and notes to keep

**Iman's setup:** new PC (i3 4th gen, 8 GB, 120 GB SSD, 20" 1600x900) = admin + main PC, file mode, cloud backup on;
the owner's Linux laptop = cashier PC with the XP-80T (receipts) through the print helper; XP-365B (labels) on the main
PC; one wired 2D scanner. Plugins: aicatalog, alternatives, combos (the user's gift, not charged), pharmacy,
pocketcashier (the user won't mention it to him), productplus. Add printer models that pass to
`docs/guide/printing.md` as known-good. Al Raj, Mum Digital and MUM Motors stay on Karots POS.

**Receipts:** pictures in the shop's style (Classic receipts gone; the hidden plain text stays for a failed picture
and slips with no template). Product local names don't print; only the shop's own local name does. **Check slips on
the till, not only in Appearance, and with real data** (the till is zoomed: a slip once measured small and drew full
size, "TOTA / L"; `shoot()` cancels the zoom). Classic labels stay until the roll test passes.

**Send to cashier:** a Users choice **Sales person** (`users.sends_to_cashier`): same Sell screen, no money; the
server's allow-list is `internal/api/sales_person.go`, money routes carry `middleware.RequireHandlesMoney`. A ticket is
a held sale with `to_cashier` (random number unique among those waiting); unopened after 2 minutes it moves on;
closing a till deals with its tickets first; `sales.prepared_by` records who prepared it. Sales people stay off Pocket
Cashier for good.

**How expiry really works (the pharmacist, 2026-11-01):** the supplier's notice days are a warning and a reminder,
never a block; the claim is what's left when the rep takes it; made good as the same product (Losses & Recovery
restocks it), money or credit, or nothing. A different product is deliberately not a feature: settle as Credited
us, enter what they bring as a delivery, and the credit pays that bill (oldest open bills first; the rest pays the
next, `loss_recoveries.credit_unapplied`). Damaged on arrival = a supplier return from that delivery's lot. No "who
bought it" list for recalls unless a dangerous medicine needs customers called (sales record each line's lot).

**Dropped (user):** prices per customer / customer group; promo videos. **Parked:** open-price items (brainstorm 2026-11-17, paused until the user
has asked real shops; notes in memory `open-price-items`); staff
commission; delivery orders; advance orders. **Future plugins, not now:** loyalty points, gift vouchers / credit
notes, several shops.

**Left on purpose (only if a shop runs into it):** a same-product Replaced it lot doesn't remember that supplier (set a
preferred supplier); a bounced cheque that paid old debt moves the linked balance, not the old-debt part; a partial
supplier-cheque bounce reopens bills before the payment's advance part.

**Power cuts (Iman has generators, no UPS):** tested on `karots-main` 2026-11-04 (file mode): confirmed sales survive,
in-flight ones vanish whole, up in 7-9 s; receipt numbers jump ~30 after a cut. Backups and downloaded updates are
forced to disk (`internal/durable`). Not tested: the gap between Swap's two renames, the real SSD.

## Working here

- **Tests:** `make test-all` runs everything side by side in ~9½ min (Go, Vitest, Playwright with 3 workers, then
  check-core; each Go package and Playwright worker on its own database). Details in AGENTS.md → Dev workflow and
  **Rules and battle tests**. Before a Release: `make test-all`, then `make check-core`.
- **Unreliable tests:** `plugin-aicatalog.spec.ts:278` fails when its worker's database holds over 200 products (combo
  suggestions only look at the 200 oldest — a product question for the plugin pass); `palettes.spec.ts:45`,
  `plugin-repairs.spec.ts:145` and Go `TestTodayTraded` each failed once long ago and haven't since.
- **Type-check the build the way it runs:** `cd web && bunx tsc -b` (a plain `tsc --noEmit -p .` misses test files
  and unused imports, and the Playwright build then fails).
- **Power cuts happen here.** After one: `git fsck`, `make check-core` (a cut during a build leaves the plugin lists on:
  `make core-only`, then `git checkout -- internal/spa/dist/.gitkeep`).
- **Dev server:** `make plugins P=all && make migrate && make dev`; dev logins Admin 0771234567 / 1234, Cashier
  0771111111 / 1111, system user 0000000001 / 2273. `make core-only` before committing.
- **VMs** (`virsh -c qemu:///system`): `karots-main` (Xubuntu, snapshot `before-powercut`), `karots-cashier`, `win10`. `win10`
  has SSH with this PC's key (`ssh karots@<its IP>`); its printer passthrough (USB 0483:5743) was removed 2026-11-12:
  `virsh … attach-device win10 /mnt/nvme_512GB/vmhd/win10-printer-hostdev.xml --config` puts it back.
  Only one VM can hold the XP-80T. Shop `windows-test` in `shops.json` is for Windows tests (never Released).
  Printer queues here: `XP80T`, `POS80` (default; `cancel -a POS80` before plugging an old one in), `XP365`.
- **The user tests with real sales and scans of real paper** (phone scans in `/mnt/nvme_512GB/Downloads/`,
  screenshots in `~/Pictures/Screenshots/`). Don't print on the real printer until a picture looks right.

## Done

M0–M6 core (till, counter tabs, stock, purchasing, money, reports, people) · M7 plugins (clearance, requests,
productplus, combos, alternatives, documents, barcodes + merge, lots, AI Assistant, loose versions, packs, pharmacy,
repairs, recharge) · M8a file mode · M8b Shop Builder + karots-setup · M8c self-update + Release · M8c2 backup copies
+ cloud · live users · till left open · smart reorder levels · print as image + receipt styles · palettes (22
built-ins + custom) · labels as pictures + label styles · M8d Demo Room · docs + screenshots + legal pack · cheques ·
M8e Pocket Cashier release 1 and 2 · pharmacy expiry work · Send to cashier · print helper + cashier PCs' printers ·
power-cut fix · AI Assistant jobs + hints + Alternatives Auto levels · default unit · Karots Shelf · import by ref ·
receipt line rules + text size.

## How we work

1. **Plan, then execute.** A milestone gets a spec (brainstormed with the user) and one plan in `docs/plans/`, tasks
   with files, interfaces, test-first steps and a commit per step. Delete the plan and the spec when it's done.
2. **UI gates:** take screenshots in the palette × mode combinations, review them yourself, carry on without waiting
   unless something needs a product decision. Shots of a whole screen go through `make docs-shots`.
3. **Review** the whole milestone at the end (a fresh reviewer), fix what's found test-first.
4. **No Claude or AI attribution** in any commit or PR, ever.
5. **Work on `main`, local**; the user pushes.

What the user rejects: plain or default-looking UI (stock shadcn cards, "coming later" placeholders) — every screen
must look like the approved mockups.

## Design references

- `docs/design/theme.md` is the source of truth for tokens, palettes, login, the dock and the till layout; its
  mockups are in `docs/design/mockups/`.
- **Shell, login and palettes mockup:** https://claude.ai/artifact/MANYgogqsxC4evmSADNkia
- **Till and dashboard mockup (look only):** https://claude.ai/artifact/GkJjnypEes4GmHiHMDzFT5

## Open items

- **Login hero:** the tilted receipt card (yesterday's close) and the "Counter 1 / last closed" pills wait for an API
  that gives that data.
- **Till top bar:** the counter name on the drawer pill has no data source yet.
