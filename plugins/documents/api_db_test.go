package documents

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"
	"karots-suite/internal/validator"
	"karots-suite/plugins/documents/migrations"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func testDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_documents"); err != nil {
		t.Fatal(err)
	}
	return db
}

// testAPI mounts the routes the way the server does; ?cashier=1 is refused on
// admin routes (a stand-in for the admin guard).
func testAPI(t *testing.T, db *sqlx.DB) func(method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = middleware.ErrorHandler(false)
	mux := plugin.NewMux()
	p := &Plugin{}
	p.Setup(plugin.NewRegistry(plugin.Core{DB: db, Products: products.NewService(db), Audit: audit.NewService(db), Stock: stock.NewService(db)}, mux, e))
	var uid int64
	if err := db.Get(&uid, `SELECT id FROM users ORDER BY id LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("user_id", uid); return next(c) }
	})
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
		req := httptest.NewRequest(method, "/api/v1/x/documents"+path, strings.NewReader(body))
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

func TestPrintAndCopyFlow(t *testing.T) {
	db := testDB(t)
	do := testAPI(t, db)
	tag := fmt.Sprintf("zd%d", time.Now().UnixNano())
	var cat, cashier, since, paper int64
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.Get(&cat, `SELECT id FROM categories LIMIT 1`))
	must(db.Get(&cashier, `SELECT id FROM users LIMIT 1`))
	must(db.Get(&since, `SELECT COALESCE(MAX(id),0) FROM sales`))
	must(db.Get(&paper, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ($1, $2, (SELECT id FROM units LIMIT 1), 2, 0) RETURNING id`, "A4 paper "+tag, cat))
	_, err := db.Exec(`INSERT INTO stock (product_id, quantity) VALUES ($1, 500)
		ON CONFLICT (product_id) DO UPDATE SET quantity = EXCLUDED.quantity`, paper)
	must(err)
	_, err = db.Exec(`INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, selling_price, source)
		VALUES ($1, 500, 500, 2, 0, 'opening')`, paper)
	must(err)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM doc_job WHERE description LIKE '%' || $1 || '%'`, tag)                                                //nolint:errcheck
		db.Exec(`DELETE FROM sales WHERE id > $1`, since)                                                                          //nolint:errcheck
		db.Exec(`DELETE FROM doc_service WHERE name LIKE '%' || $1 || '%'`, tag)                                                   //nolint:errcheck
		db.Exec(`DELETE FROM stock_movements WHERE product_id IN (SELECT id FROM products WHERE name LIKE '%' || $1 || '%')`, tag) //nolint:errcheck
		db.Exec(`DELETE FROM stock_batches WHERE product_id = $1`, paper)                                                          //nolint:errcheck
		db.Exec(`DELETE FROM stock WHERE product_id = $1`, paper)                                                                  //nolint:errcheck
		db.Exec(`DELETE FROM products WHERE name LIKE '%' || $1 || '%'`, tag)                                                      //nolint:errcheck
	})

	// Admin routes refuse a cashier.
	for _, r := range []struct{ m, p string }{{"GET", "/services?cashier=1"}, {"POST", "/services?cashier=1"}, {"PUT", "/services/1/grid?cashier=1"}, {"GET", "/report?cashier=1"}} {
		if rec := do(r.m, r.p, "{}"); rec.Code != 403 {
			t.Errorf("%s %s as cashier = %d, want 403", r.m, r.p, rec.Code)
		}
	}

	// A new print service = a core service + its link.
	rec := do("POST", "/services", fmt.Sprintf(`{"name":"Photocopy %s","categoryId":%d,"kind":"metered","category":"copy"}`, tag, cat))
	if rec.Code != 200 {
		t.Fatalf("create = %d %s", rec.Code, rec.Body)
	}
	made := decode[map[string]int64](t, rec)
	sid, pid := made["id"], made["productId"]
	var isService bool
	must(db.Get(&isService, `SELECT is_service FROM products WHERE id = $1`, pid))
	if !isService {
		t.Fatal("the print service's product is not a core service")
	}
	if rec := do("POST", "/services", fmt.Sprintf(`{"productId":%d,"kind":"metered","category":"copy"}`, pid)); rec.Code != 409 {
		t.Errorf("linking twice = %d, want 409", rec.Code)
	}

	// Grid: A4 B&W 8 (from 100: 6), 2-side 12; paper 1 per sheet.
	grid := fmt.Sprintf(`{"rows":[{"size":"A4","tiers":[{"minQty":1,"cells":["8","12","",""]},{"minQty":100,"cells":["6","","",""]}],
		"paper":[{"productId":%d,"perSheet":"1"}]}]}`, paper)
	rec = do("PUT", fmt.Sprintf("/services/%d/grid", sid), grid)
	if rec.Code != 200 {
		t.Fatalf("grid = %d %s", rec.Code, rec.Body)
	}
	g := decode[GridDTO](t, rec)
	if len(g.Rows) != 1 || len(g.Rows[0].Tiers) != 2 || g.Rows[0].Tiers[0].Cells != [4]string{"8.00", "12.00", "", ""} || len(g.Rows[0].Paper) != 1 {
		t.Fatalf("grid back: %+v", g)
	}
	if rec := do("PUT", fmt.Sprintf("/services/%d/grid", sid), `{"rows":[{"size":"A4","tiers":[]},{"size":"a4","tiers":[]}]}`); rec.Code != 422 {
		t.Errorf("a size twice = %d, want 422", rec.Code)
	}

	// Quote: 21 double-sided = 11 sheets at 12 each; 120 single = band 100 at 6.
	q := decode[QuoteDTO](t, do("GET", fmt.Sprintf("/quote?service=%d&size=A4&double=1&copies=21", sid), ""))
	if q.UnitPrice != "12.00" || q.Total != "252.00" || q.Sheets != 11 || len(q.Components) != 1 || q.Components[0].Quantity != "11" {
		t.Fatalf("quote: %+v", q)
	}
	q = decode[QuoteDTO](t, do("GET", fmt.Sprintf("/quote?service=%d&size=A4&copies=120", sid), ""))
	if q.UnitPrice != "6.00" || q.MinQty != 100 {
		t.Fatalf("bulk quote: %+v", q)
	}
	if rec := do("GET", fmt.Sprintf("/quote?service=%d&size=A4&color=1&copies=1", sid), ""); rec.Code != 422 {
		t.Errorf("colour (not offered) = %d, want 422", rec.Code)
	}

	// Sell the 21 double-sided through core, then record the job after the sale.
	q = decode[QuoteDTO](t, do("GET", fmt.Sprintf("/quote?service=%d&size=A4&double=1&copies=21", sid), ""))
	d, err := sales.NewService(db).Create(t.Context(), sales.CreateInput{SaleType: "retail",
		Items: []sales.ItemInput{{ProductID: pid, Quantity: "21", PriceOverride: q.UnitPrice, Description: q.Description,
			Components: []sales.ServiceComponent{{ProductID: paper, Quantity: q.Components[0].Quantity}}}},
		Payments: []sales.PaymentInput{{Method: "cash", Amount: "252"}}}, cashier)
	must(err)
	job := fmt.Sprintf(`{"saleId":%d,"jobs":[{"serviceId":%d,"description":%q,"size":"A4","color":false,"double":true,"sheets":11}]}`, d.Sale.ID, sid, q.Description)
	if rec := do("POST", "/jobs", job); rec.Code != 204 {
		t.Fatalf("jobs = %d %s", rec.Code, rec.Body)
	}
	if rec := do("POST", "/jobs", job); rec.Code != 422 {
		t.Errorf("the same job again = %d, want 422 (its line is taken)", rec.Code)
	}

	// Shop use of 5 single-sided: 5 sheets as own_use.
	if rec := do("POST", "/own-use", fmt.Sprintf(`{"serviceId":%d,"size":"A4","copies":5}`, sid)); rec.Code != 204 {
		t.Fatalf("own use = %d %s", rec.Code, rec.Body)
	}
	var left decimal.Decimal
	must(db.Get(&left, `SELECT quantity FROM stock WHERE product_id = $1`, paper))
	if !left.Equal(decimal.NewFromInt(484)) { // 500 − 11 − 5
		t.Errorf("paper left %s, want 484", left)
	}

	// Return 1 copy of the job: the report takes it off; sheets stay printed.
	_, _, err = sales.NewService(db).PartialReturn(t.Context(), d.Sale.ID, sales.PartialReturnInput{
		Lines: []sales.ReturnLineInput{{SaleItemID: d.Items[0].ID, Quantity: "1"}}}, cashier)
	must(err)
	rrec := do("GET", "/report?preset=today", "")
	if rrec.Code != 200 {
		t.Fatalf("report = %d %s", rrec.Code, rrec.Body)
	}
	r := decode[ReportDTO](t, rrec)
	var row *ReportRowDTO
	for i := range r.Rows {
		if strings.Contains(r.Rows[i].Name, tag) {
			row = &r.Rows[i]
		}
	}
	if row == nil {
		t.Fatalf("report: %+v", r)
	}
	if row.Copies != "20" || row.Sheets != "11" || row.Revenue != "240.00" || row.Earlier || row.Size == nil || *row.Size != "A4" {
		t.Fatalf("report row: %+v", row)
	}
	if r.OwnUseJobs < 1 {
		t.Errorf("own use jobs %d, want ≥ 1", r.OwnUseJobs)
	}

	// The till lists it with its bands; 🖨 badges its product.
	ts := decode[[]TillService](t, do("GET", "/till", ""))
	found := false
	for _, s := range ts {
		if s.ID == sid {
			found = len(s.Offers) == 2 && len(s.Offers[0].Bands) == 2
		}
	}
	if !found {
		t.Errorf("till services: %+v", ts)
	}
	b, err := NewStore(db).Badges(t.Context(), []int64{pid, paper})
	must(err)
	if len(b) != 1 || b[pid][0] != Badge {
		t.Errorf("badges %v", b)
	}
}

// Laminate by size and type: each type its own price and pouch, no ink or sides.
func TestLaminateTypes(t *testing.T) {
	db := testDB(t)
	do := testAPI(t, db)
	tag := fmt.Sprintf("zl%d", time.Now().UnixNano())
	var cat int64
	if err := db.Get(&cat, `SELECT id FROM categories LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	pouch := map[string]int64{}
	for _, m := range []string{"Gloss", "Matte"} {
		var id int64
		if err := db.Get(&id, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
			VALUES ($1, $2, (SELECT id FROM units LIMIT 1), 20, 0) RETURNING id`, "A4 pouch "+m+" "+tag, cat); err != nil {
			t.Fatal(err)
		}
		pouch[m] = id
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM doc_service WHERE name LIKE '%' || $1 || '%'`, tag) //nolint:errcheck
		db.Exec(`DELETE FROM products WHERE name LIKE '%' || $1 || '%'`, tag)    //nolint:errcheck
	})
	made := decode[map[string]int64](t, do("POST", "/services", fmt.Sprintf(`{"name":"Laminate %s","categoryId":%d,"kind":"metered","category":"laminate"}`, tag, cat)))
	sid := made["id"]
	row := func(m, price string) string {
		return fmt.Sprintf(`{"size":"A4","material":%q,"tiers":[{"minQty":1,"cells":[%q,"","",""]}],"paper":[{"productId":%d,"perSheet":"1"}]}`, m, price, pouch[m])
	}
	rec := do("PUT", fmt.Sprintf("/services/%d/grid", sid), `{"rows":[`+row("Gloss", "100")+`,`+row("Matte", "120")+`]}`)
	if rec.Code != 200 {
		t.Fatalf("grid = %d %s", rec.Code, rec.Body)
	}
	if g := decode[GridDTO](t, rec); len(g.Rows) != 2 || g.Rows[0].Material != "Gloss" || g.Rows[1].Paper[0].ProductID != pouch["Matte"] {
		t.Fatalf("grid back: %+v", g)
	}
	if rec := do("PUT", fmt.Sprintf("/services/%d/grid", sid), `{"rows":[`+row("Gloss", "1")+`,`+row("Gloss", "2")+`]}`); rec.Code != 422 {
		t.Errorf("A4 Gloss twice = %d, want 422", rec.Code)
	}
	q := decode[QuoteDTO](t, do("GET", fmt.Sprintf("/quote?service=%d&size=A4&material=Matte&copies=3", sid), ""))
	if q.Total != "360.00" || q.Description != "Laminate "+tag+" A4 · Matte" || len(q.Components) != 1 ||
		q.Components[0].ProductID != pouch["Matte"] || q.Components[0].Quantity != "3" {
		t.Fatalf("matte quote: %+v", q)
	}
	if rec := do("GET", fmt.Sprintf("/quote?service=%d&size=A4&copies=1", sid), ""); rec.Code != 422 {
		t.Errorf("A4 with no type = %d, want 422", rec.Code)
	}
}
