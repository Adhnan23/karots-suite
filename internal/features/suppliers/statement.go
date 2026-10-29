// Package suppliers manages wholesale suppliers and their outstanding payables.
package suppliers

import (
	"context"
	"sort"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/customers"

	"github.com/shopspring/decimal"
)

// LedgerEntry is one line of a supplier's payable ledger.
type LedgerEntry struct {
	Date    time.Time
	Kind    string
	Ref     string
	Debit   decimal.Decimal
	Credit  decimal.Decimal
	Balance decimal.Decimal
}

// Statement is a supplier's full payable ledger with a forward running balance.
type Statement struct {
	Supplier    Supplier
	Entries     []LedgerEntry
	TotalDebit  decimal.Decimal
	TotalCredit decimal.Decimal
}

// Statement builds a supplier's payable ledger: the opening balance, purchase
// debits, and payment/return credits, time-ordered with a running balance. Like
// the customer statement it queries the transaction tables directly (no package
// import) and leans on the authoritative outstanding_balance — payments and
// returns are itemised from when their logging began.
func (s *Service) Statement(ctx context.Context, id int64) (*Statement, error) {
	sup, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	type evRow struct {
		CreatedAt time.Time       `db:"created_at"`
		Ref       string          `db:"ref"`
		Amount    decimal.Decimal `db:"amount"`
	}

	var purchases []evRow
	if err := s.db.SelectContext(ctx, &purchases, `
		SELECT created_at,
		       COALESCE(NULLIF(invoice_no, ''), 'PO #' || id::text) AS ref,
		       total AS amount
		FROM purchases
		WHERE supplier_id = $1 AND status <> 'draft'
		ORDER BY created_at`, id); err != nil {
		return nil, apperr.Internal("failed to load purchases", err)
	}
	var returns []evRow
	if err := s.db.SelectContext(ctx, &returns, `
		SELECT created_at, COALESCE(reference, '') AS ref, total AS amount
		FROM purchase_returns
		WHERE supplier_id = $1
		ORDER BY created_at`, id); err != nil {
		return nil, apperr.Internal("failed to load purchase returns", err)
	}
	type payRow struct {
		CreatedAt time.Time       `db:"created_at"`
		Method    string          `db:"method"`
		Reference *string         `db:"reference"`
		Amount    decimal.Decimal `db:"amount"`
	}
	var pays []payRow
	if err := s.db.SelectContext(ctx, &pays, `
		SELECT sp.created_at, sp.method, sp.amount,
		       COALESCE((SELECT string_agg(c.number || ' (' || l.name || ')', ', ' ORDER BY c.id)
		                 FROM cheques c JOIN lockers l ON l.id = c.locker_id WHERE c.supplier_payment_id = sp.id), sp.reference) AS reference
		FROM supplier_payments sp WHERE sp.supplier_id = $1 ORDER BY sp.created_at`, id); err != nil {
		return nil, apperr.Internal("failed to load supplier payments", err)
	}
	// A bounced / cancelled cheque we wrote leaves the supplier owed again.
	var returned []customers.ChequeReturn
	if err := s.db.SelectContext(ctx, &returned, customers.ChequeReturnsSQL+` WHERE direction = 'out' AND supplier_id = $1`, id); err != nil {
		return nil, apperr.Internal("failed to load returned cheques", err)
	}

	// Money a supplier handed back (supplierpay refunds) raises the payable back
	// toward zero — leaving it out left the running balance short of the real one.
	var refunds []payRow
	if err := s.db.SelectContext(ctx, &refunds, `
		SELECT created_at, method, reference, amount
		FROM supplier_refunds WHERE supplier_id = $1 ORDER BY created_at`, id); err != nil {
		return nil, apperr.Internal("failed to load supplier refunds", err)
	}

	// A supplier's credit for a damaged or faulty item (Losses & Recovery →
	// Credited us) lowers the payable too; leaving it out left the running
	// balance above the real one.
	var credits []evRow
	if err := s.db.SelectContext(ctx, &credits, `
		SELECT lr.created_at, p.name AS ref, lr.recovered_value AS amount
		FROM loss_recoveries lr JOIN products p ON p.id = lr.product_id
		WHERE lr.supplier_id = $1 AND lr.outcome = 'paid' AND lr.recovered_value > 0
		ORDER BY lr.created_at`, id); err != nil {
		return nil, apperr.Internal("failed to load supplier credits", err)
	}
	// What a supplier asked on top to replace one (it costs more now) raises it.
	var charges []evRow
	if err := s.db.SelectContext(ctx, &charges, `
		SELECT lr.created_at, p.name AS ref, lr.extra_charge AS amount
		FROM loss_recoveries lr JOIN products p ON p.id = lr.product_id
		WHERE lr.supplier_id = $1 AND lr.extra_charge > 0
		ORDER BY lr.created_at`, id); err != nil {
		return nil, apperr.Internal("failed to load replacement charges", err)
	}

	entries := make([]LedgerEntry, 0, len(purchases)+len(returns)+len(pays)+len(refunds)+len(credits)+len(charges)+1)
	// The opening balance is the carried-forward figure from before this system: a
	// debit when we owed them, a credit when they owed us (a pre-system advance).
	if sup.OpeningBalance.IsPositive() {
		entries = append(entries, LedgerEntry{Date: sup.CreatedAt, Kind: "Opening balance", Debit: sup.OpeningBalance})
	} else if sup.OpeningBalance.IsNegative() {
		entries = append(entries, LedgerEntry{Date: sup.CreatedAt, Kind: "Opening balance", Credit: sup.OpeningBalance.Neg()})
	}
	for _, r := range purchases {
		entries = append(entries, LedgerEntry{Date: r.CreatedAt, Kind: "Purchase", Ref: r.Ref, Debit: r.Amount})
	}
	for _, r := range returns {
		entries = append(entries, LedgerEntry{Date: r.CreatedAt, Kind: "Return", Ref: r.Ref, Credit: r.Amount})
	}
	for _, r := range pays {
		ref := r.Method
		if r.Reference != nil && strings.TrimSpace(*r.Reference) != "" {
			ref += " · " + *r.Reference
		}
		entries = append(entries, LedgerEntry{Date: r.CreatedAt, Kind: "Payment", Ref: ref, Credit: r.Amount})
	}
	for _, r := range refunds {
		ref := r.Method
		if r.Reference != nil && strings.TrimSpace(*r.Reference) != "" {
			ref += " · " + *r.Reference
		}
		entries = append(entries, LedgerEntry{Date: r.CreatedAt, Kind: "Refund received", Ref: ref, Debit: r.Amount})
	}
	for _, r := range credits {
		entries = append(entries, LedgerEntry{Date: r.CreatedAt, Kind: "Credit for a loss", Ref: r.Ref, Credit: r.Amount})
	}
	for _, r := range charges {
		entries = append(entries, LedgerEntry{Date: r.CreatedAt, Kind: "Replacement charge", Ref: r.Ref, Debit: r.Amount})
	}
	for _, r := range returned {
		e := r.Entry()
		entries = append(entries, LedgerEntry{Date: e.Date, Kind: e.Kind, Ref: e.Ref, Debit: e.Debit})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Date.Before(entries[j].Date) })

	bal, totalDebit, totalCredit := decimal.Zero, decimal.Zero, decimal.Zero
	for i := range entries {
		bal = bal.Add(entries[i].Debit).Sub(entries[i].Credit)
		entries[i].Balance = bal
		totalDebit = totalDebit.Add(entries[i].Debit)
		totalCredit = totalCredit.Add(entries[i].Credit)
	}
	return &Statement{Supplier: *sup, Entries: entries, TotalDebit: totalDebit, TotalCredit: totalCredit}, nil
}
