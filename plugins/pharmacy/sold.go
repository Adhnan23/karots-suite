package pharmacy

import (
	"context"
	"encoding/json"
	"errors"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
)

// recordSale writes the dispense of a sale with a patient on it, inside the
// sale's own transaction (core's sale hook; the patient is the till's sale meta,
// a script line's tag is on its cart line) — PHA-5. Anything off (no such
// patient, nothing medicinal, already recorded) is skipped, never refused: a
// sale is never stopped; the receipt can still be recorded by hand.
func (p *Plugin) recordSale(ctx context.Context, tx *sqlx.Tx, ev plugin.SaleLines) error {
	var sale struct {
		PatientID int64 `json:"patientId"`
	}
	if json.Unmarshal(ev.Data, &sale) != nil || sale.PatientID <= 0 {
		return nil
	}
	tags := map[int64]int64{}
	for _, l := range ev.Lines {
		var d struct {
			RxItemID int64 `json:"rxItemId"`
		}
		if json.Unmarshal(l.Data, &d) == nil && d.RxItemID > 0 {
			tags[l.ProductID] = d.RxItemID
		}
	}
	_, err := p.store.RecordDispense(appdb.InTx(ctx, tx), DispenseInput{SaleID: ev.SaleID, PatientID: sale.PatientID, Tags: tags, By: ev.ActorID})
	var ae *apperr.AppError
	if errors.Is(err, ErrNothingToRecord) || errors.As(err, &ae) {
		return nil
	}
	return err
}
