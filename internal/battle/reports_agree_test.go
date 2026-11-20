package battle

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"karots-suite/internal/datetime"
	"karots-suite/internal/ledgercheck/ltest"
)

// agree is area 7's auditor (docs/rules/reports.md): reports store nothing, so
// instead of corrupt-a-row checks it reads every report over [from, to] (shop
// days, YYYY-MM-DD, to inclusive) through the real API and holds each figure to
// plain sums over the raw rows (written fresh here, never the reports' own SQL)
// and to the other reports showing the same thing. Every miss fails the test
// with its rule id.
func (s *shop) agree(t ltest.T, from, to string) {
	helperOf(t)
	a := &agreeing{t: t, s: s, q: "?from=" + from + "&to=" + to, ctx: context.Background()}
	var err error
	if a.from, err = time.ParseInLocation("2006-01-02", from, datetime.Location); err != nil {
		t.Fatal(err)
	}
	if a.to, err = time.ParseInLocation("2006-01-02", to, datetime.Location); err != nil {
		t.Fatal(err)
	}
	a.to = a.to.AddDate(0, 0, 1)
	a.raw()
	a.pl()
	a.byTime()
	a.lines()
	a.people()
	a.pages()
	a.zReports()
}

type agreeing struct {
	t        ltest.T
	s        *shop
	ctx      context.Context
	q        string
	from, to time.Time
	// the raw rows' sums (RPT-1, 6, 7, 16, 17)
	count                                   int
	revenue, cost, tax, taxBack, received   decimal.Decimal
	refunds                                 decimal.Decimal
	byMethod                                map[string]decimal.Decimal
	finRevenue, finProfit, finTax, finRecv  decimal.Decimal
	finExpenses, finLosses, finRecoveries   decimal.Decimal
	finReceivables, finPayables, finAdvance decimal.Decimal
}

// eq fails when got and want differ by more than tol (each summed row rounds to
// the cent on its own, so a sum of n rows may be off by n half-cents).
func (a *agreeing) eq(rule, what string, got, want decimal.Decimal, tol decimal.Decimal) {
	helperOf(a.t)
	if got.Sub(want).Abs().GreaterThan(tol) {
		a.t.Errorf("%s %s %s: got %s, want %s", rule, a.q, what, got.StringFixed(2), want.StringFixed(2))
	}
}

var cent = decimal.RequireFromString("0.01")

func halfCents(n int) decimal.Decimal {
	return decimal.NewFromInt(int64(n)).Mul(decimal.RequireFromString("0.005")).Add(cent)
}

func (a *agreeing) d(s string) decimal.Decimal {
	helperOf(a.t)
	if s == "" {
		return decimal.Zero
	}
	return dec(a.t, s)
}

func (a *agreeing) get(path string, out any) {
	helperOf(a.t)
	a.s.must(a.t, a.s.admin.Do("GET", path, nil, out))
}

func (a *agreeing) sum(out *decimal.Decimal, q string, args ...any) {
	helperOf(a.t)
	a.s.must(a.t, a.s.db.GetContext(a.ctx, out, q, args...))
}

// nonPT: a sale line of the shop's own goods (pass-through face is never revenue, RPT-3).
const (
	saleIn = `s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2`
	retIn  = `s.status <> 'void' AND r.created_at >= $1 AND r.created_at < $2`
	// payouts: the sale's money handed over (≤ 0)
	payouts = `(SELECT COALESCE(SUM(x.subtotal),0) FROM sale_items x WHERE x.sale_id = s.id AND x.subtotal < 0)`
)

