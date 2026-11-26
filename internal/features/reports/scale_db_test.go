package reports

import (
	"encoding/json"
	"testing"
	"time"

	"karots-suite/internal/features/sales"
)

// RPT-38: a report's work grows with the period's sale lines, never with the
// shop's product count. A typical shop's shape: the 300 products it sells came
// first (low ids), 1,700 it never sold came after; 3,000 sales of two lines.
// The planner then thinks walking the products list in id order stops early
// and walks ~300 entries for every sale (Finance (year) took 35 s on a fast PC).
// Every per-sale query a report or the sales list runs must read only a handful
// of rows per line.
func TestReportWorkDoesNotGrowWithProducts(t *testing.T) {
	db := freshDB(t, "reports_scale_test_db")
	db.MustExec(`
		INSERT INTO categories (name) VALUES ('Scale');
		INSERT INTO users (name, phone, pin_hash) VALUES ('Cashier', '0700000038', 'x');
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, pass_through)
		  SELECT 'P' || g, (SELECT id FROM categories WHERE name = 'Scale'), (SELECT id FROM units LIMIT 1), 60, 100, g = 300
		  FROM generate_series(1, 2000) g;
		INSERT INTO sales (receipt_no, cashier_id, subtotal, total, paid_amount, created_at)
		  SELECT 'S' || g, (SELECT id FROM users WHERE phone = '0700000038'), 200, 200, 200, now() - g * interval '2 hours'
		  FROM generate_series(1, 3000) g;
		INSERT INTO sale_items (sale_id, product_id, quantity, unit_price, subtotal, cost_price)
		  SELECT s.id, p.id, 1, 100, 100, 60
		  FROM sales s JOIN products p ON p.name IN ('P' || (s.id * 7 % 300 + 1), 'P' || (s.id * 13 % 300 + 1));
		ANALYZE;`)
	var lines float64
	mustR(t, db.Get(&lines, `SELECT COUNT(*) FROM sale_items`))
	from, to := time.Now().AddDate(-1, 0, 0), time.Now().Add(time.Hour)

	for name, q := range map[string]string{
		"flow (P&L, products, categories)": `SELECT SUM(revenue), SUM(cost) FROM (` + flowSQL + `) f`,
		"sold (by time, by cashier)":       `SELECT SUM(revenue), SUM(cost) FROM (` + soldSQL + `) x`,
		"by cashier":                       `SELECT SUM(net) FROM (` + byCashierSQL + ` GROUP BY u.id, u.name) c`,
	} {
		var raw string
		mustR(t, db.Get(&raw, `EXPLAIN (ANALYZE, FORMAT JSON) `+q, from, to))
		var plan []struct{ Plan planNode }
		mustR(t, json.Unmarshal([]byte(raw), &plan))
		perLine := plan[0].Plan.rows() / lines
		t.Logf("%s: %.1f rows handled per sale line", name, perLine)
		if perLine > 40 {
			t.Errorf("RPT-38: %s handles %.0f rows per sale line (want ≤ 40) — it grows with the products list", name, perLine)
		}
	}
}

// RPT-38, the sales list: its totals (and the Sales report's) and a page of it
// never run a step once per sale of the period — the year's totals ran four
// small queries for each of ~39,000 sales.
func TestSalesListWorkIsNotPerSale(t *testing.T) {
	db := freshDB(t, "reports_scale_list_db")
	db.MustExec(`
		INSERT INTO categories (name) VALUES ('Scale');
		INSERT INTO users (name, phone, pin_hash) VALUES ('Cashier', '0700000039', 'x');
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		  SELECT 'P' || g, (SELECT id FROM categories WHERE name = 'Scale'), (SELECT id FROM units LIMIT 1), 60, 100
		  FROM generate_series(1, 300) g;
		INSERT INTO sales (receipt_no, cashier_id, subtotal, total, paid_amount, created_at)
		  SELECT 'S' || g, (SELECT id FROM users WHERE phone = '0700000039'), 200, 200, 200, now() - g * interval '2 hours'
		  FROM generate_series(1, 3000) g;
		INSERT INTO sale_items (sale_id, product_id, quantity, unit_price, subtotal, cost_price)
		  SELECT s.id, p.id, 1, 100, 100, 60
		  FROM sales s JOIN products p ON p.name IN ('P' || (s.id * 7 % 300 + 1), 'P' || (s.id * 13 % 300 + 1));
		ANALYZE;`)
	from, to := time.Now().AddDate(-1, 0, 0), time.Now().Add(time.Hour)
	tz := "Asia/Colombo"
	for name, c := range map[string]struct {
		q    string
		args []any
	}{
		"totals (Summarize)": {sales.SummarySQL, []any{from, to, nil, nil, nil, nil, nil, tz, nil, nil}},
		"a page of 50 (List)": {sales.ListSQL, []any{from, to, nil, nil, nil, 50, 0, nil, nil, tz, nil, nil}},
	} {
		var raw string
		mustR(t, db.Get(&raw, `EXPLAIN (ANALYZE, FORMAT JSON) `+c.q, c.args...))
		var plan []struct{ Plan planNode }
		mustR(t, json.Unmarshal([]byte(raw), &plan))
		loops := plan[0].Plan.maxLoops()
		t.Logf("%s: the busiest step ran %.0f times (3,000 sales)", name, loops)
		if loops > 100 {
			t.Errorf("RPT-38: %s runs a step %.0f times — once per sale (want ≤ 100)", name, loops)
		}
	}
}

// planNode is one step of an EXPLAIN ANALYZE plan; rows is every row it and the
// steps under it handled, over all their loops.
type planNode struct {
	Rows  float64    `json:"Actual Rows"`
	Loops float64    `json:"Actual Loops"`
	Plans []planNode `json:"Plans"`
}

func (n planNode) maxLoops() float64 {
	m := n.Loops
	for _, c := range n.Plans {
		m = max(m, c.maxLoops())
	}
	return m
}

func (n planNode) rows() float64 {
	sum := n.Rows * n.Loops
	for _, c := range n.Plans {
		sum += c.rows()
	}
	return sum
}
