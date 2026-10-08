package products

import (
	"context"
	"os"
	"testing"

	appdb "karots-suite/internal/db"

	"github.com/shopspring/decimal"
)

// A price-update sheet (name + price only) changes the price and nothing else:
// the cost, category, supplier and reorder level stay. ImportOne commits, so
// the rows are removed afterwards.
func TestImportOnePartialSheetKeepsOtherColumns(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx := context.Background()
	var unitID, catID, supID int64
	must(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	must(t, conn.GetContext(ctx, &catID, `INSERT INTO categories (name) VALUES ('TEST partial cat') RETURNING id`))
	must(t, conn.GetContext(ctx, &supID, `INSERT INTO suppliers (name) VALUES ('TEST partial sup') RETURNING id`))
	defer func() {
		conn.ExecContext(ctx, `DELETE FROM stock WHERE product_id IN (SELECT id FROM products WHERE name = 'TEST partial rice')`) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM products WHERE name = 'TEST partial rice'`)                                         //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM categories WHERE id = $1`, catID)                                                   //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM suppliers WHERE id = $1`, supID)                                                    //nolint:errcheck
	}()
	svc := NewService(conn)
	_, err = svc.ImportOne(ctx, ImportRow{Name: "TEST partial rice", CategoryID: catID, UnitID: unitID, PreferredSupplier: &supID,
		Cost: decimal.NewFromInt(900), Selling: decimal.NewFromInt(1200), Reorder: 7})
	must(t, err)
	var fallback int64
	must(t, conn.GetContext(ctx, &fallback, `SELECT id FROM categories WHERE id <> $1 LIMIT 1`, catID))
	res, err := svc.ImportOne(ctx, ImportRow{Name: "TEST partial rice", CategoryID: fallback, UnitID: unitID,
		Selling: decimal.NewFromInt(1300), Given: map[string]bool{"name": true, "selling_price": true}})
	must(t, err)
	var p struct {
		Cost    string `db:"cost_price"`
		Sell    string `db:"selling_price"`
		Cat     int64  `db:"category_id"`
		Sup     *int64 `db:"preferred_supplier_id"`
		Reorder int    `db:"reorder_level"`
	}
	must(t, conn.GetContext(ctx, &p, `SELECT cost_price::text, selling_price::text, category_id, preferred_supplier_id, reorder_level FROM products WHERE name = 'TEST partial rice'`))
	if res.Action != "updated" || p.Sell != "1300.00" || p.Cost != "900.00" || p.Cat != catID || p.Sup == nil || *p.Sup != supID || p.Reorder != 7 {
		t.Fatalf("after a price-only sheet: %+v (%s)", p, res.Action)
	}
}
