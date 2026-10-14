package pharmacy

import (
	"context"
	"sort"
	"time"

	"karots-suite/internal/apperr"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// Refill is one drug on the call list.
type Refill struct {
	ScriptItem
	PatientName  string
	PatientPhone string
	Prescriber   string
	Due          time.Time
}

// RefillsDue is the call list: drugs of active scripts inside the lead window
// (repeats stay, one-offs until the grace runs out), not called in the last few
// days; soonest run-out first. Computed in Go over every active line.
// ponytail: loads every active line; fine at pharmacy scale, a SQL filter if it grows.
func (s *Store) RefillsDue(ctx context.Context, now time.Time) ([]Refill, error) {
	var items []ScriptItem
	if err := s.db.SelectContext(ctx, &items, itemSelect+` WHERE rx.active ORDER BY i.id`); err != nil {
		return nil, err
	}
	var out []Refill
	pids := map[int64]bool{}
	for _, it := range items {
		due := it.NextDue()
		if due == nil || !refillIsDue(*due, it.IsRepeat, now) {
			continue
		}
		if it.CalledAt != nil && now.Before(it.CalledAt.AddDate(0, 0, calledDays)) {
			continue
		}
		out = append(out, Refill{ScriptItem: it, Due: *due})
		pids[it.PatientID] = true
	}
	if len(out) == 0 {
		return []Refill{}, nil
	}
	ids := make([]int64, 0, len(pids))
	for id := range pids {
		ids = append(ids, id)
	}
	var who []struct {
		ID    int64  `db:"id"`
		Name  string `db:"name"`
		Phone string `db:"phone"`
	}
	if err := s.db.SelectContext(ctx, &who, `SELECT p.id, c.name, COALESCE(c.phone, '') AS phone
		FROM pharmacy_patients p JOIN customers c ON c.id = p.customer_id WHERE p.id = ANY($1)`, pq.Array(ids)); err != nil {
		return nil, err
	}
	var docs []struct {
		ID  int64  `db:"id"`
		Doc string `db:"prescriber_name"`
	}
	if err := s.db.SelectContext(ctx, &docs, `SELECT id, prescriber_name FROM pharmacy_prescriptions WHERE patient_id = ANY($1)`, pq.Array(ids)); err != nil {
		return nil, err
	}
	for i := range out {
		for _, w := range who {
			if w.ID == out[i].PatientID {
				out[i].PatientName, out[i].PatientPhone = w.Name, w.Phone
			}
		}
		for _, d := range docs {
			if d.ID == out[i].RxID {
				out[i].Prescriber = d.Doc
			}
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Due.Before(out[b].Due) })
	return out, nil
}

// MarkCalled stamps a drug line as phoned about (off the call list for a few days).
func (s *Store) MarkCalled(ctx context.Context, itemID int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE pharmacy_prescription_items SET called_at = now() WHERE id = $1`, itemID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("prescription line")
	}
	return nil
}

// LogFilter narrows the dispensing log; PatientID 0 = everyone.
type LogFilter struct {
	From, To   time.Time
	Controlled bool
	PatientID  int64
}

// LogRow is one dispense with what was handed over.
type LogRow struct {
	ID         int64     `db:"id"`
	At         time.Time `db:"dispensed_at"`
	PatientID  int64     `db:"patient_id"`
	Patient    string    `db:"patient"`
	Prescriber string    `db:"prescriber"`
	SaleID     *int64    `db:"sale_id"`
	Receipt    string    `db:"sale_receipt"`
	By         string    `db:"by"`
	Returned   bool      `db:"returned"` // the sale was returned (in part)
	Controlled bool      `db:"controlled"`
	Items      []LogItem `db:"-"`
}

type LogItem struct {
	DispenseID int64           `db:"dispense_id"`
	Drug       string          `db:"drug"`
	Qty        decimal.Decimal `db:"quantity"`
	Days       int             `db:"days_supply"`
	Controlled bool            `db:"controlled"`
}

func (s *Store) Dispenses(ctx context.Context, f LogFilter) ([]LogRow, error) {
	out := []LogRow{}
	if err := s.db.SelectContext(ctx, &out, `
		SELECT d.id, d.dispensed_at, d.patient_id, COALESCE(NULLIF(d.patient_name, ''), c.name) AS patient,
		       COALESCE(NULLIF(d.prescriber_name, ''), rx.prescriber_name, '') AS prescriber,
		       d.sale_id, d.sale_receipt, COALESCE(u.name, '') AS by,
		       EXISTS (SELECT 1 FROM sale_items si WHERE si.sale_id = d.sale_id AND si.returned_qty > 0) AS returned,
		       EXISTS (SELECT 1 FROM pharmacy_dispense_items di JOIN pharmacy_controlled_products k ON k.product_id = di.product_id
		               WHERE di.dispense_id = d.id) AS controlled
		FROM pharmacy_dispenses d
		JOIN pharmacy_patients pp ON pp.id = d.patient_id JOIN customers c ON c.id = pp.customer_id
		LEFT JOIN pharmacy_prescriptions rx ON rx.id = d.prescription_id
		LEFT JOIN users u ON u.id = d.dispensed_by
		WHERE d.dispensed_at >= $1 AND d.dispensed_at < $2 AND ($3 = 0 OR d.patient_id = $3)
		ORDER BY d.dispensed_at DESC, d.id DESC LIMIT 2000`, f.From, f.To, f.PatientID); err != nil {
		return nil, err
	}
	if f.Controlled {
		kept := out[:0]
		for _, r := range out {
			if r.Controlled {
				kept = append(kept, r)
			}
		}
		out = kept
	}
	if len(out) == 0 {
		return out, nil
	}
	ids := make([]int64, len(out))
	for i, r := range out {
		ids[i] = r.ID
	}
	var items []LogItem
	if err := s.db.SelectContext(ctx, &items, `
		SELECT di.dispense_id, COALESCE(NULLIF(di.drug_text, ''), p.name, '') AS drug, di.quantity, di.days_supply,
		       k.product_id IS NOT NULL AS controlled
		FROM pharmacy_dispense_items di LEFT JOIN products p ON p.id = di.product_id
		LEFT JOIN pharmacy_controlled_products k ON k.product_id = di.product_id
		WHERE di.dispense_id = ANY($1) ORDER BY di.id`, pq.Array(ids)); err != nil {
		return nil, err
	}
	at := map[int64]int{}
	for i, r := range out {
		at[r.ID] = i
	}
	for _, it := range items {
		out[at[it.DispenseID]].Items = append(out[at[it.DispenseID]].Items, it)
	}
	return out, nil
}
