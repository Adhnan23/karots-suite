package combos

import (
	"context"
	"testing"

	"karots-suite/internal/plugin/plugintest"
	"karots-suite/plugins/combos/migrations"
)

// Merge: a combo holding both sums them on the kept line; otherwise the line moves.
func TestMergeProduct(t *testing.T) {
	tx, from, into := plugintest.MergeTx(t, migrations.FS, "goose_db_version_combos")
	var both, only int64
	tx.Get(&both, `INSERT INTO combos (name) VALUES ('both') RETURNING id`) //nolint:errcheck
	tx.Get(&only, `INSERT INTO combos (name) VALUES ('only') RETURNING id`) //nolint:errcheck
	tx.MustExec(`INSERT INTO combo_items (combo_id, product_id, qty) VALUES ($1, $3, 2), ($1, $4, 1), ($2, $3, 1)`, both, only, from, into)
	if err := mergeProduct(context.Background(), tx, from, into); err != nil {
		t.Fatal(err)
	}
	if n := plugintest.Count(t, tx, `SELECT COUNT(*) FROM combo_items WHERE product_id = $1`, from); n != 0 {
		t.Fatalf("%d combo lines left on the duplicate", n)
	}
	if n := plugintest.Count(t, tx, `SELECT qty::int FROM combo_items WHERE combo_id = $1 AND product_id = $2`, both, into); n != 3 {
		t.Fatalf("summed qty %d, want 3", n)
	}
	if n := plugintest.Count(t, tx, `SELECT COUNT(*) FROM combo_items WHERE combo_id = $1 AND product_id = $2`, only, into); n != 1 {
		t.Fatal("the combo with the duplicate alone did not move")
	}
}
