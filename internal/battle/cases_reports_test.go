package battle

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"karots-suite/internal/datetime"
	"karots-suite/internal/features/stock"
)

// ZR-2: a closed Z never changes. A sale rung in session 1 comes back in
// session 2: the first Z keeps its refunds at 0, the second shows the refund.
// ZR-4: a cashier can't read another cashier's Z.
func TestZR2_ClosedZNeverChanges(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	r := s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "2"}}, cash("200"))
	var first int64
	s.must(t, s.db.Get(&first, `SELECT session_id FROM sales WHERE id = $1`, r.ID))
	s.close(t, s.till1)
	type z struct{ Returns, Net string }
	var before z
	s.must(t, s.till1.Do("GET", fmt.Sprintf("/api/v1/till/z/%d", first), nil, &before))

	s.open(t, s.till1)
	s.giveBack(t, s.till1, r.ID, s.sale(t, s.till1, r.ID).Lines[0].ID, "1")
	var second int64
	s.must(t, s.db.Get(&second, `SELECT id FROM cash_register WHERE closed_at IS NULL AND id <> $1 ORDER BY id DESC LIMIT 1`, first))
	var after, now z
	s.must(t, s.till1.Do("GET", fmt.Sprintf("/api/v1/till/z/%d", first), nil, &after))
	s.must(t, s.till1.Do("GET", fmt.Sprintf("/api/v1/till/z/%d", second), nil, &now))
	if after != before || before.Returns != "0.00" || before.Net != "200.00" {
		t.Errorf("ZR-2 the closed Z changed: %+v, then %+v", before, after)
	}
	if now.Returns != "100.00" || now.Net != "-100.00" {
		t.Errorf("ZR-2 the session that gave the refund shows %+v, want refunds 100.00, net -100.00", now)
	}
	if err := s.till2.Do("GET", fmt.Sprintf("/api/v1/till/z/%d", first), nil, nil); err == nil {
		t.Error("ZR-4 a cashier read another cashier's Z")
	}
}

// RPT-11: a sale at 23:59 shop time is that day, one at 00:01 the next; "to"
// takes its whole last day.
func TestRPT11_ShopDayEdges(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	a := s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "1"}}, cash("100"))
	b := s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "2"}}, cash("200"))
	day := time.Date(2099, 6, 10, 0, 0, 0, 0, datetime.Location)
	s.must(t, sqlxExec(t.Context(), s.db, fmt.Sprintf(`UPDATE sales SET created_at = '%s' WHERE id = %d`, day.Add(23*time.Hour+59*time.Minute).Format(time.RFC3339), a.ID)))
	s.must(t, sqlxExec(t.Context(), s.db, fmt.Sprintf(`UPDATE sales SET created_at = '%s' WHERE id = %d`, day.Add(24*time.Hour+time.Minute).Format(time.RFC3339), b.ID)))
	var f struct {
		SalesCount int
		Revenue    string
	}
	s.must(t, s.admin.Do("GET", "/api/v1/admin/finance?from=2099-06-10&to=2099-06-10", nil, &f))
	if f.SalesCount != 1 || f.Revenue != "100.00" {
		t.Errorf("RPT-11 10 June: %d sales, %s; want the 23:59 one only", f.SalesCount, f.Revenue)
	}
	s.must(t, s.admin.Do("GET", "/api/v1/admin/finance?from=2099-06-11&to=2099-06-11", nil, &f))
	if f.SalesCount != 1 || f.Revenue != "200.00" {
		t.Errorf("RPT-11 11 June: %d sales, %s; want the 00:01 one only", f.SalesCount, f.Revenue)
	}
	s.agree(t, "2099-06-10", "2099-06-11")
}

// RPT-13: a service that only cost money in the period still shows, its net
// the expense.
func TestRPT13_ServiceWithOnlyExpenses(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	var svc struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/products", map[string]any{"name": "Battle photocopy", "categoryId": s.cat, "unitId": s.unitPcs,
		"costPrice": "0", "sellingPrice": "20", "wholesalePrice": "20", "reorderAuto": false}, &svc))
	s.must(t, sqlxExec(t.Context(), s.db, fmt.Sprintf(`UPDATE products SET is_service = true WHERE id = %d`, svc.ID)))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/expenses", map[string]any{"category": "Toner", "amount": "450", "serviceId": svc.ID}, nil))
	var r struct {
		Rows []struct {
			ProductID     int64
			Expenses, Net string
		}
	}
	today := time.Now().In(datetime.Location).Format(time.DateOnly)
	s.must(t, s.admin.Do("GET", "/api/v1/admin/reports/service-profit?from="+today+"&to="+today, nil, &r))
	for _, row := range r.Rows {
		if row.ProductID == svc.ID {
			if row.Expenses != "450.00" || row.Net != "-450.00" {
				t.Errorf("RPT-13 %+v, want expenses 450.00, net -450.00", row)
			}
			return
		}
	}
	t.Errorf("RPT-13 a service with only expenses is missing: %+v", r.Rows)
}

