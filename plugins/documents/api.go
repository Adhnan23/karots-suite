package documents

import (
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/reports"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

type handlers struct{ p *Plugin }

// ServiceDTO is a print service as the admin page lists it.
type ServiceDTO struct {
	ID            int64  `json:"id"`
	ProductID     int64  `json:"productId"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	Category      string `json:"category"`
	Active        bool   `json:"active"`
	ProductActive bool   `json:"productActive"`
	SellingPrice  string `json:"sellingPrice"`
	Prices        int    `json:"prices"` // grid cells set (metered)
	Paper         int    `json:"paper"`  // paper lines set
}

// ServiceRequest makes a print service: of an existing core service
// (productId), or a new one (name, categoryId, sellingPrice).
type ServiceRequest struct {
	ProductID    int64  `json:"productId"`
	Name         string `json:"name"`
	CategoryID   int64  `json:"categoryId"`
	SellingPrice string `json:"sellingPrice"`
	Kind         string `json:"kind"`
	Category     string `json:"category"`
}

func id(c echo.Context) (int64, error) {
	v, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || v <= 0 {
		return 0, apperr.BadRequest("invalid id")
	}
	return v, nil
}

func (h handlers) audit(c echo.Context, action string, sid int64, detail string) {
	if h.p.core.Audit != nil {
		h.p.core.Audit.Record(c.Request().Context(), middleware.CurrentUserID(c), action, "documents_service", strconv.FormatInt(sid, 10), detail)
	}
}

func (h handlers) services(c echo.Context) error {
	ctx := c.Request().Context()
	svcs, err := h.p.store.Services(ctx, false)
	if err != nil {
		return err
	}
	var counts []struct {
		ID     int64 `db:"id"`
		Prices int   `db:"prices"`
		Paper  int   `db:"paper"`
	}
	if err := h.p.core.DB.SelectContext(ctx, &counts, `SELECT s.id,
		(SELECT COUNT(*) FROM doc_price WHERE service_id = s.id) AS prices,
		(SELECT COUNT(*) FROM doc_consumable WHERE service_id = s.id) AS paper FROM doc_service s`); err != nil {
		return err
	}
	n := map[int64][2]int{}
	for _, x := range counts {
		n[x.ID] = [2]int{x.Prices, x.Paper}
	}
	out := make([]ServiceDTO, 0, len(svcs))
	for _, s := range svcs {
		out = append(out, ServiceDTO{ID: s.ID, ProductID: s.ProductID, Name: s.Name, Kind: s.Kind, Category: s.Category,
			Active: s.IsActive, ProductActive: s.ProductOn, SellingPrice: s.SellingPrice.StringFixed(2), Prices: n[s.ID][0], Paper: n[s.ID][1]})
	}
	return c.JSON(http.StatusOK, out)
}

func (h handlers) create(c echo.Context) error {
	ctx := c.Request().Context()
	var req ServiceRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid body")
	}
	if err := checkKind(req.Kind, req.Category); err != nil {
		return err
	}
	pid := req.ProductID
	if pid == 0 {
		name := strings.TrimSpace(req.Name)
		if name == "" || req.CategoryID <= 0 {
			return apperr.Validation("give the service a name and a category")
		}
		price := strings.TrimSpace(req.SellingPrice)
		if price == "" {
			price = "0"
		}
		var unit int64
		if err := h.p.core.DB.GetContext(ctx, &unit, `SELECT id FROM units ORDER BY (abbreviation = 'pcs') DESC, id LIMIT 1`); err != nil {
			return apperr.Validation("add a unit first")
		}
		in := products.CreateInput{Name: name, CategoryID: req.CategoryID, UnitID: unit, SellingPrice: price,
			CostPrice: "0", WholesalePrice: "0", TaxRate: "0", IsService: true}
		if err := c.Validate(&in); err != nil {
			return err
		}
		p, err := h.p.core.Products.Create(ctx, in)
		if err != nil {
			return err
		}
		pid = p.ID
	}
	sid, err := h.p.store.Link(ctx, h.p.core.DB, pid, req.Kind, req.Category)
	if err != nil {
		return err
	}
	h.audit(c, "documents_service_create", sid, req.Kind)
	return c.JSON(http.StatusOK, map[string]int64{"id": sid, "productId": pid})
}

func (h handlers) update(c echo.Context) error {
	sid, err := id(c)
	if err != nil {
		return err
	}
	var req ServiceRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid body")
	}
	if _, err := h.p.store.Service(c.Request().Context(), sid); err != nil {
		return err
	}
	if err := h.p.store.Update(c.Request().Context(), sid, req.Kind, req.Category); err != nil {
		return err
	}
	h.audit(c, "documents_service_update", sid, req.Kind+" "+req.Category)
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) setActive(c echo.Context) error {
	sid, err := id(c)
	if err != nil {
		return err
	}
	var req struct {
		Active bool `json:"active"`
	}
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid body")
	}
	if _, err := h.p.store.Service(c.Request().Context(), sid); err != nil {
		return err
	}
	if err := h.p.store.SetActive(c.Request().Context(), sid, req.Active); err != nil {
		return err
	}
	h.audit(c, map[bool]string{true: "documents_service_enable", false: "documents_service_disable"}[req.Active], sid, "")
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) grid(c echo.Context) error {
	sid, err := id(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	if _, err := h.p.store.Service(ctx, sid); err != nil {
		return err
	}
	prices, err := h.p.store.Prices(ctx, sid)
	if err != nil {
		return err
	}
	paper, err := h.p.store.Paper(ctx, sid)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toGrid(prices, paper))
}

func (h handlers) saveGrid(c echo.Context) error {
	sid, err := id(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	if _, err := h.p.store.Service(ctx, sid); err != nil {
		return err
	}
	var g GridDTO
	if err := c.Bind(&g); err != nil {
		return apperr.BadRequest("invalid body")
	}
	prices, paper, err := fromGrid(g)
	if err != nil {
		return err
	}
	for _, p := range paper {
		var svc bool
		if err := h.p.core.DB.GetContext(ctx, &svc, `SELECT is_service FROM products WHERE id = $1`, p.ProductID); err != nil {
			return apperr.Validation("pick the paper from your products")
		}
		if svc {
			return apperr.Validation("paper must be a stocked item, not a service")
		}
	}
	if err := h.p.store.SaveGrid(ctx, sid, prices, paper); err != nil {
		return err
	}
	h.audit(c, "documents_grid_save", sid, strconv.Itoa(len(prices))+" prices, "+strconv.Itoa(len(paper))+" paper")
	return h.grid(c)
}

// ReportRowDTO is one service × options; options are null for custom and
// karots-pos jobs (earlier = sold before the suite, kept as it was).
type ReportRowDTO struct {
	Name    string  `json:"name"`
	Size    *string `json:"size"`
	Color   *bool   `json:"color"`
	Double  *bool   `json:"double"`
	Earlier bool    `json:"earlier"`
	Jobs    int     `json:"jobs"`
	Copies  string  `json:"copies"`
	Sheets  string  `json:"sheets"`
	Revenue string  `json:"revenue"`
}

type ReportDTO struct {
	Rows         []ReportRowDTO `json:"rows"`
	Jobs         int            `json:"jobs"`
	Copies       string         `json:"copies"`
	Sheets       string         `json:"sheets"`
	Revenue      string         `json:"revenue"`
	OwnUseJobs   int            `json:"ownUseJobs"`
	OwnUseSheets string         `json:"ownUseSheets"`
	OwnUseCost   string         `json:"ownUseCost"`
}

func (h handlers) report(c echo.Context) error {
	from, to, _, _, err := reports.ResolveRange(c.QueryParam("preset"), c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return err
	}
	r, err := h.p.store.Report(c.Request().Context(), from, to)
	if err != nil {
		return err
	}
	out := ReportDTO{Rows: make([]ReportRowDTO, 0, len(r.Rows)), Jobs: r.Jobs, Copies: r.Copies.String(), Sheets: r.Sheets.String(),
		Revenue: r.Revenue.StringFixed(2), OwnUseJobs: r.OwnUse.Jobs, OwnUseSheets: r.OwnUse.Sheets.String(), OwnUseCost: r.OwnUse.Cost.StringFixed(2)}
	for _, x := range r.Rows {
		out.Rows = append(out.Rows, ReportRowDTO{Name: x.Name, Size: x.Size, Color: x.Color, Double: x.Double, Earlier: !x.Linked,
			Jobs: x.Jobs, Copies: x.Copies.String(), Sheets: x.Sheets.String(), Revenue: x.Revenue.StringFixed(2)})
	}
	return c.JSON(http.StatusOK, out)
}
