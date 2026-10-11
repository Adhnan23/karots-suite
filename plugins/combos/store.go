package combos

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"karots-suite/internal/apperr"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

type Store struct{ db *sqlx.DB }

func NewStore(db *sqlx.DB) *Store { return &Store{db: db} }

var hundred = decimal.NewFromInt(100)

// Combo is a deal with its members read live (current prices, stock, state),
// where it stands against its dates, and what it has sold.
type Combo struct {
	ID            int64           `db:"id"`
	Name          string          `db:"name"`
	DiscountType  string          `db:"discount_type"` // percent | fixed (per set) | price (a set costs)
	DiscountValue decimal.Decimal `db:"discount_value"`
	IsActive      bool            `db:"is_active"`
	StartsOn      *string         `db:"starts_on"` // YYYY-MM-DD, inclusive
	EndsOn        *string         `db:"ends_on"`
	Window        string          `db:"win"` // on | scheduled | ended
	SoldSets      decimal.Decimal `db:"sold_sets"`
	SoldNet       decimal.Decimal `db:"sold_net"`
	LastSold      *time.Time      `db:"last_sold"`
	Items         []Item
	FullPrice     decimal.Decimal // one set at retail
}

type Item struct {
	ComboID        int64           `db:"combo_id"`
	ProductID      int64           `db:"product_id"`
	Qty            decimal.Decimal `db:"qty"`
	Name           string          `db:"name"`
	SellingPrice   decimal.Decimal `db:"selling_price"`
	WholesalePrice decimal.Decimal `db:"wholesale_price"`
	Active         bool            `db:"is_active"`
	IsService      bool            `db:"is_service"`
	Stock          decimal.Decimal `db:"stock"`
}

// Problems is why the combo can't be added at the till ("" = it can).
func (c Combo) Problems() string {
	for _, it := range c.Items {
		switch {
		case !it.Active:
			return it.Name + " is disabled"
		case it.IsService:
			return it.Name + " is a service"
		}
	}
	return ""
}

// Warnings are members short of a set's worth of stock (the till still adds;
// checkout's found-at-till prompt takes over).
func (c Combo) Warnings() []string {
	var out []string
	for _, it := range c.Items {
		if it.Active && !it.IsService && it.Stock.LessThan(it.Qty) {
			out = append(out, it.Name+": "+it.Stock.String()+" in stock")
		}
	}
	return out
}

type ItemInput struct {
	ProductID int64
	Qty       decimal.Decimal
}

type Input struct {
	Name          string
	DiscountType  string
	DiscountValue decimal.Decimal
	StartsOn      string
	EndsOn        string
	Items         []ItemInput
}

const comboCols = `c.id, c.name, c.discount_type, c.discount_value, c.is_active,
	to_char(c.starts_on, 'YYYY-MM-DD') AS starts_on, to_char(c.ends_on, 'YYYY-MM-DD') AS ends_on,
	CASE WHEN c.starts_on > CURRENT_DATE THEN 'scheduled' WHEN c.ends_on < CURRENT_DATE THEN 'ended' ELSE 'on' END AS win,
	COALESCE(sold.sets, 0) AS sold_sets, COALESCE(sold.net, 0) AS sold_net, sold.last AS last_sold`

const soldJoin = ` LEFT JOIN (
	SELECT cs.combo_id, SUM(cs.sets) AS sets, SUM(cs.net) AS net, MAX(cs.created_at) AS last
	FROM combo_sales cs JOIN sales s ON s.id = cs.sale_id AND s.status <> 'void'
	GROUP BY cs.combo_id) sold ON sold.combo_id = c.id`

// inWindow: active and today is within its dates.
const inWindow = `c.is_active AND (c.starts_on IS NULL OR c.starts_on <= CURRENT_DATE) AND (c.ends_on IS NULL OR c.ends_on >= CURRENT_DATE)`

