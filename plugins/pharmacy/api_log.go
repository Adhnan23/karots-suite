package pharmacy

import (
	"net/http"
	"strconv"
	"time"

	"karots-suite/internal/features/reports"

	"github.com/labstack/echo/v4"
)

type RefillDTO struct {
	ItemID     int64  `json:"itemId"`
	RxID       int64  `json:"rxId"`
	PatientID  int64  `json:"patientId"`
	Patient    string `json:"patient"`
	Phone      string `json:"phone"`
	Prescriber string `json:"prescriber"`
	Drug       string `json:"drug"`
	Dosage     string `json:"dosage"`
	IsRepeat   bool   `json:"isRepeat"`
	LastAt     string `json:"lastAt"`
	Due        string `json:"due"`
	Status     string `json:"status"` // overdue | soon
	CalledAt   string `json:"calledAt"`
}

type LogItemDTO struct {
	Drug       string `json:"drug"`
	Qty        string `json:"qty"`
	Days       int    `json:"days"`
	Controlled bool   `json:"controlled"`
}

type LogRowDTO struct {
	ID         int64        `json:"id"`
	At         string       `json:"at"`
	PatientID  int64        `json:"patientId"`
	Patient    string       `json:"patient"`
	Prescriber string       `json:"prescriber"`
	SaleID     *int64       `json:"saleId"`
	Receipt    string       `json:"receipt"`
	By         string       `json:"by"`
	Returned   bool         `json:"returned"`
	Controlled bool         `json:"controlled"`
	Items      []LogItemDTO `json:"items"`
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (h handlers) refills(c echo.Context) error {
	now := time.Now()
	rows, err := h.p.store.RefillsDue(c.Request().Context(), now)
	if err != nil {
		return fail("list refills", err)
	}
	out := make([]RefillDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, RefillDTO{ItemID: r.ID, RxID: r.RxID, PatientID: r.PatientID, Patient: r.PatientName, Phone: r.PatientPhone,
			Prescriber: r.Prescriber, Drug: r.Drug, Dosage: r.Dosage, IsRepeat: r.IsRepeat, LastAt: deref(iso(r.LastAt)),
			Due: *iso(&r.Due), Status: dueStatus(&r.Due, now), CalledAt: deref(iso(r.CalledAt))})
	}
	return c.JSON(http.StatusOK, out)
}

func (h handlers) called(c echo.Context) error {
	id, err := param(c, "id")
	if err != nil {
		return err
	}
	if err := h.p.store.MarkCalled(c.Request().Context(), id); err != nil {
		return fail("mark called", err)
	}
	return c.NoContent(http.StatusNoContent)
}

// dispenses is the dispensing log (?preset / from / to, controlled=1, patient=id).
func (h handlers) dispenses(c echo.Context) error {
	f := LogFilter{Controlled: c.QueryParam("controlled") == "1"}
	if v := c.QueryParam("patient"); v != "" {
		f.PatientID, _ = strconv.ParseInt(v, 10, 64)
		f.From, f.To = time.Time{}, time.Now().AddDate(1, 0, 0) // a patient's whole history
	} else {
		var err error
		if f.From, f.To, _, _, err = reports.ResolveRange(c.QueryParam("preset"), c.QueryParam("from"), c.QueryParam("to")); err != nil {
			return err
		}
	}
	rows, err := h.p.store.Dispenses(c.Request().Context(), f)
	if err != nil {
		return fail("list dispenses", err)
	}
	out := make([]LogRowDTO, 0, len(rows))
	for _, r := range rows {
		d := LogRowDTO{ID: r.ID, At: *iso(&r.At), PatientID: r.PatientID, Patient: r.Patient, Prescriber: r.Prescriber, SaleID: r.SaleID,
			Receipt: r.Receipt, By: r.By, Returned: r.Returned, Controlled: r.Controlled, Items: make([]LogItemDTO, 0, len(r.Items))}
		for _, it := range r.Items {
			d.Items = append(d.Items, LogItemDTO{Drug: it.Drug, Qty: it.Qty.String(), Days: it.Days, Controlled: it.Controlled})
		}
		out = append(out, d)
	}
	return c.JSON(http.StatusOK, out)
}
