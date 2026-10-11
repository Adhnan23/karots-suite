package products

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

func (s *Service) Create(ctx context.Context, in CreateInput) (*Product, error) {
	w, err := toWriteRow(in)
	if err != nil {
		return nil, err
	}
	id, err := s.repo.Insert(ctx, w)
	if err != nil {
		return nil, mapWriteErr(err)
	}
	return s.Get(ctx, id)
}

// QuickInput is a till-side quick-add: the cashier hit an item that isn't in the
// catalog and must still sell it. Only name + price are required; the barcode is
// optional (scanned, generated, or left for the admin).
type QuickInput struct {
	Name    string `json:"name"    form:"name"`
	Price   string `json:"price"   form:"price"`
	Qty     string `json:"qty"     form:"qty"`
	Barcode string `json:"barcode" form:"barcode"`
	UnitID  int64  `json:"unit_id" form:"unit_id"`
}

// QuickCreate makes a minimal, sellable product on the fly and seeds its stock to
// the quantity being sold, so the imminent sale nets it back to zero ("count
// later"). It is flagged needs_review and stamped with the cashier (created_by) so
// the admin can finish it (real category, unit, cost) from the review queue. The
// whole thing — product row, opening batch, stock bump and audit movement — runs
// in one transaction. cost_price is 0 (a placeholder corrected during review).
func (s *Service) QuickCreate(ctx context.Context, in QuickInput, userID int64) (*Product, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, apperr.Validation("item name is required")
	}
	price, err := money.Parse(in.Price)
	if err != nil || price.IsNegative() {
		return nil, apperr.Validation("price must be a non-negative amount")
	}
	qty, err := money.Parse(in.Qty)
	if err != nil || qty.LessThanOrEqual(decimal.Zero) {
		qty = decimal.NewFromInt(1)
	}

	var newID int64
	err = appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		catID, err := ensureUncategorized(ctx, tx)
		if err != nil {
			return apperr.Internal("failed to resolve category", err)
		}
		unitID := in.UnitID
		if unitID <= 0 {
			unitID, err = defaultUnitID(ctx, tx)
			if err != nil {
				return apperr.Internal("failed to resolve unit", err)
			}
		}
		repo := NewRepository(tx)
		id, err := repo.Insert(ctx, writeRow{
			Name:        name,
			Barcode:     nullStr(in.Barcode),
			CategoryID:  catID,
			UnitID:      unitID,
			Cost:        decimal.Zero,
			Selling:     price,
			Wholesale:   decimal.Zero,
			Tax:         decimal.Zero,
			NeedsReview: true,
			CreatedBy:   &userID,
		})
		if err != nil {
			return mapWriteErr(err)
		}
		// Seed stock = qty so the upcoming sale nets it to 0. A product-insert
		// trigger already created the stock row at 0; bump it and open a costing
		// batch + audit movement, mirroring a manual stock adjustment.
		stk := stock.NewRepository(tx)
		if err := stk.Increment(ctx, id, qty); err != nil {
			return apperr.Internal("failed to seed stock", err)
		}
		if _, err := stk.InsertBatch(ctx, stock.NewBatch{
			ProductID: id, Quantity: qty, CostPrice: decimal.Zero, Source: "opening",
		}); err != nil {
			return apperr.Internal("failed to open stock batch", err)
		}
		note := "quick-add opening (count pending)"
		if err := stk.InsertMovement(ctx, stock.MovementInput{
			ProductID: id, Type: stock.MoveAdjust, Quantity: qty, UserID: userID, Note: &note,
		}); err != nil {
			return apperr.Internal("failed to record stock movement", err)
		}
		newID = id
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, newID)
}

// NeedsReview lists the products awaiting admin review (quick-added at the till).
func (s *Service) NeedsReview(ctx context.Context) ([]Product, error) {
	rows, err := s.repo.ListNeedsReview(ctx)
	if err != nil {
		return nil, apperr.Internal("failed to list items needing review", err)
	}
	return rows, nil
}

// CountNeedsReview powers the admin-panel badge.
func (s *Service) CountNeedsReview(ctx context.Context) (int, error) {
	n, err := s.repo.CountNeedsReview(ctx)
	if err != nil {
		return 0, apperr.Internal("failed to count items needing review", err)
	}
	return n, nil
}

