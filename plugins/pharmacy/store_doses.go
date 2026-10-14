package pharmacy

import (
	"context"
	"errors"
	"strings"

	"karots-suite/internal/apperr"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// Dose is a row of the shop's dose list: how it is written, what it means on a
// dose label ("" = not named yet), doses a day (0 = when needed) and how often used.
type Dose struct {
	ID     int64           `db:"id" json:"id"`
	Dose   string          `db:"dose" json:"dose"`
	Words  string          `db:"words" json:"words"`
	PerDay decimal.Decimal `db:"per_day" json:"perDay"`
	Uses   int             `db:"uses" json:"uses"`
}

// seedDoses fills an empty dose list: the usual doses, named for the label, then
// every dose already used (with its most common doses a day). A full restore of
// an older backup empties the list; this puts the usual ones back.
func seedDoses(ctx context.Context, tx execer) error {
	res, err := tx.ExecContext(ctx, `
		INSERT INTO pharmacy_doses (dose, words, per_day)
		SELECT * FROM (VALUES
			('1-0-0', 'Morning', 1), ('0-1-0', 'Noon', 1), ('0-0-1', 'At night', 1),
			('1-0-1', 'Morning · Night', 2), ('1-1-0', 'Morning · Noon', 2), ('0-1-1', 'Noon · Night', 2),
			('1-1-1', 'Morning · Noon · Night', 3), ('1-1-1-1', 'Four times a day', 4),
			('½-0-½', 'Half morning · Half night', 1), ('2-0-2', '2 morning · 2 night', 4),
			('od', 'Once a day', 1), ('bd', 'Twice a day', 2), ('tds', 'Three times a day', 3), ('qds', 'Four times a day', 4),
			('mane', 'In the morning', 1), ('nocte', 'At night', 1), ('sos', 'When needed', 0), ('stat', 'At once', 0)
		) AS v(dose, words, per_day)
		WHERE NOT EXISTS (SELECT 1 FROM pharmacy_doses)
		ON CONFLICT DO NOTHING`)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO pharmacy_doses (dose, per_day)
		SELECT MIN(trim(d.dose)), COALESCE(MODE() WITHIN GROUP (ORDER BY d.per_day) FILTER (WHERE d.per_day > 0), 0)
		FROM (SELECT dosage AS dose, per_day FROM pharmacy_prescription_items WHERE trim(dosage) <> ''
		      UNION ALL SELECT dose, 0 FROM pharmacy_medicines WHERE trim(dose) <> '') d
		GROUP BY lower(trim(d.dose))
		ON CONFLICT DO NOTHING`)
	return err
}

// Doses is the dose list, most used first (prescription lines + usual doses).
func (s *Store) Doses(ctx context.Context) ([]Dose, error) {
	if err := seedDoses(ctx, s.db); err != nil {
		return nil, err
	}
	out := []Dose{}
	err := s.db.SelectContext(ctx, &out, `
		SELECT d.id, d.dose, d.words, d.per_day,
		       (SELECT COUNT(*) FROM pharmacy_prescription_items i WHERE lower(trim(i.dosage)) = lower(trim(d.dose)))
		     + (SELECT COUNT(*) FROM pharmacy_medicines m WHERE lower(trim(m.dose)) = lower(trim(d.dose))) AS uses
		FROM pharmacy_doses d ORDER BY uses DESC, d.id`)
	return out, err
}

type DoseInput struct {
	Dose, Words string
	PerDay      decimal.Decimal
}

func (in *DoseInput) clean() error {
	in.Dose, in.Words = strings.TrimSpace(in.Dose), strings.TrimSpace(in.Words)
	if in.Dose == "" || len(in.Dose) > 40 || len(in.Words) > 60 {
		return apperr.Validation("write the dose (up to 40 letters) and what it means (up to 60)")
	}
	if in.PerDay.IsNegative() || in.PerDay.GreaterThan(decimal.NewFromInt(48)) {
		return apperr.Validation("doses a day: 0 (when needed) to 48")
	}
	return nil
}

func dupDose(err error) error {
	var pe *pq.Error
	if errors.As(err, &pe) && pe.Code == "23505" {
		return apperr.Conflict("that dose is already on the list")
	}
	return err
}

func (s *Store) CreateDose(ctx context.Context, in DoseInput) (int64, error) {
	if err := in.clean(); err != nil {
		return 0, err
	}
	var id int64
	err := s.db.GetContext(ctx, &id, `INSERT INTO pharmacy_doses (dose, words, per_day) VALUES ($1, $2, $3) RETURNING id`, in.Dose, in.Words, in.PerDay)
	return id, dupDose(err)
}

// SaveDose edits a dose (id 0 = add it).
func (s *Store) SaveDose(ctx context.Context, id int64, in DoseInput) error {
	if id == 0 {
		_, err := s.CreateDose(ctx, in)
		return err
	}
	if err := in.clean(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE pharmacy_doses SET dose = $2, words = $3, per_day = $4 WHERE id = $1`, id, in.Dose, in.Words, in.PerDay)
	if err != nil {
		return dupDose(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("dose")
	}
	return nil
}

// DeleteDose takes a dose off the list (prescriptions keep what they say).
func (s *Store) DeleteDose(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM pharmacy_doses WHERE id = $1`, id)
	return err
}

// learnDose puts a dose that was used onto the list (unnamed), with its doses a
// day (given, else read from 1-0-1); a dose already there is left as it is.
func learnDose(ctx context.Context, tx execer, dose string, perDay decimal.Decimal) error {
	if dose = strings.TrimSpace(dose); dose == "" || len(dose) > 40 {
		return nil
	}
	if !perDay.IsPositive() {
		perDay = DosesPerDay(dose)
	}
	if err := seedDoses(ctx, tx); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO pharmacy_doses (dose, per_day) VALUES ($1, $2) ON CONFLICT DO NOTHING`, dose, perDay)
	return err
}

// commonNotes are always offered as instructions.
var commonNotes = []string{"After meals", "Before meals", "With meals", "Empty stomach, 30 min before food",
	"At bedtime", "Finish the full course", "Do not crush or chew", "Dissolve in water"}

// Notes are the instructions to offer: those used (prescription lines, usual
// ones), most used first, then the common ones not used yet.
func (s *Store) Notes(ctx context.Context) ([]string, error) {
	var used []string
	if err := s.db.SelectContext(ctx, &used, `
		SELECT MIN(n) FROM (SELECT trim(note) AS n FROM pharmacy_prescription_items WHERE trim(note) <> ''
		                    UNION ALL SELECT trim(note) FROM pharmacy_medicines WHERE trim(note) <> '') x
		GROUP BY lower(n) ORDER BY COUNT(*) DESC, MIN(n) LIMIT 40`); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, n := range used {
		seen[strings.ToLower(n)] = true
	}
	for _, n := range commonNotes {
		if !seen[strings.ToLower(n)] {
			used = append(used, n)
		}
	}
	return used, nil
}
