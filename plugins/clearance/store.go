package clearance

import (
	"context"
	"time"

	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// Store is clearance's data. Its tables are karots-pos's, unchanged, so a
// karots-pos backup restores as is: "ended" and "snoozed" are worked out from
// clearance_markdowns.updated_at, which only Approve and Snooze write.
type Store struct{ db *sqlx.DB }

func NewStore(db *sqlx.DB) *Store { return &Store{db: db} }

type Settings struct {
	StaleDays        int             `db:"stale_days"`
	DefaultPercent   decimal.Decimal `db:"default_percent"`
	MinMarginPercent decimal.Decimal `db:"min_margin_percent"`
	ExpiryDays       int             `db:"expiry_days"` // flag lots expiring within this window
	DeadMonths       int             `db:"dead_months"` // flag 0-stock products idle longer than this
}

func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	var out Settings
	err := s.db.GetContext(ctx, &out,
		`SELECT stale_days, default_percent, min_margin_percent, expiry_days, dead_months FROM clearance_settings WHERE id = 1`)
	return out, err
}

func (s *Store) SaveSettings(ctx context.Context, in Settings) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE clearance_settings SET stale_days=$1, default_percent=$2, min_margin_percent=$3, expiry_days=$4, dead_months=$5 WHERE id = 1`,
		in.StaleDays, in.DefaultPercent, in.MinMarginPercent, in.ExpiryDays, in.DeadMonths)
	return err
}

// suggestPercent returns the discount % to suggest: defaultPct, but never so
// much that the new price falls below cost*(1+minMargin/100). Returns 0 when the
// price is already at or under that floor (nothing safe to give).
func suggestPercent(sell, cost, defaultPct, minMarginPct decimal.Decimal) decimal.Decimal {
	hundred := decimal.NewFromInt(100)
	if !sell.IsPositive() {
		return decimal.Zero
	}
	floor := cost.Mul(hundred.Add(minMarginPct)).Div(hundred) // cost*(1+m/100)
	// max % that still keeps price >= floor: (1 - floor/sell) * 100
	maxPct := hundred.Sub(floor.Div(sell).Mul(hundred))
	if maxPct.IsNegative() {
		maxPct = decimal.Zero
	}
	if defaultPct.LessThan(maxPct) {
		return defaultPct
	}
	return maxPct
}

// newPrice applies a percent off a selling price, rounded to 2dp.
func newPrice(sell, pct decimal.Decimal) decimal.Decimal {
	hundred := decimal.NewFromInt(100)
	return sell.Mul(hundred.Sub(pct)).Div(hundred).Round(2)
}

// liveSQL is true for a markdown row m that still applies: approved, in stock,
// and no lot arrived after it was approved (fresh stock sells at full price).
const liveSQL = `m.status = 'approved' AND m.discount_value > 0 AND COALESCE(st.quantity, 0) > 0
	AND NOT EXISTS (SELECT 1 FROM stock_batches nb WHERE nb.product_id = m.product_id AND nb.created_at > m.updated_at)`

// Candidate is one row of the "to mark down" list.
type Candidate struct {
	ProductID     int64            `db:"product_id"`
	Name          string           `db:"name"`
	Unit          string           `db:"unit_abbr"`
	OnHand        decimal.Decimal  `db:"stock_qty"`
	Cost          decimal.Decimal  `db:"cost_price"`
	Price         decimal.Decimal  `db:"selling_price"`
	DaysSinceSale *int             `db:"days_since_sale"` // nil = never sold
	SoonestExpiry *time.Time       `db:"soonest_expiry"`  // soonest FUTURE lot expiry, nil = none
	LotCost       *decimal.Decimal `db:"lot_cost"`        // that lot's cost
	Status        string           `db:"status"`          // "" candidate | "live" | "snoozed"
	Percent       *decimal.Decimal `db:"percent"`         // the live markdown
	SnoozedUntil  *time.Time       `db:"snoozed_until"`
}

// Candidates lists products with stock that haven't sold within stale_days or
// have a live lot expiring within expiry_days, plus every live markdown (it may
// be selling again because of the markdown). Snoozed ones are included with
// their status so the page can filter them; already-expired lots stay the
// Batches write-off's job, so only future-dated lots count.
func (s *Store) Candidates(ctx context.Context) ([]Candidate, error) {
	cfg, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	rows := []Candidate{}
	err = s.db.SelectContext(ctx, &rows, `
		WITH base AS (
		SELECT p.id AS product_id, p.name, u.abbreviation AS unit_abbr,
		       COALESCE(st.quantity, 0) AS stock_qty, p.cost_price, p.selling_price,
		       CASE WHEN ls.last_sold IS NULL THEN NULL
		            ELSE EXTRACT(DAY FROM now() - ls.last_sold)::int END AS days_since_sale,
		       be.expiry_date::timestamptz AS soonest_expiry, be.cost_price AS lot_cost,
		       CASE WHEN m.product_id IS NOT NULL AND `+liveSQL+` THEN 'live'
		            WHEN m.status = 'dismissed' AND m.updated_at + ($1 || ' days')::interval > now() THEN 'snoozed'
		            ELSE '' END AS status,
		       m.discount_value, m.updated_at AS marked_at,
		       (ls.last_sold IS NULL OR ls.last_sold < now() - ($1 || ' days')::interval) AS stale,
		       (be.expiry_date IS NOT NULL AND be.expiry_date <= CURRENT_DATE + $2::int) AS expiring
		FROM products p
		JOIN units u ON u.id = p.unit_id
		LEFT JOIN stock st ON st.product_id = p.id
		LEFT JOIN (
		    SELECT si.product_id, MAX(sa.created_at) AS last_sold
		    FROM sale_items si JOIN sales sa ON sa.id = si.sale_id
		    GROUP BY si.product_id
		) ls ON ls.product_id = p.id
		LEFT JOIN (
		    SELECT DISTINCT ON (product_id) product_id, expiry_date, cost_price
		    FROM stock_batches
		    WHERE qty_remaining > 0 AND expiry_date IS NOT NULL AND expiry_date >= CURRENT_DATE
		    ORDER BY product_id, expiry_date, id
		) be ON be.product_id = p.id
		LEFT JOIN clearance_markdowns m ON m.product_id = p.id
		WHERE p.is_active = true AND p.is_service = false AND COALESCE(st.quantity, 0) > 0
		)
		SELECT product_id, name, unit_abbr, stock_qty, cost_price, selling_price, days_since_sale,
		       soonest_expiry, lot_cost, status,
		       CASE WHEN status = 'live' THEN discount_value END AS percent,
		       CASE WHEN status = 'snoozed' THEN marked_at + ($1 || ' days')::interval END AS snoozed_until
		FROM base
		WHERE stale OR expiring OR status = 'live'
		ORDER BY soonest_expiry ASC NULLS LAST, days_since_sale DESC NULLS FIRST, name`,
		cfg.StaleDays, cfg.ExpiryDays)
	return rows, err
}

type DeadItem struct {
	ProductID    int64     `db:"product_id"`
	Name         string    `db:"name"`
	Unit         string    `db:"unit_abbr"`
	LastActivity time.Time `db:"last_activity"`
	MonthsIdle   int       `db:"months_idle"`
}

// Dead lists active, out-of-stock products whose latest activity (stock
// movement, sale, or creation) is older than dead_months — candidates to
// retire. Services are excluded. Nothing is disabled automatically.
func (s *Store) Dead(ctx context.Context) ([]DeadItem, error) {
	cfg, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	rows := []DeadItem{}
	err = s.db.SelectContext(ctx, &rows, `
		WITH last_activity AS (
			SELECT p.id AS product_id,
			       GREATEST(
			           p.created_at,
			           COALESCE((SELECT MAX(created_at) FROM stock_movements sm WHERE sm.product_id = p.id), p.created_at),
			           COALESCE((SELECT MAX(sa.created_at) FROM sale_items si JOIN sales sa ON sa.id = si.sale_id WHERE si.product_id = p.id), p.created_at)
			       ) AS last_at
			FROM products p
		)
		SELECT p.id AS product_id, p.name, u.abbreviation AS unit_abbr,
		       la.last_at AS last_activity,
		       (EXTRACT(EPOCH FROM now() - la.last_at) / 2592000)::int AS months_idle
		FROM products p
		JOIN units u ON u.id = p.unit_id
		JOIN last_activity la ON la.product_id = p.id
		LEFT JOIN stock st ON st.product_id = p.id
		WHERE p.is_active = true AND p.is_service = false
		  AND COALESCE(st.quantity, 0) = 0
		  AND la.last_at < now() - ($1 || ' months')::interval
		ORDER BY la.last_at ASC, p.name`,
		cfg.DeadMonths)
	return rows, err
}

// Approve sets a live percent markdown, re-stamping updated_at: it applies to
// the stock on hand now, until that sells out or fresh stock arrives.
func (s *Store) Approve(ctx context.Context, productID int64, pct decimal.Decimal, userID int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO clearance_markdowns (product_id, discount_type, discount_value, status, approved_by, updated_at)
		VALUES ($1,'percent',$2,'approved',$3, now())
		ON CONFLICT (product_id) DO UPDATE
		SET discount_type='percent', discount_value=$2, status='approved', approved_by=$3, updated_at=now()`,
		productID, pct, userID)
	return err
}

