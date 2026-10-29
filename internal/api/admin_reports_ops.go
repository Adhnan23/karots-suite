package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/datetime"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/recipes"
	"karots-suite/internal/features/recovery"
	"karots-suite/internal/features/reports"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerReportsOps(g *echo.Group) {
	g.GET("/reports/recipe-variance", h.ReportRecipeVariance)
	g.GET("/reports/service-profit", h.ReportServiceProfit)
	g.GET("/reports/losses", h.ReportLosses)
	g.GET("/reports/losses/lines", h.ReportLossLines)
	g.POST("/recovery", h.AdminRecovery)
}

// ReportRecipeVariance is expected vs actual ingredient use (karots-pos Recipe
// Variance): a yield is an estimate, this shows the drift.
func (h *handlers) ReportRecipeVariance(c echo.Context) error {
	from, to, fs, ts, err := reportRange(c)
	if err != nil {
		return err
	}
	rows, err := recipes.NewService(h.d.DB).Variance(c.Request().Context(), from, to)
	if err != nil {
		return err
	}
	out := RecipeVarianceReport{From: fs, To: ts, Rows: make([]VarianceLine, 0, len(rows))}
	ten := decimal.NewFromInt(10)
	for _, r := range rows {
		if r.DriftPct().Abs().GreaterThan(ten) {
			out.Drifting++
		}
		out.Rows = append(out.Rows, VarianceLine{Ingredient: r.ComponentName, Unit: r.UnitAbbr, Expected: r.Expected.String(), Actual: r.Actual.String(),
			Diff: r.Diff.String(), Drift: r.DriftPct().StringFixed(1)})
	}
	if wantsSheet(c) {
		sheet := make([][]string, 0, len(out.Rows))
		for _, r := range out.Rows {
			sheet = append(sheet, []string{r.Ingredient, r.Unit, r.Expected, r.Actual, r.Diff, r.Drift})
		}
		return writeSheet(c, "recipe_variance_"+fs+"_"+ts, []string{"Ingredient", "Unit", "Expected", "Actual", "Difference", "Drift %"}, sheet)
	}
	return c.JSON(http.StatusOK, out)
}

// ReportServiceProfit is per-service takings less ingredients and expenses.
func (h *handlers) ReportServiceProfit(c echo.Context) error {
	from, to, fs, ts, err := reportRange(c)
	if err != nil {
		return err
	}
	rows, err := reports.NewService(h.d.DB).ServiceProfit(c.Request().Context(), from, to)
	if err != nil {
		return err
	}
	out := ServiceProfitReport{From: fs, To: ts, Rows: make([]ServiceLine, 0, len(rows))}
	var rev, exp, net decimal.Decimal
	for _, r := range rows {
		out.Rows = append(out.Rows, ServiceLine{ProductID: r.ProductID, Name: r.Name, Units: r.Units.String(), Revenue: dec2(r.Revenue), COGS: dec2(r.COGS),
			Gross: dec2(r.GrossProfit()), Expenses: dec2(r.Expenses), Net: dec2(r.NetProfit()), Margin: r.MarginPct().StringFixed(1)})
		rev, exp, net = rev.Add(r.Revenue), exp.Add(r.Expenses), net.Add(r.NetProfit())
	}
	if wantsSheet(c) {
		sheet := make([][]string, 0, len(out.Rows))
		for _, r := range out.Rows {
			sheet = append(sheet, []string{r.Name, r.Units, r.Revenue, r.COGS, r.Gross, r.Expenses, r.Net, r.Margin})
		}
		return writeSheet(c, "service_profit_"+fs+"_"+ts, []string{"Service", "Sold", "Income", "Ingredients", "Gross", "Expenses", "Net profit", "Margin %"}, sheet)
	}
	out.Revenue, out.Expenses, out.Net = dec2(rev), dec2(exp), dec2(net)
	return c.JSON(http.StatusOK, out)
}

