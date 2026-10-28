package documents

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/middleware"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// TillService is a print service with what the job panel can offer: each
// size × type × colour × side on sale and its price bands.
type TillService struct {
	ID        int64       `json:"id"`
	ProductID int64       `json:"productId"`
	Name      string      `json:"name"`
	Kind      string      `json:"kind"`
	Category  string      `json:"category"`
	Offers    []TillOffer `json:"offers"`
	Extras    []ExtraDTO  `json:"extras"`
}

type TillOffer struct {
	Size     string     `json:"size"`     // '' = any size
	Material string     `json:"material"` // '' = no type
	Color    bool       `json:"color"`
	Double   bool       `json:"double"`
	Bands    []TillBand `json:"bands"` // ascending minQty
}

type TillBand struct {
	MinQty int    `json:"minQty"`
	Price  string `json:"price"`
}

type QuoteDTO struct {
	UnitPrice   string         `json:"unitPrice"`
	Total       string         `json:"total"`
	MinQty      int            `json:"minQty"` // the band reached
	Description string         `json:"description"`
	Sheets      int            `json:"sheets"`
	Cost        string         `json:"cost"`
	Components  []ComponentDTO `json:"components"`
}

type ComponentDTO struct {
	ProductID int64  `json:"productId"`
	Quantity  string `json:"quantity"`
}

type OwnUseRequest struct {
	ServiceID int64  `json:"serviceId"`
	Size      string `json:"size"`
	Material  string `json:"material"`
	Color     bool   `json:"color"`
	Double    bool   `json:"double"`
	Copies    int    `json:"copies"`
}

type JobsRequest struct {
	SaleID int64 `json:"saleId"`
	Jobs   []struct {
		ServiceID   int64   `json:"serviceId"`
		Description string  `json:"description"`
		Size        *string `json:"size"`
		Material    string  `json:"material"`
		Color       *bool   `json:"color"`
		Double      *bool   `json:"double"`
		Sheets      *int    `json:"sheets"`
	} `json:"jobs"`
}

func (h handlers) till(c echo.Context) error {
	ctx := c.Request().Context()
	svcs, err := h.p.store.Services(ctx, true)
	if err != nil {
		return err
	}
	out := make([]TillService, 0, len(svcs))
	for _, s := range svcs {
		xs, err := h.p.store.Extras(ctx, s.ID)
		if err != nil {
			return err
		}
		ts := TillService{ID: s.ID, ProductID: s.ProductID, Name: s.Name, Kind: s.Kind, Category: s.Category, Offers: []TillOffer{}, Extras: extraDTOs(xs)}
		prices, err := h.p.store.Prices(ctx, s.ID)
		if err != nil {
			return err
		}
		at := map[[4]string]int{}
		for _, p := range prices { // ordered by size, type, min_qty
			k := [4]string{sizeKey(p.Size), p.Material, strconv.FormatBool(p.Color), strconv.FormatBool(p.DoubleSide)}
			i, ok := at[k]
			if !ok {
				i = len(ts.Offers)
				at[k] = i
				ts.Offers = append(ts.Offers, TillOffer{Size: k[0], Material: p.Material, Color: p.Color, Double: p.DoubleSide, Bands: []TillBand{}})
			}
			ts.Offers[i].Bands = append(ts.Offers[i].Bands, TillBand{MinQty: p.MinQty, Price: p.UnitPrice.StringFixed(2)})
		}
		out = append(out, ts)
	}
	return c.JSON(http.StatusOK, out)
}

// Plain jobs (laminate, bind) are priced by size and type only: no ink, no sides.
func Plain(category string) bool { return category == "laminate" || category == "bind" }

// Description is the receipt line: "Photocopy A4 · Photo paper · B&W · 2-side",
// "Laminate A4 · Gloss".
func Description(sv *Service, o JobOptions) string {
	parts := []string{sv.Name}
	if o.Size != "" {
		parts[0] += " " + o.Size
	}
	if o.Material != "" {
		parts = append(parts, o.Material)
	}
	if !Plain(sv.Category) {
		parts = append(parts, map[bool]string{false: "B&W", true: "Colour"}[o.Color], map[bool]string{false: "1-side", true: "2-side"}[o.Double])
	}
	return strings.Join(parts, " · ")
}

