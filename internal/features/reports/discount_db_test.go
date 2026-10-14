package reports

import (
	"context"
	"net/url"
	"os"
	"testing"
	"time"

	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/sales"
	coremigrations "karots-suite/migrations"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// discountDB is a fresh database with core migrated: the sale goes through the
// real sales.Create, so the reports read exactly what a till sale stores.
func discountDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := appdb.Connect(dsn)
	mustR(t, err)
	t.Cleanup(func() { admin.Close() })
	const name = "reports_discount_test_db"
	admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) //nolint:errcheck
	_, err = admin.Exec(`CREATE DATABASE ` + name)
	mustR(t, err)
	t.Cleanup(func() { admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) }) //nolint:errcheck
	u, _ := url.Parse(dsn)
	u.Path = "/" + name
	db, err := appdb.Connect(u.String())
	mustR(t, err)
	t.Cleanup(func() { db.Close() })
	mustR(t, appdb.RunMigrations(db.DB, coremigrations.FS))
	return db
}

// One sale with both kinds of discount must read the same in every finance
// report: 2 × Rs 100 with Rs 10 off each (line, fixed per unit) = 180, plus a
// Rs 100 service, then 10% off the bill → 280 − 28 = 252 paid. Cost 60 + 60.
func TestDiscountedSaleInEveryReport(t *testing.T) {
	db := discountDB(t)
	ctx := context.Background()
	var cat, unit, user, goods, svc int64
	mustR(t, db.Get(&cat, `INSERT INTO categories (name) VALUES ('Disc') RETURNING id`))
	mustR(t, db.Get(&unit, `SELECT id FROM units LIMIT 1`))
	mustR(t, db.Get(&user, `INSERT INTO users (name, phone, pin_hash) VALUES ('Cashier', '0700000000', 'x') RETURNING id`))
	mustR(t, db.Get(&goods, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('Soap', $1, $2, 60, 100) RETURNING id`, cat, unit))
	mustR(t, db.Get(&svc, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, is_service)
		VALUES ('Fitting', $1, $2, 0, 100, true) RETURNING id`, cat, unit))
	db.MustExec(`UPDATE stock SET quantity = 10 WHERE product_id = $1`, goods)

	d, err := sales.NewService(db).Create(ctx, sales.CreateInput{SaleType: "retail", Discount: "10", DiscountType: "percent",
		Items: []sales.ItemInput{
			{ProductID: goods, Quantity: "2", Discount: "10", DiscountType: "fixed", AllowOversell: true},
			{ProductID: svc, Quantity: "1", PriceOverride: "100"},
		},
		Payments: []sales.PaymentInput{{Method: "cash", Amount: "252"}}}, user)
	mustR(t, err)
	want := decimal.NewFromInt(252)
	if !d.Sale.Total.Equal(want) {
		t.Fatalf("sale total %s, want 252", d.Sale.Total)
	}
	from, to := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	r := NewService(db)
	eq := func(what string, got decimal.Decimal) {
		t.Helper()
		if !got.Round(2).Equal(want) {
			t.Errorf("%s = %s, want 252", what, got.Round(2))
		}
	}

	pl, err := r.Compute(ctx, from, to)
	mustR(t, err)
	eq("P&L revenue", pl.Revenue)
	eq("P&L received", pl.Received)

	days, err := r.SalesByPeriod(ctx, from, to, "day")
	mustR(t, err)
	sum := decimal.Zero
	for _, x := range days {
		sum = sum.Add(x.Revenue)
	}
	eq("Sales by day", sum)

	cashiers, err := r.SalesByCashier(ctx, from, to)
	mustR(t, err)
	if len(cashiers) == 1 {
		eq("Sales by cashier (net)", cashiers[0].Net)
	}

	cats, err := r.ProfitByCategory(ctx, from, to)
	mustR(t, err)
	sum = decimal.Zero
	for _, x := range cats {
		sum = sum.Add(x.Revenue)
	}
	eq("Profit by category", sum)

	top, err := r.TopProducts(ctx, from, to, "revenue", 50)
	mustR(t, err)
	sum = decimal.Zero
	for _, x := range top {
		sum = sum.Add(x.Revenue)
	}
	eq("Top products", sum)

	tot, err := r.ProductTotals(ctx, from, to)
	mustR(t, err)
	eq("Product totals", tot.Revenue)

	// The Rs 100 service took its 10% share of the bill discount: 90.
	sp, err := r.ServiceProfit(ctx, from, to)
	mustR(t, err)
	if len(sp) != 1 || !sp[0].Revenue.Round(2).Equal(decimal.NewFromInt(90)) {
		t.Errorf("Service profit %+v, want revenue 90", sp)
	}

	// Profit must agree too: 252 − cost 120.
	if p := pl.Revenue.Sub(pl.COGS); !p.Equal(decimal.NewFromInt(132)) {
		t.Errorf("P&L gross profit %s, want 132", p)
	}
	if !tot.Profit.Round(2).Equal(decimal.NewFromInt(132)) {
		t.Errorf("Product totals profit %s, want 132", tot.Profit.Round(2))
	}
}

