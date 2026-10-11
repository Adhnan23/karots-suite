package alternatives

import (
	"context"
	"database/sql"
	"errors"

	"karots-suite/internal/apperr"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// Membership is where a product sits.
type Membership struct {
	ProductID int64  `db:"product_id"`
	GroupID   int64  `db:"group_id"`
	TierID    int64  `db:"tier_id"`
	Group     string `db:"grp"`
	Tier      string `db:"tier"`
}

// Memberships maps each given product that is in a group to where it sits; no
// ids = every grouped product (the admin picker's hints).
func (s *Store) Memberships(ctx context.Context, ids []int64) (map[int64]Membership, error) {
	var rows []Membership
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT m.product_id, t.group_id, m.tier_id, g.name AS grp, t.name AS tier
		FROM alt_members m JOIN alt_tiers t ON t.id = m.tier_id JOIN alt_groups g ON g.id = t.group_id
		WHERE COALESCE(cardinality($1::bigint[]), 0) = 0 OR m.product_id = ANY($1)`, pq.Array(ids)); err != nil {
		return nil, err
	}
	out := make(map[int64]Membership, len(rows))
	for _, r := range rows {
		out[r.ProductID] = r
	}
	return out, nil
}

// AddMember puts a product in a tier. A product already in another tier is
// moved only when move is set; otherwise a 409 IN_OTHER_GROUP names its group.
func (s *Store) AddMember(ctx context.Context, tierID, productID int64, move bool) (moved bool, err error) {
	if _, err := s.tier(ctx, tierID); err != nil {
		return false, err
	}
	var p struct {
		Active  bool `db:"is_active"`
		Service bool `db:"is_service"`
	}
	if err := s.db.GetContext(ctx, &p, `SELECT is_active, is_service FROM products WHERE id = $1`, productID); err != nil {
		return false, notFound(err, "product")
	}
	if !p.Active {
		return false, apperr.Validation("that product is disabled")
	}
	if p.Service {
		return false, apperr.Validation("a service has no stock to swap")
	}
	cur, err := s.Memberships(ctx, []int64{productID})
	if err != nil {
		return false, err
	}
	if m, ok := cur[productID]; ok {
		if m.TierID == tierID {
			return false, nil
		}
		if !move {
			return false, apperr.ConflictCode("IN_OTHER_GROUP", "Already in "+m.Group)
		}
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO alt_members (product_id, tier_id) VALUES ($1, $2)
		ON CONFLICT (product_id) DO UPDATE SET tier_id = EXCLUDED.tier_id`, productID, tierID)
	_, was := cur[productID]
	return was, err
}

func (s *Store) RemoveMember(ctx context.Context, productID int64) error {
	return s.exec(ctx, "member", `DELETE FROM alt_members WHERE product_id = $1`, productID)
}

// liveMembers is the active members of on tiers of on groups — the only ones
// search, pins, swaps and reorder notes ever see.
const liveMembers = `alt_members m
	JOIN alt_tiers t  ON t.id = m.tier_id AND t.is_active
	JOIN alt_groups g ON g.id = t.group_id AND g.is_active
	JOIN products p   ON p.id = m.product_id AND p.is_active`

// MatchProductIDs returns every live member of each group the query touches by
// group name, tier name or a member's name: "32gb pendrive" finds the group,
// "Kingston" finds Kingston's siblings too.
func (s *Store) MatchProductIDs(ctx context.Context, q string) ([]int64, error) {
	var ids []int64
	err := s.db.SelectContext(ctx, &ids, `
		WITH hit AS (
			SELECT DISTINCT g.id FROM `+liveMembers+`
			WHERE g.name ILIKE '%' || $1 || '%' OR (t.name <> '' AND t.name ILIKE '%' || $1 || '%') OR p.name ILIKE '%' || $1 || '%')
		SELECT DISTINCT m.product_id FROM `+liveMembers+` JOIN hit h ON h.id = g.id`, q)
	return ids, err
}

