package pharmacy

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"karots-suite/internal/features/products"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"
	"karots-suite/internal/validator"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// testAPI mounts the routes as the server does; ?cashier=1 is refused on admin routes.
func testAPI(t *testing.T, db *sqlx.DB) func(method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = middleware.ErrorHandler(false)
	mux := plugin.NewMux()
	p := &Plugin{}
	p.Setup(plugin.NewRegistry(plugin.Core{DB: db, Products: products.NewService(db)}, mux, e))
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
		req := httptest.NewRequest(method, "/api/v1/x/pharmacy"+path, strings.NewReader(body))
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

func TestAPIPatientScript(t *testing.T) {
	f := newFixture(t)
	do := testAPI(t, f.db)
	if rec := do("POST", "/patients", `{"name":"N","phone":"077"}`); rec.Code != 422 && rec.Code != 400 {
		t.Fatalf("a one-letter name: %d %s", rec.Code, rec.Body)
	}
	p := decode[PatientDTO](t, do("POST", "/patients", `{"name":"Kamala","phone":"0711111111","allergies":"Sulfa"}`))
	if p.ID == 0 || p.Allergies != "Sulfa" {
		t.Fatalf("register: %+v", p)
	}
	body := fmt.Sprintf(`{"patientId":%d,"prescriberName":"Dr Fernando","prescribedOn":"2026-10-01","items":[{"groupId":%d,"dosage":"1-0-1","quantity":"28"}]}`, p.ID, f.group)
	if rec := do("POST", "/prescriptions", body); rec.Code != 200 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	scs := decode[[]ScriptDTO](t, do("GET", fmt.Sprintf("/patients/%d/scripts", p.ID), ""))
	if len(scs) != 1 || len(scs[0].Items) != 1 {
		t.Fatalf("scripts: %+v", scs)
	}
	it := scs[0].Items[0]
	if it.PerDay != "2" || it.Owed != "28" || it.Suggest == nil || it.Suggest.ProductID != f.metfor || scs[0].PrescribedOn != "2026-10-01" {
		t.Fatalf("line: %+v", it)
	}
	if rec := do("POST", fmt.Sprintf("/prescriptions/%d/active", scs[0].ID), `{"active":false}`); rec.Code != 204 {
		t.Fatalf("stop: %d %s", rec.Code, rec.Body)
	}
	if scs = decode[[]ScriptDTO](t, do("GET", fmt.Sprintf("/patients/%d/scripts", p.ID), "")); scs[0].Active || scs[0].Items[0].Suggest != nil {
		t.Fatalf("a stopped script suggests nothing: %+v", scs[0])
	}
	if rec := do("POST", "/prescriptions", fmt.Sprintf(`{"patientId":%d,"items":[]}`, p.ID)); rec.Code < 400 {
		t.Fatal("a script with no medicine must be refused")
	}
}
