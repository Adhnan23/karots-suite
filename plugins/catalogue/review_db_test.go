package catalogue

import (
	"context"
	"testing"
)

// CTL-8: a sync that changes what a linked product took puts it on review, unless it already has the new value; a
// field the shop doesn't have is never a review. CTL-12: a gone item's link follows it into the kept one.
// CTL-9: Use the catalogue's writes the name and fields (never the unit); Keep mine keeps the product; neither is
// asked again.
func TestLaterChangesWaitForReview(t *testing.T) {
	useFakeFields()
	p := newPub(t)
	ctx := context.Background()
	db := testDB(t)
	seedShop(t, db)
	s := NewStore(db)
	sy := NewSyncer(db, p.srv.URL)
	sy.afterChange = s.afterChange
	F := func(gen, company string) map[string]string {
		return map[string]string{"Generic Name": gen, "Company": company}
	}
	p.publish([]Item{
		{ID: 1, Name: "Panadol 500mg", Unit: "tab", Kinds: "pharmacy", Fields: F("Paracetamol", "GSK")},
		{ID: 2, Name: "Old Twin", Unit: "tab", Kinds: "pharmacy", Fields: map[string]string{}},
		{ID: 4, Name: "Steady", Unit: "tab", Kinds: "pharmacy", Fields: F("Steady", "S")},
	}, nil)
	if _, err := sy.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	a, b, c, d := shopProduct(t, db, "Panadol 500mg"), shopProduct(t, db, "Old Twin"), shopProduct(t, db, "Panadol 500mg B"), shopProduct(t, db, "Steady")
	for pid, item := range map[int64]int64{a: 1, b: 2, c: 1, d: 4} {
		if err := s.link(ctx, pid, item); err != nil {
			t.Fatal(err)
		}
	}
	fakeMu.Lock()
	fakeVals[c]["gen"] = "Acetaminophen" // c already has the new value
	fakeMu.Unlock()

	p.publish([]Item{
		{ID: 1, Name: "Panadol 500mg", Unit: "cap", Kinds: "pharmacy", Fields: F("Acetaminophen", "GSK Ltd")},
		{ID: 3, Name: "Twin New", Unit: "tab", Kinds: "pharmacy", Fields: map[string]string{"Generic Name": "Ibuprofen"}},
		{ID: 4, Name: "Steady", Unit: "tab", Kinds: "pharmacy", Fields: F("Steady", "Company only changed")},
	}, map[int64]int64{2: 3})
	if _, err := sy.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	want := func(pid int64, n int) []change {
		t.Helper()
		cs, err := s.Waiting(ctx, pid)
		if err != nil || len(cs) != n {
			t.Fatalf("product %d: %d changes %+v (%v), want %d", pid, len(cs), cs, err, n)
		}
		return cs
	}
	if cs := want(a, 2); cs[0].What != "Unit" && cs[1].What != "Unit" {
		t.Fatalf("a: %+v (the unit and the generic)", cs)
	}
	want(c, 1) // only the unit: it already has the new generic
	want(d, 0) // Company isn't one of the shop's fields
	var moved int64
	db.Get(&moved, `SELECT item_id FROM catalogue_links WHERE product_id = $1`, b) //nolint:errcheck
	if moved != 3 {
		t.Fatalf("the gone item's link is on %d, want 3", moved)
	}
	want(b, 2) // the name and the generic of the item it went into
	notes, _ := s.reviewSource().Notes(ctx)
	if len(notes[a]) != 2 || notes[a][1] == "" {
		t.Fatalf("notes %v", notes[a])
	}

	var renamed []string
	rename := func(_ context.Context, id int64, name string) error { renamed = append(renamed, name); return nil }
	if err := s.useCatalogue(ctx, a, rename); err != nil {
		t.Fatal(err)
	}
	if fakeValue(a, "gen") != "Acetaminophen" || len(renamed) != 0 {
		t.Fatalf("a: gen %q, renamed %v", fakeValue(a, "gen"), renamed)
	}
	var unit string
	db.Get(&unit, `SELECT u.abbreviation FROM products p JOIN units u ON u.id = p.unit_id WHERE p.id = $1`, a) //nolint:errcheck
	if unit == "cap" {
		t.Fatal("the unit was changed")
	}
	if err := s.useCatalogue(ctx, b, rename); err != nil || len(renamed) != 1 || renamed[0] != "Twin New" {
		t.Fatalf("b renamed %v (%v)", renamed, err)
	}
	if err := s.keepMine(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := s.useCatalogue(ctx, d, rename); err == nil {
		t.Fatal("used a change that wasn't there")
	}
	for _, pid := range []int64{a, b, c} {
		want(pid, 0)
	}
	if notes, _ := s.reviewSource().Notes(ctx); len(notes[a])+len(notes[b])+len(notes[c]) != 0 {
		t.Fatalf("still on review: %v", notes)
	}
	// The same version synced again (a whole reload): nothing asked again.
	db.MustExec(`UPDATE catalogue_state SET version = 0`)
	if _, err := sy.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	for _, pid := range []int64{a, b, d} {
		want(pid, 0)
	}
	want(c, 0)
}
