package purchases

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"karots-suite/internal/apperr"
)

// DeleteDraft removes a draft purchase order (only while still a draft).
func (s *Service) DeleteDraft(ctx context.Context, id int64) error {
	ok, err := s.repo.DeleteDraft(ctx, id)
	if err != nil {
		return apperr.Internal("failed to delete draft", err)
	}
	if !ok {
		return apperr.Validation("only draft purchase orders can be deleted")
	}
	return nil
}

// ListDrafts returns open Purchase Orders (drafts), newest first.
func (s *Service) ListDrafts(ctx context.Context) ([]Purchase, error) {
	rows, err := s.repo.ListByStatus(ctx, true, 200)
	if err != nil {
		return nil, apperr.Internal("failed to list draft purchases", err)
	}
	return rows, nil
}

// ListReceived returns received purchase history, newest first.
func (s *Service) ListReceived(ctx context.Context) ([]Purchase, error) {
	rows, err := s.repo.ListByStatus(ctx, false, 200)
	if err != nil {
		return nil, apperr.Internal("failed to list purchases", err)
	}
	return rows, nil
}

// GetMany loads several purchase details (for the multi-supplier PO print).
func (s *Service) GetMany(ctx context.Context, ids []int64) ([]Detail, error) {
	out := make([]Detail, 0, len(ids))
	for _, id := range ids {
		d, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id int64) (*Detail, error) {
	return s.loadDetail(ctx, s.repo, id)
}

func (s *Service) loadDetail(ctx context.Context, repo *Repository, id int64) (*Detail, error) {
	return loadDetailTx(ctx, repo, id)
}

// loadDetailTx is loadDetail without a receiver, so the package-level *Tx
// functions can use it.
func loadDetailTx(ctx context.Context, repo *Repository, id int64) (*Detail, error) {
	p, err := repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("purchase")
		}
		return nil, apperr.Internal("failed to load purchase", err)
	}
	items, err := repo.Items(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to load purchase items", err)
	}
	return &Detail{Purchase: *p, Items: items}, nil
}

// List returns recent purchases of any status for the JSON API's activity
// listing. It is capped, so it is NOT a reporting source: filtering its result
// by date would silently miss anything past the cap. Reports use ListBetween.
func (s *Service) List(ctx context.Context) ([]Purchase, error) {
	rows, err := s.repo.List(ctx, 100)
	if err != nil {
		return nil, apperr.Internal("failed to list purchases", err)
	}
	return rows, nil
}

// ListBetween returns the purchases received in a date window (drafts excluded,
// no row cap) — what the Purchases report is actually about.
func (s *Service) ListBetween(ctx context.Context, from, to time.Time) ([]Purchase, error) {
	rows, err := s.repo.ListBetween(ctx, from, to)
	if err != nil {
		return nil, apperr.Internal("failed to list purchases", err)
	}
	return rows, nil
}
