package products

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/stock"

	"github.com/jmoiron/sqlx"
)

type Service struct {
	db   *sqlx.DB
	repo *Repository
}

func NewService(db *sqlx.DB) *Service {
	return &Service{db: db, repo: NewRepository(db)}
}

func (s *Service) List(ctx context.Context, q ListQuery) ([]Product, int, error) {
	q.Normalize()
	rows, total, err := s.listOnce(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	// Typo rescue: only when an exact-word search found nothing does it fall
	// back to fuzzy matching. Running fuzzy first would pad searches that
	// already work with near-misses the user did not ask for.
	if total == 0 && !q.Fuzzy && len(searchTokens(q.Search)) > 0 {
		q.Fuzzy = true
		return s.listOnce(ctx, q)
	}
	return rows, total, nil
}

func (s *Service) listOnce(ctx context.Context, q ListQuery) ([]Product, int, error) {
	rows, err := s.repo.List(ctx, q)
	if err != nil {
		return nil, 0, apperr.Internal("failed to list products", err)
	}
	total, err := s.repo.Count(ctx, q)
	if err != nil {
		return nil, 0, apperr.Internal("failed to count products", err)
	}
	return rows, total, nil
}

// ListAll returns the whole catalogue unpaginated (disabled products too on
// request), for spreadsheet export (List clamps Limit to 100 and would export
// only the first page).
func (s *Service) ListAll(ctx context.Context, includeDisabled bool) ([]Product, error) {
	rows, err := s.repo.ListAll(ctx, includeDisabled)
	if err != nil {
		return nil, apperr.Internal("failed to list products", err)
	}
	return rows, nil
}

// ByIDs returns the given products (full rows incl. stock_qty), preserving the id
// order and silently dropping ids that don't resolve. Used to pull specific
// products into a list — e.g. the reorder worklist adding a plugin's related products.
func (s *Service) ByIDs(ctx context.Context, ids []int64) ([]Product, error) {
	rows, err := s.repo.byIDsOrdered(ctx, ids)
	if err != nil {
		return nil, apperr.Internal("failed to fetch products by id", err)
	}
	return rows, nil
}

// SyncCatalog serves the stock_capture LAN sync snapshot.
func (s *Service) SyncCatalog(ctx context.Context) ([]SyncRow, error) {
	rows, err := s.repo.SyncCatalog(ctx)
	if err != nil {
		return nil, apperr.Internal("failed to build sync catalog", err)
	}
	return rows, nil
}

// CountSheet serves the printable stock count sheet (Reports → Stock Count
// Sheet): active, stocked products filtered by category subtree and/or
// preferred supplier, grouped by category path.
func (s *Service) CountSheet(ctx context.Context, categoryID, supplierID *int64) ([]CountSheetRow, error) {
	rows, err := s.repo.CountSheet(ctx, categoryID, supplierID)
	if err != nil {
		return nil, apperr.Internal("failed to build count sheet", err)
	}
	return rows, nil
}

func (s *Service) Get(ctx context.Context, id int64) (*Product, error) {
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("product")
		}
		return nil, apperr.Internal("failed to load product", err)
	}
	return p, nil
}

// FindByName returns the active product with this exact (case-insensitive) name,
// or nil when none exists. Used by Stock Intake to warn before creating a
// duplicate. A DB error other than "no rows" is surfaced.
func (s *Service) FindByName(ctx context.Context, name string) (*Product, error) {
	p, err := s.repo.FindByName(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, apperr.Internal("failed to look up product", err)
	}
	return p, nil
}

// PriceOptions backs the till's lot prompt. It returns, for every product whose
// live lots disagree on price OR include an expired lot, that product's options —
// nothing else. The till loads this once at startup and refreshes it after each
// sale, so it can decide whether to prompt however the item was added (scan, menu
// card, search result) without a round trip per tap.
//
// It stays empty for a shop that never prices per lot and never lets stock
// expire, which is what keeps the prompt invisible for them.
func (s *Service) PriceOptions(ctx context.Context) (map[int64][]stock.PriceOption, error) {
	return stock.NewService(s.db).LotPromptProducts(ctx)
}

// QuickPicks backs the till's top-menu "Frequently sold" grid: the best-sellers
// of the last 30 days, in stock. The till loads this at open and after each sale.
func (s *Service) QuickPicks(ctx context.Context) ([]Product, error) {
	rows, err := s.repo.FrequentProducts(ctx, time.Now().AddDate(0, 0, -30), 16)
	if err != nil {
		return nil, apperr.Internal("failed to load quick picks", err)
	}
	return rows, nil
}

// LotsFor lists one product's live lots for a "which lot is this?" picker on the
// screens that remove stock (write-off, own-use, staff, stock correction).
//
// It deliberately carries no cost price, unlike the admin lots endpoint: a
// cashier writing off a broken bottle needs to identify the lot, not to see what
// the shop paid for it.
func (s *Service) LotsFor(ctx context.Context, productID int64) ([]stock.PriceOption, error) {
	rows, err := stock.NewRepository(s.db).PriceOptions(ctx, productID)
	if err != nil {
		return nil, apperr.Internal("failed to load batches", err)
	}
	return rows, nil
}

// BarcodeScanned records that a code was just scanned at the counter (its "last
// scanned" date). Best effort: a failure here must never stop a sale.
func (s *Service) BarcodeScanned(ctx context.Context, code string) {
	_ = s.repo.TouchBarcode(ctx, strings.TrimSpace(code))
}

// GetByBarcode powers the cashier scanner and price-check lookups.
func (s *Service) GetByBarcode(ctx context.Context, barcode string) (*Product, error) {
	p, err := s.repo.FindByBarcode(ctx, strings.TrimSpace(barcode))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("product")
		}
		return nil, apperr.Internal("failed to load product", err)
	}
	return p, nil
}

// AllServices returns every service product, active first (admin Services list).
func (s *Service) AllServices(ctx context.Context) ([]Product, error) {
	rows, err := s.repo.AllServices(ctx)
	if err != nil {
		return nil, apperr.Internal("failed to load services", err)
	}
	return rows, nil
}

// ListServices returns active service products for attribution pickers.
func (s *Service) ListServices(ctx context.Context) ([]Product, error) {
	rows, err := s.repo.ListServices(ctx)
	if err != nil {
		return nil, apperr.Internal("failed to load services", err)
	}
	return rows, nil
}
