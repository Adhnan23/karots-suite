package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/features/products"

	"github.com/jmoiron/sqlx"
)

// mergeDup makes a duplicate of the fixture's plain product: its own barcode and an
// extra, 4 in one lot that follows its price (90 against the plain one's 100), in the
// fixture's group, and one sale of it.
func mergeDup(t *testing.T, te *testEnv, f tillFix) (dup int64, main, extra string) {
	t.Helper()
	ctx := context.Background()
	sfx := time.Now().UnixNano() % 100_000_000_000
	main, extra = fmt.Sprintf("77%011d", sfx), fmt.Sprintf("76%011d", sfx)
	if err := te.db.GetContext(ctx, &dup, `INSERT INTO products (name, barcode, category_id, unit_id, cost_price, selling_price)
		SELECT 'T dup ' || $1, $2, category_id, unit_id, 50, 90 FROM products WHERE id = $3 RETURNING id`, sfx, main, f.plainID); err != nil {
		t.Fatal(err)
	}
	te.db.MustExec(`INSERT INTO stock (product_id, quantity) VALUES ($1, 4) ON CONFLICT (product_id) DO UPDATE SET quantity = 4`, dup)
	te.db.MustExec(`INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, selling_price) VALUES ($1, 4, 4, 40, 0)`, dup)
	te.db.MustExec(`INSERT INTO product_barcodes (product_id, barcode, source) VALUES ($1, $2, 'form')`, dup, extra)
	te.db.MustExec(`INSERT INTO product_group_items (group_id, product_id) VALUES ($1, $2)`, f.groupID, dup)
	return dup, main, extra
}