// raw sums the rows the way the rule book words each figure.
func (a *agreeing) raw() {
	a.s.must(a.t, a.s.db.GetContext(a.ctx, &a.count, `SELECT COUNT(*) FROM sales s WHERE `+saleIn, a.from, a.to))
	a.sum(&a.tax, `SELECT COALESCE(SUM(s.tax),0) FROM sales s WHERE `+saleIn, a.from, a.to)
	a.sum(&a.received, `SELECT COALESCE(SUM(s.paid_amount - s.change_given),0) FROM sales s WHERE `+saleIn, a.from, a.to)
	var sold, pt, soldCost decimal.Decimal
	a.sum(&sold, `SELECT COALESCE(SUM(s.total - s.tax),0) FROM sales s WHERE `+saleIn, a.from, a.to)
	a.sum(&pt, `SELECT COALESCE(SUM(si.subtotal),0) FROM sale_items si JOIN sales s ON s.id = si.sale_id
		JOIN products p ON p.id = si.product_id WHERE p.pass_through AND `+saleIn, a.from, a.to)
	a.sum(&soldCost, `SELECT COALESCE(SUM(si.quantity * si.cost_price),0) FROM sale_items si JOIN sales s ON s.id = si.sale_id
		JOIN products p ON p.id = si.product_id WHERE NOT p.pass_through AND `+saleIn, a.from, a.to)
	// RPT-6: a return on its own day, at the refund given before tax; its cost comes back.
	var retRev, retCost decimal.Decimal
	retFrom := ` FROM sale_return_items ri JOIN sale_returns r ON r.id = ri.return_id JOIN sales s ON s.id = r.sale_id
		JOIN sale_items si ON si.id = ri.sale_item_id JOIN products p ON p.id = ri.product_id WHERE `
	a.sum(&a.refunds, `SELECT COALESCE(SUM(ri.refund_amount),0)`+retFrom+retIn, a.from, a.to)
	a.sum(&retRev, `SELECT COALESCE(SUM(ri.refund_amount * COALESCE((s.total - s.tax - `+payouts+`) / NULLIF(s.total - `+payouts+`, 0), 1)),0)`+
		retFrom+`NOT p.pass_through AND `+retIn, a.from, a.to)
	a.sum(&retCost, `SELECT COALESCE(SUM(ri.quantity * si.cost_price),0)`+retFrom+`NOT p.pass_through AND `+retIn, a.from, a.to)
	a.sum(&a.taxBack, `SELECT COALESCE(SUM(ri.refund_amount * COALESCE(s.tax / NULLIF(s.total - `+payouts+`, 0), 0)),0)`+retFrom+retIn, a.from, a.to)
	a.revenue = sold.Sub(pt).Sub(retRev)
	a.cost = soldCost.Sub(retCost)

	var methods []struct {
		Method string          `db:"method"`
		Amount decimal.Decimal `db:"amount"`
	}
	a.s.must(a.t, a.s.db.SelectContext(a.ctx, &methods, `
		SELECT pm.method, SUM(pm.amount) - COALESCE(SUM(s.change_given) FILTER (WHERE pm.method = 'cash'),0) AS amount
		FROM payments pm JOIN sales s ON s.id = pm.sale_id WHERE `+saleIn+` GROUP BY pm.method`, a.from, a.to))
	a.byMethod = map[string]decimal.Decimal{}
	for _, m := range methods {
		a.byMethod[m.Method] = m.Amount
	}
}

