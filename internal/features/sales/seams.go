package sales

// The seams the composition root injects: plugin line / return hooks, the
// drawer kick, non-cash and plugin tender money legs (the cheque one is in
// cheque.go). Sales never imports the packages behind them.

import (
	"context"
	"encoding/json"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// TaggedLine is a sale line that carries plugin data (ItemInput.Plugin).
type TaggedLine struct {
	ItemID    int64 // sale_items.id
	ProductID int64
	Quantity  decimal.Decimal
	Amount    decimal.Decimal // the line's net (after its discount); a payout is negative
	Plugin    map[string]json.RawMessage
}

// ReturnedLine is one line coming back in a return, with the plugin data the
// till sent for it (ReturnLineInput.Plugin).
type ReturnedLine struct {
	SaleItemID int64
	ProductID  int64
	Quantity   decimal.Decimal
	Refund     decimal.Decimal // what these units are refunded (bill discount shared out)
	Plugin     map[string]json.RawMessage
}

// ReturnedSale is one return's lines, handed to the return hook.
type ReturnedSale struct {
	SaleID   int64
	ReturnID int64
	ActorID  int64
	Lines    []ReturnedLine
}

// ReturnHook lets plugins put their own rows right when sale lines come back (a
// float returned, …), inside the return's tx. Injected from the composition root.
type ReturnHook func(ctx context.Context, tx *sqlx.Tx, ev ReturnedSale) error

// WithReturnHook injects the return seam. Returns the service.
func (s *Service) WithReturnHook(fn ReturnHook) *Service {
	s.returnHook = fn
	return s
}

// TaggedSale is a completed sale's tagged lines, handed to the line hook.
type TaggedSale struct {
	SaleID     int64
	CustomerID *int64
	ActorID    int64
	Lines      []TaggedLine
}

// LineHook lets plugins record what their tagged lines mean (a repair job picked
// up, …) inside the sale's own tx — so the record can't miss the way a call
// after the sale can. Injected from the composition root, which dispatches each
// plugin's data to that plugin; core never knows which. An error rolls the sale
// back, so a hook records and never refuses (a sale is never stopped).
type LineHook func(ctx context.Context, tx *sqlx.Tx, ev TaggedSale) error

// WithLineHook injects the tagged-line seam. Returns the service.
func (s *Service) WithLineHook(fn LineHook) *Service {
	s.lineHook = fn
	return s
}

// WithDrawerKick injects a best-effort action fired AFTER a cash-tendered sale
// commits, to pop the physical cash drawer. nil = no drawer. Returns the service
// for chaining.
func (s *Service) WithDrawerKick(fn func(ctx context.Context)) *Service {
	s.drawerKick = fn
	return s
}

// NonCashPayment is one card/online tender line whose money must be booked into
// a chosen money account (a card- or bank-kind locker), inside the sale's tx.
type NonCashPayment struct {
	AccountID int64           // the picked locker (card for card, bank for online)
	Method    string          // "card" | "online"
	Amount    decimal.Decimal // positive
	SaleID    int64
	Party     string // customer name, or "" for a walk-in
	ActorID   int64
}

// NonCashLeg books the money side (+ CR- receipt) of a non-cash sale tender into
// the chosen account, on the sale's own tx. Injected by the web layer via
// WithNonCashLeg (cashflow.SaleNonCashLeg) so sales never imports cashflow — the
// same DI shape as cashregister.WithLockerLeg. nil = money routing off (the line
// is recorded as a payment only, today's behaviour).
type NonCashLeg func(ctx context.Context, tx *sqlx.Tx, ev NonCashPayment) error

// WithNonCashLeg injects the non-cash money hook. Returns the service for chaining.
func (s *Service) WithNonCashLeg(fn NonCashLeg) *Service {
	s.nonCashLeg = fn
	return s
}

// PluginTender is a tender line whose method is not one core handles (cash, card,
// online, credit) — i.e. a method a plugin registered. Meta carries that
// method's opaque per-line data; core never interprets it.
type PluginTender struct {
	Method  string
	Amount  decimal.Decimal
	SaleID  int64
	Meta    map[string]string
	ActorID int64
}

// PluginTenderLeg books the money side of a plugin tender on the sale's own tx,
// so the plugin's own ledger commits atomically with the sale instead of a
// best-effort call after it. Injected from the composition root, which dispatches
// to whatever plugin registered the method — so core stays free of any plugin.
type PluginTenderLeg func(ctx context.Context, tx *sqlx.Tx, ev PluginTender) error

// WithPluginTenders injects the plugin-tender seam: known reports whether a
// method is one a plugin registered, leg books it inside the sale's tx. Without
// it every non-core method is refused. Returns the service.
func (s *Service) WithPluginTenders(known func(method string) bool, leg PluginTenderLeg) *Service {
	s.pluginTender, s.pluginTenderLeg = known, leg
	return s
}
