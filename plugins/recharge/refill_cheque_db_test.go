package recharge

import (
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/cheques"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"
	"karots-suite/internal/validator"

	"github.com/labstack/echo/v4"
)

// Float bought with a cheque: the float is on the device and the dealer's bill
// paid at once, the bank untouched until the cheque clears. A bounce makes the
// dealer owed again (core's supplier side); the float stays.
func TestRefillByCheque(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	var bank, dealer int64
	must(t, db.Get(&bank, `INSERT INTO lockers (name, kind) VALUES ('BOC', 'bank') RETURNING id`))
	must(t, db.Get(&dealer, `INSERT INTO suppliers (name) VALUES ('Dialog dealer') RETURNING id`))
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = middleware.ErrorHandler(false)
	mux := plugin.NewMux()
	ss := sales.NewService(db)
	cf := cashflow.NewService(db, ss)
	(&Plugin{}).Setup(plugin.NewRegistry(plugin.Core{DB: db, Products: products.NewService(db), Audit: audit.NewService(db),
		Settings: settings.NewService(db), Sales: ss, Cashflow: cf, Lockers: lockers.NewService(db),
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
	cs := strconv.FormatInt(decode[map[string]int64](t, do("POST", "/carriers", `{"name":"Dialog"}`))["id"], 10)
	ds := strconv.FormatInt(decode[map[string]int64](t, do("POST", "/devices", `{"carrierId":`+cs+`,"label":"Shop phone","reload":true,"opening":"1000"}`))["id"], 10)
	dl, bk := strconv.FormatInt(dealer, 10), strconv.FormatInt(bank, 10)
	day := time.Now().Format("2006-01-02")
	owed := func() (o string) {
		must(t, db.Get(&o, `SELECT outstanding_balance::text FROM suppliers WHERE id = $1`, dealer))
		return
	}
	float := func() string { return decode[FloatPage](t, do("GET", "/float", "")).Total.StringFixed(2) }
	bankBal := func() (b string) {
		must(t, db.Get(&b, `SELECT COALESCE(SUM(balance_delta), 0)::numeric(14,2)::text FROM locker_ledger WHERE locker_id = $1`, bank))
		return
	}

	if rec := do("POST", "/refill", `{"deviceId":`+ds+`,"supplierId":`+dl+`,"amount":"2000","method":"cheque","cheques":[{"lockerId":`+bk+`,"number":"100001","amount":"1500","date":"`+day+`"}]}`); rec.Code < 400 {
		t.Fatal("cheques that don't add up to the float were taken")
	}
	decode[map[string]int64](t, do("POST", "/refill", `{"deviceId":`+ds+`,"supplierId":`+dl+`,"amount":"2000","method":"cheque","cheques":[{"lockerId":`+bk+`,"number":"100001","amount":"2000","date":"`+day+`"}]}`))
	var chq int64
	var status string
	must(t, db.QueryRow(`SELECT id, status FROM cheques WHERE direction = 'out' AND supplier_id = $1`, dealer).Scan(&chq, &status))
	if o, f, b := owed(), float(), bankBal(); status != "pending" || o != "0.00" || f != "3000.00" || b != "0.00" {
		t.Fatalf("paid by cheque: cheque %s, owed %s, float %s, bank %s; want pending / 0 / 3000 / 0", status, o, f, b)
	}
	must(t, cheques.NewService(db, cf).Return(ctx, chq, "bounced", 0))
	if o, f := owed(), float(); o != "2000.00" || f != "3000.00" {
		t.Fatalf("bounced: owed %s, float %s; want 2000 / 3000", o, f)
	}
}
