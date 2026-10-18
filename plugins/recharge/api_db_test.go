package recharge

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"
	"karots-suite/internal/validator"

	"github.com/labstack/echo/v4"
)

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil || rec.Code >= 300 {
		t.Fatalf("%d %s: %v", rec.Code, rec.Body, err)
	}
	return v
}

// The admin sets up a carrier (rates, a device with its float, a package); the
// till reads it all in one call and quick-adds a package; float is bought from a
// safe with no expense; a count books the difference; the Float page, ledger
// (and its CSV) and report read it back.
func TestRechargeAPI(t *testing.T) {
	db := freshDB(t)
	db.Exec(`INSERT INTO units (name, abbreviation) VALUES ('Pieces', 'pcs') ON CONFLICT DO NOTHING`) //nolint:errcheck
	var safe int64
	must(t, db.Get(&safe, `INSERT INTO lockers (name, kind, allow_negative) VALUES ('Safe', 'safe', true) RETURNING id`))
	db.Exec(`INSERT INTO lockers (name, kind, cashier_access) VALUES ('BOC', 'bank', true)`) //nolint:errcheck
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = middleware.ErrorHandler(false)
	mux := plugin.NewMux()
	ss := sales.NewService(db)
	(&Plugin{}).Setup(plugin.NewRegistry(plugin.Core{DB: db, Products: products.NewService(db), Audit: audit.NewService(db),
		Settings: settings.NewService(db), Sales: ss, Cashflow: cashflow.NewService(db, ss), Lockers: lockers.NewService(db),
		CashRegister: cashregister.NewService(db, ss)}, mux, e))
	x := e.Group("/api/v1/x")
	mux.Mount(x, x, x)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1/x/recharge"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	cid := decode[map[string]int64](t, do("POST", "/carriers", `{"name":"Dialog"}`))["id"]
	cs := strconv.FormatInt(cid, 10)
	if rec := do("PUT", "/carriers/"+cs+"/rates", `[{"kind":"reload","percent":"3","fixed":"0","charge":"0"},{"kind":"billpay","percent":"0","fixed":"0","charge":"30"}]`); rec.Code != 204 {
		t.Fatalf("rates: %d %s", rec.Code, rec.Body)
	}
	dev := decode[map[string]int64](t, do("POST", "/devices", `{"carrierId":`+cs+`,"label":"Shop phone","reload":true,"money":true,"target":"5000","opening":"1000"}`))["id"]
	ds := strconv.FormatInt(dev, 10)
	decode[Package](t, do("POST", "/packages", `{"carrierId":`+cs+`,"name":"Data 100","price":"100","floatCost":"88"}`))
	if rec := do("POST", "/packages", `{"carrierId":`+cs+`,"name":"data 100","price":"100","floatCost":"88"}`); rec.Code != 409 {
		t.Fatalf("a twin package in admin: %d", rec.Code)
	}

	td := decode[TillData](t, do("GET", "/till", ""))
	if len(td.Carriers) != 1 || len(td.Floats) != 1 || !td.Floats[0].Balance.Equal(d("1000")) || len(td.Packages) != 1 ||
		len(td.Banks) != 1 || td.Products["cashout"] == 0 || !rateOf(td.Carriers[0].Rates, KindBillpay).Charge.Equal(d("30")) {
		t.Fatalf("till data: %+v", td)
	}
	q := decode[struct {
		Package Package `json:"package"`
		Created bool    `json:"created"`
	}](t, do("POST", "/packages/quick", `{"carrierId":`+cs+`,"name":"Night 50","price":"50","commission":"6"}`))
	if !q.Created || !q.Package.FloatCost.Equal(d("44")) {
		t.Fatalf("quick package: %+v", q)
	}
	if rec := do("POST", "/packages/quick", `{"carrierId":`+cs+`,"name":"No commission","price":"50"}`); rec.Code < 400 {
		t.Fatal("a quick package with no commission was taken")
	}

	var dealer int64
	must(t, db.Get(&dealer, `INSERT INTO suppliers (name) VALUES ('Dialog dealer') RETURNING id`))
	dl := strconv.FormatInt(dealer, 10)
	if rec := do("PUT", "/carriers/"+cs+"/supplier", `{"supplierId":`+dl+`}`); rec.Code != 204 {
		t.Fatalf("dealer: %d %s", rec.Code, rec.Body)
	}
	if rec := do("POST", "/refill", `{"deviceId":`+ds+`,"amount":"2000","source":"locker:`+strconv.FormatInt(safe, 10)+`"}`); rec.Code < 400 {
		t.Fatal("float bought from nobody went through")
	}
	ref := decode[map[string]int64](t, do("POST", "/refill", `{"deviceId":`+ds+`,"supplierId":`+dl+`,"amount":"2000","bonus":"20","reference":"F-1","source":"locker:`+strconv.FormatInt(safe, 10)+`"}`))
	var exps int
	must(t, db.Get(&exps, `SELECT count(*) FROM expenses`))
	if ref["receiptId"] == 0 || exps != 0 {
		t.Fatalf("refill receipt %v, expenses %d (buying float is not a cost)", ref, exps)
	}
	owed := func() (o string, open int) {
		must(t, db.Get(&o, `SELECT outstanding_balance::text FROM suppliers WHERE id = $1`, dealer))
		must(t, db.Get(&open, `SELECT count(*) FROM purchases WHERE supplier_id = $1 AND status <> 'paid'`, dealer))
		return
	}
	if o, open := owed(); o != "0.00" || open != 0 {
		t.Fatalf("paid now: dealer owed %s with %d open bills, want 0 / 0", o, open)
	}
	if rec := do("POST", "/refill", `{"deviceId":`+ds+`,"supplierId":`+dl+`,"amount":"100","source":"locker:999999"}`); rec.Code < 400 {
		t.Fatal("a refill from a locker that doesn't exist went through")
	}

	res := decode[[]CountResult](t, do("POST", "/counts", `{"counts":[{"deviceId":`+ds+`,"counted":"3000"}]}`))
	if len(res) != 1 || !res[0].Expected.Equal(d("3020")) || !res[0].Diff.Equal(d("-20")) {
		t.Fatalf("count: %+v", res)
	}
	fp := decode[FloatPage](t, do("GET", "/float", ""))
	if len(fp.Floats) != 1 || !fp.Total.Equal(d("3000")) || fp.NewPackages != 0 || !fp.Today.Equal(d("0")) { // +20 bonus − 20 short; no signed-in cashier here, so nothing is flagged (TestStoreSetup covers it)
		t.Fatalf("float page: %+v", fp)
	}
	rows := decode[[]LedgerRow](t, do("GET", "/ledger?preset=today", ""))
	if len(rows) != 4 { // opening, refill, its bonus, the count
		t.Fatalf("ledger: %+v", rows)
	}
	csvRec := do("GET", "/ledger?preset=today&format=csv", "")
	if csvRec.Code != 200 || !strings.Contains(csvRec.Body.String(), "refill") || !strings.HasPrefix(csvRec.Body.String(), "when,carrier") {
		t.Fatalf("csv: %d %s", csvRec.Code, csvRec.Body)
	}
	rep := decode[Report](t, do("GET", "/report?preset=today", ""))
	if len(rep.Counts) != 1 || len(rep.Lines) == 0 {
		t.Fatalf("report: %+v", rep)
	}

	// On credit: the bill is owed (the dealer's normal Pay settles it), no money moves.
	cr := decode[map[string]int64](t, do("POST", "/refill", `{"deviceId":`+ds+`,"supplierId":`+dl+`,"amount":"500","credit":true}`))
	if o, open := owed(); cr["receiptId"] != 0 || o != "500.00" || open != 1 {
		t.Fatalf("on credit: receipt %v, dealer owed %s with %d open bills, want none / 500 / 1", cr, o, open)
	}
	fp = decode[FloatPage](t, do("GET", "/float", ""))
	if !fp.Total.Equal(d("3500")) || fp.Floats[0].SupplierID != dealer {
		t.Fatalf("float after credit: %+v", fp)
	}
}
