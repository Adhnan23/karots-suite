package productplus

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

const gridPageSize = 50

type handlers struct{ p *Plugin }

func (h handlers) audit(c echo.Context, action, key, detail string) {
	if h.p.core.Audit != nil {
		h.p.core.Audit.Record(c.Request().Context(), middleware.CurrentUserID(c), action, "product_field", key, detail)
	}
}

func pathID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.BadRequest("invalid id")
	}
	return id, nil
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return apperr.NotFound("field")
	}
	return err
}

// GET /fields?disabled=1 — every field with its scope and counts.
func (h handlers) fields(c echo.Context) error {
	ctx := c.Request().Context()
	fs, err := h.p.store.Fields(ctx, c.QueryParam("disabled") == "1")
	if err != nil {
		return err
	}
	st, err := h.p.store.Stats(ctx)
	if err != nil {
		return err
	}
	out := make([]FieldDTO, len(fs))
	for i, f := range fs {
		out[i] = fieldDTO(f, st[f.ID])
	}
	return c.JSON(http.StatusOK, out)
}

func (h handlers) one(c echo.Context, id int64, code int) error {
	ctx := c.Request().Context()
	f, err := h.p.store.Field(ctx, id)
	if err != nil {
		return notFound(err)
	}
	st, err := h.p.store.Stats(ctx)
	if err != nil {
		return err
	}
	return c.JSON(code, fieldDTO(*f, st[id]))
}

// POST /fields
func (h handlers) create(c echo.Context) error {
	var r FieldRequest
	if err := c.Bind(&r); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	f, err := r.toField()
	if err != nil {
		return err
	}
	id, err := h.p.store.CreateField(c.Request().Context(), f, r.Scope)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, strconv.FormatInt(id, 10), "added custom field "+f.Label+" ("+f.Type+")")
	return h.one(c, id, http.StatusCreated)
}

// PUT /fields/:id — a lossy type change needs confirmLoss (409 otherwise); a
// default change keeps existing products unless defaultFor is "all".
func (h handlers) update(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var r FieldRequest
	if err := c.Bind(&r); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	f, err := r.toField()
	if err != nil {
		return err
	}
	f.ID = id
	err = h.p.store.UpdateField(c.Request().Context(), f, r.Scope, UpdateOpts{DefaultFor: r.DefaultFor, ConfirmLoss: r.ConfirmLoss})
	var loss *ErrDataLoss
	if errors.As(err, &loss) {
		return apperr.ConflictCode("DATA_LOSS", loss.Error()+" — they would be deleted")
	}
	if err != nil {
		return notFound(err)
	}
	h.audit(c, audit.ActionUpdate, strconv.FormatInt(id, 10), "updated custom field "+f.Label)
	return h.one(c, id, http.StatusOK)
}

// GET /fields/:id/impact?type=&option=a&option=b — what an edit would do.
func (h handlers) impact(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	opts := c.QueryParams()["option"]
	im, err := h.p.store.Impact(c.Request().Context(), id, c.QueryParam("type"), opts)
	if err != nil {
		return notFound(err)
	}
	return c.JSON(http.StatusOK, im)
}

// POST /fields/:id/active {active}
func (h handlers) setActive(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var r struct {
		Active bool `json:"active"`
	}
	if err := c.Bind(&r); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := h.p.store.SetFieldActive(c.Request().Context(), id, r.Active); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, strconv.FormatInt(id, 10), map[bool]string{true: "field enabled", false: "field disabled"}[r.Active])
	return c.NoContent(http.StatusNoContent)
}

// POST /fields/:id/move {dir: "up" | "down"}
func (h handlers) move(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var r struct {
		Dir string `json:"dir"`
	}
	if err := c.Bind(&r); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := h.p.store.MoveField(c.Request().Context(), id, r.Dir == "up"); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// GET /form?productId=&categoryId=&where=form|intake — the fields a product form
// shows for that category, with the product's values (productId 0 = defaults).
func (h handlers) form(c echo.Context) error {
	pid, _ := strconv.ParseInt(c.QueryParam("productId"), 10, 64)
	cat, _ := strconv.ParseInt(c.QueryParam("categoryId"), 10, 64)
	rows, err := h.p.store.FormFields(c.Request().Context(), pid, cat, c.QueryParam("where"))
	if err != nil {
		return err
	}
	out := make([]FormFieldDTO, len(rows))
	for i, r := range rows {
		out[i] = formDTO(r)
	}
	return c.JSON(http.StatusOK, out)
}

// GET /grid?fields=1,2&categoryId=&missing=1&q=&page=
func (h handlers) grid(c echo.Context) error {
	q := GridQuery{Q: c.QueryParam("q"), MissingOnly: c.QueryParam("missing") == "1", Limit: gridPageSize}
	for _, s := range strings.Split(c.QueryParam("fields"), ",") {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			q.FieldIDs = append(q.FieldIDs, id)
		}
	}
	q.CategoryID, _ = strconv.ParseInt(c.QueryParam("categoryId"), 10, 64)
	if page, _ := strconv.Atoi(c.QueryParam("page")); page > 1 {
		q.Offset = (page - 1) * gridPageSize
	}
	rows, total, err := h.p.store.Grid(c.Request().Context(), q)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, GridDTO{Rows: rows, Total: total})
}

// PUT /values [{productId, fieldId, value}] — Fill in's save.
func (h handlers) values(c echo.Context) error {
	var cells []Cell
	if err := c.Bind(&cells); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	saved, bad, err := h.p.store.SetValues(c.Request().Context(), cells)
	if err != nil {
		return err
	}
	if saved > 0 {
		h.audit(c, audit.ActionUpdate, "", "set "+strconv.Itoa(saved)+" custom field values")
	}
	if bad == nil {
		bad = []CellError{}
	}
	return c.JSON(http.StatusOK, SaveResult{Saved: saved, Errors: bad})
}
