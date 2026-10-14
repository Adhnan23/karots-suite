package pharmacy

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"karots-suite/internal/apperr"

	"github.com/shopspring/decimal"
)

// slot is a script line a sale can fill.
type slot struct {
	ID        int64
	GroupID   *int64
	ProductID *int64
	PerDay    decimal.Decimal
}

// saleLine is one product on the sale, its lines summed, net of returns.
type saleLine struct {
	ProductID int64           `db:"product_id"`
	Name      string          `db:"name"`
	Qty       decimal.Decimal `db:"qty"`
	GroupID   *int64          `db:"group_id"`
	Medicine  bool            `db:"medicine"` // grouped, Rx-only or controlled
}

// part is one dispense row: tablets of a product for a script line (0 = off-script).
type part struct {
	RxItemID  int64
	ProductID int64
	Name      string
	Qty       decimal.Decimal
	Days      int
}

func same(a, b *int64) bool { return a != nil && b != nil && *a == *b }

// allocate fills script lines from the sale: a line the cashier tagged goes to
// its script line, an untagged one to the first line of its medicine group (or
// product); other medicines stay off-script. Days = the script line's tablets ÷
// doses a day, carried on each of its rows (so the latest row has the total).
func allocate(slots []slot, lines []saleLine, tags map[int64]int64) []part {
	var parts []part
	total := map[int64]decimal.Decimal{}
	for _, l := range lines {
		if !l.Qty.IsPositive() {
			continue
		}
		rx := int64(0)
		for _, s := range slots {
			if tags[l.ProductID] == s.ID {
				rx = s.ID
				break
			}
		}
		if rx == 0 {
			for _, s := range slots {
				if same(s.GroupID, l.GroupID) || (s.ProductID != nil && *s.ProductID == l.ProductID) {
					rx = s.ID
					break
				}
			}
		}
		if rx == 0 && !l.Medicine {
			continue
		}
		parts = append(parts, part{RxItemID: rx, ProductID: l.ProductID, Name: l.Name, Qty: l.Qty})
		total[rx] = total[rx].Add(l.Qty)
	}
	for i, p := range parts {
		for _, s := range slots {
			if s.ID == p.RxItemID {
				parts[i].Days = supplyDays(total[s.ID], s.PerDay)
			}
		}
	}
	return parts
}

// DispenseInput records what a sale handed to a patient. Tags maps a product on
// the sale to the script line the cashier added it for.
type DispenseInput struct {
	SaleID         int64
	PatientID      int64
	PrescriptionID int64 // 0 = no script
	Tags           map[int64]int64
	By             int64
}

// ErrNothingToRecord: the sale has no medicine (nothing for the patient's record).
var ErrNothingToRecord = errors.New("no medicines on this receipt")

