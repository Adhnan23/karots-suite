package pharmacy

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"karots-suite/internal/apperr"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// Script is a prescription with its drug lines.
type Script struct {
	ID                 int64      `db:"id"`
	PatientID          int64      `db:"patient_id"`
	PrescriberName     string     `db:"prescriber_name"`
	PrescriberReg      string     `db:"prescriber_reg"`
	PrescribedOn       *time.Time `db:"prescribed_on"`
	Notes              string     `db:"notes"`
	IsRepeat           bool       `db:"is_repeat"`
	RepeatIntervalDays int        `db:"repeat_interval_days"`
	Active             bool       `db:"active"`
	CreatedAt          time.Time  `db:"created_at"`
	Items              []ScriptItem
}

// ScriptItem is one drug line with what was handed over against it so far.
type ScriptItem struct {
	ID          int64           `db:"id"`
	RxID        int64           `db:"rx_id"`
	PatientID   int64           `db:"patient_id"`
	ProductID   *int64          `db:"product_id"`
	GroupID     *int64          `db:"group_id"` // stored, else the product's alternatives group
	Drug        string          `db:"drug"`     // what the doctor wrote, else the product / group name
	ProductName string          `db:"product_name"`
	GroupName   string          `db:"group_name"`
	Dosage      string          `db:"dosage"`
	Quantity    decimal.Decimal `db:"quantity"`
	PerDay      decimal.Decimal `db:"per_day"`
	IsRepeat    bool            `db:"is_repeat"`
	Active      bool            `db:"active"`
	Dispensed   decimal.Decimal `db:"dispensed"`
	LastAt      *time.Time      `db:"last_at"`
	DaysSupply  int             `db:"days_supply"`
	CalledAt    *time.Time      `db:"called_at"`
}

// NextDue is when the last hand-over runs out (nil = never handed over / untracked).
func (it ScriptItem) NextDue() *time.Time {
	if d, ok := nextRefillDue(it.LastAt, it.DaysSupply); ok {
		return &d
	}
	return nil
}

// Owed is what the line still needs (see owed).
func (it ScriptItem) Owed() decimal.Decimal { return owed(it.Quantity, it.Dispensed, it.IsRepeat) }

// itemSelect reads drug lines with their group (stored or the product's), what was
// handed over against each and the latest hand-over.
const itemSelect = `
	SELECT i.id, i.prescription_id AS rx_id, rx.patient_id, i.product_id, gg.gid AS group_id,
	       COALESCE(NULLIF(i.drug_text, ''), pr.name, g.name, '') AS drug,
	       COALESCE(pr.name, '') AS product_name, COALESCE(g.name, '') AS group_name,
	       i.dosage, i.quantity, i.per_day, rx.is_repeat, rx.active,
	       ds.dispensed, ld.last_at, COALESCE(ld.days_supply, 0) AS days_supply, i.called_at
	FROM pharmacy_prescription_items i
	JOIN pharmacy_prescriptions rx ON rx.id = i.prescription_id
	LEFT JOIN products pr ON pr.id = i.product_id
	LEFT JOIN LATERAL (SELECT COALESCE(i.group_id, (SELECT t.group_id FROM alt_members m JOIN alt_tiers t ON t.id = m.tier_id
		WHERE m.product_id = i.product_id)) AS gid) gg ON true
	LEFT JOIN alt_groups g ON g.id = gg.gid
	LEFT JOIN LATERAL (SELECT COALESCE(SUM(di.quantity), 0) AS dispensed FROM pharmacy_dispense_items di
		WHERE di.prescription_item_id = i.id) ds ON true
	LEFT JOIN LATERAL (SELECT d.dispensed_at AS last_at, di.days_supply FROM pharmacy_dispense_items di
		JOIN pharmacy_dispenses d ON d.id = di.dispense_id
		WHERE di.prescription_item_id = i.id ORDER BY d.dispensed_at DESC, di.id DESC LIMIT 1) ld ON true`

const scriptSelect = `
	SELECT id, patient_id, prescriber_name, prescriber_reg, prescribed_on, notes, is_repeat,
	       repeat_interval_days, active, created_at
	FROM pharmacy_prescriptions`

