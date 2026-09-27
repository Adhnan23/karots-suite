// Package sales records POS transactions. A sale is written atomically: stock
// is decremented under a guard (no oversell), movements are audited, and any
// credit portion updates the customer balance — all in one transaction.
package sales

import (
	"context"
	"fmt"
	"time"

	"karots-suite/internal/db"
	"karots-suite/internal/features/warranty"

	"github.com/shopspring/decimal"
)

type Sale struct {
	ID            int64           `db:"id"           json:"id"`
	ReceiptNo     string          `db:"receipt_no"   json:"receipt_no"`
	CustomerID    *int64          `db:"customer_id"  json:"customer_id,omitempty"`
	SaleType      string          `db:"sale_type"    json:"sale_type"`
	Subtotal      decimal.Decimal `db:"subtotal"       json:"subtotal"`
	Discount      decimal.Decimal `db:"discount"       json:"discount"`
	DiscountType  string          `db:"discount_type"  json:"discount_type"`  // bill discount: fixed|percent
	DiscountValue decimal.Decimal `db:"discount_value" json:"discount_value"` // entered value
	Tax           decimal.Decimal `db:"tax"          json:"tax"`
	Total         decimal.Decimal `db:"total"        json:"total"`
	PaidAmount    decimal.Decimal `db:"paid_amount"  json:"paid_amount"`
	ChangeGiven   decimal.Decimal `db:"change_given" json:"change_given"`
	// RoundingToAccount is the surplus (cash the customer left behind) that was
	// booked to an account customer's balance as an advance instead of kept as a
	// drawer rounding gain. The receipt derives kept = paid - change - total and
	// splits it: this column is the on-account part, the remainder is rounding.
	RoundingToAccount decimal.Decimal `db:"rounding_to_account" json:"rounding_to_account"`
	// CustomerBalanceAfter is the customer's account right after a credit sale
	// (negative = advance left). NULL on older sales.
	CustomerBalanceAfter *decimal.Decimal `db:"customer_balance_after" json:"customer_balance_after,omitempty"`
	Status               string           `db:"status"       json:"status"`
	CashierID            int64            `db:"cashier_id"   json:"cashier_id"`
	Notes                *string          `db:"notes"        json:"notes,omitempty"`
	CreatedAt            time.Time        `db:"created_at"   json:"created_at"`
	// joined
	CashierName  string  `db:"cashier_name"  json:"cashier_name"`
	CustomerName *string `db:"customer_name" json:"customer_name,omitempty"`
	// Profit fields are populated only by the reports List query (a lateral join
	// over sale_items); other queries leave them zero. COGS/Profit mirror the
	// Finance P&L definition (net of pass-through + returns).
	COGS   decimal.Decimal `db:"cogs"   json:"cogs"`
	Profit decimal.Decimal `db:"profit" json:"profit"`
	Margin decimal.Decimal `db:"margin" json:"margin"`
}

type SaleItem struct {
	ID            int64           `db:"id"           json:"id"`
	SaleID        int64           `db:"sale_id"      json:"sale_id"`
	ProductID     int64           `db:"product_id"   json:"product_id"`
	Quantity      decimal.Decimal `db:"quantity"     json:"quantity"`
	UnitPrice     decimal.Decimal `db:"unit_price"   json:"unit_price"`
	CostPrice     decimal.Decimal `db:"cost_price"   json:"cost_price"`
	Discount      decimal.Decimal `db:"discount"       json:"discount"`
	DiscountType  string          `db:"discount_type"  json:"discount_type"` // fixed|percent
	DiscountValue decimal.Decimal `db:"discount_value" json:"discount_value"`
	Subtotal      decimal.Decimal `db:"subtotal"     json:"subtotal"`
	ReturnedQty   decimal.Decimal `db:"returned_qty" json:"returned_qty"`
	Description   *string         `db:"description"  json:"description,omitempty"`
	// BatchID records the lot this line was rung from when the cashier picked one
	// at the "which price?" prompt. NULL for the normal FEFO path (which may span
	// several lots), so it answers "why was this price used" without pretending to
	// be a complete lot trace.
	BatchID *int64 `db:"batch_id" json:"batch_id,omitempty"`
	// joined
	ProductName    string `db:"product_name"    json:"product_name"`
	UnitAbbr       string `db:"unit_abbr"       json:"unit_abbr"`
	IsService      bool   `db:"is_service"      json:"is_service"` // service line (e.g. recharge) — non-returnable, no stock
	WarrantyMonths int    `db:"warranty_months" json:"warranty_months"`
}

// ReturnableQty is how much of this line can still be sent back.
func (i SaleItem) ReturnableQty() decimal.Decimal { return i.Quantity.Sub(i.ReturnedQty) }

// Gross is the line amount before its discount (qty × unit price). Subtotal
// stores the NET (after discount); adding the discount back recovers the gross,
// which the receipt shows struck-through beside the net.
func (i SaleItem) Gross() decimal.Decimal { return i.Subtotal.Add(i.Discount) }

