package pocketcashier

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/heldsales"
	"karots-suite/internal/features/sales"

	"github.com/shopspring/decimal"
)

// The phone's bodies, in the till's JSON shapes.

type SaleItem struct {
	ProductID     int64          `json:"productId"`
	Quantity      string         `json:"quantity"`
	Discount      string         `json:"discount"`
	DiscountType  string         `json:"discountType"`
	PriceOverride string         `json:"priceOverride"`
	BatchID       int64          `json:"batchId"`
	Serials       []string       `json:"serials"`
	Description   string         `json:"description"`
	ExpectPrice   string         `json:"expectPrice"` // the unit price the phone showed
	Plugin        map[string]any `json:"plugin,omitempty"`
	Components    []struct {
		ProductID int64  `json:"productId"`
		Quantity  string `json:"quantity"`
	} `json:"components"`
	NoRecipe bool `json:"noRecipe"`
	Payout   bool `json:"payout"`
	Group    *struct {
		No       int    `json:"no"`
		Label    string `json:"label"`
		Sets     string `json:"sets"`
		Itemised bool   `json:"itemised"`
	} `json:"group"`
}

type SaleCheque struct {
	Bank   string `json:"bank"`
	Number string `json:"number"`
	Date   string `json:"date"`
	Note   string `json:"note"`
}

type SalePayment struct {
	Method    string            `json:"method"`
	Amount    string            `json:"amount"`
	Reference string            `json:"reference"`
	AccountID int64             `json:"accountId"`
	Meta      map[string]string `json:"meta,omitempty"`
	Cheque    *SaleCheque       `json:"cheque,omitempty"`
}

type SaleBody struct {
	SaleType          string        `json:"saleType"`
	CustomerID        *int64        `json:"customerId,omitempty"`
	Discount          string        `json:"discount"`
	DiscountType      string        `json:"discountType"`
	ChangeGiven       string        `json:"changeGiven"`
	OnAccountRounding bool          `json:"onAccountRounding"`
	PreparedBy        *int64        `json:"preparedBy,omitempty"` // the sales person whose ticket this was
	Items             []SaleItem    `json:"items"`
	Payments          []SalePayment `json:"payments"`
}

type OpenBody struct {
	OpeningCash    string          `json:"openingCash"`
	Breakdown      json.RawMessage `json:"breakdown,omitempty"`
	SourceLockerID int64           `json:"sourceLockerId"`
}

type CloseBody struct {
	ClosingCash  string          `json:"closingCash"`
	Breakdown    json.RawMessage `json:"breakdown,omitempty"`
	DestLockerID int64           `json:"destLockerId"`
}

type MoveBody struct {
	Amount   string `json:"amount"`
	Reason   string `json:"reason"`
	LockerID int64  `json:"lockerId"`
}

type HoldBody struct {
	Label      string `json:"label"`
	SaleType   string `json:"saleType"`
	CustomerID *int64 `json:"customerId,omitempty"`
	Discount   string `json:"discount"`
	Cart       string `json:"cart"`
	ItemCount  int    `json:"itemCount"`
	Total      string `json:"total"`
}

type UnholdBody struct {
	HeldID int64 `json:"heldId"`
}

// clockSlack is how far ahead of the server a phone's clock may be.
const clockSlack = 5 * time.Minute

func hhmm(t time.Time) string { return t.Local().Format("15:04") }

// run applies one action through the core services.
func (h handlers) run(ctx context.Context, p *Phone, a Action) ActionResult {
	var who struct {
		Role   string `db:"role"`
		System bool   `db:"is_system"`
		Active bool   `db:"is_active"`
		Sales  bool   `db:"sends_to_cashier"`
	}
	if err := h.p.core.DB.GetContext(ctx, &who, `SELECT role::text AS role, is_system, is_active, sends_to_cashier FROM users WHERE id = $1`, a.UserID); err != nil {
		return ActionResult{Status: "failed", Note: "no such person"}
	}
	if who.System || who.Role == "admin" {
		return ActionResult{Status: "failed", Note: "admins sell from a cashier account on phones"}
	}
	// Someone made a sales person after this phone's copy was taken: what they sold
	// stands, flagged (never stop a sale); the drawer isn't theirs. A close still
	// counts, or the till a late sale opened would be stuck.
	if who.Sales && (a.Kind == "open" || a.Kind == "withdraw" || a.Kind == "deposit") {
		return ActionResult{Status: "failed", Note: "a sales person doesn't handle money"}
	}
	var flag string
	switch {
	case !who.Active:
		flag = "sold by a person who was turned off meanwhile"
	case who.Sales:
		flag = "sold on a phone by a sales person, who doesn't handle money"
	}
	switch a.Kind {
	case "sale":
		return h.runSale(ctx, p, a, flag)
	case "call":
		return h.runCall(ctx, p, a)
	case "open":
		return h.runOpen(ctx, p, a)
	case "close":
		return h.runClose(ctx, a)
	case "withdraw", "deposit":
		return h.runMove(ctx, a)
	case "hold":
		return h.runHold(ctx, a)
	case "unhold":
		var b UnholdBody
		if err := json.Unmarshal(a.Body, &b); err != nil {
			return failed(err)
		}
		if err := h.p.core.Held.Delete(ctx, b.HeldID, a.UserID); err != nil {
			return ActionResult{Status: "skipped", Note: "already resumed"}
		}
		return ActionResult{Status: "applied", HeldID: b.HeldID}
	}
	return ActionResult{Status: "failed", Note: "unknown action " + a.Kind}
}

