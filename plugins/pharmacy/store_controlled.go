package pharmacy

import (
	"context"
	"strings"
	"time"

	"karots-suite/internal/apperr"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// MedicineFlags is what the pharmacy marks on a product: controlled (+ schedule)
// and prescription-only — hints at the till, never a block. Medicine (read-only)
// = grouped, Rx-only or controlled: the till offers Days… on its line.
type MedicineFlags struct {
	Controlled bool   `db:"controlled" json:"controlled"`
	Schedule   string `db:"schedule"   json:"schedule"`
	RxOnly     bool   `db:"rx_only"    json:"rxOnly"`
	Medicine   bool   `db:"medicine"   json:"medicine"`
	Dose       string `db:"dose"       json:"dose"` // usual dose, "" = none
	Note       string `db:"note"       json:"note"` // usual instruction (After meals …)
}

// Flags maps each given product with a mark to its flags.
func (s *Store) Flags(ctx context.Context, ids []int64) (map[int64]MedicineFlags, error) {
	var rows []struct {
		ID int64 `db:"id"`
		MedicineFlags
	}
	err := s.db.SelectContext(ctx, &rows, `
		SELECT p.id, c.product_id IS NOT NULL AS controlled, COALESCE(c.schedule, '') AS schedule, COALESCE(m.rx_only, false) AS rx_only,
		       true AS medicine, COALESCE(m.dose, '') AS dose, COALESCE(m.note, '') AS note
		FROM unnest($1::bigint[]) AS p(id)
		LEFT JOIN pharmacy_controlled_products c ON c.product_id = p.id
		LEFT JOIN pharmacy_medicines m ON m.product_id = p.id
		WHERE c.product_id IS NOT NULL OR m.rx_only OR m.dose <> '' OR m.note <> '' OR EXISTS (SELECT 1 FROM alt_members a WHERE a.product_id = p.id)`, pq.Array(ids))
	out := make(map[int64]MedicineFlags, len(rows))
	for _, r := range rows {
		out[r.ID] = r.MedicineFlags
	}
	return out, err
}

// SetFlags saves a product's marks (the product form's Pharmacy section).
func (s *Store) SetFlags(ctx context.Context, productID int64, f MedicineFlags) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	sched := strings.TrimSpace(f.Schedule)
	q, args := `DELETE FROM pharmacy_controlled_products WHERE product_id = $1`, []any{productID}
	if f.Controlled {
		q, args = `INSERT INTO pharmacy_controlled_products (product_id, schedule) VALUES ($1, $2)
			ON CONFLICT (product_id) DO UPDATE SET schedule = EXCLUDED.schedule`, []any{productID, sched}
	}
	if _, err := tx.ExecContext(ctx, q, args...); err != nil {
		return err
	}
	// A controlled drug is always prescription-only.
	if _, err := tx.ExecContext(ctx, `INSERT INTO pharmacy_medicines (product_id, rx_only, dose, note) VALUES ($1, $2, $3, $4)
		ON CONFLICT (product_id) DO UPDATE SET rx_only = EXCLUDED.rx_only, dose = EXCLUDED.dose, note = EXCLUDED.note`,
		productID, f.RxOnly || f.Controlled, strings.TrimSpace(f.Dose), strings.TrimSpace(f.Note)); err != nil {
		return err
	}
	if err := learnDose(ctx, tx, f.Dose, decimal.Zero); err != nil {
		return err
	}
	return tx.Commit()
}

// ControlledRow is one controlled drug on the register list.
type ControlledRow struct {
	ProductID  int64           `db:"product_id"`
	Name       string          `db:"name"`
	Schedule   string          `db:"schedule"`
	Stock      decimal.Decimal `db:"stock"`
	Unrecorded int             `db:"unrecorded"` // sales still needing a record
	OldRows    int             `db:"old_rows"`   // the karots-pos hand-kept register
}

// unrecordedSales: sales of a controlled drug (since it was marked, net of
// returns) with no dispense record — the register's OUT row has no patient.
const unrecordedSales = `
	SELECT s.id AS sale_id, s.receipt_no, s.created_at, si.product_id, p.name, SUM(si.quantity - si.returned_qty) AS qty
	FROM sale_items si
	JOIN sales s ON s.id = si.sale_id AND s.status <> 'void'
	JOIN pharmacy_controlled_products c ON c.product_id = si.product_id AND s.created_at >= c.created_at
	JOIN products p ON p.id = si.product_id
	WHERE NOT EXISTS (SELECT 1 FROM pharmacy_dispenses d WHERE d.sale_id = s.id)
	GROUP BY s.id, s.receipt_no, s.created_at, si.product_id, p.name
	HAVING SUM(si.quantity - si.returned_qty) > 0`

func (s *Store) Controlled(ctx context.Context) ([]ControlledRow, error) {
	out := []ControlledRow{}
	err := s.db.SelectContext(ctx, &out, `
		SELECT c.product_id, p.name, c.schedule, COALESCE(st.quantity, 0) AS stock,
		       (SELECT COUNT(*) FROM (`+unrecordedSales+`) u WHERE u.product_id = c.product_id) AS unrecorded,
		       (SELECT COUNT(*) FROM pharmacy_cd_ledger l WHERE l.product_id = c.product_id) AS old_rows
		FROM pharmacy_controlled_products c JOIN products p ON p.id = c.product_id
		LEFT JOIN stock st ON st.product_id = c.product_id
		ORDER BY p.name`)
	return out, err
}

