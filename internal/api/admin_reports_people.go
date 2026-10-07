package api

import (
	"net/http"
	"strconv"
	"time"

	"karots-suite/internal/datetime"
	"karots-suite/internal/features/customers"
	"karots-suite/internal/features/purchases"
	"karots-suite/internal/features/reports"
	"karots-suite/internal/features/suppliers"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerReportsPeople(g *echo.Group) {
	g.GET("/reports/returns", h.ReportReturns)
	g.GET("/reports/returns/lines", h.ReportReturnLines)
	g.GET("/reports/customer-dues", h.ReportCustomerDues)
	g.GET("/reports/supplier-dues", h.ReportSupplierDues)
	g.GET("/reports/supplier-spend", h.ReportSupplierSpend)
	g.GET("/reports/purchases", h.ReportPurchases)
}

// ReportReturns is the Returns report's head (karots-pos Returns / Refunds),
// by the date the money went back. ?format= downloads every line.
func (h *handlers) ReportReturns(c echo.Context) error {
	ctx := c.Request().Context()
	from, to, fs, ts, err := reportRange(c)
	if err != nil {
		return err
	}
	rep := reports.NewService(h.d.DB)
	if wantsSheet(c) {
		lines, err := rep.ReturnLines(ctx, from, to, 0, 0)
		if err != nil {
			return err
		}
		out := make([][]string, 0, len(lines))
		for _, l := range lines {
			out = append(out, []string{datetime.DateTime(l.When), l.ReceiptNo, datetime.Date(l.SaleDate), l.Product, l.Qty.String(), dec2(l.Refund), strOr(l.Customer), strOr(l.Reason)})
		}
		return writeSheet(c, "returns_"+fs+"_"+ts, []string{"Returned", "Receipt", "Sold", "Product", "Qty", "Refund", "Customer", "Reason"}, out)
	}
	tot, err := rep.ReturnsIn(ctx, from, to)
	if err != nil {
		return err
	}
	top, err := rep.MostReturned(ctx, from, to, 10)
	if err != nil {
		return err
	}
	out := ReturnsReport{From: fs, To: ts, Returns: tot.Returns, Receipts: tot.Receipts, Qty: tot.Qty.String(), Refund: dec2(tot.Refund), Top: make([]ReturnTop, 0, len(top))}
	for _, p := range top {
		out.Top = append(out.Top, ReturnTop{Name: p.Name, Qty: p.Qty.String(), Refund: dec2(p.Refund)})
	}
	return c.JSON(http.StatusOK, out)
}

// ReportReturnLines pages the returned lines, newest first.
func (h *handlers) ReportReturnLines(c echo.Context) error {
	from, to, _, _, err := reportRange(c)
	if err != nil {
		return err
	}
	lines, err := reports.NewService(h.d.DB).ReturnLines(c.Request().Context(), from, to, counterPage, listOffset(c))
	if err != nil {
		return err
	}
	out := make([]ReturnLineRow, 0, len(lines))
	for _, l := range lines {
		out = append(out, ReturnLineRow{When: l.When.Format(time.RFC3339), SaleID: l.SaleID, ReceiptNo: l.ReceiptNo, SaleDate: datetime.Date(l.SaleDate),
			Product: l.Product, Qty: l.Qty.String(), Refund: dec2(l.Refund), Customer: strOr(l.Customer), Reason: strOr(l.Reason)})
	}
	return c.JSON(http.StatusOK, out)
}

// dues ages each row by its oldest unpaid date and totals the buckets.
// ponytail: whole-balance aging by the oldest date (karots-pos), not per bill —
// per-bill buckets if a shop mixes very old and fresh debt on one account.
func dues(rows []DueRow, balances []decimal.Decimal) DuesReport {
	out := DuesReport{Count: len(rows), Rows: rows}
	var total, flagged decimal.Decimal
	var b [4]decimal.Decimal
	for i, r := range rows {
		total = total.Add(balances[i])
		if r.Flag {
			out.Flagged++
			flagged = flagged.Add(balances[i])
		}
		out.OldestDays = max(out.OldestDays, r.AgeDays)
		switch {
		case r.AgeDays <= 30:
			b[0] = b[0].Add(balances[i])
		case r.AgeDays <= 60:
			b[1] = b[1].Add(balances[i])
		case r.AgeDays <= 90:
			b[2] = b[2].Add(balances[i])
		default:
			b[3] = b[3].Add(balances[i])
		}
	}
	out.Total, out.FlaggedSum = dec2(total), dec2(flagged)
	for i := range b {
		out.Buckets[i] = dec2(b[i])
	}
	return out
}

// ageOf is whole shop-time days since t (0 when unknown).
func ageOf(t *time.Time) (string, int) {
	if t == nil {
		return "", 0
	}
	return datetime.Date(*t), max(int(time.Since(*t).Hours()/24), 0)
}

func duesSheet(c echo.Context, name, terms string, d DuesReport) error {
	out := make([][]string, 0, len(d.Rows))
	for _, r := range d.Rows {
		out = append(out, []string{r.Name, r.Phone, r.Terms, r.Balance, r.Oldest, strconv.Itoa(r.AgeDays)})
	}
	return writeSheet(c, name, []string{"Name", "Phone", terms, "Outstanding", "Oldest unpaid", "Age (days)"}, out)
}

// ReportCustomerDues is who owes us (karots-pos Customer Dues). Advances are
// not dues; a deactivated debtor still is.
func (h *handlers) ReportCustomerDues(c echo.Context) error {
	rows, err := customers.NewService(h.d.DB).Owing(c.Request().Context())
	if err != nil {
		return err
	}
	out, bal := make([]DueRow, 0, len(rows)), make([]decimal.Decimal, 0, len(rows))
	for _, r := range rows {
		oldest, age := ageOf(r.OldestCredit)
		over := r.CreditLimit.IsPositive() && r.OutstandingBalance.GreaterThan(r.CreditLimit)
		out = append(out, DueRow{ID: r.ID, Name: r.Name, Phone: strOr(r.Phone), Balance: dec2(r.OutstandingBalance), Terms: dec2(r.CreditLimit),
			Oldest: oldest, AgeDays: age, Flag: over, Disabled: !r.IsActive})
		bal = append(bal, r.OutstandingBalance)
	}
	d := dues(out, bal)
	if wantsSheet(c) {
		return duesSheet(c, "customer_dues_"+datetime.Date(time.Now()), "Credit limit", d)
	}
	return c.JSON(http.StatusOK, d)
}

// ReportSupplierDues is who we owe (karots-pos Supplier Dues); overdue = the
// oldest unpaid bill is past the supplier's credit days.
func (h *handlers) ReportSupplierDues(c echo.Context) error {
	rows, err := suppliers.NewService(h.d.DB).Owing(c.Request().Context())
	if err != nil {
		return err
	}
	out, bal := make([]DueRow, 0, len(rows)), make([]decimal.Decimal, 0, len(rows))
	for _, r := range rows {
		oldest, age := ageOf(r.OldestUnpaid)
		out = append(out, DueRow{ID: r.ID, Name: r.Name, Phone: strOr(r.Phone), Balance: dec2(r.OutstandingBalance), Terms: strconv.Itoa(r.CreditDays),
			Oldest: oldest, AgeDays: age, Flag: r.CreditDays > 0 && r.OldestUnpaid != nil && age > r.CreditDays, Disabled: !r.IsActive})
		bal = append(bal, r.OutstandingBalance)
	}
	d := dues(out, bal)
	if wantsSheet(c) {
		return duesSheet(c, "supplier_dues_"+datetime.Date(time.Now()), "Credit days", d)
	}
	return c.JSON(http.StatusOK, d)
}

// ReportSupplierSpend ranks suppliers by what was bought from them in range,
// net of goods sent back.
func (h *handlers) ReportSupplierSpend(c echo.Context) error {
	from, to, fs, ts, err := reportRange(c)
	if err != nil {
		return err
	}
	rows, err := reports.NewService(h.d.DB).SupplierSpend(c.Request().Context(), from, to)
	if err != nil {
		return err
	}
	out := SupplierSpendReport{From: fs, To: ts, Rows: make([]SpendRow, 0, len(rows))}
	var spend, paid, due decimal.Decimal
	for _, r := range rows {
		out.Rows = append(out.Rows, SpendRow{SupplierID: r.SupplierID, Supplier: r.Supplier, Orders: r.Orders, Spend: dec2(r.Spend), Paid: dec2(r.Paid), Due: dec2(r.Due)})
		out.Orders += r.Orders
		spend, paid, due = spend.Add(r.Spend), paid.Add(r.Paid), due.Add(r.Due)
	}
	if wantsSheet(c) {
		sheet := make([][]string, 0, len(out.Rows))
		for _, r := range out.Rows {
			sheet = append(sheet, []string{r.Supplier, strconv.Itoa(r.Orders), r.Spend, r.Paid, r.Due})
		}
		return writeSheet(c, "supplier_spend_"+fs+"_"+ts, []string{"Supplier", "Orders", "Spend", "Paid", "Due"}, sheet)
	}
	out.Spend, out.Paid, out.Due = dec2(spend), dec2(paid), dec2(due)
	return c.JSON(http.StatusOK, out)
}

// ReportPurchases is the deliveries received in range (karots-pos Purchases;
// drafts — orders not yet received — are not purchases). A delivery's due is
// after payments and goods credited back.
func (h *handlers) ReportPurchases(c echo.Context) error {
	from, to, fs, ts, err := reportRange(c)
	if err != nil {
		return err
	}
	rows, err := purchases.NewService(h.d.DB).ListBetween(c.Request().Context(), from, to)
	if err != nil {
		return err
	}
	out := PurchasesReport{From: fs, To: ts, Count: len(rows), Rows: make([]PurchaseReportRow, 0, len(rows))}
	var total, paid, cred, due decimal.Decimal
	for _, p := range rows {
		out.Rows = append(out.Rows, PurchaseReportRow{ID: p.ID, When: p.CreatedAt.Format(time.RFC3339), Supplier: p.SupplierName, Invoice: strOr(p.InvoiceNo),
			Total: dec2(p.Total), Paid: dec2(p.PaidAmount), Credited: dec2(p.CreditedAmount), Due: dec2(p.Balance())})
		total, paid, cred, due = total.Add(p.Total), paid.Add(p.PaidAmount), cred.Add(p.CreditedAmount), due.Add(p.Balance())
	}
	if wantsSheet(c) {
		sheet := make([][]string, 0, len(rows))
		for _, p := range rows {
			sheet = append(sheet, []string{datetime.Date(p.CreatedAt), p.SupplierName, strOr(p.InvoiceNo), dec2(p.Total), dec2(p.PaidAmount), dec2(p.CreditedAmount), dec2(p.Balance())})
		}
		return writeSheet(c, "purchases_"+fs+"_"+ts, []string{"Date", "Supplier", "Invoice", "Total", "Paid", "Returned", "Due"}, sheet)
	}
	out.Total, out.Paid, out.Credited, out.Due = dec2(total), dec2(paid), dec2(cred), dec2(due)
	return c.JSON(http.StatusOK, out)
}
