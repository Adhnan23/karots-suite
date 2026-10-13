package alternatives

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"karots-suite/internal/apperr"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

type Store struct{ db *sqlx.DB }

func NewStore(db *sqlx.DB) *Store { return &Store{db: db} }

// Group is one set of interchangeable products (e.g. "USB Flash Drive 32GB").
type Group struct {
	ID        int64  `db:"id"`
	Name      string `db:"name"`
	SortOrder int    `db:"sort_order"`
	IsActive  bool   `db:"is_active"`
	Products  int    `db:"products"` // active members
	Tiers     []Tier
}

// Tier is a quality level inside a group, best first by sort_order. A blank
// name is the tier of a group that has no named tiers. Total is the summed
// stock of its active members; Low = tracked (level > 0) and total ≤ level.
type Tier struct {
	ID           int64           `db:"id"`
	GroupID      int64           `db:"group_id"`
	Name         string          `db:"name"`
	ReorderLevel int             `db:"reorder_level"`
	SortOrder    int             `db:"sort_order"`
	IsActive     bool            `db:"is_active"`
	Total        decimal.Decimal `db:"total"`
	Low          bool            `db:"low"`
	Members      []Member
}

// Member is a product in a tier. Active is false for a disabled (or deleted)
// product: it shows greyed and never counts.
type Member struct {
	ProductID int64           `db:"product_id"`
	TierID    int64           `db:"tier_id"`
	Name      string          `db:"name"`
	Stock     decimal.Decimal `db:"stock"`
	Active    bool            `db:"active"`
	Unit      string          `db:"unit"` // abbreviation
}

const tierCols = `t.id, t.group_id, t.name, t.reorder_level, t.sort_order, t.is_active,
	COALESCE((SELECT SUM(st.quantity) FROM alt_members m JOIN products p ON p.id = m.product_id AND p.is_active
	          LEFT JOIN stock st ON st.product_id = m.product_id WHERE m.tier_id = t.id), 0) AS total`

func notFound(err error, what string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return apperr.NotFound(what)
	}
	return err
}

// Groups lists groups (on only, unless disabled) with their tiers in order.
func (s *Store) Groups(ctx context.Context, disabled bool) ([]Group, error) {
	var gs []Group
	if err := s.db.SelectContext(ctx, &gs, `
		SELECT g.id, g.name, g.sort_order, g.is_active,
		       (SELECT COUNT(*) FROM alt_members m JOIN alt_tiers t ON t.id = m.tier_id
		          JOIN products p ON p.id = m.product_id AND p.is_active WHERE t.group_id = g.id) AS products
		FROM alt_groups g WHERE ($1 OR g.is_active) ORDER BY g.sort_order, lower(g.name), g.id`, disabled); err != nil {
		return nil, err
	}
	var ts []Tier
	if err := s.db.SelectContext(ctx, &ts, `SELECT `+tierCols+` FROM alt_tiers t ORDER BY t.sort_order, t.id`); err != nil {
		return nil, err
	}
	at := map[int64]int{}
	for i := range gs {
		at[gs[i].ID] = i
		gs[i].Tiers = []Tier{}
	}
	for _, t := range ts {
		if i, ok := at[t.GroupID]; ok {
			gs[i].Tiers = append(gs[i].Tiers, low(t))
		}
	}
	return gs, nil
}

func low(t Tier) Tier {
	t.Low = t.ReorderLevel > 0 && t.Total.LessThanOrEqual(decimal.NewFromInt(int64(t.ReorderLevel)))
	return t
}

// Group is one group with its tiers and every member (inactive ones too).
func (s *Store) Group(ctx context.Context, id int64) (Group, error) {
	var g Group
	if err := s.db.GetContext(ctx, &g, `SELECT id, name, sort_order, is_active, 0 AS products FROM alt_groups WHERE id = $1`, id); err != nil {
		return g, notFound(err, "group")
	}
	var ts []Tier
	if err := s.db.SelectContext(ctx, &ts, `SELECT `+tierCols+` FROM alt_tiers t WHERE t.group_id = $1 ORDER BY t.sort_order, t.id`, id); err != nil {
		return g, err
	}
	var ms []Member
	if err := s.db.SelectContext(ctx, &ms, `
		SELECT m.product_id, m.tier_id, COALESCE(p.name, 'Deleted product #' || m.product_id) AS name,
		       COALESCE(st.quantity, 0) AS stock, COALESCE(p.is_active, false) AS active, COALESCE(u.abbreviation, '') AS unit
		FROM alt_members m JOIN alt_tiers t ON t.id = m.tier_id
		LEFT JOIN products p ON p.id = m.product_id LEFT JOIN stock st ON st.product_id = m.product_id
		LEFT JOIN units u ON u.id = p.unit_id
		WHERE t.group_id = $1 ORDER BY COALESCE(p.is_active, false) DESC, lower(COALESCE(p.name, '')), m.product_id`, id); err != nil {
		return g, err
	}
	g.Tiers = []Tier{}
	for _, t := range ts {
		t = low(t)
		t.Members = []Member{}
		for _, m := range ms {
			if m.TierID == t.ID {
				t.Members = append(t.Members, m)
				if m.Active {
					g.Products++
				}
			}
		}
		g.Tiers = append(g.Tiers, t)
	}
	return g, nil
}

func cleanName(n, what string) (string, error) {
	n = strings.TrimSpace(n)
	if n == "" {
		return "", apperr.Validation(what + " name is required")
	}
	return n, nil
}

