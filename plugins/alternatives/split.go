package alternatives

import (
	"context"
	"sort"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/reports"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

const (
	salesDays        = 90 // what "recent sales" means, as on core's Reorder page
	fasterMinSold    = 10 // too few sales say nothing
	fasterMinPoints  = 20 // a brand's sales share above its manual share, in points
	splitTotalShares = 100
)

// splitOrder divides total by weight with the largest-remainder method, so the parts
// always add up to total: extra units go to the biggest weight, ties by position. All
// weights zero splits evenly.
func splitOrder(total int, weights []int64) []int {
	out := make([]int, len(weights))
	if total <= 0 || len(weights) == 0 {
		return out
	}
	var sum int64
	for _, w := range weights {
		sum += max(w, 0)
	}
	if sum == 0 {
		weights = make([]int64, len(weights))
		for i := range weights {
			weights[i] = 1
		}
		sum = int64(len(weights))
	}
	type rem struct {
		at     int
		rem, w int64
	}
	rems := make([]rem, len(weights))
	left := total
	for i, w := range weights {
		w = max(w, 0)
		out[i] = int(int64(total) * w / sum)
		left -= out[i]
		rems[i] = rem{i, int64(total) * w % sum, w}
	}
	sort.SliceStable(rems, func(a, b int) bool {
		if rems[a].rem != rems[b].rem {
			return rems[a].rem > rems[b].rem
		}
		return rems[a].w > rems[b].w
	})
	for i := 0; i < left; i++ {
		out[rems[i%len(rems)].at]++
	}
	return out
}

// fasterBrand finds a brand selling well above the share it was given by hand.
func fasterBrand(manual map[int64]int, sold map[int64]int64) (int64, bool) {
	var total int64
	ids := make([]int64, 0, len(sold))
	for id, n := range sold {
		total += n
		ids = append(ids, id)
	}
	if total < fasterMinSold {
		return 0, false
	}
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
	var best int64
	bestBy := 0
	for _, id := range ids {
		by := int(sold[id]*splitTotalShares/total) - manual[id]
		if by >= fasterMinPoints && by > bestBy {
			best, bestBy = id, by
		}
	}
	return best, best != 0
}

// soldLately is each product's whole units sold over the recent window.
func (s *Store) soldLately(ctx context.Context, ids []int64) (map[int64]int64, error) {
	now := time.Now()
	m, err := reports.NewService(s.db).ProductQtySold(ctx, ids, now.AddDate(0, 0, -salesDays), now)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]int64, len(m))
	for id, q := range m {
		out[id] = q.Ceil().IntPart()
	}
	return out, nil
}

// splitNeeds shares each low tier's shortfall among its live members.
func (s *Store) splitNeeds(ctx context.Context, short map[int64]int) (map[int64]decimal.Decimal, error) {
	out := map[int64]decimal.Decimal{}
	if len(short) == 0 {
		return out, nil
	}
	tierIDs := make([]int64, 0, len(short))
	for id := range short {
		tierIDs = append(tierIDs, id)
	}
	var rows []struct {
		TierID    int64 `db:"tier_id"`
		ProductID int64 `db:"product_id"`
		Share     int64 `db:"share"`
		Auto      bool  `db:"split_auto"`
	}
	if err := s.db.SelectContext(ctx, &rows, `SELECT m.tier_id, m.product_id, COALESCE(m.split_share, 0) AS share, t.split_auto
		FROM `+liveMembers+` WHERE m.tier_id = ANY($1) ORDER BY m.product_id`, pq.Array(tierIDs)); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ProductID)
	}
	sold, err := s.soldLately(ctx, ids)
	if err != nil {
		return nil, err
	}
	members, weights := map[int64][]int64{}, map[int64][]int64{}
	for _, r := range rows {
		w := r.Share
		if r.Auto {
			w = sold[r.ProductID]
		}
		members[r.TierID] = append(members[r.TierID], r.ProductID)
		weights[r.TierID] = append(weights[r.TierID], w)
	}
	for tier, list := range members {
		for i, n := range splitOrder(short[tier], weights[tier]) {
			out[list[i]] = decimal.NewFromInt(int64(n))
		}
	}
	return out, nil
}

// SetSplit chooses how a tier's order divides: by what sells, or by the shop's shares
// (whole percents of its live members, adding up to 100).
func (s *Store) SetSplit(ctx context.Context, tierID int64, auto bool, shares map[int64]int) error {
	if _, err := s.tier(ctx, tierID); err != nil {
		return err
	}
	if !auto {
		sum := 0
		for _, n := range shares {
			if n < 0 || n > splitTotalShares {
				return apperr.Validation("a share is between 0 and 100")
			}
			sum += n
		}
		if sum != splitTotalShares {
			return apperr.Validation("the shares must add up to 100")
		}
		var in []int64
		if err := s.db.SelectContext(ctx, &in, `SELECT product_id FROM alt_members WHERE tier_id = $1`, tierID); err != nil {
			return err
		}
		member := map[int64]bool{}
		for _, id := range in {
			member[id] = true
		}
		for id := range shares {
			if !member[id] {
				return apperr.Validation("that product isn't in this tier")
			}
		}
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, `UPDATE alt_tiers SET split_auto = $2 WHERE id = $1`, tierID, auto); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE alt_members SET split_share = NULL WHERE tier_id = $1`, tierID); err != nil {
		return err
	}
	if !auto {
		for id, n := range shares {
			if _, err := tx.ExecContext(ctx, `UPDATE alt_members SET split_share = $3 WHERE tier_id = $1 AND product_id = $2`, tierID, id, n); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// SplitSuggest is the offer on a tier split by hand when another brand sells faster:
// nothing changes until it's used.
type SplitSuggest struct {
	ProductID int64
	Name      string
	Shares    map[int64]int
}

// suggestFor reads a tier's sales for its offer; nil = none.
func (s *Store) suggestFor(ctx context.Context, t Tier) (*SplitSuggest, error) {
	if t.SplitAuto || len(t.Members) == 0 {
		return nil, nil
	}
	ids, manual, names := []int64{}, map[int64]int{}, map[int64]string{}
	for _, m := range t.Members {
		if !m.Active {
			continue
		}
		ids = append(ids, m.ProductID)
		names[m.ProductID] = m.Name
		if m.SplitShare != nil {
			manual[m.ProductID] = *m.SplitShare
		}
	}
	sold, err := s.soldLately(ctx, ids)
	if err != nil {
		return nil, err
	}
	id, ok := fasterBrand(manual, sold)
	if !ok {
		return nil, nil
	}
	weights := make([]int64, len(ids))
	for i, x := range ids {
		weights[i] = sold[x]
	}
	shares := map[int64]int{}
	for i, n := range splitOrder(splitTotalShares, weights) {
		shares[ids[i]] = n
	}
	return &SplitSuggest{ProductID: id, Name: names[id], Shares: shares}, nil
}
