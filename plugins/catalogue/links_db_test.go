package catalogue

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
)

// shopProduct makes a product of the test's own.
func shopProduct(t *testing.T, db *sqlx.DB, name string) int64 {
	t.Helper()
	var id int64
	if err := db.Get(&id, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ($1, (SELECT id FROM categories ORDER BY id LIMIT 1), (SELECT id FROM units ORDER BY id LIMIT 1), 0, 0) RETURNING id`, name); err != nil {
		t.Fatal(err)
	}
	return id
}

func seedShop(t *testing.T, db *sqlx.DB) {
	t.Helper()
	db.MustExec(`INSERT INTO categories (name) VALUES ('T cat')`)
	db.MustExec(`INSERT INTO units (name, abbreviation) VALUES ('Tablet T', 'tab')`)
}

// CTL-7: a saved pick is linked and waits on Items to review until reviewed; PLG-3: an item not in the copy is
// refused. CTL-6: the shop's empty fields fill, a typed one stays.
func TestPickLinksFillsAndWaitsForReview(t *testing.T) {
	useFakeFields()
	p := newPub(t)
	ctx := context.Background()
	db := testDB(t)
	seedShop(t, db)
	p.publish([]Item{{ID: 7, Name: "Panadol 500mg", Unit: "tab", Kinds: "pharmacy", Fields: map[string]string{"Generic Name": "Paracetamol", "Strength": "500mg", "Company": "GSK"}}}, nil)
	if _, err := NewSyncer(db, p.srv.URL).Sync(ctx); err != nil {
		t.Fatal(err)
	}
	s := NewStore(db)
	x := s.extras()
	raw := func(id int64) json.RawMessage { return json.RawMessage(fmt.Sprintf(`{"itemId":%d}`, id)) }

	if err := x.Validate(ctx, plugin.ExtraCtx{IsNew: true}, raw(9)); err == nil {
		t.Fatal("an item not in the copy was taken")
	}
	if err := x.Validate(ctx, plugin.ExtraCtx{IsNew: true}, json.RawMessage(`{}`)); err == nil {
		t.Fatal("a pick without its item was taken")
	}
	if err := x.Validate(ctx, plugin.ExtraCtx{IsNew: true}, raw(7)); err != nil {
		t.Fatal(err)
	}
	pid := shopProduct(t, db, "Panadol 500mg")
	fakeMu.Lock()
	fakeVals[pid] = map[string]string{"strength": "typed by the person"}
	fakeMu.Unlock()
	if err := x.Save(ctx, plugin.ExtraCtx{ProductID: pid, IsNew: true}, raw(7)); err != nil {
		t.Fatal(err)
	}
	if fakeValue(pid, "gen") != "Paracetamol" || fakeValue(pid, "strength") != "typed by the person" {
		t.Fatalf("fields: gen %q strength %q", fakeValue(pid, "gen"), fakeValue(pid, "strength"))
	}
	// Saved twice (CTL-10): still one link.
	if err := x.Save(ctx, plugin.ExtraCtx{ProductID: pid, IsNew: true}, raw(7)); err != nil {
		t.Fatal(err)
	}
	var links int
	db.Get(&links, `SELECT count(*) FROM catalogue_links WHERE product_id = $1`, pid) //nolint:errcheck
	if links != 1 {
		t.Fatalf("%d links", links)
	}

	rs := s.reviewSource()
	notes, err := rs.Notes(ctx)
	if err != nil || len(notes[pid]) != 1 || notes[pid][0] != "From the catalogue" {
		t.Fatalf("notes %v %v", notes, err)
	}
	if err := rs.Clear(ctx, pid); err != nil {
		t.Fatal(err)
	}
	if notes, _ = rs.Notes(ctx); len(notes[pid]) != 0 {
		t.Fatalf("still waiting after review: %v", notes[pid])
	}
}