// ReportLosses is Losses & Recovery's head:
// damage and warranty losses in range against supplier recoveries. ?format=
// downloads every loss in range.
func (h *handlers) ReportLosses(c echo.Context) error {
	ctx := c.Request().Context()
	from, to, fs, ts, err := reportRange(c)
	if err != nil {
		return err
	}
	if wantsSheet(c) {
		rows, err := recovery.NewService(h.d.DB).LossesIn(ctx, from, to, 0, 0)
		if err != nil {
			return err
		}
		sheet := make([][]string, 0, len(rows))
		for _, l := range rows {
			sheet = append(sheet, []string{datetime.DateTime(l.At), l.Kind, l.Product, l.Quantity.String(), dec2(l.LossValue), strOr(l.Outcome), dec2(l.Recovered), strOr(l.Note), l.By})
		}
		return writeSheet(c, "losses_"+fs+"_"+ts, []string{"When", "Kind", "Product", "Qty", "Lost", "Recovery", "Recovered", "Note", "By"}, sheet)
	}
	sum, err := reports.NewService(h.d.DB).WarrantyAndRecovery(ctx, from, to)
	if err != nil {
		return err
	}
	out := LossesReport{From: fs, To: ts, DamageCount: sum.DamageCount, DamageCost: dec2(sum.DamageCost), WarrantyCount: sum.ReplacementCount,
		WarrantyCost: dec2(sum.ReplacementCost), Recovered: dec2(sum.RecoveredValue),
		Net: dec2(sum.DamageCost.Add(sum.ReplacementCost).Sub(sum.RecoveredValue)), Outcomes: make([]RecoveryOutcome, 0, len(sum.Recoveries))}
	for _, r := range sum.Recoveries {
		out.Outcomes = append(out.Outcomes, RecoveryOutcome{Outcome: r.Outcome, Count: r.Count, LossValue: dec2(r.LossValue), Recovered: dec2(r.Recovered)})
	}
	return c.JSON(http.StatusOK, out)
}

// ReportLossLines pages the losses in range, newest first.
func (h *handlers) ReportLossLines(c echo.Context) error {
	from, to, _, _, err := reportRange(c)
	if err != nil {
		return err
	}
	rows, err := recovery.NewService(h.d.DB).LossesIn(c.Request().Context(), from, to, counterPage, listOffset(c))
	if err != nil {
		return err
	}
	out := make([]LossRow, 0, len(rows))
	for _, l := range rows {
		r := LossRow{Kind: l.Kind, ID: l.MovementID, ProductID: l.ProductID, Product: l.Product, Qty: l.Quantity.String(), Loss: dec2(l.LossValue), Note: strOr(l.Note), By: l.By,
			At: l.At.Format(time.RFC3339), Outcome: strOr(l.Outcome), Recovered: dec2(l.Recovered)}
		if l.SourceID != nil {
			r.SourceID = *l.SourceID
		}
		out = append(out, r)
	}
	return c.JSON(http.StatusOK, out)
}

// AdminRecovery books what a supplier made good on a loss (karots-pos
// RecoveryRecord), audited. Recovering the same loss twice is a 409.
func (h *handlers) AdminRecovery(c echo.Context) error {
	var req RecoveryRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := recovery.CreateInput{SourceType: req.SourceType, SourceID: req.SourceID, Outcome: req.Outcome, RecoveredAmount: strings.TrimSpace(req.Amount), Note: strings.TrimSpace(req.Note),
		BatchID: req.BatchID, Expiry: strings.TrimSpace(req.Expiry), ExtraPerUnit: strings.TrimSpace(req.ExtraPerUnit), Price: strings.TrimSpace(req.SellingPrice)}
	if req.SupplierID > 0 {
		in.SupplierID = &req.SupplierID
	}
	ctx := c.Request().Context()
	if err := recovery.NewService(h.d.DB).Record(ctx, in, middleware.CurrentUserID(c)); err != nil {
		return err
	}
	// A replacement at a new selling price moves the product's price too, as
	// at intake (the new batch already carries it).
	if req.Outcome == recovery.OutcomeReplacement && in.Price != "" {
		if pid, err := recovery.NewService(h.d.DB).LossProduct(ctx, in.SourceType, in.SourceID); err == nil {
			if p, err := h.t.products.Get(ctx, pid); err == nil {
				if _, err := h.applyCount(ctx, p, CountLine{ProductID: p.ID, Selling: in.Price}, "", 0); err != nil {
					return err
				}
			}
		}
	}
	h.audit(c, audit.ActionUpdate, req.SourceType, strconv.FormatInt(req.SourceID, 10), "supplier recovery: "+req.Outcome)
	return c.NoContent(http.StatusNoContent)
}
