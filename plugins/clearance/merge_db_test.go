package clearance

import (
	"context"
	"testing"

	"karots-suite/internal/plugin/plugintest"
	"karots-suite/plugins/clearance/migrations"
)

// Merge: the kept product's markdown wins; the duplicate's alone moves over.
func TestMergeProduct(t *testing.T) {
	tx, from, into := plugintest.MergeTx(t, migrations.FS, "goose_db_version_clearance")
	tx.MustExec(`INSERT INTO clearance_markdowns (product_id, discount_value) VALUES ($1, 10), ($2, 20)`, from, into)
	if err := mergeProduct(context.Background(), tx, from, into); err != nil {
		t.Fatal(err)
	}
	if n := plugintest.Count(t, tx, `SELECT discount_value::int FROM clearance_markdowns WHERE product_id = $1`, into); n != 20 {
		t.Fatalf("kept markdown %d, want 20", n)
	}
	tx.MustExec(`DELETE FROM clearance_markdowns WHERE product_id = $1`, into)
	tx.MustExec(`INSERT INTO clearance_markdowns (product_id, discount_value) VALUES ($1, 10)`, from)
	if err := mergeProduct(context.Background(), tx, from, into); err != nil {
		t.Fatal(err)
	}
	if n := plugintest.Count(t, tx, `SELECT COUNT(*) FROM clearance_markdowns WHERE product_id = $1`, into); n != 1 {
		t.Fatal("the duplicate's markdown did not move")
	}
}
