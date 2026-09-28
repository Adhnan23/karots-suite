package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type tillFix struct {
	plainID, lotID, expiringID, scarceID, groupID int64
	plainBarcode                                  string
}

// seedTill makes a category, four products with stock + lots, and a group, all
// with fresh names/barcodes. Rows are left behind on purpose: sales reference
// them and the dev DB is disposable (make test-fresh).
//   plain     Rs 100, 10 in one lot
//   lot       Rs 100, lots of 5 @ product price and 5 @ 120 (price disagreement)
//   expiring  Rs 80, 3 expired yesterday (FEFO-first) + 3 fresh
//   scarce    Rs 250, 1 on hand
func (te *testEnv) seedTill(t *testing.T) tillFix {
	t.Helper()
	ctx := context.Background()
	sfx := time.Now().UnixNano()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var cat int64
	must(te.db.GetContext(ctx, &cat, `INSERT INTO categories (name) VALUES ($1) RETURNING id`, fmt.Sprintf("T cat %d", sfx)))
	product := func(name, barcode, price, qty string) int64 {
		var id int64
		must(te.db.GetContext(ctx, &id, `INSERT INTO products (name, barcode, category_id, unit_id, cost_price, selling_price)
			VALUES ($1, NULLIF($2, ''), $3, 1, 50, $4) RETURNING id`, fmt.Sprintf("%s %d", name, sfx), barcode, cat, price))
		_, err := te.db.ExecContext(ctx, `INSERT INTO stock (product_id, quantity) VALUES ($1, $2)
			ON CONFLICT (product_id) DO UPDATE SET quantity = EXCLUDED.quantity`, id, qty)
		must(err)
		return id
	}
	lot := func(pid int64, qty string, price *string, expiry *time.Time) {
		// selling_price is NOT NULL (0 = "no own price, inherit the product's" —
		// see stock.PriceOptions' own_price check), so a nil price means 0, not NULL.
		p := "0"
		if price != nil {
			p = *price
		}
		_, err := te.db.ExecContext(ctx, `INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, selling_price, expiry_date)
			VALUES ($1, $2, $2, 50, $3, $4)`, pid, qty, p, expiry)
		must(err)
	}
	var f tillFix
	f.plainBarcode = fmt.Sprintf("88%011d", sfx%100_000_000_000)
	f.plainID = product("T plain", f.plainBarcode, "100", "10")
	lot(f.plainID, "10", nil, nil)
	f.lotID = product("T lot", "", "100", "10")
	p120 := "120"
	lot(f.lotID, "5", nil, nil)
	lot(f.lotID, "5", &p120, nil)
	yesterday, later := time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 6, 0)
	f.expiringID = product("T expiring", "", "80", "6")
	lot(f.expiringID, "3", nil, &yesterday)
	lot(f.expiringID, "3", nil, &later)
	f.scarceID = product("T scarce", "", "250", "1")
	lot(f.scarceID, "1", nil, nil)
	must(te.db.GetContext(ctx, &f.groupID, `INSERT INTO product_groups (name) VALUES ($1) RETURNING id`, fmt.Sprintf("T group %d", sfx)))
	_, err := te.db.ExecContext(ctx, `INSERT INTO product_group_items (group_id, product_id) VALUES ($1, $2)`, f.groupID, f.plainID)
	must(err)
	return f
}

// login seeds a user (PIN 1234) and returns their session cookie.
func (te *testEnv) login(t *testing.T, phone, role string) *http.Cookie {
	t.Helper()
	te.seedUser(t, phone, role)
	return sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: phone, Pin: "1234"}, nil))
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %d %s: %v", rec.Code, rec.Body, err)
	}
	return v
}
