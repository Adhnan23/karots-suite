package requests

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

// The plugin's own JSON (core's types.gen.ts is core-only); mirrored by hand in
// web/src/plugins/requests/types.ts. Times are ISO 8601.

type AskRow struct {
	ID       int64   `json:"id"` // 0 = the request's own first ask
	Name     string  `json:"name"`
	Phone    string  `json:"phone"`
	By       string  `json:"by"`
	At       string  `json:"at"`
	CalledAt *string `json:"calledAt"`
}

type RequestRow struct {
	ID          int64    `json:"id"`
	Note        string   `json:"note"`
	Status      string   `json:"status"` // open | converted | declined
	Stage       string   `json:"stage"`  // open | waiting | call | done
	Reason      string   `json:"reason"`
	ProductID   *int64   `json:"productId"`
	ProductName string   `json:"productName"`
	AskCount    int      `json:"askCount"`
	LastAsked   string   `json:"lastAsked"`
	Asks        []AskRow `json:"asks"`
}

type ListDTO struct {
	Rows   []RequestRow `json:"rows"`
	Counts Counts       `json:"counts"`
	Total  int          `json:"total"`
}

func iso(t time.Time) string { return t.Format(time.RFC3339) }

func row(r Request) RequestRow {
	out := RequestRow{ID: r.ID, Note: r.Note, Status: r.Status, Stage: r.Stage, Reason: r.Reason, ProductID: r.ProductID,
		ProductName: r.ProductName, AskCount: len(r.Asks), LastAsked: iso(r.LastAsked), Asks: make([]AskRow, 0, len(r.Asks))}
	for _, a := range r.Asks {
		ar := AskRow{ID: a.ID, Name: a.Name, Phone: a.Phone, By: a.By, At: iso(a.At)}
		if a.CalledAt != nil {
			s := iso(*a.CalledAt)
			ar.CalledAt = &s
		}
		out.Asks = append(out.Asks, ar)
	}
	return out
}

func rows(rs []Request) []RequestRow {
	out := make([]RequestRow, 0, len(rs))
	for _, r := range rs {
		out = append(out, row(r))
	}
	return out
}

// storeErr maps the store's errors to API errors.
func storeErr(what string, err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return apperr.NotFound("request")
	case errors.Is(err, ErrClosed):
		return apperr.Conflict("this request is no longer open for that — reload the list")
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	return apperr.Internal(what, err)
}

func param(c echo.Context, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id < 0 {
		return 0, apperr.Validation("invalid " + name)
	}
	return id, nil
}

type askIn struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

func (a askIn) clean() (string, string) { return strings.TrimSpace(a.Name), strings.TrimSpace(a.Phone) }

type handlers struct{ p *Plugin }

// add notes a new request from the till.
func (h handlers) add(c echo.Context) error {
	var in struct {
		Note string `json:"note"`
		askIn
	}
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid request")
	}
	note := strings.TrimSpace(in.Note)
	if note == "" {
		return apperr.Validation("say what the customer asked for")
	}
	name, phone := in.clean()
	id, err := h.p.store.Add(c.Request().Context(), note, name, phone, middleware.CurrentUserID(c))
	if err != nil {
		return storeErr("add request", err)
	}
	return c.JSON(http.StatusOK, RequestRow{ID: id, Note: note, Status: "open", Stage: "open"})
}

// askAgain adds one more customer to a request still wanted (or declined).
func (h handlers) askAgain(c echo.Context) error {
	id, err := param(c, "id")
	if err != nil {
		return err
	}
	var in askIn
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid ask")
	}
	name, phone := in.clean()
	if err := h.p.store.AddAsk(c.Request().Context(), id, name, phone, middleware.CurrentUserID(c)); err != nil {
		return storeErr("ask again", err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) similar(c echo.Context) error {
	rs, err := h.p.store.Similar(c.Request().Context(), c.QueryParam("q"))
	if err != nil {
		return storeErr("find similar requests", err)
	}
	return c.JSON(http.StatusOK, rows(rs))
}
