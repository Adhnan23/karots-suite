package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Profit by category rolls a child's sales up into its parent without counting
// a line twice; Expenses and Tax total the range (tax by shop day); till
// session totals cover every session, not the 50 on screen; Finance exports
// its statement.
func TestMoneyReports(t *testing.T) {
	te := newTestEnv(t)
	admin := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	var cashier, unit int64
	te.db.Get(&cashier, `SELECT id FROM users ORDER BY id LIMIT 1`) //nolint:errcheck
	te.db.Get(&unit, `SELECT id FROM units LIMIT 1`)               //nolint:errcheck
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM sales WHERE receipt_no LIKE 'T-RM-%'`)                       //nolint:errcheck
		te.db.Exec(`DELETE FROM products WHERE name LIKE 'T rm %'`)                          //nolint:errcheck
		te.db.Exec(`DELETE FROM categories WHERE name LIKE 'T rm %'`)                        //nolint:errcheck
		te.db.Exec(`DELETE FROM expenses WHERE category LIKE 'T rm %'`)                      //nolint:errcheck
		te.db.Exec(`DELETE FROM cash_register WHERE opened_at >= '2097-02-01' AND opened_at < '2097-03-01'`) //nolint:errcheck
	})
	var parent, child int64
	te.db.Get(&parent, `INSERT INTO categories (name) VALUES ('T rm Drinks') RETURNING id`)                          //nolint:errcheck
	te.db.Get(&child, `INSERT INTO categories (name, parent_id) VALUES ('T rm Juice', $1) RETURNING id`, parent) //nolint:errcheck
	sell := func(n int, cat int64, price, cost int, at string, tax int) {
		var pid, sid int64
		te.db.Get(&pid, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price) VALUES ($1,$2,$3,$4,$5) RETURNING id`, //nolint:errcheck
			fmt.Sprintf("T rm p%d", n), cat, unit, cost, price)
		te.db.Get(&sid, `INSERT INTO sales (receipt_no, subtotal, tax, total, paid_amount, status, cashier_id, created_at)
			VALUES ($1,$2,$3,$2::numeric+$3::numeric,$2::numeric+$3::numeric,'completed',$4,$5) RETURNING id`, fmt.Sprintf("T-RM-%d", n), price, tax, cashier, at) //nolint:errcheck
		te.db.MustExec(`INSERT INTO sale_items (sale_id, product_id, quantity, unit_price, subtotal, cost_price) VALUES ($1,$2,1,$3,$3,$4)`, sid, pid, price, cost)
	}
	sell(1, parent, 100, 50, "2097-02-03T10:00:00+05:30", 0)
	sell(2, child, 200, 100, "2097-02-04T00:30:00+05:30", 18) // 00:30: still 3 Feb in UTC
	q := "from=2097-02-01&to=2097-02-28"

	cp := decode[CategoryProfitReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/profit-by-category?"+q, nil, admin))
	node := map[string]CategoryProfitNode{}
	for _, n := range cp.Nodes {
		node[n.Name] = n
	}
	if cp.Revenue != "300.00" || cp.Profit != "150.00" || node["T rm Drinks"].Revenue != "300.00" || node["T rm Juice"].Depth != 1 || node["T rm Juice"].Profit != "100.00" {
		t.Fatalf("by category %+v", cp)
	}
	only := decode[CategoryProfitReport](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/reports/profit-by-category?category=%d&%s", child, q), nil, admin))
	if only.Revenue != "200.00" || len(only.Nodes) != 1 || only.Nodes[0].Depth != 0 {
		t.Fatalf("one category %+v", only)
	}

	te.db.MustExec(`INSERT INTO expenses (category, amount, expense_date) VALUES ('T rm Rent', 5000, '2097-02-01'), ('T rm Rent', 1000, '2097-02-15'), ('T rm Power', 800, '2097-02-20')`)
	ex := decode[ExpensesReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/expenses?"+q, nil, admin))
	if ex.Total != "6800.00" || ex.Count != 3 || ex.Rows[0].Category != "T rm Rent" || ex.Rows[0].Total != "6000.00" {
		t.Fatalf("expenses %+v", ex)
	}

	tx := decode[TaxReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/tax?"+q, nil, admin))
	if tx.Tax != "18.00" || len(tx.Rows) != 2 || tx.Rows[1].Day != "2097-02-04" || tx.Rows[1].Tax != "18.00" {
		t.Fatalf("tax %+v", tx)
	}

	// 55 closed sessions, each 10 short: the totals see all 55.
	for i := 0; i < 55; i++ {
		te.db.MustExec(`INSERT INTO cash_register (user_id, opening_cash, expected_cash, closing_cash, difference, opened_at, closed_at)
			VALUES ($1, 1000, 1500, 1490, -10, $2, $2::timestamptz + interval '8 hours')`, cashier, fmt.Sprintf("2097-02-%02dT08:00:00+05:30", 1+i%27))
	}
	ts := decode[TillTotals](t, te.do(http.MethodGet, "/api/v1/admin/tills/summary?"+q, nil, admin))
	if ts.Sessions != 55 || ts.OverShort != "-550.00" || ts.Short != 55 || ts.Counted != "81950.00" {
		t.Fatalf("till totals %+v", ts)
	}
	if rec := te.do(http.MethodGet, "/api/v1/admin/tills?format=csv&"+q, nil, admin); strings.Count(rec.Body.String(), "\n") != 56 {
		t.Fatalf("till export lines %d", strings.Count(rec.Body.String(), "\n"))
	}
	if rec := te.do(http.MethodGet, "/api/v1/admin/finance?format=csv&"+q, nil, admin); !strings.Contains(rec.Body.String(), "Net profit,") {
		t.Fatalf("finance export %d %s", rec.Code, rec.Body.String())
	}
}
