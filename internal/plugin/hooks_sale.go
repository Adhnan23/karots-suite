package plugin

import (
	"context"
	"encoding/json"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// SaleLineHook records what a plugin's cart lines mean once the sale is saved —
// inside the sale's transaction, so it commits with the sale. The till tags a
// line with plugin data under the plugin's Key (CartLine.plugin[key]); Record
// gets only the lines carrying its key, with that data. Record must not refuse
// (an error rolls the whole sale back — a sale is never stopped); keep errors
// for real failures.
type SaleLineHook struct {
	Key    string
	Record func(ctx context.Context, tx *sqlx.Tx, ev SaleLines) error
}

// SaleLines is one sale's lines tagged by one plugin.
type SaleLines struct {
	SaleID     int64
	CustomerID *int64
	ActorID    int64
	Lines      []SaleLine
}

// SaleLine is a tagged line: its sale_items row, product, qty, net amount and
// the plugin's own data.
type SaleLine struct {
	ItemID    int64
	ProductID int64
	Quantity  decimal.Decimal
	Amount    decimal.Decimal // the line's net after its discount (a payout is negative)
	Data      json.RawMessage
}

var saleLineHooks hook[SaleLineHook]

func (r *Registry) AddSaleLineHook(h SaleLineHook) { saleLineHooks.add(h) }

func SaleLineHooks() []SaleLineHook { return saleLineHooks.all() }

// ReturnLineHook follows sale lines coming back (counter Returns), inside the
// return's transaction. Every hook gets every returned line — a plugin picks
// the ones it owns by sale item (its own rows) and fixes only those; Data is
// what its Returns field sent for that line under Key (nil when none). Like
// SaleLineHook it must not refuse for business reasons.
type ReturnLineHook struct {
	Key    string
	Record func(ctx context.Context, tx *sqlx.Tx, ev ReturnLines) error
}

// ReturnLines is one return's lines.
type ReturnLines struct {
	SaleID   int64
	ReturnID int64
	ActorID  int64
	Lines    []ReturnLine
}

// ReturnLine is one returned line: what came back and what it refunds.
type ReturnLine struct {
	SaleItemID int64
	ProductID  int64
	Quantity   decimal.Decimal
	Refund     decimal.Decimal
	Data       json.RawMessage
}

var returnLineHooks hook[ReturnLineHook]

func (r *Registry) AddReturnLineHook(h ReturnLineHook) { returnLineHooks.add(h) }

func ReturnLineHooks() []ReturnLineHook { return returnLineHooks.all() }
