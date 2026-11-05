package api

import (
	"context"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/warranty"

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
	if r.Sales, err = viewsOf(ctx, db, svc, ids); err != nil {
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
		WHERE r.created_at > $1 AND r.sale_id <= $2 AND s.created_at >= now() - make_interval(days => $3)
		UNION
		SELECT u.sale_id FROM warranty_claims wc JOIN warranty_units u ON u.id = wc.unit_id
		JOIN sales s ON s.id = u.sale_id
		WHERE wc.created_at > $1 AND u.sale_id <= $2 AND s.created_at >= now() - make_interval(days => $3)`, since, after, RecentSalesDays)
	if err != nil {
		return nil, apperr.Internal("failed to read returns", err)
	}
	if r.Changed, err = viewsOf(ctx, db, svc, changed); err != nil {
		return nil, err
	}
	return r, nil
}

func viewsOf(ctx context.Context, db *sqlx.DB, svc *sales.Service, ids []int64) ([]SaleView, error) {
	out := make([]SaleView, 0, len(ids))
	for _, id := range ids {
		d, err := svc.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		v := saleView(d)
		if v.Warranty, err = saleWarranty(ctx, db, d); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// saleWarranty is nil for most sales: nothing they sold carries a warranty.
func saleWarranty(ctx context.Context, db *sqlx.DB, d *sales.Detail) (*SaleWarranty, error) {
	var serials []string
	if err := db.SelectContext(ctx, &serials, `SELECT serial_no FROM warranty_units WHERE sale_id = $1 AND serial_no IS NOT NULL ORDER BY id`, d.Sale.ID); err != nil {
		return nil, apperr.Internal("failed to read warranty units", err)
	}
	var warranted bool
	if err := db.GetContext(ctx, &warranted, `SELECT EXISTS (SELECT 1 FROM sale_items si JOIN products p ON p.id = si.product_id
		WHERE si.sale_id = $1 AND p.warranty_months > 0)`, d.Sale.ID); err != nil {
		return nil, apperr.Internal("failed to read warranty lines", err)
	}
	if len(serials) == 0 && !warranted {
		return nil, nil
	}
	svc := warranty.NewService(db)
	w := &SaleWarranty{Units: []WarrantyUnitRow{}}
	for _, serial := range serials {
		u, err := svc.Lookup(ctx, serial)
		if err != nil {
			return nil, err
		}
		w.Units = append(w.Units, unitRow(u.Unit))
	}
	if warranted {
		r, err := svc.LookupByReceipt(ctx, d.Sale.ReceiptNo)
		if err != nil {
			return nil, err
		}
		v := warrantyReceiptView(r)
		w.Receipt = &v
	}
	return w, nil
}
