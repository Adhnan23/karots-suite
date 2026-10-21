package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/datetime"
	"karots-suite/internal/escpos"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/warranty"
	"karots-suite/internal/middleware"
	"karots-suite/internal/receiptimg"

	"github.com/labstack/echo/v4"
)

// monthsLeft is karots-pos monthsLeftLabel: "11 mo left", "<1 mo left", "expired".
func monthsLeft(until time.Time) string {
	now := time.Now()
	if !until.After(now) {
		return "expired"
	}
	if m := int(until.Sub(now).Hours() / 24 / 30.4375); m >= 1 {
		return fmt.Sprintf("%d mo left", m)
	}
	return "<1 mo left"
}

func unitRow(u warranty.Unit) WarrantyUnitRow {
	return WarrantyUnitRow{ID: u.ID, Serial: u.SerialNo, ProductName: u.ProductName, CustomerName: strOr(u.CustomerName),
		ReceiptNo: strOr(u.ReceiptNo), SoldAt: u.SoldAt.Format(time.RFC3339), Months: u.WarrantyMonths,
		Until: u.WarrantyUntil.UTC().Format("2006-01-02"), Left: monthsLeft(u.WarrantyUntil), Status: u.Status, UnderWarranty: u.UnderWarranty()}
}

func claimRow(cl warranty.Claim) WarrantyClaimRow {
	return WarrantyClaimRow{ID: cl.ID, At: cl.CreatedAt.Format(time.RFC3339), Reason: strOr(cl.Reason), Resolution: cl.Resolution,
		ProductName: cl.ProductName, OldSerial: cl.OldSerial, ReplacementSerial: strOr(cl.ReplacementSerial),
		CustomerName: strOr(cl.CustomerName), HandledBy: cl.HandledByName}
}

// CounterWarrantyUnits lists serial-tracked units (?status=all|active|expired|replaced&q=).
func (h *handlers) CounterWarrantyUnits(c echo.Context) error {
	rows, err := h.t.warranty.List(c.Request().Context(), c.QueryParam("status"), strings.TrimSpace(c.QueryParam("q")), counterPage, listOffset(c))
	if err != nil {
		return err
	}
	out := make([]WarrantyUnitRow, 0, len(rows))
	for _, u := range rows {
		out = append(out, unitRow(u))
	}
	return c.JSON(http.StatusOK, out)
}

// CounterWarrantySerial looks one serial up with its claim history.
func (h *handlers) CounterWarrantySerial(c echo.Context) error {
	d, err := h.t.warranty.Lookup(c.Request().Context(), c.Param("serial"))
	if err != nil {
		return err
	}
	v := WarrantyUnitView{WarrantyUnitRow: unitRow(d.Unit), Claims: []WarrantyClaimRow{}}
	for _, cl := range d.Claims {
		v.Claims = append(v.Claims, claimRow(cl))
	}
	return c.JSON(http.StatusOK, v)
}

// CounterWarrantyReceipt finds a sale's warranted non-serial lines by receipt no.
func (h *handlers) CounterWarrantyReceipt(c echo.Context) error {
	d, err := h.t.warranty.LookupByReceipt(c.Request().Context(), strings.TrimSpace(c.QueryParam("no")))
	if err != nil {
		return err
	}
	v := WarrantyReceiptView{SaleID: d.Sale.SaleID, ReceiptNo: d.Sale.ReceiptNo, SoldAt: d.Sale.SoldAt.Format(time.RFC3339),
		CustomerName: strOr(d.Sale.CustomerName), Lines: []WarrantyReceiptLine{}}
	for _, l := range d.Lines {
		v.Lines = append(v.Lines, WarrantyReceiptLine{ProductID: l.ProductID, Name: l.ProductName, UnitAbbr: l.UnitAbbr, Qty: l.Qty.String(),
			Replaced: l.Replaced, Remaining: l.Remaining.String(), Months: l.WarrantyMonths,
			Until: l.WarrantyUntil.UTC().Format("2006-01-02"), UnderWarranty: l.UnderWarranty})
	}
	return c.JSON(http.StatusOK, v)
}

// CounterWarrantyReplace swaps a faulty serial for a new one from stock.
func (h *handlers) CounterWarrantyReplace(c echo.Context) error {
	var req WarrantyReplaceRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	ctx := c.Request().Context()
	old, err := h.t.warranty.GetUnit(ctx, req.UnitID)
	if err != nil {
		return err
	}
	nu, claimID, err := h.t.warranty.RecordReplacement(ctx, req.UnitID, req.NewSerial, req.Reason, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "warranty", fmt.Sprint(req.UnitID), "replaced "+old.SerialNo+" -> "+nu.SerialNo)
	return c.JSON(http.StatusCreated, WarrantyClaimRef{ClaimID: claimID})
}

// CounterWarrantyReceiptReplace records a like-for-like swap for a non-serial line.
func (h *handlers) CounterWarrantyReceiptReplace(c echo.Context) error {
	var req WarrantyReceiptReplaceRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	claimID, err := h.t.warranty.RecordReceiptReplacement(c.Request().Context(), req.SaleID, req.ProductID, req.Reason, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "warranty", fmt.Sprint(claimID), "receipt warranty replacement")
	return c.JSON(http.StatusCreated, WarrantyClaimRef{ClaimID: claimID})
}

// CounterWarrantyClaims lists claims for the Receipts hub (?q= serial / customer).
func (h *handlers) CounterWarrantyClaims(c echo.Context) error {
	from, to, err := listRange(c)
	if err != nil {
		return err
	}
	rows, err := h.t.warranty.ListClaims(c.Request().Context(), warranty.ClaimFilter{Search: strings.TrimSpace(c.QueryParam("q")), From: from, To: to, Limit: counterPage, Offset: listOffset(c)})
	if err != nil {
		return err
	}
	out := make([]WarrantyClaimRow, 0, len(rows))
	for _, cl := range rows {
		out = append(out, claimRow(cl))
	}
	return c.JSON(http.StatusOK, out)
}

// CounterPrintWarrantySlip prints a serial replacement's slip.
func (h *handlers) CounterPrintWarrantySlip(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	cl, err := h.t.warranty.GetClaim(ctx, id)
	if err != nil {
		return err
	}
	if cl.ReplacementUnitID == nil {
		return apperr.Validation("this claim has no replacement slip")
	}
	u, err := h.t.warranty.GetUnit(ctx, *cl.ReplacementUnitID)
	if err != nil {
		return err
	}
	cfg, err := h.slipSettings(c)
	if err != nil {
		return err
	}
	slip := escpos.WarrantySlip{ProductName: u.ProductName, OldSerial: cl.OldSerial, NewSerial: u.SerialNo,
		WarrantyUntil: datetime.Date(u.WarrantyUntil), WarrantyLeft: monthsLeft(u.WarrantyUntil), CustomerName: strOr(u.CustomerName)}
	payload := escpos.WarrantyDocument(slip, *cfg, receiptimg.SlipOptions(ctx, cfg, nil))
	if ok, err := previewSlip(c, cfg, payload); ok {
		return err
	}
	if job, ok, err := slipImage(c, cfg); err != nil {
		return err
	} else if ok {
		payload = job
	}
	if err := escpos.Send(ctx, h.receiptQueue(c, cfg), payload); err != nil {
		log.Printf("warranty slip %d: %v", id, err)
		return apperr.ConflictCode("PRINT_FAILED", printFailed)
	}
	return c.NoContent(http.StatusNoContent)
}
