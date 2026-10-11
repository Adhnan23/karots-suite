package combos

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	appdb "karots-suite/internal/db"
	"karots-suite/plugins/combos/migrations"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// testStore migrates combos onto the dev DB. Combos a test makes carry a
// per-test tag in their name and are removed afterwards.
func testStore(t *testing.T) (*Store, *sqlx.DB, string) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_combos"); err != nil {
		t.Fatal(err)
	}
	tag := fmt.Sprintf("zc%d", time.Now().UnixNano())
	t.Cleanup(func() {
		db.Exec(`DELETE FROM combo_sales WHERE name LIKE '%' || $1 || '%'`, tag) //nolint:errcheck
		db.Exec(`DELETE FROM combos WHERE name LIKE '%' || $1 || '%'`, tag)      //nolint:errcheck
	})
	return NewStore(db), db, tag
}

// product makes a product of the test's own; service / inactive as asked.
func product(t *testing.T, db *sqlx.DB, price, qty string, service, active bool) int64 {
	t.Helper()
	var pid int64
	if err := db.Get(&pid, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, wholesale_price, is_service, is_active)
		VALUES ($1, (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 0, $2, 0, $3, $4) RETURNING id`,
		fmt.Sprintf("Combo test %d", time.Now().UnixNano()), price, service, active); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO stock (product_id, quantity) VALUES ($1, $2)
		ON CONFLICT (product_id) DO UPDATE SET quantity = EXCLUDED.quantity`, pid, qty); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM combo_items WHERE product_id = $1`, pid) //nolint:errcheck
		db.Exec(`DELETE FROM stock WHERE product_id = $1`, pid)       //nolint:errcheck
		db.Exec(`DELETE FROM products WHERE id = $1`, pid)            //nolint:errcheck
	})
	return pid
}

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func input(name, typ, val string, items ...ItemInput) Input {
	return Input{Name: name, DiscountType: typ, DiscountValue: d(val), Items: items}
}

func find(t *testing.T, list []Combo, id int64) *Combo {
	t.Helper()
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}

func TestValidation(t *testing.T) {
	s, db, tag := testStore(t)
	ctx := context.Background()
	a := product(t, db, "100", "5", false, true)
	b := product(t, db, "50", "5", false, true)
	svc := product(t, db, "10", "0", true, true)
	off := product(t, db, "10", "5", false, false)
	bad := map[string]Input{
		"no name":      input(" ", "percent", "10", ItemInput{a, d("1")}, ItemInput{b, d("1")}),
		"one unit":     input("x"+tag, "percent", "10", ItemInput{a, d("1")}),
		"bad type":     input("x"+tag, "bogo", "10", ItemInput{a, d("2")}),
		"over 100%":    input("x"+tag, "percent", "101", ItemInput{a, d("2")}),
		"price zero":   input("x"+tag, "price", "0", ItemInput{a, d("2")}),
		"service":      input("x"+tag, "percent", "10", ItemInput{a, d("1")}, ItemInput{svc, d("1")}),
		"disabled":     input("x"+tag, "percent", "10", ItemInput{a, d("1")}, ItemInput{off, d("1")}),
		"twice":        input("x"+tag, "percent", "10", ItemInput{a, d("1")}, ItemInput{a, d("1")}),
		"ends first":   {Name: "x" + tag, DiscountType: "percent", DiscountValue: d("5"), StartsOn: "2026-05-02", EndsOn: "2026-05-01", Items: []ItemInput{{a, d("2")}}},
		"bad date":     {Name: "x" + tag, DiscountType: "percent", DiscountValue: d("5"), StartsOn: "2026-13-01", Items: []ItemInput{{a, d("2")}}},
		"negative qty": input("x"+tag, "percent", "10", ItemInput{a, d("-1")}, ItemInput{b, d("3")}),
	}
	for name, in := range bad {
		if _, err := s.Create(ctx, in); err == nil {
			t.Errorf("%s: want a validation error", name)
		}
	}
	id, err := s.Create(ctx, input("Pair "+tag, "price", "120", ItemInput{a, d("1")}, ItemInput{b, d("1")}))
	if err != nil {
		t.Fatal(err)
	}
	// One product ×2 is a combo too (multi-buy).
	if _, err := s.Create(ctx, input("Multi "+tag, "fixed", "100", ItemInput{a, d("4")})); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(ctx, id, input("Pair "+tag, "percent", "10", ItemInput{a, d("2")})); err != nil {
		t.Fatal(err)
	}
	c, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if c.DiscountType != "percent" || len(c.Items) != 1 || !c.Items[0].Qty.Equal(d("2")) {
		t.Fatalf("update not applied: %+v", c)
	}
}

func TestWindowHealthAndTill(t *testing.T) {
	s, db, tag := testStore(t)
	ctx := context.Background()
	a := product(t, db, "100", "5", false, true)
	b := product(t, db, "50", "0", false, true)
	today := time.Now().Format("2006-01-02")
	later := time.Now().AddDate(0, 0, 3).Format("2006-01-02")
	past := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	mk := func(name, starts, ends string) int64 {
		in := input(name+" "+tag, "percent", "10", ItemInput{a, d("1")}, ItemInput{b, d("1")})
		in.StartsOn, in.EndsOn = starts, ends
		id, err := s.Create(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	on := mk("On", today, today)
	scheduled := mk("Soon", later, "")
	ended := mk("Over", "", past)
	off := mk("Off", "", "")
	if err := s.SetActive(ctx, off, false); err != nil {
		t.Fatal(err)
	}
	all, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[int64]string{on: "on", scheduled: "scheduled", ended: "ended", off: "on"} {
		if c := find(t, all, id); c == nil || c.Window != want {
			t.Errorf("combo %d window = %+v, want %s", id, c, want)
		}
	}
	c := find(t, all, on)
	if len(c.Items) != 2 || !c.FullPrice.Equal(d("150")) {
		t.Fatalf("items / full price: %+v", c)
	}
	if c.Problems() != "" || len(c.Warnings()) != 1 { // b is out of stock
		t.Fatalf("health: problems %q warnings %v", c.Problems(), c.Warnings())
	}
	till, err := s.Till(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if find(t, till, on) == nil || find(t, till, scheduled) != nil || find(t, till, ended) != nil || find(t, till, off) != nil {
		t.Fatal("till must list only active combos in their window")
	}
	badges, err := s.BadgesFor(ctx, []int64{a, b})
	if err != nil || len(badges[a]) != 1 {
		t.Fatalf("badges: %v %v", badges, err)
	}
	// A member disabled later blocks the combo.
	db.MustExec(`UPDATE products SET is_active = false WHERE id = $1`, b)
	got, _ := s.Get(ctx, on)
	if got.Problems() == "" {
		t.Fatal("a disabled member must block the combo")
	}
}

func TestSoldAndReport(t *testing.T) {
	s, db, tag := testStore(t)
	ctx := context.Background()
	a := product(t, db, "100", "5", false, true)
	id, err := s.Create(ctx, input("Sold "+tag, "fixed", "20", ItemInput{a, d("2")}))
	if err != nil {
		t.Fatal(err)
	}
	sale := func(status string) int64 {
		var sid int64
		if err := db.Get(&sid, `INSERT INTO sales (receipt_no, cashier_id, total, status)
			VALUES ($1, (SELECT id FROM users LIMIT 1), 180, $2) RETURNING id`,
			fmt.Sprintf("ZC%d", time.Now().UnixNano()%1e12), status); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Exec(`DELETE FROM sales WHERE id = $1`, sid) }) //nolint:errcheck
		return sid
	}
	s1, s2 := sale("completed"), sale("void")
	if err := s.RecordSold(ctx, 999999999, []Sold{{ComboID: id, Sets: d("1"), Net: d("180"), Discount: d("20")}}); err == nil {
		t.Fatal("an unknown sale must be refused")
	}
	for _, sid := range []int64{s1, s1, s2} { // s1 twice: an upsert, not a double count
		if err := s.RecordSold(ctx, sid, []Sold{{ComboID: id, Sets: d("1"), Net: d("180"), Discount: d("20")}}); err != nil {
			t.Fatal(err)
		}
	}
	day := time.Now().Format("2006-01-02")
	rep, err := s.Report(ctx, day, day)
	if err != nil {
		t.Fatal(err)
	}
	var row *ReportRow
	for i := range rep.Rows {
		if rep.Rows[i].Name == "Sold "+tag {
			row = &rep.Rows[i]
		}
	}
	if row == nil || !row.Sets.Equal(d("1")) || !row.Discount.Equal(d("20")) || row.Bills != 1 {
		t.Fatalf("report row: %+v", row)
	}
	c, _ := s.Get(ctx, id)
	if !c.SoldSets.Equal(d("1")) || c.LastSold == nil {
		t.Fatalf("sold summary: %+v", c)
	}
	// Deleting a combo keeps its history under its name.
	if err := s.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	rep, _ = s.Report(ctx, day, day)
	found := false
	for _, r := range rep.Rows {
		found = found || r.Name == "Sold "+tag
	}
	if !found {
		t.Fatal("a deleted combo's sales must stay in the report")
	}
}
