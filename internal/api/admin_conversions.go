package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/conversions"

	"github.com/labstack/echo/v4"
)

const runPageSize = 30

// Product conversions (karots-pos /admin/conversions): repack / break bulk.
func (h *handlers) registerAdminConversions(g *echo.Group) {
	g.GET("/conversions", h.AdminConversions)
	g.GET("/conversions/runs", h.AdminConversionRuns)
	g.POST("/conversions", h.AdminConversionCreate)
	g.PUT("/conversions/:id", h.AdminConversionUpdate)
	g.DELETE("/conversions/:id", h.AdminConversionDisable)
	g.POST("/conversions/:id/activate", h.AdminConversionReactivate)
	g.POST("/conversions/:id/run", h.CounterConversionRun) // same run as the counter
}

// AdminConversions lists recipes with both products' on-hand; ?disabled=1 includes disabled ones.
func (h *handlers) AdminConversions(c echo.Context) error {
	svc := conversions.NewService(h.d.DB)
	q := strings.TrimSpace(c.QueryParam("q"))
	list := svc.List
	if c.QueryParam("disabled") == "1" {
		list = svc.ListAll
	}
	rows, err := list(c.Request().Context(), q)
	if err != nil {
		return err
	}
	out := make([]AdminConversion, 0, len(rows))
	for _, r := range rows {
		out = append(out, AdminConversion{ID: r.ID, FromProductID: r.FromProductID, ToProductID: r.ToProductID, FromName: r.FromName, ToName: r.ToName,
			FromUnit: r.FromUnitAbbr, ToUnit: r.ToUnitAbbr, Ratio: r.Ratio.String(), FromStock: r.FromStock.String(), ToStock: r.ToStock.String(),
			FromCost: r.FromCost.String(), Note: strOr(r.Note), IsActive: r.IsActive})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) AdminConversionCreate(c echo.Context) error {
	var req ConversionRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if req.FromProductID == req.ToProductID {
		return apperr.Validation("a product can't convert into itself")
	}
	in := conversions.CreateInput{FromProductID: req.FromProductID, ToProductID: req.ToProductID, Ratio: strings.TrimSpace(req.Ratio), Note: optStr(req.Note)}
	if err := c.Validate(&in); err != nil {
		return err
	}
	cv, err := conversions.NewService(h.d.DB).Create(c.Request().Context(), in)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "conversion", strconv.FormatInt(cv.ID, 10), "ratio "+in.Ratio)
	return c.JSON(http.StatusCreated, Option{ID: cv.ID})
}

func (h *handlers) AdminConversionUpdate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req ConversionRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := conversions.UpdateInput{Ratio: strings.TrimSpace(req.Ratio), Note: optStr(req.Note)}
	if err := c.Validate(&in); err != nil {
		return err
	}
	if _, err := conversions.NewService(h.d.DB).Update(c.Request().Context(), id, in); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "conversion", strconv.FormatInt(id, 10), "ratio "+in.Ratio)
	return c.NoContent(http.StatusNoContent)
}

// AdminConversionDisable retires a recipe; its run history stays.
func (h *handlers) AdminConversionDisable(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := conversions.NewService(h.d.DB).Delete(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionDelete, "conversion", strconv.FormatInt(id, 10), "disabled")
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminConversionReactivate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := conversions.NewService(h.d.DB).Reactivate(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "conversion", strconv.FormatInt(id, 10), "reactivated")
	return c.NoContent(http.StatusNoContent)
}

// AdminConversionRuns is the run history: ?conversion=, a period, paged;
// ?format=csv|xlsx|ods downloads every match.
func (h *handlers) AdminConversionRuns(c echo.Context) error {
	var f conversions.RunFilter
	if id, err := strconv.ParseInt(c.QueryParam("conversion"), 10, 64); err == nil && id > 0 {
		f.ConversionID = &id
	}
	from, to, err := listRange(c)
	if err != nil {
		return err
	}
	f.From, f.To = from, to
	page, _ := strconv.Atoi(c.QueryParam("page"))
	page = max(page, 1)
	download := c.QueryParam("format") != ""
	if !download {
		f.Limit, f.Offset = runPageSize, (page-1)*runPageSize
	}
	rows, total, err := conversions.NewService(h.d.DB).ListRuns(c.Request().Context(), f)
	if err != nil {
		return err
	}
	if download {
		out := make([][]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, []string{r.CreatedAt.Format("2006-01-02 15:04"), r.FromName, r.FromQty.String(), r.FromUnitAbbr, r.ToName, r.ToQty.String(), r.ToUnitAbbr, r.UserName})
		}
		return writeSheet(c, "conversion_runs_"+time.Now().Format("2006-01-02"), []string{"When", "From", "Qty out", "Unit", "To", "Qty in", "Unit", "By"}, out)
	}
	out := ConversionRunList{Items: make([]ConversionRunRow, 0, len(rows)), Total: total, Page: page, PageSize: runPageSize}
	for _, r := range rows {
		out.Items = append(out.Items, ConversionRunRow{ID: r.ID, FromName: r.FromName, ToName: r.ToName, FromQty: r.FromQty.String(), ToQty: r.ToQty.String(),
			FromUnit: r.FromUnitAbbr, ToUnit: r.ToUnitAbbr, UserName: r.UserName, CreatedAt: r.CreatedAt.Format(time.RFC3339)})
	}
	return c.JSON(http.StatusOK, out)
}