// CountByPreferredSupplier is how many products default to this supplier.
func (s *Service) CountByPreferredSupplier(ctx context.Context, supplierID int64) (int, error) {
	n, err := s.repo.CountByPreferredSupplier(ctx, supplierID)
	if err != nil {
		return 0, apperr.Internal("failed to count supplier's products", err)
	}
	return n, nil
}

// ReassignPreferredSupplier moves every product defaulting to one supplier over
// to another — for when a company changes the distributor behind the same goods.
func (s *Service) ReassignPreferredSupplier(ctx context.Context, fromID, toID int64) (int64, error) {
	if fromID <= 0 || toID <= 0 {
		return 0, apperr.Validation("pick both suppliers")
	}
	if fromID == toID {
		return 0, apperr.Validation("pick a different supplier to move to")
	}
	n, err := s.repo.ReassignPreferredSupplier(ctx, fromID, toID)
	if err != nil {
		return 0, apperr.Internal("failed to reassign products", err)
	}
	return n, nil
}

// SetCost updates a product's cost price (stock-take opening-stock valuation).
func (s *Service) SetCost(ctx context.Context, id int64, cost decimal.Decimal) error {
	if cost.IsNegative() {
		return apperr.Validation("cost must be a non-negative amount")
	}
	if err := s.repo.SetCost(ctx, id, cost); err != nil {
		return apperr.Internal("failed to update cost", err)
	}
	return nil
}

// SetPrices updates a product's selling and wholesale prices (bulk set from the
// stock-take screen / count-sheet import). Both must be non-negative.
func (s *Service) SetPrices(ctx context.Context, id int64, selling, wholesale decimal.Decimal) error {
	if selling.IsNegative() || wholesale.IsNegative() {
		return apperr.Validation("prices must be non-negative amounts")
	}
	if err := s.repo.SetPrices(ctx, id, selling, wholesale); err != nil {
		return apperr.Internal("failed to update prices", err)
	}
	return nil
}

// MarkReviewed clears the review flag once the admin has finished an item.
func (s *Service) MarkReviewed(ctx context.Context, id int64) error {
	if err := s.repo.ClearReview(ctx, id); err != nil {
		return apperr.Internal("failed to mark reviewed", err)
	}
	return nil
}

// BackfillCost corrects the placeholder cost 0 on a product's past sale lines once
// a real cost is known, so historical COGS/profit are accurate. Returns how many
// lines were corrected.
func (s *Service) BackfillCost(ctx context.Context, productID int64, costStr string) (int64, error) {
	cost, err := money.Parse(costStr)
	if err != nil || cost.IsNegative() {
		return 0, apperr.Validation("cost must be a non-negative amount")
	}
	n, err := s.repo.BackfillZeroCost(ctx, productID, cost)
	if err != nil {
		return 0, apperr.Internal("failed to backfill cost", err)
	}
	return n, nil
}

func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) (*Product, error) {
	w, err := toWriteRow(in)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, id, w); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("product")
		}
		return nil, mapWriteErr(err)
	}
	// "Apply this price to the stock I already have." Without it a price change
	// only reaches stock that arrives later, and the lots on the shelf keep
	// selling at what they were stickered at.
	if in.RepriceStock {
		if err := stock.NewRepository(s.db).ClearBatchPrices(ctx, id); err != nil {
			return nil, apperr.Internal("failed to reprice stock on hand", err)
		}
	}
	return s.Get(ctx, id)
}

// Rename changes only a product's name, leaving every other field untouched.
//
// A narrow operation on purpose: callers that hold a product id but not a full
// UpdateInput (a plugin renaming a hidden service product it owns)
// would otherwise have to reconstruct every field from a Product and convert the
// decimals back to strings, which loses precision and silently drops anything
// added to the input struct later.
func (s *Service) Rename(ctx context.Context, id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return apperr.Validation("product name is required")
	}
	if err := s.repo.Rename(ctx, id, name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound("product")
		}
		return mapWriteErr(err)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return apperr.NotFound("product")
	}
	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return apperr.Internal("failed to delete product", err)
	}
	return nil
}

// Reactivate re-enables a disabled product.
func (s *Service) Reactivate(ctx context.Context, id int64) error {
	if err := s.repo.Reactivate(ctx, id); err != nil {
		return apperr.Internal("failed to reactivate product", err)
	}
	return nil
}

