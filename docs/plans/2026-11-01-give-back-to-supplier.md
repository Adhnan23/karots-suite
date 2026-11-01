# Give a lot back to its supplier: implementation plan

> Execute tasks in order and tick the boxes. Delete this plan and its spec when done.

**Goal:** count a lot, hand it to its supplier and have it wait there until settled; see close-to-expiry lots on
"From this supplier"; record that the supplier was told.

**Architecture:** one new stock operation (`GiveBack`) settles the count difference between the product's lots, then
uses the existing damage write-off, so losses, the waiting list and `RecoverSheet` keep working. A `given_back` flag on
the movement keeps it on the waiting list past `claimWindow`.

**Spec:** `docs/specs/2026-11-01-give-back-to-supplier.md`

## Global constraints

- Core only (`make check-core`); files at 500 lines or fewer; tests first; one commit per task on `main`; no AI attribution.
- Theme tokens only; every screen works at 360px, every palette, both modes.
- Kill anything on 3000 / 5173 before Playwright. Never run `go test` during a Playwright run.
- Never block: a difference in the count never refuses a hand-over.

## Review focus

Each has a test in the owning task.

1. The product's `stock.quantity` is below the count (it can drift from the lots): the hand-over still works.
2. A decimal count on a loose product (2.5): differences stay exact.
3. Pressing Give back twice (or a lot already emptied): the second is refused and nothing moves twice.
4. A lot of another supplier sent to this supplier's route: refused.
5. A count of zero: no loss, nothing waiting, the lot still ends at zero.

---

### Task 1: `stock.GiveBack` and the told date

**Files:**
- Create: `migrations/00020_give_back.sql`, `internal/features/stock/giveback.go`, `internal/features/stock/giveback_test.go`
- Modify: `internal/features/stock/batches.go` (`Batch.SupplierToldOn`, `SupplierID`), `internal/features/stock/stock.go`
  (`Movement.GivenBack`), `AGENTS.md` (migration list)

**Interfaces (produces):**
- `stock_batches.supplier_told_on date NULL`, `stock_movements.given_back boolean NOT NULL DEFAULT false`
- `lotSelect` also gives `s.id AS supplier_id` as `Batch.SupplierID *int64`
- `type GiveBackInput struct { BatchID int64; Counted string; SupplierName string }`
- `func (s *Service) GiveBack(ctx, in GiveBackInput, userID int64) (lossID int64, err error)`; `lossID` is 0 for a zero count
- `func (s *Service) MarkSupplierTold(ctx, batchID int64) error`: sets the date to today
- `func (s *Service) SupplierLots(ctx, supplierID int64) ([]Batch, error)`: live lots with an expiry date, soonest first

- [ ] **Step 1: Write failing tests** in `giveback_test.go`, one per row, asserting lot quantities, `stock.quantity` and the movements:
  - counted 12, lot 10, other lot 5: lot 0, other 3, stock total −12, loss of 12 at this lot's cost
  - counted 12, lot 10, other lot 1: other 0, plus an adjust of +1; total −11 net
  - counted 12, lot 10, no other lot: an adjust of +2
  - counted 7, lot 10, other lot 5: lot 0, other 8, loss of 7
  - counted 7, lot 10, no other lot: an adjust of −3, loss of 7
  - counted 2.5, lot 4 on a loose product, other lot 1: other 2.5, loss of 2.5
  - counted 0, lot 10, other lot 5: other 15, `lossID` 0, no damage movement
  - `stock.quantity` 3, lot 10, counted 10: succeeds, no "not enough stock"
  - empty lot, or Give back twice: conflict, nothing moves
  - bad count (negative, not a number): validation error
  - the damage movement has `given_back` true and note `given back to <supplier>`
- [ ] **Step 2:** Run `go test ./internal/features/stock -run GiveBack`; it fails to compile.
- [ ] **Step 3:** Write the migration and implement. In one transaction: lock the lot and the product's other live lots in
  FEFO order; settle the difference (more: take from those lots in order, the rest is an `adjust` up; less: onto the first
  of those lots, else an `adjust` down); raise `stock.quantity` by an `adjust` if it's below the count; then `consumeTx`
  with reason `damage` naming the lot and mark that movement `given_back`. A move between lots writes an `adjust` movement
  of quantity 0 whose note says how many moved and between which lots, so Movements shows it.
- [ ] **Step 4:** Run the same tests; they pass. Run `go test ./internal/features/stock`.
- [ ] **Step 5:** Commit.

### Task 2: the supplier claims API

**Files:**
- Modify: `internal/features/recovery/claims.go`, `internal/api/supplier_claims.go`, `internal/api/dto_reports_ops.go`,
  `internal/api/dto_stock.go` (`LotRow.SupplierID`, `LotRow.ToldOn`), `internal/api/admin_stock.go` (`lotRow`),
  `internal/api/supplier_recovery_test.go`

