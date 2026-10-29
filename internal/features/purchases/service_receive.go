package purchases

import (
	"context"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/features/suppliers"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// lotCost is the per-unit inventory cost of a received lot. When nothing reduced
// the price (no free units and the paid basis equals list × qty) it is exactly
// the list cost — no divide, no rounding drift — and a zero/empty basis still
// lets the batch inherit the product's own cost (a free or fully-discounted
// line). Otherwise it is what was actually paid for the lot (costBasis: the line
// net after its own discount AND its share of the bill discount) spread across
// every unit that arrived (gotQty = paid + free), so a discount or freebie
// surfaces as real margin when the units sell.
// ponytail: rounds to 2dp (the column's scale), so blended×gotQty can drift a
// few cents from paid (1000 over 12 = 83.33, ×12 = 999.96) — returning a whole
// odd-split lot leaves those cents owed on the bill. Accepted; widen the column
// to 4dp only if that ever matters.
func lotCost(costBasis, gotQty, freeQty, listCost decimal.Decimal) decimal.Decimal {
	if !gotQty.IsPositive() {
		return decimal.Zero
	}
	if !costBasis.IsPositive() {
		return listCost
	}
	// Nothing reduced the price: keep the exact list cost (no divide, no drift).
	if freeQty.IsZero() && costBasis.Equal(gotQty.Mul(listCost).Round(2)) {
		return listCost
	}
	return costBasis.DivRound(gotQty, 2)
}

// applyReceivedLines inserts purchase item rows and posts their inventory effects
// (stock increment, FEFO batch, movement, expiry flag, pricing refresh), then the
// supplier payable. Shared by instant GRNs (Create) and draft receipts (Receive).
// Each line's OrderedQty is preserved as supplied by the caller.
func applyReceivedLines(ctx context.Context, repo *Repository, stk *stock.Repository, sup *suppliers.Repository, purchaseID, supplierID int64, lines []PurchaseItem, owed decimal.Decimal, userID int64) error {
	// The whole-bill discount is spread across the lines pro-rata to each line's
	// net, so it lowers the received COST (not just what is owed): each line's
	// cost basis is scaled by owed/netSum. With no bill discount owed == netSum,
	// so the basis is the line net untouched (and lotCost keeps the exact list
	// cost when no discount or freebie touched the line at all).
	netSum := decimal.Zero
	for _, ln := range lines {
		netSum = netSum.Add(ln.Subtotal)
	}
	billScaled := owed.LessThan(netSum) && netSum.IsPositive()
	for _, ln := range lines {
		ln.PurchaseID = purchaseID
		itemID, err := repo.InsertItemReturningID(ctx, purchaseID, ln)
		if err != nil {
			return apperr.Internal("failed to save purchase item", err)
		}
		// Free bonus units physically arrive, so stock, the batch, and the
		// movement all count paid + free. Only the payable (subtotal, above) left
		// them out.
		gotQty := ln.Quantity.Add(ln.FreeQty)
		if err := stk.Increment(ctx, ln.ProductID, gotQty); err != nil {
			return apperr.Internal("failed to update stock", err)
		}
		// Create the batch (carries expiry + cost) for FEFO depletion later. The
		// GRN line's selling price rides along so THIS lot keeps ringing up at the
		// price it was received at, even after a later delivery moves the shelf
		// price on. Blank (zero) leaves the lot following the product, as before.
		//
		// The lot's cost is what was actually paid spread across every unit
		// received — buy 10 @ 100 + 2 free values all 12 at 1000/12, so the
		// freebie surfaces as margin when the units sell, not as a phantom that
		// vanishes into COGS. lotCost stays the list cost when there are no free
		// units, and zero (a free/unpriced line) still lets the batch inherit the
		// product's own cost.
		costBasis := ln.Subtotal
		if billScaled {
			costBasis = ln.Subtotal.Mul(owed).DivRound(netSum, 4)
		}
		if _, err := stk.InsertBatch(ctx, stock.NewBatch{
			ProductID: ln.ProductID, PurchaseItemID: &itemID, ExpiryDate: ln.ExpiryDate,
			Quantity: gotQty, CostPrice: lotCost(costBasis, gotQty, ln.FreeQty, ln.CostPrice),
			SellingPrice: ln.SellingPrice, Source: "purchase",
		}); err != nil {
			return apperr.Internal("failed to create stock batch", err)
		}
		if ln.ExpiryDate != nil {
			if err := repo.MarkHasExpiry(ctx, ln.ProductID); err != nil {
				return apperr.Internal("failed to flag product expiry", err)
			}
		}
		ref := "purchase"
		pid := purchaseID
		if err := stk.InsertMovement(ctx, stock.MovementInput{
			ProductID: ln.ProductID, Type: stock.MovePurchase, Quantity: gotQty,
			ReferenceID: &pid, ReferenceType: &ref, UserID: userID,
		}); err != nil {
			return apperr.Internal("failed to record stock movement", err)
		}
		// Master cost tracks the list price paid per unit, NOT the blended lot
		// cost — it is the replacement cost the info popup and reorder use. Zero
		// (a free line) leaves it untouched via the guard in RefreshProductPricing.
		if err := repo.RefreshProductPricing(ctx, ln.ProductID, ln.CostPrice, ln.SellingPrice); err != nil {
			return apperr.Internal("failed to refresh product pricing", err)
		}
		// Backfill the default supplier from whoever actually delivered this, so
		// the reorder / low-stock-by-supplier report has something to filter on
		// without the owner tagging every product by hand. No-op if it already
		// has one.
		if err := repo.SetPreferredSupplierIfEmpty(ctx, ln.ProductID, supplierID); err != nil {
			return apperr.Internal("failed to set preferred supplier", err)
		}
	}
	if owed.IsPositive() {
		if err := sup.AddBalance(ctx, supplierID, owed); err != nil {
			return apperr.Internal("failed to update supplier balance", err)
		}
	}
	return nil
}

// ReceiveInput is the payload the receive screen posts for a draft.
type ReceiveInput struct {
	InvoiceNo    *string     `json:"invoice_no"`
	Discount     string      `json:"discount"`
	DiscountType string      `json:"discount_type" validate:"omitempty,oneof=fixed percent"`
	DueDate      string      `json:"due_date"`
	Notes        *string     `json:"notes"`
	Items        []ItemInput `json:"items" validate:"required,min=1,dive"`
	// KeepRemainder, when true, spins the still-unreceived quantities (ordered −
	// received, where positive) into a new draft PO so the rest stays on order.
	KeepRemainder bool `json:"keep_remainder"`
}

// parseReceiveLines parses the received lines, carrying each line's ordered_qty.
func parseReceiveLines(items []ItemInput) ([]PurchaseItem, decimal.Decimal, error) {
	lines, subtotal, err := parseLines(items)
	if err != nil {
		return nil, decimal.Zero, err
	}
	for i := range lines {
		if items[i].OrderedQty == "" {
			continue
		}
		oq, err := money.Parse(items[i].OrderedQty)
		if err == nil && !oq.IsNegative() {
			lines[i].OrderedQty = &oq
		}
	}
	return lines, subtotal, nil
}

// Receive turns a draft into a received purchase: it rewrites the lines with the
// actually-received quantities (overstock allowed), records invoice/paid/due, and
// posts all the inventory + payable effects. Only drafts can be received.
// ReceiveTx takes in the goods against a draft order, inside the caller's
// transaction. Like CreateTx it records no payment: the web layer composes
// paying on top so a real payment row, drawer movement and receipt all exist.
func ReceiveTx(ctx context.Context, tx *sqlx.Tx, id int64, in ReceiveInput, userID int64) (*Detail, error) {
	billValue, err := money.Parse(in.Discount)
	if err != nil || billValue.IsNegative() {
		return nil, apperr.Validation("discount must be a non-negative amount")
	}
	billType := normDiscountType(in.DiscountType)
	dueDate, err := parseDate(in.DueDate)
	if err != nil {
		return nil, err
	}
	var detail *Detail
	err = func(tx *sqlx.Tx) error {
		repo := NewRepository(tx)
		stk := stock.NewRepository(tx)
		sup := suppliers.NewRepository(tx)
		cur, err := repo.FindByID(ctx, id)
		if err != nil {
			return notFoundOr(err)
		}
		if cur.Status != "draft" {
			return apperr.Validation("this purchase has already been received")
		}
		lines, subtotal, err := parseReceiveLines(in.Items)
		if err != nil {
			return err
		}
		discount := resolveBillDiscount(billType, billValue, subtotal)
		total := subtotal.Sub(discount)
		notes := in.Notes
		if notes == nil {
			notes = cur.Notes
		}
		// What was ordered, read before the planned lines are dropped — the only
		// record of an item that never arrived.
		ordered, err := repo.Items(ctx, id)
		if err != nil {
			return apperr.Internal("failed to load the ordered lines", err)
		}
		// Drop the planned lines, then re-insert as received lines (which posts the
		// stock/batch/movement/pricing effects via applyReceivedLines).
		if err := repo.DeleteItems(ctx, id); err != nil {
			return apperr.Internal("failed to clear draft lines", err)
		}
		if err := repo.UpdateHeader(ctx, id, purchaseRow{
			InvoiceNo: in.InvoiceNo, Status: receivedStatus(decimal.Zero, total), Subtotal: subtotal,
			Discount: discount, DiscountType: billType, DiscountValue: billValue,
			Total: total, Paid: decimal.Zero, DueDate: dueDate,
			ExpectedDate: cur.ExpectedDate, ReceivedBy: userID, Notes: notes,
		}); err != nil {
			return apperr.Internal("failed to update purchase", err)
		}
		if err := applyReceivedLines(ctx, repo, stk, sup, id, cur.SupplierID, lines, total, userID); err != nil {
			return err
		}
		if err := applySettlement(ctx, tx, cur.SupplierID, id); err != nil {
			return err
		}
		// Partial receipt: carry the still-unreceived quantities into a new draft PO.
		//
		// Worked out from the draft's own lines, not from what turned up: an item
		// ordered but delivered in no quantity at all is left off the receive
		// entirely, and used to be forgotten with it.
		if in.KeepRemainder {
			rem := remainderLines(ordered, lines)
			if len(rem) > 0 {
				if _, err := createDraftTx(ctx, tx, CreateInput{
					SupplierID: cur.SupplierID, Discount: "0",
					Notes: cur.Notes, Items: rem,
				}, userID); err != nil {
					return err
				}
			}
		}
		d, err := loadDetailTx(ctx, repo, id)
		if err != nil {
			return err
		}
		detail = d
		return nil
	}(tx)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

// remainderLines works out what is still owed after a delivery: for each item on
// the original order, how much of it did not turn up.
//
// It reads from the order rather than from the delivery on purpose. A line that
// arrived in no quantity at all is simply absent from the receive — the form
// drops zero-quantity rows — so deriving the remainder from what arrived loses
// it silently, and the shop goes on believing it is still coming.
//
// Quantities are summed per product, so an order listing the same item twice
// behaves. Anything delivered that was never ordered is ignored: an unexpected
// extra is not something still owed.
func remainderLines(ordered, received []PurchaseItem) []ItemInput {
	got := make(map[int64]decimal.Decimal, len(received))
	for _, ln := range received {
		got[ln.ProductID] = got[ln.ProductID].Add(ln.Quantity)
	}
	want := make(map[int64]decimal.Decimal, len(ordered))
	order := make([]int64, 0, len(ordered))
	first := make(map[int64]PurchaseItem, len(ordered))
	for _, ln := range ordered {
		if _, seen := want[ln.ProductID]; !seen {
			order = append(order, ln.ProductID)
			first[ln.ProductID] = ln
		}
		want[ln.ProductID] = want[ln.ProductID].Add(ln.Quantity)
	}

	out := make([]ItemInput, 0, len(order))
	for _, pid := range order {
		short := want[pid].Sub(got[pid])
		if !short.IsPositive() {
			continue
		}
		src := first[pid]
		out = append(out, ItemInput{
			ProductID:    pid,
			Quantity:     short.String(),
			CostPrice:    src.CostPrice.String(),
			SellingPrice: src.SellingPrice.String(),
		})
	}
	return out
}
