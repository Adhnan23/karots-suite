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

// SaleLine is a tagged line: its product, qty and the plugin's own data.
type SaleLine struct {
	ProductID int64
	Quantity  decimal.Decimal
	Data      json.RawMessage
}

var saleLineHooks hook[SaleLineHook]

func (r *Registry) AddSaleLineHook(h SaleLineHook) { saleLineHooks.add(current, h) }

func SaleLineHooks() []SaleLineHook { return saleLineHooks.all() }
