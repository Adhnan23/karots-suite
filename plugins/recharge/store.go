package recharge

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/products"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// Store is the plugin's data over the core database. It writes only recharge_*
// (and its own products through products.CreateTx).
type Store struct{ db *sqlx.DB }

func NewStore(db *sqlx.DB) *Store { return &Store{db: db} }

// Carrier is a carrier (Dialog, Mobitel, …); ProductID is the pass-through
// service product its reloads and packages ring on.
type Carrier struct {
	ID        int64  `db:"id"         json:"id"`
	Name      string `db:"name"       json:"name"`
	ProductID int64  `db:"product_id" json:"productId"`
	IsActive  bool   `db:"is_active"  json:"isActive"`
}

// Device is a SIM / phone / terminal under a carrier holding a float. Reload and
// Money say which picker it shows in (Mobitel keeps two floats = two devices).
type Device struct {
	ID          int64           `db:"id"                json:"id"`
	CarrierID   int64           `db:"carrier_id"        json:"carrierId"`
	Carrier     string          `db:"carrier"           json:"carrier"`
	Label       string          `db:"label"             json:"label"`
	Number      string          `db:"number"            json:"number"`
	Reload      bool            `db:"for_recharge"      json:"reload"`
	Money       bool            `db:"for_money"         json:"money"`
	TracksFloat bool            `db:"tracks_float"      json:"-"`
	IsActive    bool            `db:"is_active"         json:"isActive"`
	Target      decimal.Decimal `db:"recommended_float" json:"target"`
}

// Package is a carrier package sold from its float.
type Package struct {
	ID         int64           `db:"id"          json:"id"`
	CarrierID  int64           `db:"carrier_id"  json:"carrierId"`
	Name       string          `db:"name"        json:"name"`
	Price      decimal.Decimal `db:"price"       json:"price"`
	FloatCost  decimal.Decimal `db:"float_cost"  json:"floatCost"`
	IsActive   bool            `db:"is_active"   json:"isActive"`
	Sort       int             `db:"sort"        json:"sort"`
	AddedBy    string          `db:"added_by"    json:"addedBy"` // cashier who added it at the till, until reviewed
	ReviewedAt *time.Time      `db:"reviewed_at" json:"-"`
}

func (s *Store) Carriers(ctx context.Context, all bool) ([]Carrier, error) {
	out := []Carrier{}
	err := s.db.SelectContext(ctx, &out, `SELECT id, name, product_id, is_active FROM recharge_carriers
		WHERE is_active OR $1 ORDER BY NOT is_active, name`, all)
	return out, err
}

func (s *Store) Carrier(ctx context.Context, q appdb.Queryer, id int64) (*Carrier, error) {
	var c Carrier
	err := q.GetContext(ctx, &c, `SELECT id, name, product_id, is_active FROM recharge_carriers WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("carrier")
	}
	return &c, err
}

// nameTaken: another enabled carrier already uses name (case-insensitive).
func (s *Store) nameTaken(ctx context.Context, q appdb.Queryer, name string, except int64) error {
	var n int
	if err := q.GetContext(ctx, &n, `SELECT count(*) FROM recharge_carriers WHERE is_active AND lower(name) = lower($1) AND id <> $2`, name, except); err != nil {
		return err
	}
	if n > 0 {
		return apperr.Conflict("a carrier called " + name + " already exists")
	}
	return nil
}

// CreateCarrier adds a carrier and its pass-through service product (in Recharge).
func (s *Store) CreateCarrier(ctx context.Context, name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, apperr.Validation("name the carrier")
	}
	var id int64
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		if err := s.nameTaken(ctx, tx, name, 0); err != nil {
			return err
		}
		pid, err := newProduct(ctx, tx, name+" Recharge", true)
		if err != nil {
			return err
		}
		return tx.GetContext(ctx, &id, `INSERT INTO recharge_carriers (name, product_id) VALUES ($1, $2) RETURNING id`, name, pid)
	})
	return id, err
}

// RenameCarrier renames a carrier and its product (a rebrand keeps its history).
func (s *Store) RenameCarrier(ctx context.Context, id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return apperr.Validation("name the carrier")
	}
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		c, err := s.Carrier(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.nameTaken(ctx, tx, name, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE recharge_carriers SET name = $2 WHERE id = $1`, id, name); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE products SET name = $2 WHERE id = $1`, c.ProductID, name+" Recharge")
		return err
	})
}

// SetCarrierActive disables or brings back a carrier (never deleted).
func (s *Store) SetCarrierActive(ctx context.Context, id int64, on bool) error {
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		c, err := s.Carrier(ctx, tx, id)
		if err != nil {
			return err
		}
		if on {
			if err := s.nameTaken(ctx, tx, c.Name, id); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE recharge_carriers SET is_active = $2 WHERE id = $1`, id, on)
		return err
	})
}

