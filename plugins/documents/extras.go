package documents

import (
	"context"
	"net/http"
	"strings"

	"karots-suite/internal/apperr"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// Extra is something added to a job — Urgent, Hole punch, Corner rounding —
// at its own price per job or per piece, optionally taking stock (Qty of
// ProductID per job / piece). The till rings each ticked extra as its own line.
type Extra struct {
	ID          int64           `db:"id"`
	Name        string          `db:"name"`
	Price       decimal.Decimal `db:"price"`
	PerPiece    bool            `db:"per_piece"`
	ProductID   *int64          `db:"product_id"`
	ProductName *string         `db:"product_name"`
	Qty         decimal.Decimal `db:"qty"`
}

// ExtraDTO is an extra on the wire: money and qty as decimal strings.
type ExtraDTO struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Price       string  `json:"price"`
	PerPiece    bool    `json:"perPiece"`
	ProductID   *int64  `json:"productId"`
	ProductName *string `json:"productName"`
	Qty         string  `json:"qty"`
}

func extraDTOs(xs []Extra) []ExtraDTO {
	out := make([]ExtraDTO, 0, len(xs))
	for _, x := range xs {
		out = append(out, ExtraDTO{ID: x.ID, Name: x.Name, Price: x.Price.StringFixed(2), PerPiece: x.PerPiece,
			ProductID: x.ProductID, ProductName: x.ProductName, Qty: x.Qty.String()})
	}
	return out
}

// Extras lists a service's extras in the order the admin set.
func (s *Store) Extras(ctx context.Context, serviceID int64) ([]Extra, error) {
	out := []Extra{}
	err := s.db.SelectContext(ctx, &out, `SELECT e.id, e.name, e.price, e.per_piece, e.product_id, p.name AS product_name, e.qty
		FROM doc_extra e LEFT JOIN products p ON p.id = e.product_id
		WHERE e.service_id = $1 ORDER BY e.sort, e.id`, serviceID)
	return out, err
}

// SaveExtras replaces a service's extras.
func (s *Store) SaveExtras(ctx context.Context, serviceID int64, xs []Extra) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, `DELETE FROM doc_extra WHERE service_id = $1`, serviceID); err != nil {
		return err
	}
	for i, x := range xs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO doc_extra (service_id, name, price, per_piece, product_id, qty, sort)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, serviceID, x.Name, x.Price, x.PerPiece, x.ProductID, x.Qty, i); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// fromExtras validates extras as typed.
func fromExtras(in []ExtraDTO) ([]Extra, error) {
	out := make([]Extra, 0, len(in))
	seen := map[string]bool{}
	for _, x := range in {
		name := strings.TrimSpace(x.Name)
		if name == "" || len(name) > 40 {
			return nil, apperr.Validation("give each extra a name (at most 40 letters)")
		}
		if seen[strings.ToLower(name)] {
			return nil, apperr.Validation("each extra once — " + name + " is there twice")
		}
		seen[strings.ToLower(name)] = true
		price, err := decimal.NewFromString(strings.TrimSpace(x.Price))
		if err != nil || price.IsNegative() {
			return nil, apperr.Validation(name + ": the price must be a number of 0 or more")
		}
		e := Extra{Name: name, Price: price.Round(2), PerPiece: x.PerPiece, Qty: decimal.NewFromInt(1)}
		if x.ProductID != nil && *x.ProductID > 0 {
			q, err := decimal.NewFromString(strings.TrimSpace(x.Qty))
			if err != nil || !q.IsPositive() {
				return nil, apperr.Validation(name + ": say how much stock it takes")
			}
			e.ProductID, e.Qty = x.ProductID, q
		}
		out = append(out, e)
	}
	return out, nil
}

func (h handlers) extras(c echo.Context) error {
	sid, err := id(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	if _, err := h.p.store.Service(ctx, sid); err != nil {
		return err
	}
	xs, err := h.p.store.Extras(ctx, sid)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, extraDTOs(xs))
}

func (h handlers) saveExtras(c echo.Context) error {
	sid, err := id(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	if _, err := h.p.store.Service(ctx, sid); err != nil {
		return err
	}
	var in []ExtraDTO
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid body")
	}
	xs, err := fromExtras(in)
	if err != nil {
		return err
	}
	for _, x := range xs {
		if x.ProductID == nil {
			continue
		}
		var svc bool
		if err := h.p.core.DB.GetContext(ctx, &svc, `SELECT is_service FROM products WHERE id = $1`, *x.ProductID); err != nil {
			return apperr.Validation(x.Name + ": pick the stock from your products")
		}
		if svc {
			return apperr.Validation(x.Name + ": the stock must be a stocked item, not a service")
		}
	}
	if err := h.p.store.SaveExtras(ctx, sid, xs); err != nil {
		return err
	}
	h.audit(c, "documents_extras_save", sid, strings.Join(names(xs), ", "))
	return h.extras(c)
}

func names(xs []Extra) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, x.Name)
	}
	return out
}
