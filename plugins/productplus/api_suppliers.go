package productplus

import (
	"errors"
	"net/http"
	"strconv"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/audit"

	"github.com/labstack/echo/v4"
)

// Supplier by field's screen (PPF-10…12). DTOs mirror web/src/plugins/productplus/types.ts.

type FieldOption struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
}

type SupplierValueDTO struct {
	Value        string `json:"value"`
	Products     int    `json:"products"`
	SupplierID   int64  `json:"supplierId"`
	SupplierName string `json:"supplierName"`
}

type SupplierRulesDTO struct {
	FieldID int64              `json:"fieldId"` // 0 = none chosen
	Fields  []FieldOption      `json:"fields"`  // the fields that can be chosen
	Values  []SupplierValueDTO `json:"values"`
}

// GET /supplier-rules
func (h handlers) supplierRules(c echo.Context) error {
	ctx := c.Request().Context()
	fid, err := h.p.store.SupplierField(ctx)
	if err != nil {
		return err
	}
	fs, err := h.p.store.Fields(ctx, false)
	if err != nil {
		return err
	}
	out := SupplierRulesDTO{FieldID: fid, Fields: []FieldOption{}, Values: []SupplierValueDTO{}}
	for _, f := range fs {
		if f.Type == "text" || f.Type == "select" {
			out.Fields = append(out.Fields, FieldOption{ID: f.ID, Label: f.Label})
		}
	}
	vals, err := h.p.store.SupplierValues(ctx, fid)
	if err != nil {
		return err
	}
	for _, v := range vals {
		out.Values = append(out.Values, SupplierValueDTO(v))
	}
	return c.JSON(http.StatusOK, out)
}

// PUT /supplier-rules/field {fieldId, confirm}
func (h handlers) supplierField(c echo.Context) error {
	var r struct {
		FieldID int64 `json:"fieldId"`
		Confirm bool  `json:"confirm"`
	}
	if err := c.Bind(&r); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	switch err := h.p.store.SetSupplierField(c.Request().Context(), r.FieldID, r.Confirm); {
	case errors.Is(err, ErrNeedsConfirm):
		return apperr.ConflictCode("NEEDS_CONFIRM", "Choosing another field clears the suppliers set for this one")
	case errors.Is(err, ErrUnusableField):
		return apperr.Validation("Only an active text or dropdown field can give the supplier")
	case err != nil:
		return err
	}
	h.audit(c, audit.ActionUpdate, "supplier-field", "supplier field "+strconv.FormatInt(r.FieldID, 10))
	return c.NoContent(http.StatusNoContent)
}

// PUT /supplier-rules/value {value, supplierId} — 0 removes the rule.
func (h handlers) supplierValue(c echo.Context) error {
	var r struct {
		Value      string `json:"value"`
		SupplierID int64  `json:"supplierId"`
	}
	if err := c.Bind(&r); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	ctx := c.Request().Context()
	if r.SupplierID != 0 {
		var ok bool
		if err := appdb.On(ctx, h.p.store.db).GetContext(ctx, &ok, `SELECT EXISTS (SELECT 1 FROM suppliers WHERE id = $1 AND is_active)`, r.SupplierID); err != nil {
			return err
		}
		if !ok {
			return apperr.Validation("Pick an active supplier")
		}
	}
	if fid, err := h.p.store.SupplierField(ctx); err != nil {
		return err
	} else if fid == 0 {
		return apperr.Validation("Choose the field first")
	}
	if err := h.p.store.SetSupplierRule(ctx, r.Value, r.SupplierID); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "supplier-rule", r.Value+" → supplier "+strconv.FormatInt(r.SupplierID, 10))
	return c.NoContent(http.StatusNoContent)
}
