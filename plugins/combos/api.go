package combos

import (
	"net/http"
	"strconv"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/reports"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// The plugin's own JSON (core's types.gen.ts is core-only); mirrored by hand in
// web/src/plugins/combos/types.ts. Money is decimal strings, dates YYYY-MM-DD.

type MemberDTO struct {
	ProductID      int64  `json:"productId"`
	Name           string `json:"name"`
	Qty            string `json:"qty"`
	SellingPrice   string `json:"sellingPrice"`
	WholesalePrice string `json:"wholesalePrice"`
	Active         bool   `json:"active"`
	IsService      bool   `json:"isService"`
	Stock          string `json:"stock"`
}

type ComboDTO struct {
	ID            int64       `json:"id"`
	Name          string      `json:"name"`
	DiscountType  string      `json:"discountType"`
	DiscountValue string      `json:"discountValue"`
	Active        bool        `json:"active"`
	StartsOn      string      `json:"startsOn"`
	EndsOn        string      `json:"endsOn"`
	Window        string      `json:"window"` // on | scheduled | ended
	Members       []MemberDTO `json:"members"`
	FullPrice     string      `json:"fullPrice"` // one set at retail
	Problem       string      `json:"problem"`   // "" = can be added
	Warnings      []string    `json:"warnings"`
	SoldSets      string      `json:"soldSets"`
	SoldNet       string      `json:"soldNet"`
	LastSold      *string     `json:"lastSold"`
}

type ComboRequest struct {
	Name          string `json:"name"`
	DiscountType  string `json:"discountType"`
	DiscountValue string `json:"discountValue"`
	StartsOn      string `json:"startsOn"`
	EndsOn        string `json:"endsOn"`
	Items         []struct {
		ProductID int64  `json:"productId"`
		Qty       string `json:"qty"`
	} `json:"items"`
}

type SoldRequest struct {
	SaleID int64 `json:"saleId"`
	Combos []struct {
		ComboID  int64  `json:"comboId"`
		Sets     string `json:"sets"`
		Net      string `json:"net"`
		Discount string `json:"discount"`
	} `json:"combos"`
}

type ReportRowDTO struct {
	ComboID  *int64 `json:"comboId"`
	Name     string `json:"name"`
	Sets     string `json:"sets"`
	Net      string `json:"net"`
	Discount string `json:"discount"`
	Bills    int    `json:"bills"`
}

type ReportDTO struct {
	Rows   []ReportRowDTO `json:"rows"`
	Totals ReportRowDTO   `json:"totals"`
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func dto(c Combo) ComboDTO {
	out := ComboDTO{ID: c.ID, Name: c.Name, DiscountType: c.DiscountType, DiscountValue: c.DiscountValue.String(),
		Active: c.IsActive, StartsOn: str(c.StartsOn), EndsOn: str(c.EndsOn), Window: c.Window,
		Members: make([]MemberDTO, 0, len(c.Items)), FullPrice: c.FullPrice.StringFixed(2), Problem: c.Problems(),
		Warnings: c.Warnings(), SoldSets: c.SoldSets.String(), SoldNet: c.SoldNet.StringFixed(2)}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}
	if c.LastSold != nil {
		s := c.LastSold.Format(time.RFC3339)
		out.LastSold = &s
	}
	for _, it := range c.Items {
		out.Members = append(out.Members, MemberDTO{ProductID: it.ProductID, Name: it.Name, Qty: it.Qty.String(),
			SellingPrice: it.SellingPrice.StringFixed(2), WholesalePrice: it.WholesalePrice.StringFixed(2),
			Active: it.Active, IsService: it.IsService, Stock: it.Stock.String()})
	}
	return out
}

func dtos(cs []Combo) []ComboDTO {
	out := make([]ComboDTO, 0, len(cs))
	for _, c := range cs {
		out = append(out, dto(c))
	}
	return out
}

func dec(s, what string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.Zero, nil
	}
	v, err := decimal.NewFromString(s)
	if err != nil {
		return v, apperr.Validation(what + " must be a number")
	}
	return v, nil
}

