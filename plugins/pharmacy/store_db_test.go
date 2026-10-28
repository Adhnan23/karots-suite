package pharmacy

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// fixture: a medicine group "Metformin 500" (tiers Genuine, Cheap) with
// Glucophage (Genuine, 0 left) and Metfor (Cheap, 40 left), and Panadol alone.
type fixture struct {
	db                         *sqlx.DB
	st                         *Store
	group, gluco, metfor, pana int64
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	db := freshDB(t)
	f := fixture{db: db, st: NewStore(db)}
	db.MustExec(`INSERT INTO categories (name) VALUES ('Medicines')`)
	db.MustExec(`INSERT INTO users (name, phone, pin_hash) VALUES ('Cashier', '0700000000', 'x')`)
	prod := func(name string, stock int) int64 {
		var id int64
		if err := db.Get(&id, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
			VALUES ($1, (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 2, 3) RETURNING id`, name); err != nil {
			t.Fatal(err)
		}
		db.MustExec(`UPDATE stock SET quantity = $2 WHERE product_id = $1`, id, stock)
		return id
	}
	f.gluco, f.metfor, f.pana = prod("Glucophage 500", 0), prod("Metfor 500", 40), prod("Panadol 500", 100)
	var gen, cheap int64
	db.Get(&f.group, `INSERT INTO alt_groups (name) VALUES ('Metformin 500') RETURNING id`)                            //nolint:errcheck
	db.Get(&gen, `INSERT INTO alt_tiers (group_id, name, sort_order) VALUES ($1, 'Genuine', 0) RETURNING id`, f.group) //nolint:errcheck
	db.Get(&cheap, `INSERT INTO alt_tiers (group_id, name, sort_order) VALUES ($1, 'Cheap', 1) RETURNING id`, f.group) //nolint:errcheck
	db.MustExec(`INSERT INTO alt_members (product_id, tier_id) VALUES ($1, $2), ($3, $4)`, f.gluco, gen, f.metfor, cheap)
	return f
}

func dec(n int64) decimal.Decimal { return decimal.NewFromInt(n) }

func TestPatientsAndScripts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p, err := f.st.Register(ctx, "Nimal Perera", "077 123 4567", Profile{Allergies: "Penicillin"})
	if err != nil || p.Allergies != "Penicillin" || p.Phone == nil || *p.Phone != "0771234567" {
		t.Fatalf("register: %+v %v", p, err)
	}
	// Same phone again = the same customer and patient; nothing typed keeps the allergies.
	again, err := f.st.Register(ctx, "Nimal", "0771234567", Profile{})
	if err != nil || again.ID != p.ID || again.Allergies != "Penicillin" {
		t.Fatalf("register again: %+v %v", again, err)
	}
	if again, err = f.st.FromCustomer(ctx, p.CustomerID); err != nil || again.ID != p.ID {
		t.Fatalf("from customer is idempotent: %+v %v", again, err)
	}
	found, _ := f.st.Search(ctx, "nimal", true, 10)
	if len(found) != 1 || found[0].PatientID != p.ID {
		t.Fatalf("search: %+v", found)
	}

	g, pana := f.group, f.pana
	id, err := f.st.CreateScript(ctx, p.ID, 0, ScriptInput{PrescriberName: "Dr Silva", IsRepeat: true, Items: []ItemInput{
		{GroupID: &g, Dosage: "1-0-1", Quantity: dec(60)},
		{ProductID: &pana, Dosage: "tds", Quantity: dec(15)},
		{DrugText: "  "}, // blank line dropped
	}})
	if err != nil {
		t.Fatal(err)
	}
	sc, err := f.st.Script(ctx, id)
	if err != nil || len(sc.Items) != 2 {
		t.Fatalf("script: %+v %v", sc, err)
	}
	met, par := sc.Items[0], sc.Items[1]
	if met.Drug != "Metformin 500" || !met.PerDay.Equal(dec(2)) || !par.PerDay.Equal(dec(3)) || par.GroupID != nil {
		t.Fatalf("lines: %+v / %+v", met, par)
	}
	if !met.Owed().Equal(dec(60)) || met.NextDue() != nil {
		t.Fatalf("nothing handed over yet: owed %s", met.Owed())
	}
	// Glucophage (best tier) is out — Add suggests Metfor.
	if sg, err := f.st.Suggest(ctx, met); err != nil || sg == nil || sg.ProductID != f.metfor {
		t.Fatalf("suggest: %+v %v", sg, err)
	}

	// A hand-over of 30 against the Metformin line, then an edit: the line keeps its history.
	var d int64
	f.db.Get(&d, `INSERT INTO pharmacy_dispenses (patient_id, prescription_id, days_supply) VALUES ($1, $2, 15) RETURNING id`, p.ID, id) //nolint:errcheck
	f.db.MustExec(`INSERT INTO pharmacy_dispense_items (dispense_id, product_id, quantity, prescription_item_id, days_supply)
		VALUES ($1, $2, 30, $3, 15)`, d, f.metfor, met.ID)
	if err := f.st.UpdateScript(ctx, id, ScriptInput{PrescriberName: "Dr Silva", IsRepeat: true, Items: []ItemInput{
		{ID: met.ID, GroupID: &g, Dosage: "1-1-1", Quantity: dec(90)},
	}}); err != nil {
		t.Fatal(err)
	}
	sc, _ = f.st.Script(ctx, id)
	if len(sc.Items) != 1 || sc.Items[0].ID != met.ID || !sc.Items[0].Dispensed.Equal(dec(30)) ||
		!sc.Items[0].Owed().Equal(dec(90)) || sc.Items[0].NextDue() == nil {
		t.Fatalf("after edit: %+v", sc.Items)
	}
	// A line id from another script is refused.
	if err := f.st.UpdateScript(ctx, id, ScriptInput{Items: []ItemInput{{ID: met.ID + 999, DrugText: "x"}}}); err == nil {
		t.Fatal("a foreign line id must be refused")
	}
}

// The script's medicine picker finds a product by its local name too.
func TestScriptPickerFindsLocalName(t *testing.T) {
	f := newFixture(t)
	f.db.MustExec(`UPDATE products SET name_local = 'පැරසිටමෝල්' WHERE id = $1`, f.pana)
	got, err := f.st.Medicines(context.Background(), "පැරසි")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range got {
		if m.Kind == "product" && m.ID == f.pana {
			return
		}
	}
	t.Fatalf("local name not found: %+v", got)
}
