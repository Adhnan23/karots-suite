package gen

import (
	"context"
	"testing"
	"time"

	"karots-suite/internal/ledgercheck/ltest"
)

// The year is built "as of now" and moved into the past, so a unit's warranty_until is already a past date while
// the server judges cover by today (WAR-1). A unit covered on the day being built but not today must never be
// claimed — the server refuses it, and `make demo` failed on exactly that (2026-11-22, four of five shops).
func TestWarrantyClaimPicksOnlyWhatTheServerStillCovers(t *testing.T) {
	ctx := context.Background()
	d := ltest.NewDB(t)
	var cat, unit, prod int64
	if err := d.Get(&cat, `INSERT INTO categories (name) VALUES ('Phones') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	if err := d.Get(&unit, `SELECT id FROM units ORDER BY id LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	if err := d.Get(&prod, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price) VALUES ('Phone', $1, $2, 900, 1000) RETURNING id`, cat, unit); err != nil {
		t.Fatal(err)
	}
	today := localDay(time.Now())
	add := func(soldMonthsAgo int) int64 {
		sold := today.AddDate(0, -soldMonthsAgo, 0)
		var id int64
		if err := d.Get(&id, `INSERT INTO warranty_units (product_id, sold_at, warranty_months, warranty_until, created_at)
			VALUES ($1, $2, 12, $3, $2) RETURNING id`, prod, sold, sold.AddDate(1, 0, 0).Format("2006-01-02")); err != nil {
			t.Fatal(err)
		}
		return id
	}
	covered := add(3)              // until 9 months from now
	ended := add(13)               // until a month ago: the server refuses it today
	day := today.AddDate(0, -2, 0) // the day being built: both were covered then
	if ended < covered {
		t.Fatal("the ended unit must sort first (newest id), or the test proves nothing")
	}
	u, err := claimableUnit(ctx, d, time.Now(), day, 0)
	if err != nil {
		t.Fatalf("no unit picked: %v", err)
	}
	if u.ID != covered {
		t.Fatalf("picked unit %d, want %d: unit %d's warranty ended before today, so the server refuses the claim", u.ID, covered, ended)
	}
}
