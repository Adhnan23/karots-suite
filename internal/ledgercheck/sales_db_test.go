package ledgercheck_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"

	"karots-suite/internal/features/sales"
	"karots-suite/internal/ledgercheck"
	"karots-suite/internal/ledgercheck/ltest"
)

// A discounted sale with a partial return breaks no rule; each corruption below
// breaks exactly the rule it names.
//
//	A: 3 @ 100, 10% item discount → net 270
//	B: 2 @  50                    → net 100
//	bill Rs 10 off → paid 360 (A paid 262.70, B 97.30); 1 of A comes back.
func TestSalesRules(t *testing.T) {
	ctx := context.Background()
	conn := ltest.NewDB(t)
	saleID, itemA := ringAndReturnOne(t, ctx, conn)

	if vs := run(t, ctx, conn); len(vs) != 0 {
		t.Fatalf("clean sale broke rules: %+v", vs)
	}

	cases := []struct {
		rule, breakIt, mend string
	}{
		{"SALE-1", `UPDATE sales SET total = total + 0.01 WHERE id = $1`, `UPDATE sales SET total = total - 0.01 WHERE id = $1`},
		{"SALE-1", `UPDATE sales SET subtotal = subtotal + 1, total = total + 1 WHERE id = $1`, `UPDATE sales SET subtotal = subtotal - 1, total = total - 1 WHERE id = $1`},
		{"TEND-9", `UPDATE payments SET amount = amount + 5 WHERE sale_id = $1`, `UPDATE payments SET amount = amount - 5 WHERE sale_id = $1`},
		{"TEND-1", `UPDATE sales SET paid_amount = paid_amount - 1 WHERE id = $1; UPDATE payments SET amount = amount - 1 WHERE sale_id = $1`,
			`UPDATE sales SET paid_amount = paid_amount + 1 WHERE id = $1; UPDATE payments SET amount = amount + 1 WHERE sale_id = $1`},
		{"RET-1", `UPDATE sale_items SET returned_qty = returned_qty + 1 WHERE id = $2`, `UPDATE sale_items SET returned_qty = returned_qty - 1 WHERE id = $2`},
		{"RET-21", `UPDATE sale_returns SET refund_amount = refund_amount + 1 WHERE sale_id = $1`, `UPDATE sale_returns SET refund_amount = refund_amount - 1 WHERE sale_id = $1`},
		{"RET-4", `UPDATE sale_return_items SET refund_amount = refund_amount + 200 WHERE sale_item_id = $2; UPDATE sale_returns SET refund_amount = refund_amount + 200 WHERE sale_id = $1`,
			`UPDATE sale_return_items SET refund_amount = refund_amount - 200 WHERE sale_item_id = $2; UPDATE sale_returns SET refund_amount = refund_amount - 200 WHERE sale_id = $1`},
		{"RET-2", `UPDATE sales SET status = 'returned' WHERE id = $1`, `UPDATE sales SET status = 'partially_returned' WHERE id = $1`},
		{"SALE-20", `UPDATE stock SET quantity = quantity + 1 WHERE product_id = (SELECT product_id FROM sale_items WHERE id = $2)`,
			`UPDATE stock SET quantity = quantity - 1 WHERE product_id = (SELECT product_id FROM sale_items WHERE id = $2)`},
	}
	for _, c := range cases {
		exec(t, ctx, conn, c.breakIt, saleID, itemA)
		got := rules(run(t, ctx, conn))
		exec(t, ctx, conn, c.mend, saleID, itemA)
		if !slices.Contains(got, c.rule) {
			t.Errorf("%s: corruption not seen (got %v)", c.rule, got)
		}
	}
	if vs := run(t, ctx, conn); len(vs) != 0 {
		t.Fatalf("after mending: %+v", vs)
	}
}

// The auditor writes nothing: it runs inside a read-only transaction.
func TestRunIsReadOnly(t *testing.T) {
	ctx := context.Background()
	conn := ltest.NewDB(t)
	ringAndReturnOne(t, ctx, conn)
	tx, err := conn.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck
	exec(t, ctx, tx, `SET TRANSACTION READ ONLY`)
	if _, err := ledgercheck.Run(ctx, tx); err != nil {
		t.Fatal(err)
	}
}

func ringAndReturnOne(t *testing.T, ctx context.Context, conn *sqlx.DB) (saleID, itemA int64) {
	t.Helper()
	var cat, unit, user, prodA, prodB int64
	get(t, ctx, conn, &cat, `INSERT INTO categories (name) VALUES ('Test') RETURNING id`)
	get(t, ctx, conn, &unit, `SELECT id FROM units ORDER BY id LIMIT 1`)
	get(t, ctx, conn, &user, `SELECT id FROM users ORDER BY id LIMIT 1`)
	get(t, ctx, conn, &prodA, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price) VALUES ('A', $1, $2, 60, 100) RETURNING id`, cat, unit)
	get(t, ctx, conn, &prodB, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price) VALUES ('B', $1, $2, 30, 50) RETURNING id`, cat, unit)
	svc := sales.NewService(conn)
	d, err := svc.Create(ctx, sales.CreateInput{
		SaleType: "retail", Discount: "10", DiscountType: "fixed",
		Items: []sales.ItemInput{
			{ProductID: prodA, Quantity: "3", Discount: "10", DiscountType: "percent", AllowOversell: true},
			{ProductID: prodB, Quantity: "2", AllowOversell: true},
		},
		Payments: []sales.PaymentInput{{Method: "cash", Amount: "400"}},
	}, user)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range d.Items {
		if it.ProductID == prodA {
			itemA = it.ID
		}
	}
	if _, _, err := svc.PartialReturn(ctx, d.Sale.ID, sales.PartialReturnInput{
		Lines: []sales.ReturnLineInput{{SaleItemID: itemA, Quantity: "1"}}}, user); err != nil {
		t.Fatal(err)
	}
	return d.Sale.ID, itemA
}

func run(t *testing.T, ctx context.Context, q sqlx.QueryerContext) []ledgercheck.Violation {
	t.Helper()
	vs, err := ledgercheck.Run(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	return vs
}

func rules(vs []ledgercheck.Violation) []string {
	var out []string
	for _, v := range vs {
		out = append(out, v.Rule)
	}
	return out
}

// exec runs each "; "-separated statement; $1 / $2 are written in as the
// sale / line ids (an UPDATE that names only one can't type an unused $n).
func exec(t *testing.T, ctx context.Context, q sqlx.ExecerContext, sql string, ids ...int64) {
	t.Helper()
	r := []string{}
	for i, id := range ids {
		r = append(r, fmt.Sprintf("$%d", i+1), fmt.Sprint(id))
	}
	for _, one := range strings.Split(strings.NewReplacer(r...).Replace(sql), "; ") {
		if _, err := q.ExecContext(ctx, one); err != nil {
			t.Fatalf("%s: %v", one, err)
		}
	}
}

func get(t *testing.T, ctx context.Context, q sqlx.QueryerContext, dest any, sql string, args ...any) {
	t.Helper()
	if err := sqlx.GetContext(ctx, q, dest, sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}