// NetUnitPrice is the per-unit price the customer actually paid after the line
// discount (net line ÷ qty). The receipt shows it beside the struck-through
// original unit price so the discount is visible on the item line itself.
func (i SaleItem) NetUnitPrice() decimal.Decimal {
	if i.Quantity.IsZero() {
		return i.UnitPrice
	}
	return i.Subtotal.Div(i.Quantity)
}

// WarrantyUntil is the cover-expiry date for this line given the sale time, or
// the zero time when the product carries no warranty. Uses the canonical
// date-only math in the warranty package so it matches recorded serial units.
func (i SaleItem) WarrantyUntil(soldAt time.Time) time.Time {
	if i.WarrantyMonths <= 0 {
		return time.Time{}
	}
	return warranty.Until(soldAt, i.WarrantyMonths)
}

type Payment struct {
	ID        int64           `db:"id"        json:"id"`
	SaleID    int64           `db:"sale_id"   json:"sale_id"`
	Method    string          `db:"method"    json:"method"`
	Amount    decimal.Decimal `db:"amount"    json:"amount"`
	Reference *string         `db:"reference" json:"reference,omitempty"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
}

// Detail bundles a sale with its lines and payments (for receipts and views).
type Detail struct {
	Sale     Sale       `json:"sale"`
	Items    []SaleItem `json:"items"`
	Payments []Payment  `json:"payments"`
}

// --- repository ---

type Repository struct{ q db.Queryer }

func NewRepository(q db.Queryer) *Repository { return &Repository{q: q} }

func (r *Repository) NextReceiptNo(ctx context.Context) (string, error) {
	var n int64
	if err := r.q.GetContext(ctx, &n, `SELECT nextval('sales_receipt_seq')`); err != nil {
		return "", err
	}
	return fmt.Sprintf("S-%05d", n), nil
}

type saleRow struct {
	ReceiptNo            string
	CustomerID           *int64
	SaleType             string
	Subtotal             decimal.Decimal
	Discount             decimal.Decimal
	DiscountType         string
	DiscountValue        decimal.Decimal
	Tax                  decimal.Decimal
	Total                decimal.Decimal
	PaidAmount           decimal.Decimal
	ChangeGiven          decimal.Decimal
	RoundingToAccount    decimal.Decimal
	CustomerBalanceAfter *decimal.Decimal
	Status               string
	CashierID            int64
	Notes                *string
}

func (r *Repository) InsertSale(ctx context.Context, s saleRow) (int64, error) {
	var id int64
	err := r.q.GetContext(ctx, &id, `
		INSERT INTO sales
			(receipt_no, customer_id, sale_type, subtotal, discount, discount_type, discount_value,
			 tax, total, paid_amount, change_given, rounding_to_account, status, cashier_id, notes,
			 customer_balance_after)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING id`,
		s.ReceiptNo, s.CustomerID, s.SaleType, s.Subtotal, s.Discount, s.DiscountType, s.DiscountValue,
		s.Tax, s.Total, s.PaidAmount, s.ChangeGiven, s.RoundingToAccount, s.Status, s.CashierID, s.Notes,
		s.CustomerBalanceAfter)
	return id, err
}

func (r *Repository) InsertItem(ctx context.Context, saleID int64, it SaleItem) error {
	_, err := r.q.ExecContext(ctx, `
		INSERT INTO sale_items (sale_id, product_id, quantity, unit_price, cost_price, discount, discount_type, discount_value, subtotal, description, batch_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		saleID, it.ProductID, it.Quantity, it.UnitPrice, it.CostPrice, it.Discount, it.DiscountType, it.DiscountValue, it.Subtotal, it.Description, it.BatchID)
	return err
}

func (r *Repository) InsertPayment(ctx context.Context, saleID int64, method string, amount decimal.Decimal, ref *string) error {
	_, err := r.q.ExecContext(ctx, `
		INSERT INTO payments (sale_id, method, amount, reference)
		VALUES ($1,$2,$3,$4)`, saleID, method, amount, ref)
	return err
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := r.q.ExecContext(ctx, `UPDATE sales SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*Sale, error) {
	var s Sale
	err := r.q.GetContext(ctx, &s, `
		SELECT s.*, u.name AS cashier_name, c.name AS customer_name
		FROM sales s
		JOIN users u ON u.id = s.cashier_id
		LEFT JOIN customers c ON c.id = s.customer_id
		WHERE s.id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) Items(ctx context.Context, saleID int64) ([]SaleItem, error) {
	var rows []SaleItem
	err := r.q.SelectContext(ctx, &rows, `
		SELECT si.*, p.name AS product_name, u.abbreviation AS unit_abbr, p.is_service AS is_service,
		       p.warranty_months AS warranty_months
		FROM sale_items si
		JOIN products p ON p.id = si.product_id
		JOIN units u    ON u.id = p.unit_id
		WHERE si.sale_id = $1 ORDER BY si.id`, saleID)
	return rows, err
}

func (r *Repository) Payments(ctx context.Context, saleID int64) ([]Payment, error) {
	var rows []Payment
	err := r.q.SelectContext(ctx, &rows,
		`SELECT * FROM payments WHERE sale_id = $1 ORDER BY id`, saleID)
	return rows, err
}
