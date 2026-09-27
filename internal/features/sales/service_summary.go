package sales

import (
	"context"
	"time"

	"karots-suite/internal/apperr"

	"github.com/shopspring/decimal"
)

// Summarize returns the totals for every sale matching f, regardless of paging.
func (s *Service) Summarize(ctx context.Context, f ListFilter) (*ListSummary, error) {
	sum, err := s.repo.Summarize(ctx, f)
	if err != nil {
		return nil, apperr.Internal("failed to summarise sales", err)
	}
	return sum, nil
}

// MethodTotal is the money taken via one payment method in a period.
type MethodTotal struct {
	Method string          `db:"method" json:"method"`
	Amount decimal.Decimal `db:"amount" json:"amount"`
}

// PeriodSummary aggregates a cashier's sales within a time window, for the
// day-end (Z) report.
type PeriodSummary struct {
	Count    int             `json:"count"`
	Gross    decimal.Decimal `json:"gross"`
	Discount decimal.Decimal `json:"discount"`
	Returns  decimal.Decimal `json:"returns"` // value of returned lines on these sales
	Net      decimal.Decimal `json:"net"`     // sales total − returns
	ByMethod []MethodTotal   `json:"by_method"`
}

// PeriodSummary totals a cashier's non-void sales between [from,to] and breaks
// payments down by method.
func (s *Service) PeriodSummary(ctx context.Context, cashierID int64, from, to time.Time) (*PeriodSummary, error) {
	out := &PeriodSummary{}
	var agg struct {
		Count    int             `db:"count"`
		Gross    decimal.Decimal `db:"gross"`
		Discount decimal.Decimal `db:"discount"`
		Net      decimal.Decimal `db:"net"`
	}
	err := s.db.GetContext(ctx, &agg, `
		SELECT COUNT(*) AS count,
		       COALESCE(SUM(subtotal),0) AS gross,
		       COALESCE(SUM(discount),0) AS discount,
		       COALESCE(SUM(total),0)    AS net
		FROM sales
		WHERE cashier_id = $1 AND created_at >= $2 AND created_at <= $3 AND status <> 'void'`,
		cashierID, from, to)
	if err != nil {
		return nil, apperr.Internal("failed to summarize sales", err)
	}
	out.Count, out.Gross, out.Discount = agg.Count, agg.Gross, agg.Discount

	// Returns booked against this cashier's sales in the window, valued per unit
	// at the line's net price — mirrors the P&L so the day-end Net is net of
	// refunds (matches the cash actually kept after refunds out of the drawer).
	var returns decimal.Decimal
	err = s.db.GetContext(ctx, &returns, `
		SELECT COALESCE(SUM((si.subtotal / NULLIF(si.quantity,0)) * si.returned_qty), 0)
		FROM sale_items si JOIN sales s ON s.id = si.sale_id
		WHERE s.cashier_id = $1 AND s.created_at >= $2 AND s.created_at <= $3 AND s.status <> 'void'`,
		cashierID, from, to)
	if err != nil {
		return nil, apperr.Internal("failed to summarize returns", err)
	}
	out.Returns = returns
	out.Net = agg.Net.Sub(returns)

	err = s.db.SelectContext(ctx, &out.ByMethod, `
		SELECT pmt.method AS method, COALESCE(SUM(pmt.amount),0) AS amount
		FROM payments pmt JOIN sales s ON s.id = pmt.sale_id
		WHERE s.cashier_id = $1 AND s.created_at >= $2 AND s.created_at <= $3 AND s.status <> 'void'
		GROUP BY pmt.method ORDER BY pmt.method`,
		cashierID, from, to)
	if err != nil {
		return nil, apperr.Internal("failed to summarize payments", err)
	}
	return out, nil
}

// CashCollectedSince totals cash payments taken by a cashier since a time,
// used to compute the expected drawer cash at register close.
// CashCollectedSince is the NET cash a cashier put in the drawer since `since`:
// cash tendered minus change handed back (change is always cash), so an
// overpaid sale doesn't overstate the expected drawer balance.
func (s *Service) CashCollectedSince(ctx context.Context, cashierID int64, since time.Time) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := s.db.GetContext(ctx, &total, `
		SELECT COALESCE((
			SELECT SUM(pmt.amount) FROM payments pmt
			JOIN sales s ON s.id = pmt.sale_id
			WHERE pmt.method = 'cash' AND s.cashier_id = $1 AND s.created_at >= $2
		), 0) - COALESCE((
			SELECT SUM(s.change_given) FROM sales s
			WHERE s.cashier_id = $1 AND s.created_at >= $2
		), 0)`,
		cashierID, since)
	if err != nil {
		return decimal.Zero, apperr.Internal("failed to total cash", err)
	}
	return total, nil
}
