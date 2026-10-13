package productplus

import (
	"context"
	"testing"

	"karots-suite/internal/plugin/plugintest"
	"karots-suite/plugins/productplus/migrations"
)

// Merge: the kept product's value for a field wins; a value only the duplicate had moves.
func TestMergeProduct(t *testing.T) {
	tx, from, into := plugintest.MergeTx(t, migrations.FS, "goose_db_version_productplus")
	var a, b int64
	tx.Get(&a, `INSERT INTO pp_fields (key, label, type) VALUES ('merge_a', 'A', 'text') RETURNING id`) //nolint:errcheck
	tx.Get(&b, `INSERT INTO pp_fields (key, label, type) VALUES ('merge_b', 'B', 'text') RETURNING id`) //nolint:errcheck
	tx.MustExec(`INSERT INTO pp_values (field_id, product_id, value) VALUES ($1, $3, 'dup'), ($1, $4, 'kept'), ($2, $3, 'only')`, a, b, from, into)
	if err := mergeProduct(context.Background(), tx, from, into); err != nil {
		t.Fatal(err)
	}
	var va, vb string
	tx.Get(&va, `SELECT value FROM pp_values WHERE field_id = $1 AND product_id = $2`, a, into) //nolint:errcheck
	tx.Get(&vb, `SELECT value FROM pp_values WHERE field_id = $1 AND product_id = $2`, b, into) //nolint:errcheck
	if va != "kept" || vb != "only" || plugintest.Count(t, tx, `SELECT COUNT(*) FROM pp_values WHERE product_id = $1`, from) != 0 {
		t.Fatalf("values a=%q b=%q", va, vb)
	}
}

// Category merge: a field scoped to the merged category also covers the kept one;
// undo removes only what the merge added (a scope already on the kept one stays).
func TestMergeCategory(t *testing.T) {
	tx, _, _ := plugintest.MergeTx(t, migrations.FS, "goose_db_version_productplus")
	ctx := context.Background()
	var from, into, only, both int64
	tx.Get(&from, `INSERT INTO categories (name) VALUES ('cm from') RETURNING id`)                            //nolint:errcheck
	tx.Get(&into, `INSERT INTO categories (name) VALUES ('cm into') RETURNING id`)                            //nolint:errcheck
	tx.Get(&only, `INSERT INTO pp_fields (key, label, type) VALUES ('cm_only', 'Only', 'text') RETURNING id`) //nolint:errcheck
	tx.Get(&both, `INSERT INTO pp_fields (key, label, type) VALUES ('cm_both', 'Both', 'text') RETURNING id`) //nolint:errcheck
	tx.MustExec(`INSERT INTO pp_field_categories (field_id, category_id) VALUES ($1, $3), ($2, $3), ($2, $4)`, only, both, from, into)
	scoped := func(f, c int64) int {
		return plugintest.Count(t, tx, `SELECT count(*) FROM pp_field_categories WHERE field_id = $1 AND category_id = $2`, f, c)
	}
	note, err := mergeCategory(ctx, tx, from, into)
	if err != nil || note == nil {
		t.Fatalf("note %s %v", note, err)
	}
	if scoped(only, into) != 1 || scoped(both, into) != 1 {
		t.Fatal("the merged category's fields should cover the kept one")
	}
	if err := unmergeCategory(ctx, tx, note); err != nil {
		t.Fatal(err)
	}
	if scoped(only, into) != 0 || scoped(both, into) != 1 || scoped(only, from) != 1 || scoped(both, from) != 1 {
		t.Fatal("undo should remove only the scope rows the merge added")
	}
	// Nothing to add (into's only field is already on from): no note.
	if n, err := mergeCategory(ctx, tx, into, from); err != nil || n != nil {
		t.Fatalf("reverse merge note %s %v", n, err)
	}
}
