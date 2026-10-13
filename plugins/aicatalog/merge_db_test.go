package aicatalog

import (
	"context"
	"testing"

	"karots-suite/internal/plugin/plugintest"
	"karots-suite/plugins/aicatalog/migrations"
)

// Merge: the kept product's learnt item wins; the duplicate's alone moves over.
func TestMergeProduct(t *testing.T) {
	tx, from, into := plugintest.MergeTx(t, migrations.FS, "goose_db_version_aicatalog")
	ins := `INSERT INTO aicatalog_items (product_id, resolved_name, source) VALUES ($1, $2, 'user_picked')`
	tx.MustExec(ins, from, "dup")
	tx.MustExec(ins, into, "kept")
	if err := mergeProduct(context.Background(), tx, from, into); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := tx.Get(&name, `SELECT resolved_name FROM aicatalog_items WHERE product_id = $1`, into); err != nil || name != "kept" {
		t.Fatalf("kept item %q %v", name, err)
	}
	tx.MustExec(`DELETE FROM aicatalog_items WHERE product_id = $1`, into)
	tx.MustExec(ins, from, "dup")
	if err := mergeProduct(context.Background(), tx, from, into); err != nil {
		t.Fatal(err)
	}
	if n := plugintest.Count(t, tx, `SELECT COUNT(*) FROM aicatalog_items WHERE product_id = $1`, into); n != 1 {
		t.Fatal("the duplicate's item did not move")
	}
}