func (h handlers) runSale(ctx context.Context, p *Phone, a Action, personFlag string) ActionResult {
	var b SaleBody
	raw, err := h.resolveBody(ctx, a.Body) // a plugin's line may refer to something made on the phone
	if err != nil {
		return failed(err)
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return failed(err)
	}
	now := time.Now()
	at := a.At
	rungAt := a.At
	var flags [][2]string
	// A clock ahead of the server: count it now (never in a day not yet come).
	if at.After(now.Add(clockSlack)) {
		flags = append(flags, [2]string{sales.FlagClock, fmt.Sprintf("the phone said %s; counted when it arrived", a.At.Local().Format("2 Jan 15:04"))})
		at, rungAt = now, now
	}
	in := toCreate(b)
	in.ClientRef, in.RungAt, in.Review, in.DeferDrawerKick = a.Ref, &rungAt, true, true
	// Its till still open since before it was rung: it counts when it was sold.
	// Otherwise (closed meanwhile — its Z may be printed) it counts now, in an
	// open session (a new one, flagged, when there is none).
	sess, err := h.p.core.CashRegister.Current(ctx, a.UserID)
	if err != nil {
		return failed(err)
	}
	switch {
	case sess != nil && !sess.OpenedAt.After(at):
		in.At = &at
	case sess != nil:
		flags = append(flags, [2]string{sales.FlagLate, "rung at " + hhmm(rungAt) + " on " + p.Name + ", before this till opened; counted when it arrived"})
	default:
		if _, err := h.p.core.CashRegister.Open(ctx, a.UserID, cashregister.OpenInput{OpeningCash: "0", Remote: true,
			ReviewNote: "opened for late sales from " + p.Name}); err != nil {
			return failed(err)
		}
		flags = append(flags, [2]string{sales.FlagLate, "rung at " + hhmm(rungAt) + " on " + p.Name + " after its till was closed; counted when it arrived"})
	}
	if personFlag != "" {
		flags = append(flags, [2]string{sales.FlagInactiveUser, personFlag})
	}
	d, err := h.p.core.Sales.Create(ctx, in, a.UserID)
	if err != nil {
		return failed(err)
	}
	r := ActionResult{Status: "applied", SaleID: d.Sale.ID, ReceiptNo: d.Sale.ReceiptNo}
	if d.Duplicate { // booked before (a send cut off before its reply): nothing more to add
		return r
	}
	for _, f := range flags {
		_ = sales.AddFlag(ctx, h.p.core.DB, d.Sale.ID, f[0], f[1])
	}
	h.flagLookalike(ctx, d)
	return r
}

func toCreate(b SaleBody) sales.CreateInput {
	in := sales.CreateInput{CustomerID: b.CustomerID, SaleType: b.SaleType, Discount: b.Discount, DiscountType: b.DiscountType,
		ChangeGiven: b.ChangeGiven, OnAccountRounding: b.OnAccountRounding, PreparedBy: b.PreparedBy}
	for _, it := range b.Items {
		var plug map[string]json.RawMessage
		for k, v := range it.Plugin {
			raw, _ := json.Marshal(v)
			if plug == nil {
				plug = map[string]json.RawMessage{}
			}
			plug[k] = raw
		}
		item := sales.ItemInput{ProductID: it.ProductID, Quantity: it.Quantity, Discount: it.Discount, DiscountType: it.DiscountType,
			PriceOverride: it.PriceOverride, BatchID: it.BatchID, Serials: it.Serials, Description: it.Description, ExpectPrice: it.ExpectPrice, Plugin: plug}
		item.NoRecipe, item.Payout = it.NoRecipe, it.Payout
		for _, c := range it.Components {
			item.Components = append(item.Components, sales.ServiceComponent{ProductID: c.ProductID, Quantity: c.Quantity})
		}
		if g := it.Group; g != nil {
			sets, _ := decimal.NewFromString(g.Sets) // a bad value stays 0 and is refused as a validation error
			item.Group = &sales.LineGroup{No: g.No, Label: g.Label, Sets: sets, Itemised: g.Itemised}
		}
		in.Items = append(in.Items, item)
	}
	for _, p := range b.Payments {
		pi := sales.PaymentInput{Method: p.Method, Amount: p.Amount, AccountID: p.AccountID, Meta: p.Meta}
		if ref := strings.TrimSpace(p.Reference); ref != "" {
			pi.Reference = &ref
		}
		if c := p.Cheque; c != nil {
			pi.Cheque = &sales.ChequeInput{Bank: strings.TrimSpace(c.Bank), Number: strings.TrimSpace(c.Number), Date: strings.TrimSpace(c.Date), Note: strings.TrimSpace(c.Note)}
		}
		in.Payments = append(in.Payments, pi)
	}
	return in
}

