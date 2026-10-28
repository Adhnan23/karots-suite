package documents

import (
	"context"
	"testing"

	"karots-suite/internal/plugin/plugintest"
	"karots-suite/plugins/documents/migrations"
)

// Merge: paper used by a service moves (a size the service already takes from the
// kept product keeps that one); two print services become one — the duplicate's
// jobs move to the kept service and its own link goes.
func TestMergeProduct(t *testing.T) {
	tx, from, into := plugintest.MergeTx(t, migrations.FS, "goose_db_version_documents")
	var svcFrom, svcInto, other int64
	tx.Get(&svcFrom, `INSERT INTO doc_service (name, product_id) VALUES ('dup', $1) RETURNING id`, from)  //nolint:errcheck
	tx.Get(&svcInto, `INSERT INTO doc_service (name, product_id) VALUES ('kept', $1) RETURNING id`, into) //nolint:errcheck
	tx.Get(&other, `INSERT INTO doc_service (name, product_id) VALUES ('other', $1) RETURNING id`, into)  //nolint:errcheck
	tx.MustExec(`INSERT INTO doc_job (service_id, description) VALUES ($1, 'old job')`, svcFrom)
	tx.MustExec(`INSERT INTO doc_consumable (service_id, size, product_id) VALUES ($1, 'A4', $2), ($1, 'A4', $3), ($1, 'A3', $2)`, other, from, into)
	tx.MustExec(`INSERT INTO doc_extra (service_id, name, product_id) VALUES ($1, 'Lanyard', $2)`, other, from)
	if err := mergeProduct(context.Background(), tx, from, into); err != nil {
		t.Fatal(err)
	}
	if n := plugintest.Count(t, tx, `SELECT (SELECT COUNT(*) FROM doc_service WHERE product_id = $1) + (SELECT COUNT(*) FROM doc_consumable WHERE product_id = $1)
		+ (SELECT COUNT(*) FROM doc_extra WHERE product_id = $1)`, from); n != 0 {
		t.Fatalf("%d rows left on the duplicate", n)
	}
	if n := plugintest.Count(t, tx, `SELECT COUNT(*) FROM doc_job WHERE service_id = $1`, svcInto); n != 1 {
		t.Fatal("the duplicate service's job did not move to the kept service")
	}
	if n := plugintest.Count(t, tx, `SELECT COUNT(*) FROM doc_consumable WHERE service_id = $1 AND product_id = $2`, other, into); n != 2 {
		t.Fatalf("paper rows %d, want A4 (kept) + A3 (moved)", n)
	}
}
