# M8e part 2 — the phone sells like the PC till — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The Pocket Cashier phone does everything the PC cashier does on Sell and Pay (discounts, weights, Which price, packs, serials, split payments, note tray, keypad, keep change, change breakdown, customers, quick item), in a phone layout, live and offline; the PC till gets a "+ Quick item" button.

**Architecture:** The phone keeps the PC till's cart reducer and logic (`web/src/features/till/cart/*`) and moves Pay onto the reducer's own tenders (`tenders`, `draft`, `keep`, `roundToAccount`) so the request is `buildSaleRequest` exactly as the PC builds it. New phone sheets live in `web/src/plugins/pocketcashier/screens/`; prop-driven PC pieces (`NoteTray`, `TouchKeys`) are reused as-is, provider-bound ones (`PricePick`, `PackPick`, `CustomerPicker`, `AddCustomer`, `QuickItem`, `ChangeBanner` — all call `useTill()`) are re-done as phone sheets over the same helpers. The server gains two copy fields and a Review-mode serial flag.

**Tech Stack:** Go (Echo, sqlx), React + TS, TanStack Query, Vitest + Testing Library, Playwright.

**Spec:** `docs/specs/2026-11-01-m8e-phone-till-parity-design.md` (read it first). Release 1 spec: `docs/specs/2026-10-29-m8e-pocket-cashier-design.md`. Ledger: `.superpowers/sdd/2026-10-30-m8e-pocket-cashier/progress.md` (append a line per task).