// BadgesFor pins "⇄ Genuine" (the tier) or a bare "⇄" (no named tiers) on live members.
func (s *Store) BadgesFor(ctx context.Context, ids []int64) (map[int64][]string, error) {
	var rows []struct {
		ProductID int64  `db:"product_id"`
		Tier      string `db:"tier"`
	}
	if err := s.db.SelectContext(ctx, &rows, `SELECT m.product_id, t.name AS tier FROM `+liveMembers+` WHERE m.product_id = ANY($1)`, pq.Array(ids)); err != nil {
		return nil, err
	}
	out := make(map[int64][]string, len(rows))
	for _, r := range rows {
		b := "⇄"
		if r.Tier != "" {
			b += " " + r.Tier
		}
		out[r.ProductID] = []string{b}
	}
	return out, nil
}

// Alt is an in-stock alternative the till can offer.
type Alt struct {
	ProductID      int64           `db:"product_id"`
	Name           string          `db:"name"`
	Tier           string          `db:"tier"`
	Stock          decimal.Decimal `db:"stock"`
	SellingPrice   decimal.Decimal `db:"selling_price"`
	WholesalePrice decimal.Decimal `db:"wholesale_price"`
}

// Alternatives lists the product's in-stock live siblings, best tier first, then
// most stock. None when the product itself isn't a live member.
func (s *Store) Alternatives(ctx context.Context, productID int64, limit int) ([]Alt, error) {
	rows := []Alt{}
	err := s.db.SelectContext(ctx, &rows, `
		SELECT m.product_id, p.name, t.name AS tier, st.quantity AS stock, p.selling_price, p.wholesale_price
		FROM `+liveMembers+` JOIN stock st ON st.product_id = m.product_id
		WHERE g.id = (SELECT t2.group_id FROM `+liveMembersAs2+` WHERE m2.product_id = $1)
		  AND m.product_id <> $1 AND NOT p.is_service AND st.quantity > 0
		ORDER BY t.sort_order, t.id, st.quantity DESC, p.name
		LIMIT $2`, productID, limit)
	return rows, err
}

const liveMembersAs2 = `alt_members m2
	JOIN alt_tiers t2  ON t2.id = m2.tier_id AND t2.is_active
	JOIN alt_groups g2 ON g2.id = t2.group_id AND g2.is_active
	JOIN products p2   ON p2.id = m2.product_id AND p2.is_active`

// Coverage says whether a tier still covers a low product.
type Coverage struct {
	Group, Tier  string
	Total        decimal.Decimal
	ReorderLevel int
	Covered      bool // tracked and the tier's total is above its level
	Low          bool // tracked and the tier's total is at or under it
}

// CoverageFor computes each given live member's tier total against its level.
func (s *Store) CoverageFor(ctx context.Context, ids []int64) (map[int64]Coverage, error) {
	var rows []struct {
		ProductID int64  `db:"product_id"`
		Group     string `db:"grp"`
		Tier
	}
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT m.product_id, g.name AS grp, `+tierCols+`
		FROM `+liveMembers+` WHERE m.product_id = ANY($1)`, pq.Array(ids)); err != nil {
		return nil, err
	}
	out := make(map[int64]Coverage, len(rows))
	for _, r := range rows {
		t := low(r.Tier)
		out[r.ProductID] = Coverage{Group: r.Group, Tier: t.Name, Total: t.Total, ReorderLevel: t.ReorderLevel,
			Covered: t.ReorderLevel > 0 && !t.Low, Low: t.Low}
	}
	return out, nil
}

// LowTierMemberIDs is every live member of a tier that is low as a whole, to pull
// into the reorder list even when the member isn't low on its own.
func (s *Store) LowTierMemberIDs(ctx context.Context) ([]int64, error) {
	var ids []int64
	err := s.db.SelectContext(ctx, &ids, `
		SELECT m.product_id FROM `+liveMembers+`
		WHERE t.reorder_level > 0 AND t.reorder_level >= (SELECT COALESCE(SUM(st.quantity), 0) FROM `+liveMembersAs2+`
		       LEFT JOIN stock st ON st.product_id = m2.product_id WHERE m2.tier_id = t.id)`)
	return ids, err
}

// TierOf returns the product's tier id, 0 when it is in no group.
func (s *Store) TierOf(ctx context.Context, productID int64) (groupID, tierID int64, err error) {
	var m Membership
	err = s.db.GetContext(ctx, &m, `SELECT m.product_id, t.group_id, m.tier_id, '' AS grp, '' AS tier
		FROM alt_members m JOIN alt_tiers t ON t.id = m.tier_id WHERE m.product_id = $1`, productID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, nil
	}
	return m.GroupID, m.TierID, err
}
