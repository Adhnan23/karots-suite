package stock

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// Stock freeze: a frozen product sells without touching its stock (the front
// shelf isn't counted); counts, GRNs and conversions still work. Unfreezing adds
// what's on the shelf. Neither opening a box onto the shelf nor unfreezing is an
// adjustment or a loss: both are 'to_shelf' movements, which no report sums.

// FreezeRefusals is set at the composition root from the plugins' freeze guards
// (products a plugin must keep counted, with the reason). Nil = none refused.
var FreezeRefusals func(ctx context.Context, ids []int64) (map[int64]string, error)

// Frozen reports whether a product's stock is frozen (its sales don't touch stock).
func (r *Repository) Frozen(ctx context.Context, productID int64) (bool, error) {
	var on bool
	err := r.q.GetContext(ctx, &on, `SELECT stock_frozen FROM products WHERE id = $1`, productID)
	return on, err
}

// FreezeResult: how many were frozen, and the ones a plugin refused (id → reason).
type FreezeResult struct {
	Frozen  int              `json:"frozen"`
	Refused map[int64]string `json:"refused"`
}

// Freeze freezes these products (services and refused ones are skipped).
func (s *Service) Freeze(ctx context.Context, ids []int64) (FreezeResult, error) {
	res := FreezeResult{Refused: map[int64]string{}}
	if len(ids) == 0 {
		return res, nil
	}
	if FreezeRefusals != nil {
		got, err := FreezeRefusals(ctx, ids)
		if err != nil {
			return res, apperr.Internal("failed to check plugins", err)
		}
		res.Refused = got
	}
	ok := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, no := res.Refused[id]; !no {
			ok = append(ok, id)
		}
	}
	r, err := s.db.ExecContext(ctx, `UPDATE products SET stock_frozen = true, stock_frozen_at = now()
		WHERE id = ANY($1) AND NOT is_service AND NOT stock_frozen`, pq.Array(ok))
	if err != nil {
		return res, apperr.Internal("failed to freeze", err)
	}
	n, _ := r.RowsAffected()
	res.Frozen = int(n)
	return res, nil
}

// FreezeShop freezes every product and makes new ones start frozen.
func (s *Service) FreezeShop(ctx context.Context) (FreezeResult, error) {
	var ids []int64
	if err := s.db.SelectContext(ctx, &ids, `SELECT id FROM products WHERE NOT is_service AND NOT stock_frozen AND merged_into_id IS NULL`); err != nil {
		return FreezeResult{}, apperr.Internal("failed to list products", err)
	}
	res, err := s.Freeze(ctx, ids)
	if err != nil {
		return res, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE settings SET freeze_new_products = true WHERE id = 1`); err != nil {
		return res, apperr.Internal("failed to save the setting", err)
	}
	return res, nil
}

// UnfreezeShop unfreezes every product without changing stock (the admin counted
// the front shelves, or accepts them uncounted) and stops freezing new ones.
func (s *Service) UnfreezeShop(ctx context.Context) (int, error) {
	n := 0
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		r, err := tx.ExecContext(ctx, `UPDATE products SET stock_frozen = false, stock_frozen_at = NULL WHERE stock_frozen`)
		if err != nil {
			return apperr.Internal("failed to unfreeze", err)
		}
		c, _ := r.RowsAffected()
		n = int(c)
		_, err = tx.ExecContext(ctx, `UPDATE settings SET freeze_new_products = false WHERE id = 1`)
		return err
	})
	return n, err
}

// lockFrozen locks the product row and refuses one that isn't frozen.
func lockFrozen(ctx context.Context, tx *sqlx.Tx, productID int64) error {
	var on bool
	if err := tx.GetContext(ctx, &on, `SELECT stock_frozen FROM products WHERE id = $1 FOR UPDATE`, productID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound("product")
		}
		return apperr.Internal("failed to read product", err)
	}
	if !on {
		return apperr.Conflict("this product isn't frozen")
	}
	return nil
}

func shelfQty(raw string, what string) (decimal.Decimal, error) {
	q, err := money.Parse(strings.TrimSpace(raw))
	if strings.TrimSpace(raw) == "" || err != nil || q.IsNegative() {
		return q, apperr.Validation(what + " must be a number, 0 or more")
	}
	return q, nil
}

// Unfreeze adds what's on the shelf now (0 allowed) to stock and unfreezes;
// from then on its sales reduce stock.
func (s *Service) Unfreeze(ctx context.Context, productID int64, shelf string, userID int64) error {
	qty, err := shelfQty(shelf, "how many are out for sale")
	if err != nil {
		return err
	}
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		if err := lockFrozen(ctx, tx, productID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE products SET stock_frozen = false, stock_frozen_at = NULL WHERE id = $1`, productID); err != nil {
			return apperr.Internal("failed to unfreeze", err)
		}
		if !qty.IsPositive() {
			return nil
		}
		repo := NewRepository(tx)
		cost, err := repo.ProductCost(ctx, productID)
		if err != nil {
			return apperr.Internal("failed to read cost", err)
		}
		if err := repo.Increment(ctx, productID, qty); err != nil {
			return apperr.Internal("failed to update stock", err)
		}
		if _, err := repo.InsertBatch(ctx, NewBatch{ProductID: productID, Quantity: qty, CostPrice: cost, Source: "shelf"}); err != nil {
			return apperr.Internal("failed to open the shelf lot", err)
		}
		return repo.InsertMovement(ctx, MovementInput{ProductID: productID, Type: MoveToShelf, Quantity: qty,
			UserID: userID, Note: nilIfEmpty("unfrozen: " + qty.String() + " out for sale"), Cost: cost.Mul(qty)})
	})
}

