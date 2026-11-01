package stock

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"karots-suite/internal/apperr"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

type giveBackFix struct {
	conn    *sqlx.DB
	svc     *Service
	product int64
	lot     int64
	other   int64 // 0 = none
	user    int64
}

// giveBackFixture is a product with a lot (expiring first) and maybe another.
func giveBackFixture(t *testing.T, lot, other, stockQty string) giveBackFix {
	t.Helper()
	conn := testDB(t)
	t.Cleanup(func() { conn.Close() })
	ctx := context.Background()
	f := giveBackFix{conn: conn, svc: NewService(conn)}
	must(t, conn.GetContext(ctx, &f.user, `SELECT id FROM users ORDER BY id LIMIT 1`))
	var cat int64
	must(t, conn.GetContext(ctx, &cat, `INSERT INTO categories (name) VALUES ($1) RETURNING id`, fmt.Sprintf("T give back %d", time.Now().UnixNano())))
	must(t, conn.GetContext(ctx, &f.product, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price) VALUES ($1, $2, 1, 40, 60) RETURNING id`,
		fmt.Sprintf("T give back %d", time.Now().UnixNano()), cat))
	_, err := conn.ExecContext(ctx, `INSERT INTO stock (product_id, quantity) VALUES ($1, $2) ON CONFLICT (product_id) DO UPDATE SET quantity = EXCLUDED.quantity`, f.product, stockQty)
	must(t, err)
	add := func(qty string, days, cost int) int64 {
		var id int64
		must(t, conn.GetContext(ctx, &id, `INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, expiry_date, source)
			VALUES ($1, $2, $2, $3, CURRENT_DATE + $4::int, 'adjust') RETURNING id`, f.product, qty, cost, days))
		return id
	}
	f.lot = add(lot, 10, 50)
	if other != "" {
		f.other = add(other, 200, 30)
	}
	return f
}

func (f giveBackFix) qty(t *testing.T, sql string, args ...any) decimal.Decimal {
	t.Helper()
	var v decimal.Decimal
	must(t, f.conn.Get(&v, sql, args...))
	return v
}

func (f giveBackFix) lotQty(t *testing.T, id int64) decimal.Decimal {
	return f.qty(t, `SELECT qty_remaining FROM stock_batches WHERE id = $1`, id)
}

func (f giveBackFix) total(t *testing.T) decimal.Decimal {
	return f.qty(t, `SELECT quantity FROM stock WHERE product_id = $1`, f.product)
}

func (f giveBackFix) moved(t *testing.T, mtype string) decimal.Decimal {
	return f.qty(t, `SELECT COALESCE(SUM(quantity), 0) FROM stock_movements WHERE product_id = $1 AND type = $2`, f.product, mtype)
}

func TestGiveBackSettlesTheCount(t *testing.T) {
	for _, c := range []struct {
		name              string
		lot, other, stock string
		counted           string
		wantOther         string
		wantTotal         string
		wantAdjust        string
		wantLossValue     string
		wantDamage        string
	}{
		{"more, the other lot covers it", "10", "5", "15", "12", "3", "3", "0", "600", "-12"},
		{"more, the other lot covers part", "10", "1", "11", "12", "0", "0", "1", "600", "-12"},
		{"more, no other lot", "10", "", "10", "12", "", "0", "2", "600", "-12"},
		{"less, onto the other lot", "10", "5", "15", "7", "8", "8", "0", "350", "-7"},
		{"less, no other lot", "10", "", "10", "7", "", "0", "-3", "350", "-7"},
		{"decimal on a loose product", "4", "1", "5", "2.5", "2.5", "2.5", "0", "125", "-2.5"},
		{"zero moves it all", "10", "5", "15", "0", "15", "15", "0", "0", "0"},
		{"zero, no other lot", "10", "", "10", "0", "", "0", "-10", "0", "0"},
		{"the total had drifted below the count", "10", "", "3", "10", "", "0", "7", "500", "-10"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := giveBackFixture(t, c.lot, c.other, c.stock)
			loss, err := f.svc.GiveBack(context.Background(), GiveBackInput{BatchID: f.lot, Counted: c.counted, SupplierName: "Hemas"}, f.user)
			must(t, err)
			if !f.lotQty(t, f.lot).IsZero() {
				t.Errorf("the lot holds %s, want 0", f.lotQty(t, f.lot))
			}
			if c.other != "" {
				if got := f.lotQty(t, f.other); !got.Equal(decimal.RequireFromString(c.wantOther)) {
					t.Errorf("the other lot holds %s, want %s", got, c.wantOther)
				}
			}
			if got := f.total(t); !got.Equal(decimal.RequireFromString(c.wantTotal)) {
				t.Errorf("total %s, want %s", got, c.wantTotal)
			}
			if got := f.moved(t, MoveAdjust); !got.Equal(decimal.RequireFromString(c.wantAdjust)) {
				t.Errorf("adjusted %s, want %s", got, c.wantAdjust)
			}
			if got := f.moved(t, MoveDamage); !got.Equal(decimal.RequireFromString(c.wantDamage)) {
				t.Errorf("damage %s, want %s", got, c.wantDamage)
			}
			if c.wantDamage == "0" {
				if loss != 0 {
					t.Errorf("a zero count booked loss %d", loss)
				}
				return
			}
			var m struct {
				Cost      decimal.Decimal `db:"cost"`
				Note      string          `db:"note"`
				GivenBack bool            `db:"given_back"`
				RefID     int64           `db:"reference_id"`
			}
			must(t, f.conn.Get(&m, `SELECT cost, note, given_back, reference_id FROM stock_movements WHERE id = $1`, loss))
			if !m.Cost.Equal(decimal.RequireFromString(c.wantLossValue)) || !m.GivenBack || m.Note != "given back to Hemas" || m.RefID != f.lot {
				t.Errorf("loss movement: %+v, want value %s", m, c.wantLossValue)
			}
		})
	}
}

// A move between lots leaves a trace in Movements though the total doesn't change.
func TestGiveBackShowsTheMoveInMovements(t *testing.T) {
	f := giveBackFixture(t, "10", "5", "15")
	_, err := f.svc.GiveBack(context.Background(), GiveBackInput{BatchID: f.lot, Counted: "7", SupplierName: "Hemas"}, f.user)
	must(t, err)
	var n int
	must(t, f.conn.Get(&n, `SELECT COUNT(*) FROM stock_movements WHERE product_id = $1 AND type = $2 AND quantity = 0 AND note LIKE '3 moved%'`, f.product, MoveAdjust))
	if n != 1 {
		t.Errorf("%d move notes, want 1", n)
	}
}

func TestGiveBackRefuses(t *testing.T) {
	ctx := context.Background()
	f := giveBackFixture(t, "10", "5", "15")
	for _, bad := range []string{"-1", "abc", ""} {
		_, err := f.svc.GiveBack(ctx, GiveBackInput{BatchID: f.lot, Counted: bad, SupplierName: "Hemas"}, f.user)
		var ae *apperr.AppError
		if !errors.As(err, &ae) || ae.Status != http.StatusUnprocessableEntity {
			t.Errorf("count %q: %v", bad, err)
		}
	}
	if !f.lotQty(t, f.lot).Equal(decimal.NewFromInt(10)) {
		t.Fatalf("a refused count moved stock")
	}
	_, err := f.svc.GiveBack(ctx, GiveBackInput{BatchID: f.lot, Counted: "10", SupplierName: "Hemas"}, f.user)
	must(t, err)
	before := f.total(t)
	_, err = f.svc.GiveBack(ctx, GiveBackInput{BatchID: f.lot, Counted: "10", SupplierName: "Hemas"}, f.user)
	var ae *apperr.AppError
	if !errors.As(err, &ae) || ae.Status != http.StatusConflict {
		t.Errorf("twice: %v", err)
	}
	if !f.total(t).Equal(before) {
		t.Errorf("the second press moved stock")
	}
	if _, err := f.svc.GiveBack(ctx, GiveBackInput{BatchID: 999999999, Counted: "1"}, f.user); err == nil {
		t.Errorf("an unknown lot was accepted")
	}
}

func TestMarkSupplierTold(t *testing.T) {
	f := giveBackFixture(t, "10", "", "10")
	must(t, f.svc.MarkSupplierTold(context.Background(), f.lot))
	var today bool
	must(t, f.conn.Get(&today, `SELECT supplier_told_on = CURRENT_DATE FROM stock_batches WHERE id = $1`, f.lot))
	if !today {
		t.Errorf("told date not today")
	}
	if err := f.svc.MarkSupplierTold(context.Background(), 999999999); err == nil {
		t.Errorf("an unknown lot was accepted")
	}
}