// Scripts lists a patient's prescriptions (active first, newest first) with their lines.
func (s *Store) Scripts(ctx context.Context, patientID int64) ([]Script, error) {
	out := []Script{}
	if err := s.db.SelectContext(ctx, &out, scriptSelect+` WHERE patient_id = $1
		ORDER BY active DESC, COALESCE(prescribed_on, created_at::date) DESC, id DESC`, patientID); err != nil {
		return nil, err
	}
	var items []ScriptItem
	if err := s.db.SelectContext(ctx, &items, itemSelect+` WHERE rx.patient_id = $1 ORDER BY i.id`, patientID); err != nil {
		return nil, err
	}
	at := make(map[int64]int, len(out))
	for i := range out {
		at[out[i].ID] = i
	}
	for _, it := range items {
		out[at[it.RxID]].Items = append(out[at[it.RxID]].Items, it)
	}
	return out, nil
}

func (s *Store) Script(ctx context.Context, id int64) (Script, error) {
	var sc Script
	err := s.db.GetContext(ctx, &sc, scriptSelect+` WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return sc, apperr.NotFound("prescription")
	}
	if err != nil {
		return sc, err
	}
	err = s.db.SelectContext(ctx, &sc.Items, itemSelect+` WHERE i.prescription_id = $1 ORDER BY i.id`, id)
	return sc, err
}

// ScriptInput writes a prescription. An item with an ID edits that line (its
// hand-over history stays linked); lines left out are removed.
type ScriptInput struct {
	PrescriberName     string
	PrescriberReg      string
	PrescribedOn       *time.Time
	Notes              string
	IsRepeat           bool
	RepeatIntervalDays int
	Items              []ItemInput
}

type ItemInput struct {
	ID        int64
	ProductID *int64
	GroupID   *int64
	DrugText  string
	Dosage    string
	Quantity  decimal.Decimal
	PerDay    decimal.Decimal
}

func (in *ScriptInput) clean() error {
	in.PrescriberName, in.PrescriberReg = strings.TrimSpace(in.PrescriberName), strings.TrimSpace(in.PrescriberReg)
	kept := in.Items[:0]
	for _, it := range in.Items {
		it.DrugText, it.Dosage = strings.TrimSpace(it.DrugText), strings.TrimSpace(it.Dosage)
		if it.ProductID == nil && it.GroupID == nil && it.DrugText == "" {
			continue
		}
		if it.Quantity.IsNegative() || it.PerDay.IsNegative() {
			return apperr.Validation("quantity and doses a day can't be negative")
		}
		if it.PerDay.IsZero() {
			it.PerDay = DosesPerDay(it.Dosage)
		}
		kept = append(kept, it)
	}
	in.Items = kept
	if len(in.Items) == 0 {
		return apperr.Validation("add at least one medicine")
	}
	if in.RepeatIntervalDays < 0 {
		in.RepeatIntervalDays = 0
	}
	return nil
}

// CreateScript writes a new prescription for a patient.
func (s *Store) CreateScript(ctx context.Context, patientID, by int64, in ScriptInput) (int64, error) {
	if err := in.clean(); err != nil {
		return 0, err
	}
	if _, err := s.Patient(ctx, patientID); err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck
	var id int64
	if err := tx.GetContext(ctx, &id, `
		INSERT INTO pharmacy_prescriptions (patient_id, prescriber_name, prescriber_reg, prescribed_on, notes, is_repeat, repeat_interval_days, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, 0)) RETURNING id`,
		patientID, in.PrescriberName, in.PrescriberReg, in.PrescribedOn, strings.TrimSpace(in.Notes), in.IsRepeat, in.RepeatIntervalDays, by); err != nil {
		return 0, err
	}
	if err := saveItems(ctx, tx, id, in.Items); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// UpdateScript edits a prescription in place. Lines keep their ids, so each
// drug's hand-over history (and its refill reminder) survives the edit.
func (s *Store) UpdateScript(ctx context.Context, id int64, in ScriptInput) error {
	if err := in.clean(); err != nil {
		return err
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	res, err := tx.ExecContext(ctx, `UPDATE pharmacy_prescriptions SET prescriber_name = $2, prescriber_reg = $3,
		prescribed_on = $4, notes = $5, is_repeat = $6, repeat_interval_days = $7 WHERE id = $1`,
		id, in.PrescriberName, in.PrescriberReg, in.PrescribedOn, strings.TrimSpace(in.Notes), in.IsRepeat, in.RepeatIntervalDays)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("prescription")
	}
	keep := []int64{}
	for _, it := range in.Items {
		if it.ID > 0 {
			keep = append(keep, it.ID)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM pharmacy_prescription_items WHERE prescription_id = $1 AND NOT (id = ANY($2))`,
		id, pq.Array(keep)); err != nil {
		return err
	}
	if err := saveItems(ctx, tx, id, in.Items); err != nil {
		return err
	}
	return tx.Commit()
}

