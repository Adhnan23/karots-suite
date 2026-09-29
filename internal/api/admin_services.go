package api

import (
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/recipes"
	"karots-suite/internal/money"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerAdminServices(g *echo.Group) {
	g.GET("/services", h.AdminServices)
	g.POST("/services", h.AdminServiceCreate)
	g.GET("/products/:id/recipe", h.AdminRecipe)
	g.POST("/products/:id/recipe", h.AdminRecipeSave)
}

// AdminServices lists service products with what their recipe costs per unit.
// Services are left out of the Products list, so a recipe is reached from here.
func (h *handlers) AdminServices(c echo.Context) error {
	ctx := c.Request().Context()
	rows, err := h.t.products.AllServices(ctx)
	if err != nil {
		return err
	}
	rs := recipes.NewService(h.d.DB)
	counts, err := rs.Counts(ctx)
	if err != nil {
		return err
	}
	costs, err := rs.Summaries(ctx)
	if err != nil {
		return err
	}
	out := make([]ServiceRow, 0, len(rows))
	for _, p := range rows {
		r := ServiceRow{ID: p.ID, Name: p.Name, CategoryName: p.CategoryName, UnitAbbr: p.UnitAbbr, SellingPrice: dec2(p.SellingPrice), Ingredients: counts[p.ID], IsActive: p.IsActive}
		if cs, ok := costs[p.ID]; ok {
			r.StockCost, r.OtherCost, r.TrueCost, r.Margin = dec2(cs.Stock), dec2(cs.Other), dec2(cs.True()), dec2(p.SellingPrice.Sub(cs.True()))
		}
		out = append(out, r)
	}
	return c.JSON(http.StatusOK, out)
}

// AdminServiceCreate makes a non-stocked product. IsService is set here, never
// bound: the product form must not be able to turn stock into a service.
func (h *handlers) AdminServiceCreate(c echo.Context) error {
	var req ServiceRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := products.CreateInput{Name: strings.TrimSpace(req.Name), CategoryID: req.CategoryID, UnitID: req.UnitID, SellingPrice: strings.TrimSpace(req.SellingPrice),
		CostPrice: "0", WholesalePrice: "0", TaxRate: "0", IsService: true}
	if in.SellingPrice == "" {
		in.SellingPrice = "0"
	}
	if in.CategoryID <= 0 || in.UnitID <= 0 {
		return apperr.Validation("pick a category and a unit")
	}
	if err := c.Validate(&in); err != nil {
		return err
	}
	p, err := h.t.products.Create(c.Request().Context(), in)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "product", strconv.FormatInt(p.ID, 10), "created service "+in.Name)
	return c.JSON(http.StatusCreated, ProductSaved{ID: p.ID, Message: "Service added"})
}

func (h *handlers) serviceProduct(c echo.Context) (*products.Product, error) {
	id, err := pathID(c)
	if err != nil {
		return nil, err
	}
	p, err := h.t.products.Get(c.Request().Context(), id)
	if err != nil {
		return nil, err
	}
	// A stocked product has inventory of its own; consuming ingredients as well
	// would count its cost twice.
	if !p.IsService {
		return nil, apperr.Validation("only service products can have a recipe")
	}
	return p, nil
}

func (h *handlers) AdminRecipe(c echo.Context) error {
	p, err := h.serviceProduct(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	rs := recipes.NewService(h.d.DB)
	cs, err := rs.For(ctx, p.ID)
	if err != nil {
		return err
	}
	ls, err := rs.CostsFor(ctx, p.ID)
	if err != nil {
		return err
	}
	out := Recipe{ProductID: p.ID, Name: p.Name, UnitAbbr: p.UnitAbbr, Selling: dec2(p.SellingPrice), Components: []RecipeComponent{}, Costs: []RecipeCost{}}
	for _, x := range cs {
		rc := RecipeComponent{ProductID: x.ComponentProductID, Name: x.ComponentName, UnitAbbr: x.UnitAbbr, CostPrice: dec2(x.CostPrice), Mode: "per_unit", Whole: x.WholeUnits}
		if x.YieldUnits.Valid {
			rc.Mode, rc.Amount = "yield", x.YieldUnits.Decimal.String()
		} else {
			rc.Amount = x.QtyPerUnit.Decimal.String()
		}
		out.Components = append(out.Components, rc)
	}
	for _, l := range ls {
		out.Costs = append(out.Costs, RecipeCost{Label: l.Label, Amount: dec2(l.CostPerUnit)})
	}
	return c.JSON(http.StatusOK, out)
}

// AdminRecipeSave replaces the whole recipe: stock ingredients and non-stock
// cost lines, in one transaction (recipes.Service.Replace validates).
func (h *handlers) AdminRecipeSave(c echo.Context) error {
	p, err := h.serviceProduct(c)
	if err != nil {
		return err
	}
	var req RecipeRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	cs := make([]recipes.Component, 0, len(req.Components))
	for _, x := range req.Components {
		amt, err := money.Parse(x.Amount)
		if err != nil || !amt.IsPositive() || x.ProductID <= 0 {
			return apperr.Validation("each ingredient needs a positive amount")
		}
		comp := recipes.Component{ComponentProductID: x.ProductID, WholeUnits: x.Whole}
		if x.Mode == "yield" {
			comp.YieldUnits = decimal.NullDecimal{Decimal: amt, Valid: true}
		} else {
			comp.QtyPerUnit = decimal.NullDecimal{Decimal: amt, Valid: true}
		}
		cs = append(cs, comp)
	}
	ls := make([]recipes.CostLine, 0, len(req.Costs))
	for _, x := range req.Costs {
		label, raw := strings.TrimSpace(x.Label), strings.TrimSpace(x.Amount)
		if label == "" && raw == "" {
			continue // the editor's spare empty row
		}
		amt, err := money.Parse(raw)
		if err != nil || amt.IsNegative() {
			return apperr.Validation("each cost line needs an amount of zero or more")
		}
		ls = append(ls, recipes.CostLine{Label: label, CostPerUnit: amt})
	}
	if err := recipes.NewService(h.d.DB).Replace(c.Request().Context(), p.ID, cs, ls); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(p.ID, 10), "recipe updated")
	return c.NoContent(http.StatusNoContent)
}
