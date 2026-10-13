package api

import (
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/conversions"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// CounterConversions lists active pack-breaking recipes with on-hand stock.
func (h *handlers) CounterConversions(c echo.Context) error {
	rows, err := conversions.NewService(h.d.DB).List(c.Request().Context(), c.QueryParam("search"))
	if err != nil {
		return err
	}
	out := make([]ConversionRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, ConversionRow{ID: r.ID, FromName: r.FromName, ToName: r.ToName, FromUnit: r.FromUnitAbbr,
			ToUnit: r.ToUnitAbbr, Ratio: r.Ratio.String(), FromStock: r.FromStock.String(), ToStock: r.ToStock.String(), Note: strOr(r.Note)})
	}
	return c.JSON(http.StatusOK, out)
}

// CounterConversionRun breaks N source units into Ratio×N target units.
func (h *handlers) CounterConversionRun(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req RunConversionRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	qty, err := money.Parse(req.Quantity)
	if err != nil || !qty.IsPositive() {
		return apperr.Validation("quantity is invalid")
	}
	got := decimal.Zero
	if strings.TrimSpace(req.ToQty) != "" {
		if got, err = money.Parse(req.ToQty); err != nil || !got.IsPositive() {
			return apperr.Validation("the amount you got is invalid")
		}
	}
	if err := conversions.NewService(h.d.DB).Run(c.Request().Context(), id, qty, got, middleware.CurrentUserID(c)); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "conversion", strconv.FormatInt(id, 10), "ran conversion ("+qty.String()+gotNote(got)+")")
	return c.NoContent(http.StatusNoContent)
}

func gotNote(got decimal.Decimal) string {
	if got.IsZero() {
		return ""
	}
	return " → got " + got.String()
}
