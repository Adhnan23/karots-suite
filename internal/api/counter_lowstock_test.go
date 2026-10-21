package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// The till's Low stock tab: behind its own permission, cost only with See cost.
func TestCounterLowStock(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	phone := fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000)
	ck := te.login(t, phone, "cashier")
	te.db.Exec(`UPDATE products SET reorder_level = 5 WHERE id = $1`, f.scarceID) //nolint:errcheck
	url := "/api/v1/till/low-stock?all=1"

	if rec := te.do(http.MethodGet, url, nil, ck); rec.Code != http.StatusForbidden {
		t.Fatalf("cashier without the tick: %d", rec.Code)
	}
	te.db.Exec(`UPDATE users SET can_see_low_stock = true WHERE phone = $1`, phone) //nolint:errcheck
	l := decode[ReorderList](t, te.do(http.MethodGet, url, nil, ck))
	var row *ReorderRow
	for i := range l.Items {
		if l.Items[i].ProductID == f.scarceID {
			row = &l.Items[i]
		}
	}
	if row == nil || row.Cost != "" || row.Suggested == "" {
		t.Fatalf("scarce row (want listed, cost hidden): %+v", row)
	}
	te.db.Exec(`UPDATE users SET can_see_cost = true WHERE phone = $1`, phone) //nolint:errcheck
	for _, r := range decode[ReorderList](t, te.do(http.MethodGet, url, nil, ck)).Items {
		if r.ProductID == f.scarceID && r.Cost == "" {
			t.Fatalf("cost hidden with See cost: %+v", r)
		}
	}
}
