package productplus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"
	"karots-suite/internal/validator"

	"github.com/labstack/echo/v4"
)

// testAPI mounts the routes the way the server does, with a stand-in admin
// guard (?cashier=1 is refused), and returns the plugin and a request helper.
func testAPI(t *testing.T, f *fixture) (*Plugin, func(method, path, body string) *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = middleware.ErrorHandler(false)
	mux := plugin.NewMux()
	p := &Plugin{}
	p.Setup(plugin.NewRegistry(plugin.Core{DB: f.db, Audit: audit.NewService(f.db)}, mux, e))
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
	return p, func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1/x/productplus"+path, strings.NewReader(body))
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

func TestAPIFields(t *testing.T) {
	f := setup(t)
	_, do := testAPI(t, f)
	if rec := do("POST", "/fields", `{"label":" ","type":"text"}`); rec.Code != 422 {
		t.Fatalf("empty label: %d", rec.Code)
	}
	if rec := do("POST", "/fields", fmt.Sprintf(`{"label":"%s Size","type":"select","options":[]}`, f.tag)); rec.Code != 422 {
		t.Fatalf("dropdown with no options: %d", rec.Code)
	}
	if rec := do("GET", "/fields?cashier=1", ""); rec.Code != 403 {
		t.Fatalf("a cashier on the fields list: %d", rec.Code)
	}
	rec := do("POST", "/fields", fmt.Sprintf(`{"label":"%s Note","type":"text","required":true,"scope":[%d]}`, f.tag, f.parent))
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	fd := decode[FieldDTO](t, rec)
	if len(fd.Scope) != 1 || fd.Scope[0].ID != f.parent || fd.DefaultValue != "" {
		t.Fatalf("created: %+v", fd)
	}
	p1 := f.product(t, "P1", f.child)
	p2 := f.product(t, "P2", f.child)
	f.set(t, p1, Field{ID: fd.ID, Label: fd.Label, Type: "text"}, "12")
	f.set(t, p2, Field{ID: fd.ID, Label: fd.Label, Type: "text"}, "twelve")

	list := decode[[]FieldDTO](t, do("GET", "/fields", ""))
	for _, l := range list {
		if l.ID == fd.ID && l.Missing != 0 {
			t.Fatalf("missing with both filled: %+v", l)
		}
	}
	im := decode[Impact](t, do("GET", fmt.Sprintf("/fields/%d/impact?type=number", fd.ID), ""))
	if im.Lossy != 1 || im.Total != 2 {
		t.Fatalf("impact: %+v", im)
	}
	upd := fmt.Sprintf(`{"label":"%s","type":"number","required":true,"scope":[%d]%%s}`, fd.Label, f.parent)
	if rec := do("PUT", fmt.Sprintf("/fields/%d", fd.ID), fmt.Sprintf(upd, "")); rec.Code != 409 || !strings.Contains(rec.Body.String(), "1 of 2") {
		t.Fatalf("lossy type change without a confirm: %d %s", rec.Code, rec.Body)
	}
	if rec := do("PUT", fmt.Sprintf("/fields/%d", fd.ID), fmt.Sprintf(upd, `,"confirmLoss":true`)); rec.Code != 200 {
		t.Fatalf("confirmed type change: %d %s", rec.Code, rec.Body)
	}

	// Fill in: one bad cell, the rest saved.
	body := fmt.Sprintf(`[{"productId":%d,"fieldId":%d,"value":"7"},{"productId":%d,"fieldId":%d,"value":"x"}]`, p1, fd.ID, p2, fd.ID)
	res := decode[SaveResult](t, do("PUT", "/values", body))
	if res.Saved != 1 || len(res.Errors) != 1 || res.Errors[0].ProductID != p2 {
		t.Fatalf("fill in: %+v", res)
	}
	grid := decode[GridDTO](t, do("GET", fmt.Sprintf("/grid?fields=%d&categoryId=%d&missing=1", fd.ID, f.parent), ""))
	if grid.Total != 1 || grid.Rows[0].ID != p2 {
		t.Fatalf("grid missing: %+v", grid)
	}
	form := decode[[]FormFieldDTO](t, do("GET", fmt.Sprintf("/form?productId=%d&categoryId=%d", p1, f.child), ""))
	if len(form) != 1 || form[0].Value != "7" || !form[0].Required {
		t.Fatalf("form: %+v", form)
	}
}

func TestHooks(t *testing.T) {
	f := setup(t)
	p, _ := testAPI(t, f)
	ctx := context.Background()
	model := f.field(t, Field{Label: "Model", Type: "text", Required: true}, f.parent)
	made := f.field(t, Field{Label: "Made", Type: "date"})
	extra := func(vals map[string]string) json.RawMessage {
		b, _ := json.Marshal(map[string]any{"values": vals})
		return b
	}
	x := plugin.ExtraCtx{CategoryID: f.child, IsNew: true}

	err := p.validateExtra(ctx, x, extra(map[string]string{model.Key: " "}))
	if ae, ok := apperr.As(err); !ok || !strings.Contains(ae.Message, model.Label+" is required") {
		t.Fatalf("blank required: %v", err)
	}
	if err := p.validateExtra(ctx, x, extra(map[string]string{made.Key: "2026-01-02"})); err != nil {
		t.Fatalf("required field not shown (not sent) must pass: %v", err)
	}
	pid := f.product(t, "Copy", f.child)
	x.ProductID = pid
	if err := p.saveExtra(ctx, x, extra(map[string]string{model.Key: "XJ-1", made.Key: "2026-01-02"})); err != nil {
		t.Fatal(err)
	}
	if v, _ := f.stored(pid, model); v != "XJ-1" {
		t.Fatalf("saved: %q", v)
	}
	note, err := p.store.SheetImport(ctx, pid, f.child, map[string]string{made.Label: "02/01/2026", model.Label: "XJ-2"})
	if err != nil || !strings.Contains(note, made.Label+": not a date") {
		t.Fatalf("sheet import note: %q %v", note, err)
	}
	if v, _ := f.stored(pid, model); v != "XJ-2" {
		t.Fatalf("good cell beside a bad one: %q", v)
	}
}