// pl: the P&L (Finance) against the raw rows — RPT-1, 3, 6, 7, 8, 9.
func (a *agreeing) pl() {
	var f struct {
		SalesCount                                    int
		Returns, Revenue, COGS, GrossProfit, Received string
		Tax, Expenses, Losses, OwnUse, StaffWelfare   string
		StockCorrections, Recoveries, OtherIncome     string
		DiscountsReceived, RegisterDiff, NetProfit    string
		Receivables, CustomerAdvances, Payables       string
		SupplierCredit                                string
		Trend                                         []struct{ Revenue, Profit string }
		ByPayment                                     []struct{ Method, Total string }
		Categories                                    []struct{ Revenue, Profit string }
	}
	a.get("/api/v1/admin/finance"+a.q, &f)
	a.finRevenue, a.finProfit, a.finTax, a.finRecv = a.d(f.Revenue), a.d(f.GrossProfit), a.d(f.Tax), a.d(f.Received)
	if f.SalesCount != a.count {
		a.t.Errorf("RPT-1 %s sales count %d, want %d", a.q, f.SalesCount, a.count)
	}
	a.eq("RPT-6", "P&L revenue", a.finRevenue, a.revenue, cent)
	a.eq("RPT-6", "P&L cost", a.d(f.COGS), a.cost, cent)
	a.eq("RPT-1", "gross profit = revenue − cost", a.finProfit, a.finRevenue.Sub(a.d(f.COGS)), cent)
	a.eq("RPT-7", "received", a.finRecv, a.received, cent)
	a.eq("RPT-17", "P&L tax", a.finTax, a.tax, cent)
	got := decimal.Zero
	for _, m := range f.ByPayment {
		a.eq("RPT-7", "by method "+m.Method, a.d(m.Total), a.byMethod[m.Method], cent)
		if m.Method != "credit" { // on account is owed, not received (RPT-7)
			got = got.Add(a.d(m.Total))
		}
	}
	a.eq("RPT-32", "Σ by method (on account apart) = received", got, a.finRecv, halfCents(len(f.ByPayment)))

	var exp, loss, own, staff, corr, rec, interest, disc, diff decimal.Decimal
	a.sum(&exp, `SELECT COALESCE(SUM(amount),0) FROM expenses WHERE expense_date >= $1::date AND expense_date < $2::date`,
		a.from.Format("2006-01-02"), a.to.Format("2006-01-02"))
	mv := `SELECT COALESCE(SUM(cost),0) FROM stock_movements WHERE created_at >= $1 AND created_at < $2 AND `
	a.sum(&loss, mv+`type IN ('damage','warranty_replacement')`, a.from, a.to)
	a.sum(&own, mv+`type = 'own_use'`, a.from, a.to)
	a.sum(&staff, mv+`type = 'staff'`, a.from, a.to)
	a.sum(&corr, `SELECT COALESCE(SUM(CASE WHEN quantity < 0 THEN cost ELSE -cost END),0) FROM stock_movements
		WHERE type = 'adjust' AND created_at >= $1 AND created_at < $2`, a.from, a.to)
	a.sum(&rec, `SELECT COALESCE(SUM(recovered_value),0) FROM loss_recoveries WHERE created_at >= $1 AND created_at < $2`, a.from, a.to)
	a.sum(&interest, `SELECT COALESCE(SUM(balance_delta),0) FROM locker_ledger WHERE kind = 'interest' AND created_at >= $1 AND created_at < $2`, a.from, a.to)
	a.sum(&disc, `SELECT COALESCE(SUM(amount),0) FROM supplier_payments WHERE method = 'discount' AND created_at >= $1 AND created_at < $2`, a.from, a.to)
	a.sum(&diff, `SELECT COALESCE(SUM(difference),0) FROM cash_register WHERE closed_at >= $1 AND closed_at < $2`, a.from, a.to)
	for _, c := range []struct {
		what      string
		got, want decimal.Decimal
	}{{"expenses", a.d(f.Expenses), exp}, {"losses", a.d(f.Losses), loss}, {"own use", a.d(f.OwnUse), own},
		{"staff", a.d(f.StaffWelfare), staff}, {"stock corrections", a.d(f.StockCorrections), corr},
		{"recoveries", a.d(f.Recoveries), rec}, {"interest", a.d(f.OtherIncome), interest},
		{"discounts received", a.d(f.DiscountsReceived), disc}, {"over / short", a.d(f.RegisterDiff), diff}} {
		a.eq("RPT-9", c.what, c.got, c.want, cent)
	}
	net := a.finProfit.Sub(exp).Sub(loss).Sub(own).Sub(staff).Sub(corr).Add(rec).Add(interest).Add(disc).Add(diff)
	a.eq("RPT-9", "net profit", a.d(f.NetProfit), net, cent)
	a.finExpenses, a.finLosses, a.finRecoveries = a.d(f.Expenses), a.d(f.Losses), a.d(f.Recoveries)

	var owed, adv, pay, credit decimal.Decimal
	a.sum(&owed, `SELECT COALESCE(SUM(outstanding_balance),0) FROM customers WHERE outstanding_balance > 0`)
	a.sum(&adv, `SELECT COALESCE(SUM(-outstanding_balance),0) FROM customers WHERE outstanding_balance < 0`)
	a.sum(&pay, `SELECT COALESCE(SUM(outstanding_balance),0) FROM suppliers WHERE outstanding_balance > 0`)
	a.sum(&credit, `SELECT COALESCE(SUM(-outstanding_balance),0) FROM suppliers WHERE outstanding_balance < 0`)
	a.finReceivables, a.finAdvance, a.finPayables = a.d(f.Receivables), a.d(f.CustomerAdvances), a.d(f.Payables)
	a.eq("RPT-8", "receivables", a.finReceivables, owed, cent)
	a.eq("RPT-8", "customer advances", a.finAdvance, adv, cent)
	a.eq("RPT-8", "payables", a.finPayables, pay, cent)
	a.eq("RPT-8", "supplier credit", a.d(f.SupplierCredit), credit, cent)

	rev, prof := decimal.Zero, decimal.Zero
	for _, p := range f.Trend {
		rev, prof = rev.Add(a.d(p.Revenue)), prof.Add(a.d(p.Profit))
	}
	a.eq("RPT-10", "Σ Finance trend revenue", rev, a.finRevenue, halfCents(len(f.Trend)))
	a.eq("RPT-10", "Σ Finance trend profit", prof, a.finProfit, halfCents(len(f.Trend)))
	rev, prof = decimal.Zero, decimal.Zero
	for _, c := range f.Categories {
		rev, prof = rev.Add(a.d(c.Revenue)), prof.Add(a.d(c.Profit))
	}
	a.eq("RPT-12", "Σ Finance categories revenue", rev, a.finRevenue, halfCents(len(f.Categories)))
	a.eq("RPT-12", "Σ Finance categories profit", prof, a.finProfit, halfCents(len(f.Categories)))
}

