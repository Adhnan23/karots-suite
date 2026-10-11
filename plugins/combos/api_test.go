package combos

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"
	"karots-suite/internal/validator"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// testAPI mounts the routes the way the server does, with a stand-in for the
// admin guard (?cashier=1 is refused on admin routes).
func testAPI(t *testing.T, db *sqlx.DB) func(method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = middleware.ErrorHandler(false)
	mux := plugin.NewMux()
	p := &Plugin{}
	p.Setup(plugin.NewRegistry(plugin.Core{DB: db, Products: products.NewService(db), Audit: audit.NewService(db)}, mux, e))
	x := e.Group("/api/v1/x")
	admin := e.Group("/api/v1/x", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.QueryParam("cashier") == "1" {
				return c.NoContent(403)
			}
			return next(c)
		}
	})
	mux.Mount(x, x, admin)
	return func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1/x/combos"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("%d %s: %v", rec.Code, rec.Body, err)
	}
	return v
}

func TestAPI(t *testing.T) {
	_, db, tag := testStore(t)
	do := testAPI(t, db)
	a := product(t, db, "380", "5", false, true)
	b := product(t, db, "220", "5", false, true)

	for _, r := range []struct{ m, p string }{{"GET", "?cashier=1"}, {"POST", "?cashier=1"}, {"GET", "/report?cashier=1"}, {"DELETE", "/1?cashier=1"}} {
		if rec := do(r.m, r.p, "{}"); rec.Code != 403 {
			t.Errorf("%s %s as cashier = %d, want 403", r.m, r.p, rec.Code)
		}
	}
	if rec := do("POST", "", `{"name":"x","discountType":"percent","discountValue":"10","items":[{"productId":1,"qty":"1"}]}`); rec.Code != 422 {
		t.Fatalf("one unit = %d %s, want 422", rec.Code, rec.Body)
	}
	body := fmt.Sprintf(`{"name":"Pair %s","discountType":"price","discountValue":"560","items":[{"productId":%d,"qty":"1"},{"productId":%d,"qty":"1"}]}`, tag, a, b)
	rec := do("POST", "", body)
	if rec.Code != 200 {
		t.Fatalf("create = %d %s", rec.Code, rec.Body)
	}
	c := decode[ComboDTO](t, rec)
	if c.FullPrice != "600.00" || len(c.Members) != 2 || c.Window != "on" || c.Problem != "" {
		t.Fatalf("created: %+v", c)
	}
	if n := countTill(t, do, c.ID); n != 1 {
		t.Fatal("an on combo must be on the till list")
	}
	if rec := do("POST", fmt.Sprintf("/%d/active", c.ID), `{"active":false}`); rec.Code != 204 {
		t.Fatalf("disable = %d", rec.Code)
	}
	if n := countTill(t, do, c.ID); n != 0 {
		t.Fatal("a disabled combo must leave the till list")
	}
	if rec := do("POST", "/sold", `{"saleId":999999999,"combos":[]}`); rec.Code != 404 {
		t.Fatalf("unknown sale = %d, want 404", rec.Code)
	}
	var sid int64
	if err := db.Get(&sid, `INSERT INTO sales (receipt_no, cashier_id, total) VALUES ($1, (SELECT id FROM users LIMIT 1), 1120) RETURNING id`,
		fmt.Sprintf("ZA%d", time.Now().UnixNano()%1e12)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM sales WHERE id = $1`, sid) }) //nolint:errcheck
	if rec := do("POST", "/sold", fmt.Sprintf(`{"saleId":%d,"combos":[{"comboId":%d,"sets":"2","net":"1120","discount":"80"}]}`, sid, c.ID)); rec.Code != 204 {
		t.Fatalf("sold = %d %s", rec.Code, rec.Body)
	}
	day := time.Now().Format("2006-01-02")
	rep := decode[ReportDTO](t, do("GET", "/report?preset=today", ""))
	found := false
	for _, r := range rep.Rows {
		found = found || (r.Name == "Pair "+tag && r.Sets == "2" && r.Discount == "80.00" && r.Bills == 1)
	}
	if !found {
		t.Fatalf("report: %+v", rep)
	}
	if rec := do("GET", "/report?from=x&to="+day, ""); rec.Code != 422 {
		t.Fatalf("bad date = %d", rec.Code)
	}
	if rec := do("DELETE", fmt.Sprintf("/%d", c.ID), ""); rec.Code != 204 {
		t.Fatalf("delete = %d", rec.Code)
	}
	var n int
	db.Get(&n, `SELECT count(*) FROM audit_log WHERE entity = 'combo' AND entity_id = $1`, fmt.Sprint(c.ID)) //nolint:errcheck
	if n != 3 {
		t.Fatalf("audit rows = %d, want 3 (create, disable, delete)", n)
	}
}

func countTill(t *testing.T, do func(m, p, b string) *httptest.ResponseRecorder, id int64) int {
	n := 0
	for _, c := range decode[[]ComboDTO](t, do("GET", "/till", "")) {
		if c.ID == id {
			n++
		}
	}
	return n
}
