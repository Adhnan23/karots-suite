package sales

import (
	"karots-suite/internal/apperr"
	"karots-suite/internal/money"

	"github.com/shopspring/decimal"
)

// MethodOnAccount is the payment method that settles a sale without money
// changing hands. It is shown to cashiers as "On account".
const MethodOnAccount = "credit"

// MethodCheque is a customer's cheque: money taken now, held in Cheques in
// hand until it clears (or bounces back onto their account).
const MethodCheque = "cheque"

// Tender is a sale's payment split into what was actually received and what the
// customer now owes. They are kept apart because an on-account line is a debt,
// not money: counting it as paid would report a debt as a completed cash sale,
// and would inflate the drawer's expected balance.
type Tender struct {
	Paid      decimal.Decimal
	OnAccount decimal.Decimal
	// Cash is the cash part of Paid — the only part change can come out of.
	Cash decimal.Decimal
}

// SplitTender sorts parallel method/amount lists into the two figures.
func SplitTender(methods []string, amounts []decimal.Decimal) Tender {
	t := Tender{Paid: decimal.Zero, OnAccount: decimal.Zero, Cash: decimal.Zero}
	for i, m := range methods {
		if i >= len(amounts) {
			break
		}
		if m == MethodOnAccount {
			t.OnAccount = t.OnAccount.Add(amounts[i])
		} else {
			t.Paid = t.Paid.Add(amounts[i])
		}
		if m == "cash" {
			t.Cash = t.Cash.Add(amounts[i])
		}
	}
	return t
}

// CheckTender validates a split against the bill.
//
// A shortfall is refused rather than silently becoming credit — that silence
// was the defect: a credit sale would be recorded as an ordinary retail one and
// the debt was invisible in the receipts list. The till resolves a shortfall
// through its confirmation prompt and posts an explicit on-account line, so
// this is the backstop rather than the cashier's experience of it.
func CheckTender(t Tender, total decimal.Decimal, hasCustomer bool, availableCredit decimal.Decimal, allowOverLimit bool) error {
	covered := t.Paid.Add(t.OnAccount)
	if covered.LessThan(total) {
		return apperr.Validation(money.Display(total.Sub(covered)) +
			" is unpaid — take the money, or put it on a customer's account")
	}
	// Change is handed back from the drawer, so it can't be more than the cash
	// handed over: a card or online payment above what's due would turn the
	// difference into cash out of the till.
	// A payout (total below zero) is change too: what the customer is given.
	if change := covered.Sub(total); change.GreaterThan(t.Cash.Add(decimal.Max(total.Neg(), decimal.Zero))) && !t.OnAccount.IsPositive() {
		return apperr.Validation("card, online and cheque can't be more than what's due — only cash gets change")
	}
	if !t.OnAccount.IsPositive() {
		return nil
	}
	if !hasCustomer {
		return apperr.Validation("choose a customer to put this on account")
	}
	// No change against money that was never paid: a part-account sale must
	// land exactly on the total.
	if covered.GreaterThan(total) {
		return apperr.Validation("this sale is over-paid — reduce the amount on account")
	}
	if !allowOverLimit && t.OnAccount.GreaterThan(availableCredit) {
		return apperr.Conflict("credit limit exceeded (available " +
			money.Display(availableCredit) + ")")
	}
	return nil
}

// CreditSummary is how a credit sale left the customer's account, for the receipt.
type CreditSummary struct {
	Due         decimal.Decimal // this sale's on-account part
	FromAdvance decimal.Decimal // part of Due covered by an advance they already had
	Balance     decimal.Decimal // account after the sale: > 0 still owed, < 0 advance left
}

// Credit explains a credit sale's on-account part for the receipt; ok is false
// when nothing went on account. live is the customer's current balance, used
// only for older sales saved before the balance was snapshotted.
func (s Sale) Credit(live decimal.Decimal) (cs CreditSummary, ok bool) {
	due := s.Total.Sub(s.PaidAmount)
	if s.Status != "credit" || !due.IsPositive() {
		return cs, false
	}
	if s.CustomerBalanceAfter == nil {
		// Older sale: live balance, never printed below this sale's own due.
		return CreditSummary{Due: due, Balance: decimal.Max(live, due)}, true
	}
	bal := *s.CustomerBalanceAfter
	// before = bal - due; an advance (before < 0) covered up to all of due.
	used := decimal.Min(due, decimal.Max(decimal.Zero, due.Sub(bal)))
	return CreditSummary{Due: due, FromAdvance: used, Balance: bal}, true
}