// Snooze hides a product for one stale period (karots-pos "dismiss" hid it forever).
func (s *Store) Snooze(ctx context.Context, productID, userID int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO clearance_markdowns (product_id, discount_type, discount_value, status, approved_by, updated_at)
		VALUES ($1,'percent',0,'dismissed',$2, now())
		ON CONFLICT (product_id) DO UPDATE
		SET status='dismissed', discount_value=0, approved_by=$2, updated_at=now()`,
		productID, userID)
	return err
}

// Unsnooze brings a snoozed product straight back.
func (s *Store) Unsnooze(ctx context.Context, productID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM clearance_markdowns WHERE product_id = $1 AND status = 'dismissed'`, productID)
	return err
}

type markdown struct {
	Type  string
	Value decimal.Decimal
}

func (m markdown) label() string {
	if m.Type == "percent" {
		return "Clearance -" + m.Value.String() + "%"
	}
	return "Clearance -" + m.Value.String()
}

// live returns the markdowns that still apply for the given products.
func (s *Store) live(ctx context.Context, ids []int64) (map[int64]markdown, error) {
	out := map[int64]markdown{}
	if len(ids) == 0 {
		return out, nil
	}
	q, args, err := sqlx.In(`SELECT m.product_id, m.discount_type, m.discount_value
		FROM clearance_markdowns m LEFT JOIN stock st ON st.product_id = m.product_id
		WHERE m.product_id IN (?) AND `+liveSQL, ids)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryxContext(ctx, s.db.Rebind(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var m markdown
		if err := rows.Scan(&id, &m.Type, &m.Value); err != nil {
			return nil, err
		}
		out[id] = m
	}
	return out, rows.Err()
}

// BadgesFor pins "Clearance -N%" on products with a live markdown.
func (s *Store) BadgesFor(ctx context.Context, ids []int64) (map[int64][]string, error) {
	m, err := s.live(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := map[int64][]string{}
	for id, md := range m {
		out[id] = []string{md.label()}
	}
	return out, nil
}

// SuggestionsFor is the till's apply / skip prompt for products with a live markdown.
func (s *Store) SuggestionsFor(ctx context.Context, ids []int64) (map[int64]plugin.SaleSuggestion, error) {
	m, err := s.live(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := map[int64]plugin.SaleSuggestion{}
	for id, md := range m {
		out[id] = plugin.SaleSuggestion{DiscountType: md.Type, DiscountValue: md.Value.String(), Label: md.label(),
			Prompt: "Clearance item — apply the markdown?"}
	}
	return out, nil
}
