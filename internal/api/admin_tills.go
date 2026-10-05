package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/denominations"

	"github.com/labstack/echo/v4"
)

func (h *handlers) registerAdminTills(g *echo.Group) {
	g.GET("/tills", h.AdminTills)
	g.GET("/tills/cashiers", h.AdminTillCashiers)
	g.GET("/tills/:id", h.AdminTill)
	g.GET("/denominations", h.AdminDenominations)
	g.POST("/denominations", h.AdminDenominationCreate)
	g.PUT("/denominations/:id", h.AdminDenominationUpdate)
	g.DELETE("/denominations/:id", h.AdminDenominationDelete)
}

func (h *handlers) tillRow(c echo.Context, s cashregister.SessionRow) TillSessionRow {
	r := TillSessionRow{ID: s.ID, UserID: s.UserID, Cashier: s.UserName, OpenedAt: s.OpenedAt.Format(time.RFC3339), Opening: dec2(s.OpeningCash)}
	if s.ClosedAt == nil {
		r.Open = true
		if sum, err := h.d.CashRegister.Summary(c.Request().Context(), s.UserID); err == nil {
			r.Expected = dec2(sum.Expected)
		}
		return r
	}
	r.ClosedAt = s.ClosedAt.Format(time.RFC3339)
	if s.ExpectedCash != nil {
		r.Expected = dec2(*s.ExpectedCash)
	}
	if s.ClosingCash != nil {
		r.Counted = dec2(*s.ClosingCash)
	}
	if s.Difference != nil {
		r.Difference = dec2(*s.Difference)
	}
	return r
}

// AdminTills pages till sessions, newest first: ?preset/from/to on the opening
// time, ?user= one cashier, ?open=1 the tills open now.
func (h *handlers) AdminTills(c echo.Context) error {
	from, to, err := listRange(c)
	if err != nil {
		return err
	}
	user, _ := strconv.ParseInt(c.QueryParam("user"), 10, 64)
	rows, err := h.d.CashRegister.ListSessions(c.Request().Context(), cashregister.SessionFilter{From: from, To: to, UserID: user,
		OpenOnly: c.QueryParam("open") == "1", Limit: counterPage, Offset: listOffset(c)})
	if err != nil {
		return err
	}
	out := make([]TillSessionRow, 0, len(rows))
	for _, s := range rows {
		out = append(out, h.tillRow(c, s))
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) AdminTillCashiers(c echo.Context) error {
	rows, err := h.d.CashRegister.Cashiers(c.Request().Context())
	if err != nil {
		return err
	}
	out := make([]CashierOption, 0, len(rows))
	for _, s := range rows {
		out = append(out, CashierOption{ID: s.UserID, Name: s.UserName})
	}
	return c.JSON(http.StatusOK, out)
}

func breakdownMap(raw []byte) map[string]float64 {
	m := map[string]float64{}
	_ = json.Unmarshal(raw, &m) // nil / bad = no count
	return m
}

// AdminTill is one session: money, movements, and the drawer counts.
func (h *handlers) AdminTill(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	s, moves, err := h.d.CashRegister.SessionDetail(c.Request().Context(), id)
	if err != nil {
		return err
	}
	out := TillSessionView{Session: h.tillRow(c, *s), Moves: make([]TillMove, 0, len(moves)),
		OpeningCount: breakdownMap(s.OpeningBreakdown), ClosingCount: breakdownMap(s.ClosingBreakdown)}
	for _, m := range moves {
		out.Moves = append(out.Moves, TillMove{At: m.CreatedAt.Format(time.RFC3339), Type: m.Type, Amount: dec2(m.Amount), Reason: strOr(m.Reason)})
	}
	return c.JSON(http.StatusOK, out)
}

// ---- Denominations (karots-pos /admin/denominations) ----

func (h *handlers) AdminDenominations(c echo.Context) error {
	rows, err := h.t.denoms.List(c.Request().Context(), false)
	if err != nil {
		return err
	}
	out := make([]AdminDenomination, 0, len(rows))
	for _, d := range rows {
		out = append(out, AdminDenomination{ID: d.ID, Value: d.Value.String(), IsNote: d.IsNote, Active: d.IsActive})
	}
	return c.JSON(http.StatusOK, out)
}

func bindDenomination(c echo.Context) (denominations.Input, error) {
	var in DenominationRequest
	if err := c.Bind(&in); err != nil {
		return denominations.Input{}, apperr.BadRequest("invalid denomination")
	}
	v, err := parseAmount(in.Value, "value")
	if err != nil {
		return denominations.Input{}, err
	}
	return denominations.Input{Value: v.String(), IsNote: in.IsNote, IsActive: in.Active}, nil
}

// AdminDenominationCreate adds a note or coin; a value already listed is a 409.
func (h *handlers) AdminDenominationCreate(c echo.Context) error {
	in, err := bindDenomination(c)
	if err != nil {
		return err
	}
	d, err := h.t.denoms.Create(c.Request().Context(), in)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "denomination", strconv.FormatInt(d.ID, 10), "added denomination "+in.Value)
	return c.JSON(http.StatusCreated, AdminDenomination{ID: d.ID, Value: d.Value.String(), IsNote: d.IsNote, Active: d.IsActive})
}

// AdminDenominationUpdate edits the value / note-or-coin / in-circulation flag
// (the list's inline toggle sends the whole row).
func (h *handlers) AdminDenominationUpdate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	in, err := bindDenomination(c)
	if err != nil {
		return err
	}
	if err := h.t.denoms.Update(c.Request().Context(), id, in); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "denomination", strconv.FormatInt(id, 10), "updated denomination "+in.Value)
	return c.NoContent(http.StatusNoContent)
}

// AdminDenominationDelete removes one no saved count uses (else 409).
func (h *handlers) AdminDenominationDelete(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.t.denoms.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionDelete, "denomination", strconv.FormatInt(id, 10), "")
	return c.NoContent(http.StatusNoContent)
}
