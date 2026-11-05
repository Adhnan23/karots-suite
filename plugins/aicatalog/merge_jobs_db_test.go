package aicatalog

import (
	"context"
	"strings"
	"testing"

	"karots-suite/internal/plugin/plugintest"
	"karots-suite/plugins/aicatalog/migrations"
)

func TestMergeMovesJobsAndOldNames(t *testing.T) {
	tx, from, into := plugintest.MergeTx(t, migrations.FS, "goose_db_version_aicatalog")
	ins := `INSERT INTO aicatalog_items (product_id, resolved_name, source, old_names) VALUES ($1, $2, 'user_picked', $3)`
	tx.MustExec(ins, from, "dup", "OLD-1")
	tx.MustExec(ins, into, "kept", "")
	job := `INSERT INTO aicatalog_job_items (job, product_id, state) VALUES ($1, $2, $3)`
	tx.MustExec(job, "identify", from, "skipped")
	tx.MustExec(job, "identify", into, "applied")
	tx.MustExec(job, "groups", from, "applied")
	if err := mergeProduct(context.Background(), tx, from, into); err != nil {
		t.Fatal(err)
	}
	var names, state string
	if err := tx.Get(&names, `SELECT old_names FROM aicatalog_items WHERE product_id = $1`, into); err != nil || !strings.Contains(names, "OLD-1") {
		t.Fatalf("old names %q %v", names, err)
	}
	if err := tx.Get(&state, `SELECT state FROM aicatalog_job_items WHERE job = 'identify' AND product_id = $1`, into); err != nil || state != "applied" {
		t.Fatalf("kept state %q %v", state, err)
	}
	if n := plugintest.Count(t, tx, `SELECT COUNT(*) FROM aicatalog_job_items WHERE product_id = $1`, into); n != 2 {
		t.Fatalf("job rows on the kept product: %d", n)
	}
	if n := plugintest.Count(t, tx, `SELECT COUNT(*) FROM aicatalog_job_items WHERE product_id = $1`, from); n != 0 {
		t.Fatalf("job rows left on the duplicate: %d", n)
	}
}
