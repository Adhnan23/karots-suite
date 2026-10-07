package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/datetime"
)

// Top products ranks by the asked metric with totals over every product sold
// (the Pareto line needs the whole, not the page); Product sales lays last
// year's same period beside this one; Sales by cashier splits two staff who
// share a name.
func TestProductReportsAndCashiers(t *testing.T) {
	te := newTestEnv(t)
	admin := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	var cat, unit int64
	te.db.Get(&cat, `SELECT id FROM categories LIMIT 1`) //nolint:errcheck
	te.db.Get(&unit, `SELECT id FROM units LIMIT 1`)     //nolint:errcheck
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM sales WHERE receipt_no LIKE 'T-RP-%'`)             //nolint:errcheck
		te.db.Exec(`DELETE FROM products WHERE name LIKE 'T rp %'`)                //nolint:errcheck
		te.db.Exec(`DELETE FROM users WHERE phone IN ('0700000991','0700000992')`) //nolint:errcheck
	})
	cashier := func(phone string) (id int64) {
		te.db.Get(&id, `INSERT INTO users (name, phone, pin_hash, role) VALUES ('T Same Name', $1, 'x', 'cashier') RETURNING id`, phone) //nolint:errcheck
		return
	}
	c1, c2 := cashier("0700000991"), cashier("0700000992")
	product := func(name string, cost int) (id int64) {
		te.db.Get(&id, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price) VALUES ($1,$2,$3,$4,100) RETURNING id`, name, cat, unit, cost) //nolint:errcheck
		return
	}
	a, b, c := product("T rp A", 90), product("T rp B", 20), product("T rp C", 50)
	n := 0
	sell := func(at time.Time, by, pid int64, qty int, price int) {
		n++
		var sid int64
		te.db.Get(&sid, `INSERT INTO sales (receipt_no, subtotal, total, paid_amount, status, cashier_id, created_at)
			VALUES ($1,$2,$2,$2,'completed',$3,$4) RETURNING id`, fmt.Sprintf("T-RP-%d", n), qty*price, by, at) //nolint:errcheck
		te.db.MustExec(`INSERT INTO sale_items (sale_id, product_id, quantity, unit_price, subtotal, cost_price)
			SELECT $1, $2, $3::numeric, $4::numeric, $3::numeric * $4::numeric, cost_price FROM products WHERE id = $2`, sid, pid, qty, price)
	}
	loc := datetime.Location
	may := func(d int) time.Time { return time.Date(2098, 5, d, 0, 30, 0, 0, loc) } // 00:30: the UTC day before
	sell(may(3), c1, a, 8, 100)                                                     // A: 800 revenue, 80 profit
	sell(may(3), c2, b, 5, 100)                                                     // B: 500, 400 profit
	sell(may(10), c1, c, 1, 100)                                                    // C: 100, 50 profit
	sell(time.Date(2097, 5, 3, 12, 0, 0, 0, loc), c1, a, 3, 100)                    // last year

	q := "from=2098-05-01&to=2098-05-31"
	top := decode[TopProductsReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/top-products?order=profit&limit=2&"+q, nil, admin))
	if len(top.Rows) != 2 || top.Rows[0].Name != "T rp B" || top.Products != 3 || top.Revenue != "1400.00" || top.Profit != "530.00" || top.Qty != "14" {
		t.Fatalf("top %+v", top)
	}
	if rec := te.do(http.MethodGet, "/api/v1/admin/reports/top-products?format=csv&"+q, nil, admin); !strings.Contains(rec.Body.String(), "T rp A,8,800.00,80.00,10.0") {
		t.Fatalf("top csv %s", rec.Body.String())
	}

	ps := decode[ProductSalesReport](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/reports/product-sales?product=%d&grain=day&%s", a, q), nil, admin))
	if ps.Name != "T rp A" || ps.Qty != "8" || len(ps.Rows) != 1 || ps.Rows[0].Day != "2098-05-03" || ps.Rows[0].LastYearQty != "3" {
		t.Fatalf("product sales %+v", ps)
	}

	cs := decode[CashierReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/sales-by-cashier?"+q, nil, admin))
	if len(cs.Rows) != 2 || cs.Count != 3 || cs.Net != "1400.00" || cs.Rows[0].Net != "900.00" {
		t.Fatalf("cashiers %+v", cs)
	}
}