type execer interface {
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
}

// saveItems updates the lines with an id (of this script) and inserts the rest.
// A product line without a group takes the product's alternatives group.
func saveItems(ctx context.Context, tx execer, rxID int64, items []ItemInput) error {
	for _, it := range items {
		group := `COALESCE($3::bigint, (SELECT t.group_id FROM alt_members m JOIN alt_tiers t ON t.id = m.tier_id WHERE m.product_id = $2))`
		var res sql.Result
		var err error
		if it.ID > 0 {
			res, err = tx.ExecContext(ctx, `UPDATE pharmacy_prescription_items SET product_id = $2, group_id = `+group+`,
				drug_text = $4, dosage = $5, quantity = $6, per_day = $7 WHERE id = $8 AND prescription_id = $1`,
				rxID, it.ProductID, it.GroupID, it.DrugText, it.Dosage, it.Quantity, it.PerDay, it.ID)
		} else {
			res, err = tx.ExecContext(ctx, `INSERT INTO pharmacy_prescription_items (prescription_id, product_id, group_id, drug_text, dosage, quantity, per_day)
				VALUES ($1, $2, `+group+`, $4, $5, $6, $7)`, rxID, it.ProductID, it.GroupID, it.DrugText, it.Dosage, it.Quantity, it.PerDay)
		}
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return apperr.Validation("a medicine line is not on this prescription — reload it")
		}
	}
	return nil
}

// SetScriptActive stops (or resumes) a script: stopped ones never remind.
func (s *Store) SetScriptActive(ctx context.Context, id int64, on bool) error {
	res, err := s.db.ExecContext(ctx, `UPDATE pharmacy_prescriptions SET active = $2 WHERE id = $1`, id, on)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("prescription")
	}
	return nil
}

// Suggest is what Add puts in the cart for a line: its own product while in
// stock, else its group's best in-stock member (best tier, most stock); with
// nothing in stock, the line's product (or the group's first) anyway — the sale
// is never stopped.
type Suggest struct {
	ProductID int64           `db:"id"`
	Name      string          `db:"name"`
	Stock     decimal.Decimal `db:"stock"`
}

func (s *Store) Suggest(ctx context.Context, it ScriptItem) (*Suggest, error) {
	var out Suggest
	err := s.db.GetContext(ctx, &out, `
		SELECT p.id, p.name, COALESCE(st.quantity, 0) AS stock
		FROM products p LEFT JOIN stock st ON st.product_id = p.id
		LEFT JOIN alt_members m ON m.product_id = p.id
		LEFT JOIN alt_tiers t ON t.id = m.tier_id AND t.is_active
		LEFT JOIN alt_groups g ON g.id = t.group_id AND g.is_active
		WHERE p.is_active AND NOT p.is_service
		  AND (p.id = $1 OR (g.id = $2 AND t.id IS NOT NULL))
		ORDER BY COALESCE(st.quantity, 0) > 0 DESC, p.id = $1 DESC, t.sort_order, t.id, st.quantity DESC, p.name
		LIMIT 1`, it.ProductID, it.GroupID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &out, err
}