// RPT-20: a disabled customer who owes is still on Customer dues.
func TestRPT20_DisabledDebtorStillOwes(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	id := s.newCustomer(t, "0712000020", "100000", "")
	_, err := s.onAccount(t, id, 3, "0", nil)
	s.must(t, err)
	s.must(t, s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/customers/%d", id), nil, nil))
	var d struct {
		Rows []struct {
			ID       int64
			Balance  string
			Disabled bool
		}
	}
	s.must(t, s.admin.Do("GET", "/api/v1/admin/reports/customer-dues", nil, &d))
	for _, r := range d.Rows {
		if r.ID == id {
			if r.Balance != "300.00" || !r.Disabled {
				t.Errorf("RPT-20 %+v, want 300.00 owed, marked disabled", r)
			}
			today := time.Now().In(datetime.Location).Format(time.DateOnly)
			s.agree(t, today, today)
			return
		}
	}
	t.Error("RPT-20 a disabled customer who owes is missing from dues")
}

// RPT-24: stock value is each lot at its own cost, and on hand no lot accounts
// for at the product's cost.
func TestRPT24_StockValueByLots(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Battle valued", s.unitPcs, 0, nil) // cost 10
	s.addLot(t, p, "3", "40", "")
	s.addLot(t, p, "2", "55", "")
	var v struct{ CostValue string }
	var cat struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/categories", map[string]string{"name": "Valued only"}, &cat))
	s.must(t, sqlxExec(t.Context(), s.db, fmt.Sprintf(`UPDATE products SET category_id = %d WHERE id = %d`, cat.ID, p)))
	s.must(t, s.admin.Do("GET", fmt.Sprintf("/api/v1/admin/reports/inventory?category=%d", cat.ID), nil, &v))
	if v.CostValue != "230.00" { // 3 × 40 + 2 × 55
		t.Errorf("RPT-24 cost value %s, want 230.00 (each lot at its cost)", v.CostValue)
	}
}

// RPT-35: a sheet download holds every row of the period, past the page the
// screen shows (50 lines).
func TestRPT35_SheetHasEveryRow(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	const n = 51
	items := make([]line, n)
	for i := range items {
		items[i] = line{"productId": s.prod["plain"], "quantity": "1"}
	}
	r := s.sell(t, s.till1, "0", items, cash("10000"))
	for _, l := range s.sale(t, s.till1, r.ID).Lines {
		s.giveBack(t, s.till1, r.ID, l.ID, "1")
	}
	today := time.Now().In(datetime.Location).Format(time.DateOnly)
	q := "from=" + today + "&to=" + today
	var page []struct{ SaleID int64 }
	s.must(t, s.admin.Do("GET", "/api/v1/admin/reports/returns/lines?"+q, nil, &page))
	var body []byte
	s.must(t, s.admin.Do("GET", "/api/v1/admin/reports/returns?format=csv&"+q, nil, &body))
	if rows := strings.Count(strings.TrimSpace(string(body)), "\n"); rows != n || len(page) != 50 {
		t.Errorf("RPT-35 the sheet has %d rows (want %d), the page %d (want 50)", rows, n, len(page))
	}
}

// RPT-9: opening stock (a product sheet's opening quantity) puts stock on the
// books — it is not a count, and never profit. A count that finds more still is.
func TestRPT9_OpeningStockIsNotProfit(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Battle opening", s.unitPcs, 0, nil)
	if _, err := stock.NewRepository(s.db).AddStock(t.Context(), stock.AddStockInput{ProductID: p, Qty: "40", Cost: "25",
		Source: "opening", Note: "CSV import opening stock"}, s.userID(t, "0770000001")); err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(datetime.Location).Format(time.DateOnly)
	var before, after struct{ StockCorrections string }
	s.must(t, s.admin.Do("GET", "/api/v1/admin/finance?from="+today+"&to="+today, nil, &before))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": p, "newQuantity": "42", "note": "count"}, nil))
	s.must(t, s.admin.Do("GET", "/api/v1/admin/finance?from="+today+"&to="+today, nil, &after))
	// The shop's fixtures came in by counts (−the worth found); the opening adds nothing; the count's
	// +2 come in at the product's cost, 10 (area 4) = −20.
	if d := dec(t, after.StockCorrections).Sub(dec(t, before.StockCorrections)); !d.Equal(dec(t, "-20")) {
		t.Errorf("RPT-9 the count of +2 moved stock corrections by %s, want -20", d)
	}
	var fixtures decimal.Decimal
	s.must(t, s.db.Get(&fixtures, `SELECT COALESCE(SUM(-cost),0) FROM stock_movements WHERE type = 'adjust' AND note = 'battle'`))
	if !dec(t, before.StockCorrections).Equal(fixtures) {
		t.Errorf("RPT-9 before the count, stock corrections %s, want %s (the fixtures' counts only — the opening stock is no profit)", before.StockCorrections, fixtures)
	}
	s.agreeToday(t)
}