**Interfaces:**
- Consumes: Task 1's `GiveBack`, `MarkSupplierTold`, `SupplierLots`, `Batch.ExpiringSoon`, `Batch.Expired`
- Produces:
  - `ClaimLot` gains `ToldOn string` (`YYYY-MM-DD` or `""`) and `WarnDays int`; `SupplierClaims` gains `Expiring []ClaimLot`
  - `LossRow` gains `GivenBack bool`
  - `POST {base}/suppliers/:id/claims/lots/:lot/give-back` body `GiveBackRequest{Counted string}` answers `ConsumeResult`
  - `POST {base}/suppliers/:id/claims/lots/:lot/told` answers 204
  - `POST .../claims/expired/:lot` and `SupplierClaimWriteOff` are removed (the spec replaces them)

- [ ] **Step 1: Write failing tests** in `supplier_recovery_test.go`. Convert `TestSupplierClaims` to the new route so it
  keeps covering the same flow (expired, give back, waiting, counter settles). Add `TestSupplierClaimsGiveBack`:
  - a lot inside its warning days is in `Expiring`, not `Expired`; one outside is in neither
  - `told` sets `ToldOn` on the lot in the claims answer and on the Batches page
  - a lot of another supplier: both routes answer 404, nothing changes
  - a zero count: answers `LossID` 0 and nothing is added to `Waiting`
  - a given-back movement older than `claimWindow` stays in `Waiting` (back-date `created_at` by SQL); a plain write-off that old doesn't
- [ ] **Step 2:** Run `go test ./internal/api -run SupplierClaims`; it fails.
- [ ] **Step 3:** Implement. `Claims` returns expired, expiring and waiting, building the lots from `SupplierLots`. The
  waiting query keeps the window for ordinary write-offs only: `(m.given_back OR m.created_at >= $2)`. Audit both actions
  with `h.audit`. Run `make types`.
- [ ] **Step 4:** Run `go test ./internal/api ./internal/features/recovery`; they pass.
- [ ] **Step 5:** Commit.

### Task 3: the screens

**Files:**
- Create: `web/src/features/purchasing/GiveBackSheet.tsx`, `web/e2e/give-back.spec.ts`
- Modify: `web/src/features/purchasing/SupplierClaims.tsx`, `web/src/features/purchasing/suppliers/SupplierPage.tsx`,
  `web/src/features/admin/stock/BatchesPage.tsx`, `web/e2e/batches.spec.ts`

**Interfaces:**
- Consumes: Task 2's routes and types
- Produces: `GiveBackSheet({ lot, supplierId, supplierName, sym, base, onClose, onDone })`, where `lot` is
  `{ lotId, product, unit, qty, expiry, cost }`; `onDone(lossId)` opens `RecoverSheet` only if the user chooses to settle now

- [ ] **Step 1: Write the failing e2e** in `give-back.spec.ts`, using a delivery that expires in 20 days from a new supplier:
  - the supplier page's panel shows the lot under Close to expiry; Told them shows "told" with today's date
  - Give back from the Batches page: counting 3 against a system 2 with another lot of 5 leaves that lot at 4, the sheet's
    line says so before saving, and the row becomes "3 given back"
  - Settle on that row with money lowers what's owed
  - Update `batches.spec.ts`: "Write off & settle" becomes Give back with a typed count, then the same replacement flow
- [ ] **Step 2:** Run it; it fails (kill 3000 / 5173 first).
- [ ] **Step 3:** Implement.
  - **Panel:** three groups with Told them and Give back on lots. The heading counts all three (`n to settle with them`).
    Waiting rows say "given back" when `givenBack`.
  - **Sheet:** the `FormSheet` kit; one field, **Counted on the shelf**, empty and required. Below it, one line that
    changes with the number: `2 more than the system: taken from this medicine's other batches` /
    `3 fewer than the system: moved to its other batches` / `Nothing goes back`. Use "this product's", not "medicine".
    After saving, a dialog (never a toast) asks: **Settle now** or **Later**.
  - **Batches page:** Give back on lots with `supplierId`; "told 3 Nov" when set. Keep Write off and Write off all.
  - **Supplier page:** render `SupplierClaimsPanel`.
- [ ] **Step 4:** Run the two specs; they pass. Take shots at 360px and 1280px, light and dark, and review them.
- [ ] **Step 5:** Commit.

### Task 4: docs and the whole check

**Files:** `docs/guide/purchasing.md`, `docs/guide/stock.md`, `docs/status.md`, `docs/images/stock/batches.jpg`

- [ ] **Step 1:** Add **Giving a batch back** to `purchasing.md` and link it from `stock.md`. Mark A3 done in `status.md`.
- [ ] **Step 2:** Full checks in turn: `make test`, `make test-web`, `make check-core`, then all of Playwright.
- [ ] **Step 3:** A fresh reviewer reads the whole change; fix what's found test-first.
- [ ] **Step 4:** Retake the screenshots if the Demo Room can be rebuilt; otherwise say so.
- [ ] **Step 5:** Delete this plan and the spec. Commit.
