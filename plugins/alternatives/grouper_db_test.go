package alternatives

import (
	"context"
	"testing"
)

func TestGrouperCreate(t *testing.T) {
	s, db, sfx := testStore(t)
	ctx, g := context.Background(), s.grouper()
	a, b, c := product(t, db, "A"+sfx, "1", false, true), product(t, db, "B"+sfx, "1", false, true), product(t, db, "C"+sfx, "1", false, true)
	off := product(t, db, "Off"+sfx, "1", false, false)
	id, err := g.Create(ctx, "Twins "+sfx, []int64{a, b, off})
	noErr(t, err)
	if id == 0 {
		t.Fatal("no group")
	}
	full, err := s.Group(ctx, id)
	noErr(t, err)
	if len(full.Tiers) != 1 || full.Tiers[0].Name != "" || len(full.Tiers[0].Members) != 2 {
		t.Fatalf("%+v", full)
	}
	// a and b are taken now; c alone can't make a group, and nothing is left behind.
	if got, err := g.Create(ctx, "Too few "+sfx, []int64{a, b, c}); err != nil || got != 0 {
		t.Fatalf("%d %v", got, err)
	}
	infos, err := g.Groups(ctx)
	noErr(t, err)
	found := 0
	for _, x := range infos {
		if x.ID == id {
			found++
			if len(x.Members) != 2 || len(x.Tiers) != 1 {
				t.Fatalf("%+v", x)
			}
		}
		if x.Name == "Too few "+sfx {
			t.Fatal("an empty group was left behind")
		}
	}
	if found != 1 {
		t.Fatal("the group isn't listed")
	}
	noErr(t, g.AddTo(ctx, full.Tiers[0].ID, c))
	if err := g.AddTo(ctx, full.Tiers[0].ID, c); err != nil {
		t.Fatal("adding again is a no-op")
	}
	noErr(t, g.Remove(ctx, c))
	noErr(t, g.Delete(ctx, id))
	if _, err := s.Group(ctx, id); err == nil {
		t.Fatal("still there")
	}
}
