package pharmacy

import (
	"errors"
	"net/http"

	"karots-suite/internal/apperr"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

type RecentSaleDTO struct {
	ID        int64  `json:"id"`
	Receipt   string `json:"receipt"`
	At        string `json:"at"`
	Total     string `json:"total"`
	Customer  string `json:"customer"`
	Items     string `json:"items"`
	Recorded  string `json:"recorded"` // the patient it is recorded for, "" = not yet
	Medicines int    `json:"medicines"`
}

// dispense records a sale's medicines for a patient (after the sale, or for a
// past receipt). lines tag a product with the script line it was added for.
func (h handlers) dispense(c echo.Context) error {
	var in struct {
		SaleID         int64 `json:"saleId"`
		PatientID      int64 `json:"patientId"`
		PrescriptionID int64 `json:"prescriptionId"`
		Lines          []struct {
			ProductID int64 `json:"productId"`
			RxItemID  int64 `json:"rxItemId"`
		} `json:"lines"`
	}
	if err := c.Bind(&in); err != nil || in.SaleID <= 0 || in.PatientID <= 0 {
		return apperr.Validation("say which sale and which patient")
	}
	tags := map[int64]int64{}
	for _, l := range in.Lines {
		if l.RxItemID > 0 {
			tags[l.ProductID] = l.RxItemID
		}
	}
	id, err := h.p.store.RecordDispense(c.Request().Context(), DispenseInput{SaleID: in.SaleID, PatientID: in.PatientID,
		PrescriptionID: in.PrescriptionID, Tags: tags, By: middleware.CurrentUserID(c)})
	if errors.Is(err, ErrNothingToRecord) {
		return apperr.Validation("there are no medicines on this receipt")
	}
	if err != nil {
		return fail("record dispense", err)
	}
	return c.JSON(http.StatusOK, map[string]int64{"id": id})
}

func (h handlers) recentSales(c echo.Context) error {
	rows, err := h.p.store.RecentSales(c.Request().Context(), c.QueryParam("q"), 20)
	if err != nil {
		return fail("list receipts", err)
	}
	out := make([]RecentSaleDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, RecentSaleDTO{ID: r.ID, Receipt: r.Receipt, At: *iso(&r.At), Total: r.Total.StringFixed(2),
			Customer: r.Customer, Items: r.Items, Recorded: r.Recorded, Medicines: r.Medicines})
	}
	return c.JSON(http.StatusOK, out)
}