func (s *Store) list(ctx context.Context, where string, args ...any) ([]Combo, error) {
	q := `SELECT ` + comboCols + ` FROM combos c` + soldJoin
	if where != "" {
		q += ` WHERE ` + where
	}
	q += ` ORDER BY c.is_active DESC, COALESCE(c.starts_on > CURRENT_DATE OR c.ends_on < CURRENT_DATE, false), c.name, c.id`
	var out []Combo
	if err := s.db.SelectContext(ctx, &out, q, args...); err != nil {
		return nil, err
	}
	return out, s.loadItems(ctx, out)
}

func (s *Store) loadItems(ctx context.Context, cs []Combo) error {
	if len(cs) == 0 {
		return nil
	}
	ids := make([]int64, len(cs))
	at := map[int64]int{}
	for i, c := range cs {
		ids[i], at[c.ID] = c.ID, i
	}
	q, args, err := sqlx.In(`
		SELECT ci.combo_id, ci.product_id, ci.qty, p.name, p.selling_price, p.wholesale_price,
		       p.is_active, p.is_service, COALESCE(st.quantity, 0) AS stock
		FROM combo_items ci
		JOIN products p ON p.id = ci.product_id
		LEFT JOIN stock st ON st.product_id = p.id
		WHERE ci.combo_id IN (?) ORDER BY ci.id`, ids)
	if err != nil {
		return err
	}
	var items []Item
	if err := s.db.SelectContext(ctx, &items, s.db.Rebind(q), args...); err != nil {
		return err
	}
	for _, it := range items {
		c := &cs[at[it.ComboID]]
		c.Items = append(c.Items, it)
		c.FullPrice = c.FullPrice.Add(it.SellingPrice.Mul(it.Qty))
	}
	return nil
}

// List is every combo (admin).
func (s *Store) List(ctx context.Context) ([]Combo, error) { return s.list(ctx, "") }

// Till is the combos a cashier can ring today.
func (s *Store) Till(ctx context.Context) ([]Combo, error) { return s.list(ctx, inWindow) }

func (s *Store) Get(ctx context.Context, id int64) (Combo, error) {
	cs, err := s.list(ctx, `c.id = $1`, id)
	if err != nil {
		return Combo{}, err
	}
	if len(cs) == 0 {
		return Combo{}, apperr.NotFound("combo")
	}
	return cs[0], nil
}

func (s *Store) validate(ctx context.Context, in *Input) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return apperr.Validation("Give the combo a name")
	}
	switch in.DiscountType {
	case "percent":
		if in.DiscountValue.GreaterThan(hundred) {
			return apperr.Validation("A percent off can't be over 100")
		}
	case "fixed":
	case "price":
		if !in.DiscountValue.IsPositive() {
			return apperr.Validation("Set the combo price")
		}
	default:
		return apperr.Validation("Pick % off, Rs off or a combo price")
	}
	if in.DiscountValue.IsNegative() {
		return apperr.Validation("The discount can't be negative")
	}
	for _, d := range []string{in.StartsOn, in.EndsOn} {
		if d == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", d); err != nil {
			return apperr.Validation("Dates are YYYY-MM-DD")
		}
	}
	if in.StartsOn != "" && in.EndsOn != "" && in.EndsOn < in.StartsOn {
		return apperr.Validation("The end date is before the start")
	}
	units := decimal.Zero
	seen := map[int64]bool{}
	ids := []int64{}
	for _, it := range in.Items {
		if !it.Qty.IsPositive() {
			return apperr.Validation("Each item needs a quantity")
		}
		if seen[it.ProductID] {
			return apperr.Validation("An item is in the combo twice — raise its quantity instead")
		}
		seen[it.ProductID] = true
		ids = append(ids, it.ProductID)
		units = units.Add(it.Qty)
	}
	if units.LessThan(decimal.NewFromInt(2)) {
		return apperr.Validation("A combo needs at least 2 units — two products, or one product ×2 or more")
	}
	q, args, err := sqlx.In(`SELECT name, is_active, is_service FROM products WHERE id IN (?)`, ids)
	if err != nil {
		return err
	}
	var rows []struct {
		Name      string `db:"name"`
		Active    bool   `db:"is_active"`
		IsService bool   `db:"is_service"`
	}
	if err := s.db.SelectContext(ctx, &rows, s.db.Rebind(q), args...); err != nil {
		return err
	}
	if len(rows) != len(ids) {
		return apperr.Validation("An item in the combo no longer exists")
	}
	for _, r := range rows {
		if !r.Active {
			return apperr.Validation(r.Name + " is disabled")
		}
		if r.IsService {
			return apperr.Validation(r.Name + " is a service — combos are for products")
		}
	}
	return nil
}

