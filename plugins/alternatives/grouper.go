package alternatives

import (
	"context"
	appdb "karots-suite/internal/db"

	"karots-suite/internal/plugin"
)

// grouper offers the shop's groups through the generic hook; every change goes
// through this store's own rules.
func (s *Store) grouper() plugin.ProductGrouper {
	return plugin.ProductGrouper{
		Groups: s.groupInfos,
		Create: s.createFilled,
		AddTo: func(ctx context.Context, tierID, productID int64) error {
			_, err := s.AddMember(ctx, tierID, productID, false)
			return err
		},
		Delete: s.DeleteGroup,
		Remove: s.RemoveMember,
	}
}

func (s *Store) groupInfos(ctx context.Context) ([]plugin.ProductGroupInfo, error) {
	groups, err := s.Groups(ctx, true)
	if err != nil {
		return nil, err
	}
	var ms []struct {
		GroupID   int64 `db:"group_id"`
		ProductID int64 `db:"product_id"`
	}
	if err := appdb.On(ctx, s.db).SelectContext(ctx, &ms, `
		SELECT t.group_id, m.product_id FROM alt_members m JOIN alt_tiers t ON t.id = m.tier_id
		LEFT JOIN products p ON p.id = m.product_id
		ORDER BY t.sort_order, t.id, COALESCE(p.is_active, false) DESC, lower(COALESCE(p.name, '')), m.product_id`); err != nil {
		return nil, err
	}
	members := map[int64][]int64{}
	for _, m := range ms {
		members[m.GroupID] = append(members[m.GroupID], m.ProductID)
	}
	out := make([]plugin.ProductGroupInfo, 0, len(groups))
	for _, g := range groups {
		info := plugin.ProductGroupInfo{ID: g.ID, Name: g.Name, Members: members[g.ID]}
		for _, t := range g.Tiers {
			info.Tiers = append(info.Tiers, plugin.ProductGroupTier{ID: t.ID, Name: t.Name})
		}
		out = append(out, info)
	}
	return out, nil
}

// createFilled makes a one-tier group of the given products, skipping any already
// in a group or that the store refuses. Fewer than two left means no group.
func (s *Store) createFilled(ctx context.Context, name string, productIDs []int64) (int64, error) {
	cur, err := s.Memberships(ctx, productIDs)
	if err != nil {
		return 0, err
	}
	free := make([]int64, 0, len(productIDs))
	for _, id := range productIDs {
		if _, grouped := cur[id]; !grouped {
			free = append(free, id)
		}
	}
	if len(free) < 2 {
		return 0, nil
	}
	id, err := s.CreateGroup(ctx, name)
	if err != nil {
		return 0, err
	}
	g, err := s.Group(ctx, id)
	if err != nil {
		return 0, err
	}
	added := 0
	for _, pid := range free {
		if _, err := s.AddMember(ctx, g.Tiers[0].ID, pid, false); err == nil {
			added++
		}
	}
	if added < 2 {
		return 0, s.DeleteGroup(ctx, id)
	}
	return id, nil
}
