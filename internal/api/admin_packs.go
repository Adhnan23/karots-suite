package api

import (
	"net/http"
	"strconv"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"

	"github.com/labstack/echo/v4"
)

func (h *handlers) registerAdminPacks(g *echo.Group) {
	g.GET("/products/:id/packs", h.AdminPacks)
	g.POST("/products/:id/packs", h.AdminPackSave)
	g.PUT("/products/:id/packs/:pid", h.AdminPackSave)
	g.DELETE("/products/:id/packs/:pid", h.AdminPackDelete)
	g.DELETE("/products/:id/packs/:pid/barcodes/:code", h.AdminPackBarcodeRemove)
}

func packDTO(k products.Pack) AdminPack {
	codes := []string(k.Barcodes)
	if codes == nil {
		codes = []string{}
	}
	return AdminPack{ID: k.ID, Name: k.Name, Size: k.Size.String(), SellingPrice: dec2(k.SellingPrice),
		WholesalePrice: dec2(k.WholesalePrice), SetAtTill: k.SetAtTill, Barcodes: codes}
}

// AdminPacks lists a product's packs, smallest first, for the form's Packs block.
func (h *handlers) AdminPacks(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ks, err := h.t.products.Packs(c.Request().Context(), id)
	if err != nil {
		return apperr.Internal("failed to load packs", err)
	}
	out := make([]AdminPack, 0, len(ks))
	for _, k := range ks {
		out = append(out, packDTO(k))
	}
	return c.JSON(http.StatusOK, out)
}

// AdminPackSave creates (POST) or edits (PUT …/:pid) a pack. Editing a size set
// at the till counts as checking it.
func (h *handlers) AdminPackSave(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req PackRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := products.PackInput{Name: req.Name, Size: req.Size, SellingPrice: req.SellingPrice,
		WholesalePrice: req.WholesalePrice, Barcode: req.Barcode}
	if p := c.Param("pid"); p != "" {
		if in.ID, err = strconv.ParseInt(p, 10, 64); err != nil || in.ID <= 0 {
			return apperr.BadRequest("invalid pack id")
		}
	}
	k, err := h.t.products.SavePack(c.Request().Context(), id, in)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(id, 10), "pack "+k.Name+" ×"+k.Size.String())
	return c.JSON(http.StatusOK, packDTO(*k))
}

func (h *handlers) AdminPackDelete(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	pid, err := strconv.ParseInt(c.Param("pid"), 10, 64)
	if err != nil {
		return apperr.BadRequest("invalid pack id")
	}
	if err := h.t.products.DeletePack(c.Request().Context(), id, pid); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(id, 10), "removed a pack")
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminPackBarcodeRemove(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	pid, err := strconv.ParseInt(c.Param("pid"), 10, 64)
	if err != nil {
		return apperr.BadRequest("invalid pack id")
	}
	code := c.Param("code")
	if err := h.t.products.RemovePackBarcode(c.Request().Context(), id, pid, code); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(id, 10), "removed pack barcode "+code)
	return c.NoContent(http.StatusNoContent)
}