func (h handlers) runOpen(ctx context.Context, p *Phone, a Action) ActionResult {
	var b OpenBody
	if err := json.Unmarshal(a.Body, &b); err != nil {
		return failed(err)
	}
	if sess, err := h.p.core.CashRegister.Current(ctx, a.UserID); err != nil {
		return failed(err)
	} else if sess != nil {
		return ActionResult{Status: "skipped", Note: "already open since " + hhmm(sess.OpenedAt)}
	}
	in := cashregister.OpenInput{OpeningCash: b.OpeningCash, Breakdown: b.Breakdown, SourceLockerID: b.SourceLockerID, Remote: true}
	at := a.At
	in.At = &at
	if _, err := h.p.core.CashRegister.Open(ctx, a.UserID, in); err != nil {
		// Its time can't be used (a clock off, or a close came after it): open now, noted.
		in.At, in.ReviewNote = nil, "opened on "+p.Name+" at "+hhmm(a.At)+"; counted from when it arrived"
		if _, err := h.p.core.CashRegister.Open(ctx, a.UserID, in); err != nil {
			return failed(err)
		}
	}
	return ActionResult{Status: "applied"}
}

func (h handlers) runClose(ctx context.Context, a Action) ActionResult {
	var b CloseBody
	if err := json.Unmarshal(a.Body, &b); err != nil {
		return failed(err)
	}
	sess, err := h.p.core.CashRegister.Current(ctx, a.UserID)
	if err != nil {
		return failed(err)
	}
	if sess == nil {
		return ActionResult{Status: "skipped", Note: "no open till to close"}
	}
	in := cashregister.CloseInput{ClosingCash: b.ClosingCash, Breakdown: b.Breakdown, DestLockerID: b.DestLockerID, Remote: true}
	if at := a.At; !at.Before(sess.OpenedAt) && !at.After(time.Now().Add(clockSlack)) {
		in.At = &at
	}
	if _, err := h.p.core.CashRegister.Close(ctx, a.UserID, in); err != nil {
		return failed(err)
	}
	return ActionResult{Status: "applied"}
}

func (h handlers) runMove(ctx context.Context, a Action) ActionResult {
	var b MoveBody
	if err := json.Unmarshal(a.Body, &b); err != nil {
		return failed(err)
	}
	in := cashregister.MovementInput{Amount: b.Amount, Reason: b.Reason, CounterLockerID: b.LockerID, DeferDrawerKick: true}
	var err error
	if a.Kind == "withdraw" {
		_, err = h.p.core.CashRegister.Withdraw(ctx, a.UserID, in)
	} else {
		_, err = h.p.core.CashRegister.PayIn(ctx, a.UserID, in)
	}
	if err != nil {
		return failed(err)
	}
	return ActionResult{Status: "applied"}
}

func (h handlers) runHold(ctx context.Context, a Action) ActionResult {
	var b HoldBody
	if err := json.Unmarshal(a.Body, &b); err != nil {
		return failed(err)
	}
	if !json.Valid([]byte(b.Cart)) {
		return ActionResult{Status: "failed", Note: "the cart can't be read"}
	}
	held, err := h.p.core.Held.Hold(ctx, a.UserID, heldsales.Input{Label: b.Label, SaleType: b.SaleType, CustomerID: b.CustomerID,
		Discount: b.Discount, Cart: json.RawMessage(b.Cart), ItemCount: b.ItemCount, Total: b.Total})
	if err != nil {
		return failed(err)
	}
	return ActionResult{Status: "applied", HeldID: held.ID}
}
