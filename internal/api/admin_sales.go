package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

func (h *handlers) registerAdminSales(g *echo.Group) {
	g.GET("/sales", h.AdminSales)
	g.GET("/sales/summary", h.AdminSalesSummary)
	g.GET("/sales/export", h.AdminSalesExport)
	g.POST("/sales/:id/return", h.AdminSaleReturn)
}

// salesFilter reads the list's filters (karots-pos salesFilterFromQuery):
// range, ?status=, ?method=, ?q= (receipt / customer / product, the till's
// word rules). No range = all time.
func salesFilter(c echo.Context) (sales.ListFilter, error) {
	from, to, err := listRange(c)
	return sales.ListFilter{From: from, To: to, Status: strings.TrimSpace(c.QueryParam("status")), Method: strings.TrimSpace(c.QueryParam("method")),
		Query: strings.TrimSpace(c.QueryParam("q"))}, err
}

// AdminSales pages the sales, newest first.
func (h *handlers) AdminSales(c echo.Context) error {
	f, err := salesFilter(c)
	if err != nil {
		return err
	}
	f.Limit, f.Offset = counterPage, listOffset(c)
	rows, err := h.d.Sales.List(c.Request().Context(), f)
	if err != nil {
		return err
	}
	out := make([]AdminSaleRow, 0, len(rows))
	for _, s := range rows {
		out = append(out, AdminSaleRow{SaleSummary: saleSummary(s), Paid: dec2(s.PaidAmount.Sub(s.ChangeGiven)), Profit: dec2(s.Profit)})
	}
	return c.JSON(http.StatusOK, out)
}

// AdminSalesSummary totals every match from SQL (sales.Summarize, the same
// WHERE as the list).
func (h *handlers) AdminSalesSummary(c echo.Context) error {
	f, err := salesFilter(c)
	if err != nil {
		return err
	}
	s, err := h.d.Sales.Summarize(c.Request().Context(), f)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, AdminSalesSummary{Count: s.Count, Total: dec2(s.Net), Paid: dec2(s.Paid), Profit: dec2(s.Profit), Margin: s.Margin().StringFixed(1)})
}

// AdminSalesExport downloads every match (csv / xlsx / ods), paging past one
// List call (karots-pos SalesExport).
func (h *handlers) AdminSalesExport(c echo.Context) error {
	f, err := salesFilter(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	f.Limit = sales.MaxListLimit
	header := []string{"Receipt", "When", "Cashier", "Customer", "Type", "Subtotal", "Discount", "Total", "Paid", "Change", "Profit", "Status"}
	var out [][]string
	for {
		rows, err := h.d.Sales.List(ctx, f)
		if err != nil {
			return err
		}
		for _, s := range rows {
			out = append(out, []string{s.ReceiptNo, s.CreatedAt.Format("2006-01-02 15:04"), s.CashierName, strOr(s.CustomerName), s.SaleType,
				dec2(s.Subtotal), dec2(s.Discount), dec2(s.Total), dec2(s.PaidAmount), dec2(s.ChangeGiven), dec2(s.Profit), s.Status})
		}
		if len(rows) < f.Limit {
			break
		}
		f.Offset += f.Limit
	}
	return writeSheet(c, "sales_"+time.Now().Format("2006-01-02"), header, out)
}

// AdminSaleReturn takes goods back from the admin (karots-pos SaleReturnSubmit):
// the counter's return, with the cash refund leaving from a picked locker or
// open till instead of the admin's own drawer.
func (h *handlers) AdminSaleReturn(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req AdminReturnRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := sales.PartialReturnInput{}
	if r := strings.TrimSpace(req.Reason); r != "" {
		in.Reason = &r
	}
	for _, l := range req.Lines {
		in.Lines = append(in.Lines, sales.ReturnLineInput{SaleItemID: l.SaleItemID, Quantity: l.Quantity, Disposition: l.Disposition})
	}
	if err := c.Validate(&in); err != nil {
		return err
	}
	ctx := c.Request().Context()
	var src cashflow.Location
	from := func() (cashflow.Location, error) {
		if strings.TrimSpace(req.Source) == "" {
			return src, apperr.Validation("pick where the refund cash comes from")
		}
		src, err = parseLocation(req.Source)
		return src, err
	}
	cash, rec, err := h.returnSale(ctx, id, in, from, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionReturn, "sale", strconv.FormatInt(id, 10), "return from the admin")
	out := AdminReturnResult{CashRefund: dec2(cash)}
	if rec != nil {
		out.ReceiptID = &rec.ID
		if src.Kind == cashflow.KindTill {
			h.d.KickDrawer(ctx)
		}
	}
	return c.JSON(http.StatusOK, out)
}
