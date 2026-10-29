package lockers

import (
	"context"
	"database/sql"
	"errors"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"
)

// refuseCheque stops an edit or archive of the Cheques in hand place.
func (s *Service) refuseCheque(ctx context.Context, id int64) error {
	var kind string
	err := s.db.GetContext(ctx, &kind, `SELECT kind FROM lockers WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // the update itself reports "not found"
	}
	if err != nil {
		return apperr.Internal("failed to load locker", err)
	}
	if kind == KindCheque {
		return apperr.Validation("Cheques in hand is kept by the suite")
	}
	return nil
}

// ChequesInHandTx returns the id of the Cheques in hand place, making it first if
// it is missing (a restored old archive has none).
func ChequesInHandTx(ctx context.Context, q db.Queryer) (int64, error) {
	if _, err := q.ExecContext(ctx, `INSERT INTO lockers (name, kind, allow_negative, is_active, cashier_access)
		VALUES ('Cheques in hand', 'cheque', false, true, false) ON CONFLICT DO NOTHING`); err != nil {
		return 0, err
	}
	var id int64
	err := q.GetContext(ctx, &id, `SELECT id FROM lockers WHERE kind = 'cheque'`)
	return id, err
}
