package lockers

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

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
	var id int64
	err := q.GetContext(ctx, &id, `SELECT id FROM lockers WHERE kind = 'cheque'`)
	if !errors.Is(err, sql.ErrNoRows) {
		return id, err
	}
	if _, err := q.ExecContext(ctx, `INSERT INTO lockers (name, kind, allow_negative, is_active, cashier_access)
		VALUES ('Cheques in hand', 'cheque', false, true, false) ON CONFLICT DO NOTHING`); err != nil {
		return 0, err
	}
	err = q.GetContext(ctx, &id, `SELECT id FROM lockers WHERE kind = 'cheque'`)
	return id, err
}

// refuseWaitingCheques stops archiving (or re-kinding) a bank place that the
// shop's own cheques are still waiting on: they could never be cleared.
func (s *Service) refuseWaitingCheques(ctx context.Context, id int64) error {
	var n int
	if err := s.db.GetContext(ctx, &n, `SELECT count(*) FROM cheques WHERE direction = 'out' AND status = 'pending' AND locker_id = $1`, id); err != nil {
		return apperr.Internal("failed to check cheques", err)
	}
	if n == 1 {
		return apperr.Validation("a cheque you wrote on this account is still waiting — mark it cleared or cancelled first")
	}
	if n > 1 {
		return apperr.Validation(strconv.Itoa(n) + " cheques you wrote on this account are still waiting — mark them cleared or cancelled first")
	}
	return nil
}
