package alternatives

import (
	"context"
	"testing"

	"karots-suite/internal/plugin/plugintest"
	"karots-suite/plugins/alternatives/migrations"
)

// Merge: a duplicate in a tier while the kept product is in another drops its own
// membership (exactly-one); one alone in a tier hands it to the kept product.
func TestMergeProduct(t *testing.T) {
	tx, from, into := plugintest.MergeTx(t, migrations.FS, "goose_db_version_alternatives")
	ctx := context.Background()
	var g, tier int64
	tx.Get(&g, `INSERT INTO alt_groups (name) VALUES ('m') RETURNING id`)                    //nolint:errcheck
	tx.Get(&tier, `INSERT INTO alt_tiers (group_id, name) VALUES ($1, 'x') RETURNING id`, g) //nolint:errcheck
	tx.MustExec(`INSERT INTO alt_members (product_id, tier_id) VALUES ($1, $3), ($2, $3)`, from, into, tier)
	if err := mergeProduct(ctx, tx, from, into); err != nil {
		t.Fatal(err)
	}
	if n := plugintest.Count(t, tx, `SELECT COUNT(*) FROM alt_members WHERE product_id IN ($1, $2)`, from, into); n != 1 {
		t.Fatalf("members %d, want the kept one only", n)
	}
	tx.MustExec(`DELETE FROM alt_members WHERE product_id = $1`, into)
	tx.MustExec(`INSERT INTO alt_members (product_id, tier_id) VALUES ($1, $2)`, from, tier)
	if err := mergeProduct(ctx, tx, from, into); err != nil {
		t.Fatal(err)
	}
	if n := plugintest.Count(t, tx, `SELECT COUNT(*) FROM alt_members WHERE product_id = $1 AND tier_id = $2`, into, tier); n != 1 {
		t.Fatal("the duplicate's membership did not move to the kept product")
	}
}
