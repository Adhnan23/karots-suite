package products

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// ImportRow is one resolved row of a bulk catalog import. Category/unit/supplier
// are pre-resolved to IDs by the caller (the web import handler) so the products
// service doesn't reach across features. OpeningQty seeds stock at OpeningCost.
type ImportRow struct {
	Name              string
	NameLocal         string
	Barcode           string
	CategoryID        int64
	UnitID            int64
	UserID            int64
	PreferredSupplier *int64
	Cost              decimal.Decimal
	Selling           decimal.Decimal
	Wholesale         decimal.Decimal
	Tax               decimal.Decimal
	Reorder           int
	WarrantyMonths    int
	TrackSerial       bool
	OpeningQty        decimal.Decimal
	// MatchByName lets a row that carries a barcode still fall back to matching an
	// existing product by name when the barcode doesn't match — so a drifted/changed
	// barcode updates the right product (and is realigned) instead of duplicating.
	// Off for routine imports; on for deliberate catalogue merges.
	MatchByName bool
	// Active, when the sheet has an "active" column, enables or disables the
	// product; it also lets the row match a disabled product (an export with
	// disabled products round-trips instead of duplicating them). nil = leave as is.
	Active *bool
	// Given names the columns the sheet really fills for this row (canonical
	// names: "name_local", "category", "cost_price", …). When updating an
	// existing product only those change — a price-update sheet with just name
	// and price must not wipe the cost, category or supplier. nil = all given.
	Given map[string]bool
}

// keepMissing puts back an existing product's value for every column the row
// doesn't fill (see ImportRow.Given).
func keepMissing(w writeRow, p *Product, given map[string]bool) writeRow {
	if given == nil {
		return w
	}
	keep := func(col string) bool { return !given[col] }
	if keep("name_local") {
		w.NameLocal = p.NameLocal
	}
	if keep("barcode") {
		w.Barcode = p.Barcode
	}
	if keep("category") {
		w.CategoryID = p.CategoryID
	}
	if keep("unit") {
		w.UnitID = p.UnitID
	}
	if keep("cost_price") {
		w.Cost = p.CostPrice
	}
	if keep("selling_price") {
		w.Selling = p.SellingPrice
	}
	if keep("wholesale_price") {
		w.Wholesale = p.WholesalePrice
	}
	if keep("tax_rate") {
		w.Tax = p.TaxRate
	}
	if keep("reorder_level") {
		w.Reorder = p.ReorderLevel
	}
	if keep("track_serial") {
		w.TrackSerial = p.TrackSerial
	}
	if keep("warranty_months") {
		w.WarrantyMonths = p.WarrantyMonths
	}
	if keep("supplier") {
		w.PreferredSupplier = p.PreferredSupplierID
	}
	return w
}

// ImportResult reports what a single row did, for the import summary.
type ImportResult struct {
	ProductID int64
	Action    string // "created" | "updated"
	Note      string // optional caveat, e.g. opening stock skipped
}

// ImportOne upserts one catalog row in a single transaction. It matches an
// existing product by barcode (when given) and updates its master fields;
// otherwise it inserts a new product. Opening stock is seeded — at the real
// cost — only for a brand-new product or one currently holding zero on-hand, so
// re-running the same import never double-counts stock.
func (s *Service) ImportOne(ctx context.Context, in ImportRow) (ImportResult, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return ImportResult{}, apperr.Validation("name is required")
	}
	w := writeRow{
		Name:              name,
		NameLocal:         nullStr(strings.TrimSpace(in.NameLocal)),
		Barcode:           nullStr(strings.TrimSpace(in.Barcode)),
		CategoryID:        in.CategoryID,
		UnitID:            in.UnitID,
		Cost:              in.Cost,
		Selling:           in.Selling,
		Wholesale:         in.Wholesale,
		Tax:               in.Tax,
		Reorder:           in.Reorder,
		TrackSerial:       in.TrackSerial,
		WarrantyMonths:    in.WarrantyMonths,
		PreferredSupplier: nilIfZero(in.PreferredSupplier),
	}
	var res ImportResult
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		repo := NewRepository(tx)
		// Match an existing active product: prefer barcode, else fall back to name
		// so barcode-less products round-trip on re-import instead of duplicating.
		var existing *Product
		if w.Barcode != nil {
			if p, ferr := repo.FindByBarcode(ctx, *w.Barcode); ferr == nil {
				existing = p
			} else if !errors.Is(ferr, sql.ErrNoRows) {
				return ferr
			}
		}
		// A row WITHOUT a barcode always falls back to matching by name so
		// barcode-less products round-trip instead of duplicating. A row that
		// CARRIES a barcode only falls back to name when the caller opted in
		// (MatchByName) — otherwise a barcode that matched nothing is treated as a
		// different product, since two distinct same-named goods with different
		// barcodes used to merge into a single mongrel holding one row's quantity
		// and the other's barcode and price. When it does match by name here, the
		// update below writes w.Barcode, realigning the existing product's barcode.
		if existing == nil && (w.Barcode == nil || in.MatchByName) {
			if p, ferr := repo.FindByName(ctx, name); ferr == nil {
				existing = p
			} else if !errors.Is(ferr, sql.ErrNoRows) {
				return ferr
			}
		}
		if existing == nil && in.Active != nil {
			if p, ferr := findDisabled(ctx, tx, w.Barcode, name); ferr == nil {
				existing = p
			} else if !errors.Is(ferr, sql.ErrNoRows) {
				return ferr
			}
		}
		if existing != nil {
			if !existing.IsActive { // repo.Update writes active products only; setActive below restores the sheet's value
				on := true
				if err := setActive(ctx, tx, existing.ID, &on); err != nil {
					return err
				}
			}
			if uerr := repo.Update(ctx, existing.ID, keepMissing(w, existing, in.Given)); uerr != nil {
				return mapWriteErr(uerr)
			}
			res.ProductID, res.Action = existing.ID, "updated"
			if err := setActive(ctx, tx, existing.ID, in.Active); err != nil {
				return err
			}
			if in.OpeningQty.IsPositive() {
				if existing.StockQty.IsZero() {
					if serr := seedOpeningStock(ctx, tx, existing.ID, in.OpeningQty, in.Cost, in.UserID); serr != nil {
						return serr
					}
				} else {
					res.Note = "opening stock skipped (already in stock)"
				}
			}
			return nil
		}
		id, ierr := repo.Insert(ctx, w)
		if ierr != nil {
			return mapWriteErr(ierr)
		}
		res.ProductID, res.Action = id, "created"
		if err := setActive(ctx, tx, id, in.Active); err != nil {
			return err
		}
		if in.OpeningQty.IsPositive() {
			if serr := seedOpeningStock(ctx, tx, id, in.OpeningQty, in.Cost, in.UserID); serr != nil {
				return serr
			}
		}
		return nil
	})
	if err != nil {
		return ImportResult{}, err
	}
	return res, nil
}

