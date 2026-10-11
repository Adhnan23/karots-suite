package clearance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"
	"karots-suite/internal/validator"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// testAPI mounts the plugin's routes the way the server does (minus auth) and
// returns a request helper.
func testAPI(t *testing.T, db *sqlx.DB) func(method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = middleware.ErrorHandler(false)
	mux := plugin.NewMux()
	p := &Plugin{}
	p.Setup(plugin.NewRegistry(plugin.Core{DB: db, Products: products.NewService(db), Audit: audit.NewService(db)}, mux, e))
	x := e.Group("/api/v1/x")
	mux.Mount(x, x, x)
	return func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1/x/clearance"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
}

func findRow(t *testing.T, do func(string, string, string) *httptest.ResponseRecorder, pid int64) (PageRow, PageDTO) {
	t.Helper()
	rec := do("GET", "", "")
	var page PageDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || rec.Code != 200 {
		t.Fatalf("GET %d %s", rec.Code, rec.Body)
	}
	for _, r := range page.Rows {
		if r.ProductID == pid {
			return r, page
		}
	}
	t.Fatalf("product %d not on the page", pid)
	return PageRow{}, page
}

func TestAPIApproveSnooze(t *testing.T) {
	_, db, pid := testStore(t, "5")
	do := testAPI(t, db)
	id := strconv.FormatInt(pid, 10)

	r, page := findRow(t, do, pid)
	// price 200, cost 100, default 20% / 5% margin → 20% is safe → 160.00
	if r.Status != "" || r.Percent != "20" || r.NewPrice != "160.00" || r.Reason != "stale" || r.ReasonLabel != "never sold" || page.Settings.StaleDays < 1 {
		t.Fatalf("candidate row %+v", r)
	}
	if rec := do("POST", "/"+id+"/approve", `{"percent":"120"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("approve 120%%: %d %s", rec.Code, rec.Body)
	}
	if rec := do("POST", "/"+id+"/approve", `{"percent":"15"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("approve: %d %s", rec.Code, rec.Body)
	}
	if r, _ = findRow(t, do, pid); r.Status != "live" || r.Percent != "15" || r.NewPrice != "170.00" {
		t.Fatalf("live row %+v", r)
	}
	if rec := do("POST", "/"+id+"/snooze", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("snooze: %d", rec.Code)
	}
	if r, _ = findRow(t, do, pid); r.Status != "snoozed" || r.SnoozedUntil == "" {
		t.Fatalf("snoozed row %+v", r)
	}
}

// An expiring row floors at the lot's cost and says when it expires.
func TestAPIExpiringFloor(t *testing.T) {
	_, db, pid := testStore(t, "4")
	mustExec(t, db, nil, `UPDATE products SET cost_price = 180 WHERE id = $1`, pid)
	mustExec(t, db, nil, `INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, expiry_date)
		VALUES ($1, 2, 2, 100, CURRENT_DATE + 3)`, pid)
	r, _ := findRow(t, testAPI(t, db), pid)
	// latest cost 180 would allow only 5.5%; the expiring lot cost 100 allows the full 20%.
	if r.Reason != "expiring" || r.ReasonLabel != "expires in 3 days" || r.FloorCost != "100.00" || r.Percent != "20" {
		t.Fatalf("expiring row %+v", r)
	}
}

func TestAPIDisableAndSettings(t *testing.T) {
	_, db, a := testStore(t, "0")
	_, _, b := testStore(t, "0")
	do := testAPI(t, db)
	body := `{"productIds":[` + strconv.FormatInt(a, 10) + `,` + strconv.FormatInt(b, 10) + `]}`
	rec := do("POST", "/disable", body)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"disabled":2`) {
		t.Fatalf("disable: %d %s", rec.Code, rec.Body)
	}
	var active int
	if err := db.Get(&active, `SELECT count(*) FROM products WHERE id IN ($1,$2) AND is_active`, a, b); err != nil || active != 0 {
		t.Fatalf("still active: %d (%v)", active, err)
	}
	if rec := do("POST", "/disable", `{"productIds":[]}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty disable: %d", rec.Code)
	}
	if rec := do("PUT", "/settings", `{"staleDays":0,"defaultPercent":"20","minMarginPercent":"5","expiryDays":14,"deadMonths":6}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("staleDays 0: %d %s", rec.Code, rec.Body)
	}
	var before PageDTO
	json.Unmarshal(do("GET", "", "").Body.Bytes(), &before) //nolint:errcheck
	defer do("PUT", "/settings", mustJSON(before.Settings))
	rec = do("PUT", "/settings", `{"staleDays":45,"defaultPercent":"25","minMarginPercent":"8","expiryDays":10,"deadMonths":4}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"staleDays":45`) {
		t.Fatalf("settings: %d %s", rec.Code, rec.Body)
	}
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