func toWriteRow(in CreateInput) (writeRow, error) {
	cost, err := money.Parse(in.CostPrice)
	if err != nil {
		return writeRow{}, apperr.Validation("cost price is not a valid amount")
	}
	sell, err := money.Parse(in.SellingPrice)
	if err != nil {
		return writeRow{}, apperr.Validation("selling price is not a valid amount")
	}
	whole, err := money.Parse(in.WholesalePrice)
	if err != nil {
		return writeRow{}, apperr.Validation("wholesale price is not a valid amount")
	}
	tax, err := money.Parse(in.TaxRate)
	if err != nil {
		return writeRow{}, apperr.Validation("tax rate is not a valid number")
	}
	return writeRow{
		Name:              strings.TrimSpace(in.Name),
		NameLocal:         nullStr(deref(in.NameLocal)),
		Barcode:           nullStr(deref(in.Barcode)),
		CategoryID:        in.CategoryID,
		UnitID:            in.UnitID,
		Cost:              cost,
		Selling:           sell,
		Wholesale:         whole,
		Tax:               tax,
		Reorder:           in.ReorderLevel,
		TrackSerial:       in.TrackSerial,
		WarrantyMonths:    in.WarrantyMonths,
		IsService:         in.IsService,
		PreferredSupplier: nilIfZero(in.PreferredSupplierID),
	}, nil
}

// nilIfZero maps a 0 (or nil) supplier id to nil. The product form's "— none —"
// option posts an empty value, which the form binder turns into a *int64 of 0;
// stored as-is that 0 violates the preferred_supplier_id foreign key. Treating 0
// as "no supplier" keeps the field genuinely optional (create products first,
// attach a supplier later).
func nilIfZero(p *int64) *int64 {
	if p == nil || *p == 0 {
		return nil
	}
	return p
}

// seedOpeningStock mirrors the till quick-add opening seed (service.go QuickCreate)
// but with a real cost: bump the cached quantity, open a costing batch, and log a
// stock-adjust movement — all within the caller's transaction.
func seedOpeningStock(ctx context.Context, tx *sqlx.Tx, productID int64, qty, cost decimal.Decimal, userID int64) error {
	stk := stock.NewRepository(tx)
	if err := stk.Increment(ctx, productID, qty); err != nil {
		return apperr.Internal("failed to seed opening stock", err)
	}
	if _, err := stk.InsertBatch(ctx, stock.NewBatch{
		ProductID: productID, Quantity: qty, CostPrice: cost, Source: "opening",
	}); err != nil {
		return apperr.Internal("failed to open stock batch", err)
	}
	note := "CSV import opening stock"
	if err := stk.InsertMovement(ctx, stock.MovementInput{
		ProductID: productID, Type: stock.MoveAdjust, Quantity: qty, UserID: userID, Note: &note,
	}); err != nil {
		return apperr.Internal("failed to record stock movement", err)
	}
	return nil
}

func mapWriteErr(err error) error {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "products_barcode_key") || strings.Contains(msg, "barcode"):
		return apperr.Conflict("a product with that barcode already exists")
	case strings.Contains(msg, "foreign key") && strings.Contains(msg, "category"):
		return apperr.Validation("selected category does not exist")
	case strings.Contains(msg, "foreign key") && strings.Contains(msg, "unit"):
		return apperr.Validation("selected unit does not exist")
	default:
		return apperr.Internal("failed to save product", err)
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ensureUncategorized returns the id of the "Uncategorized" category, creating it
// once if it doesn't exist. Quick-added items land here until an admin recategorizes
// them during review.
func ensureUncategorized(ctx context.Context, tx *sqlx.Tx) (int64, error) {
	var id int64
	err := tx.GetContext(ctx, &id, `SELECT id FROM categories WHERE name = 'Uncategorized' LIMIT 1`)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	err = tx.GetContext(ctx, &id,
		`INSERT INTO categories (name, parent_id) VALUES ('Uncategorized', NULL) RETURNING id`)
	return id, err
}

// defaultUnitID picks a sensible default unit for quick-added items — the seeded
// "Piece" (pcs) when present, otherwise the lowest-id unit.
func defaultUnitID(ctx context.Context, tx *sqlx.Tx) (int64, error) {
	var id int64
	err := tx.GetContext(ctx, &id,
		`SELECT id FROM units ORDER BY (abbreviation = 'pcs') DESC, id ASC LIMIT 1`)
	return id, err
}