// findDisabled matches a disabled product by barcode, else (no barcode) by name.
func findDisabled(ctx context.Context, tx *sqlx.Tx, barcode *string, name string) (*Product, error) {
	var p Product
	err := tx.GetContext(ctx, &p, selectProduct+`
		WHERE NOT p.is_active AND NOT p.is_service
		  AND (p.barcode = $1 OR ($1::text IS NULL AND LOWER(p.name) = LOWER($2)))
		ORDER BY p.id LIMIT 1`, barcode, name)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func setActive(ctx context.Context, tx *sqlx.Tx, id int64, active *bool) error {
	if active == nil {
		return nil
	}
	_, err := tx.ExecContext(ctx, `UPDATE products SET is_active=$2 WHERE id=$1`, id, *active)
	return err
}

// IntakeInput is a product first met on a supplier's delivery note.
type IntakeInput struct {
	Name    string `json:"name"          form:"name"`
	Cost    string `json:"cost_price"    form:"cost_price"`
	Selling string `json:"selling_price" form:"selling_price"`
	Barcode string `json:"barcode"       form:"barcode"`
	UnitID  int64  `json:"unit_id"       form:"unit_id"`
}

// CreateForIntake makes a product first met on a supplier's visit, so a
// purchase line can carry it. Stock always starts at zero.
//
// Deliberately not QuickCreate: that one exists for a sale about to happen, so
// it seeds stock to the quantity being sold and leaves cost at zero. Here the
// goods have not been booked yet — a receive posts the stock and opens the
// costed batch itself.
//
// Prices are optional because the two callers differ. A delivery has the
// invoice in hand, so the web layer insists on both. An order does not: the rep
// quotes from memory, and that number would become the catalogue price of
// something never seen. A priceless item is safe here because with no stock it
// cannot be sold, and receiving overwrites cost and price from the real invoice.
//
// Lands in Uncategorized and flagged needs_review, stamped with whoever entered
// it, so it surfaces in the admin review queue for a real category and a tidy
// name.
func (s *Service) CreateForIntake(ctx context.Context, in IntakeInput, userID int64) (*Product, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, apperr.Validation("item name is required")
	}
	cost, err := parseOptionalMoney(in.Cost)
	if err != nil {
		return nil, apperr.Validation("cost price must be a non-negative amount")
	}
	selling, err := parseOptionalMoney(in.Selling)
	if err != nil {
		return nil, apperr.Validation("selling price must be a non-negative amount")
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
		id, err := NewRepository(tx).Insert(ctx, writeRow{
			Name:        name,
			Barcode:     nullStr(strings.TrimSpace(in.Barcode)),
			CategoryID:  catID,
			UnitID:      unitID,
			Cost:        cost,
			Selling:     selling,
			Wholesale:   decimal.Zero,
			Tax:         decimal.Zero,
			NeedsReview: true,
			CreatedBy:   &userID,
		})
		if err != nil {
			return mapWriteErr(err)
		}
		newID = id
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, newID)
}

// parseOptionalMoney reads a money field that may legitimately be blank (zero).
func parseOptionalMoney(raw string) (decimal.Decimal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return decimal.Zero, nil
	}
	v, err := money.Parse(raw)
	if err != nil || v.IsNegative() {
		return decimal.Zero, errors.New("invalid amount")
	}
	return v, nil
}
