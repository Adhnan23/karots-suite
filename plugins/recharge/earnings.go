package recharge

import (
	"context"
	"time"

	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/activity"
	"karots-suite/internal/money"
	"karots-suite/internal/plugin"

	"github.com/shopspring/decimal"
)

// Earnings is the shop's real Reload & Bills earning for [from, to): commission
// booked with each move, count differences (a promo, a rate that changed, money
// missing), and — from older data — its service charges and the bonus its
// closed float sessions showed. Service charges the suite sells are ordinary
// revenue already; face value is pass-through and never counted.
func Earnings(ctx context.Context, q appdb.Queryer, from, to time.Time) (decimal.Decimal, error) {
	var v decimal.Decimal
	err := q.GetContext(ctx, &v, `
		SELECT
		  COALESCE((SELECT SUM(float_delta) FROM recharge_transactions
		            WHERE type IN ('commission','count') AND created_at >= $1 AND created_at < $2), 0)
		+ COALESCE((SELECT SUM(service_charge) FROM recharge_transactions WHERE created_at >= $1 AND created_at < $2), 0)
		+ COALESCE((SELECT SUM(service_charge) FROM recharge_bill_tx WHERE created_at >= $1 AND created_at < $2), 0)
		+ COALESCE((SELECT SUM(ds.closing - ds.opening - COALESCE(td.delta, 0))
		            FROM recharge_device_sessions ds
		            LEFT JOIN (SELECT session_id, device_id, SUM(float_delta) AS delta FROM recharge_transactions
		                       WHERE session_id IS NOT NULL GROUP BY session_id, device_id) td
		              ON td.session_id = ds.session_id AND td.device_id = ds.device_id
		            WHERE ds.closing IS NOT NULL AND ds.closed_at >= $1 AND ds.closed_at < $2), 0)`, from, to)
	return v, err
}

// OldFloatBought is the float older data booked as an expense ("Float top-up")
// in [from, to). Buying float is cash turned into float, not a cost: the P&L
// adds it back on its dates. The suite books no such expense.
func OldFloatBought(ctx context.Context, q appdb.Queryer, from, to time.Time) (decimal.Decimal, error) {
	var v decimal.Decimal
	err := q.GetContext(ctx, &v, `
		SELECT COALESCE(SUM(e.amount), 0) FROM expenses e
		WHERE e.id IN (SELECT expense_id FROM recharge_transactions WHERE expense_id IS NOT NULL)
		  AND e.expense_date >= $1 AND e.expense_date < $2`, from, to)
	return v, err
}

// expenseOwner claims the old float top-up expenses (voiding one would leave the
// float it bought on the device).
func (s *Store) expenseOwner(ctx context.Context, id int64) (string, error) {
	var n int
	if err := s.db.GetContext(ctx, &n, `SELECT count(*) FROM recharge_transactions WHERE expense_id = $1`, id); err != nil {
		return "", err
	}
	if n > 0 {
		return "a reload float bought (Reload & Bills)", nil
	}
	return "", nil
}

// moneyLocations: float sitting on devices is the shop's money (Cash Flow, Z report).
func (s *Store) moneyLocations(ctx context.Context) ([]plugin.MoneyLocation, error) {
	fs, err := s.Floats(ctx, s.db, false)
	if err != nil {
		return nil, err
	}
	out := make([]plugin.MoneyLocation, 0, len(fs))
	for _, f := range fs {
		out = append(out, plugin.MoneyLocation{Name: f.Carrier + " — " + f.Label, Balance: f.Balance})
	}
	return out, nil
}

// readiness: each device's float against its target — the gap is cash to buy float.
func (p *Plugin) readiness(ctx context.Context) ([]plugin.ReadinessSection, error) {
	fs, err := p.store.Floats(ctx, p.store.db, false)
	if err != nil {
		return nil, err
	}
	sym := "Rs."
	if cfg, err := p.core.Settings.Get(ctx); err == nil && cfg.CurrencySymbol != "" {
		sym = cfg.CurrencySymbol
	}
	sec := plugin.ReadinessSection{Title: "Reload float"}
	for _, f := range fs {
		need := decimal.Max(f.Target.Sub(f.Balance), decimal.Zero)
		target := "—"
		if f.Target.IsPositive() {
			target = money.Format(sym, f.Target)
		} else {
			need = decimal.Zero
		}
		sec.Rows = append(sec.Rows, plugin.ReadinessRow{Name: f.Carrier + " — " + f.Label, Have: money.Format(sym, f.Balance), Target: target, Need: need})
		sec.NeedTotal = sec.NeedTotal.Add(need)
	}
	return []plugin.ReadinessSection{sec}, nil
}

// activityRows feeds float moves and bank bills into Activity (never the system user's).
func (s *Store) activityRows(ctx context.Context, f activity.Filter) ([]activity.Row, error) {
	limit := f.Limit
	if limit <= 0 || limit > 20000 {
		limit = 20000
	}
	var before *time.Time
	if f.Before != nil {
		before = &f.Before.At
	}
	var rows []activity.Row
	err := s.db.SelectContext(ctx, &rows, `
		SELECT id, created_at, user_id, user_name, 'recharge'::text AS source, action, detail, amount FROM (
			SELECT t.id, t.created_at, t.created_by AS user_id, COALESCE(u.name, '') AS user_name,
			       ('float ' || t.type) AS action,
			       (c.name || COALESCE(' · ' || d.label, '') || COALESCE(' · ' || NULLIF(t.customer_number, ''), '')
			        || COALESCE(' — ' || NULLIF(t.reference, ''), '') || COALESCE(' — ' || NULLIF(t.note, ''), '')) AS detail,
			       t.amount
			FROM recharge_transactions t JOIN recharge_carriers c ON c.id = t.carrier_id
			LEFT JOIN recharge_devices d ON d.id = t.device_id LEFT JOIN users u ON u.id = t.created_by
			WHERE t.type <> 'commission'
			UNION ALL
			SELECT -b.id, b.created_at, b.created_by, COALESCE(u.name, ''), ('bill ' || b.type),
			       (b.bank_name || COALESCE(' · ' || NULLIF(b.bill_type, ''), '') || COALESCE(' — ' || NULLIF(b.reference, ''), '')), b.amount
			FROM recharge_bill_tx b LEFT JOIN users u ON u.id = b.created_by
		) x
		WHERE ($1::timestamptz IS NULL OR created_at >= $1) AND ($2::timestamptz IS NULL OR created_at < $2)
		  AND ($3::bigint IS NULL OR user_id = $3)
		  AND ($4 = '' OR action ILIKE '%'||$4||'%' OR detail ILIKE '%'||$4||'%' OR user_name ILIKE '%'||$4||'%')
		  AND ($5::timestamptz IS NULL OR created_at <= $5)
		  AND (user_id IS NULL OR user_id NOT IN (SELECT id FROM users WHERE is_system))
		ORDER BY created_at DESC, id DESC LIMIT $6`, f.From, f.To, f.UserID, f.Query, before, limit)
	if err != nil || f.Before == nil {
		return rows, err
	}
	out := rows[:0]
	for _, r := range rows {
		if f.Before.After(r) {
			out = append(out, r)
		}
	}
	return out, nil
}
