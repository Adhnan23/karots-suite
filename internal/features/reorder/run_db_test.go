package reorder

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// fixture: an Auto product (starting guess 0), a Fixed one at 50, and a bag
// whose loose version (1 bag = 25 kg) is sold by the kg. Each sold every day for
// 60 days; removed afterwards.
func fixture(t *testing.T) (db *sqlx.DB, auto, fixed, bag, loose int64) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := appdb.Connect(dsn)
	must(t, err)
	ctx := context.Background()
	var unitID, catID, userID int64
	must(t, db.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	must(t, db.GetContext(ctx, &userID, `SELECT id FROM users ORDER BY id LIMIT 1`))
	must(t, db.GetContext(ctx, &catID, `INSERT INTO categories (name) VALUES ('TEST reorder run') RETURNING id`))
	mk := func(name string, auto bool, level int) int64 {
		var id int64
		must(t, db.GetContext(ctx, &id, `INSERT INTO products (name, category_id, unit_id, reorder_auto, reorder_level, created_at)
			VALUES ($1, $2, $3, $4, $5, now() - interval '400 days') RETURNING id`, name, catID, unitID, auto, level))
		_, err := db.ExecContext(ctx, `INSERT INTO stock (product_id, quantity) VALUES ($1, 1000) ON CONFLICT (product_id) DO UPDATE SET quantity = 1000`, id)
		must(t, err)
		return id
	}
	auto, fixed, bag, loose = mk("TEST run auto", true, 0), mk("TEST run fixed", false, 50), mk("TEST run bag", true, 0), mk("TEST run loose", true, 0)
	_, err = db.ExecContext(ctx, `INSERT INTO product_conversions (from_product_id, to_product_id, ratio) VALUES ($1, $2, 25)`, bag, loose)
	must(t, err)
	var saleIDs []int64
	for d := 0; d < 60; d++ {
		var sid int64
		must(t, db.GetContext(ctx, &sid, `INSERT INTO sales (receipt_no, cashier_id, created_at) VALUES ($1, $2, now() - make_interval(days => $3) - interval '1 hour') RETURNING id`,
			"TESTRR"+time.Now().Format("150405")+string(rune('a'+d%26))+string(rune('a'+d/26)), userID, d))
		saleIDs = append(saleIDs, sid)
		for _, l := range []struct {
			p   int64
			qty int
		}{{auto, 10}, {fixed, 2}, {loose, 25}} { // 25 kg loose a day = 1 bag
			_, err := db.ExecContext(ctx, `INSERT INTO sale_items (sale_id, product_id, quantity, unit_price, subtotal) VALUES ($1, $2, $3, 1, 1)`, sid, l.p, l.qty)
			must(t, err)
		}
	}
	t.Cleanup(func() {
		ids := []any{auto, fixed, bag, loose}
		db.ExecContext(ctx, `DELETE FROM sale_items WHERE product_id IN ($1,$2,$3,$4)`, ids...) //nolint:errcheck
		for _, s := range saleIDs {
			db.ExecContext(ctx, `DELETE FROM sales WHERE id = $1`, s) //nolint:errcheck
		}
		db.ExecContext(ctx, `DELETE FROM product_conversions WHERE from_product_id = $1`, bag) //nolint:errcheck
		db.ExecContext(ctx, `DELETE FROM stock WHERE product_id IN ($1,$2,$3,$4)`, ids...)     //nolint:errcheck
		db.ExecContext(ctx, `DELETE FROM products WHERE id IN ($1,$2,$3,$4)`, ids...)          //nolint:errcheck
		db.ExecContext(ctx, `DELETE FROM categories WHERE id = $1`, catID)                     //nolint:errcheck
		db.Close()
	})
	return db, auto, fixed, bag, loose
}

func TestRunWritesAutoLevelsOnly(t *testing.T) {
	db, auto, fixed, bag, _ := fixture(t)
	ctx := context.Background()
	must(t, RunIDs(ctx, db, time.Now(), []int64{auto, fixed, bag}))

	level := func(id int64) (int, Calc) {
		var row struct {
			Level int    `db:"reorder_level"`
			Calc  []byte `db:"reorder_calc"`
		}
		must(t, db.GetContext(ctx, &row, `SELECT reorder_level, reorder_calc FROM products WHERE id = $1`, id))
		var c Calc
		must(t, json.Unmarshal(row.Calc, &c))
		return row.Level, c
	}
	// No deliveries → the shop default rhythm (7) + safety (3) = 10 days.
	if l, c := level(auto); l != 100 || c.Daily != 10 || c.RhythmFrom != "default" {
		t.Fatalf("auto: level %d daily %v from %s, want 100 10 default", l, c.Daily, c.RhythmFrom)
	}
	if l, c := level(fixed); l != 50 || c.CalcLevel != 20 {
		t.Fatalf("fixed: level %d calc %d, want 50 (untouched) and 20", l, c.CalcLevel)
	}
	if l, c := level(bag); l != 10 || c.Daily != 1 {
		t.Fatalf("bag: level %d daily %v, want 10 from 1 bag a day sold loose", l, c.Daily)
	}
}