func (r ComboRequest) input() (Input, error) {
	in := Input{Name: r.Name, DiscountType: r.DiscountType, StartsOn: r.StartsOn, EndsOn: r.EndsOn}
	var err error
	if in.DiscountValue, err = dec(r.DiscountValue, "The discount"); err != nil {
		return in, err
	}
	for _, it := range r.Items {
		q, err := dec(it.Qty, "A quantity")
		if err != nil {
			return in, err
		}
		in.Items = append(in.Items, ItemInput{ProductID: it.ProductID, Qty: q})
	}
	return in, nil
}

type handlers struct{ p *Plugin }

func id(c echo.Context) (int64, error) {
	v, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return 0, apperr.BadRequest("invalid id")
	}
	return v, nil
}

func (h handlers) audit(c echo.Context, action string, cid int64, detail string) {
	if h.p.core.Audit != nil {
		h.p.core.Audit.Record(c.Request().Context(), middleware.CurrentUserID(c), action, "combo", strconv.FormatInt(cid, 10), detail)
	}
}

func (h handlers) list(c echo.Context) error {
	cs, err := h.p.store.List(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, dtos(cs))
}

func (h handlers) till(c echo.Context) error {
	cs, err := h.p.store.Till(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, dtos(cs))
}

func (h handlers) save(c echo.Context, cid int64) error {
	var req ComboRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid body")
	}
	in, err := req.input()
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	action := "combo_update"
	if cid == 0 {
		action = "combo_create"
		cid, err = h.p.store.Create(ctx, in)
	} else {
		err = h.p.store.Update(ctx, cid, in)
	}
	if err != nil {
		return err
	}
	h.audit(c, action, cid, in.Name)
	cb, err := h.p.store.Get(ctx, cid)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, dto(cb))
}

func (h handlers) create(c echo.Context) error { return h.save(c, 0) }

func (h handlers) update(c echo.Context) error {
	cid, err := id(c)
	if err != nil {
		return err
	}
	return h.save(c, cid)
}

func (h handlers) setActive(c echo.Context) error {
	cid, err := id(c)
	if err != nil {
		return err
	}
	var req struct {
		Active bool `json:"active"`
	}
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid body")
	}
	if err := h.p.store.SetActive(c.Request().Context(), cid, req.Active); err != nil {
		return err
	}
	h.audit(c, map[bool]string{true: "combo_enable", false: "combo_disable"}[req.Active], cid, "")
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) remove(c echo.Context) error {
	cid, err := id(c)
	if err != nil {
		return err
	}
	if err := h.p.store.Delete(c.Request().Context(), cid); err != nil {
		return err
	}
	h.audit(c, "combo_delete", cid, "")
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) sold(c echo.Context) error {
	var req SoldRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid body")
	}
	rows := make([]Sold, 0, len(req.Combos))
	for _, x := range req.Combos {
		var s Sold
		var err error
		s.ComboID = x.ComboID
		if s.Sets, err = dec(x.Sets, "sets"); err != nil {
			return err
		}
		if s.Net, err = dec(x.Net, "net"); err != nil {
			return err
		}
		if s.Discount, err = dec(x.Discount, "discount"); err != nil {
			return err
		}
		rows = append(rows, s)
	}
	if err := h.p.store.RecordSold(c.Request().Context(), req.SaleID, rows); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func reportRow(r ReportRow) ReportRowDTO {
	return ReportRowDTO{ComboID: r.ComboID, Name: r.Name, Sets: r.Sets.String(), Net: r.Net.StringFixed(2),
		Discount: r.Discount.StringFixed(2), Bills: r.Bills}
}

func (h handlers) report(c echo.Context) error {
	// The period the core reports use (preset, or from / to inclusive days).
	from, to, _, _, err := reports.ResolveRange(c.QueryParam("preset"), c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return apperr.Validation(err.Error())
	}
	r, err := h.p.store.Report(c.Request().Context(), from, to)
	if err != nil {
		return err
	}
	out := ReportDTO{Rows: make([]ReportRowDTO, 0, len(r.Rows)), Totals: reportRow(r.Totals)}
	for _, x := range r.Rows {
		out.Rows = append(out.Rows, reportRow(x))
	}
	return c.JSON(http.StatusOK, out)
}
