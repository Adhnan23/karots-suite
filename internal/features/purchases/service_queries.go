package purchases

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
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

// ListBetween returns the purchases received in a date window (drafts excluded,
// no row cap) — what the Purchases report is actually about.
func (s *Service) ListBetween(ctx context.Context, from, to time.Time) ([]Purchase, error) {
	rows, err := s.repo.ListBetween(ctx, from, to)
	if err != nil {
		return nil, apperr.Internal("failed to list purchases", err)
	}
	return rows, nil
}

// Filter narrows the admin purchase list. Q matches the invoice no or the
// supplier's name.
type Filter struct {
	Draft      bool
	SupplierID int64
	Q          string
	Unpaid     bool // received with something still owed
	Page       int
	Limit      int
}

// Find is the paged purchase list, newest first, with the matching count.
func (s *Service) Find(ctx context.Context, f Filter) ([]Purchase, int, error) {
	cond, args := "pu.status <> 'draft'", []any{}
	if f.Draft {
		cond = "pu.status = 'draft'"
	}
	add := func(sql string, v any) {
		args = append(args, v)
		cond += " AND " + strings.ReplaceAll(sql, "$?", "$"+strconv.Itoa(len(args)))
	}
	if f.SupplierID > 0 {
		add("pu.supplier_id = $?", f.SupplierID)
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		add("(pu.invoice_no ILIKE '%' || $? || '%' OR s.name ILIKE '%' || $? || '%')", q)
	}
	if f.Unpaid {
		cond += " AND pu.total - pu.paid_amount - pu.credited_amount > 0"
	}
	from := ` FROM purchases pu JOIN suppliers s ON s.id = pu.supplier_id LEFT JOIN users u ON u.id = pu.received_by WHERE ` + cond
	var total int
	if err := s.db.GetContext(ctx, &total, `SELECT COUNT(*)`+from, args...); err != nil {
		return nil, 0, apperr.Internal("failed to count purchases", err)
	}
	limit := max(1, min(f.Limit, 200))
	var rows []Purchase
	err := s.db.SelectContext(ctx, &rows, `SELECT pu.*, s.name AS supplier_name, u.name AS received_by_name`+from+
		` ORDER BY pu.created_at DESC, pu.id DESC LIMIT `+strconv.Itoa(limit)+` OFFSET `+strconv.Itoa((max(f.Page, 1)-1)*limit), args...)
	if err != nil {
		return nil, 0, apperr.Internal("failed to list purchases", err)
	}
	return rows, total, nil
}
