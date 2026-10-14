package repairs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/customers"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"
	"karots-suite/internal/validator"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

type fixture struct {
	db               *sqlx.DB
	do               func(method, path, body string) *httptest.ResponseRecorder
	sales            *sales.Service
	cust, part, user int64
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	db := freshDB(t)
	f := fixture{db: db, sales: sales.NewService(db)}
	must(t, db.Get(new(int64), `INSERT INTO categories (name) VALUES ('Phones') RETURNING id`))
	must(t, db.Get(&f.user, `INSERT INTO users (name, phone, pin_hash) VALUES ('Cashier', '0700000000', 'x') RETURNING id`))
	must(t, db.Get(&f.cust, `INSERT INTO customers (name, phone) VALUES ('Kamal Silva', '0712223334') RETURNING id`))
	must(t, db.Get(&f.part, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('A12 screen', (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 2500, 4000) RETURNING id`))
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = middleware.ErrorHandler(false)
	mux := plugin.NewMux()
	p := &Plugin{}
	p.Setup(plugin.NewRegistry(plugin.Core{DB: db, Products: products.NewService(db), Audit: audit.NewService(db),
		Settings: settings.NewService(db), Sales: f.sales, Cashflow: cashflow.NewService(db, f.sales)}, mux, e))
	x := e.Group("/api/v1/x")
	mux.Mount(x, x, x)
	f.do = func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1/x/repairs"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	return f
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil || rec.Code >= 300 {
		t.Fatalf("%d %s: %v", rec.Code, rec.Body, err)
	}
	return v
}

// advance records a core customer payment (the credit form) and returns its id.
func (f fixture) advance(t *testing.T, amount string) int64 {
	t.Helper()
	var id int64
	must(t, appdb.WithTx(context.Background(), f.db, func(tx *sqlx.Tx) error {
		r, err := customers.NewService(f.db).RecordPaymentTx(context.Background(), tx, f.cust, customers.PaymentInput{Amount: amount, Method: "cash"}, 0)
		if r != nil {
			id = r.PaymentID
		}
		return err
	}))
	return id
}

// A job with a deposit taken as an advance: the bill pre-fills On account with
// it, the sale that rang the job marks it collected once, warranty stamped.
func TestJobDepositPickup(t *testing.T) {
	f := newFixture(t)
	if rec := f.do("POST", "/jobs", `{"deviceModel":"Galaxy A12"}`); rec.Code < 400 {
		t.Fatalf("a job needs its customer: %d", rec.Code)
	}
	j := decode[JobDTO](t, f.do("POST", "/jobs", fmt.Sprintf(`{"customerId":%d,"deviceModel":"Galaxy A12","fault":"Cracked","warrantyDays":30}`, f.cust)))
	if j.TicketNo != "R-0001" || j.CustomerPhone != "0712223334" || j.Status != "received" {
		t.Fatalf("new job: %+v", j)
	}
	path := fmt.Sprintf("/jobs/%d", j.ID)
	decode[JobDTO](t, f.do("POST", path+"/parts", fmt.Sprintf(`{"productId":%d,"qty":"1"}`, f.part)))
	j = decode[JobDTO](t, f.do("POST", path+"/charges", `{"label":"Fitting","amount":"1500"}`))
	if j.Estimate != "5500.00" {
		t.Fatalf("estimate: %+v", j)
	}
	pay := f.advance(t, "1000")
	j = decode[JobDTO](t, f.do("POST", path+"/deposits", fmt.Sprintf(`{"paymentId":%d}`, pay)))
	if j.DepositTotal != "1000.00" || j.Outstanding != "-1000.00" {
		t.Fatalf("deposit: %+v", j)
	}
	if rec := f.do("POST", path+"/deposits", fmt.Sprintf(`{"paymentId":%d}`, pay)); rec.Code < 400 {
		t.Fatal("the same payment can't be linked twice")
	}
	b := decode[BillDTO](t, f.do("GET", path+"/bill", ""))
	if len(b.Lines) != 2 || b.OnAccount != "1000.00" || b.Lines[1].Kind != "labour" || b.Lines[1].Name != "Fitting — R-0001" || b.Lines[1].Price != "1500.00" {
		t.Fatalf("bill: %+v", b)
	}
	// The till rings it: labour only here (no stock in the fixture), all on account.
	sale, err := f.sales.Create(context.Background(), sales.CreateInput{CustomerID: &f.cust, SaleType: "retail", AllowOverLimit: true,
		Items:    []sales.ItemInput{{ProductID: b.Lines[1].ProductID, Quantity: "1", PriceOverride: "1500"}},
		Payments: []sales.PaymentInput{{Method: "credit", Amount: "1500"}}}, f.user)
	must(t, err)
	j = decode[JobDTO](t, f.do("POST", path+"/collected", fmt.Sprintf(`{"saleId":%d}`, sale.Sale.ID)))
	if j.Status != "collected" || j.SaleID == nil || j.WarrantyUntil == "" || !j.InWarranty || j.Outstanding != "500.00" {
		t.Fatalf("collected: %+v", j)
	}
	if rec := f.do("POST", path+"/collected", fmt.Sprintf(`{"saleId":%d}`, sale.Sale.ID)); rec.Code < 400 {
		t.Fatal("a collected job can't be collected again")
	}
	// Free re-repair: a linked job whose bill is 100% off and pre-fills nothing.
	rw := decode[JobDTO](t, f.do("POST", path+"/rework", ""))
	decode[JobDTO](t, f.do("POST", fmt.Sprintf("/jobs/%d/parts", rw.ID), fmt.Sprintf(`{"productId":%d,"qty":"1"}`, f.part)))
	rb := decode[BillDTO](t, f.do("GET", fmt.Sprintf("/jobs/%d/bill", rw.ID), ""))
	if !rb.Free || rb.Lines[0].Discount != "100" || rb.Lines[0].DiscountType != "percent" || rb.OnAccount != "0.00" || rw.ReworkOf == nil {
		t.Fatalf("rework bill: %+v %+v", rb, rw)
	}
	// Report: revenue from the sale's job lines.
	rep := decode[ReportDTO](t, f.do("GET", "/report", ""))
	if len(rep.Rows) != 1 || rep.Rows[0].Revenue != "1500.00" || rep.Profit != "1500.00" {
		t.Fatalf("report: %+v", rep)
	}
}

// A sale of someone else, or with none of the job's lines, doesn't collect it.
func TestCollectedChecksTheSale(t *testing.T) {
	f := newFixture(t)
	j := decode[JobDTO](t, f.do("POST", "/jobs", fmt.Sprintf(`{"customerId":%d,"deviceModel":"iPhone"}`, f.cust)))
	decode[JobDTO](t, f.do("POST", fmt.Sprintf("/jobs/%d/charges", j.ID), `{"amount":"500"}`))
	var other int64
	must(t, f.db.Get(&other, `INSERT INTO customers (name) VALUES ('Someone') RETURNING id`))
	lab, err := NewStore(f.db).LabourID(context.Background())
	must(t, err)
	sale, err := f.sales.Create(context.Background(), sales.CreateInput{CustomerID: &other, SaleType: "retail", AllowOverLimit: true,
		Items:    []sales.ItemInput{{ProductID: lab, Quantity: "1", PriceOverride: "500"}},
		Payments: []sales.PaymentInput{{Method: "credit", Amount: "500"}}}, f.user)
	must(t, err)
	if rec := f.do("POST", fmt.Sprintf("/jobs/%d/collected", j.ID), fmt.Sprintf(`{"saleId":%d}`, sale.Sale.ID)); rec.Code < 400 {
		t.Fatalf("another customer's sale collected it: %d", rec.Code)
	}
}

// A karots-pos held deposit is spent by its tender, never past what's held.
func TestLegacyDepositTender(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	j := decode[JobDTO](t, f.do("POST", "/jobs", fmt.Sprintf(`{"customerId":%d,"deviceModel":"Nokia"}`, f.cust)))
	f.db.MustExec(`INSERT INTO repair_payments (job_id, amount, kind) VALUES ($1, 300, 'deposit')`, j.ID)
	settle := func(amt string) error {
		return appdb.WithTx(ctx, f.db, func(tx *sqlx.Tx) error {
			return settleLegacyDeposit(ctx, tx, plugin.TenderSettle{Amount: decimal.RequireFromString(amt), Meta: map[string]string{"job": fmt.Sprint(j.ID)}})
		})
	}
	if settle("400") == nil {
		t.Fatal("spent more than held")
	}
	must(t, settle("300"))
	if settle("0.01") == nil {
		t.Fatal("spent it twice")
	}
	if got := decode[JobDTO](t, f.do("GET", fmt.Sprintf("/jobs/%d", j.ID), "")); got.LegacyHeld != "0.00" {
		t.Fatalf("held after: %+v", got)
	}
}

// Paying the repairer books a core expense out of a locker and shows on the job;
// voiding that expense takes it off again.
func TestPayout(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	lk, err := lockers.NewService(f.db).Create(ctx, lockers.CreateInput{Name: "Safe", Kind: lockers.KindSafe, OpeningBalance: "5000", CashierAccess: "on"}, 0)
	must(t, err)
	j := decode[JobDTO](t, f.do("POST", "/jobs", fmt.Sprintf(`{"customerId":%d,"deviceModel":"Nokia","repairedBy":"Sunil"}`, f.cust)))
	if rec := f.do("POST", fmt.Sprintf("/jobs/%d/payouts", j.ID), `{"amount":"150","source":"external"}`); rec.Code < 400 {
		t.Fatal("money must come from a place")
	}
	j = decode[JobDTO](t, f.do("POST", fmt.Sprintf("/jobs/%d/payouts", j.ID), fmt.Sprintf(`{"amount":"150","source":"locker:%d"}`, lk.ID)))
	if j.RepairerPaid != "150.00" || len(j.Payouts) != 1 || !strings.Contains(j.Payouts[0].Note, "paid Sunil") {
		t.Fatalf("payout: %+v", j)
	}
	var cat string
	var svc int64
	must(t, f.db.QueryRow(`SELECT category, service_product_id FROM expenses WHERE id = $1`, j.Payouts[0].ExpenseID).Scan(&cat, &svc))
	if cat != "Repairs" || svc == 0 {
		t.Fatalf("expense: %s %d", cat, svc)
	}
	f.db.MustExec(`DELETE FROM expenses WHERE id = $1`, j.Payouts[0].ExpenseID)
	if j = decode[JobDTO](t, f.do("GET", fmt.Sprintf("/jobs/%d", j.ID), "")); j.RepairerPaid != "0.00" {
		t.Fatalf("after void: %+v", j)
	}
}

// A bill discount on the pickup sale is shared out onto the job's lines, as
// core's P&L counts it: 1,000 labour + 1,000 of something else − 200 off the
// bill = 1,800 paid, so the job took 900.
func TestReportSharesBillDiscount(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	j := decode[JobDTO](t, f.do("POST", "/jobs", fmt.Sprintf(`{"customerId":%d,"deviceModel":"Nokia"}`, f.cust)))
	decode[JobDTO](t, f.do("POST", fmt.Sprintf("/jobs/%d/charges", j.ID), `{"label":"Fitting","amount":"1000"}`))
	lab, err := NewStore(f.db).LabourID(ctx)
	must(t, err)
	var other int64
	must(t, f.db.Get(&other, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, is_service)
		VALUES ('Data transfer', (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 0, 0, true) RETURNING id`))
	sale, err := f.sales.Create(ctx, sales.CreateInput{CustomerID: &f.cust, SaleType: "retail", AllowOverLimit: true,
		Discount: "200", DiscountType: "fixed",
		Items: []sales.ItemInput{{ProductID: lab, Quantity: "1", PriceOverride: "1000"}, {ProductID: other, Quantity: "1", PriceOverride: "1000"}},
		Payments: []sales.PaymentInput{{Method: "credit", Amount: "1800"}}}, f.user)
	must(t, err)
	decode[JobDTO](t, f.do("POST", fmt.Sprintf("/jobs/%d/collected", j.ID), fmt.Sprintf(`{"saleId":%d}`, sale.Sale.ID)))
	rep := decode[ReportDTO](t, f.do("GET", "/report", ""))
	if len(rep.Rows) != 1 || rep.Rows[0].Revenue != "900.00" {
		t.Fatalf("report: %+v", rep)
	}
}