func nullDate(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *Store) Create(ctx context.Context, in Input) (int64, error) {
	if err := s.validate(ctx, &in); err != nil {
		return 0, err
	}
	var id int64
	err := s.withTx(ctx, func(tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, &id, `INSERT INTO combos (name, discount_type, discount_value, starts_on, ends_on)
			VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			in.Name, in.DiscountType, in.DiscountValue, nullDate(in.StartsOn), nullDate(in.EndsOn)); err != nil {
			return err
		}
		return insertItems(ctx, tx, id, in.Items)
	})
	return id, err
}

func (s *Store) Update(ctx context.Context, id int64, in Input) error {
	if err := s.validate(ctx, &in); err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE combos SET name=$1, discount_type=$2, discount_value=$3, starts_on=$4, ends_on=$5 WHERE id=$6`,
			in.Name, in.DiscountType, in.DiscountValue, nullDate(in.StartsOn), nullDate(in.EndsOn), id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return apperr.NotFound("combo")
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM combo_items WHERE combo_id=$1`, id); err != nil {
			return err
		}
		return insertItems(ctx, tx, id, in.Items)
	})
}

func insertItems(ctx context.Context, tx *sqlx.Tx, comboID int64, items []ItemInput) error {
	for _, it := range items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO combo_items (combo_id, product_id, qty) VALUES ($1,$2,$3)`,
			comboID, it.ProductID, it.Qty); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) exec(ctx context.Context, q string, args ...any) error {
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("combo")
	}
	return nil
}

func (s *Store) SetActive(ctx context.Context, id int64, active bool) error {
	return s.exec(ctx, `UPDATE combos SET is_active=$1 WHERE id=$2`, active, id)
}

// Delete removes the combo; its combo_sales rows stay (combo_id → NULL, name kept).
func (s *Store) Delete(ctx context.Context, id int64) error {
	return s.exec(ctx, `DELETE FROM combos WHERE id=$1`, id)
}

// BadgesFor tags every product in a combo the till can ring today.
func (s *Store) BadgesFor(ctx context.Context, ids []int64) (map[int64][]string, error) {
	out := map[int64][]string{}
	if len(ids) == 0 {
		return out, nil
	}
	q, args, err := sqlx.In(`SELECT DISTINCT ci.product_id FROM combo_items ci JOIN combos c ON c.id = ci.combo_id
		WHERE `+inWindow+` AND ci.product_id IN (?)`, ids)
	if err != nil {
		return nil, err
	}
	var pids []int64
	if err := s.db.SelectContext(ctx, &pids, s.db.Rebind(q), args...); err != nil {
		return nil, err
	}
	for _, id := range pids {
		out[id] = []string{"🎁 Combo"}
	}
	return out, nil
}

// Sold is one combo's share of a sale, as the till worked it out.
type Sold struct {
	ComboID  int64
	Sets     decimal.Decimal
	Net      decimal.Decimal
	Discount decimal.Decimal
}

// RecordSold stores a sale's combos (a resend replaces, never doubles).
func (s *Store) RecordSold(ctx context.Context, saleID int64, rows []Sold) error {
	var one int
	if err := s.db.GetContext(ctx, &one, `SELECT 1 FROM sales WHERE id = $1`, saleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound("sale")
		}
		return err
	}
	return s.withTx(ctx, func(tx *sqlx.Tx) error {
		for _, r := range rows {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO combo_sales (combo_id, name, sale_id, sets, net, discount)
				SELECT id, name, $2, $3, $4, $5 FROM combos WHERE id = $1
				ON CONFLICT (sale_id, combo_id) DO UPDATE SET sets = EXCLUDED.sets, net = EXCLUDED.net, discount = EXCLUDED.discount`,
				r.ComboID, saleID, r.Sets, r.Net, r.Discount); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) withTx(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