// byTime: the Sales report, its trend and peak hours add up to the P&L (RPT-10, 30).
func (a *agreeing) byTime() {
	var sr struct {
		Sum struct {
			Count           int
			Revenue, Profit string
		}
		Trend []struct{ Revenue, Profit string }
	}
	a.get("/api/v1/admin/reports/sales"+a.q, &sr)
	if sr.Sum.Count != a.count {
		a.t.Errorf("RPT-37 %s Sales report count %d, want %d", a.q, sr.Sum.Count, a.count)
	}
	// The Sales report is the period's receipts, each less what came back on it so far (RPT-30).
	var receipts, back, soldCost, backCost decimal.Decimal
	a.sum(&receipts, `SELECT COALESCE(SUM(s.total - s.tax),0) - COALESCE((SELECT SUM(si.subtotal) FROM sale_items si
		JOIN sales s ON s.id = si.sale_id JOIN products p ON p.id = si.product_id WHERE p.pass_through AND `+saleIn+`),0)
		FROM sales s WHERE `+saleIn, a.from, a.to)
	onThem := ` FROM sale_return_items ri JOIN sale_returns r ON r.id = ri.return_id JOIN sales s ON s.id = r.sale_id
		JOIN sale_items si ON si.id = ri.sale_item_id WHERE ` + saleIn
	a.sum(&back, `SELECT COALESCE(SUM(ri.refund_amount * COALESCE((s.total - s.tax - `+payouts+`) / NULLIF(s.total - `+payouts+`, 0), 1)),0)`+onThem, a.from, a.to)
	a.sum(&backCost, `SELECT COALESCE(SUM(ri.quantity * si.cost_price),0)`+onThem, a.from, a.to)
	a.sum(&soldCost, `SELECT COALESCE(SUM(si.quantity * si.cost_price),0) FROM sale_items si JOIN sales s ON s.id = si.sale_id
		JOIN products p ON p.id = si.product_id WHERE NOT p.pass_through AND `+saleIn, a.from, a.to)
	a.eq("RPT-37", "Sales report revenue (its receipts)", a.d(sr.Sum.Revenue), receipts.Sub(back), cent)
	a.eq("RPT-37", "Sales report profit (its receipts)", a.d(sr.Sum.Profit), receipts.Sub(back).Sub(soldCost).Add(backCost), cent)
	rev := decimal.Zero
	for _, p := range sr.Trend {
		rev = rev.Add(a.d(p.Revenue))
	}
	a.eq("RPT-10", "Σ Sales report trend", rev, a.finRevenue, halfCents(len(sr.Trend)))

	var ph struct {
		Count   int
		Revenue string
		Cells   []struct{}
	}
	a.get("/api/v1/admin/reports/peak-hours"+a.q, &ph)
	if ph.Count != a.count {
		a.t.Errorf("RPT-30 %s peak hours count %d, want %d", a.q, ph.Count, a.count)
	}
	a.eq("RPT-30", "Σ peak hours", a.d(ph.Revenue), a.finRevenue, halfCents(len(ph.Cells)))
}

