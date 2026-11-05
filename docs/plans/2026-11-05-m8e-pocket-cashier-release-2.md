# Pocket Cashier release 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Everything a cashier needs to avoid pen and paper works on the phone, the same online and offline, from a synced copy that holds the last 30 days of sales.

**Architecture:** The phone reads from its encrypted copy and saves every action to the outbox, as release 1 does. New actions are one generic kind, `call` (method, path, body), which the server replays through the real route as that person, so no business rule is written twice. Plugin screens run unchanged on the phone: plugins declare their offline data through one Go hook, and Pocket Cashier answers their calls from that data when the PC is down.

**Tech Stack:** Go, Echo, sqlx, goose, PostgreSQL; React, TypeScript, IndexedDB, Vitest, Playwright.

**Spec:** `docs/specs/2026-11-05-m8e-pocket-cashier-release-2-design.md`

## Global Constraints

- Screens and features are the same online and offline; only freshness differs.
- Cashiers and managers only; sales people stay off the phone.
- Nothing is refused, it is flagged; flags appear under **Phones → What came in**. The PC settles clashes.
- No phone-to-phone sync. No printing while the PC is down. The phone never opens the cash drawer.
- The phone holds sales of the **last 30 days**; products, customers and plugin selling data whatever their age; no profit, supplier bills or reports.
- Offline data: a plugin may declare it through one generic hook, must not mention Pocket Cashier, and its logic doesn't change. Everything else lives in Pocket Cashier.
- No prompts, banners or dashboard hints suggesting the phone.
- Repo: files ≤ 500 lines; theme tokens only; work at 360px; `make types` after DTO changes; no AI attribution in commits; the next plugin migration number follows `plugins/pocketcashier/migrations`.
- Never run `go test` on `suite_db` during a Playwright run. Import `test` from `e2e/fixtures.ts`.
- TanStack work: `intent list` in `web/`, then `intent load` the matching skill.

## Review Focus

1. **The app updates while work waits on the phone:** the IndexedDB upgrade keeps the outbox and copy; nothing is lost (test in Task 2).
2. **A shop restores a backup:** sale ids move backwards; the phone must drop its stale sales and start again, not keep showing sales that no longer exist (tests in Tasks 1 and 2).
3. **The same goods come back twice** (two phones, or phone and PC): cash that left the drawer is recorded, stock is not counted back twice, and it shows for review (test in Task 5).
4. **The PC comes back the next day:** a replayed credit payment or return keeps when it really happened on record and is marked late, so the owner can explain the day's figures (test in Task 3).
5. **Something created offline is refused when replayed** (a new patient or repair job): what depended on it doesn't vanish or attach to the wrong row; it's failed with a note naming the parent (test in Task 3).

---

## Stage 1: the new sync

### Task 1: Recent sales from the server, and the measurement gate

**Files:**
- Create: `internal/api/till_copy_recent.go`, `internal/api/till_copy_recent_test.go`, `plugins/pocketcashier/recent.go`, `plugins/pocketcashier/recent_db_test.go`
- Modify: `internal/api/dto_copy.go`, `internal/plugin/core.go:51-54`, `internal/app/plugins.go:42-43`, `plugins/pocketcashier/pocketcashier.go`

**Interfaces:**
- Produces: `api.BuildRecentSales(ctx, db, sales, after int64, since time.Time, limit int) (*RecentSales, error)`; `Core.RecentSales func(ctx, after int64, since time.Time, limit int) (any, error)`.
- Produces DTO: `RecentSales{Sales []SaleView; Changed []SaleView; NextAfter int64; NextSince string; More bool; MaxID int64; Reset bool}`.
- Produces route: `GET /pocketcashier/recent?after=&since=` behind `h.phoneAuth`, gzip like `copyFor`.

- [x] **Step 1: Write failing tests** in `till_copy_recent_test.go`:
  - `TestRecentSalesPages`: 3 sales, limit 2 → 2 sales, `More` true; `after=NextAfter` → 1 sale, `More` false.
  - `TestRecentSalesWindow`: a sale dated 31 days ago is absent; 29 days ago is present.
  - `TestRecentSalesChanged`: a `sale_returns` row after `since` puts that older sale in `Changed` with its new `returnedQty`.
  - `TestRecentSalesResetAfterRestore`: `after` above the highest sale id → `Reset` true, sales from the start.
