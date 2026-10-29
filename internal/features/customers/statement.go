// Package customers manages registered customers, especially credit ("vade")
// buyers. Walk-in sales leave customer_id NULL.
package customers

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// LedgerEntry is one line of a customer statement (a debit raises what the
// customer owes; a credit lowers it).
type LedgerEntry struct {
	Date    time.Time
	Kind    string
	Ref     string
	Debit   decimal.Decimal
	Credit  decimal.Decimal
	Balance decimal.Decimal
}

// Statement is a customer's full credit ledger with a forward running balance.
type Statement struct {
	Customer    Customer
	Entries     []LedgerEntry
	TotalDebit  decimal.Decimal
	TotalCredit decimal.Decimal
}

// Statement builds the full credit ledger for a customer: credit-sale debits,
// return credits, and repayment credits, time-ordered with a running balance.
// Note: repayments are only logged from migration 0028 onward, so the running
// balance reconciles to the current outstanding balance for activity recorded
// since then; the authoritative figure is always Customer.OutstandingBalance.
func (s *Service) Statement(ctx context.Context, id int64) (*Statement, error) {
	cust, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	type evRow struct {
		CreatedAt time.Time       `db:"created_at"`
		Ref       string          `db:"ref"`
		Amount    decimal.Decimal `db:"amount"`
	}
	var creditSales, returns []evRow
	if err := s.db.SelectContext(ctx, &creditSales, `
		SELECT created_at, receipt_no AS ref, (total - paid_amount) AS amount
		FROM sales
		WHERE customer_id = $1 AND total > paid_amount
		ORDER BY created_at`, id); err != nil {
		return nil, err
	}
	if err := s.db.SelectContext(ctx, &returns, `
		SELECT sr.created_at, s.receipt_no AS ref, sr.credit_reduction AS amount
		FROM sale_returns sr JOIN sales s ON s.id = sr.sale_id
		WHERE s.customer_id = $1 AND sr.credit_reduction > 0
		ORDER BY sr.created_at`, id); err != nil {
		return nil, err
	}
	type payRow struct {
		CreatedAt time.Time       `db:"created_at"`
		Method    string          `db:"method"`
		Reference *string         `db:"reference"`
		Amount    decimal.Decimal `db:"amount"`
	}
	var pays []payRow
	if err := s.db.SelectContext(ctx, &pays, `
		SELECT cp.created_at, cp.method, cp.amount,
		       COALESCE((SELECT string_agg(c.number || ' (' || c.bank || ')', ', ' ORDER BY c.id)
		                 FROM cheques c WHERE c.customer_payment_id = cp.id), cp.reference) AS reference
		FROM customer_payments cp WHERE cp.customer_id = $1 ORDER BY cp.created_at`, id); err != nil {
		return nil, err
	}
	// A bounced / cancelled cheque puts its amount back on what they owe.
	var returned []ChequeReturn
	if err := s.db.SelectContext(ctx, &returned, ChequeReturnsSQL+` WHERE direction = 'in' AND customer_id = $1`, id); err != nil {
		return nil, err
	}

	entries := make([]LedgerEntry, 0, len(creditSales)+len(returns)+len(pays)+1)
	// An onboarding opening balance is the carried-forward debit that predates all
	// recorded activity, so the running balance reconciles to OutstandingBalance.
	// A negative one (allowed by AdjustOpening) is a pre-system advance: a credit.
	if cust.OpeningBalance.IsPositive() {
		entries = append(entries, LedgerEntry{Date: cust.CreatedAt, Kind: "Opening balance", Debit: cust.OpeningBalance})
	} else if cust.OpeningBalance.IsNegative() {
		entries = append(entries, LedgerEntry{Date: cust.CreatedAt, Kind: "Opening balance", Credit: cust.OpeningBalance.Neg()})
	}
	for _, r := range creditSales {
		entries = append(entries, LedgerEntry{Date: r.CreatedAt, Kind: "Credit sale", Ref: r.Ref, Debit: r.Amount})
	}
	for _, r := range returns {
		entries = append(entries, LedgerEntry{Date: r.CreatedAt, Kind: "Return", Ref: r.Ref, Credit: r.Amount})
	}
	for _, r := range pays {
		ref := r.Method
		if r.Reference != nil && strings.TrimSpace(*r.Reference) != "" {
			ref += " · " + *r.Reference
		}
		if r.Amount.IsNegative() { // advance handed back (RefundAdvanceTx)
			entries = append(entries, LedgerEntry{Date: r.CreatedAt, Kind: "Advance refunded", Ref: ref, Debit: r.Amount.Neg()})
			continue
		}
		entries = append(entries, LedgerEntry{Date: r.CreatedAt, Kind: "Payment", Ref: ref, Credit: r.Amount})
	}
	for _, r := range returned {
		entries = append(entries, r.Entry())
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Date.Before(entries[j].Date) })

	bal, totalDebit, totalCredit := decimal.Zero, decimal.Zero, decimal.Zero
	for i := range entries {
		bal = bal.Add(entries[i].Debit).Sub(entries[i].Credit)
		entries[i].Balance = bal
		totalDebit = totalDebit.Add(entries[i].Debit)
		totalCredit = totalCredit.Add(entries[i].Credit)
	}
	return &Statement{Customer: *cust, Entries: entries, TotalDebit: totalDebit, TotalCredit: totalCredit}, nil
}

// ChequeReturn is a bounced or cancelled cheque: a statement debit (the money
// is owed again). Shared with the supplier statement.
type ChequeReturn struct {
	Day    time.Time       `db:"status_date"`
	Status string          `db:"status"`
	Number string          `db:"number"`
	Bank   string          `db:"bank"`
	Amount decimal.Decimal `db:"amount"`
}

// ChequeReturnsSQL selects ChequeReturn rows; callers add the WHERE (direction + party).
const ChequeReturnsSQL = `SELECT COALESCE(status_date, created_at::date) AS status_date, status, number,
	COALESCE(NULLIF(bank, ''), (SELECT name FROM lockers WHERE id = locker_id), '') AS bank, amount
	FROM (SELECT * FROM cheques WHERE status IN ('bounced', 'cancelled')) c`

// Entry is the statement line: "Cheque bounced" / "Cheque cancelled", 123456 (BOC).
func (r ChequeReturn) Entry() LedgerEntry {
	ref := r.Number
	if r.Bank != "" {
		ref += " (" + r.Bank + ")"
	}
	return LedgerEntry{Date: r.Day, Kind: "Cheque " + r.Status, Ref: ref, Debit: r.Amount}
}
