package api

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/escpos"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/reports"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/middleware"
	"karots-suite/internal/receiptimg"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func saleSummary(s sales.Sale) SaleSummary {
	return SaleSummary{ID: s.ID, ReceiptNo: s.ReceiptNo, CreatedAt: s.CreatedAt.Format(time.RFC3339),
		CustomerName: strOr(s.CustomerName), CashierName: s.CashierName, Total: dec2(s.Total), Status: s.Status}
}

func saleView(d *sales.Detail) SaleView {
	s := d.Sale
	v := SaleView{SaleSummary: saleSummary(s), Subtotal: dec2(s.Subtotal), Discount: dec2(s.Discount), Tax: dec2(s.Tax),
		PaidAmount: dec2(s.PaidAmount), ChangeGiven: dec2(s.ChangeGiven), OwedOnSale: "0.00", Lines: []SaleLine{}, Payments: []SalePayment{}}
	if s.CustomerID != nil {
		v.CustomerID = *s.CustomerID
		if owed := s.Total.Sub(s.PaidAmount).Sub(d.CreditReduced); owed.IsPositive() {
			v.OwedOnSale = dec2(owed)
		}
	}
	for _, it := range d.Items {
		// Valued exactly as sales.PartialReturnTx refunds: part of a line is
		// paid × qty / quantity, the rest of it is what's left of paid.
		paid, unit := it.PaidValue(d.PaidFactor), decimal.Zero
		if it.Quantity.IsPositive() {
			unit = paid.Div(it.Quantity).Round(2)
		}
		name := it.ProductName
		if it.Description != nil && *it.Description != "" {
			name = *it.Description
		}
		line := SaleLine{ID: it.ID, Name: name, UnitAbbr: it.UnitAbbr, Quantity: it.Quantity.String(),
			ReturnedQty: it.ReturnedQty.String(), UnitPrice: dec2(it.UnitPrice), Subtotal: dec2(it.Subtotal), RefundUnit: dec2(unit), Paid: dec2(paid), RefundLeft: dec2(decimal.Max(paid.Sub(it.Refunded), decimal.Zero)), IsService: it.IsService}
		if it.LineGroup != nil && it.LineGroupLabel != nil && it.LineGroupSets != nil {
			line.Group, line.GroupLabel, line.GroupSets = *it.LineGroup, *it.LineGroupLabel, it.LineGroupSets.String()
		}
		v.Lines = append(v.Lines, line)
	}
	for _, p := range d.Payments {
		v.Payments = append(v.Payments, SalePayment{Method: p.Method, Amount: dec2(p.Amount)})
	}
	return v
}

// listRange reads a list's period: ?preset=today|this-week|last-week|this-month|
// last-month|this-year, or ?from=&to= (YYYY-MM-DD, inclusive), via the forked
// reports.ResolveRange (karots-pos resolveReceiptRange). Nothing = unbounded
// (the latest rows).
func listRange(c echo.Context) (*time.Time, *time.Time, error) {
	preset, from, to := c.QueryParam("preset"), c.QueryParam("from"), c.QueryParam("to")
	if preset == "" && from == "" && to == "" {
		return nil, nil, nil
	}
	f, t, _, _, err := reports.ResolveRange(preset, from, to)
	if err != nil {
		return nil, nil, apperr.Validation(err.Error())
	}
	return &f, &t, nil
}

// CounterSales finds sales by receipt no / customer name / phone / product sold.
func (h *handlers) CounterSales(c echo.Context) error {
	from, to, err := listRange(c)
	if err != nil {
		return err
	}
	rows, err := h.d.Sales.List(c.Request().Context(), sales.ListFilter{Query: strings.TrimSpace(c.QueryParam("q")), From: from, To: to, Limit: counterPage, Offset: listOffset(c)})
	if err != nil {
		return err
	}
	out := make([]SaleSummary, 0, len(rows))
	for _, s := range rows {
		out = append(out, saleSummary(s))
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) CounterSale(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	d, err := h.d.Sales.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, saleView(d))
}

// CounterReturn takes goods back: restock (or damage) + customer-credit
// reduction in sales.PartialReturnTx, and the cash part leaves the drawer through
// cashflow in the SAME tx (karots-pos cashier ReturnSubmit).
func (h *handlers) CounterReturn(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req ReturnRequest
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
	userID := middleware.CurrentUserID(c)
	cash, _, err := h.returnSale(ctx, id, in, func() (cashflow.Location, error) { return cashflow.Till(userID), nil }, userID)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionReturn, "sale", strconv.FormatInt(id, 10), "partial return")
	if cash.IsPositive() && !req.DeferDrawerKick {
		h.d.KickDrawer(ctx)
	}
	return c.JSON(http.StatusOK, ReturnResult{CashRefund: dec2(cash)})
}

// returnSale takes goods back (sales.PartialReturnTx: restock or damage, a
// credit sale's debt cleared first) and pays the cash part out of from() in
// the SAME tx (karots-pos refundCashTx). from is asked only when there is cash
// to refund. Returns the cash refunded and its CR- receipt (nil when none).
func (h *handlers) returnSale(ctx context.Context, id int64, in sales.PartialReturnInput, from func() (cashflow.Location, error), userID int64) (decimal.Decimal, *cashflow.Receipt, error) {
	cash := decimal.Zero
	var rec *cashflow.Receipt
	err := appdb.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		d, refund, returnID, err := h.d.Sales.PartialReturnTx(ctx, tx, id, in, userID)
		if err != nil {
			return err
		}
		cash = refund
		if !refund.IsPositive() {
			return nil
		}
		src, err := from()
		if err != nil {
			return err
		}
		rec, err = h.d.Cashflow.MoveTx(ctx, tx, cashflow.MoveInput{From: src, To: cashflow.External(), Amount: refund,
			Reason: "cash refund: " + d.Sale.ReceiptNo, ReceiptKind: "refund", Party: strOr(d.Sale.CustomerName),
			Ref: &cashflow.Ref{Kind: "sale_return", ID: returnID}, ActorID: userID})
		return err
	})
	return cash, rec, err
}

// CounterPrintRefund prints the latest return's refund slip. ?kick=1 folds the
// drawer pulse in (auto-print right after a cash refund).
func (h *handlers) CounterPrintRefund(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	rr, err := h.d.Sales.ReturnReceipt(ctx, id)
	if err != nil {
		return err
	}
	cfg, err := h.slipSettings(c)
	if err != nil {
		return err
	}
	payload := escpos.ReturnDocument(*rr, *cfg, receiptimg.SlipOptions(ctx, cfg, nil))
	if ok, err := previewSlip(c, cfg, payload); ok {
		return err
	}
	if c.QueryParam("kick") == "1" {
		payload = append(escpos.DrawerKick(*cfg), payload...)
	}
	if err := escpos.Send(ctx, h.receiptQueue(c, cfg), payload); err != nil {
		log.Printf("refund slip for sale %d: %v", id, err)
		return apperr.ConflictCode("PRINT_FAILED", printFailed)
	}
	return c.NoContent(http.StatusNoContent)
}
