package requests

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
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

// testAPI mounts the routes the way the server does, with a stand-in for the
// admin guard (?cashier=1 is refused on admin routes), and returns a helper.
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
		req := httptest.NewRequest(method, "/api/v1/x/requests"+path, strings.NewReader(body))
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

func TestAPIValidation(t *testing.T) {
	s, db, tag := testStore(t)
	do := testAPI(t, db)
	if rec := do("POST", "", `{"note":"  "}`); rec.Code != 422 {
		t.Fatalf("empty note: %d", rec.Code)
	}
	id := add(t, s, "Kettle "+tag, "", "")
	if rec := do("POST", fmt.Sprintf("/%d/decline", id), `{"reason":""}`); rec.Code != 422 {
		t.Fatalf("decline without a reason: %d", rec.Code)
	}
	if rec := do("POST", fmt.Sprintf("/%d/product", id), `{"name":"Kettle 1.7L","unitId":1}`); rec.Code != 422 {
		t.Fatalf("product without a category: %d", rec.Code)
	}
	if rec := do("POST", fmt.Sprintf("/%d/link", id), `{"productId":999999999}`); rec.Code != 404 {
		t.Fatalf("link to a missing product: %d", rec.Code)
	}
	if rec := do("GET", "/similar?q=kettle&cashier=1", ""); rec.Code != 200 {
		t.Fatalf("a cashier may search: %d", rec.Code)
	}
	if rec := do("GET", "?cashier=1", ""); rec.Code != 403 {
		t.Fatalf("a cashier can't open the admin list: %d", rec.Code)
	}
}

func TestAPIMakeProduct(t *testing.T) {
	s, db, tag := testStore(t)
	do := testAPI(t, db)
	id := add(t, s, "Kettle "+tag, "", "0771234567")
	var cat, unit, sup int64
	noErr(t, db.Get(&cat, `SELECT id FROM categories LIMIT 1`))
	noErr(t, db.Get(&unit, `SELECT id FROM units LIMIT 1`))
	noErr(t, db.Get(&sup, `SELECT id FROM suppliers LIMIT 1`))
	name := "Kettle 1.7L " + tag
	rec := do("POST", fmt.Sprintf("/%d/product", id),
		fmt.Sprintf(`{"name":%q,"categoryId":%d,"unitId":%d,"sellingPrice":"4500","costPrice":"","supplierId":%d}`, name, cat, unit, sup))
	if rec.Code != 200 {
		t.Fatalf("make product: %d %s", rec.Code, rec.Body)
	}
	var p struct {
		ID     int64  `db:"id"`
		Active bool   `db:"is_active"`
		Price  string `db:"selling_price"`
		Cost   string `db:"cost_price"`
		Sup    *int64 `db:"preferred_supplier_id"`
	}
	noErr(t, db.Get(&p, `SELECT id, is_active, selling_price::text, cost_price::text, preferred_supplier_id FROM products WHERE name = $1`, name))
	t.Cleanup(func() { db.Exec(`DELETE FROM products WHERE id = $1`, p.ID) }) //nolint:errcheck
	if !p.Active || !strings.HasPrefix(p.Price, "4500") || !strings.HasPrefix(p.Cost, "0") || p.Sup == nil || *p.Sup != sup {
		t.Fatalf("product: %+v", p)
	}
	r, ok := get(t, s, "open", tag, id)
	if !ok || r.Status != "converted" || r.ProductID == nil || *r.ProductID != p.ID {
		t.Fatalf("request converted to it: %+v", r)
	}
	if rec := do("POST", fmt.Sprintf("/%d/product", id), fmt.Sprintf(`{"name":"again","categoryId":%d,"unitId":%d}`, cat, unit)); rec.Code != 409 {
		t.Fatalf("making a product twice: %d", rec.Code)
	}
}

func TestAPIListAndAsks(t *testing.T) {
	s, db, tag := testStore(t)
	do := testAPI(t, db)
	rec := do("POST", "", fmt.Sprintf(`{"note":"Torch %s","name":"Sunil","phone":"0779998887"}`, tag))
	if rec.Code != 200 {
		t.Fatalf("add: %d %s", rec.Code, rec.Body)
	}
	id := decode[RequestRow](t, rec).ID
	if rec := do("POST", fmt.Sprintf("/%d/asks", id), `{"name":"Mala","phone":""}`); rec.Code != 204 {
		t.Fatalf("ask again: %d %s", rec.Code, rec.Body)
	}
	list := decode[ListDTO](t, do("GET", "?tab=open&q="+tag, ""))
	if len(list.Rows) != 1 || list.Rows[0].AskCount != 2 || list.Rows[0].Asks[0].Name != "Sunil" || list.Total != 1 {
		t.Fatalf("list: %+v", list)
	}
	pid := product(t, db, "4")
	if rec := do("POST", fmt.Sprintf("/%d/link", id), fmt.Sprintf(`{"productId":%d}`, pid)); rec.Code != 204 {
		t.Fatalf("link: %d %s", rec.Code, rec.Body)
	}
	call := decode[ListDTO](t, do("GET", "?tab=call&q="+tag, ""))
	if len(call.Rows) != 1 || call.Counts.Call < 1 || call.Rows[0].Stage != "call" {
		t.Fatalf("to call: %+v", call)
	}
	if rec := do("POST", fmt.Sprintf("/%d/asks/0/called", id), ""); rec.Code != 204 {
		t.Fatalf("called: %d", rec.Code)
	}
	if rec := do("POST", fmt.Sprintf("/%d/asks", id), `{}`); rec.Code != 409 {
		t.Fatalf("asking again once done: %d", rec.Code)
	}
	_ = s
}