// lines: products and categories add up to the P&L (RPT-12).
func (a *agreeing) lines() {
	var tp struct {
		Revenue, Profit string
		Rows            []struct{ Revenue string }
	}
	a.get("/api/v1/admin/reports/top-products"+a.q+"&limit=200", &tp)
	a.eq("RPT-12", "product totals revenue", a.d(tp.Revenue), a.finRevenue, cent)
	a.eq("RPT-12", "product totals profit", a.d(tp.Profit), a.finProfit, cent)
	if len(tp.Rows) < 200 {
		rev := decimal.Zero
		for _, r := range tp.Rows {
			rev = rev.Add(a.d(r.Revenue))
		}
		a.eq("RPT-12", "Σ ranked products", rev, a.d(tp.Revenue), halfCents(len(tp.Rows)))
	}
	var cp struct{ Revenue, Profit string }
	a.get("/api/v1/admin/reports/profit-by-category"+a.q, &cp)
	a.eq("RPT-12", "categories revenue", a.d(cp.Revenue), a.finRevenue, cent)
	a.eq("RPT-12", "categories profit", a.d(cp.Profit), a.finProfit, cent)
}

// people: sales by cashier and the Returns page (RPT-14, 16).
func (a *agreeing) people() {
	var cr struct {
		Count int
		Net   string
		Rows  []struct{}
	}
	a.get("/api/v1/admin/reports/sales-by-cashier"+a.q, &cr)
	if cr.Count != a.count {
		a.t.Errorf("RPT-14 %s cashiers' count %d, want %d", a.q, cr.Count, a.count)
	}
	a.eq("RPT-14", "cashiers' net", a.d(cr.Net), a.finRevenue, halfCents(len(cr.Rows)))

	var rr struct{ Refund string }
	a.get("/api/v1/admin/reports/returns"+a.q, &rr)
	a.eq("RPT-16", "Returns page refunds", a.d(rr.Refund), a.refunds, cent)
}

// pages: tax, expenses, losses and dues (RPT-17, 20–25, 33, 34).
func (a *agreeing) pages() {
	var tx struct {
		Tax             string
		GivenBack, Owed *string
	}
	a.get("/api/v1/admin/reports/tax"+a.q, &tx)
	a.eq("RPT-30", "tax report = P&L tax", a.d(tx.Tax), a.finTax, cent)
	if tx.GivenBack == nil || tx.Owed == nil {
		a.t.Errorf("RPT-17 %s the tax report has no given back / owed", a.q)
	} else {
		a.eq("RPT-17", "tax given back", a.d(*tx.GivenBack), a.taxBack, cent)
		a.eq("RPT-17", "tax owed", a.d(*tx.Owed), a.tax.Sub(a.taxBack), cent)
	}

	var ex struct{ Total string }
	a.get("/api/v1/admin/reports/expenses"+a.q, &ex)
	a.eq("RPT-25", "expenses report = P&L", a.d(ex.Total), a.finExpenses, cent)

	var ls struct{ DamageCost, WarrantyCost, Recovered string }
	a.get("/api/v1/admin/reports/losses"+a.q, &ls)
	a.eq("RPT-33", "Losses page lost = P&L losses", a.d(ls.DamageCost).Add(a.d(ls.WarrantyCost)), a.finLosses, cent)
	a.eq("RPT-33", "Losses page recovered = P&L", a.d(ls.Recovered), a.finRecoveries, cent)

	var cd, sd struct{ Total string }
	a.get("/api/v1/admin/reports/customer-dues", &cd)
	a.get("/api/v1/admin/reports/supplier-dues", &sd)
	a.eq("RPT-20", "customer dues = receivables", a.d(cd.Total), a.finReceivables, cent)
	a.eq("RPT-34", "supplier dues = payables", a.d(sd.Total), a.finPayables, cent)

	if today := time.Now().In(datetime.Location).Format("2006-01-02"); a.q == "?from="+today+"&to="+today {
		var db struct {
			Today struct {
				Count           int
				Revenue, Profit string
			}
			Owed string
		}
		a.get("/api/v1/admin/dashboard", &db)
		if db.Today.Count != a.count {
			a.t.Errorf("RPT-30 dashboard today count %d, want %d", db.Today.Count, a.count)
		}
		a.eq("RPT-30", "dashboard today revenue", a.d(db.Today.Revenue), a.finRevenue, cent)
		a.eq("RPT-30", "dashboard today profit", a.d(db.Today.Profit), a.finProfit, cent)
		a.eq("RPT-8", "dashboard owed", a.d(db.Owed), a.finReceivables, cent)
	}
}