// refsTo counts, per foreign key that points at products(id), the rows still on id
// (merged_into_id itself excluded). Read from the catalog, so a table added later
// is checked too.
func refsTo(t *testing.T, te *testEnv, id int64) map[string]int {
	t.Helper()
	var fks []struct {
		Table  string `db:"tbl"`
		Column string `db:"col"`
	}
	if err := te.db.Select(&fks, `
		SELECT c.conrelid::regclass::text AS tbl, a.attname AS col
		FROM pg_constraint c JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
		WHERE c.contype = 'f' AND c.confrelid = 'products'::regclass`); err != nil {
		t.Fatal(err)
	}
	left := map[string]int{}
	for _, fk := range fks {
		if fk.Table == "products" && fk.Column == "merged_into_id" {
			continue
		}
		var n int
		te.db.Get(&n, fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s = $1`, fk.Table, fk.Column), id) //nolint:errcheck
		if n > 0 {
			left[fk.Table+"."+fk.Column] = n
		}
	}
	return left
}

func TestMergeProducts(t *testing.T) {
	te := newTestEnv(t)
	te.db.Exec(`UPDATE settings SET allow_untracked_cash = true`) //nolint:errcheck
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "cashier")
	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2}}, ck)
	defer te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{}, ck)
	dup, main, extra := mergeDup(t, te, f)
	if rec := te.do(http.MethodPost, "/api/v1/till/sales", sale(dup, "1", PaymentRequest{Method: "cash", Amount: "90"}), ck); rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("sale: %d %s", rec.Code, rec.Body)
	}
	ctx := context.Background()
	svc := products.NewService(te.db)

	pv, err := svc.MergePreview(ctx, dup, f.plainID)
	if err != nil || pv.Refusal != "" || pv.Barcodes != 2 || pv.Stock.String() != "3" || pv.Sales != 1 || !pv.PriceClash {
		t.Fatalf("preview %+v %v", pv, err)
	}
	type money struct {
		Revenue string `db:"revenue"`
		COGS    string `db:"cogs"`
		Stock   string `db:"stock"`
	}
	sums := func() money {
		var m money
		te.db.Get(&m, `SELECT (SELECT COALESCE(SUM(subtotal),0)::text FROM sale_items WHERE product_id IN ($1,$2)) AS revenue,
			(SELECT COALESCE(SUM(cost_price*quantity),0)::text FROM sale_items WHERE product_id IN ($1,$2)) AS cogs,
			(SELECT COALESCE(SUM(qty_remaining*cost_price),0)::text FROM stock_batches WHERE product_id IN ($1,$2)) AS stock`, dup, f.plainID) //nolint:errcheck
		return m
	}
	before := sums()

	if err := svc.Merge(ctx, dup, f.plainID, "keep"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if left := refsTo(t, te, dup); len(left) > 0 {
		t.Fatalf("rows still on the duplicate: %v", left)
	}
	if after := sums(); after != before {
		t.Fatalf("money moved: %+v → %+v", before, after)
	}
	var stock, lots string
	te.db.Get(&stock, `SELECT quantity::numeric(10,0)::text FROM stock WHERE product_id = $1`, f.plainID)                  //nolint:errcheck
	te.db.Get(&lots, `SELECT SUM(qty_remaining)::numeric(10,0)::text FROM stock_batches WHERE product_id = $1`, f.plainID) //nolint:errcheck
	if stock != "13" || lots != "13" {
		t.Fatalf("stock %s, lots %s, want 13 (10 + 3 left of the duplicate)", stock, lots)
	}
	var stamped int
	te.db.Get(&stamped, `SELECT COUNT(*) FROM stock_batches WHERE product_id = $1 AND selling_price = 90 AND cost_price = 40`, f.plainID) //nolint:errcheck
	if stamped != 1 {
		t.Fatal("keep: the duplicate's lot should keep selling at its price (90)")
	}
	for _, code := range []string{main, extra} {
		if p, err := svc.GetByBarcode(ctx, code); err != nil || p.ID != f.plainID {
			t.Fatalf("old code %s should scan to the kept product: %v", code, err)
		}
	}
	var gone struct {
		Active bool    `db:"is_active"`
		Into   *int64  `db:"merged_into_id"`
		Code   *string `db:"barcode"`
	}
	te.db.Get(&gone, `SELECT is_active, merged_into_id, barcode FROM products WHERE id = $1`, dup) //nolint:errcheck
	if gone.Active || gone.Into == nil || *gone.Into != f.plainID || gone.Code != nil {
		t.Fatalf("duplicate not retired: %+v", gone)
	}
	var inGroup int
	te.db.Get(&inGroup, `SELECT COUNT(*) FROM product_group_items WHERE group_id = $1 AND product_id = $2`, f.groupID, f.plainID) //nolint:errcheck
	if inGroup != 1 {
		t.Fatalf("group items not collapsed: %d", inGroup)
	}
	if err := svc.Merge(ctx, dup, f.plainID, "kept"); err == nil {
		t.Fatal("a merged product merged again")
	}
}

func TestMergeProductsRefusals(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ctx := context.Background()
	svc := products.NewService(te.db)
	dup, _, _ := mergeDup(t, te, f)
	refused := func(from, into int64, want string) {
		t.Helper()
		pv, err := svc.MergePreview(ctx, from, into)
		if err != nil || !strings.Contains(pv.Refusal, want) {
			t.Fatalf("preview refusal %q, want %q (%v)", pv.Refusal, want, err)
		}
		if err := svc.Merge(ctx, from, into, "kept"); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("merge must refuse with %q: %v", want, err)
		}
	}
	refused(dup, dup, "itself")
	var uid int64
	te.db.Get(&uid, `SELECT id FROM users LIMIT 1`) //nolint:errcheck
	var held int64
	te.db.Get(&held, `INSERT INTO held_sales (cashier_id, cart) VALUES ($1, $2) RETURNING id`, uid, //nolint:errcheck
		fmt.Sprintf(`{"lines":[{"productId":%d,"qty":1}]}`, dup))
	refused(dup, f.plainID, "held")
	te.db.MustExec(`DELETE FROM held_sales WHERE id = $1`, held)
	te.db.MustExec(`UPDATE products SET unit_id = (SELECT id FROM units WHERE id <> 1 LIMIT 1) WHERE id = $1`, dup)
	refused(dup, f.plainID, "unit")
	te.db.MustExec(`UPDATE products SET unit_id = 1, is_service = true WHERE id = $1`, dup)
	refused(dup, f.plainID, "service")
}

// Set-up rows collapse instead of clashing: a recipe that uses both keeps the kept
// product's line, a recipe line between the two goes, a recipe cost label already
// on the kept product wins, and a conversion between the two goes.
func TestMergeCollapsesSetup(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ctx := context.Background()
	dup, _, _ := mergeDup(t, te, f)
	keep := f.plainID
	te.db.MustExec(`INSERT INTO product_recipes (product_id, component_product_id, qty_per_unit) VALUES
		($1, $3, 1), ($1, $4, 2), ($2, $4, 3), ($4, $3, 1)`, f.scarceID, f.expiringID, keep, dup)
	te.db.MustExec(`INSERT INTO product_recipe_costs (product_id, label, cost_per_unit) VALUES ($1, 'Gas', 5), ($2, 'Gas', 7), ($2, 'Box', 2)`, keep, dup)
	te.db.MustExec(`INSERT INTO product_conversions (from_product_id, to_product_id, ratio) VALUES ($1, $2, 6), ($2, $3, 2), ($1, $3, 3)`, keep, dup, f.lotID)
	if err := products.NewService(te.db).Merge(ctx, dup, keep, "kept"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	var lines []string
	te.db.Select(&lines, `SELECT product_id || '>' || component_product_id || '=' || qty_per_unit::numeric(6,0) FROM product_recipes
		WHERE product_id IN ($1,$2,$3) OR component_product_id IN ($1,$2,$3) ORDER BY 1`, keep, f.scarceID, f.expiringID) //nolint:errcheck
	// scarce used both: the kept product's line stays; expiring used only the duplicate: moved.
	want := []string{fmt.Sprintf("%d>%d=1", f.scarceID, keep), fmt.Sprintf("%d>%d=3", f.expiringID, keep)}
	sort.Strings(want)
	if strings.Join(lines, " ") != strings.Join(want, " ") {
		t.Fatalf("recipes %v, want %v (the line between the two is gone)", lines, want)
	}
	var costs []string
	te.db.Select(&costs, `SELECT label || '=' || cost_per_unit::numeric(6,0) FROM product_recipe_costs WHERE product_id = $1 ORDER BY label`, keep) //nolint:errcheck
	if strings.Join(costs, " ") != "Box=2 Gas=5" {
		t.Fatalf("recipe costs %v", costs)
	}
	var conv []string
	te.db.Select(&conv, `SELECT from_product_id || '>' || to_product_id || '=' || ratio::numeric(6,0) FROM product_conversions WHERE $1 IN (from_product_id, to_product_id)`, keep) //nolint:errcheck
	if len(conv) != 1 || conv[0] != fmt.Sprintf("%d>%d=3", keep, f.lotID) {
		t.Fatalf("conversions %v", conv)
	}
}

// A plugin that fails its part rolls the whole merge back: nothing moved.
func TestMergeRollsBackOnPluginError(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	dup, main, _ := mergeDup(t, te, f)
	prev := products.MergeHook
	products.MergeHook = func(context.Context, *sqlx.Tx, int64, int64) error { return fmt.Errorf("plugin says no") }
	t.Cleanup(func() { products.MergeHook = prev })
	if err := products.NewService(te.db).Merge(context.Background(), dup, f.plainID, "kept"); err == nil || !strings.Contains(err.Error(), "plugin says no") {
		t.Fatalf("merge: %v", err)
	}
	var code *string
	var lots int
	te.db.Get(&code, `SELECT barcode FROM products WHERE id = $1`, dup)               //nolint:errcheck
	te.db.Get(&lots, `SELECT COUNT(*) FROM stock_batches WHERE product_id = $1`, dup) //nolint:errcheck
	if code == nil || *code != main || lots != 1 {
		t.Fatalf("partly merged: barcode %v lots %d", code, lots)
	}
}

// The merge through the admin API: managers only, a preview with the counts, then
// the merge itself; a refusal is a 422 with the reason.
func TestAdminMergeAPI(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	dup, main, _ := mergeDup(t, te, f)
	ck := te.login(t, "0770009594", "manager")
	cashier := te.login(t, "0770009595", "cashier")
	path := fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", dup, f.plainID)
	if rec := te.do(http.MethodGet, path, nil, cashier); rec.Code != http.StatusForbidden {
		t.Fatalf("cashier preview: %d", rec.Code)
	}
	pv := decode[MergePreview](t, te.do(http.MethodGet, path, nil, ck))
	if pv.Refusal != "" || pv.Barcodes != 2 || pv.Stock != "4" || !pv.PriceClash || pv.FromPrice != "90.00" || pv.IntoPrice != "100.00" {
		t.Fatalf("preview %+v", pv)
	}
	if rec := te.do(http.MethodPost, path, MergeRequest{LotPrice: "kept"}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("merge: %d %s", rec.Code, rec.Body)
	}
	if p := decode[TillProduct](t, te.do(http.MethodGet, "/api/v1/till/products/barcode/"+main, nil, cashier)); p.ID != f.plainID {
		t.Fatalf("old code scans to %d", p.ID)
	}
	if rec := te.do(http.MethodPost, path, MergeRequest{LotPrice: "kept"}, ck); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "already merged") {
		t.Fatalf("merge twice: %d %s", rec.Code, rec.Body)
	}
}

// The held-bill check reads where a held cart names products — a line, or a recipe
// component of a line, in either cart shape — not any "id" anywhere in the JSON.
func TestMergeHeldBillCheckIsPrecise(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ctx := context.Background()
	svc := products.NewService(te.db)
	dup, _, _ := mergeDup(t, te, f)
	var uid int64
	te.db.Get(&uid, `SELECT id FROM users LIMIT 1`) //nolint:errcheck
	held := func(cart string) (refused bool) {
		t.Helper()
		var id int64
		if err := te.db.Get(&id, `INSERT INTO held_sales (cashier_id, cart) VALUES ($1, $2) RETURNING id`, uid, cart); err != nil {
			t.Fatal(err)
		}
		defer te.db.MustExec(`DELETE FROM held_sales WHERE id = $1`, id)
		pv, err := svc.MergePreview(ctx, dup, f.plainID)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Contains(pv.Refusal, "held")
	}
	other := f.scarceID
	cases := []struct {
		name, cart string
		want       bool
	}{
		{"a line", fmt.Sprintf(`{"lines":[{"productId":%d}]}`, dup), true},
		{"a recipe component", fmt.Sprintf(`{"lines":[{"productId":%d,"components":[{"productId":%d,"quantity":"1"}]}]}`, other, dup), true},
		{"a karots-pos line", fmt.Sprintf(`[{"id":%d,"qty":1}]`, dup), true},
		{"a karots-pos component", fmt.Sprintf(`[{"id":%d,"components":[{"product_id":%d}]}]`, other, dup), true},
		{"another number that happens to match", fmt.Sprintf(`{"lines":[{"productId":%d,"key":%d,"batchId":%d}],"customer":{"id":%d}}`, other, dup, dup, dup), false},
	}
	for _, c := range cases {
		if got := held(c.cart); got != c.want {
			t.Errorf("%s: refused=%v, want %v", c.name, got, c.want)
		}
	}
}
