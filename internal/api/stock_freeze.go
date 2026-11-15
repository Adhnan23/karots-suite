package api

import (
	"net/http"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
)

// Stock freeze routes. Freezing (a product, the shop) is the admin's; anyone at
// the till may unfreeze with a shelf count or open a box onto the shelf.
func (h *handlers) registerAdminFreeze(g *echo.Group) {
	a := g.Group("/stock", middleware.RequireRole(auth.RoleAdmin))
	a.POST("/freeze", h.StockFreeze)
	a.POST("/freeze-shop", h.StockFreezeShop)
	a.POST("/unfreeze-shop", h.StockUnfreezeShop)
	g.GET("/stock/frozen", h.StockFrozenList)
	g.POST("/stock/:id/unfreeze", h.StockUnfreeze)
	g.POST("/stock/:id/to-shelf", h.StockToShelf)
}

// registerTillFreeze: every till user, the sales person too (user, 2026-11-14).
func (h *handlers) registerTillFreeze(g *echo.Group) {
	g.GET("/frozen", h.StockFrozenList)
	g.POST("/stock/:id/unfreeze", h.StockUnfreeze)
	g.POST("/stock/:id/to-shelf", h.StockToShelf)
}

func (h *handlers) freezeResult(c echo.Context, r stock.FreezeResult) error {
	out := FreezeResultDTO{Frozen: r.Frozen, Refused: []FreezeRefusal{}}
	if len(r.Refused) > 0 {
		ids := make([]int64, 0, len(r.Refused))
		for id := range r.Refused {
			ids = append(ids, id)
		}
		var rows []struct {
			ID   int64  `db:"id"`
			Name string `db:"name"`
		}
		if err := h.d.DB.SelectContext(c.Request().Context(), &rows, `SELECT id, name FROM products WHERE id = ANY($1) ORDER BY name`, pq.Array(ids)); err != nil {
			return apperr.Internal("failed to name refused products", err)
		}
		for _, p := range rows {
			out.Refused = append(out.Refused, FreezeRefusal{ProductID: p.ID, Name: p.Name, Reason: r.Refused[p.ID]})
		}
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) StockFreeze(c echo.Context) error {
	var req FreezeRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	r, err := h.t.stock.Freeze(c.Request().Context(), req.ProductIDs)
	if err != nil {
		return err
	}
	return h.freezeResult(c, r)
}

func (h *handlers) StockFreezeShop(c echo.Context) error {
	r, err := h.t.stock.FreezeShop(c.Request().Context())
	if err != nil {
		return err
	}
	return h.freezeResult(c, r)
}

func (h *handlers) StockUnfreezeShop(c echo.Context) error {
	n, err := h.t.stock.UnfreezeShop(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]int{"unfrozen": n})
}

func (h *handlers) StockUnfreeze(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req UnfreezeRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := h.t.stock.Unfreeze(c.Request().Context(), id, req.ShelfQty, middleware.CurrentUserID(c)); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) StockToShelf(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req ToShelfRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := h.t.stock.MoveToShelf(c.Request().Context(), id, req.Quantity, middleware.CurrentUserID(c)); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) StockFrozenList(c echo.Context) error {
	ctx := c.Request().Context()
	rows, err := h.t.stock.FrozenProducts(ctx)
	if err != nil {
		return err
	}
	out := FrozenListDTO{Products: make([]FrozenProductDTO, 0, len(rows))}
	_ = h.d.DB.GetContext(ctx, &out.ShopFrozen, `SELECT freeze_new_products FROM settings WHERE id = 1`)
	for _, r := range rows {
		d := FrozenProductDTO{ID: r.ID, Name: r.Name, Stock: r.Stock.String(), Unit: r.Unit}
		if r.Since != nil {
			d.Since = r.Since.Format(time.RFC3339)
		}
		out.Products = append(out.Products, d)
	}
	return c.JSON(http.StatusOK, out)
}
