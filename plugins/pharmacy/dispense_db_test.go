package pharmacy

import (
	"context"
	"errors"
	"testing"
	"time"
)

// sale books a sale of (product, qty) pairs directly (core's sale path is not the
// subject here) and returns its id.
func (f fixture) sale(t *testing.T, lines ...[2]int64) int64 {
	t.Helper()
	var id int64
	if err := f.db.Get(&id, `INSERT INTO sales (receipt_no, cashier_id) VALUES ('S-T' || nextval('sales_id_seq'), (SELECT id FROM users ORDER BY id LIMIT 1)) RETURNING id`); err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		f.db.MustExec(`INSERT INTO sale_items (sale_id, product_id, quantity, unit_price, subtotal) VALUES ($1, $2, $3::numeric, 3, $3::numeric * 3)`, id, l[0], l[1])
	}
	return id
}

func TestRecordDispense(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p, _ := f.st.Register(ctx, "Sunil", "0712222222", Profile{})
	g := f.group
	rx, err := f.st.CreateScript(ctx, p.ID, 0, ScriptInput{PrescriberName: "Dr Silva", IsRepeat: true,
		Items: []ItemInput{{GroupID: &g, Dosage: "1-0-1", Quantity: dec(60)}}})
	if err != nil {
		t.Fatal(err)
	}
	var soap int64
	f.db.Get(&soap, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price) VALUES ('Soap', (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 1, 2) RETURNING id`) //nolint:errcheck

	// 1 card of Glucophage (12) + 10 loose Metfor on two lines + soap; 2 Metfor returned.
	sale := f.sale(t, [2]int64{f.gluco, 12}, [2]int64{f.metfor, 6}, [2]int64{f.metfor, 6}, [2]int64{soap, 1})
	f.db.MustExec(`UPDATE sale_items SET returned_qty = 2 WHERE sale_id = $1 AND product_id = $2 AND id = (SELECT MAX(id) FROM sale_items WHERE sale_id = $1)-1`, sale, f.metfor)
	id, err := f.st.RecordDispense(ctx, DispenseInput{SaleID: sale, PatientID: p.ID, PrescriptionID: rx})
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Product int64  `db:"product_id"`
		Qty     int64  `db:"quantity"`
		Item    *int64 `db:"prescription_item_id"`
		Days    int    `db:"days_supply"`
	}
	f.db.Select(&rows, `SELECT product_id, quantity::bigint AS quantity, prescription_item_id, days_supply FROM pharmacy_dispense_items WHERE dispense_id = $1 ORDER BY id`, id) //nolint:errcheck
	// Stored as sold (12 + 12); the 2 returned come off when the script is read.
	if len(rows) != 2 || rows[0].Product != f.gluco || rows[0].Qty != 12 || rows[1].Qty != 12 || rows[0].Days != 12 || rows[1].Days != 12 || rows[0].Item == nil {
		t.Fatalf("rows: %+v", rows)
	}
	var who, doc string
	f.db.QueryRow(`SELECT patient_name, prescriber_name FROM pharmacy_dispenses WHERE id = $1`, id).Scan(&who, &doc) //nolint:errcheck
	if who != "Sunil" || doc != "Dr Silva" {
		t.Fatalf("snapshots %q %q", who, doc)
	}
	sc, _ := f.st.Script(ctx, rx)
	if !sc.Items[0].Dispensed.Equal(dec(22)) || !sc.Items[0].Owed().Equal(dec(60)) || sc.Items[0].DaysSupply != 11 { // 12 × 22/24
		t.Fatalf("script line after: %+v", sc.Items[0])
	}
	// A second record of the same sale is refused; soap alone is nothing to record.
	if _, err := f.st.RecordDispense(ctx, DispenseInput{SaleID: sale, PatientID: p.ID}); err == nil {
		t.Fatal("recorded the same receipt twice")
	}
	if _, err := f.st.RecordDispense(ctx, DispenseInput{SaleID: f.sale(t, [2]int64{soap, 2}), PatientID: p.ID}); !errors.Is(err, ErrNothingToRecord) {
		t.Fatalf("soap only: %v", err)
	}
	recent, err := f.st.RecentSales(ctx, "", 10)
	if err != nil || len(recent) < 2 || recent[1].Recorded != "Sunil" || recent[1].Medicines != 3 {
		t.Fatalf("recent: %+v %v", recent, err)
	}
}

// With no script named, the sale fills the patient's active scripts (each line
// by its group); the record takes the first filled script and its doctor.
func TestRecordDispenseFindsScripts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p, _ := f.st.Register(ctx, "Kumari", "0715555555", Profile{})
	g, pana := f.group, f.pana
	old, _ := f.st.CreateScript(ctx, p.ID, 0, ScriptInput{PrescriberName: "Dr Old", Items: []ItemInput{{GroupID: &g, Dosage: "1-0-1", Quantity: dec(20)}}})
	f.st.SetScriptActive(ctx, old, false) //nolint:errcheck
	a, _ := f.st.CreateScript(ctx, p.ID, 0, ScriptInput{PrescriberName: "Dr A", Items: []ItemInput{{GroupID: &g, Dosage: "1-0-1", Quantity: dec(20)}}})
	b, _ := f.st.CreateScript(ctx, p.ID, 0, ScriptInput{PrescriberName: "Dr B", Items: []ItemInput{{ProductID: &pana, Dosage: "tds", Quantity: dec(9)}}})
	id, err := f.st.RecordDispense(ctx, DispenseInput{SaleID: f.sale(t, [2]int64{f.metfor, 20}, [2]int64{f.pana, 9}), PatientID: p.ID})
	if err != nil {
		t.Fatal(err)
	}
	var rx int64
	var doc string
	f.db.QueryRow(`SELECT prescription_id, prescriber_name FROM pharmacy_dispenses WHERE id = $1`, id).Scan(&rx, &doc) //nolint:errcheck
	if rx != a || doc != "Dr A" {
		t.Fatalf("header: rx %d doc %q", rx, doc)
	}
	for sc, want := range map[int64]int64{a: 20, b: 9, old: 0} {
		s, _ := f.st.Script(ctx, sc)
		if !s.Items[0].Dispensed.Equal(dec(want)) {
			t.Errorf("script %d dispensed %s, want %d", sc, s.Items[0].Dispensed, want)
		}
	}
}

// A returned sale gives back what the script was given: "given" and the days
// shrink with the return (60 / 30 days, 20 back → 40 / 20 days); a hand-over
// returned in full no longer counts — the one before it does.
func TestReturnRollsBackTheScript(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p, _ := f.st.Register(ctx, "Ret P", "0719999999", Profile{})
	g := f.group
	rx, _ := f.st.CreateScript(ctx, p.ID, 0, ScriptInput{IsRepeat: true, Items: []ItemInput{{GroupID: &g, Dosage: "1-0-1", Quantity: dec(60)}}})
	first := f.sale(t, [2]int64{f.metfor, 20})
	if _, err := f.st.RecordDispense(ctx, DispenseInput{SaleID: first, PatientID: p.ID}); err != nil {
		t.Fatal(err)
	}
	f.db.MustExec(`UPDATE pharmacy_dispenses SET dispensed_at = now() - interval '5 days' WHERE sale_id = $1`, first)
	second := f.sale(t, [2]int64{f.metfor, 60})
	if _, err := f.st.RecordDispense(ctx, DispenseInput{SaleID: second, PatientID: p.ID}); err != nil {
		t.Fatal(err)
	}
	line := func() ScriptItem { sc, _ := f.st.Script(ctx, rx); return sc.Items[0] }
	if it := line(); !it.Dispensed.Equal(dec(80)) || it.DaysSupply != 30 {
		t.Fatalf("before the return: %+v", it)
	}
	f.db.MustExec(`UPDATE sale_items SET returned_qty = 20 WHERE sale_id = $1`, second)
	if it := line(); !it.Dispensed.Equal(dec(60)) || it.DaysSupply != 20 {
		t.Fatalf("20 of 60 returned: given %s, days %d", it.Dispensed, it.DaysSupply)
	}
	f.db.MustExec(`UPDATE sale_items SET returned_qty = 60 WHERE sale_id = $1`, second)
	it := line()
	if !it.Dispensed.Equal(dec(20)) || it.DaysSupply != 10 || it.LastAt == nil || time.Since(*it.LastAt) < 4*24*time.Hour {
		t.Fatalf("all of it returned — the earlier hand-over counts: %+v", it)
	}
}
