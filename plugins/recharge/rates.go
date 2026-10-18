package recharge

import (
	"github.com/shopspring/decimal"
)

// Kinds of move a carrier pays commission on (recharge_rates.kind).
const (
	KindReload   = "reload"
	KindBillpay  = "billpay"
	KindDeposit  = "deposit"
	KindCashout  = "cashout"
	KindWalletIn = "wallet_in"
)

var kinds = []string{KindReload, KindBillpay, KindDeposit, KindCashout, KindWalletIn}

func validKind(k string) bool {
	for _, x := range kinds {
		if x == k {
			return true
		}
	}
	return false
}

// Rate is what a carrier pays on one kind of move: a percent of the amount and/or
// a fixed amount per move. Charge is the service charge the till offers for it.
type Rate struct {
	Kind    string          `db:"kind"    json:"kind"`
	Percent decimal.Decimal `db:"percent" json:"percent"`
	Fixed   decimal.Decimal `db:"fixed"   json:"fixed"`
	Charge  decimal.Decimal `db:"charge"  json:"charge"`
}

var hundred = decimal.NewFromInt(100)

// Commission on a move of amount (≥ 0) at this rate, to the cent; none on nothing.
func (r Rate) Commission(amount decimal.Decimal) decimal.Decimal {
	if !amount.IsPositive() {
		return decimal.Zero
	}
	return amount.Mul(r.Percent).Div(hundred).Add(r.Fixed).Round(2)
}

// packageCommission is what selling a package earns: its price less the float it uses.
func packageCommission(price, floatCost decimal.Decimal) decimal.Decimal {
	return price.Sub(floatCost)
}

// Suggestion is a rate the counts point to: the extra the floats kept showing
// over the last weeks, spread over the reloads sold on that carrier.
type Suggestion struct {
	CarrierID int64           `json:"carrierId"`
	Carrier   string          `json:"carrier"`
	Current   decimal.Decimal `json:"current"`   // reload percent now
	Suggested decimal.Decimal `json:"suggested"` // reload percent the counts point to
	Counts    int             `json:"counts"`
	Extra     decimal.Decimal `json:"extra"` // Σ count differences
	Reloads   decimal.Decimal `json:"reloads"`
}

// suggestRate reads a carrier's recent counts against its reloads: with at least
// two counts all leaning the same way and an extra of 0.2% or more of the reloads,
// the reload percent is moved by that share (to 0.1%). ok is false otherwise —
// one odd count or a promo is not a new rate.
func suggestRate(current decimal.Decimal, diffs []decimal.Decimal, reloads decimal.Decimal) (decimal.Decimal, bool) {
	if len(diffs) < 2 || !reloads.IsPositive() {
		return decimal.Zero, false
	}
	sum, sign := decimal.Zero, 0
	for _, d := range diffs {
		s := d.Sign()
		if s == 0 || (sign != 0 && s != sign) {
			return decimal.Zero, false
		}
		sign = s
		sum = sum.Add(d)
	}
	share := sum.Div(reloads).Mul(hundred)
	if share.Abs().LessThan(decimal.RequireFromString("0.2")) {
		return decimal.Zero, false
	}
	next := current.Add(share).Round(1)
	if next.IsNegative() {
		next = decimal.Zero
	}
	return next, !next.Equal(current)
}
