package purchases

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"karots-suite/internal/apperr"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// SettleCredit spends whatever a supplier already holds for us — advances paid
// before the goods arrived, value from returned goods — against a freshly booked
// invoice, inside the same transaction.
//
// It is a hook rather than a direct call because settling lives in supplierpay,
// which imports this package; wiring it in main.go (as cashregister does for its
// locker leg) keeps the dependency one-way. nil = off, which is exactly the old
// behaviour.
//
// It hangs off CreateTx/ReceiveTx rather than off each caller so no future entry
// point can book a payable and quietly forget to spend the credit — that gap is
// how an invoice ends up paid twice.
type SettleCredit func(ctx context.Context, tx *sqlx.Tx, supplierID, purchaseID int64) error

var settleCredit SettleCredit

// WithSettleCredit wires the settlement hook. Called once at startup.
func WithSettleCredit(fn SettleCredit) { settleCredit = fn }

// applySettlement runs the settlement hook when one is wired.
func applySettlement(ctx context.Context, tx *sqlx.Tx, supplierID, purchaseID int64) error {
	if settleCredit == nil {
		return nil
	}
	return settleCredit(ctx, tx, supplierID, purchaseID)
}

type Service struct {
	db   *sqlx.DB
	repo *Repository
}

func NewService(db *sqlx.DB) *Service { return &Service{db: db, repo: NewRepository(db)} }

type ItemInput struct {
	ProductID int64  `json:"product_id"    validate:"required,gt=0"`
	Quantity  string `json:"quantity"      validate:"required"`
	// FreeQty is bonus units the supplier added at no charge. Optional; blank or
	// "0" means none. It adds stock but nothing to the payable — see parseLines.
	FreeQty      string `json:"free_qty"`
	CostPrice    string `json:"cost_price"    validate:"required"`
	SellingPrice string `json:"selling_price"`
	// Per-line discount the supplier knocked off this item. Value is the entered
	// number; Type is "fixed" (Rs per unit) or "percent" (off the line).
	Discount     string `json:"discount"`
	DiscountType string `json:"discount_type" validate:"omitempty,oneof=fixed percent"`
	ExpiryDate   string `json:"expiry_date"`
	// OrderedQty carries the originally-ordered amount through a receive so the
	// draft's planned qty is preserved alongside the actually-received quantity.
	OrderedQty string `json:"ordered_qty"`
}

type CreateInput struct {
	SupplierID   int64       `json:"supplier_id" validate:"required,gt=0"`
	InvoiceNo    *string     `json:"invoice_no"`
	Discount     string      `json:"discount"`
	DiscountType string      `json:"discount_type" validate:"omitempty,oneof=fixed percent"`
	DueDate      string      `json:"due_date"`
	ExpectedDate string      `json:"expected_date"`
	Notes        *string     `json:"notes"`
	Items        []ItemInput `json:"items" validate:"required,min=1,dive"`
}

// parseDate parses an optional YYYY-MM-DD date (nil when blank).
func parseDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, apperr.Validation("date must be YYYY-MM-DD")
	}
	return &d, nil
}

// normDiscountType defaults a blank/unknown discount type to "fixed".
func normDiscountType(t string) string {
	if t == "percent" {
		return "percent"
	}
	return "fixed"
}

// clampDiscount keeps a discount within [0, base] so it can never go negative or
// exceed what it is discounting.
func clampDiscount(amt, base decimal.Decimal) decimal.Decimal {
	if amt.IsNegative() {
		return decimal.Zero
	}
	if amt.GreaterThan(base) {
		return base
	}
	return amt
}

// resolveItemDiscount turns a per-line discount into an amount off the line.
// Fixed is per unit (× qty); percent is off the line gross. Mirrors the sell side.
func resolveItemDiscount(dtype string, value, lineGross, qty decimal.Decimal) decimal.Decimal {
	var amt decimal.Decimal
	if dtype == "percent" {
		amt = lineGross.Mul(value).Div(decimal.NewFromInt(100))
	} else {
		amt = value.Mul(qty)
	}
	return clampDiscount(amt.Round(2), lineGross)
}

// resolveBillDiscount turns the whole-bill discount into an amount off the given
// base (the net subtotal after item discounts). Fixed is a flat amount; percent
// is off the base. Mirrors the sell side.
func resolveBillDiscount(dtype string, value, base decimal.Decimal) decimal.Decimal {
	var amt decimal.Decimal
	if dtype == "percent" {
		amt = base.Mul(value).Div(decimal.NewFromInt(100))
	} else {
		amt = value
	}
	return clampDiscount(amt.Round(2), base)
}

func mapErr(err error) error {
	return apperr.Internal("failed to save purchase", err)
}

func notFoundOr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return apperr.NotFound("purchase")
	}
	return apperr.Internal("failed to load purchase", err)
}