// Unrecorded is a controlled sale with no patient on record yet.
type Unrecorded struct {
	SaleID    int64           `db:"sale_id"`
	Receipt   string          `db:"receipt_no"`
	At        time.Time       `db:"created_at"`
	ProductID int64           `db:"product_id"`
	Name      string          `db:"name"`
	Qty       decimal.Decimal `db:"qty"`
}

func (s *Store) Unrecorded(ctx context.Context) ([]Unrecorded, error) {
	out := []Unrecorded{}
	err := s.db.SelectContext(ctx, &out, `SELECT * FROM (`+unrecordedSales+`) u ORDER BY created_at DESC, sale_id DESC`)
	return out, err
}

// RegisterRow is one line of a controlled drug's register: a core stock movement
// (IN > 0, OUT < 0) with the balance after it; a sale names the patient and
// prescriber from its dispense record (blank = still needs a record).
type RegisterRow struct {
	ID         int64           `db:"id"`
	At         time.Time       `db:"created_at"`
	Type       string          `db:"type"`
	Qty        decimal.Decimal `db:"quantity"`
	SaleID     *int64          `db:"sale_id"`
	Reference  string          `db:"reference"`
	Note       string          `db:"note"`
	By         string          `db:"by"`
	Patient    string          `db:"patient"`
	Prescriber string          `db:"prescriber"`
	Balance    decimal.Decimal `db:"-"`
}

// OldEntry is a row of the karots-pos hand-kept register (read-only history).
type OldEntry struct {
	At         time.Time       `db:"entry_at"`
	Direction  string          `db:"direction"`
	Qty        decimal.Decimal `db:"quantity"`
	Patient    string          `db:"patient_name"`
	Prescriber string          `db:"prescriber_name"`
	Reference  string          `db:"reference"`
	Notes      string          `db:"notes"`
}

// Register is a controlled drug's in / out record from core stock movements.
type Register struct {
	ControlledRow
	From    *time.Time
	Opening decimal.Decimal // stock before the first row shown
	Rows    []RegisterRow
	Old     []OldEntry
}

// RegisterFor reads the drug's movements from `from` (nil = all), oldest first.
// The opening balance is today's stock less everything since, so stock set
// without a movement (an opening count) is in it and the last balance = stock.
func (s *Store) RegisterFor(ctx context.Context, productID int64, from *time.Time) (Register, error) {
	var r Register
	if err := s.db.GetContext(ctx, &r.ControlledRow, `
		SELECT p.id AS product_id, p.name, COALESCE(c.schedule, '') AS schedule, COALESCE(st.quantity, 0) AS stock,
		       0 AS unrecorded, (SELECT COUNT(*) FROM pharmacy_cd_ledger l WHERE l.product_id = p.id) AS old_rows
		FROM products p LEFT JOIN pharmacy_controlled_products c ON c.product_id = p.id
		LEFT JOIN stock st ON st.product_id = p.id WHERE p.id = $1`, productID); err != nil {
		return r, apperr.NotFound("product")
	}
	r.From = from
	if err := s.db.SelectContext(ctx, &r.Rows, `
		SELECT m.id, m.created_at, m.type::text AS type, m.quantity,
		       CASE WHEN m.reference_type = 'sale' THEN m.reference_id END AS sale_id,
		       COALESCE(CASE WHEN m.reference_type = 'sale' THEN s.receipt_no
		                     WHEN m.reference_id IS NOT NULL THEN m.reference_type || ' #' || m.reference_id END, '') AS reference,
		       COALESCE(m.note, '') AS note, COALESCE(u.name, '') AS by,
		       CASE WHEN m.type = 'sale' THEN COALESCE(NULLIF(d.patient_name, ''), dc.name, '') ELSE '' END AS patient,
		       CASE WHEN m.type = 'sale' THEN COALESCE(NULLIF(d.prescriber_name, ''), rx.prescriber_name, '') ELSE '' END AS prescriber
		FROM stock_movements m
		LEFT JOIN users u ON u.id = m.user_id
		LEFT JOIN sales s ON m.reference_type = 'sale' AND s.id = m.reference_id
		LEFT JOIN LATERAL (SELECT * FROM pharmacy_dispenses d WHERE m.reference_type = 'sale' AND d.sale_id = m.reference_id
			ORDER BY d.id LIMIT 1) d ON true
		LEFT JOIN pharmacy_patients dp ON dp.id = d.patient_id
		LEFT JOIN customers dc ON dc.id = dp.customer_id
		LEFT JOIN pharmacy_prescriptions rx ON rx.id = d.prescription_id
		WHERE m.product_id = $1 AND m.quantity <> 0 AND ($2::timestamptz IS NULL OR m.created_at >= $2)
		ORDER BY m.created_at, m.id`, productID, from); err != nil {
		return r, err
	}
	since := decimal.Zero
	for _, row := range r.Rows {
		since = since.Add(row.Qty)
	}
	r.Opening = r.Stock.Sub(since)
	bal := r.Opening
	for i := range r.Rows {
		bal = bal.Add(r.Rows[i].Qty)
		r.Rows[i].Balance = bal
	}
	r.Old = []OldEntry{}
	err := s.db.SelectContext(ctx, &r.Old, `SELECT entry_at, direction, quantity, patient_name, prescriber_name, reference, notes
		FROM pharmacy_cd_ledger WHERE product_id = $1 ORDER BY entry_at, id`, productID)
	return r, err
}
