package alternatives

import (
	"context"
	"fmt"
	"strings"

	"karots-suite/internal/apperr"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// FindProduct finds the product a sheet line names (core's products.FindForMenu:
// barcode, then exact name). A non-empty reason skips the line.
type FindProduct func(ctx context.Context, barcode, name string) (id int64, reason string, err error)

// Skipped is a line left out, with why.
type Skipped struct {
	Line   int
	Reason string
}

// SheetResult is what a sheet did, or would do in a preview.
type SheetResult struct {
	Groups, Members            int // in the new set
	GroupsAdded, GroupsRemoved int
	Placed, Moved, TakenOut    int // members new to Alternatives / in another tier now / gone
	Removed                    []string
	Skipped                    []Skipped
}

// sheetKey is how names match: case and spacing ignored (ALT-15).
func sheetKey(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

type sheetTier struct {
	key, name string
	members   []int64
}

type sheetGroup struct {
	key, name string
	tiers     []*sheetTier
}

// oldSet is the groups before the import: by name key (the first of a name wins).
type oldSet struct {
	groupID   map[string]int64
	groupName map[int64]string
	groups    []int64                    // in page order
	tierID    map[int64]map[string]int64 // group → tier key → tier
	tiers     map[int64][]int64          // group → its tiers
	tierOf    map[int64]int64            // product → tier
}

func loadOld(ctx context.Context, tx *sqlx.Tx) (oldSet, error) {
	o := oldSet{groupID: map[string]int64{}, groupName: map[int64]string{}, tierID: map[int64]map[string]int64{},
		tiers: map[int64][]int64{}, tierOf: map[int64]int64{}}
	var gs []struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}
	if err := tx.SelectContext(ctx, &gs, `SELECT id, name FROM alt_groups ORDER BY sort_order, lower(name), id`); err != nil {
		return o, err
	}
	for _, g := range gs {
		o.groups = append(o.groups, g.ID)
		o.groupName[g.ID] = g.Name
		o.tierID[g.ID] = map[string]int64{}
		if _, dup := o.groupID[sheetKey(g.Name)]; !dup {
			o.groupID[sheetKey(g.Name)] = g.ID
		}
	}
	var ts []struct {
		ID    int64  `db:"id"`
		Group int64  `db:"group_id"`
		Name  string `db:"name"`
	}
	if err := tx.SelectContext(ctx, &ts, `SELECT id, group_id, name FROM alt_tiers ORDER BY sort_order, id`); err != nil {
		return o, err
	}
	for _, t := range ts {
		o.tiers[t.Group] = append(o.tiers[t.Group], t.ID)
		if _, dup := o.tierID[t.Group][sheetKey(t.Name)]; !dup {
			o.tierID[t.Group][sheetKey(t.Name)] = t.ID
		}
	}
	var ms []struct {
		P int64 `db:"product_id"`
		T int64 `db:"tier_id"`
	}
	if err := tx.SelectContext(ctx, &ms, `SELECT product_id, tier_id FROM alt_members`); err != nil {
		return o, err
	}
	for _, m := range ms {
		o.tierOf[m.P] = m.T
	}
	return o, nil
}

// matchTier is the existing tier a sheet group + tier name lands in, 0 when new.
func (o oldSet) matchTier(groupKey, tierKey string) int64 {
	if g, ok := o.groupID[groupKey]; ok {
		return o.tierID[g][tierKey]
	}
	return 0
}

// read walks the lines in order into groups → tiers → products (ALT-13).
func read(ctx context.Context, tx *sqlx.Tx, lines []SheetLine, find FindProduct, o oldSet, res *SheetResult) ([]*sheetGroup, error) {
	type place struct {
		group, groupKey, tierKey string
		line                     int
	}
	placed := map[int64]place{}
	byKey := map[string]*sheetGroup{}
	var groups []*sheetGroup
	skip := func(l SheetLine, why string) { res.Skipped = append(res.Skipped, Skipped{l.Line, why}) }
	for _, l := range lines {
		gk, tk := sheetKey(l.Group), sheetKey(l.Tier)
		if gk == "" {
			skip(l, "no group on this line")
			continue
		}
		pid, reason, err := find(ctx, l.Barcode, l.Product)
		if err != nil {
			return nil, err
		}
		if reason != "" {
			skip(l, reason)
			continue
		}
		var p struct {
			Active  bool `db:"is_active"`
			Service bool `db:"is_service"`
		}
		if err := tx.GetContext(ctx, &p, `SELECT is_active, is_service FROM products WHERE id = $1`, pid); err != nil {
			return nil, err
		}
		if p.Service {
			skip(l, "a service has no stock to swap")
			continue
		}
		if was, ok := placed[pid]; ok {
			if was.groupKey != gk || was.tierKey != tk {
				skip(l, fmt.Sprintf("already in %s on line %d", was.group, was.line))
			}
			continue
		}
		if !p.Active && (o.tierOf[pid] == 0 || o.tierOf[pid] != o.matchTier(gk, tk)) {
			skip(l, "a disabled product can only stay where it already is")
			continue
		}
		g := byKey[gk]
		if g == nil {
			g = &sheetGroup{key: gk, name: strings.TrimSpace(l.Group)}
			byKey[gk] = g
			groups = append(groups, g)
		}
		var t *sheetTier
		for _, x := range g.tiers {
			if x.key == tk {
				t = x
			}
		}
		if t == nil {
			t = &sheetTier{key: tk, name: strings.TrimSpace(l.Tier)}
			g.tiers = append(g.tiers, t)
		}
		t.members = append(t.members, pid)
		placed[pid] = place{g.name, gk, tk, l.Line}
	}
	return groups, nil
}

// Rebuild makes the groups exactly what the sheet says (ALT-10), keeping what a
// sheet can't carry for the groups, tiers and members it still names (ALT-15). Run
// it in the caller's transaction: a preview rolls it back (ALT-11), a failure
// leaves everything as it was (ALT-12).
func Rebuild(ctx context.Context, tx *sqlx.Tx, lines []SheetLine, find FindProduct) (SheetResult, error) {
	var res SheetResult
	if len(lines) == 0 {
		return res, apperr.BadRequest("the sheet has no product lines")
	}
	// Two imports at once would each keep what they saw: the second waits here (ALT-12).
	if _, err := tx.ExecContext(ctx, `LOCK TABLE alt_groups IN EXCLUSIVE MODE`); err != nil {
		return res, err
	}
	o, err := loadOld(ctx, tx)
	if err != nil {
		return res, err
	}
	groups, err := read(ctx, tx, lines, find, o, &res)
	if err != nil {
		return res, err
	}
	if len(groups) == 0 { // a sheet from another shop must not wipe this one's groups
		return res, apperr.BadRequest("no line of the sheet matches a product here")
	}
	keptGroups, keptTiers := map[int64]bool{}, map[int64]bool{}
	want := map[int64]int64{} // product → tier
	for i, g := range groups {
		gid, kept := o.groupID[g.key]
		if kept {
			_, err = tx.ExecContext(ctx, `UPDATE alt_groups SET name = $2, sort_order = $3 WHERE id = $1`, gid, g.name, i)
		} else {
			err = tx.GetContext(ctx, &gid, `INSERT INTO alt_groups (name, sort_order) VALUES ($1, $2) RETURNING id`, g.name, i)
			res.GroupsAdded++
		}
		if err != nil {
			return res, err
		}
		keptGroups[gid] = true
		for j, t := range g.tiers {
			tid := o.tierID[gid][t.key]
			if kept && tid != 0 {
				_, err = tx.ExecContext(ctx, `UPDATE alt_tiers SET name = $2, sort_order = $3 WHERE id = $1`, tid, t.name, j)
			} else {
				err = tx.GetContext(ctx, &tid, `INSERT INTO alt_tiers (group_id, name, sort_order, level_auto, split_auto)
					VALUES ($1, $2, $3, true, true) RETURNING id`, gid, t.name, j)
			}
			if err != nil {
				return res, err
			}
			keptTiers[tid] = true
			for _, pid := range t.members {
				want[pid] = tid
			}
			res.Members += len(t.members)
		}
	}
	res.Groups = len(groups)
	var dropGroups, dropTiers []int64
	for _, gid := range o.groups {
		if !keptGroups[gid] {
			dropGroups = append(dropGroups, gid)
			res.Removed = append(res.Removed, o.groupName[gid])
			continue
		}
		for _, tid := range o.tiers[gid] {
			if !keptTiers[tid] {
				dropTiers = append(dropTiers, tid)
			}
		}
	}
	res.GroupsRemoved = len(dropGroups)
	touched := map[int64]bool{} // kept tiers whose members changed (ALT-5)
	var out []int64
	for pid, tid := range o.tierOf {
		if _, stays := want[pid]; !stays {
			out = append(out, pid)
			touched[tid] = true
		}
	}
	res.TakenOut = len(out)
	for _, q := range []struct {
		sql string
		ids []int64
	}{
		{`DELETE FROM alt_groups WHERE id = ANY($1)`, dropGroups},
		{`DELETE FROM alt_tiers WHERE id = ANY($1)`, dropTiers},
		{`DELETE FROM alt_members WHERE product_id = ANY($1)`, out},
	} {
		if _, err := tx.ExecContext(ctx, q.sql, pq.Array(q.ids)); err != nil {
			return res, err
		}
	}
	for pid, tid := range want {
		old, had := o.tierOf[pid]
		if had && old == tid {
			continue
		}
		if had {
			res.Moved++
			touched[old] = true
		} else {
			res.Placed++
		}
		touched[tid] = true
		if _, err := tx.ExecContext(ctx, `INSERT INTO alt_members (product_id, tier_id) VALUES ($1, $2)
			ON CONFLICT (product_id) DO UPDATE SET tier_id = EXCLUDED.tier_id, split_share = NULL`, pid, tid); err != nil {
			return res, err
		}
	}
	for tid := range touched {
		if keptTiers[tid] {
			if err := rescaleSplit(ctx, tx, tid); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}