const deviceCols = `d.id, d.carrier_id, c.name AS carrier, d.label, COALESCE(d.number, '') AS number, d.for_recharge, d.for_money,
	d.tracks_float, d.is_active, d.recommended_float`

func (s *Store) Devices(ctx context.Context, all bool) ([]Device, error) {
	out := []Device{}
	err := s.db.SelectContext(ctx, &out, `SELECT `+deviceCols+` FROM recharge_devices d JOIN recharge_carriers c ON c.id = d.carrier_id
		WHERE d.tracks_float AND ((d.is_active AND c.is_active) OR $1) ORDER BY c.name, d.label`, all)
	return out, err
}

func (s *Store) Device(ctx context.Context, q appdb.Queryer, id int64) (*Device, error) {
	var d Device
	err := q.GetContext(ctx, &d, `SELECT `+deviceCols+` FROM recharge_devices d JOIN recharge_carriers c ON c.id = d.carrier_id WHERE d.id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.Validation("that device isn't set up")
	}
	return &d, err
}

// DeviceInput is the device form. Opening is the float already on it when it is
// added (onboarding — like opening stock, no expense).
type DeviceInput struct {
	CarrierID int64           `json:"carrierId"`
	Label     string          `json:"label"`
	Number    string          `json:"number"`
	Reload    bool            `json:"reload"`
	Money     bool            `json:"money"`
	Target    decimal.Decimal `json:"target"`
	Opening   decimal.Decimal `json:"opening"`
}

func (in DeviceInput) check() error {
	if strings.TrimSpace(in.Label) == "" {
		return apperr.Validation("name the device (e.g. Shop phone)")
	}
	if !in.Reload && !in.Money {
		return apperr.Validation("a device is for reloads, money or both")
	}
	if in.Target.IsNegative() || in.Opening.IsNegative() {
		return apperr.Validation("amounts can't be below zero")
	}
	return nil
}

func (s *Store) CreateDevice(ctx context.Context, in DeviceInput, by int64) (int64, error) {
	if err := in.check(); err != nil {
		return 0, err
	}
	var id int64
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		if _, err := s.Carrier(ctx, tx, in.CarrierID); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &id, `INSERT INTO recharge_devices (carrier_id, label, number, for_recharge, for_money, recommended_float)
			VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6) RETURNING id`, in.CarrierID, strings.TrimSpace(in.Label), strings.TrimSpace(in.Number),
			in.Reload, in.Money, in.Target); err != nil {
			return err
		}
		if !in.Opening.IsPositive() {
			return nil
		}
		_, err := recordTx(ctx, tx, TxInput{CarrierID: in.CarrierID, DeviceID: id, Type: "opening", Amount: in.Opening,
			FloatDelta: in.Opening, Note: "float on it when added", CreatedBy: by})
		return err
	})
	return id, err
}

// UpdateDevice edits a device in place (its carrier never changes: the float is that carrier's).
func (s *Store) UpdateDevice(ctx context.Context, id int64, in DeviceInput) error {
	if err := in.check(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE recharge_devices SET label = $2, number = NULLIF($3, ''), for_recharge = $4, for_money = $5,
		recommended_float = $6 WHERE id = $1`, id, strings.TrimSpace(in.Label), strings.TrimSpace(in.Number), in.Reload, in.Money, in.Target)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("device")
	}
	return nil
}

