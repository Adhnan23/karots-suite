package requests

import (
	"context"
	"testing"

	"karots-suite/internal/plugin/plugintest"
	"karots-suite/plugins/requests/migrations"
)

// Merge: a request converted to the duplicate now points at the kept product.
func TestMergeProduct(t *testing.T) {
	tx, from, into := plugintest.MergeTx(t, migrations.FS, "goose_db_version_requests")
	tx.MustExec(`INSERT INTO requests (note, status, product_id) VALUES ('wanted', 'converted', $1)`, from)
	if err := mergeProduct(context.Background(), tx, from, into); err != nil {
		t.Fatal(err)
	}
	if n := plugintest.Count(t, tx, `SELECT COUNT(*) FROM requests WHERE product_id = $1`, into); n != 1 {
		t.Fatal("request not moved")
	}
}