// CreateGroup adds a group with its first (blank-named) tier: a group starts
// as a plain list of products.
func (s *Store) CreateGroup(ctx context.Context, name string) (int64, error) {
	name, err := cleanName(name, "group")
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.db.GetContext(ctx, &id, `
		WITH g AS (INSERT INTO alt_groups (name, sort_order) VALUES ($1, 0) RETURNING id),
		     t AS (INSERT INTO alt_tiers (group_id, name) SELECT id, '' FROM g)
		SELECT id FROM g`, name)
	return id, err
}

func (s *Store) exec(ctx context.Context, what, q string, args ...any) error {
	r, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return apperr.NotFound(what)
	}
	return nil
}

func (s *Store) RenameGroup(ctx context.Context, id int64, name string) error {
	name, err := cleanName(name, "group")
	if err != nil {
		return err
	}
	return s.exec(ctx, "group", `UPDATE alt_groups SET name = $2 WHERE id = $1`, id, name)
}

func (s *Store) SetGroupActive(ctx context.Context, id int64, on bool) error {
	return s.exec(ctx, "group", `UPDATE alt_groups SET is_active = $2 WHERE id = $1`, id, on)
}

// DeleteGroup removes the group, its tiers and memberships (the products stay).
func (s *Store) DeleteGroup(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	// alt_members cascades from alt_tiers, alt_tiers from alt_groups.
	r, err := tx.ExecContext(ctx, `DELETE FROM alt_groups WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return apperr.NotFound("group")
	}
	return tx.Commit()
}

// TierInput is a tier's editable fields. A blank name is allowed only while it
// is the group's one tier (a group without named tiers).
type TierInput struct {
	Name         string
	ReorderLevel int
}

func (s *Store) tierCount(ctx context.Context, groupID int64) (int, error) {
	var n int
	err := s.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM alt_tiers WHERE group_id = $1`, groupID)
	return n, err
}

// CreateTier adds a tier at the bottom (worst) of the group. The group's blank
// tier must be named first, so tiers never read as two nameless boxes.
func (s *Store) CreateTier(ctx context.Context, groupID int64, in TierInput) (int64, error) {
	name, err := cleanName(in.Name, "tier")
	if err != nil {
		return 0, err
	}
	if in.ReorderLevel < 0 {
		return 0, apperr.Validation("reorder level can't be negative")
	}
	var blank bool
	if err := s.db.GetContext(ctx, &blank, `SELECT EXISTS (SELECT 1 FROM alt_tiers WHERE group_id = $1 AND name = '')`, groupID); err != nil {
		return 0, err
	}
	if blank {
		return 0, apperr.Validation("name this group's first tier before adding another")
	}
	var id int64
	err = s.db.GetContext(ctx, &id, `
		INSERT INTO alt_tiers (group_id, name, reorder_level, sort_order)
		SELECT g.id, $2, $3, COALESCE((SELECT MAX(sort_order) + 1 FROM alt_tiers WHERE group_id = g.id), 0)
		FROM alt_groups g WHERE g.id = $1 RETURNING id`, groupID, name, in.ReorderLevel)
	return id, notFound(err, "group")
}

func (s *Store) tier(ctx context.Context, id int64) (Tier, error) {
	var t Tier
	err := s.db.GetContext(ctx, &t, `SELECT `+tierCols+` FROM alt_tiers t WHERE t.id = $1`, id)
	return t, notFound(err, "tier")
}

func (s *Store) UpdateTier(ctx context.Context, id int64, in TierInput) error {
	t, err := s.tier(ctx, id)
	if err != nil {
		return err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.ReorderLevel < 0 {
		return apperr.Validation("reorder level can't be negative")
	}
	if in.Name == "" {
		if n, err := s.tierCount(ctx, t.GroupID); err != nil {
			return err
		} else if n > 1 {
			return apperr.Validation("tier name is required")
		}
	}
	return s.exec(ctx, "tier", `UPDATE alt_tiers SET name = $2, reorder_level = $3 WHERE id = $1`, id, in.Name, in.ReorderLevel)
}

func (s *Store) SetTierActive(ctx context.Context, id int64, on bool) error {
	return s.exec(ctx, "tier", `UPDATE alt_tiers SET is_active = $2 WHERE id = $1`, id, on)
}

// MoveTier swaps a tier with its neighbour (up = towards best). Sort orders are
// renumbered first, so old rows that all share 0 still move.
func (s *Store) MoveTier(ctx context.Context, id int64, up bool) error {
	t, err := s.tier(ctx, id)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var ids []int64
	if err := tx.SelectContext(ctx, &ids, `SELECT id FROM alt_tiers WHERE group_id = $1 ORDER BY sort_order, id`, t.GroupID); err != nil {
		return err
	}
	for i, x := range ids {
		j := i - 1
		if !up {
			j = i + 1
		}
		if x == id && j >= 0 && j < len(ids) {
			ids[i], ids[j] = ids[j], ids[i]
			break
		}
	}
	for i, x := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE alt_tiers SET sort_order = $2 WHERE id = $1`, x, i); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteTier removes an empty tier; never the group's last one.
func (s *Store) DeleteTier(ctx context.Context, id int64) error {
	t, err := s.tier(ctx, id)
	if err != nil {
		return err
	}
	var members int
	if err := s.db.GetContext(ctx, &members, `SELECT COUNT(*) FROM alt_members WHERE tier_id = $1`, id); err != nil {
		return err
	}
	if members > 0 {
		return apperr.Conflict("move or remove this tier's products first")
	}
	if n, err := s.tierCount(ctx, t.GroupID); err != nil {
		return err
	} else if n == 1 {
		return apperr.Conflict("a group keeps at least one tier")
	}
	return s.exec(ctx, "tier", `DELETE FROM alt_tiers WHERE id = $1`, id)
}