func (s *Store) SetDeviceActive(ctx context.Context, id int64, on bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE recharge_devices SET is_active = $2 WHERE id = $1`, id, on)
	return err
}

// Rates returns a carrier's rate for every kind (zero where none is set).
func (s *Store) Rates(ctx context.Context, q appdb.Queryer, carrierID int64) ([]Rate, error) {
	var set []Rate
	if err := q.SelectContext(ctx, &set, `SELECT kind, percent, fixed, charge FROM recharge_rates WHERE carrier_id = $1`, carrierID); err != nil {
		return nil, err
	}
	out := make([]Rate, 0, len(kinds))
	for _, k := range kinds {
		r := Rate{Kind: k}
		for _, x := range set {
			if x.Kind == k {
				r = x
			}
		}
		out = append(out, r)
	}
	return out, nil
}

func rateOf(rs []Rate, kind string) Rate {
	for _, r := range rs {
		if r.Kind == kind {
			return r
		}
	}
	return Rate{Kind: kind}
}

func (s *Store) SetRates(ctx context.Context, carrierID int64, rs []Rate) error {
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		if _, err := s.Carrier(ctx, tx, carrierID); err != nil {
			return err
		}
		for _, r := range rs {
			if !validKind(r.Kind) {
				return apperr.Validation("unknown kind " + r.Kind)
			}
			if r.Percent.IsNegative() || r.Fixed.IsNegative() || r.Charge.IsNegative() || r.Percent.GreaterThan(hundred) {
				return apperr.Validation("rates are between 0 and 100%, amounts 0 or more")
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO recharge_rates (carrier_id, kind, percent, fixed, charge) VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (carrier_id, kind) DO UPDATE SET percent = EXCLUDED.percent, fixed = EXCLUDED.fixed, charge = EXCLUDED.charge`,
				carrierID, r.Kind, r.Percent, r.Fixed, r.Charge); err != nil {
				return err
			}
		}
		return nil
	})
}

// newProduct makes one of the plugin's service products in the Recharge category
// (karots-pos's), pass-through when it is money passing through.
func newProduct(ctx context.Context, tx *sqlx.Tx, name string, passThrough bool) (int64, error) {
	var cat, unit int64
	err := tx.GetContext(ctx, &cat, `SELECT id FROM categories WHERE name = 'Recharge' AND parent_id IS NULL ORDER BY id LIMIT 1`)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.GetContext(ctx, &cat, `INSERT INTO categories (name) VALUES ('Recharge') RETURNING id`)
	}
	if err != nil {
		return 0, err
	}
	if err := tx.GetContext(ctx, &unit, `SELECT id FROM units ORDER BY (abbreviation = 'pcs') DESC, id LIMIT 1`); err != nil {
		return 0, apperr.Validation("add a unit first")
	}
	id, err := products.CreateTx(ctx, tx, products.CreateInput{Name: name, CategoryID: cat, UnitID: unit,
		CostPrice: "0", SellingPrice: "0", WholesalePrice: "0", TaxRate: "0", IsService: true})
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE products SET pass_through = $2 WHERE id = $1`, id, passThrough)
	return id, err
}

// ownProducts are the plugin's products by key: name and whether money passes through.
var ownProducts = map[string]struct {
	name        string
	passThrough bool
}{
	"billpay": {"Bill payment", true},
	"deposit": {"Wallet deposit", true},
	"cashout": {"Cash out", true},
	"charge":  {"Service charge", false},
}

// ProductIDs returns the plugin's own products, making any that are missing once.
func (s *Store) ProductIDs(ctx context.Context) (map[string]int64, error) {
	out := map[string]int64{}
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		// One maker at a time: two tills opening the menu at once make one set.
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('recharge_config'))`); err != nil {
			return err
		}
		var rows []struct {
			Key       string `db:"key"`
			ProductID int64  `db:"product_id"`
		}
		if err := tx.SelectContext(ctx, &rows, `SELECT c.key, c.product_id FROM recharge_config c JOIN products p ON p.id = c.product_id`); err != nil {
			return err
		}
		for _, r := range rows {
			out[r.Key] = r.ProductID
		}
		for key, p := range ownProducts {
			if out[key] != 0 {
				continue
			}
			id, err := newProduct(ctx, tx, p.name, p.passThrough)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO recharge_config (key, product_id) VALUES ($1, $2)
				ON CONFLICT (key) DO UPDATE SET product_id = EXCLUDED.product_id`, key, id); err != nil {
				return err
			}
			out[key] = id
		}
		return nil
	})
	return out, err
}
