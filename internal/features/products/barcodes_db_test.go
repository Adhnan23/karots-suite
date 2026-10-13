package products

import (
	"context"
	"os"
	"testing"

	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
)

// barcodeFixture makes two products in their own category: A (main TEST-B-MAIN,
// extra TEST-B-EXTRA) and B (no barcode). Everything is removed afterwards.
func barcodeFixture(t *testing.T) (*sqlx.DB, int64, int64) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var unitID, catID, a, b int64
	must(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	must(t, conn.GetContext(ctx, &catID, `INSERT INTO categories (name) VALUES ('TEST barcodes cat') RETURNING id`))
	must(t, conn.GetContext(ctx, &a, `INSERT INTO products (name, barcode, category_id, unit_id) VALUES ('TEST bc Coke 1L', 'TEST-B-MAIN', $1, $2) RETURNING id`, catID, unitID))
	must(t, conn.GetContext(ctx, &b, `INSERT INTO products (name, category_id, unit_id) VALUES ('TEST bc Coca Cola', $1, $2) RETURNING id`, catID, unitID))
	_, err = conn.ExecContext(ctx, `INSERT INTO product_barcodes (product_id, barcode, source) VALUES ($1, 'TEST-B-EXTRA', 'form')`, a)
	must(t, err)
	t.Cleanup(func() {
		conn.ExecContext(ctx, `DELETE FROM product_barcodes WHERE product_id IN ($1, $2)`, a, b) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM products WHERE id IN ($1, $2)`, a, b)                 //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM categories WHERE id = $1`, catID)                     //nolint:errcheck
		conn.Close()
	})
	return conn, a, b
}

// An extra barcode finds its product everywhere a main one does: the scanner
// lookup, the search (ranked first), the "already used" check and the import.
func TestExtraBarcodeIsFoundLikeTheMainOne(t *testing.T) {
	conn, a, _ := barcodeFixture(t)
	ctx := context.Background()
	svc := NewService(conn)
	p, err := svc.GetByBarcode(ctx, "TEST-B-EXTRA")
	if err != nil || p.ID != a {
		t.Fatalf("scanner lookup by extra: %v %+v", err, p)
	}
	if len(p.Barcodes) != 1 || p.Barcodes[0] != "TEST-B-EXTRA" {
		t.Fatalf("product row carries its extras: %v", p.Barcodes)
	}
	rows, err := NewRepository(conn).List(ctx, ListQuery{Search: "TEST-B-EXTRA", Limit: 10, Page: 1})
	if err != nil || len(rows) == 0 || rows[0].ID != a {
		t.Fatalf("search by extra: %v %+v", err, rows)
	}
	if ok, err := NewRepository(conn).BarcodeExists(ctx, "TEST-B-EXTRA"); err != nil || !ok {
		t.Fatalf("BarcodeExists must see extras: %v %v", ok, err)
	}
	code := "TEST-B-EXTRA"
	tx := conn.MustBeginTx(ctx, nil)
	defer tx.Rollback() //nolint:errcheck
	if got, err := NewRepository(tx).FindByBarcode(ctx, code); err != nil || got.ID != a {
		t.Fatalf("import matcher (FindByBarcode in a tx): %v", err)
	}
}

// One code, one product — enforced by the database in both directions.
func TestBarcodeCannotSitOnTwoProducts(t *testing.T) {
	conn, a, b := barcodeFixture(t)
	ctx := context.Background()
	if _, err := conn.ExecContext(ctx, `INSERT INTO product_barcodes (product_id, barcode, source) VALUES ($1, 'TEST-B-MAIN', 'form')`, b); err == nil {
		t.Fatal("an extra equal to another product's main barcode was accepted")
	}
	if _, err := conn.ExecContext(ctx, `UPDATE products SET barcode = 'TEST-B-EXTRA' WHERE id = $1`, b); err == nil {
		t.Fatal("a main barcode equal to another product's extra was accepted")
	}
	// Its own extra may become its main (Make main swaps them in one tx).
	tx := conn.MustBeginTx(ctx, nil)
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, `DELETE FROM product_barcodes WHERE barcode = 'TEST-B-EXTRA'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE products SET barcode = 'TEST-B-EXTRA' WHERE id = $1`, a); err != nil {
		t.Fatalf("own extra → main: %v", err)
	}
}

// A scan stamps when that code was last seen, main or extra.
func TestTouchBarcodeStampsLastScan(t *testing.T) {
	conn, a, _ := barcodeFixture(t)
	ctx := context.Background()
	repo := NewRepository(conn)
	must(t, repo.TouchBarcode(ctx, "TEST-B-EXTRA"))
	must(t, repo.TouchBarcode(ctx, "TEST-B-MAIN"))
	var extra, main bool
	must(t, conn.GetContext(ctx, &extra, `SELECT last_scanned_at IS NOT NULL FROM product_barcodes WHERE barcode = 'TEST-B-EXTRA'`))
	must(t, conn.GetContext(ctx, &main, `SELECT barcode_scanned_at IS NOT NULL FROM products WHERE id = $1`, a))
	if !extra || !main {
		t.Fatalf("stamped extra=%v main=%v", extra, main)
	}
}

// A merged product is gone from every list, "Show disabled" included.
func TestMergedProductIsHiddenEverywhere(t *testing.T) {
	conn, a, b := barcodeFixture(t)
	ctx := context.Background()
	_, err := conn.ExecContext(ctx, `UPDATE products SET is_active = false, merged_into_id = $1 WHERE id = $2`, a, b)
	must(t, err)
	repo := NewRepository(conn)
	rows, err := repo.List(ctx, ListQuery{Search: "TEST bc", Limit: 10, Page: 1, IncludeInactive: true})
	must(t, err)
	for _, p := range rows {
		if p.ID == b {
			t.Fatal("merged product listed with Show disabled")
		}
	}
	all, err := repo.ListAll(ctx, true)
	must(t, err)
	for _, p := range all {
		if p.ID == b {
			t.Fatal("merged product in ListAll(includeDisabled)")
		}
	}
	tx := conn.MustBeginTx(ctx, nil)
	defer tx.Rollback() //nolint:errcheck
	if p, err := findDisabled(ctx, tx, nil, "TEST bc Coca Cola"); err == nil {
		t.Fatalf("import would revive a merged product: %+v", p)
	}
}