- [x] **Step 2: Run them.** `DATABASE_URL=… go test ./internal/api -run TestRecentSales -v`. Expected: FAIL, undefined.
- [x] **Step 3: Implement** using `saleView(d)` and `h.d.Sales.Get` so a line's `refundLeft` means what the PC's Returns tab shows. `Changed` = sales with a `sale_returns.created_at > since`. Limit 200 per page. `TillSource` and `Flags` stay empty on this path.
- [x] **Step 4: Run them.** Expected: PASS.
- [x] **Step 5: Test the route.** In `recent_db_test.go`, `TestRecentNeedsPhoneKey`: no key → 401; paired phone → 200.
- [x] **Step 6: Measure.** Temporary test `TestMeasureRecent` gated on `MEASURE_RECENT=1` against a database restored from a Demo Room year: logs sales count, total gzip bytes, and server time. Then open the phone in Playwright with 4× CPU throttle and record first download time and search time.
- [x] **Step 7: Gate.** Continue only if, on the busiest built-in shop: total gzip ≤ 30 MB, first download ≤ 2 minutes on wifi, product search stays ≤ 200 ms. **Otherwise stop and take the numbers to the user.** Write the numbers in `docs/status.md`; delete the temporary test.
- [x] **Step 8: Commit.** `feat(pocketcashier): the last 30 days of sales for the phone`

### Task 2: The phone keeps those sales

**Files:**
- Create: `web/src/plugins/pocketcashier/store/recent.ts`, `store/recent.test.ts`
- Modify: `store/db.ts` (version 2), `sync.ts`, `sync.test.ts`

**Interfaces:**
- Consumes: `GET /pocketcashier/recent` (Task 1).
- Produces: `saveRecent(page: RecentSales, now: Date): Promise<void>`, `findSales(q: string, limit: number): Promise<SaleView[]>`, `getSale(id: number): Promise<SaleView | null>`, `recentCursor(): Promise<{ after: number; since: string; ready: boolean }>`, `clearRecent(): Promise<void>`, `refreshRecent(): Promise<void>`.

Store: object store `recent`, keyed by sale id, one sealed row per sale, with an unencrypted `createdAt` index for pruning only. Search decrypts into an in-memory list on first use, so nothing searchable sits in the clear.

- [x] **Step 1: Write failing tests:**
  - `upgrade keeps the outbox and the copy`: open a version 1 database holding a row in each, open version 2, both remain.
  - `pages until done`: mocks `More: true` then `false`; both saved; cursor at the last.
  - `a dropped download resumes`: page 2 fails; `ready` false, page 1 kept, the next call asks from page 1's cursor; selling isn't affected.
  - `changed sale replaces the row`; `prunes sales past 30 days`.
  - `reset clears then reloads`: `Reset: true` leaves only the new sales.
  - `findSales` matches receipt number, customer name and product name.
