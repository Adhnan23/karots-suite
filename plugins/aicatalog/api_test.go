package aicatalog

import (
	"context"
	"encoding/json"
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

// testAPI mounts the plugin's routes the way the server does (minus auth).
func testAPI(t *testing.T, db *sqlx.DB) (*Plugin, func(method, path, body string) *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = middleware.ErrorHandler(false)
	mux := plugin.NewMux()
	p := &Plugin{}
	p.Setup(plugin.NewRegistry(plugin.Core{DB: db, Products: products.NewService(db), Audit: audit.NewService(db)}, mux, e))
	x := e.Group("/api/v1/x")
	mux.Mount(x, x, x)
	return p, func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1/x/aicatalog"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if rec.Code >= 300 || json.Unmarshal(rec.Body.Bytes(), &v) != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	return v
}

func TestAPISettingsNeverEchoKey(t *testing.T) {
	_, do := testAPI(t, freshDB(t, "aicatalog_api_settings"))
	if s := decode[SettingsDTO](t, do("GET", "/settings", "")); !s.ManualMode || s.HasKey {
		t.Fatalf("fresh settings %+v", s)
	}
	rec := do("PUT", "/settings", `{"manualMode":false,"provider":"groq","baseUrl":"https://api.groq.com/openai/v1","model":"llama","apiKey":"sk-secret"}`)
	if s := decode[SettingsDTO](t, rec); !s.HasKey || s.ManualMode || strings.Contains(rec.Body.String(), "sk-secret") {
		t.Fatalf("after save %s", rec.Body)
	}
	if s := decode[SettingsDTO](t, do("PUT", "/settings", `{"manualMode":true,"provider":"groq","baseUrl":"x","model":"y","apiKey":""}`)); !s.HasKey {
		t.Fatal("a blank key must keep the stored one")
	}
	if rec := do("PUT", "/settings", `{"manualMode":false}`); rec.Code != 422 {
		t.Fatalf("API mode without a model: %d", rec.Code)
	}
}

func TestAPIIdentifyPasteAndAPI(t *testing.T) {
	db := freshDB(t, "aicatalog_api_identify")
	_, do := testAPI(t, db)
	r := decode[AskReply[IdentifyResult]](t, do("POST", "/identify", `{"query":"6201 ZZ"}`))
	if r.Result != nil || !strings.Contains(r.Prompt, "Identify this item: 6201 ZZ") || !strings.Contains(r.Prompt, "Old shop") {
		t.Fatalf("paste mode: %+v", r)
	}
	if r = decode[AskReply[IdentifyResult]](t, do("POST", "/identify", `{"query":"6202","short":true}`)); !strings.HasPrefix(r.Prompt, "Next item: 6202") {
		t.Fatalf("short: %q", r.Prompt)
	}
	reply, _ := json.Marshal(map[string]string{"query": "6201 ZZ", "reply": "Here you go:\n```json\n" + answer + "\n```"})
	if r = decode[AskReply[IdentifyResult]](t, do("POST", "/identify", string(reply))); r.Result == nil || r.Result.Best.Name != "Coca-Cola 1L" {
		t.Fatalf("pasted reply: %+v", r)
	}
	if r = decode[AskReply[IdentifyResult]](t, do("POST", "/identify", `{"query":"6201","reply":"sorry, no idea"}`)); r.Result != nil || r.Error == "" || r.Prompt == "" {
		t.Fatalf("bad paste should give the prompt back with why: %+v", r)
	}
	if rec := do("POST", "/identify", `{}`); rec.Code != 422 {
		t.Fatalf("empty query: %d", rec.Code)
	}

	// API mode: an answer comes straight back; a failure offers the prompt.
	ok := fake(t, 200, `{"choices":[{"message":{"content":"`+strings.ReplaceAll(answer, `"`, `\"`)+`"}}]}`, nil)
	do("PUT", "/settings", `{"manualMode":false,"provider":"openai","baseUrl":"`+ok.URL+`","model":"m"}`)
	if r = decode[AskReply[IdentifyResult]](t, do("POST", "/identify", `{"query":"coke"}`)); r.Result == nil || r.Prompt != "" {
		t.Fatalf("API answer: %+v", r)
	}
	bad := fake(t, 500, `{"error":{"message":"overloaded"}}`, nil)
	do("PUT", "/settings", `{"manualMode":false,"provider":"openai","baseUrl":"`+bad.URL+`","model":"m"}`)
	if r = decode[AskReply[IdentifyResult]](t, do("POST", "/identify", `{"query":"coke"}`)); r.Result != nil || !strings.Contains(r.Error, "overloaded") || !strings.Contains(r.Prompt, "Identify this item: coke") {
		t.Fatalf("API failure: %+v", r)
	}
}

func TestAPICategoryOnceAndLearn(t *testing.T) {
	db := freshDB(t, "aicatalog_api_category")
	p, do := testAPI(t, db)
	before := count(t, db, `SELECT count(*) FROM categories`)
	a := decode[map[string]int64](t, do("POST", "/category", `{"path":"Bearings / Ball Bearings"}`))
	b := decode[map[string]int64](t, do("POST", "/category", `{"path":"Bearings > Ball Bearings"}`))
	if a["id"] == 0 || a["id"] != b["id"] || count(t, db, `SELECT count(*) FROM categories`) != before+2 {
		t.Fatalf("path created once: %v %v", a, b)
	}
	if rec := do("POST", "/category", `{"path":" > "}`); rec.Code != 422 {
		t.Fatalf("blank path: %d", rec.Code)
	}
	var pid int64
	mustExec(t, db, &pid, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('6201-2Z', $1, (SELECT id FROM units LIMIT 1), 1, 2) RETURNING id`, a["id"])
	ctx := context.Background()
	raw := json.RawMessage(`{"name":"6201-2Z Ball Bearing","category":"Bearings > Ball Bearings","specs":"12x32x10 mm","source":"user_picked","query":"6201 ZZ"}`)
	if err := p.learn(ctx, plugin.ExtraCtx{ProductID: pid, IsNew: true}, raw); err != nil {
		t.Fatal(err)
	}
	rows, err := p.detailRows(ctx, pid)
	if err != nil || len(rows) != 1 || rows[0].Value != "6201-2Z Ball Bearing — 12x32x10 mm" {
		t.Fatalf("ⓘ row %+v %v", rows, err)
	}
	if _, err := decodeExtra(json.RawMessage(`{"name":"x","source":"guess"}`)); err == nil {
		t.Error("an unknown source should be refused")
	}
}

// The preview drops ops naming things this chunk lacks and keeps the look-alike with more sales.
func TestAPICleanupPreview(t *testing.T) {
	db := freshDB(t, "aicatalog_api_cleanup")
	_, do := testAPI(t, db)
	c := cat(t, db, "Drinks", nil)
	a, b := prod(t, db, "Coke 1L", c), prod(t, db, "Coca Cola 1 Ltr", c)
	var uid int64
	mustExec(t, db, &uid, `INSERT INTO users (name, phone, pin_hash) VALUES ('T', '0770009999', 'x') RETURNING id`)
	mustExec(t, db, nil, `INSERT INTO sales (receipt_no, total, cashier_id) VALUES ('S-1', 2, $1)`, uid)
	mustExec(t, db, nil, `INSERT INTO sale_items (sale_id, product_id, quantity, unit_price, subtotal) VALUES ((SELECT max(id) FROM sales), $1, 1, 2, 2)`, b)
	if r := decode[AskReply[CleanupPreview]](t, do("POST", "/cleanup", `{"offset":0}`)); r.Result != nil || !strings.Contains(r.Prompt, "|Coke 1L|") {
		t.Fatalf("paste mode prompt: %+v", r)
	}
	reply := `{"renames":[{"id":999999,"to":"x"}],"moves":[{"product_id":` + itoa(a) + `,"category_id":` + itoa(c) + `}],
		"duplicates":[{"a":` + itoa(a) + `,"b":` + itoa(b) + `,"reason":"same drink"},{"a":` + itoa(b) + `,"b":` + itoa(a) + `},{"a":` + itoa(a) + `,"b":999999}]}`
	body, _ := json.Marshal(CleanupRequest{Reply: reply})
	r := decode[AskReply[CleanupPreview]](t, do("POST", "/cleanup", string(body)))
	if r.Result == nil {
		t.Fatalf("%+v", r)
	}
	pv := r.Result
	if len(pv.Plan.Renames) != 0 || len(pv.Plan.Moves) != 1 || len(pv.Duplicates) != 1 || pv.Duplicates[0].Keep.ID != b || pv.Duplicates[0].Drop.ID != a || pv.Next != -1 || pv.Chunks != 1 {
		t.Fatalf("preview %+v", pv)
	}
	ap := decode[ApplyReply](t, do("POST", "/cleanup/apply", `{"plan":{"merges":[]},"runId":0}`))
	if ap.Summary != "No changes applied." {
		t.Fatalf("empty apply %+v", ap)
	}
	if rec := do("POST", "/cleanup/undo", ""); rec.Code != 422 {
		t.Fatalf("undo with nothing: %d", rec.Code)
	}
}
