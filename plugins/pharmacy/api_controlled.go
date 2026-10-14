package pharmacy

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

type ControlledDTO struct {
	ProductID  int64  `json:"productId"`
	Name       string `json:"name"`
	Schedule   string `json:"schedule"`
	Stock      string `json:"stock"`
	Unrecorded int    `json:"unrecorded"`
	OldRows    int    `json:"oldRows"`
}

type RegisterRowDTO struct {
	ID         int64  `json:"id"`
	At         string `json:"at"`
	Type       string `json:"type"`
	In         string `json:"in"`
	Out        string `json:"out"`
	Balance    string `json:"balance"`
	SaleID     *int64 `json:"saleId"`
	Reference  string `json:"reference"`
	Note       string `json:"note"`
	By         string `json:"by"`
	Patient    string `json:"patient"`
	Prescriber string `json:"prescriber"`
}

type OldEntryDTO struct {
	At         string `json:"at"`
	Direction  string `json:"direction"`
	Qty        string `json:"qty"`
	Patient    string `json:"patient"`
	Prescriber string `json:"prescriber"`
	Reference  string `json:"reference"`
	Notes      string `json:"notes"`
}

type RegisterDTO struct {
	ControlledDTO
	From    string           `json:"from"`
	Opening string           `json:"opening"`
	Rows    []RegisterRowDTO `json:"rows"`
	Old     []OldEntryDTO    `json:"old"`
}

type UnrecordedDTO struct {
	SaleID    int64  `json:"saleId"`
	Receipt   string `json:"receipt"`
	At        string `json:"at"`
	ProductID int64  `json:"productId"`
	Name      string `json:"name"`
	Qty       string `json:"qty"`
}

func controlledDTO(c ControlledRow) ControlledDTO {
	return ControlledDTO{ProductID: c.ProductID, Name: c.Name, Schedule: c.Schedule, Stock: c.Stock.String(),
		Unrecorded: c.Unrecorded, OldRows: c.OldRows}
}

func (h handlers) controlled(c echo.Context) error {
	rows, err := h.p.store.Controlled(c.Request().Context())
	if err != nil {
		return fail("list controlled drugs", err)
	}
	out := make([]ControlledDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, controlledDTO(r))
	}
	return c.JSON(http.StatusOK, out)
}

func (h handlers) drugRegister(c echo.Context) error {
	pid, err := param(c, "pid")
	if err != nil {
		return err
	}
	from, err := parseYMD(c.QueryParam("from"))
	if err != nil {
		return err
	}
	r, err := h.p.store.RegisterFor(c.Request().Context(), pid, from)
	if err != nil {
		return fail("load register", err)
	}
	out := RegisterDTO{ControlledDTO: controlledDTO(r.ControlledRow), From: ymd(from), Opening: r.Opening.String(),
		Rows: make([]RegisterRowDTO, 0, len(r.Rows)), Old: make([]OldEntryDTO, 0, len(r.Old))}
	for _, x := range r.Rows {
		d := RegisterRowDTO{ID: x.ID, At: *iso(&x.At), Type: x.Type, Balance: x.Balance.String(), SaleID: x.SaleID,
			Reference: x.Reference, Note: x.Note, By: x.By, Patient: x.Patient, Prescriber: x.Prescriber}
		if x.Qty.IsPositive() {
			d.In = x.Qty.String()
		} else {
			d.Out = x.Qty.Neg().String()
		}
		out.Rows = append(out.Rows, d)
	}
	for _, o := range r.Old {
		out.Old = append(out.Old, OldEntryDTO{At: *iso(&o.At), Direction: o.Direction, Qty: o.Qty.String(), Patient: o.Patient,
			Prescriber: o.Prescriber, Reference: o.Reference, Notes: o.Notes})
	}
	return c.JSON(http.StatusOK, out)
}

func (h handlers) unrecorded(c echo.Context) error {
	rows, err := h.p.store.Unrecorded(c.Request().Context())
	if err != nil {
		return fail("list controlled sales", err)
	}
	out := make([]UnrecordedDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, UnrecordedDTO{SaleID: r.SaleID, Receipt: r.Receipt, At: *iso(&r.At), ProductID: r.ProductID, Name: r.Name, Qty: r.Qty.String()})
	}
	return c.JSON(http.StatusOK, out)
}

// tillInfo flags the cart's controlled / prescription-only lines (?ids=1,2,3).
func (h handlers) tillInfo(c echo.Context) error {
	var ids []int64
	for _, s := range strings.Split(c.QueryParam("ids"), ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) > 500 {
		return apperr.Validation("too many products")
	}
	flags, err := h.p.store.Flags(c.Request().Context(), ids)
	if err != nil {
		return fail("load medicine flags", err)
	}
	out := make(map[string]MedicineFlags, len(flags))
	for id, f := range flags {
		out[strconv.FormatInt(id, 10)] = f
	}
	return c.JSON(http.StatusOK, out)
}

// productFlags is the product form's Pharmacy section value.
func (h handlers) productFlags(c echo.Context) error {
	pid, err := param(c, "pid")
	if err != nil {
		return err
	}
	flags, err := h.p.store.Flags(c.Request().Context(), []int64{pid})
	if err != nil {
		return fail("load medicine flags", err)
	}
	return c.JSON(http.StatusOK, flags[pid])
}

// detailRows: the till's ⓘ says a drug is controlled (and its schedule) or Rx-only.
func (p *Plugin) detailRows(ctx context.Context, id int64) ([]plugin.DetailRow, error) {
	flags, err := p.store.Flags(ctx, []int64{id})
	f, ok := flags[id]
	if err != nil || !ok {
		return nil, err
	}
	var rows []plugin.DetailRow
	if f.Controlled {
		v := "Yes — note the patient (💊)"
		if f.Schedule != "" {
			v = f.Schedule + " — note the patient (💊)"
		}
		rows = append(rows, plugin.DetailRow{Label: "⚠ Controlled drug", Value: v})
	}
	if f.RxOnly {
		rows = append(rows, plugin.DetailRow{Label: "℞ Prescription medicine", Value: "Ask for the prescription"})
	}
	return rows, nil
}

func decodeFlags(raw json.RawMessage) (MedicineFlags, error) {
	var f MedicineFlags
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, apperr.BadRequest("invalid pharmacy section")
	}
	if len(f.Schedule) > 60 || len(f.Dose) > 40 {
		return f, apperr.Validation("keep the schedule (60 letters) and the usual dose (40) short")
	}
	return f, nil
}

func (p *Plugin) validateExtra(_ context.Context, _ plugin.ExtraCtx, raw json.RawMessage) error {
	_, err := decodeFlags(raw)
	return err
}

func (p *Plugin) saveExtra(ctx context.Context, x plugin.ExtraCtx, raw json.RawMessage) error {
	f, err := decodeFlags(raw)
	if err != nil {
		return err
	}
	return p.store.SetFlags(ctx, x.ProductID, f)
}
