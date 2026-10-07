package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Returns are listed by when the money went back; dues include a deactivated
// debtor (Finance counts them) but never an advance, aged by the oldest unpaid
// bill — and a bill settled by sending goods back is not unpaid; spend and
// purchases net out what was credited back.
func TestPeopleReports(t *testing.T) {
	te := newTestEnv(t)
	admin := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM sale_return_items WHERE return_id IN (SELECT r.id FROM sale_returns r JOIN sales s ON s.id = r.sale_id WHERE s.receipt_no LIKE 'T-RPE-%')`) //nolint:errcheck
		te.db.Exec(`DELETE FROM sale_returns WHERE sale_id IN (SELECT id FROM sales WHERE receipt_no LIKE 'T-RPE-%')`)                                                      //nolint:errcheck
		te.db.Exec(`DELETE FROM sales WHERE receipt_no LIKE 'T-RPE-%'`)                                                                                                     //nolint:errcheck
		te.db.Exec(`DELETE FROM purchases WHERE supplier_id IN (SELECT id FROM suppliers WHERE name LIKE 'T rpe %')`)                                                       //nolint:errcheck
		te.db.Exec(`DELETE FROM suppliers WHERE name LIKE 'T rpe %'`)                                                                                                       //nolint:errcheck
		te.db.Exec(`DELETE FROM customers WHERE name LIKE 'T rpe %'`)                                                                                                       //nolint:errcheck
		te.db.Exec(`DELETE FROM products WHERE name LIKE 'T rpe %'`)                                                                                                        //nolint:errcheck
	})
	var cashier, cat, unit, prod int64
	te.db.Get(&cashier, `SELECT id FROM users ORDER BY id LIMIT 1`)                                                                                             //nolint:errcheck
	te.db.Get(&cat, `SELECT id FROM categories LIMIT 1`)                                                                                                        //nolint:errcheck
	te.db.Get(&unit, `SELECT id FROM units LIMIT 1`)                                                                                                            //nolint:errcheck
	te.db.Get(&prod, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price) VALUES ('T rpe soap',$1,$2,40,100) RETURNING id`, cat, unit) //nolint:errcheck

	// A June sale, part returned on 2 July.
	var sale, item, ret int64
	te.db.Get(&sale, `INSERT INTO sales (receipt_no, subtotal, total, paid_amount, status, cashier_id, created_at)
		VALUES ('T-RPE-1', 300, 300, 300, 'partially_returned', $1, '2098-06-28T10:00:00+05:30') RETURNING id`, cashier) //nolint:errcheck
	te.db.Get(&item, `INSERT INTO sale_items (sale_id, product_id, quantity, unit_price, subtotal, cost_price, returned_qty)
		VALUES ($1,$2,3,100,300,40,2) RETURNING id`, sale, prod) //nolint:errcheck
	te.db.Get(&ret, `INSERT INTO sale_returns (sale_id, refund_amount, reason, created_by, created_at)
		VALUES ($1, 200, 'leaking', $2, '2098-07-02T09:00:00+05:30') RETURNING id`, sale, cashier) //nolint:errcheck
	te.db.MustExec(`INSERT INTO sale_return_items (return_id, sale_item_id, product_id, quantity, refund_amount) VALUES ($1,$2,$3,2,200)`, ret, item, prod)

	july := decode[ReturnsReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/returns?from=2098-07-01&to=2098-07-31", nil, admin))
	if july.Returns != 1 || july.Refund != "200.00" || july.Qty != "2" || len(july.Top) != 1 || july.Top[0].Name != "T rpe soap" {
		t.Fatalf("july %+v", july)
	}
	lines := decode[[]ReturnLineRow](t, te.do(http.MethodGet, "/api/v1/admin/reports/returns/lines?from=2098-07-01&to=2098-07-31", nil, admin))
	if len(lines) != 1 || lines[0].ReceiptNo != "T-RPE-1" || lines[0].SaleDate != "2098-06-28" || lines[0].Reason != "leaking" {
		t.Fatalf("lines %+v", lines)
	}
	if june := decode[ReturnsReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/returns?from=2098-06-01&to=2098-06-30", nil, admin)); june.Returns != 0 {
		t.Fatalf("a July refund showed in June: %+v", june)
	}

	// Customers: one owing 500 since 40 days ago, one deactivated owing 300,
	// one 200 in advance.
	var owing int64
	te.db.Get(&owing, `INSERT INTO customers (name, outstanding_balance, credit_limit) VALUES ('T rpe owes', 500, 400) RETURNING id`) //nolint:errcheck
	te.db.MustExec(`INSERT INTO customers (name, outstanding_balance, is_active) VALUES ('T rpe gone', 300, false)`)
	te.db.MustExec(`INSERT INTO customers (name, outstanding_balance) VALUES ('T rpe ahead', -200)`)
	te.db.MustExec(`INSERT INTO sales (receipt_no, subtotal, total, paid_amount, status, cashier_id, customer_id, created_at)
		VALUES ('T-RPE-2', 500, 500, 0, 'credit', $1, $2, now() - interval '40 days')`, cashier, owing)
	cd := decode[DuesReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/customer-dues", nil, admin))
	mine := map[string]DueRow{}
	for _, r := range cd.Rows {
		mine[r.Name] = r
	}
	if mine["T rpe owes"].AgeDays != 40 || !mine["T rpe owes"].Flag || !mine["T rpe gone"].Disabled || mine["T rpe ahead"].Name != "" {
		t.Fatalf("customer dues %+v", mine)
	}

	// Suppliers: a bill settled by a return (paid 200 + credited 800) is not
	// unpaid; the 95-day-old 500 bill is, and is past 30 credit days.
	var sup int64
	te.db.Get(&sup, `INSERT INTO suppliers (name, outstanding_balance, credit_days) VALUES ('T rpe dist', 500, 30) RETURNING id`) //nolint:errcheck
	te.db.MustExec(`INSERT INTO purchases (supplier_id, status, total, paid_amount, credited_amount, created_at) VALUES
		($1, 'received', 1000, 200, 800, now() - interval '120 days'),
		($1, 'received', 500, 0, 0, now() - interval '95 days'),
		($1, 'received', 300, 100, 50, '2098-07-05T10:00:00+05:30')`, sup)
	sd := decode[DuesReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/supplier-dues", nil, admin))
	for _, r := range sd.Rows {
		if r.Name == "T rpe dist" && (r.AgeDays != 95 || !r.Flag) {
			t.Fatalf("supplier due %+v", r)
		}
	}
	sp := decode[SupplierSpendReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/supplier-spend?from=2098-07-01&to=2098-07-31", nil, admin))
	if len(sp.Rows) != 1 || sp.Rows[0].Spend != "250.00" || sp.Rows[0].Due != "150.00" {
		t.Fatalf("spend %+v", sp)
	}
	pr := decode[PurchasesReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/purchases?from=2098-07-01&to=2098-07-31", nil, admin))
	if pr.Count != 1 || pr.Due != "150.00" || pr.Rows[0].Due != "150.00" {
		t.Fatalf("purchases %+v", pr)
	}
}