// MoveToShelf books a frozen product's box opened onto the front shelf: out of
// the counted stock, never more than it holds.
func (s *Service) MoveToShelf(ctx context.Context, productID int64, raw string, userID int64) error {
	qty, err := shelfQty(raw, "quantity")
	if err != nil {
		return err
	}
	if !qty.IsPositive() {
		return apperr.Validation("quantity must be greater than zero")
	}
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		if err := lockFrozen(ctx, tx, productID); err != nil {
			return err
		}
		repo := NewRepository(tx)
		ok, err := repo.DecrementGuarded(ctx, productID, qty)
		if err != nil {
			return apperr.Internal("failed to update stock", err)
		}
		if !ok {
			return apperr.Conflict("that's more than the counted stock")
		}
		cost, err := repo.DepleteFEFO(ctx, productID, qty)
		if err != nil {
			return apperr.Internal("failed to deplete batches", err)
		}
		return repo.InsertMovement(ctx, MovementInput{ProductID: productID, Type: MoveToShelf, Quantity: qty.Neg(),
			UserID: userID, Note: nilIfEmpty("put out for sale"), Cost: cost.Mul(qty)})
	})
}

// FrozenRow is one product on the Frozen list.
type FrozenRow struct {
	ID    int64           `db:"id"         json:"id"`
	Name  string          `db:"name"       json:"name"`
	Stock decimal.Decimal `db:"stock"      json:"stock"`
	Unit  string          `db:"unit"       json:"unit"`
	Since *time.Time      `db:"since"      json:"since,omitempty"`
}

// FrozenProducts lists the frozen products, longest frozen first.
func (s *Service) FrozenProducts(ctx context.Context) ([]FrozenRow, error) {
	rows := []FrozenRow{}
	err := s.db.SelectContext(ctx, &rows, `
		SELECT p.id, p.name, COALESCE(s.quantity, 0) AS stock, u.abbreviation AS unit, p.stock_frozen_at AS since
		FROM products p JOIN units u ON u.id = p.unit_id LEFT JOIN stock s ON s.product_id = p.id
		WHERE p.stock_frozen ORDER BY p.stock_frozen_at NULLS FIRST, p.name`)
	if err != nil {
		return nil, apperr.Internal("failed to list frozen products", err)
	}
	return rows, nil
}
