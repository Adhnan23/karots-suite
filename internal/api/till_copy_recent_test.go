package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"karots-suite/internal/features/sales"
)

// recentFix signs a cashier in with an open drawer, and notes the newest sale,
// so the tests read only what they make in the shared database.
func recentFix(t *testing.T) (*testEnv, tillFix, *http.Cookie, int64) {
	te := newTestEnv(t)
	te.db.Exec(`UPDATE settings SET allow_untracked_cash = true`) //nolint:errcheck
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "cashier")
	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2}}, ck)
	t.Cleanup(func() { te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{}, ck) })
	var last int64
	if err := te.db.Get(&last, `SELECT COALESCE(max(id), 0) FROM sales`); err != nil {
		t.Fatal(err)
	}
	return te, f, ck, last
}

func recentSale(t *testing.T, te *testEnv, f tillFix, ck *http.Cookie) int64 {
	t.Helper()
	return decode[SaleResult](t, te.do(http.MethodPost, "/api/v1/till/sales", sale(f.plainID, "1", PaymentRequest{Method: "cash", Amount: "100"}), ck)).ID
}

func recentRead(t *testing.T, te *testEnv, after int64, since time.Time, limit int) *RecentSales {
	t.Helper()
	r, err := BuildRecentSales(context.Background(), te.db, sales.NewService(te.db), after, since, limit)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRecentSalesPages(t *testing.T) {
	te, f, ck, last := recentFix(t)
	a, b, c := recentSale(t, te, f, ck), recentSale(t, te, f, ck), recentSale(t, te, f, ck)

	p1 := recentRead(t, te, last, time.Time{}, 2)
	if len(p1.Sales) != 2 || !p1.More || p1.Sales[0].ID != a || p1.Sales[1].ID != b || p1.NextAfter != b {
		t.Fatalf("first page: %+v", p1)
	}
	if len(p1.Sales[0].Lines) != 1 || p1.Sales[0].Lines[0].RefundLeft == "" {
		t.Fatalf("a page's sale needs its lines, for Returns: %+v", p1.Sales[0])
	}
	p2 := recentRead(t, te, p1.NextAfter, time.Time{}, 2)
	if len(p2.Sales) != 1 || p2.More || p2.Sales[0].ID != c || p2.NextAfter != c {
		t.Fatalf("second page: %+v", p2)
	}
	empty := recentRead(t, te, p2.NextAfter, time.Time{}, 2)
	if len(empty.Sales) != 0 || empty.More || empty.NextAfter != c {
		t.Fatalf("nothing new must keep the cursor: %+v", empty)
	}
}

func TestRecentSalesWindow(t *testing.T) {
	te, f, ck, last := recentFix(t)
	old, recent := recentSale(t, te, f, ck), recentSale(t, te, f, ck)
	te.db.MustExec(`UPDATE sales SET created_at = now() - interval '31 days' WHERE id = $1`, old)
	te.db.MustExec(`UPDATE sales SET created_at = now() - interval '29 days' WHERE id = $1`, recent)

	p := recentRead(t, te, last, time.Time{}, 50)
	if len(p.Sales) != 1 || p.Sales[0].ID != recent {
		t.Fatalf("want only the sale inside 30 days: %+v", p.Sales)
	}
}

func TestRecentSalesChanged(t *testing.T) {
	te, f, ck, last := recentFix(t)
	id := recentSale(t, te, f, ck)
	first := recentRead(t, te, last, time.Time{}, 50)
	if len(first.Changed) != 0 || first.NextSince == "" {
		t.Fatalf("first read: %+v", first)
	}
	since, err := time.Parse(time.RFC3339Nano, first.NextSince)
	if err != nil {
		t.Fatal(err)
	}
	v := decode[SaleView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/sales/%d", id), nil, ck))
	rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/sales/%d/return", id), ReturnRequest{Lines: []ReturnLine{{SaleItemID: v.Lines[0].ID, Quantity: "1"}}}, ck)
	if rec.Code != http.StatusOK {
		t.Fatalf("return: %d %s", rec.Code, rec.Body)
	}
	p := recentRead(t, te, first.NextAfter, since, 50)
	if len(p.Sales) != 0 || len(p.Changed) != 1 || p.Changed[0].ID != id || p.Changed[0].Lines[0].ReturnedQty != "1" {
		t.Fatalf("want the returned sale in Changed: %+v", p)
	}
}

func TestRecentSalesResetAfterRestore(t *testing.T) {
	te, f, ck, _ := recentFix(t)
	id := recentSale(t, te, f, ck)
	p := recentRead(t, te, id+1000, time.Now(), 500)
	if !p.Reset || p.MaxID != id {
		t.Fatalf("a cursor beyond the last sale means a restore: %+v", p)
	}
	if len(p.Sales) == 0 || p.Sales[len(p.Sales)-1].ID > id {
		t.Fatalf("a reset starts again from the start: %+v", p.Sales)
	}
	if ok := recentRead(t, te, id, time.Time{}, 50); ok.Reset {
		t.Fatalf("a cursor at the last sale isn't a restore: %+v", ok)
	}
}
