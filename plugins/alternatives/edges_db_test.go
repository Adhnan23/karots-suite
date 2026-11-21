package alternatives

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"karots-suite/internal/plugin"
)

// The till's ⓘ rows and the Reorder note name a tier when it has a name, and only then.
func TestTierNamesShown(t *testing.T) {
	s, db, sfx := testStore(t)
	ctx := context.Background()
	p := &Plugin{store: s}
	a, b := product(t, db, "TA"+sfx, "5", false, true), product(t, db, "TB"+sfx, "5", false, true)
	plain := groupTier(t, s, "Names "+sfx, a, b)
	rows, err := p.detailRows(ctx, a)
	noErr(t, err)
	if len(rows) != 1 || rows[0].Label != "Alternative" {
		t.Errorf("an unnamed tier: %+v", rows)
	}
	noErr(t, s.UpdateTier(ctx, plain, TierInput{Name: "Generic"}))
	rows, err = p.detailRows(ctx, a)
	noErr(t, err)
	if len(rows) != 1 || rows[0].Label != "Alternative · Generic" {
		t.Errorf("a named tier: %+v", rows)
	}
	setLevel(t, s, a, 50)
	notes, err := p.reorderNotes(ctx, []int64{a})
	noErr(t, err)
	if !strings.Contains(notes[a].Note, "Names "+sfx+" · Generic has") {
		t.Errorf("reorder note %q", notes[a].Note)
	}
}

// The product form's tier pick: a missing tier is refused, a new one is saved, 0 leaves the group.
func TestProductFormTier(t *testing.T) {
	s, db, sfx := testStore(t)
	ctx := context.Background()
	p := &Plugin{store: s}
	a := product(t, db, "FA"+sfx, "1", false, true)
	tid := groupTier(t, s, "Form "+sfx)
	raw := func(id int64) json.RawMessage { b, _ := json.Marshal(map[string]int64{"tierId": id}); return b }
	pc := plugin.ExtraCtx{ProductID: a}
	if err := p.validateExtra(ctx, pc, raw(999999999)); err == nil {
		t.Errorf("a missing tier was accepted")
	}
	noErr(t, p.validateExtra(ctx, pc, raw(tid)))
	noErr(t, p.saveExtra(ctx, pc, raw(tid)))
	if _, cur, _ := s.TierOf(ctx, a); cur != tid {
		t.Errorf("saved into tier %d, want %d", cur, tid)
	}
	noErr(t, p.saveExtra(ctx, pc, raw(0)))
	if _, cur, _ := s.TierOf(ctx, a); cur != 0 {
		t.Errorf("still in tier %d", cur)
	}
}

// One brand may take the whole 100; the last tier moved down stays last.
func TestSplitEdgesAndLastTier(t *testing.T) {
	s, db, sfx := testStore(t)
	ctx := context.Background()
	a, b := product(t, db, "EA"+sfx, "0", false, true), product(t, db, "EB"+sfx, "0", false, true)
	tid := groupTier(t, s, "Edge "+sfx, a, b)
	noErr(t, s.SetSplit(ctx, tid, false, map[int64]int{a: 100, b: 0}))
	noErr(t, s.UpdateTier(ctx, tid, TierInput{Name: "First"}))
	tier, _ := s.tier(ctx, tid)
	second, err := s.CreateTier(ctx, tier.GroupID, TierInput{Name: "Last"})
	noErr(t, err)
	noErr(t, s.MoveTier(ctx, second, false))
	g, err := s.Group(ctx, tier.GroupID)
	noErr(t, err)
	if len(g.Tiers) != 2 || g.Tiers[1].ID != second {
		t.Errorf("the last tier moved down: %+v", g.Tiers)
	}
}

// The group page offers the sales' own shares when a brand outsells its manual share.
func TestGroupPageOffersSuggestion(t *testing.T) {
	s, db, sfx := testStore(t)
	ctx := context.Background()
	a, b := product(t, db, "SF"+sfx, "0", false, true), product(t, db, "SS"+sfx, "0", false, true)
	tid := groupTier(t, s, "Offer "+sfx, a, b)
	var sale, user int64
	noErr(t, db.Get(&user, `SELECT id FROM users ORDER BY id LIMIT 1`))
	noErr(t, db.Get(&sale, `INSERT INTO sales (receipt_no, cashier_id) VALUES ($1, $2) RETURNING id`, "o"+sfx[len(sfx)-11:], user))
	t.Cleanup(func() { db.Exec(`DELETE FROM sales WHERE id = $1`, sale) }) //nolint:errcheck
	noErrExec(t, db, `INSERT INTO sale_items (sale_id, product_id, quantity, unit_price, subtotal) VALUES ($1, $2, 30, 1, 30)`, sale, a)
	noErr(t, s.SetSplit(ctx, tid, false, map[int64]int{a: 50, b: 50}))
	tier, _ := s.tier(ctx, tid)
	g, err := s.Group(ctx, tier.GroupID)
	noErr(t, err)
	sg, err := s.suggestFor(ctx, g.Tiers[0])
	noErr(t, err)
	if sg == nil || sg.ProductID != a {
		t.Errorf("no offer for the faster brand: %+v", sg)
	}
}
