package catalogue

import (
	"context"
	"sync"
	"testing"

	"karots-suite/internal/plugin"
)

// CTL-5: every word somewhere (name or a field), any order; a result the shop has says which product.
// CTL-6: the unit by short name (else the shop's default); only the fields the shop has; none without them.
func TestSearchFindsAndFills(t *testing.T) {
	p := newPub(t)
	ctx := context.Background()
	db := testDB(t)
	p.publish([]Item{
		{ID: 1, Name: "Panadol 500mg", Unit: "tab", Kinds: "pharmacy", Fields: map[string]string{"Generic Name": "Paracetamol", "Company": "GSK"}},
		{ID: 2, Name: "Panadol Syrup 60ml", Unit: "bottle-x", Kinds: "pharmacy", Fields: map[string]string{"Generic Name": "Paracetamol"}},
		{ID: 3, Name: "Bisolol 2.5mg", Unit: "tab", Kinds: "pharmacy", Fields: map[string]string{"Generic Name": "Bisoprolol"}},
		{ID: 4, Name: "50% Off_Item", Unit: "pcs", Kinds: "grocery"},
	}, nil)
	if _, err := NewSyncer(db, p.srv.URL).Sync(ctx); err != nil {
		t.Fatal(err)
	}
	s := NewStore(db)
	var tab, def, cat, mine int64
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.Get(&tab, `INSERT INTO units (name, abbreviation) VALUES ('Tablet T', 'TAB') RETURNING id`))
	db.MustExec(`UPDATE units SET is_default = false`)
	must(db.Get(&def, `INSERT INTO units (name, abbreviation, is_default) VALUES ('Each T', 'ea', true) RETURNING id`))
	must(db.Get(&cat, `INSERT INTO categories (name) VALUES ('T cat') RETURNING id`))
	must(db.Get(&mine, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('bisolol 2.5MG', $1, $2, 0, 0) RETURNING id`, cat, tab))

	ids := func(q string) (out []int64) {
		got, err := s.Search(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range got {
			out = append(out, g.ID)
		}
		return
	}
	if got := ids("paracetamol syrup"); len(got) != 1 || got[0] != 2 {
		t.Fatalf("a field and a name word: %v", got)
	}
	if got := ids("500 panadol"); len(got) != 1 || got[0] != 1 {
		t.Fatalf("any order: %v", got)
	}
	if got := ids("panadol"); len(got) != 2 || got[0] != 1 {
		t.Fatalf("shorter name first: %v", got)
	}
	if got := ids("50%"); len(got) != 1 || got[0] != 4 {
		t.Fatalf("%% is a letter, not a wildcard: %v", got)
	}
	if got := ids("  "); len(got) != 0 {
		t.Fatalf("an empty search finds nothing: %v", got)
	}

	got, _ := s.Search(ctx, "bisolol")
	if got[0].Have == nil || got[0].Have.ID != mine {
		t.Fatalf("the shop's own product (same name) not named: %+v", got[0].Have)
	}
	got, _ = s.Search(ctx, "panadol")
	if got[0].UnitID != tab || got[1].UnitID != def {
		t.Fatalf("units %d %d, want tab %d and the default %d", got[0].UnitID, got[1].UnitID, tab, def)
	}
	if sec := (filler{fields: map[string]fieldRef{}}).sections(got[0].Fields); sec != nil {
		t.Fatalf("a shop without its own fields gets none: %+v", sec) // without Product Plus: name and unit only
	}

	// With the shop's own fields: only those it has, matched by name (case and spaces ignored).
	useFakeFields()
	got, _ = s.Search(ctx, "panadol 500")
	want := map[string]string{"gen": "Paracetamol"}
	if v, _ := got[0].Sections["fake"]["values"].(map[string]string); len(got[0].Sections) != 1 || len(v) != 1 || v["gen"] != want["gen"] {
		t.Fatalf("sections %+v, want fake.values %v", got[0].Sections, want)
	}
}

// fakeFields is a stand-in for Product Plus: the shop's fields "generic  NAME" (key gen) and Strength, values kept
// in memory. Registered once for the package (hooks are global); a test only reads its own products' values.
var (
	fakeOnce sync.Once
	fakeMu   sync.Mutex
	fakeVals = map[int64]map[string]string{}
)

func useFakeFields() {
	fakeOnce.Do(func() {
		plugin.NewRegistry(plugin.Core{}, plugin.NewMux(), nil).AddProductAttributeSet(plugin.ProductAttributeSet{Key: "fake",
			Attributes: func(context.Context) ([]plugin.ProductAttribute, error) {
				return []plugin.ProductAttribute{{Key: "gen", Label: "generic  NAME", Type: "text"}, {Key: "strength", Label: "Strength", Type: "text"}}, nil
			},
			Values: func(_ context.Context, ids []int64) (map[int64]map[string]string, error) {
				fakeMu.Lock()
				defer fakeMu.Unlock()
				out := map[int64]map[string]string{}
				for _, id := range ids {
					out[id] = map[string]string{}
					for k, v := range fakeVals[id] {
						out[id][k] = v
					}
				}
				return out, nil
			},
			Set: func(_ context.Context, id int64, key, v string) error {
				fakeMu.Lock()
				defer fakeMu.Unlock()
				if fakeVals[id] == nil {
					fakeVals[id] = map[string]string{}
				}
				fakeVals[id][key] = v
				return nil
			},
		})
	})
}

func fakeValue(id int64, key string) string {
	fakeMu.Lock()
	defer fakeMu.Unlock()
	return fakeVals[id][key]
}
