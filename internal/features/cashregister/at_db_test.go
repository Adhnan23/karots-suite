package cashregister

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/sales"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// atFixture: a throwaway cashier (no sessions yet), a priced product with
// stock, untracked cash allowed; everything removed afterwards.
func atFixture(t *testing.T) (conn *sqlx.DB, userID, productID int64) {
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
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var orig bool
	must(conn.GetContext(ctx, &orig, `SELECT allow_untracked_cash FROM settings WHERE id = 1`))
	conn.MustExec(`UPDATE settings SET allow_untracked_cash = true WHERE id = 1`)
	must(conn.GetContext(ctx, &userID, `INSERT INTO users (name, phone, role, pin_hash) VALUES ('TEST at', $1, 'cashier', 'x') RETURNING id`,
		fmt.Sprintf("08%08d", time.Now().UnixNano()%100_000_000)))
	var cat, unit int64
	must(conn.GetContext(ctx, &cat, `SELECT id FROM categories LIMIT 1`))
	must(conn.GetContext(ctx, &unit, `SELECT id FROM units LIMIT 1`))
	must(conn.GetContext(ctx, &productID, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price) VALUES ('TEST at', $1, $2, 10, 100) RETURNING id`, cat, unit))
	conn.MustExec(`UPDATE stock SET quantity = 50 WHERE product_id = $1`, productID)
	t.Cleanup(func() {
		conn.Exec(`DELETE FROM sales WHERE cashier_id = $1`, userID)                                                                   //nolint:errcheck
		conn.Exec(`DELETE FROM stock_movements WHERE product_id = $1`, productID)                                                      //nolint:errcheck
		conn.Exec(`DELETE FROM products WHERE id = $1`, productID)                                                                     //nolint:errcheck
		conn.Exec(`DELETE FROM cash_movements WHERE user_id = $1`, userID)                                                             //nolint:errcheck
		conn.Exec(`DELETE FROM locker_ledger WHERE counter_till_session IN (SELECT id FROM cash_register WHERE user_id = $1)`, userID) //nolint:errcheck
		conn.Exec(`DELETE FROM cash_register WHERE user_id = $1`, userID)                                                              //nolint:errcheck
		conn.Exec(`DELETE FROM users WHERE id = $1`, userID)                                                                           //nolint:errcheck
		conn.Exec(`UPDATE settings SET allow_untracked_cash = $1 WHERE id = 1`, orig)                                                  //nolint:errcheck
		conn.Close()
	})
	return conn, userID, productID
}

func cashSale(t *testing.T, s *sales.Service, pid, uid int64, at time.Time) {
	t.Helper()
	if _, err := s.Create(context.Background(), sales.CreateInput{SaleType: "retail", At: &at,
		Items:    []sales.ItemInput{{ProductID: pid, Quantity: "1"}},
		Payments: []sales.PaymentInput{{Method: "cash", Amount: "100"}}}, uid); err != nil {
		t.Fatal(err)
	}
}

func TestOpenAtPast(t *testing.T) {
	conn, uid, pid := atFixture(t)
	ctx := context.Background()
	ss := sales.NewService(conn)
	svc := NewService(conn, ss)
	at := time.Now().Add(-time.Hour).Truncate(time.Second)
	sess, err := svc.Open(ctx, uid, OpenInput{OpeningCash: "500", At: &at})
	if err != nil {
		t.Fatal(err)
	}
	if !sess.OpenedAt.Equal(at) {
		t.Fatalf("opened %v, want %v", sess.OpenedAt, at)
	}
	cashSale(t, ss, pid, uid, time.Now().Add(-30*time.Minute))
	sum, err := svc.Summary(ctx, uid)
	if err != nil || !sum.CashSales.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("cash sales %v (%v)", sum.CashSales, err)
	}
}

func TestOpenAtBeforeLastClose(t *testing.T) {
	conn, uid, _ := atFixture(t)
	ctx := context.Background()
	svc := NewService(conn, sales.NewService(conn))
	if _, err := svc.Open(ctx, uid, OpenInput{OpeningCash: "0"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Close(ctx, uid, CloseInput{ClosingCash: "0"}); err != nil {
		t.Fatal(err)
	}
	before := time.Now().Add(-time.Hour)
	if _, err := svc.Open(ctx, uid, OpenInput{OpeningCash: "0", At: &before}); err == nil {
		t.Fatal("an open before the last close must be refused")
	}
	future := time.Now().Add(time.Hour)
	if _, err := svc.Open(ctx, uid, OpenInput{OpeningCash: "0", At: &future}); err == nil {
		t.Fatal("an open in the future must be refused")
	}
}

func TestCloseAt(t *testing.T) {
	conn, uid, pid := atFixture(t)
	ctx := context.Background()
	ss := sales.NewService(conn)
	svc := NewService(conn, ss)
	opened := time.Now().Add(-time.Hour)
	if _, err := svc.Open(ctx, uid, OpenInput{OpeningCash: "500", At: &opened}); err != nil {
		t.Fatal(err)
	}
	cashSale(t, ss, pid, uid, time.Now().Add(-20*time.Minute))
	early := opened.Add(-time.Minute)
	if _, err := svc.Close(ctx, uid, CloseInput{ClosingCash: "600", At: &early}); err == nil {
		t.Fatal("a close before the open must be refused")
	}
	at := time.Now().Add(-5 * time.Minute).Truncate(time.Second)
	res, err := svc.Close(ctx, uid, CloseInput{ClosingCash: "600", At: &at})
	if err != nil {
		t.Fatal(err)
	}
	if res.Session.ClosedAt == nil || !res.Session.ClosedAt.Equal(at) || !res.ExpectedCash.Equal(decimal.NewFromInt(600)) {
		t.Fatalf("closed %v expected %s", res.Session.ClosedAt, res.ExpectedCash)
	}
}

func TestRemoteNoKick(t *testing.T) {
	conn, uid, _ := atFixture(t)
	ctx := context.Background()
	kicks := 0
	svc := NewService(conn, sales.NewService(conn)).WithDrawerKick(func(context.Context) { kicks++ })
	if _, err := svc.Open(ctx, uid, OpenInput{OpeningCash: "100", Remote: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Close(ctx, uid, CloseInput{ClosingCash: "100", Remote: true}); err != nil {
		t.Fatal(err)
	}
	if kicks != 0 {
		t.Fatalf("kicks %d, want 0", kicks)
	}
}

func TestReviewNoteStored(t *testing.T) {
	conn, uid, _ := atFixture(t)
	svc := NewService(conn, sales.NewService(conn))
	sess, err := svc.Open(context.Background(), uid, OpenInput{OpeningCash: "0", ReviewNote: "opened for late sales"})
	if err != nil {
		t.Fatal(err)
	}
	if sess.ReviewNote == nil || *sess.ReviewNote != "opened for late sales" {
		t.Fatalf("note %v", sess.ReviewNote)
	}
}
