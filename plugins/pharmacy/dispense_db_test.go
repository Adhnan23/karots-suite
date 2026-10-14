package pharmacy

import (
	"context"
	"errors"
	"testing"
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
	if len(rows) != 2 || rows[0].Product != f.gluco || rows[0].Qty != 12 || rows[1].Qty != 10 || rows[0].Days != 11 || rows[1].Days != 11 || rows[0].Item == nil {
		t.Fatalf("rows: %+v", rows)
	}
	var who, doc string
	f.db.QueryRow(`SELECT patient_name, prescriber_name FROM pharmacy_dispenses WHERE id = $1`, id).Scan(&who, &doc) //nolint:errcheck
	if who != "Sunil" || doc != "Dr Silva" {
		t.Fatalf("snapshots %q %q", who, doc)
	}
	sc, _ := f.st.Script(ctx, rx)
	if !sc.Items[0].Dispensed.Equal(dec(22)) || !sc.Items[0].Owed().Equal(dec(38)) || sc.Items[0].DaysSupply != 11 {
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
