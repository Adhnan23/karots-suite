package requests

import (
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

const pageSize = 50

func (h handlers) list(c echo.Context) error {
	tab, q := c.QueryParam("tab"), c.QueryParam("q")
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	ctx := c.Request().Context()
	rs, counts, err := h.p.store.List(ctx, tab, q, pageSize, (page-1)*pageSize)
	if err != nil {
		return storeErr("list requests", err)
	}
	total, err := h.p.store.Count(ctx, tab, q)
	if err != nil {
		return storeErr("count requests", err)
	}
	return c.JSON(http.StatusOK, ListDTO{Rows: rows(rs), Counts: counts, Total: total})
}

// done runs a write on request :id, audits it and answers 204.
func (h handlers) done(c echo.Context, what string, fn func(id int64) error) error {
	id, err := param(c, "id")
	if err != nil {
		return err
	}
	if err := fn(id); err != nil {
		return storeErr(what, err)
	}
	h.audit(c, audit.ActionUpdate, strconv.FormatInt(id, 10), what)
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) updateNote(c echo.Context) error {
	var in struct {
		Note string `json:"note"`
	}
	if err := c.Bind(&in); err != nil || strings.TrimSpace(in.Note) == "" {
		return apperr.Validation("say what the customer asked for")
	}
	return h.done(c, "note edited", func(id int64) error {
		return h.p.store.UpdateNote(c.Request().Context(), id, strings.TrimSpace(in.Note))
	})
}

func (h handlers) updateAsk(c echo.Context) error {
	ask, err := param(c, "ask")
	if err != nil {
		return err
	}
	var in askIn
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid ask")
	}
	name, phone := in.clean()
	return h.done(c, "ask edited", func(id int64) error { return h.p.store.UpdateAsk(c.Request().Context(), id, ask, name, phone) })
}

func (h handlers) deleteAsk(c echo.Context) error {
	ask, err := param(c, "ask")
	if err != nil {
		return err
	}
	if ask == 0 {
		return apperr.Validation("the first ask is the request — remove the request instead")
	}
	return h.done(c, "ask removed", func(id int64) error { return h.p.store.DeleteAsk(c.Request().Context(), id, ask) })
}

func (h handlers) called(c echo.Context) error {
	ask, err := param(c, "ask")
	if err != nil {
		return err
	}
	return h.done(c, "customer called", func(id int64) error { return h.p.store.MarkCalled(c.Request().Context(), id, ask) })
}

func (h handlers) remove(c echo.Context) error {
	return h.done(c, "removed", func(id int64) error { return h.p.store.Delete(c.Request().Context(), id) })
}

// makeProduct creates a product with no stock from the request (with cost
// and supplier) and closes the request onto it.
func (h handlers) makeProduct(c echo.Context) error {
	var in struct {
		Name         string `json:"name"`
		CategoryID   int64  `json:"categoryId"`
		UnitID       int64  `json:"unitId"`
		SellingPrice string `json:"sellingPrice"`
		CostPrice    string `json:"costPrice"`
		SupplierID   int64  `json:"supplierId"`
	}
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid product")
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return apperr.Validation("give the product a name")
	}
	if in.CategoryID <= 0 || in.UnitID <= 0 {
		return apperr.Validation("pick a category and a unit")
	}
	or0 := func(s string) string {
		if s = strings.TrimSpace(s); s == "" {
			return "0"
		}
		return s
	}
	ctx := c.Request().Context()
	id, err := param(c, "id")
	if err != nil {
		return err
	}
	if status, _, err := h.p.store.stage(ctx, h.p.store.db, id); err != nil {
		return storeErr("make product", err)
	} else if status != "open" {
		return storeErr("make product", ErrClosed)
	}
	pin := products.CreateInput{Name: in.Name, CategoryID: in.CategoryID, UnitID: in.UnitID,
		CostPrice: or0(in.CostPrice), SellingPrice: or0(in.SellingPrice)}
	if in.SupplierID > 0 {
		pin.PreferredSupplierID = &in.SupplierID
	}
	prod, err := h.p.core.Products.Create(ctx, pin)
	if err != nil {
		return err
	}
	if err := h.p.store.MarkConverted(ctx, id, prod.ID); err != nil {
		return storeErr("make product", err)
	}
	h.audit(c, audit.ActionUpdate, strconv.FormatInt(id, 10), "made product "+prod.Name)
	return c.JSON(http.StatusOK, map[string]int64{"productId": prod.ID})
}

func (h handlers) link(c echo.Context) error {
	var in struct {
		ProductID int64 `json:"productId"`
	}
	if err := c.Bind(&in); err != nil || in.ProductID <= 0 {
		return apperr.Validation("pick a product")
	}
	prod, err := h.p.core.Products.Get(c.Request().Context(), in.ProductID)
	if err != nil {
		return err
	}
	return h.done(c, "linked to "+prod.Name, func(id int64) error { return h.p.store.MarkConverted(c.Request().Context(), id, prod.ID) })
}

func (h handlers) decline(c echo.Context) error {
	var in struct {
		Reason string `json:"reason"`
	}
	if err := c.Bind(&in); err != nil || strings.TrimSpace(in.Reason) == "" {
		return apperr.Validation("say why it won't be stocked — the till shows it to the next asker")
	}
	reason := strings.TrimSpace(in.Reason)
	return h.done(c, "won't stock: "+reason, func(id int64) error { return h.p.store.Decline(c.Request().Context(), id, reason) })
}

func (h handlers) reopen(c echo.Context) error {
	return h.done(c, "reopened", func(id int64) error { return h.p.store.Reopen(c.Request().Context(), id) })
}

func (h handlers) merge(c echo.Context) error {
	var in struct {
		Into int64 `json:"into"`
	}
	if err := c.Bind(&in); err != nil || in.Into <= 0 {
		return apperr.Validation("pick the request to merge into")
	}
	return h.done(c, "merged into "+strconv.FormatInt(in.Into, 10), func(id int64) error {
		return h.p.store.Merge(c.Request().Context(), id, in.Into)
	})
}

func (h handlers) clearDone(c echo.Context) error {
	n, err := h.p.store.ClearDone(c.Request().Context())
	if err != nil {
		return storeErr("clear done requests", err)
	}
	h.audit(c, audit.ActionDelete, "", "cleared "+strconv.FormatInt(n, 10)+" done")
	return c.JSON(http.StatusOK, map[string]int64{"cleared": n})
}

func (h handlers) audit(c echo.Context, action, id, detail string) {
	if h.p.core.Audit != nil {
		h.p.core.Audit.Record(c.Request().Context(), middleware.CurrentUserID(c), action, "request", id, detail)
	}
}
