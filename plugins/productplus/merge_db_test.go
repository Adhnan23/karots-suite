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
