package cashregister

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"karots-suite/internal/apperr"
)

// SessionTotals totals every session the filter matches (the Cash Register
// report's figures).
func (s *Service) SessionTotals(ctx context.Context, f SessionFilter) (*SessionTotals, error) {
	t, err := s.repo.SessionTotals(ctx, f)
	if err != nil {
		return nil, apperr.Internal("failed to total till sessions", err)
	}
	return t, nil
}

// ListSessions pages the admin's till sessions.
func (s *Service) ListSessions(ctx context.Context, f SessionFilter) ([]SessionRow, error) {
	rows, err := s.repo.ListSessions(ctx, f)
	if err != nil {
		return nil, apperr.Internal("failed to list register sessions", err)
	}
	return rows, nil
}

func (s *Service) Cashiers(ctx context.Context) ([]SessionRow, error) {
	rows, err := s.repo.Cashiers(ctx)
	if err != nil {
		return nil, apperr.Internal("failed to list cashiers", err)
	}
	return rows, nil
}

func (s *Service) RecentSessions(ctx context.Context, limit int) ([]SessionRow, error) {
	rows, err := s.repo.RecentSessions(ctx, limit)
	if err != nil {
		return nil, apperr.Internal("failed to list register sessions", err)
	}
	return rows, nil
}

// OpenSessions lists currently-open tills (for moving cash to/from a drawer).
func (s *Service) OpenSessions(ctx context.Context) ([]SessionRow, error) {
	rows, err := s.repo.OpenSessions(ctx)
	if err != nil {
		return nil, apperr.Internal("failed to list open tills", err)
	}
	return rows, nil
}

func (s *Service) SessionsInRange(ctx context.Context, from, to time.Time) ([]SessionRow, error) {
	rows, err := s.repo.SessionsInRange(ctx, from, to)
	if err != nil {
		return nil, apperr.Internal("failed to list register sessions", err)
	}
	return rows, nil
}

func (s *Service) SessionDetail(ctx context.Context, id int64) (*SessionRow, []Movement, error) {
	sess, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, apperr.NotFound("register session")
		}
		return nil, nil, apperr.Internal("failed to load session", err)
	}
	moves, err := s.repo.ListMovements(ctx, id)
	if err != nil {
		return nil, nil, apperr.Internal("failed to load cash movements", err)
	}
	return sess, moves, nil
}
