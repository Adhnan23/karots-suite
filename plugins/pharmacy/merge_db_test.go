package pharmacy

import (
	"context"
	"testing"

	"karots-suite/internal/plugin/plugintest"
	"karots-suite/plugins/pharmacy/migrations"
)

// Merge: script / dispense / old-register rows follow the kept product; a
// controlled mark or Rx flag on both keeps the kept one's, on the duplicate alone moves.
func TestMergeProduct(t *testing.T) {
	testDB(t)
	tx, from, into := plugintest.MergeTx(t, migrations.FS, "goose_db_version_pharmacy")
	ctx := context.Background()
	var cust, pat, rx, d int64
	tx.Get(&cust, `INSERT INTO customers (name) VALUES ('merge patient') RETURNING id`)          //nolint:errcheck
	tx.Get(&pat, `INSERT INTO pharmacy_patients (customer_id) VALUES ($1) RETURNING id`, cust)   //nolint:errcheck
	tx.Get(&rx, `INSERT INTO pharmacy_prescriptions (patient_id) VALUES ($1) RETURNING id`, pat) //nolint:errcheck
	tx.Get(&d, `INSERT INTO pharmacy_dispenses (patient_id) VALUES ($1) RETURNING id`, pat)      //nolint:errcheck
	tx.MustExec(`INSERT INTO pharmacy_prescription_items (prescription_id, product_id) VALUES ($1, $2)`, rx, from)
	tx.MustExec(`INSERT INTO pharmacy_dispense_items (dispense_id, product_id) VALUES ($1, $2)`, d, from)
	tx.MustExec(`INSERT INTO pharmacy_cd_ledger (product_id, direction, quantity) VALUES ($1, 'in', 5)`, from)
	tx.MustExec(`INSERT INTO pharmacy_controlled_products (product_id, schedule) VALUES ($1, 'dup'), ($2, 'kept')`, from, into)
	tx.MustExec(`INSERT INTO pharmacy_medicines (product_id, rx_only) VALUES ($1, true)`, from)
	if err := mergeProduct(ctx, tx, from, into); err != nil {
		t.Fatal(err)
	}
	for q, want := range map[string]int{
		`SELECT COUNT(*) FROM pharmacy_prescription_items WHERE product_id = $1`:                        1,
		`SELECT COUNT(*) FROM pharmacy_dispense_items WHERE product_id = $1`:                            1,
		`SELECT COUNT(*) FROM pharmacy_cd_ledger WHERE product_id = $1`:                                 1,
		`SELECT COUNT(*) FROM pharmacy_controlled_products WHERE product_id = $1 AND schedule = 'kept'`: 1,
		`SELECT COUNT(*) FROM pharmacy_medicines WHERE product_id = $1 AND rx_only`:                     1,
	} {
		if n := plugintest.Count(t, tx, q, into); n != want {
			t.Fatalf("%s = %d, want %d", q, n, want)
		}
	}
	if n := plugintest.Count(t, tx, `SELECT COUNT(*) FROM pharmacy_controlled_products WHERE product_id = $1`, from); n != 0 {
		t.Fatal("the duplicate's controlled mark stayed")
	}
}
