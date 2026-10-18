// Package sales records POS transactions. A sale is written atomically: stock
// is decremented under a guard (no oversell), movements are audited, and any
// credit portion updates the customer balance — all in one transaction.
package sales

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// PaidFactorSQL is what each Rs of line net was actually paid: the sale total
// over the sum of its line nets (sale_items.subtotal already has the item
// discount off; the bill discount and tax live only on the sale). One factor
// every reader multiplies by, so the refund, the sales list, the day-end
// summary and the Returns report all agree with the cash that went back.
// Correlated on the outer `s`.
// Payout lines (the only negative ones) are money handed over, not goods: they
// come off the total and stay out of the base.
const PaidFactorSQL = `COALESCE((s.total - ` + PayoutsSQL + `) / NULLIF((SELECT SUM(x.subtotal) FROM sale_items x WHERE x.sale_id = s.id AND x.subtotal > 0), 0), 1)`

// PayoutsSQL is the sale's payout lines (≤ 0) — money the shop handed the
// customer. Correlated on `s`.
const PayoutsSQL = `(SELECT COALESCE(SUM(x.subtotal), 0) FROM sale_items x WHERE x.sale_id = s.id AND x.subtotal < 0)`

func (r *Repository) PaidFactor(ctx context.Context, saleID int64) (decimal.Decimal, error) {
	var f decimal.Decimal
	err := r.q.GetContext(ctx, &f, `SELECT `+PaidFactorSQL+` FROM sales s WHERE s.id = $1`, saleID)
	return f, err
}

// CreditReduced is what earlier returns on a sale already took off the
// customer's account. sales.paid_amount never changes, so the debt a return can
// still clear is total − paid − this.
func (r *Repository) CreditReduced(ctx context.Context, saleID int64) (decimal.Decimal, error) {
	var v decimal.Decimal
	err := r.q.GetContext(ctx, &v, `SELECT COALESCE(SUM(credit_reduction), 0) FROM sale_returns WHERE sale_id = $1`, saleID)
	return v, err
}

// RefundedForItem is what earlier returns already refunded on one sale line.
func (r *Repository) RefundedForItem(ctx context.Context, saleItemID int64) (decimal.Decimal, error) {
	var v decimal.Decimal
	err := r.q.GetContext(ctx, &v, `SELECT COALESCE(SUM(refund_amount), 0) FROM sale_return_items WHERE sale_item_id = $1`, saleItemID)
	return v, err
}

// AddReturnedQty increments a line's returned quantity (guarded so it never
// exceeds the quantity sold). Returns false if the requested qty isn't available.
func (r *Repository) AddReturnedQty(ctx context.Context, itemID int64, qty decimal.Decimal) (bool, error) {
	res, err := r.q.ExecContext(ctx,
		`UPDATE sale_items SET returned_qty = returned_qty + $1
		 WHERE id = $2 AND returned_qty + $1 <= quantity`, qty, itemID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// FindItem loads one sale line (to value a partial return).
func (r *Repository) FindItem(ctx context.Context, saleID, itemID int64) (*SaleItem, error) {
	var it SaleItem
	err := r.q.GetContext(ctx, &it, `
		SELECT si.*, p.name AS product_name, u.abbreviation AS unit_abbr, p.is_service AS is_service
		FROM sale_items si
		JOIN products p ON p.id = si.product_id
		JOIN units u    ON u.id = p.unit_id
		WHERE si.id = $1 AND si.sale_id = $2`, itemID, saleID)
	if err != nil {
		return nil, err
	}
	return &it, nil
}

// OutstandingItems counts sale lines that still have un-returned quantity, to
// decide whether a sale becomes 'partially_returned' or fully 'returned'.
func (r *Repository) OutstandingItems(ctx context.Context, saleID int64) (int, error) {
	var n int
	err := r.q.GetContext(ctx, &n,
		`SELECT COUNT(*) FROM sale_items WHERE sale_id = $1 AND returned_qty < quantity AND subtotal >= 0`, saleID)
	return n, err
}

func (r *Repository) InsertSaleReturn(ctx context.Context, saleID, userID int64, refund, creditReduction decimal.Decimal, reason *string) (int64, error) {
	var id int64
	err := r.q.GetContext(ctx, &id, `
		INSERT INTO sale_returns (sale_id, refund_amount, credit_reduction, reason, created_by)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`, saleID, refund, creditReduction, reason, userID)
	return id, err
}

func (r *Repository) SetReturnTotals(ctx context.Context, returnID int64, refund, creditReduction decimal.Decimal) error {
	_, err := r.q.ExecContext(ctx,
		`UPDATE sale_returns SET refund_amount = $1, credit_reduction = $2 WHERE id = $3`,
		refund, creditReduction, returnID)
	return err
}

func (r *Repository) InsertSaleReturnItem(ctx context.Context, returnID, saleItemID, productID int64, qty, refund decimal.Decimal) error {
	_, err := r.q.ExecContext(ctx, `
		INSERT INTO sale_return_items (return_id, sale_item_id, product_id, quantity, refund_amount)
		VALUES ($1,$2,$3,$4,$5)`, returnID, saleItemID, productID, qty, refund)
	return err
}

// ReturnReceipt is the data for a printed refund slip: the most recent return on
// a sale, with its returned lines and refund/credit split.
type ReturnReceipt struct {
	ReturnID         int64            `db:"id"`
	ReceiptNo        string           `db:"receipt_no"`
	CreatedAt        time.Time        `db:"created_at"`
	Reason           *string          `db:"reason"`
	Refund           decimal.Decimal  `db:"refund_amount"`
	CreditReduction  decimal.Decimal  `db:"credit_reduction"`
	CustomerName     *string          `db:"customer_name"`
	RemainingBalance *decimal.Decimal `db:"remaining_balance"`
	Items            []ReturnReceiptItem
}

type ReturnReceiptItem struct {
	ProductName string          `db:"product_name"`
	UnitAbbr    string          `db:"unit_abbr"`
	Quantity    decimal.Decimal `db:"quantity"`
	Refund      decimal.Decimal `db:"refund_amount"`
}

// LatestReturn loads the most recent return on a sale plus its line items, for
// printing a refund slip right after a return is processed.
func (r *Repository) LatestReturn(ctx context.Context, saleID int64) (*ReturnReceipt, error) {
	var rr ReturnReceipt
	if err := r.q.GetContext(ctx, &rr, `
		SELECT sr.id, s.receipt_no, sr.created_at, sr.reason, sr.refund_amount, sr.credit_reduction,
		       c.name AS customer_name, c.outstanding_balance AS remaining_balance
		FROM sale_returns sr JOIN sales s ON s.id = sr.sale_id
		LEFT JOIN customers c ON c.id = s.customer_id
		WHERE sr.sale_id = $1 ORDER BY sr.id DESC LIMIT 1`, saleID); err != nil {
		return nil, err
	}
	if err := r.q.SelectContext(ctx, &rr.Items, `
		SELECT p.name AS product_name, u.abbreviation AS unit_abbr, sri.quantity, sri.refund_amount
		FROM sale_return_items sri
		JOIN products p ON p.id = sri.product_id
		JOIN units u ON u.id = p.unit_id
		WHERE sri.return_id = $1
		ORDER BY sri.id`, rr.ReturnID); err != nil {
		return nil, err
	}
	return &rr, nil
}