// RecordDispense writes the dispense for a sale: each script line gets the
// tablets sold for it and the days they cover; the patient's and prescriber's
// names are kept as they are now. One record per sale.
func (s *Store) RecordDispense(ctx context.Context, in DispenseInput) (int64, error) {
	pat, err := s.Patient(ctx, in.PatientID)
	if err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck
	// One record per sale (the lock stops a double tap recording twice).
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('pharmacy_dispense'), $1::int)`, in.SaleID); err != nil {
		return 0, err
	}
	var who string
	err = tx.GetContext(ctx, &who, `SELECT COALESCE(NULLIF(d.patient_name, ''), c.name) FROM pharmacy_dispenses d
		JOIN pharmacy_patients pp ON pp.id = d.patient_id JOIN customers c ON c.id = pp.customer_id
		WHERE d.sale_id = $1 LIMIT 1`, in.SaleID)
	if err == nil {
		return 0, apperr.Conflict("this receipt is already recorded for " + who)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	var receipt string
	if err := tx.GetContext(ctx, &receipt, `SELECT receipt_no FROM sales WHERE id = $1 AND status <> 'void'`, in.SaleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, apperr.NotFound("sale")
		}
		return 0, err
	}
	var slots []slot
	prescriber := ""
	if in.PrescriptionID > 0 {
		sc, err := s.Script(ctx, in.PrescriptionID)
		if err != nil {
			return 0, err
		}
		if sc.PatientID != in.PatientID {
			return 0, apperr.Validation("that prescription is another patient's")
		}
		prescriber = sc.PrescriberName
		for _, it := range sc.Items {
			slots = append(slots, slot{ID: it.ID, GroupID: it.GroupID, ProductID: it.ProductID, PerDay: it.PerDay})
		}
	}
	var lines []saleLine
	if err := s.db.SelectContext(ctx, &lines, `
		SELECT si.product_id, p.name, SUM(si.quantity - si.returned_qty) AS qty, mem.group_id,
		       (mem.group_id IS NOT NULL OR EXISTS (SELECT 1 FROM pharmacy_medicines m WHERE m.product_id = si.product_id)
		        OR EXISTS (SELECT 1 FROM pharmacy_controlled_products c WHERE c.product_id = si.product_id)) AS medicine
		FROM sale_items si JOIN products p ON p.id = si.product_id
		LEFT JOIN LATERAL (SELECT t.group_id FROM alt_members m JOIN alt_tiers t ON t.id = m.tier_id
			WHERE m.product_id = si.product_id) mem ON true
		WHERE si.sale_id = $1 AND NOT p.is_service
		GROUP BY si.product_id, p.name, mem.group_id ORDER BY MIN(si.id)`, in.SaleID); err != nil {
		return 0, err
	}
	parts := allocate(slots, lines, in.Tags)
	if len(parts) == 0 {
		return 0, ErrNothingToRecord
	}
	days := 0
	for _, p := range parts {
		days = max(days, p.Days)
	}

	var rx *int64
	if in.PrescriptionID > 0 {
		rx = &in.PrescriptionID
	}
	var id int64
	if err := tx.GetContext(ctx, &id, `
		INSERT INTO pharmacy_dispenses (patient_id, prescription_id, sale_id, sale_receipt, dispensed_by, days_supply, patient_name, prescriber_name)
		VALUES ($1, $2, $3, $4, NULLIF($5, 0), $6, $7, $8) RETURNING id`,
		in.PatientID, rx, in.SaleID, receipt, in.By, days, pat.Name, prescriber); err != nil {
		return 0, err
	}
	for _, p := range parts {
		var item *int64
		if p.RxItemID > 0 {
			item = &p.RxItemID
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO pharmacy_dispense_items (dispense_id, product_id, drug_text, quantity, prescription_item_id, days_supply)
			VALUES ($1, $2, $3, $4, $5, $6)`, id, p.ProductID, p.Name, p.Qty, item, p.Days); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// RecentSale is a receipt the counter can record a dispense against.
type RecentSale struct {
	ID        int64           `db:"id"`
	Receipt   string          `db:"receipt_no"`
	At        time.Time       `db:"created_at"`
	Total     decimal.Decimal `db:"total"`
	Customer  string          `db:"customer"`
	Items     string          `db:"items"`
	Recorded  string          `db:"recorded"` // the patient it is recorded for, "" = not yet
	Medicines int             `db:"medicines"`
}

// RecentSales lists the last receipts (q = a receipt number or customer name).
func (s *Store) RecentSales(ctx context.Context, q string, limit int) ([]RecentSale, error) {
	out := []RecentSale{}
	q = strings.TrimSpace(q)
	err := s.db.SelectContext(ctx, &out, `
		SELECT s.id, s.receipt_no, s.created_at, s.total, COALESCE(c.name, '') AS customer,
		       COALESCE((SELECT string_agg(p.name, ', ' ORDER BY si.id) FROM sale_items si JOIN products p ON p.id = si.product_id
		                 WHERE si.sale_id = s.id), '') AS items,
		       COALESCE((SELECT COALESCE(NULLIF(d.patient_name, ''), dc.name) FROM pharmacy_dispenses d
		                 JOIN pharmacy_patients dp ON dp.id = d.patient_id JOIN customers dc ON dc.id = dp.customer_id
		                 WHERE d.sale_id = s.id LIMIT 1), '') AS recorded,
		       (SELECT COUNT(*) FROM sale_items si WHERE si.sale_id = s.id AND (
		            EXISTS (SELECT 1 FROM alt_members m WHERE m.product_id = si.product_id)
		         OR EXISTS (SELECT 1 FROM pharmacy_medicines m WHERE m.product_id = si.product_id)
		         OR EXISTS (SELECT 1 FROM pharmacy_controlled_products k WHERE k.product_id = si.product_id))) AS medicines
		FROM sales s LEFT JOIN customers c ON c.id = s.customer_id
		WHERE s.status <> 'void'
		  AND ($1 = '' OR s.receipt_no ILIKE '%' || $1 || '%' OR c.name ILIKE '%' || $1 || '%')
		ORDER BY s.created_at DESC, s.id DESC LIMIT $2`, q, limit)
	return out, err
}
