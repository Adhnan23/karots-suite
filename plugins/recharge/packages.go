package recharge

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

const packageCols = `p.id, p.carrier_id, p.name, p.price, p.float_cost, p.is_active, p.sort, p.reviewed_at,
	CASE WHEN p.reviewed_at IS NULL THEN COALESCE(u.name, '') ELSE '' END AS added_by`

// Packages lists packages (active ones unless all), carrier then sort / price.
func (s *Store) Packages(ctx context.Context, all bool) ([]Package, error) {
	out := []Package{}
	err := s.db.SelectContext(ctx, &out, `SELECT `+packageCols+` FROM recharge_packages p
		LEFT JOIN users u ON u.id = p.added_at_till_by
		WHERE p.is_active OR $1 ORDER BY p.carrier_id, NOT p.is_active, p.sort, p.price, p.name`, all)
	return out, err
}

func (s *Store) pkg(ctx context.Context, q appdb.Queryer, id int64) (*Package, error) {
	var p Package
	err := q.GetContext(ctx, &p, `SELECT `+packageCols+` FROM recharge_packages p LEFT JOIN users u ON u.id = p.added_at_till_by WHERE p.id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.Validation("that package isn't set up")
	}
	return &p, err
}

// PackageInput is the package form; at the till the cashier types the commission
// the carrier's app shows (FloatCost = Price − Commission), in admin either one.
type PackageInput struct {
	CarrierID  int64            `json:"carrierId"`
	Name       string           `json:"name"`
	Price      decimal.Decimal  `json:"price"`
	FloatCost  *decimal.Decimal `json:"floatCost,omitempty"`
	Commission *decimal.Decimal `json:"commission,omitempty"`
	Sort       int              `json:"sort"`
	IsActive   *bool            `json:"isActive,omitempty"`
}

func (in PackageInput) floatCost() (decimal.Decimal, error) {
	if strings.TrimSpace(in.Name) == "" {
		return decimal.Zero, apperr.Validation("name the package")
	}
	if !in.Price.IsPositive() {
		return decimal.Zero, apperr.Validation("enter the package price")
	}
	fc := in.Price
	switch {
	case in.FloatCost != nil:
		fc = *in.FloatCost
	case in.Commission != nil:
		fc = in.Price.Sub(*in.Commission)
	}
	if fc.IsNegative() || fc.GreaterThan(in.Price) {
		return decimal.Zero, apperr.Validation("the commission is between 0 and the price")
	}
	return fc, nil
}

// CreatePackage adds a package. tillBy > 0 = added at the till by that cashier (in
// a hurry): flagged on Carriers until an admin reviews it. A name the carrier
// already has returns that package instead of a twin (created false).
func (s *Store) CreatePackage(ctx context.Context, in PackageInput, tillBy int64) (p *Package, created bool, err error) {
	fc, err := in.floatCost()
	if err != nil {
		return nil, false, err
	}
	name := strings.TrimSpace(in.Name)
	err = appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		c, err := s.Carrier(ctx, tx, in.CarrierID)
		if err != nil {
			return err
		}
		if !c.IsActive {
			return apperr.Validation(c.Name + " is turned off")
		}
		var id int64
		err = tx.GetContext(ctx, &id, `SELECT id FROM recharge_packages WHERE carrier_id = $1 AND lower(name) = lower($2) AND is_active`, in.CarrierID, name)
		if errors.Is(err, sql.ErrNoRows) {
			var by *int64
			if tillBy > 0 {
				by = &tillBy
			}
			err = tx.GetContext(ctx, &id, `INSERT INTO recharge_packages (carrier_id, name, price, float_cost, sort, added_at_till_by, reviewed_at)
				VALUES ($1, $2, $3, $4, $5, $6, CASE WHEN $6::bigint IS NULL THEN now() END) RETURNING id`, in.CarrierID, name, in.Price, fc, in.Sort, by)
			created = true
		}
		if err != nil {
			return err
		}
		p, err = s.pkg(ctx, tx, id)
		return err
	})
	return p, created, err
}

// UpdatePackage edits a package in admin; saving it counts as the review.
func (s *Store) UpdatePackage(ctx context.Context, id int64, in PackageInput) error {
	fc, err := in.floatCost()
	if err != nil {
		return err
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	res, err := s.db.ExecContext(ctx, `UPDATE recharge_packages SET name = $2, price = $3, float_cost = $4, sort = $5, is_active = $6,
		reviewed_at = COALESCE(reviewed_at, now()) WHERE id = $1`, id, strings.TrimSpace(in.Name), in.Price, fc, in.Sort, active)
	if err != nil {
		if strings.Contains(err.Error(), "recharge_packages_name_idx") {
			return apperr.Conflict("that carrier already has a package called " + in.Name)
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("package")
	}
	return nil
}

// ReviewPackage clears a till-added package's "new" flag as it is.
func (s *Store) ReviewPackage(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE recharge_packages SET reviewed_at = now() WHERE id = $1 AND reviewed_at IS NULL`, id)
	return err
}