**Branch:** `m8e-pocket-cashier` (not merged until this plan and the old plan's Task 27 are done).

## Global Constraints

- The phone never re-implements pricing: totals, discounts, packs, lots and rounding come from `web/src/features/till/cart/*`. The same cart must give the same total on the phone and the PC.
- Never stop a sale: PC prompts (oversell, over limit, shortfall) become non-blocking notes on the phone; Finish always sends; the server flags.
- Offline works for everything except quick item and add customer (they say so in place when the PC isn't reachable).
- A phone sale never pops the drawer; no printing from the phone (online the receipt prints at the counter, as now).
- Core stays plugin-inert (`make check-core`); Pocket Cashier adapts to other plugins from its own side (AGENTS hard rule).
- Files ≤ 500 lines; theme tokens only (`theme/guard.test.ts`); every palette light and dark; phone screens at 390 px; PC/admin at 360 px.
- `make types` after any `dto*.go` change. No Claude/AI attribution in commits. One commit per task.
- Kill anything on 3100/5173/8443 before Playwright; Go DB tests and Playwright share `suite_db` — never run both at once.

## Review Focus

1. **A person signed in offline, now live:** the phone has no session cookie for that person, so `/till/*` calls (print, quick item, add customer, detail) answer 401. Expect a plain message in place ("Switch person and sign in again while the PC is up"), never a silent failure. Pinned in Task 7 (`quickItem.test.ts`).
2. **Same cart, same total:** a cart with a percent bill discount, a fixed line discount, a pack line and a lot-priced line totals the same on the phone as on the PC. Pinned in Task 11 (e2e: the phone's shown total equals the created sale's `total`).
3. **Keep the change offline:** a cash sale with kept change, made offline, leaves the drawer's expected cash at paid − change given, not paid − full change. Pinned in Task 9 (`drawer.test.ts`).
4. **A serial already sold, rung offline:** the replay applies the sale and flags `serial`; it is not stored `failed`. Pinned in Task 2.
5. **Decimals on a whole-only product:** typing 1.5 on a product that doesn't allow decimals rounds to whole, as the PC's reducer does; a weighed product keeps 1.25. Pinned in Task 4.

---

## Phase A — core and server

### Task 1: PC till "+ Quick item" button

**Files:**
- Modify: `web/src/features/till/catalog/CatalogPanel.tsx` (a button beside the search box)
- Test: `web/e2e/till.spec.ts` (one new test)

**Interfaces:**
- Consumes: `ui.open({ kind: 'quickItem', barcode: string, name: string })` (`TillUi.tsx`), already used by `useEntry.ts`.

- [ ] **Step 1: Write the failing test** `test('the till opens Quick item from its own button', …)`: sign in as `0770000002`, open `/till`, click `getByRole('button', { name: '+ Quick item' })`, expect `getByRole('dialog', { name: /Quick item/ })` visible with empty name and barcode.
- [ ] **Step 2: Run it to see it fail.** `make e2e-seed && cd web && bunx playwright test e2e/till.spec.ts -g "Quick item from its own button"` → FAIL (no button).
- [ ] **Step 3: Implement** the button (label `+ Quick item`, title "Sell an item that isn't in the catalogue yet") calling `ui.open({ kind: 'quickItem', barcode: '', name: '' })`. Theme tokens only; fits 1024 px.
- [ ] **Step 4: Run it to see it pass.**
- [ ] **Step 5: Commit.** `feat(till): a Quick item button beside the search box`

### Task 2: Serials never refuse a Review sale

**Files:**
- Modify: `internal/features/sales/service_lines.go:98-126`, `internal/features/sales/flags.go` (`FlagSerial = "serial"`), wherever `Review` flags are collected after `InsertSale` (`service.go`)
- Test: `internal/features/sales/replay_db_test.go`

**Interfaces:**
- Produces: `sales.FlagSerial = "serial"`. With `CreateInput.Review`, a serial already on record, repeated in the sale, or a serial count that doesn't match the quantity is kept and flagged (note names the serial / the counts); warranty units are recorded for the serials given (a repeated or known serial is not recorded twice). Without `Review` nothing changes.

- [ ] **Step 1: Write the failing DB tests:** `TestReviewSerialOnRecordFlagged` (a serial already in `warranty_units`, `Review: true` → sale saved, one `serial` flag whose note contains the serial); `TestReviewSerialCountFlagged` (qty 2, one serial, `Review: true` → saved, `serial` flag mentioning `1 of 2`); `TestSerialStillRefusedWithoutReview` (same inputs, `Review: false` → the existing validation errors).
- [ ] **Step 2: Run them to see them fail.** `set -a; . ./.env; set +a; go test ./internal/features/sales -run 'TestReviewSerial|TestSerialStill' -count=1`
- [ ] **Step 3: Implement.** Thread `in.Review` into the serial checks; collect flag notes the way `oversold` / `over_limit` already are.
- [ ] **Step 4: Run them to see them pass,** then `go test ./internal/features/sales/... ./plugins/pocketcashier -count=1`.
- [ ] **Step 5: Commit.** `feat(sales): a Review sale flags serial problems instead of refusing`

### Task 3: Lot prices and the shop's notes in the copy

**Files:**
- Modify: `internal/api/dto_copy.go`, `internal/api/till_copy.go` (fill + `TillCopyVersion`), then `make types`
- Test: `internal/api/till_copy_test.go`

**Interfaces:**
- Produces: `TillCopy.PriceOptions map[string][]Lot \`json:"priceOptions"\`` (exactly what `/till/price-options` answers: `products.PriceOptions` → `lotsOf`), `TillCopy.Denominations []Denomination \`json:"denominations"\`` (the active notes and coins, as the drawer answers them). The version changes when a batch's price or remaining qty changes and when denominations change.

- [ ] **Step 1: Write the failing test** `TestTillCopyLotsAndNotes`: E2E-style product with two batches at 100 and 120 → `c.PriceOptions[id]` has 2 lots with prices `"100.00"`/`"120.00"` (match `lotsOf`'s format); `c.Denominations` non-empty with at least one `IsNote`. `TestTillCopyVersionChanges` gains: update a batch's `selling_price` → version changes.
- [ ] **Step 2: Run to see it fail.** `go test ./internal/api -run TestTillCopy -count=1`
- [ ] **Step 3: Implement,** reusing `h.t.products.PriceOptions` / the drawer's denomination query rather than new SQL where `BuildTillCopy` can reach them (it takes a `*sqlx.DB`; construct the services the way it already builds `cashregister`).
- [ ] **Step 4: Run to see it pass;** `make types`; `cd web && bunx tsc -b`.
- [ ] **Step 5: Commit.** `feat(api): the till copy carries lot prices and the shop's notes`

## Phase B — the phone's cart

### Task 4: Line sheet — quantity, discount, serials, service price

**Files:**
- Create: `web/src/plugins/pocketcashier/screens/LineSheet.tsx`, `LineSheet.test.tsx`
- Modify: `screens/CartBar.tsx` (tapping a line opens the sheet; keep ± buttons)

**Interfaces:**
- Consumes: reducer actions `setQty {key, qty, lots}`, `lineDiscount {key, discount, discountType}`, `serial {key, index, value}`, `remove {key}`; `lotsFor(copy.priceOptions, productId)` for `lots`.
- Produces: `LineSheet({ line, cart, dispatch, copy, onClose })`. A service line edits `priceOverride` and `description` through the same actions the PC uses (check `cart/store.ts` for the service-line action; add none if one exists).

- [ ] **Step 1: Write the failing test** (`render` + the real `tillReducer` in a tiny harness): a decimal product line → type `1.25` in the qty keypad → state line qty `1.25`; a whole-only line → type `1.5` → qty `2` (the reducer's rounding, Review Focus 5); discount `10` `%` → `lineNet` = 90% of gross; a serial-tracked line at qty 2 shows 2 serial inputs.
- [ ] **Step 2: Run to see it fail.** `cd web && bunx vitest run src/plugins/pocketcashier/screens/LineSheet.test.tsx`
- [ ] **Step 3: Implement** a bottom sheet (`components/ui/sheet`) with a numeric keypad for qty, fixed/% discount (`Seg`-style toggle), serial inputs (count = qty), price + description for service lines, Remove.
- [ ] **Step 4: Run to see it pass.**
- [ ] **Step 5: Commit.** `feat(pocketcashier): line sheet — qty keypad, discount, serials, service price`

### Task 5: Adding — Which price? and the pack picker

**Files:**
- Create: `web/src/plugins/pocketcashier/addFlow.ts`, `addFlow.test.ts`, `screens/PriceSheet.tsx`, `screens/PackSheet.tsx`
- Modify: `screens/Sell.tsx` (tap / scan goes through `addFlow`)

**Interfaces:**
- Consumes: `copy.priceOptions` (Task 3), `lotsFor`, `lotExpired`, `lotLabel` (`cart/lines.ts`), `linePacks` (`cart/packs.ts`), reducer `add {product, lot?, lots, units?}`.
- Produces: `addFlow(copy: TillCopy, p: CopyProduct, scanned: boolean, packId?: number): { kind: 'add'; units?: number } | { kind: 'price'; lots: Lot[] } | { kind: 'pack' }` — the same decision as the PC's `useAddProduct` (lots that disagree on price or an expired selling lot → `price`; a product with packs, not scanned, not a service → `pack`; a pack's own code → `add` with that pack's units).

- [ ] **Step 1: Write the failing test:** two lots 100/120 → `price`; one lot → `add`; packs and not scanned → `pack`; scanned pack code → `add` with `units` = pack size; service with packs → `add`.
- [ ] **Step 2: Run to see it fail.**
- [ ] **Step 3: Implement** `addFlow` and the two sheets (PriceSheet lists lots with `lotLabel` and price, expired marked; PackSheet lists "1 unit" + each pack). The line's `expectPrice` is already the lot price via `unitPrice` (`sale.ts`).
- [ ] **Step 4: Run to see it pass,** plus `catalog.test.ts`.
- [ ] **Step 5: Commit.** `feat(pocketcashier): Which price? and the pack picker on the phone`

### Task 6: Cart summary — bill discount, retail/wholesale, product info

**Files:**
- Create: `web/src/plugins/pocketcashier/screens/CartSummary.tsx`, `screens/InfoSheet.tsx`, `CartSummary.test.tsx`
- Modify: `screens/CartBar.tsx` (a "Discount / Retail·Wholesale" row on the opened cart), `screens/Sell.tsx` (ⓘ on a product row opens InfoSheet)

**Interfaces:**
- Consumes: reducer `billDiscount {value?, discountType?}`, `saleType {saleType}`; `GET /api/v1/till/products/:id/detail` (live only; offline the sheet shows the copy's card: price, wholesale, stock, packs, badges).

- [ ] **Step 1: Write the failing test:** bill discount `10` `%` on a 1,000 cart → shown total `900.00` and equal to `saleTotal`; switching to wholesale re-prices a line that has a wholesale price.
- [ ] **Step 2: Run to see it fail.**
- [ ] **Step 3: Implement.** InfoSheet shows cost only when the copy person has `canSeeCost` (from `copy.people`).
- [ ] **Step 4: Run to see it pass.**
- [ ] **Step 5: Commit.** `feat(pocketcashier): bill discount, retail/wholesale and product info on the phone`

### Task 7: Quick item on the phone

**Files:**
- Create: `web/src/plugins/pocketcashier/quickItem.ts`, `quickItem.test.ts`, `screens/QuickItemSheet.tsx`
- Modify: `screens/Sell.tsx` (a `+ Quick item` button beside search; a scan/search with no match offers it, prefilled as the PC does: digits → barcode, else name)

**Interfaces:**
- Consumes: `POST /api/v1/till/quick-item` and `GET /api/v1/till/barcode/generate` (same bodies as `features/till/catalog/QuickItem.tsx`); `Scanner` for the barcode.
- Produces: `createQuickItem(input): Promise<{ product: TillProduct } | { error: string }>` mapping a network failure to "The PC isn't reachable — sell it at the counter" and a 401 to "Switch person and sign in again while the PC is up" (Review Focus 1); the new product is added to the cart and the copy refreshed.

- [ ] **Step 1: Write the failing test** (fetch mocked): 200 → product; TypeError → the PC-down message; 401 → the sign-in-again message.
- [ ] **Step 2: Run to see it fail.**
- [ ] **Step 3: Implement** the helper and the sheet (name, price, qty, barcode with camera scan and Generate).
- [ ] **Step 4: Run to see it pass.**
- [ ] **Step 5: Commit.** `feat(pocketcashier): Quick item on the phone — button, offered on no match, camera barcode`

## Phase C — the phone's Pay

### Task 8: Pay on the reducer's tenders — split payments and Online

**Files:**
- Modify: `web/src/plugins/pocketcashier/sale.ts` (+ `sale.test.ts`), `screens/Pay.tsx`, `screens/PayTenders.tsx`

**Interfaces:**
- Consumes: reducer `draft {patch}`, `commitDraft`, `removeTender {index}`, `customer {id}`; `effectivePayments(state, total)` and `buildSaleRequest` (`cart/checkout.ts`); `copy.places` (`card` → Card, `bank` → Online, cashier-accessible only).
- Produces: `payIssue(cart: TillState, copy: TillCopy): string` (was `(cart, tender, copy)`) checking the payments from `effectivePayments`: credit/cheque need a customer, card/online need their account, cheque fields (`chequeMissing`), more card/online/cheque than due → the PC's message. `payments()` and the local `Tender` type are removed; `saleBody(cart, effectivePayments(cart, total))`.

- [ ] **Step 1: Update `sale.test.ts`:** cash 500 + card 300 on an 800 cart → two payments in the body; Online without a bank account → `'Choose the account for the online payment'`; credit without a customer → `'Pick a customer to sell on account'`; an untouched cash draft on a walk-in = exact cash (as the PC).
- [ ] **Step 2: Run to see it fail.**
- [ ] **Step 3: Implement:** tender buttons fill `draft`; "Add payment" commits it; committed lines listed with remove; still to pay / change shown from `balanceDue` / `changeDue`.
- [ ] **Step 4: Run to see it pass.**
- [ ] **Step 5: Commit.** `feat(pocketcashier): split payments and Online on the phone, on the till's own tenders`

### Task 9: Note tray, keypad, quick cash, keep change, change by note

**Files:**
- Create: `web/src/plugins/pocketcashier/change.ts`, `change.test.ts`, `screens/ChangeBox.tsx`
- Modify: `screens/PayTenders.tsx`, `drawer.ts` (+ `drawer.test.ts`)

**Interfaces:**
- Consumes: `TouchKeys` / `NoteTray` (prop-driven, reuse as-is; notes/coins from `copy.denominations`, `LKR_NOTES` when empty), `quickCash` (all, up to 8), reducer `keep {value}` / `roundToAccount {on}`, `coinKeep`, `keptAmount`, `actualChange`, `roundable`, `notesFor`.
- Produces: `changeNotes(change: number, copy: TillCopy): { value: number; qty: number; isNote: boolean }[]` (from `copy.denominations`, notes before coins). `expectedCash` counts a sale's cash in as `cash paid − (changeGiven !== '' ? changeGiven : full change)`.

- [ ] **Step 1: Write the failing tests:** `changeNotes(1650, copy with 1000/500/100/50 notes)` → `[1000×1, 500×1, 100×1, 50×1]`; drawer: an unsent sale of 640 paid 1000 cash with `changeGiven: '355'` (kept 5) → expected rises by 645 (Review Focus 3).
- [ ] **Step 2: Run to see them fail.**
- [ ] **Step 3: Implement:** Cash shows quick-cash chips (all), the TouchKeys (Notes / Keypad switch) for the amount, the change box with "Keep Rs x (no coins)" and "round to the customer's account" when a customer is picked (exactly the PC's rules), and the note-by-note breakdown under "Give change". Same breakdown on the done screen.
- [ ] **Step 4: Run to see them pass.**
- [ ] **Step 5: Commit.** `feat(pocketcashier): note tray, keypad, all quick cash, keep change and change by note`

### Task 10: Customers on Pay — balance, limit, add new

**Files:**
- Create: `web/src/plugins/pocketcashier/screens/CustomerSheet.tsx`, `CustomerSheet.test.tsx`
- Modify: `screens/Pay.tsx` (replace the inline `CustomerPick`)

**Interfaces:**
- Consumes: `copy.customers` (balance, creditLimit), `balanceText(sym, c)` (`pay/CustomerPicker.tsx` — needs `TillCustomer`; map `CopyCustomer` → `{ id, name, phone, creditLimit, outstanding: balance }`), `POST /api/v1/till/customers` (live only; same body as `pay/AddCustomer.tsx`; offline / 401 messages as Task 7).
- Produces: picking sets `customer {id}`; a new customer is picked at once and appears after the next copy refresh.

- [ ] **Step 1: Write the failing test:** search by phone finds the customer and shows their balance text and limit; over-limit on account shows the PC's over-limit note but Finish stays enabled.
- [ ] **Step 2: Run to see it fail.**
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run to see it pass.**
- [ ] **Step 5: Commit.** `feat(pocketcashier): customers on Pay — balance, limit, add new`

## Phase D — proof and docs

### Task 11: End-to-end parity, UI gate, guide

**Files:**
- Modify: `web/e2e/pocket.spec.ts` (a parity test, and the PC → phone move test), `web/e2e/pocket-shots.spec.ts` (line sheet, Which price, Pay with note tray and change box), `docs/guide/plugins/pocketcashier.md`

- [ ] **Step 1: Write the tests:**
  - `parity`: on the phone add a weighed amount (a product with `unit_allow_decimal`; seed one in the test with the admin API if `e2e-till.sql` has none), E2E Lot Soap via Which price? at 120, a 10% bill discount and a fixed line discount; pay 500 cash + the rest by card; the phone's shown total equals the PC's total for the same lines (build the same `SaleRequest` and compare against the created sale's `total`); the sale lands with two payments (Review Focus 2).
  - `PC → phone move`: phone signed in with a cart; the same person signs in on a PC context → within 30 s the phone shows the sign-in screen with "is now on", and the PC's Held lists "Moved to another device".
- [ ] **Step 2: Run them.** Kill 3100/5173/8443, then build with all plugins and `bunx playwright test e2e/pocket.spec.ts`; then the full suite stays green.
- [ ] **Step 3: UI gate:** `SHOTS=pocket bunx playwright test e2e/pocket-shots.spec.ts`; review every new screen in Studio / Night / Bazaar × light / dark at 390 px against the existing phone look; fix overflow and raw colours.
- [ ] **Step 4: Guide:** update `docs/guide/plugins/pocketcashier.md` Selling / Pay bullets (discounts, weights, Which price, packs, serials, split payments, note tray, keep change, quick item); `go test ./internal/docscheck`.
- [ ] **Step 5: Commit.** `test(pocketcashier): parity end-to-end, PC→phone move, shots; guide`

---

After this plan: finish `docs/plans/2026-10-30-m8e-pocket-cashier.md` Task 27 (the user's phone test, clean-up, merge to main).