func (h handlers) metered(c echo.Context, sid int64, o JobOptions) (*Service, *Quote, error) {
	sv, err := h.p.store.Service(c.Request().Context(), sid)
	if err != nil {
		return nil, nil, err
	}
	if !sv.IsActive || !sv.ProductOn {
		return nil, nil, apperr.Conflict(sv.Name + " is turned off")
	}
	if sv.Kind != "metered" {
		return nil, nil, apperr.Validation(sv.Name + " is priced at the till, not from a grid")
	}
	q, err := h.p.store.Quote(c.Request().Context(), sv, o)
	return sv, q, err
}

func (h handlers) quote(c echo.Context) error {
	sid, _ := strconv.ParseInt(c.QueryParam("service"), 10, 64)
	copies, _ := strconv.Atoi(c.QueryParam("copies"))
	o := JobOptions{Size: c.QueryParam("size"), Material: c.QueryParam("material"), Color: c.QueryParam("color") == "1", Double: c.QueryParam("double") == "1", Copies: copies}
	sv, q, err := h.metered(c, sid, o)
	if err != nil {
		return err
	}
	out := QuoteDTO{UnitPrice: q.Price.UnitPrice.StringFixed(2), MinQty: q.Price.MinQty, Description: Description(sv, o),
		Total: q.Price.UnitPrice.Mul(decimal.NewFromInt(int64(copies))).StringFixed(2), Sheets: q.Sheets, Cost: q.Cost.StringFixed(2),
		Components: make([]ComponentDTO, 0, len(q.Components))}
	for _, x := range q.Components {
		out.Components = append(out.Components, ComponentDTO{ProductID: x.ProductID, Quantity: x.Qty.String()})
	}
	return c.JSON(http.StatusOK, out)
}

// ownUse consumes what a job would use as the shop's own (no sale, no cash) and
// logs it — all or nothing.
func (h handlers) ownUse(c echo.Context) error {
	var req OwnUseRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid body")
	}
	o := JobOptions{Size: req.Size, Material: req.Material, Color: req.Color, Double: req.Double, Copies: req.Copies}
	sv, q, err := h.metered(c, req.ServiceID, o)
	if err != nil {
		return err
	}
	if len(q.Components) == 0 {
		return apperr.Validation(sv.Name + " uses no paper or toner to record")
	}
	ctx := c.Request().Context()
	uid := middleware.CurrentUserID(c)
	desc := Description(sv, o) + " ×" + strconv.Itoa(o.Copies)
	err = appdb.WithTx(ctx, h.p.core.DB, func(tx *sqlx.Tx) error {
		for _, x := range q.Components {
			if err := h.p.core.Stock.ConsumeTx(ctx, tx, stock.ConsumeInput{ProductID: x.ProductID, Quantity: x.Qty.String(),
				Reason: "own_use", Note: "Print & Copy shop use: " + desc}, uid); err != nil {
				var ae *apperr.AppError
				if errors.As(err, &ae) && ae.Status == http.StatusConflict {
					return apperr.Conflict("the count shows too little paper or toner for this — correct the count, then record it")
				}
				return err
			}
		}
		return h.p.store.RecordOwnUse(ctx, tx, sv, desc, o, q.Cost)
	})
	if err != nil {
		return err
	}
	h.audit(c, "documents_own_use", sv.ID, desc)
	return c.NoContent(http.StatusNoContent)
}

// jobs records the Print & Copy lines of a sale just made (best-effort, after it).
func (h handlers) jobs(c echo.Context) error {
	var req JobsRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid body")
	}
	if req.SaleID <= 0 || len(req.Jobs) == 0 {
		return apperr.Validation("nothing to record")
	}
	jobs := make([]SoldJob, 0, len(req.Jobs))
	for _, j := range req.Jobs {
		var sheets *decimal.Decimal
		if j.Sheets != nil {
			d := decimal.NewFromInt(int64(*j.Sheets))
			sheets = &d
		}
		size := j.Size
		if size != nil && *size == "" {
			size = nil
		}
		jobs = append(jobs, SoldJob{ServiceID: j.ServiceID, Description: j.Description, Size: size, Material: strings.TrimSpace(j.Material), Color: j.Color, Double: j.Double, Sheets: sheets})
	}
	if err := h.p.store.RecordSold(c.Request().Context(), req.SaleID, jobs); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
