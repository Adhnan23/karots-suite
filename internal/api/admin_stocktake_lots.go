package api

import (
	"context"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/money"
)

// Stock-take batch by batch (M7.6b): a product with 2+ live batches is counted
// one batch at a time — on screen and in the count sheet — so "10 old, 2 new"
// has somewhere to go.

// countLot sets one live batch to what was counted; the product total moves by
// the difference, through Adjust{BatchID} like Batches → Set count.
func (h *handlers) countLot(ctx context.Context, productID, batchID int64, counted, note string, userID int64) (bool, error) {
	target, err := money.Parse(strings.TrimSpace(counted))
	if err != nil || target.IsNegative() {
		return false, apperr.Validation("the count must be a non-negative number")
	}
	lot, err := h.liveLot(ctx, productID, batchID)
	if err != nil {
		return false, err
	}
	if lot.QtyRemaining.Equal(target) {
		return false, nil
	}
	cur, err := h.t.stock.Quantity(ctx, productID)
	if err != nil {
		return false, err
	}
	total := cur.Sub(lot.QtyRemaining).Add(target)
	return true, h.t.stock.Adjust(ctx, stock.AdjustInput{ProductID: productID, NewQuantity: total.String(), Note: note, BatchID: batchID}, userID)
}

func (h *handlers) liveLot(ctx context.Context, productID, batchID int64) (*stock.Batch, error) {
	lots, err := h.t.stock.Batches(ctx, productID)
	if err != nil {
		return nil, err
	}
	for i := range lots {
		if lots[i].ID == batchID {
			return &lots[i], nil
		}
	}
	return nil, apperr.Validation("batch " + strconv.FormatInt(batchID, 10) + " is not one of its batches with stock")
}

// applyLotCounts applies a line counted batch by batch, plus any batches found
// that weren't on the list. The line's product-level count is ignored.
func (h *handlers) applyLotCounts(ctx context.Context, l CountLine, note string, userID int64) (bool, error) {
	changed := false
	for _, b := range l.Batches {
		if strings.TrimSpace(b.Counted) == "" {
			continue
		}
		c, err := h.countLot(ctx, l.ProductID, b.BatchID, b.Counted, note, userID)
		if err != nil {
			return false, err
		}
		changed = changed || c
	}
	for _, nb := range l.NewBatches {
		if strings.TrimSpace(nb.Qty) == "" {
			continue
		}
		if _, err := h.t.stock.AddStock(ctx, stock.AddStockInput{ProductID: l.ProductID, Qty: nb.Qty, Cost: nb.Cost, Price: nb.SellingPrice,
			Expiry: nb.Expiry, Note: note}, userID); err != nil {
			return false, err
		}
		changed = true
	}
	return changed, nil
}

// importLotRow applies one count-sheet row that names a batch: its count, and
// its cost / own price when given (blank leaves them).
func (h *handlers) importLotRow(ctx context.Context, p *products.Product, batch, counted, cost, selling string, userID int64) (bool, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(batch), 10, 64)
	if err != nil {
		return false, apperr.Validation("batch must be the batch number from the downloaded sheet")
	}
	lot, err := h.liveLot(ctx, p.ID, id)
	if err != nil {
		return false, err
	}
	changed := false
	if s := strings.TrimSpace(cost); s != "" {
		v, err := money.Parse(s)
		if err != nil || v.IsNegative() {
			return false, apperr.Validation("cost must be a non-negative amount")
		}
		if !v.Equal(lot.CostPrice) {
			if err := h.t.stock.SetBatchCost(ctx, id, v); err != nil {
				return false, err
			}
			changed = true
		}
	}
	if s := strings.TrimSpace(selling); s != "" {
		v, err := money.Parse(s)
		if err != nil || v.IsNegative() {
			return false, apperr.Validation("selling price must be a non-negative amount")
		}
		if !v.Equal(lot.SellingPrice) {
			if err := h.t.stock.SetBatchPrice(ctx, id, v); err != nil {
				return false, err
			}
			changed = true
		}
	}
	c, err := h.countLot(ctx, p.ID, id, counted, "stock-take sheet", userID)
	return changed || c, err
}

// lotSheetRows are a multi-batch product's count-sheet rows, one per batch:
// its qty, cost and own price (blank = follows the product).
func lotSheetRows(p products.Product, lots []stock.Batch) [][]string {
	out := make([][]string, 0, len(lots))
	for _, b := range lots {
		r := lotRow(b)
		out = append(out, []string{strOr(p.Barcode), p.Name, b.QtyRemaining.String(), "", r.CostPrice, r.SellingPrice, dec2(p.WholesalePrice),
			strconv.FormatInt(b.ID, 10), r.ExpiryDate})
	}
	return out
}
