package api

import (
	"context"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/sales"

	"github.com/jmoiron/sqlx"
)

// RecentSalesDays is how far back a till's copy of the sales goes.
const RecentSalesDays = 30

// BuildRecentSales reads one page of sales newer than after, plus the sales up
// to after that were returned since `since`, each as the Returns tab sees it.
func BuildRecentSales(ctx context.Context, db *sqlx.DB, svc *sales.Service, after int64, since time.Time, limit int) (*RecentSales, error) {
	now := time.Now().UTC()
	r := &RecentSales{Sales: []SaleView{}, Changed: []SaleView{}, NextAfter: after, NextSince: now.Format(time.RFC3339Nano)}
	if err := db.GetContext(ctx, &r.MaxID, `SELECT COALESCE(max(id), 0) FROM sales`); err != nil {
		return nil, apperr.Internal("failed to read sales", err)
	}
	if after > r.MaxID {
		r.Reset, after, since, r.NextAfter = true, 0, time.Time{}, 0
	}
	var ids []int64
	err := db.SelectContext(ctx, &ids, `SELECT id FROM sales WHERE id > $1
		AND created_at >= now() - make_interval(days => $2) ORDER BY id LIMIT $3`, after, RecentSalesDays, limit+1)
	if err != nil {
		return nil, apperr.Internal("failed to read sales", err)
	}
	if len(ids) > limit {
		ids, r.More = ids[:limit], true
	}
	if r.Sales, err = viewsOf(ctx, svc, ids); err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		r.NextAfter = ids[len(ids)-1]
	}
	if since.IsZero() {
		return r, nil
	}
	var changed []int64
	err = db.SelectContext(ctx, &changed, `SELECT DISTINCT r.sale_id FROM sale_returns r
		JOIN sales s ON s.id = r.sale_id
		WHERE r.created_at > $1 AND r.sale_id <= $2 AND s.created_at >= now() - make_interval(days => $3)`, since, after, RecentSalesDays)
	if err != nil {
		return nil, apperr.Internal("failed to read returns", err)
	}
	if r.Changed, err = viewsOf(ctx, svc, changed); err != nil {
		return nil, err
	}
	return r, nil
}

func viewsOf(ctx context.Context, svc *sales.Service, ids []int64) ([]SaleView, error) {
	out := make([]SaleView, 0, len(ids))
	for _, id := range ids {
		d, err := svc.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, saleView(d))
	}
	return out, nil
}