- [x] **Step 2: Run them.** `cd web && bunx vitest run src/plugins/pocketcashier/store/recent.test.ts`. Expected: FAIL.
- [x] **Step 3: Implement.** `refreshRecent` runs from the same `tick` as `refresh()` in `sync.ts`; a failure sets nothing on `live` (the copy's sync owns that state).
- [x] **Step 4: Run** `bunx vitest run src/plugins/pocketcashier`. Expected: PASS.
- [x] **Step 5: Commit.** `feat(pocketcashier): the phone keeps the last 30 days and takes only what changed`

---

## Stage 2: the counter on the phone

### Task 3: Replay any route (`call` actions)

**Files:**
- Create: `plugins/pocketcashier/replay_call.go`, `plugins/pocketcashier/replay_call_db_test.go`, `plugins/pocketcashier/migrations/<next>_action_results.sql`
- Modify: `plugins/pocketcashier/replay.go`, `replay_sale.go`, `pocketcashier.go`, `web/src/plugins/pocketcashier/store/outbox.ts`, `web/src/plugins/pocketcashier/sync.ts`

**Interfaces:**
- Produces body: `CallBody{Method, Path string; Body json.RawMessage; CashOut string; Label string}`; `ActionKind` gains `'call'`.
- Produces: `ActionResult.ResultID int64`; column `pocket_actions.result_id bigint`.
- Produces (web): `sendCall(userId: number, call: { method: string; path: string; body?: unknown; cashOut?: string; label: string }): Promise<ActionResult | null>` (`null` = queued). `tempRef(ref: string): string` returns `temp:<ref>`.

`runCall`: reject a path outside `/api/v1`. Fetch the user; mint their cookie with `core.Auth.AccessTokenFor`; send the request through `reg.Echo().ServeHTTP` with a recorder and the phone's device id. A 2xx → `applied`, `ResultID` from the reply's `id` / `paymentId` / `claimId` / `saleId`. Otherwise → `failed` with the API error message as `Note`.

Temporary references, run before dispatch on the path and body and in `runSale` on the body:
```
every "temp:<ref>" (quoted in JSON, bare in a path) → look up pocket_actions.result_id, else sale_id, for that ref
no such row or no id → fail this action: "depends on <label of the parent>, which didn't go through"
```

- [x] **Step 1: Write failing tests:**
  - `TestCallAppliesThroughRoute`: a credit payment `call` moves the customer's balance and returns `ResultID`.
  - `TestCallTwiceBooksOnce`: same ref twice → one payment.
  - `TestCallRunsAsThatPerson`: a route that person's rights forbid → `failed`, nothing booked.
  - `TestCallOutsideAPIRefused`: path `/admin/x` → `failed`.
  - `TestCallTempRefResolved`: a parent's `ResultID` lands in a child's body as a number.
  - `TestCallParentFailed`: parent failed → child `failed`, note names the parent's label.
  - `TestCallLateIsMarked`: `At` 26 hours ago → `applied`, note starts `Happened on the phone at`, `at` stored.
  - `TestCallRefusedKeepsCash`: refused call with `CashOut: "500"` → the till shows 500 less, status `failed`, note says so; stock unchanged.
- [x] **Step 2: Run them.** `DATABASE_URL=… go test ./plugins/pocketcashier -run TestCall -v`. Expected: FAIL.
- [x] **Step 3: Implement.** Keep the existing person checks in `run`. Apply `CashOut` through `core.Cashflow` the way `runMove` does.
  `// ponytail: a replayed call counts on the day it arrives, marked late; give routes an "at" if owners need it dated back.`
- [x] **Step 4: Run them.** Expected: PASS.
- [x] **Step 5: Commit.** `feat(pocketcashier): replay any action through its real route`

### Task 4: Counter menu, overlay and Credit

**Files:**
- Create: `web/src/plugins/pocketcashier/overlay.ts`, `overlay.test.ts`, `screens/counter/Credit.tsx`, `screens/counter/Credit.test.tsx`
- Modify: `screens/Menu.tsx`, `screens/Phone.tsx`

**Interfaces:**
- Consumes: `sendCall` (Task 3).
- Produces: `withOverlay(copy: TillCopy, out: Action[]): TillCopy` (balances after waiting credit calls), `saleWithOverlay(sale: SaleView, out: Action[]): SaleView`. Later tasks add to these.
- Produces: `type CounterPage = 'credit' | 'returns' | 'receipts' | 'warranty' | 'writeoff' | 'convert'` and a Menu row per page.

- [x] **Step 1: Write failing tests:**
  - `a waiting payment lowers the balance shown`.
  - `Credit offline saves a call to /api/v1/counter/customers/:id/payment and says it sends when the PC is back`.
  - `paying more than the shown balance is allowed and says the rest becomes an advance`, as the PC does.
  - `a refund is hidden from a person without canManageCredit`, if the PC's tab hides it; check `features/counter/credit/CreditTab.tsx`.
- [x] **Step 2: Run them.** Expected: FAIL.
- [x] **Step 3: Implement.** Same fields as `CreditForm.tsx`, phone layout from `bits.tsx`. Online, print the debt slip through the existing print route, following `copy.askToPrint`. Body includes `sawBalance`; the note on arrival reports a difference.
- [x] **Step 4: Run them.** Expected: PASS.
- [x] **Step 5: Screens.** Take 360px shots in two palettes × light and dark; review them yourself.
- [x] **Step 6: Commit.** `feat(pocketcashier): take credit on the phone, online and off`

### Task 5: Returns

**Files:**
- Create: `screens/counter/Returns.tsx`, `screens/counter/Returns.test.tsx`
- Modify: `overlay.ts`, `overlay.test.ts`, `plugins/pocketcashier/replay_call_db_test.go`

**Interfaces:** Consumes `findSales`, `getSale`, `saleWithOverlay`, `sendCall`.

- [x] **Step 1: Write failing tests:**
  - `finds a sale from the copy offline`; `an older sale asks the PC when online`; `offline, says it's too old and to come back when the PC is on`.
  - `a waiting return lowers returnable quantity`.
  - `service lines aren't returnable`, matching `IsService`.
  - Go `TestSecondReturnRefused`: the same line replayed from two phones → first `applied`; second `failed` with `CashOut` taken from the till; `returned_qty` not above quantity; the batch counts it under What came in.
- [x] **Step 2: Run them.** Expected: FAIL.
- [x] **Step 3: Implement.** Refund amount from `refund.ts` in `features/counter/returns` (reuse it). Call `POST /api/v1/counter/sales/:id/return`, with `CashOut` the cash refund, `deferDrawerKick: true`. Temporary references let an offline sale be returned offline; the sale from the outbox shows in the finder through the overlay.
- [x] **Step 4: Run them.** Expected: PASS.
- [x] **Step 5: Screens** as Task 4.
- [x] **Step 6: Commit.** `feat(pocketcashier): returns on the phone, online and off`

### Task 6: Receipts

**Files:** Create `screens/counter/Receipts.tsx`, `screens/counter/Receipts.test.tsx`.

**Interfaces:** Consumes `findSales`, `getSale`, `pending`.

- [x] **Step 1: Write failing tests:** `shows the shift's sales from the copy plus what waits on this phone`; `Print shows only when live and uses the receipt route with 80 or 58 mm`; `offline, a slip opens on screen with no Print button`.
- [x] **Step 2: Run them.** Expected: FAIL.
- [x] **Step 3: Implement.** Online, also list money, debt and warranty slips from the existing routes. Offline, list sales and this phone's own waiting work: other slip kinds aren't in the copy. Mention this in the Task 17 guide text.
- [x] **Step 4: Run them.** Expected: PASS.
- [x] **Step 5: Screens and commit.** `feat(pocketcashier): the shift's receipts on the phone`

### Task 7: Warranty

**Files:**
- Create: `screens/counter/Warranty.tsx`, `screens/counter/Warranty.test.tsx`
- Modify: `internal/api/dto_copy.go`, `internal/api/till_copy_recent.go`, `internal/api/till_copy_recent_test.go`, `store/recent.ts`

**Interfaces:** Produces `RecentSales.Units []CopyWarrantyUnit` (units sold on the page's sales, with the shape `/counter/warranty/serial/:serial` answers); `findUnit(serial: string)`.

- [x] **Step 1: Write failing tests:** Go: `a page carries its sales' warranty units`. Web: `finds a unit by serial offline`; `finds by receipt offline`; `saves a call to warranty/replace`; `a swap waiting on this phone blocks a second swap of the same unit on it`.
- [x] **Step 2: Run them.** Expected: FAIL.
- [x] **Step 3: Implement,** following `features/counter/warranty/UnitClaim.tsx` and `ReceiptClaim.tsx` for rules, `warranty/reasons.ts` for reasons.
- [x] **Step 4: Run them.** Expected: PASS.
- [x] **Step 5: Screens and commit.** `feat(pocketcashier): warranty swaps on the phone, online and off`

### Task 8: Tickets, online only

**Files:**
- Create: `screens/Tickets.tsx`, `screens/Tickets.test.tsx`
- Modify: `screens/Phone.tsx`, `screens/DrawerSheet.tsx`, `screens/Held.tsx`

**Interfaces:** Consumes `GET /till/tickets`, `POST /till/tickets/:id/open|pass|back`; loads a cart with the same action Held uses to resume.

- [x] **Step 1: Write failing tests:** `a ticket for this person pops up with a beep while live`; `offline shows no tickets and no error`; `opening is refused over a cart with items, as the PC does`; `closing the drawer with tickets waiting asks to deal with each, as CloseRegister does`; `a ticket's plugin lines and pre-filled payment survive`.
- [x] **Step 2: Run them.** Expected: FAIL.
- [x] **Step 3: Implement,** same rules and text as `features/till/cart-ui/Tickets.tsx`.
- [x] **Step 4: Run them.** Expected: PASS.
- [x] **Step 5: Add to** `web/e2e/pocket.spec.ts`: a sales person sends a ticket to a cashier on the phone; the cashier opens it and sells it; `sales.prepared_by` shows. Run: `bunx playwright test e2e/pocket.spec.ts`. Expected: PASS.
- [x] **Step 6: Commit.** `feat(pocketcashier): a cashier opens tickets on the phone`

### Task 9: The PC wins when a cashier moves back

**Files:** Modify `plugins/pocketcashier/replay.go:183` (`signedIn`), `web/src/plugins/pocketcashier/sync.ts`; Test `plugins/pocketcashier/replay_rules_db_test.go`.

- [x] **Step 1: Write the failing test** `TestSendDoesntClaimBack`: person signs in on the phone, then on a PC; the phone sends with `signedIn` set → the active device stays the PC, the action still applies.
- [x] **Step 2: Run it.** Expected: FAIL.
- [x] **Step 3: Implement.** A send only claims if that person has no active device on record; signing in on the phone stays the only deliberate takeover.
- [x] **Step 4: Run** `go test ./plugins/pocketcashier` and `bunx playwright test e2e/pocket.spec.ts`. Expected: PASS.
- [x] **Step 5: Commit.** `fix(pocketcashier): a phone's send doesn't take a cashier back from the PC`

---

## Stage 3: stock changes

### Task 10: Write-off and Convert

**Files:**
- Create: `screens/counter/WriteOff.tsx`, `screens/counter/Convert.tsx`, `screens/counter/stock.test.tsx`
- Modify: `internal/api/dto_copy.go`, `internal/api/till_copy.go`, `internal/api/till_copy_test.go`, `overlay.ts`, `overlay.test.ts`

**Interfaces:**
- Produces: `TillCopy.Conversions []ConversionRow` (the shape `/counter/conversions` answers).
- Consumes: `sendCall`; the route `WriteOffTab.tsx` posts to, and `POST /api/v1/counter/conversions/:id/run`.

- [x] **Step 1: Write failing tests:** Go: `copy carries conversions`, and `TestTillCopyVersionChanges` covers a changed conversion. Web: `a waiting write-off lowers the stock shown`; `a waiting convert moves stock from the pack to the units`; `a write-off beyond the shown stock still saves`; `both save a call offline`.
- [x] **Step 2: Run them.** Expected: FAIL.
- [x] **Step 3: Implement,** following `features/counter/stock/WriteOffTab.tsx` and `ConvertTab.tsx`. A refused replay stays `failed` under What came in; no cash is involved.
- [x] **Step 4: Run them.** Expected: PASS.
- [x] **Step 5: Screens and commit.** `feat(pocketcashier): write-off and convert on the phone, online and off`

---

## Stage 4: the plugins

### Task 11: The offline data hook

**Files:**
- Create: `internal/plugin/hooks_offline.go`, `internal/plugin/hooks_offline_test.go`, `plugins/pocketcashier/parts.go`, `plugins/pocketcashier/parts_db_test.go`
- Modify: `plugins/pocketcashier/pocketcashier.go`, `AGENTS.md` (the Pocket Cashier hard rule)

**Interfaces:**
- Produces: `plugin.OfflineData{Key string; Build func(ctx context.Context) (any, error)}`; `(*Registry).AddOfflineData(d OfflineData)`; `plugin.OfflineDataAll() []OfflineData`.
- Produces route: `GET /pocketcashier/parts`, behind `h.phoneAuth`, with ETag = a hash of the JSON; returns `{ "<key>": data }`.

- [ ] **Step 1: Write failing tests:** `TestPartsHoldsWhatPluginsDeclare`; `TestPartsNotModified` (same ETag → 304); `TestPartsNeedPhoneKey`; `one part failing returns 500, so the phone keeps its last good parts`.
- [ ] **Step 2: Run them.** Expected: FAIL.
- [ ] **Step 3: Implement.** Follow the `hook[T]` pattern in `internal/plugin/hooks_attention.go`.
- [ ] **Step 4: Run them.** Expected: PASS.
- [ ] **Step 5: Update `AGENTS.md`:** a plugin may declare its offline data with `AddOfflineData`; its logic doesn't change and it never mentions Pocket Cashier. Run `make check-core`. Expected: PASS.
- [ ] **Step 6: Commit.** `feat(plugin): a generic hook for what a plugin needs offline`

### Task 12: Plugin screens run on the phone

**Files:**
- Create: `web/src/plugins/pocketcashier/offline/fetch.ts`, `offline/fetch.test.ts`, `offline/routes.ts`, `tillApi.ts`, `tillApi.test.ts`, `screens/PluginPanel.tsx`
- Modify: `store/copy.ts`, `sync.ts`, `catalog.ts`, `catalog.test.ts`, `screens/Sell.tsx`, `screens/Menu.tsx`, `screens/LineSheet.tsx`, `screens/CartBar.tsx`, `screens/DrawerSheet.tsx`, `screens/Pay.tsx`, `screens/counter/Returns.tsx`

**Interfaces:**
- Consumes: `/pocketcashier/parts`, `sendCall`, `tempRef`.
- Produces: `saveParts`, `loadParts(): Promise<Record<string, unknown>>`.
- Produces: `type OfflineRoute = { method: string; path: string; answer: (ctx: { parts: Record<string, unknown>; params: Record<string, string>; query: URLSearchParams; body: unknown; ref: string; out: Action[] }) => unknown; queue?: boolean; label?: string }` and `addRoutes(routes: OfflineRoute[])`.
- Produces: `installOfflineFetch(): () => void`, `phoneTillApi(cart, dispatch, copy): TillApi`.

`installOfflineFetch` wraps `window.fetch` while the phone surface is mounted, with no change to core's `api()` or the plugins:
```
request under /api/v1 and a route matches:
  try the network first; a network failure → answer from the route
  queue: true → sendCall with this ref, then answer (with temp:<ref> as a new row's id)
a matching request that gets a real reply → pass it through untouched
no route, network failed → the normal network error
```

- [ ] **Step 1: Write failing tests:**
  - `fetch.test.ts`: `live calls go to the network`; `network failure answers from the route`; `a queued write saves a call and answers with temp id`; `an unknown route still fails`; `uninstall restores fetch`.
  - `tillApi.test.ts`: one test per `TillApi` method, each reaching the same reducer action the PC till uses.
  - `catalog.test.ts`: a product claimed by a plugin menu now opens that menu (update the existing test, don't delete its behaviour for unpriced services).
  - `Sell`: `shows tillMenu entries`. `Menu`: `shows tillActions`. `LineSheet`: `shows tillLineHints`. `CartBar`: `shows tillCartHints`. `DrawerSheet`: `shows drawerSections on close`. `Returns`: `shows returnLineFields`. `Pay`: `runs tillAfterSale, with temp:<ref> as the sale id offline`.
  - `no plugin compiled in`: no entry and no error.
- [ ] **Step 2: Run them.** Expected: FAIL.
- [ ] **Step 3: Implement.** A panel opens in a full-height sheet (`PluginPanel`). The `ponytail:` comment on `fetch.ts`: wrapping fetch keeps core and plugins untouched; a seam in `api()` if another caller needs it.
- [ ] **Step 4: Run them.** Expected: PASS.
- [ ] **Step 5: Commit.** `feat(pocketcashier): plugin screens run on the phone`

### Task 13: Reload & Bills

**Files:**
- Create: `web/src/plugins/pocketcashier/offline/routes/recharge.ts`, `recharge.test.ts`
- Modify: `plugins/recharge/` (one `AddOfflineData` call reusing the `/recharge/till` handler's read), its DB test

**Interfaces:** Consumes `AddOfflineData`, `addRoutes`.

Routes: `GET /x/recharge/till`, `GET /x/recharge/sale-lines`, `POST /x/recharge/packages/quick` (queue), `POST /x/recharge/counts/drawer` (queue).

- [ ] **Step 1: Write failing tests:** the part equals what `/recharge/till` answers; `till offline answers from the part`; `a waiting reload lowers the float shown`; Go `TestReloadOverFloatAfterReplay`: a reload beyond the float is `applied` and flagged; a bill payout offline sells as a payout line; commission books as on the PC.
- [ ] **Step 2: Run them.** Expected: FAIL.
- [ ] **Step 3: Implement.** If the sale's `Review` path refuses a float shortfall, make `Review` take it as found, with a flag, using its existing mechanism.
- [ ] **Step 4: Run them.** Expected: PASS.
- [ ] **Step 5: Screens at 360px.** If the panel doesn't fit, record what's wrong and fix it in the sheet's styles from Pocket Cashier; if that can't work, ask the user.
- [ ] **Step 6: Commit.** `feat(pocketcashier): Reload & Bills on the phone, online and off`

### Task 14: Print & Copy, Combos, Alternatives

**Files:**
- Create: `offline/routes/documents.ts`, `offline/routes/combos.ts`, `offline/routes/alternatives.ts`, one test each
- Modify: `plugins/documents/`, `plugins/combos/`, `plugins/alternatives/` (one `AddOfflineData` each), their DB tests

**Interfaces:** Consumes `AddOfflineData`, `addRoutes`.

Routes: Documents `GET /till`, `GET /quote`, `POST /jobs` (queue), `POST /own-use` (queue). Combos `GET /till`, `GET /settings`, `POST /sold` (queue). Alternatives `GET /till/:pid`.

- [ ] **Step 1: Write failing tests:** `an offline quote equals the server's for the same job`: run shared cases from a JSON fixture through the Go handler and through `documents.ts`; `combo deal rings at its price offline`; `combo sold replays with the real sale id`; `alternatives swap offers the group's products in stock offline`.
- [ ] **Step 2: Run them.** Expected: FAIL.
- [ ] **Step 3: Implement.** The quote logic exists in Go only; the phone's copy must match it, and the shared fixture keeps them equal.
- [ ] **Step 4: Run them.** Expected: PASS.
- [ ] **Step 5: Screens as Task 13, then commit.** `feat(pocketcashier): Print & Copy, Combos and Alternatives on the phone`

### Task 15: Pharmacy

**Files:**
- Create: `offline/routes/pharmacy.ts`, `offline/routes/pharmacy.test.ts`
- Modify: `plugins/pharmacy/` (`AddOfflineData`), its DB test

**Interfaces:** Consumes `AddOfflineData`, `addRoutes`, `tempRef`.

Routes: `GET /patients`, `GET /patients/:id`, `GET /patients/:id/scripts`, `GET /medicines`, `GET /doses`, `GET /notes`, `GET /till-info`, `GET /sales/recent` (from the 30-day copy), `POST /patients`, `POST /patients/from-customer/:cid`, `PUT /patients/:id`, `POST /prescriptions`, `PUT /prescriptions/:id`, `POST /prescriptions/:id/active`, `POST /dispenses` (all writes queue).

- [ ] **Step 1: Write failing tests:** `finds a patient offline`; `a script's medicines add to the cart`; `a new patient offline has a temp id, and a script and sale on it replay with the real id`; `a controlled medicine sells offline, and the dispense reaches the register when replayed`; `a dose label button is hidden offline`, if the print call fails offline instead, then `it shows a message, not an error`.
- [ ] **Step 2: Run them.** Expected: FAIL.
- [ ] **Step 3: Implement.** The part holds patients, active scripts, medicine details and doses.
- [ ] **Step 4: Run them.** Expected: PASS.
- [ ] **Step 5: Screens as Task 13, then commit.** `feat(pocketcashier): Pharmacy on the phone, online and off`

### Task 16: Repairs

**Files:**
- Create: `offline/routes/repairs.ts`, `offline/routes/repairs.test.ts`
- Modify: `plugins/repairs/` (`AddOfflineData`), its DB test

**Interfaces:** Consumes `AddOfflineData`, `addRoutes`, `tempRef`.

Routes: `GET /config`, `GET /suggest`, `GET /jobs`, `GET /jobs/:id`, `GET /jobs/:id/bill`, and these writes (queue): `POST /jobs`, `PUT /jobs/:id`, `POST /jobs/:id/status`, `POST /jobs/:id/deposits`, `POST /jobs/:id/parts`, `POST /jobs/:id/charges`. Read `plugins/repairs` for what the panel's collect flow calls, and add any route it uses.

- [ ] **Step 1: Write failing tests:** `a new job offline shows a temporary number and replays with a real one`; `a deposit on it replays against the real job`; `collecting a job offline sells its bill`; `a job collected on the PC and again on the phone fails with a note and keeps the cash`; `the job slip button shows a message offline`.
- [ ] **Step 2: Run them.** Expected: FAIL.
- [ ] **Step 3: Implement.** The part holds jobs not yet collected, and config.
- [ ] **Step 4: Run them.** Expected: PASS.
- [ ] **Step 5: Screens as Task 13, then commit.** `feat(pocketcashier): Repairs on the phone, online and off`

---

## Finish

### Task 17: A whole day with the PC off; docs and legal

**Files:**
- Create: `web/e2e/pocket-day.spec.ts`, `plugins/pocketcashier/day_db_test.go`
- Modify: `docs/guide/plugins/pocketcashier.md`, `docs/guide/till.md` (remove "can't open tickets yet"), `web/e2e/pocket-shots.spec.ts`, `docs/legal/README.md`, `docs/legal/agreement/lawyer-items.json`, `README.md` if the feature grid mentions the phone, `docs/status.md`

- [ ] **Step 1: Write the day test** `TestOfflineDayBalances`, every plugin compiled in, replaying one fixed list of actions: open, plain sale, reload, bill payout, print job, combo, patient + script sale, new repair + deposit, credit payment, return, warranty swap, write-off, convert, withdraw, close. Assert: till expected cash equals the sum of the phone's own figures; each customer's balance and each product's stock equal hand-worked values; every action is `applied` or flagged, none silently lost.
- [ ] **Step 2: Write the e2e:** same day in the browser with the PC blocked (the way `pocket.spec.ts` blocks it), then unblocked; **Phones → What came in** lists it all.
- [ ] **Step 3: Run them.** `DATABASE_URL=… go test ./plugins/pocketcashier -run TestOfflineDay`, then `make plugins P=all`-based Playwright per `web/playwright.config.ts`. Expected: PASS. Fix what they find, test-first.
- [ ] **Step 4: Update the guide.** Describe the phone as the power-cut backup that also goes portable. Remove "sold at the counter for now". Say what's PC-only (Labels, Suppliers, Expenses, Low stock, Customer requests), what needs the PC to print, and that offline Receipts lists only sales and this phone's work. No line encouraging daily use.
- [ ] **Step 5: Retake screenshots.** Add the new screens to `pocket-shots.spec.ts`; run `make docs-shots`; look at them.
- [ ] **Step 6: Legal.** A month of sales and patient details on a phone, Block only reaching a phone that connects: add to `docs/legal/README.md` and `lawyer-items.json`; run `make agreement`.
- [ ] **Step 7: Full check.** `make test`, `make test-web`, `make verify`, full Playwright. Expected: all pass; any skip or flake reported honestly.
- [ ] **Step 8: Fresh review** of the whole milestone; fix findings test-first.
- [ ] **Step 9: Tidy.** Update `docs/status.md`; delete this plan and the spec. Commit: `docs: Pocket Cashier release 2 done`
