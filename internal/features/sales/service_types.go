package sales

import (
	"github.com/shopspring/decimal"
)

type ItemInput struct {
	ProductID    int64  `json:"product_id"    validate:"required,gt=0"`
	Quantity     string `json:"quantity"      validate:"required"`
	Discount     string `json:"discount"`
	DiscountType string `json:"discount_type" validate:"omitempty,oneof=fixed percent"`
	// PriceOverride sets the unit price for a service line (is_service products,
	// e.g. a top-up amount). Ignored for normal stocked products, whose
	// price is always recomputed server-side from the catalogue.
	PriceOverride string `json:"price_override"`
	// BatchID names the lot the cashier picked at the "which price?" prompt when a
	// product's live batches disagree on price (an old bottle stickered at the old
	// price alongside newly-received stock). The client sends only the id — the
	// price is read from the batch here, never accepted from the client — and the
	// sale then depletes THAT lot instead of taking FEFO. Zero means the normal
	// path. Ignored for is_service products, which hold no stock.
	BatchID int64 `json:"batch_id"`
	// AllowOversell lets this line sell more than the count shows: the customer is
	// holding stock that was never counted. The server corrects the count up
	// (FoundAtTill) before depleting, so on-hand lands at 0, never negative.
	// Confirm-gated at the till; ignored for is_service products.
	AllowOversell bool `json:"allow_oversell"`
	// Serials carries one unique serial number per unit for serial-tracked
	// products (length must equal the quantity); ignored for other products.
	Serials []string `json:"serials"`
	// Description is an optional per-line label shown on the receipt/history
	// instead of the product name (e.g. a plugin service line "A4 colour x20").
	Description string `json:"description"`
	// Components lists stock to deplete for a service line (e.g. a document job
	// consuming paper). Honoured only for is_service products; the line's
	// cost_price becomes the summed FEFO cost of the consumed components.
	Components []ServiceComponent `json:"components"`
	// Group marks lines the till added together (a plugin's group): receipts
	// print them as one block. nil = a plain line.
	Group *LineGroup `json:"group"`
}

// LineGroup: No numbers the groups within one sale, Sets is how many sets.
type LineGroup struct {
	No    int             `json:"no"`
	Label string          `json:"label"`
	Sets  decimal.Decimal `json:"sets"`
}

// ServiceComponent is one consumable a service line draws down from stock.
type ServiceComponent struct {
	ProductID int64  `json:"product_id"`
	Quantity  string `json:"quantity"`
}

// ServiceComponentParsed is a validated component (qty parsed), used to record the
// stock movement for a service line's consumption after the sale id is known.
type ServiceComponentParsed struct {
	ProductID int64
	Qty       decimal.Decimal
}

// serialBatch holds the captured serials for one serial-tracked line, recorded
// as warranty units once the sale id is known.
type serialBatch struct {
	productID int64
	months    int
	serials   []string
}

type PaymentInput struct {
	Method    string  `json:"method"    validate:"required"` // core or a plugin method; Create refuses any other
	Amount    string  `json:"amount"    validate:"required"`
	Reference *string `json:"reference"`
	// AccountID is the money account a non-cash tender (card/online) lands in — a
	// card-kind locker for card, a bank-kind locker for online. Required for a
	// card/online line with an amount (Create refuses 0).
	AccountID int64 `json:"account_id"`
	// Meta is opaque per-line data for a plugin tender method (core never reads
	// it). Handed to the plugin hook.
	Meta map[string]string `json:"meta"`
}

type CreateInput struct {
	CustomerID   *int64         `json:"customer_id"`
	SaleType     string         `json:"sale_type"     validate:"required,oneof=retail wholesale"`
	Discount     string         `json:"discount"`
	DiscountType string         `json:"discount_type" validate:"omitempty,oneof=fixed percent"`
	Notes        *string        `json:"notes"`
	Items        []ItemInput    `json:"items"    validate:"required,min=1,dive"`
	Payments     []PaymentInput `json:"payments" validate:"dive"`
	// AllowOverLimit permits an account line past the customer's credit limit.
	// The web layer sets this only for a user with can_manage_credit — the
	// service trusts it as already-authorised.
	AllowOverLimit bool `json:"allow_over_limit"`
	// ChangeGiven, when set, is the ACTUAL cash change handed back — which the
	// cashier may record as less than the computed change when the customer
	// leaves the difference behind (a customer who won't wait for 2 in coins, or
	// asks for a round 500 back on a 490 bill paid with 1000). Storing the real
	// change keeps change_given == cash actually out, so the drawer reconciles
	// and the session doesn't drift. Empty string = give the full computed change
	// (every existing sale). Clamped server-side to [0, computed change].
	ChangeGiven string `json:"change_given"`
	// OnAccountRounding books the kept surplus (computed change minus ChangeGiven)
	// to the selected customer's balance as an advance instead of leaving it as a
	// drawer rounding gain. Requires a customer and a positive kept amount.
	OnAccountRounding bool `json:"on_account_rounding"`
	// DeferDrawerKick tells the service NOT to fire the standalone drawer pulse
	// for a cash sale, because the caller (the till, in auto-print mode) will
	// merge the pulse into the receipt print as a SINGLE printer job. On a USB
	// thermal printer two back-to-back jobs (kick, then receipt) each pay the
	// device open/close + inter-job wait — that gap is what made the paper come
	// out seconds after "Complete Sale". One combined job removes it.
	DeferDrawerKick bool `json:"defer_drawer_kick"`
}