// zReports: every till session opened in range — ZR-1 (its sales), ZR-2 (the
// refunds given in it), ZR-3 (by method).
func (a *agreeing) zReports() {
	var sess []struct {
		ID     int64     `db:"id"`
		UserID int64     `db:"user_id"`
		Opened time.Time `db:"opened_at"`
		Closed time.Time `db:"closed_at"`
	}
	a.s.must(a.t, a.s.db.SelectContext(a.ctx, &sess, `SELECT id, user_id, opened_at, COALESCE(closed_at, now()) AS closed_at
		FROM cash_register WHERE opened_at >= $1 AND opened_at < $2`, a.from, a.to))
	for _, se := range sess {
		var z struct {
			SalesCount                    int
			Gross, Discount, Returns, Net string
			ByMethod                      []struct{ Method, Amount string }
		}
		a.get(fmt.Sprintf("/api/v1/till/z/%d", se.ID), &z)
		var n int
		var gross, disc, total, refunds decimal.Decimal
		a.s.must(a.t, a.s.db.GetContext(a.ctx, &n, `SELECT COUNT(*) FROM sales WHERE session_id = $1 AND status <> 'void'`, se.ID))
		a.sum(&gross, `SELECT COALESCE(SUM(subtotal),0) FROM sales WHERE session_id = $1 AND status <> 'void'`, se.ID)
		a.sum(&disc, `SELECT COALESCE(SUM(discount),0) FROM sales WHERE session_id = $1 AND status <> 'void'`, se.ID)
		a.sum(&total, `SELECT COALESCE(SUM(total),0) FROM sales WHERE session_id = $1 AND status <> 'void'`, se.ID)
		a.sum(&refunds, `SELECT COALESCE(SUM(ri.refund_amount),0) FROM sale_return_items ri JOIN sale_returns r ON r.id = ri.return_id
			JOIN sales s ON s.id = r.sale_id
			WHERE s.status <> 'void' AND r.created_by = $1 AND r.created_at >= $2 AND r.created_at <= $3`, se.UserID, se.Opened, se.Closed)
		ref := fmt.Sprintf("Z %d", se.ID)
		if z.SalesCount != n {
			a.t.Errorf("ZR-1 %s sales %d, want %d", ref, z.SalesCount, n)
		}
		a.eq("ZR-1", ref+" gross", a.d(z.Gross), gross, cent)
		a.eq("ZR-1", ref+" discount", a.d(z.Discount), disc, cent)
		a.eq("ZR-2", ref+" refunds given in it", a.d(z.Returns), refunds, cent)
		a.eq("ZR-2", ref+" net", a.d(z.Net), total.Sub(refunds), cent)
		var methods []struct {
			Method string          `db:"method"`
			Amount decimal.Decimal `db:"amount"`
		}
		a.s.must(a.t, a.s.db.SelectContext(a.ctx, &methods, `
			SELECT pm.method, SUM(pm.amount) - COALESCE(SUM(s.change_given) FILTER (WHERE pm.method = 'cash'),0) AS amount
			FROM payments pm JOIN sales s ON s.id = pm.sale_id WHERE s.session_id = $1 AND s.status <> 'void' GROUP BY pm.method`, se.ID))
		want := map[string]decimal.Decimal{}
		for _, m := range methods {
			want[m.Method] = m.Amount
		}
		for _, m := range z.ByMethod {
			a.eq("ZR-3", ref+" "+m.Method, a.d(m.Amount), want[m.Method], cent)
			delete(want, m.Method)
		}
		for m, v := range want {
			a.eq("ZR-3", ref+" missing "+m, decimal.Zero, v, cent)
		}
	}
}

// helperOf marks t's caller a helper when t can (testing.T can; ltest.T may not).
func helperOf(t ltest.T) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
}

// agreeToday runs agree over today (the machines' shop lives in one day).
func (s *shop) agreeToday(t ltest.T) {
	helperOf(t)
	today := time.Now().In(datetime.Location).Format("2006-01-02")
	s.agree(t, today, today)
}

// agreeSometimes runs agreeToday every 25th machine check: every step would add
// half an hour to a long run; each run also ends with one (area 7, Task 5).
func (s *shop) agreeSometimes(t ltest.T) {
	if s.checks++; s.checks%25 == 0 {
		s.agreeToday(t)
	}
}