// Tax is the government's, not the shop's: a VAT-registered shop's sale of Rs 100
// + 18% shows Rs 100 revenue everywhere, Rs 18 as tax collected, Rs 118 received.
// A shop that isn't registered (or has no number) charges no tax at all, whatever
// rate a product carries.
func TestTaxIsNotRevenue(t *testing.T) {
	db := discountDB(t)
	ctx := context.Background()
	var cat, unit, user, item int64
	mustR(t, db.Get(&cat, `INSERT INTO categories (name) VALUES ('Taxed') RETURNING id`))
	mustR(t, db.Get(&unit, `SELECT id FROM units LIMIT 1`))
	mustR(t, db.Get(&user, `INSERT INTO users (name, phone, pin_hash) VALUES ('Cashier', '0700000000', 'x') RETURNING id`))
	mustR(t, db.Get(&item, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, tax_rate)
		VALUES ('Shampoo', $1, $2, 60, 100, 18) RETURNING id`, cat, unit))
	db.MustExec(`UPDATE stock SET quantity = 10 WHERE product_id = $1`, item)
	sell := func(paid string) *sales.Detail {
		d, err := sales.NewService(db).Create(ctx, sales.CreateInput{SaleType: "retail",
			Items:    []sales.ItemInput{{ProductID: item, Quantity: "1"}},
			Payments: []sales.PaymentInput{{Method: "cash", Amount: paid}}}, user)
		mustR(t, err)
		return d
	}
	// Ticked "registered" but no number: still no tax.
	db.MustExec(`UPDATE settings SET tax_registered = true, tax_reg_no = '' WHERE id = 1`)
	if d := sell("100"); !d.Sale.Tax.IsZero() || !d.Sale.Total.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("unregistered shop charged tax: total %s tax %s", d.Sale.Total, d.Sale.Tax)
	}
	db.MustExec(`DELETE FROM payments; DELETE FROM sale_items; DELETE FROM sales`)

	db.MustExec(`UPDATE settings SET tax_reg_no = '123456789-7000' WHERE id = 1`)
	if d := sell("118"); !d.Sale.Tax.Equal(decimal.NewFromInt(18)) || !d.Sale.Total.Equal(decimal.NewFromInt(118)) {
		t.Fatalf("registered shop: total %s tax %s", d.Sale.Total, d.Sale.Tax)
	}
	from, to := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	r := NewService(db)
	pl, err := r.Compute(ctx, from, to)
	mustR(t, err)
	hundred := decimal.NewFromInt(100)
	if !pl.Revenue.Equal(hundred) || !pl.Tax.Equal(decimal.NewFromInt(18)) || !pl.Received.Equal(decimal.NewFromInt(118)) ||
		!pl.GrossProfit.Equal(decimal.NewFromInt(40)) {
		t.Fatalf("P&L revenue %s tax %s received %s profit %s", pl.Revenue, pl.Tax, pl.Received, pl.GrossProfit)
	}
	days, err := r.SalesByPeriod(ctx, from, to, "day")
	mustR(t, err)
	cash, err := r.SalesByCashier(ctx, from, to)
	mustR(t, err)
	cats, err := r.ProfitByCategory(ctx, from, to)
	mustR(t, err)
	if len(days) != 1 || !days[0].Revenue.Round(2).Equal(hundred) || len(cash) != 1 || !cash[0].Net.Equal(hundred) ||
		len(cats) != 1 || !cats[0].Revenue.Round(2).Equal(hundred) {
		t.Fatalf("pre-tax everywhere: days %+v cashier %+v categories %+v", days, cash, cats)
	}
}
