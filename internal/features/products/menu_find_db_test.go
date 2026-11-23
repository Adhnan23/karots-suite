package products_test

import (
	"context"
	"testing"

	"karots-suite/internal/features/products"
	"karots-suite/internal/ledgercheck/ltest"
)

// MENU-5: a menu line finds its product by any barcode (a pack's too), then by
// its exact name with case and spacing ignored — disabled products included
// (MENU-13) — and a name two products share is a reason to skip, never a guess.
func TestFindForMenu(t *testing.T) {
	db := ltest.NewDB(t)
	ctx := context.Background()
	var unit, cat, panadol, off, twinA, twinB, pack int64
	q := func(dst *int64, sql string, args ...any) {
		t.Helper()
		if err := db.GetContext(ctx, dst, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	q(&unit, `SELECT id FROM units LIMIT 1`)
	q(&cat, `INSERT INTO categories (name) VALUES ('Menu find') RETURNING id`)
	q(&panadol, `INSERT INTO products (name, barcode, category_id, unit_id) VALUES ('Panadol 500mg', 'PAN-1', $1, $2) RETURNING id`, cat, unit)
	q(&off, `INSERT INTO products (name, category_id, unit_id, is_active) VALUES ('Old Syrup', $1, $2, false) RETURNING id`, cat, unit)
	q(&twinA, `INSERT INTO products (name, category_id, unit_id) VALUES ('Twin', $1, $2) RETURNING id`, cat, unit)
	q(&twinB, `INSERT INTO products (name, category_id, unit_id) VALUES ('twin', $1, $2) RETURNING id`, cat, unit)
	q(&pack, `INSERT INTO product_packs (product_id, name, size) VALUES ($1, 'Card', 10) RETURNING id`, panadol)
	if _, err := db.ExecContext(ctx, `INSERT INTO product_barcodes (product_id, barcode, source, pack_id) VALUES ($1, 'PAN-CARD', 'form', $2)`, panadol, pack); err != nil {
		t.Fatal(err)
	}
	r := products.NewRepository(db)
	for _, c := range []struct {
		barcode, name string
		want          int64
		reason        string
	}{
		{"PAN-1", "", panadol, ""},
		{"PAN-CARD", "anything", panadol, ""},
		{"", "  panadol   500MG ", panadol, ""},
		{"NOPE", "Panadol 500mg", panadol, ""},
		{"", "Old Syrup", off, ""},
		{"", "Twin", 0, "two products have this name"},
		{"", "Not here", 0, "no product with this barcode or name"},
		{"", "", 0, "no product with this barcode or name"},
	} {
		id, reason, err := r.FindForMenu(ctx, c.barcode, c.name)
		if err != nil {
			t.Fatalf("%q / %q: %v", c.barcode, c.name, err)
		}
		if id != c.want || reason != c.reason {
			t.Errorf("%q / %q: got %d %q, want %d %q", c.barcode, c.name, id, reason, c.want, c.reason)
		}
	}
	_ = twinA
	_ = twinB
}
